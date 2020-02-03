package main

import (
	"context"
	"errors"
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/storage"
	t "github.com/teamnsrg/mida/types"
	"github.com/teamnsrg/mida/util"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"
)

func CrawlerInstance(sanitizedTaskChan <-chan t.SanitizedMIDATask, rawResultChan chan<- t.RawMIDAResult, retryChan <-chan t.SanitizedMIDATask, crawlerWG *sync.WaitGroup) {

	for sanitizedTaskChan != nil {
		select {
		case st, ok := <-retryChan:
			if !ok {
				retryChan = nil
			} else {
				log.Log.WithField("URL", st.Url).Info("Begin Retry Crawl")
				rawResult, err := ProcessSanitizedTask(st)
				if err != nil {
					// This should never happen (even if the task fails), so we make it fatal
					log.Log.Fatal(err)
				}
				log.Log.WithField("URL", st.Url).Info("End Retry Crawl")
				// Put our raw crawl result into the Raw Result Channel, where it will be validated and post-processed
				rawResultChan <- rawResult
			}
		case st, ok := <-sanitizedTaskChan:
			if !ok {
				sanitizedTaskChan = nil
			} else {
				log.Log.WithField("URL", st.Url).Info("Begin Crawl")
				rawResult, err := ProcessSanitizedTask(st)
				if err != nil {
					log.Log.Fatal(err)
				}
				log.Log.WithField("URL", st.Url).Info("End Crawl")
				// Put our raw crawl result into the Raw Result Channel, where it will be validated and post-processed
				rawResultChan <- rawResult
			}
		}
	}

	// RawMIDAResult channel is closed once all crawlers have exited, where they are first created
	crawlerWG.Done()

	return
}

