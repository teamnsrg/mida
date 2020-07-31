package browser

import (
	"context"
	"errors"
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

// EventChannels are a wrapper around all of the channels where we will deliver
// messages from the various events fires by DevTools
type EventChannels struct {
	loadEventFiredChan                     chan *page.EventLoadEventFired
	domContentEventFiredChan               chan *page.EventDomContentEventFired
	frameNavigatedChan                     chan *page.EventFrameNavigated
	requestWillBeSentChan                  chan *network.EventRequestWillBeSent
	responseReceivedChan                   chan *network.EventResponseReceived
	loadingFinishedChan                    chan *network.EventLoadingFinished
	dataReceivedChan                       chan *network.EventDataReceived
	webSocketCreatedChan                   chan *network.EventWebSocketCreated
	webSocketFrameSentChan                 chan *network.EventWebSocketFrameSent
	webSocketFrameReceivedChan             chan *network.EventWebSocketFrameReceived
	webSocketFrameErrorChan                chan *network.EventWebSocketFrameError
	webSocketClosedChan                    chan *network.EventWebSocketClosed
	webSocketWillSendHandshakeRequestChan  chan *network.EventWebSocketWillSendHandshakeRequest
	webSocketHandshakeResponseReceivedChan chan *network.EventWebSocketHandshakeResponseReceived
	EventSourceMessageReceivedChan         chan *network.EventEventSourceMessageReceived
	requestPausedChan                      chan *fetch.EventRequestPaused
	scriptParsedChan                       chan *debugger.EventScriptParsed
}

type DTState struct {
	mainFrameLoaderId string
	sync.Mutex
}

// VisitPageDevtoolsProtocol is a high level function that takes a pre-sanitized TaskWrapper and processes
// it by opening a DevTools Protocol-compatible browser. It produces a RawResult object, and writes relevant
// results files to disk as specified by the Task in the TaskWrapper.
func VisitPageDevtoolsProtocol(tw *b.TaskWrapper) (*b.RawResult, error) {
	var err error

	// Fully allocate our raw result object -- should be locked whenever it is read or written
	rawResult := b.RawResult{
		CrawlerInfo: b.CrawlerInfo{},
		TaskSummary: b.TaskSummary{
			Success:      false,
			TaskWrapper:  tw,
			TaskTiming:   b.TaskTiming{},
			NumResources: 0,
		},
		DevTools: b.DevToolsRawData{
			Network: b.DevtoolsNetworkRawData{
				RequestWillBeSent: make(map[string][]network.EventRequestWillBeSent),
				ResponseReceived:  make(map[string]network.EventResponseReceived),
			},
		},
	}

	// Open all the event channels we will use to receive events from DevTools
	ec := openEventChannels()

	// DevTools-specific state we need to use across various goroutines
	var devToolsState DTState

	// Make sure user data directory exists already. If not, create it.
	// If we can't create it, we consider it a bad enough error that we
	// return an error -- likely a major misconfiguration
	_, err = os.Stat(tw.SanitizedTask.UserDataDirectory)
	if err != nil {
		err = os.MkdirAll(tw.SanitizedTask.UserDataDirectory, 0744)
		if err != nil {
			return nil, err
		}
	}

	// If we are gathering all the resources, we need to create the corresponding directory
	if *(tw.SanitizedTask.DS.AllResources) {
		// Create a subdirectory where we will store all the files
		_, err = os.Stat(path.Join(tw.TempDir, b.DefaultResourceSubdir))
		if err != nil {
			err = os.MkdirAll(path.Join(tw.TempDir, b.DefaultResourceSubdir), 0744)
			if err != nil {
				tw.Log.Error("failed to create resource subdir within UDD")
				return nil, err
			}
		}
	}

	// Build our opts slice
	var opts []chromedp.ExecAllocatorOption
	for _, flagString := range tw.SanitizedTask.BrowserFlags {
		name, val, err := ChromeFormatFlag(flagString)
		if err != nil {
			// We got a bad flag
			tw.Log.Errorf("Skipping bad flag: %s", flagString)
			continue
		}
		opts = append(opts, chromedp.Flag(name, val))
	}

	opts = append(opts, chromedp.UserDataDir(tw.SanitizedTask.UserDataDirectory))
	opts = append(opts, chromedp.ExecPath(tw.SanitizedTask.BrowserBinaryPath))

	// Build channels we need for coordinating the site visit across goroutines
	navChan := make(chan error)                                                          // A channel to signal the completion of navigation, successfully or not
	timeoutChan := time.After(time.Duration(*tw.SanitizedTask.CS.Timeout) * time.Second) // Absolute longest we can go
	loadEventChan := make(chan bool)                                                     // Used to signal the firing of load events
	var eventHandlerWG sync.WaitGroup                                                    // Used to make sure all the event handlers exit
	var postLoadWG sync.WaitGroup                                                        // Used to sync actions after load event

	// Spawn our browser
	allocContext, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	browserContext, _ := chromedp.NewContext(allocContext)

	// Get our event listener goroutines up and running
	eventHandlerWG.Add(6) // *** UPDATE ME WHEN YOU ADD A NEW EVENT HANDLER ***
	go FetchRequestPaused(ec.requestPausedChan, &rawResult, &devToolsState, &eventHandlerWG, browserContext)
	go PageFrameNavigated(ec.frameNavigatedChan, &devToolsState, &eventHandlerWG, browserContext)
	go PageLoadEventFired(ec.loadEventFiredChan, loadEventChan, &rawResult, &eventHandlerWG, browserContext)
	go NetworkLoadingFinished(ec.loadingFinishedChan, &rawResult, &eventHandlerWG, browserContext, tw.Log)
	go NetworkRequestWillBeSent(ec.requestWillBeSentChan, &rawResult, &eventHandlerWG, browserContext)
	go NetworkResponseReceived(ec.responseReceivedChan, &rawResult, &eventHandlerWG, browserContext)

	// Ensure the correct domains are enabled/disabled
	err = chromedp.Run(browserContext, chromedp.ActionFunc(func(cxt context.Context) error {
		err = page.Enable().Do(cxt)
		if err != nil {
			log.Log.Error(err)
			return err
		}

		_, err = debugger.Enable().Do(cxt)
		if err != nil {
			return err
		}

		err = network.Enable().Do(cxt)
		if err != nil {
			return err
		}

		return nil
	}))
	if err != nil {
		// If we can't enable the domains on the browser, something is seriously wrong, so we return an error. No results.
		tw.Log.Error("failed to enable DevTools domains: ", err)
		log.Log.Error("failed to enable DevTools domains:", err)

		closeContext, _ := context.WithTimeout(browserContext, 5*time.Second)
		err = chromedp.Cancel(closeContext)
		if err != nil {
			// This isn't an ideal solution, but if the graceful close fails, we have to just kill the browser to free resources
			tw.Log.Errorf("failed to close browser gracefully, so we had to force it (%s)", err.Error())
			allocCancel()
		}

		// Wait for all event handlers to finish
		eventHandlerWG.Wait()
		return nil, errors.New("failed to enable DevTools domains")
	}

	// Event Demux - just receive the events and stick them in the applicable channels
	chromedp.ListenTarget(browserContext, func(ev interface{}) {
		switch ev.(type) {
		case *page.EventLoadEventFired:
			ec.loadEventFiredChan <- ev.(*page.EventLoadEventFired)
		case *page.EventFrameNavigated:
			ec.frameNavigatedChan <- ev.(*page.EventFrameNavigated)
		case *network.EventRequestWillBeSent:
			ec.requestWillBeSentChan <- ev.(*network.EventRequestWillBeSent)
		case *network.EventResponseReceived:
			ec.responseReceivedChan <- ev.(*network.EventResponseReceived)
		case *network.EventLoadingFinished:
			ec.loadingFinishedChan <- ev.(*network.EventLoadingFinished)
		case *fetch.EventRequestPaused:
			ec.requestPausedChan <- ev.(*fetch.EventRequestPaused)
		}
	})

	// Initiate navigation to the applicable page
	go func() {
		err = chromedp.Run(browserContext, chromedp.ActionFunc(func(ctxt context.Context) error {
			_, _, text, err := page.Navigate(tw.SanitizedTask.URL).Do(ctxt)
			if err != nil {
				return err
			} else if text != "" {
				return errors.New(text)
			} else {
				return nil
			}
		}))
		navChan <- err
	}()

	select {
	case err = <-navChan:
		rawResult.Lock()
		rawResult.TaskSummary.TaskTiming.ConnectionEstablished = time.Now()
		rawResult.Unlock()
	case <-time.After(b.DefaultNavTimeout * time.Second):
		// Our connection to the web server took longer than out navigation timeout (currently 30 seconds)
		err = errors.New("timeout on connection to webserver")
	case <-timeoutChan:
		err = errors.New("total site visit time exceeded before we connected to server")
	case <-browserContext.Done():
		// The browser somehow closed before we finished navigation
		err = errors.New("browser closed during connection to site")
	}
	if err != nil {
		// Save our error message for storage
		errorCode := err.Error()
		tw.Log.Errorf("failed to navigate to site: " + errorCode)
		log.Log.Errorf("failed to navigate to site: " + errorCode)

		// We have failed to navigate to the site. Shut things down.
		closeContext, _ := context.WithTimeout(browserContext, 5*time.Second)
		err = chromedp.Cancel(closeContext)
		if err != nil {
			// We failed to close chrome gracefully within the allotted timeout
			allocCancel()
			tw.Log.Errorf("failed to close browser gracefully, so we had to force it (%s)", err.Error())
		}

		eventHandlerWG.Wait()

		rawResult.Lock()
		rawResult.TaskSummary.TaskWrapper.FailureCode = errorCode
		rawResult.TaskSummary.Success = false
		rawResult.TaskSummary.TaskTiming.BrowserClose = time.Now()
		rawResult.Unlock()

		return &rawResult, nil
	}

	// We have now successfully connected and navigated to the site. Now we wait for a termination condition.
	select {
	case <-browserContext.Done():
		// Browser crashed, closed manually, or we otherwise lost connection to it prematurely
		tw.Log.Warn("browser crashed, closed manually, or we lost connection")
	case <-loadEventChan:
		// The load event fired. What we do next depends on how the crawl completes
		switch *tw.SanitizedTask.CS.CompletionCondition {
		case b.TimeAfterLoad:
			// We are waiting for some time after the load event, so we can initiate post load actions
			postLoadWG.Add(1)
			go postLoadActions(tw, browserContext, &postLoadWG)

			select {
			case <-browserContext.Done():
				// Browser crashed, closed manually, or we otherwise lost connection to it prematurely
				tw.Log.Warn("browser crashed, closed manually, or we lost connection (after load event)")
			case <-timeoutChan:
				// We hit our general timeout before we got to timeAfterLoad. Fall through to browser close and cleanup
				tw.Log.Debug("general timeout hit before timeAfterload")
			case <-time.After(time.Duration(*tw.SanitizedTask.CS.TimeAfterLoad) * time.Second):
				// We finished our timeAfterLoad period. Fall through to browser close and cleanup
				tw.Log.Debug("hit timeAfterLoad")
			}
		case b.LoadEvent:
			// We got out load event so we are just done, fall through to browser close and cleanup
			tw.Log.Debug("got load event so we are concluding site visit")
		case b.TimeoutOnly:
			// We need to just continue waiting for the timeout (or unexpected browser close).
			// We can begin any post load event actions we need to try
			postLoadWG.Add(1)
			go postLoadActions(tw, browserContext, &postLoadWG)

			select {
			case <-browserContext.Done():
				// Browser crashed, closed manually, or we otherwise lost connection to it prematurely
				tw.Log.Warn("browser crashed, closed manually, or we lost connection (after load event)")
			case <-timeoutChan:
				// We hit our general timeout, fall through to browser close and cleanup
				tw.Log.Debug("hit general timeout")
			}
		default:
			// This state should be unreachable -- got an unknown termination condition
			tw.Log.Error("got an unknown termination condition: ", *tw.SanitizedTask.CS.CompletionCondition)
		}
	case <-timeoutChan:
		// Timeout before load event was fired, fall through to browser close and cleanup
		tw.Log.Debug("general timeout before load event fired")
	}

	closeContext, _ := context.WithTimeout(browserContext, 5*time.Second)
	err = chromedp.Cancel(closeContext)
	if err != nil {
		tw.Log.Errorf("failed to close browser gracefully, so we had to force it (%s)", err.Error())
		allocCancel()
	}
	tw.Log.Debug("browser is now closed")

	// Store time at which we closed the browser
	rawResult.Lock()
	rawResult.TaskSummary.TaskTiming.BrowserClose = time.Now()
	rawResult.Unlock()

	// Wait for post load actions to finish
	postLoadWG.Wait()

	// Wait for all event handlers to finish
	eventHandlerWG.Wait()
	tw.Log.Debug("finished waiting on background goroutines, site visit concluded")

	return &rawResult, nil
}

// openEventChannels allocates all of the channels through which DevTools events are delivered to their event listeners
func openEventChannels() EventChannels {
	ec := EventChannels{
		loadEventFiredChan:                     make(chan *page.EventLoadEventFired, b.DefaultEventChannelBufferSize),
		domContentEventFiredChan:               make(chan *page.EventDomContentEventFired, b.DefaultEventChannelBufferSize),
		frameNavigatedChan:                     make(chan *page.EventFrameNavigated, b.DefaultEventChannelBufferSize),
		requestWillBeSentChan:                  make(chan *network.EventRequestWillBeSent, b.DefaultEventChannelBufferSize),
		responseReceivedChan:                   make(chan *network.EventResponseReceived, b.DefaultEventChannelBufferSize),
		loadingFinishedChan:                    make(chan *network.EventLoadingFinished, b.DefaultEventChannelBufferSize),
		dataReceivedChan:                       make(chan *network.EventDataReceived, b.DefaultEventChannelBufferSize),
		webSocketCreatedChan:                   make(chan *network.EventWebSocketCreated, b.DefaultEventChannelBufferSize),
		webSocketFrameSentChan:                 make(chan *network.EventWebSocketFrameSent, b.DefaultEventChannelBufferSize),
		webSocketFrameReceivedChan:             make(chan *network.EventWebSocketFrameReceived, b.DefaultEventChannelBufferSize),
		webSocketFrameErrorChan:                make(chan *network.EventWebSocketFrameError, b.DefaultEventChannelBufferSize),
		webSocketClosedChan:                    make(chan *network.EventWebSocketClosed, b.DefaultEventChannelBufferSize),
		webSocketWillSendHandshakeRequestChan:  make(chan *network.EventWebSocketWillSendHandshakeRequest, b.DefaultEventChannelBufferSize),
		webSocketHandshakeResponseReceivedChan: make(chan *network.EventWebSocketHandshakeResponseReceived, b.DefaultEventChannelBufferSize),
		EventSourceMessageReceivedChan:         make(chan *network.EventEventSourceMessageReceived, b.DefaultEventChannelBufferSize),
		requestPausedChan:                      make(chan *fetch.EventRequestPaused, b.DefaultEventChannelBufferSize),
		scriptParsedChan:                       make(chan *debugger.EventScriptParsed, b.DefaultEventChannelBufferSize),
	}

	return ec
}

// ChromeFormatFlag takes a variety of possible flag formats and puts them in a format that chromedp understands (key/value)
func ChromeFormatFlag(f string) (string, interface{}, error) {
	if strings.HasPrefix(f, "--") {
		f = f[2:]
	}

	parts := strings.Split(f, "=")
	if len(parts) == 1 {
		return parts[0], true, nil
	} else if len(parts) == 2 {
		return parts[0], parts[1], nil
	}

	return "", "", errors.New("invalid flag: " + f)
}