func ProcessSanitizedTask(st t.SanitizedMIDATask) (t.RawMIDAResult, error) {

	rawResult := t.RawMIDAResult{
		Requests:      make(map[string][]network.EventRequestWillBeSent),
		Responses:     make(map[string][]network.EventResponseReceived),
		Scripts:       make(map[string]debugger.EventScriptParsed),
		WebsocketData: make(map[string]*t.WSConnection),
		SanitizedTask: st,
	}
	var rawResultLock sync.Mutex // Should be used any time this object is updated

	navChan := make(chan error, 1)
	timeoutChan := time.After(time.Duration(st.Timeout) * time.Second)
	loadEventChan := make(chan bool, 5)
	postCrawlActionsChan := make(chan bool, 1)

	var eventHandlerWG sync.WaitGroup

	// Event channels (used to asynchronously process DevTools events)
	// Naming Convention: <event name>Chan
	// Buffers need to be big enough that the demux never blocks
	ec := openEventChannels()

	rawResultLock.Lock()
	rawResult.Stats.Timing.BeginCrawl = time.Now()
	rawResult.SanitizedTask = st
	rawResultLock.Unlock()

	// Create our user data directory, if it does not yet exist
	if st.UserDataDirectory == "" {
		st.UserDataDirectory = path.Join(storage.TempDir, st.RandomIdentifier)
	}

	// Make sure user data directory exists already. If not, create it
	_, err := os.Stat(st.UserDataDirectory)
	if err != nil {
		err = os.MkdirAll(st.UserDataDirectory, 0744)
		if err != nil {
			log.Log.WithField("URL", st.Url).Fatal(err)
		}
	}

	// Create our temporary results directory within the user data directory
	resultsDir := path.Join(st.UserDataDirectory, st.RandomIdentifier)
	_, err = os.Stat(resultsDir)
	if err != nil {
		err = os.MkdirAll(resultsDir, 0755)
		if err != nil {
			log.Log.WithField("URL", st.Url).Fatal("Error creating results directory")
		}
	} else {
		log.Log.WithField("URL", st.Url).Error("Results directory already existed within user data directory")
	}

	if st.AllResources {
		// Create a subdirectory where we will store all the files
		_, err = os.Stat(path.Join(resultsDir, storage.DefaultFileSubdir))
		if err != nil {
			err = os.MkdirAll(path.Join(resultsDir, storage.DefaultFileSubdir), 0744)
			if err != nil {
				log.Log.WithField("URL", st.Url).Fatal(err)
			}
		}
	}

	if st.AllScripts {
		// Create a subdirectory where we will store all scripts parsed by browser
		_, err = os.Stat(path.Join(resultsDir, storage.DefaultScriptSubdir))
		if err != nil {
			err = os.MkdirAll(path.Join(resultsDir, storage.DefaultScriptSubdir), 0744)
			if err != nil {
				log.Log.WithField("URL", st.Url).Fatal(err)
			}
		}
	}

	// Add these the port and the user data directory as arguments to the browser as we start it up
	// No other flags should be added here unless there is a good reason they can't be put in
	// the pipeline earlier.

	// Append browser options along with exe path
	var opts []chromedp.ExecAllocatorOption
	for _, flagString := range st.BrowserFlags {
		name, val, err := util.FormatFlag(flagString)
		if err != nil {
			log.Log.WithField("URL", st.Url).Error(err)
			continue
		}
		opts = append(opts, chromedp.Flag(name, val))
	}

	opts = append(opts, chromedp.Flag("user-data-dir", st.UserDataDirectory))
	opts = append(opts, chromedp.ExecPath(st.BrowserBinary))
	if st.BrowserCoverage {
		// Set up environment so that Chromium will save coverage data
		opts = append(opts, chromedp.Env("LLVM_PROFILE_FILE="+path.Join(resultsDir, storage.DefaultCoverageSubdir, "coverage-%4m.profraw")))

		// Create directory which will contain coverage files
		_, err = os.Stat(path.Join(resultsDir, storage.DefaultCoverageSubdir))
		if err != nil {
			err = os.MkdirAll(path.Join(resultsDir, storage.DefaultCoverageSubdir), 0744)
			if err != nil {
				log.Log.WithField("URL", st.Url).Fatal(err)
			}
		}
	}

	if st.JSTrace {
		midaBrowserOutfile, err := os.Create(path.Join(resultsDir, storage.DefaultBrowserLogFileName))
		if err != nil {
			log.Log.WithField("URL", st.Url).Fatal(err)
		}
		// This allows us to redirect the output from the browser to a file we choose.
		opts = append(opts, chromedp.CombinedOutput(midaBrowserOutfile))
	}

	// Spawn the browser
	allocContext, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	cxt, _ := chromedp.NewContext(allocContext)

	// Event Demux - just receive the events and stick them in the applicable channels
	chromedp.ListenTarget(cxt, func(ev interface{}) {
		eventHandlerWG.Add(1)
		switch ev.(type) {
		// Page Domain Events
		case *page.EventLoadEventFired:
			ec.loadEventFiredChan <- ev.(*page.EventLoadEventFired)
		case *page.EventDomContentEventFired:
			ec.domContentEventFiredChan <- ev.(*page.EventDomContentEventFired)

		// General Network Domain Events
		case *network.EventRequestWillBeSent:
			ec.requestWillBeSentChan <- ev.(*network.EventRequestWillBeSent)
		case *network.EventResponseReceived:
			ec.responseReceivedChan <- ev.(*network.EventResponseReceived)
		case *network.EventLoadingFinished:
			ec.loadingFinishedChan <- ev.(*network.EventLoadingFinished)

		// Websocket Network Domain Events
		case *network.EventWebSocketCreated:
			ec.webSocketCreatedChan <- ev.(*network.EventWebSocketCreated)
		case *network.EventWebSocketFrameSent:
			ec.webSocketFrameSentChan <- ev.(*network.EventWebSocketFrameSent)
		case *network.EventWebSocketFrameReceived:
			ec.webSocketFrameReceivedChan <- ev.(*network.EventWebSocketFrameReceived)
		case *network.EventWebSocketFrameError:
			ec.webSocketFrameErrorChan <- ev.(*network.EventWebSocketFrameError)
		case *network.EventWebSocketClosed:
			ec.webSocketClosedChan <- ev.(*network.EventWebSocketClosed)
		case *network.EventWebSocketWillSendHandshakeRequest:
			ec.webSocketWillSendHandshakeRequestChan <- ev.(*network.EventWebSocketWillSendHandshakeRequest)
		case *network.EventWebSocketHandshakeResponseReceived:
			ec.webSocketHandshakeResponseReceivedChan <- ev.(*network.EventWebSocketHandshakeResponseReceived)

		case *fetch.EventRequestPaused:
			ec.requestPausedChan <- ev.(*fetch.EventRequestPaused)

		// Debugger Domain Events
		case *debugger.EventScriptParsed:
			ec.scriptParsedChan <- ev.(*debugger.EventScriptParsed)

		default:
			eventHandlerWG.Done()

		}
	})

	/*
		// Maximize browser window
		err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
			windowId, _, err := browser.GetWindowForTarget().Do(cxt)
			if err != nil {
				return err
			}

				err = browser.SetWindowBounds(windowId, &browser.Bounds{WindowState: "fullscreen"}).Do(cxt)
			if err != nil {
				return err
			}

				return nil

			}))
		if err != nil {
			closeEventChannels(ec)
				eventHandlerWG.Wait()
				cancel()

				rawResultLock.Lock()
			rawResult.SanitizedTask.TaskFailed = true
			rawResult.SanitizedTask.FailureCode = err.Error()
			rawResultLock.Unlock()

				return rawResult, nil
		}
	*/

	// Ensure the correct domains are enabled/disabled
	err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
		err = runtime.Disable().Do(cxt)
		if err != nil {
			return err
		}

		err = page.Enable().Do(cxt)
		if err != nil {
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

		err = fetch.Enable().Do(cxt)
		if err != nil {
			return err
		}

		return nil
	}))
	if err != nil {

		rawResultLock.Lock()
		rawResult.SanitizedTask.TaskFailed = true
		rawResult.SanitizedTask.FailureCode = err.Error()
		rawResultLock.Unlock()

		closeCxt, _ := context.WithTimeout(cxt, 5*time.Second)
		err = chromedp.Cancel(closeCxt)
		if err != nil {
			log.Log.Error(err)
		}
		closeEventChannels(ec)
		eventHandlerWG.Wait()

		return rawResult, nil
	}

	// Get browser data from DevTools
	err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
		protocolVersion, product, revision, userAgent, jsVersion, err := browser.GetVersion().Do(cxt)
		if err != nil {
			return err
		}
		rawResultLock.Lock()
		rawResult.CrawlHostInfo.DevToolsVersion = protocolVersion
		rawResult.CrawlHostInfo.Browser = product
		rawResult.CrawlHostInfo.V8Version = jsVersion
		rawResult.CrawlHostInfo.BrowserVersion = revision
		rawResult.CrawlHostInfo.UserAgent = userAgent
		hostname, err := os.Hostname()
		if err != nil {
			log.Log.WithField("URL", st.Url).Fatal(err)
		}
		rawResult.CrawlHostInfo.HostName = hostname
		rawResultLock.Unlock()

		return nil
	}))
	if err != nil {
		rawResultLock.Lock()
		rawResult.SanitizedTask.TaskFailed = true
		rawResult.SanitizedTask.FailureCode = err.Error()
		rawResultLock.Unlock()

		closeCxt, _ := context.WithTimeout(cxt, 5*time.Second)
		err = chromedp.Cancel(closeCxt)
		if err != nil {
			log.Log.Error(err)
		}
		closeEventChannels(ec)
		eventHandlerWG.Wait()

		return rawResult, nil
	}

	// Event Handler : Page.loadEventFired
	go func() {
		for range ec.loadEventFiredChan {
			rawResultLock.Lock()
			if rawResult.Stats.Timing.LoadEvent.IsZero() {
				rawResult.Stats.Timing.LoadEvent = time.Now()
			} else {
				log.Log.WithField("URL", st.Url).Warn("Duplicate load event")
			}
			rawResultLock.Unlock()

			var sendLoadEvent bool
			rawResultLock.Lock()
			if rawResult.SanitizedTask.CCond == CompleteOnTimeoutAfterLoad || rawResult.SanitizedTask.CCond == CompleteOnLoadEvent {
				sendLoadEvent = true
			}
			rawResultLock.Unlock()
			if sendLoadEvent {
				loadEventChan <- true
			}

			postCrawlActionsChan <- true

			eventHandlerWG.Done()
		}
	}()

	// Event Handler: Page.domContentEventFired
	go func() {
		for range ec.domContentEventFiredChan {
			rawResultLock.Lock()
			if rawResult.Stats.Timing.DOMContentEvent.IsZero() {
				rawResult.Stats.Timing.DOMContentEvent = time.Now()
			} else {
				log.Log.WithField("URL", st.Url).Warn("Duplicate DOMContentLoaded event")
			}
			rawResultLock.Unlock()
			eventHandlerWG.Done()
		}
	}()

	// Event Handler: Network.requestWillBeSent
	go func() {
		for data := range ec.requestWillBeSentChan {
			rawResultLock.Lock()
			rawResult.Requests[data.RequestID.String()] = append(rawResult.Requests[data.RequestID.String()], *data)
			rawResultLock.Unlock()
			eventHandlerWG.Done()
		}
	}()

	// Event Handler: Network.responseReceived
	go func() {
		for data := range ec.responseReceivedChan {
			rawResultLock.Lock()
			rawResult.Responses[data.RequestID.String()] = append(rawResult.Responses[data.RequestID.String()], *data)
			rawResultLock.Unlock()
			eventHandlerWG.Done()
		}
	}()

	// Event Handler: Network.loadingFinished
	go func() {
		for data := range ec.loadingFinishedChan {
			if st.AllResources {
				rawResultLock.Lock()
				if _, ok := rawResult.Requests[data.RequestID.String()]; !ok {
					log.Log.WithField("URL", st.Url).Debug("Will not get response body for unknown RequestID")
					rawResultLock.Unlock()
					return
				}
				rawResultLock.Unlock()
				var respBody []byte
				err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
					respBody, err = network.GetResponseBody(data.RequestID).Do(cxt)
					return err
				}))
				if err != nil {
					// The browser was unable to provide the content of this particular resource
					// This typically happens when we closed the browser before we could save all resources
					log.Log.WithField("URL", st.Url).Debug("Failed to get response Body for known resource: ", data.RequestID)
				} else {
					err = ioutil.WriteFile(path.Join(resultsDir, storage.DefaultFileSubdir, data.RequestID.String()), respBody, os.ModePerm)
					if err != nil {
						log.Log.WithField("URL", st.Url).Fatal(err)
					}
				}
			}
			eventHandlerWG.Done()
		}
	}()

	// Event Handler: Network.webSocketCreated
	go func() {
		for data := range ec.webSocketCreatedChan {
			rawResultLock.Lock()
			if _, ok := rawResult.WebsocketData[data.RequestID.String()]; !ok {
				// Create our new websocket connection
				wsc := t.WSConnection{
					Url:                data.URL,
					Initiator:          data.Initiator,
					HandshakeRequests:  make([]*network.EventWebSocketWillSendHandshakeRequest, 0),
					HandshakeResponses: make([]*network.EventWebSocketHandshakeResponseReceived, 0),
					FramesSent:         make([]*network.EventWebSocketFrameSent, 0),
					FramesReceived:     make([]*network.EventWebSocketFrameReceived, 0),
					FrameErrors:        make([]*network.EventWebSocketFrameError, 0),
					TSOpen:             time.Now().String(),
					TSClose:            "",
				}
				rawResult.WebsocketData[data.RequestID.String()] = &wsc
			}
			rawResultLock.Unlock()
			eventHandlerWG.Done()
		}
	}()

	// Event Handler: Network.webSocketFrameSent
	go func() {
		for data := range ec.webSocketFrameSentChan {
			rawResultLock.Lock()
			if _, ok := rawResult.WebsocketData[data.RequestID.String()]; ok {
				// Create our new websocket connection
				rawResult.WebsocketData[data.RequestID.String()].FramesSent = append(
					rawResult.WebsocketData[data.RequestID.String()].FramesSent, data)
			}
			// Otherwise, we ignore a frame for a connection we don't know about
			rawResultLock.Unlock()
			eventHandlerWG.Done()
		}
	}()

	// Event Handler: Network.webSocketFrameReceived
	go func() {
		for data := range ec.webSocketFrameReceivedChan {
			rawResultLock.Lock()
			if _, ok := rawResult.WebsocketData[data.RequestID.String()]; ok {
				// Create our new websocket connection
				rawResult.WebsocketData[data.RequestID.String()].FramesReceived = append(
					rawResult.WebsocketData[data.RequestID.String()].FramesReceived, data)
			}
			// Otherwise, we ignore a frame for a connection we don't know about
			rawResultLock.Unlock()
			eventHandlerWG.Done()
		}
	}()

	// Event Handler: Network.webSocketFrameError
	go func() {
		for data := range ec.webSocketFrameErrorChan {
			rawResultLock.Lock()
			if _, ok := rawResult.WebsocketData[data.RequestID.String()]; ok {
				// Create our new websocket connection
				rawResult.WebsocketData[data.RequestID.String()].FrameErrors = append(
					rawResult.WebsocketData[data.RequestID.String()].FrameErrors, data)
			}
			// Otherwise, we ignore a frame for a connection we don't know about
			rawResultLock.Unlock()
			eventHandlerWG.Done()
		}
	}()

	// Event Handler: Network.webSocketClosed
	go func() {
		for data := range ec.webSocketClosedChan {
			rawResultLock.Lock()
			if _, ok := rawResult.WebsocketData[data.RequestID.String()]; ok {
				// Create our new websocket connection
				rawResult.WebsocketData[data.RequestID.String()].TSClose = data.Timestamp.Time().String()
			}
			// Otherwise, we ignore a frame for a connection we don't know about
			rawResultLock.Unlock()
			eventHandlerWG.Done()
		}
	}()

	// Event Handler: Network.websocketWillSendHandshakeRequest
	go func() {
		for data := range ec.webSocketWillSendHandshakeRequestChan {
			rawResultLock.Lock()
			if _, ok := rawResult.WebsocketData[data.RequestID.String()]; ok {
				// Create our new websocket connection
				rawResult.WebsocketData[data.RequestID.String()].HandshakeRequests = append(
					rawResult.WebsocketData[data.RequestID.String()].HandshakeRequests, data)
			}
			// Otherwise, we ignore a frame for a connection we don't know about
			rawResultLock.Unlock()
			eventHandlerWG.Done()
		}
	}()

	// Event Handler: Network.websocketHandshakeResponseReceived
	go func() {
		for data := range ec.webSocketHandshakeResponseReceivedChan {
			rawResultLock.Lock()
			if _, ok := rawResult.WebsocketData[data.RequestID.String()]; ok {
				// Create our new websocket connection
				rawResult.WebsocketData[data.RequestID.String()].HandshakeResponses = append(
					rawResult.WebsocketData[data.RequestID.String()].HandshakeResponses, data)
			}
			// Otherwise, we ignore a frame for a connection we don't know about
			rawResultLock.Unlock()
			eventHandlerWG.Done()
		}
	}()

	// Event Handler: fetch.requestPaused
	go func() {
		for data := range ec.requestPausedChan {
			err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
				err = fetch.ContinueRequest(data.RequestID).Do(cxt)
				return err
			}))
			if err != nil {
				log.Log.WithField("URL", st.Url).Error(err)
			}

			eventHandlerWG.Done()
		}
	}()

	// Event Handler: Debugger.scriptParsed
	go func() {
		for data := range ec.scriptParsedChan {
			rawResultLock.Lock()
			rawResult.Scripts[data.ScriptID.String()] = *data
			rawResultLock.Unlock()
			if st.AllScripts {
				var source string
				err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
					// Second return value here is Wasm bytecode -- may want to grab that at some point
					source, _, err = debugger.GetScriptSource(data.ScriptID).Do(cxt)
					if err != nil {
						return err
					}

					return nil
				}))
				if err != nil && err.Error() != "context canceled" {
					log.Log.WithField("URL", st.Url).Error("Failed to get script source")
					log.Log.WithField("URL", st.Url).Error(err)
				} else {
					err = ioutil.WriteFile(path.Join(resultsDir, storage.DefaultScriptSubdir, data.ScriptID.String()), []byte(source), os.ModePerm)
					if err != nil {
						log.Log.WithField("URL", st.Url).Fatal(err)
					}
				}
			}

			eventHandlerWG.Done()
		}
	}()

	// Cloaking
	err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(
			`Object.defineProperty(navigator, 'webdriver', { get: () => false, });
			window.navigator.chrome = {runtime: {},};
			window.chrome = {runtime: {},};`).Do(cxt)
		if err != nil {
			return err
		}
		return nil
	}))
	if err != nil {
		log.Log.WithField("URL", st.Url).Error(err)
	}

	// Below is the MIDA navigation logic. Since MIDA offers several different
	// termination conditions (Terminate on timout, terminate on load event, etc.),
	// this logic gets a little complex.
	go func() {
		err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
			_, _, text, err := page.Navigate(st.Url).Do(cxt)
			if err != nil {
				return err
			} else if text != "" {
				return errors.New(text)
			} else {
				return nil
			}
		}))
		if err != nil {
			log.Log.WithField("URL", st.Url).Error("Nav error: ", err)
		}
		navChan <- err
	}()
	select {
	case err = <-navChan:
		rawResult.Stats.Timing.ConnectionEstablished = time.Now()
	case <-time.After(DefaultNavTimeout * time.Second):
		// This usually happens because we successfully resolved DNS,
		// but we could not connect to the server (but reset didn't get a RST either)
		err = errors.New("nav timeout during connection to site")
	case <-timeoutChan:
		// Timeout is set shorter than DefaultNavTimeout, so we are just done
		err = errors.New("full timeout during connection to site")
	}
	if err != nil {
		// We failed to connect to the site. Shut things down.
		rawResultLock.Lock()
		rawResult.SanitizedTask.TaskFailed = true
		rawResult.SanitizedTask.FailureCode = err.Error()
		rawResultLock.Unlock()

		if err != nil {
			log.Log.WithField("URL", st.Url).Error("Failed to navigate to site: ", err)
		}

		closeCxt, _ := context.WithTimeout(cxt, 5*time.Second)
		err = chromedp.Cancel(closeCxt)
		if err != nil {
			log.Log.Error(err)
		}
		closeEventChannels(ec)
		eventHandlerWG.Wait()

		rawResultLock.Lock()
		rawResult.Stats.Timing.BrowserClose = time.Now()
		rawResultLock.Unlock()

		return rawResult, nil

	} else {
		// We successfully connected to the site. At this point, we WILL gather results.
		// Wait for our termination condition.
		select {
		// This will only arrive if we are using a completion condition that requires load events
		case <-loadEventChan:
			var ccond t.CompletionCondition
			var timeAfterLoad int
			rawResultLock.Lock()
			ccond = rawResult.SanitizedTask.CCond
			timeAfterLoad = rawResult.SanitizedTask.TimeAfterLoad
			rawResultLock.Unlock()
			if ccond == CompleteOnTimeoutAfterLoad {
				select {
				case <-timeoutChan:
					// We did not make it to the TimeAfterLoad. Too Bad.
				case <-time.After(time.Duration(timeAfterLoad) * time.Second):
					// We made it to TimeAfterLoad. Nothing else to wait on.
				}
			} else if ccond == CompleteOnLoadEvent {
				// Do nothing here -- The load event happened already, we are done
			} else if ccond == CompleteOnTimeoutOnly {
				log.Log.WithField("URL", st.Url).Error("Unexpectedly received load event through channel on TimeoutOnly crawl")
			}
		case <-postCrawlActionsChan:
			// We are free to begin post crawl data gathering which requires the browser
			// Examples: Screenshot, DOM snapshot, code coverage, etc.
			// These actions may or may not finish -- We still have to be careful to observe the timeout
			log.Log.WithField("URL", st.Url).Debug("Beginning post crawl actions")

			go func() {
				err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {

					if st.ScreenShot {
						data, err := page.CaptureScreenshot().Do(cxt)
						if err != nil {
							return err
						}

						err = ioutil.WriteFile(path.Join(resultsDir, storage.DefaultScreenShotFileName), data, os.ModePerm)
						if err != nil {
							log.Log.WithField("URL", st.Url).Error(err)
						} else {
							log.Log.WithField("URL", st.Url).Debug("ScreenShot Recorded")
						}
					}

					return nil
				}))
			}()
			// Wait for the timeout to happen
			<-timeoutChan

		case <-timeoutChan:

		}
	}

	closeCxt, _ := context.WithTimeout(cxt, 5*time.Second)
	err = chromedp.Cancel(closeCxt)
	if err != nil {
		log.Log.Error(err)
	}
	closeEventChannels(ec)
	eventHandlerWG.Wait()

	rawResultLock.Lock()
	rawResult.Stats.Timing.BrowserClose = time.Now()
	rawResultLock.Unlock()

	return rawResult, nil

}

type EventChannels struct {
	loadEventFiredChan       chan *page.EventLoadEventFired
	domContentEventFiredChan chan *page.EventDomContentEventFired

	requestWillBeSentChan                  chan *network.EventRequestWillBeSent
	responseReceivedChan                   chan *network.EventResponseReceived
	loadingFinishedChan                    chan *network.EventLoadingFinished
	webSocketCreatedChan                   chan *network.EventWebSocketCreated
	webSocketFrameSentChan                 chan *network.EventWebSocketFrameSent
	webSocketFrameReceivedChan             chan *network.EventWebSocketFrameReceived
	webSocketFrameErrorChan                chan *network.EventWebSocketFrameError
	webSocketClosedChan                    chan *network.EventWebSocketClosed
	webSocketWillSendHandshakeRequestChan  chan *network.EventWebSocketWillSendHandshakeRequest
	webSocketHandshakeResponseReceivedChan chan *network.EventWebSocketHandshakeResponseReceived

	requestPausedChan chan *fetch.EventRequestPaused

	scriptParsedChan chan *debugger.EventScriptParsed
}

func openEventChannels() EventChannels {
	ec := EventChannels{}
	ec.loadEventFiredChan = make(chan *page.EventLoadEventFired, 100)
	ec.domContentEventFiredChan = make(chan *page.EventDomContentEventFired, 100)

	ec.requestWillBeSentChan = make(chan *network.EventRequestWillBeSent, 10000)
	ec.responseReceivedChan = make(chan *network.EventResponseReceived, 10000)
	ec.loadingFinishedChan = make(chan *network.EventLoadingFinished, 10000)
	ec.webSocketCreatedChan = make(chan *network.EventWebSocketCreated, 10000)
	ec.webSocketFrameSentChan = make(chan *network.EventWebSocketFrameSent, 10000)
	ec.webSocketFrameReceivedChan = make(chan *network.EventWebSocketFrameReceived, 10000)
	ec.webSocketFrameErrorChan = make(chan *network.EventWebSocketFrameError, 10000)
	ec.webSocketClosedChan = make(chan *network.EventWebSocketClosed, 10000)
	ec.webSocketWillSendHandshakeRequestChan = make(chan *network.EventWebSocketWillSendHandshakeRequest, 10000)
	ec.webSocketHandshakeResponseReceivedChan = make(chan *network.EventWebSocketHandshakeResponseReceived, 10000)

	ec.requestPausedChan = make(chan *fetch.EventRequestPaused, 10000)

	ec.scriptParsedChan = make(chan *debugger.EventScriptParsed, 10000)

	return ec
}

func closeEventChannels(ec EventChannels) {
	close(ec.loadEventFiredChan)
	close(ec.domContentEventFiredChan)

	close(ec.requestWillBeSentChan)
	close(ec.responseReceivedChan)
	close(ec.loadingFinishedChan)
	close(ec.webSocketCreatedChan)
	close(ec.webSocketFrameSentChan)
	close(ec.webSocketFrameReceivedChan)
	close(ec.webSocketFrameErrorChan)
	close(ec.webSocketClosedChan)
	close(ec.webSocketWillSendHandshakeRequestChan)
	close(ec.webSocketHandshakeResponseReceivedChan)

	close(ec.requestPausedChan)

	close(ec.scriptParsedChan)
}
