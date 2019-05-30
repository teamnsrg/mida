package main

import (
	"context"
	"errors"
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/teamnsrg/mida/util"
	"io/ioutil"

	//"github.com/chromedp/cdproto/browser"
	//"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
	//"github.com/chromedp/cdproto/page"
	"github.com/pmurley/chromedp"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/storage"
	t "github.com/teamnsrg/mida/types"
	//"io/ioutil"
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

	// Event channels (used to asynchronously process DevTools events)
	// Naming Convention: <event name>Chan
	// Buffers need to be big enough that the demux never blocks
	loadEventFiredChan := make(chan *page.EventLoadEventFired, 100)
	domContentEventFiredChan := make(chan *page.EventDomContentEventFired, 100)
	requestWillBeSentChan := make(chan *network.EventRequestWillBeSent, 100000)
	responseReceivedChan := make(chan *network.EventResponseReceived, 100000)
	loadingFinishedChan := make(chan *network.EventLoadingFinished, 100000)
	webSocketCreatedChan := make(chan *network.EventWebSocketCreated, 100000)
	webSocketFrameSentChan := make(chan *network.EventWebSocketFrameSent, 100000)
	webSocketFrameReceivedChan := make(chan *network.EventWebSocketFrameReceived, 100000)
	webSocketFrameErrorChan := make(chan *network.EventWebSocketFrameError, 100000)
	webSocketClosedChan := make(chan *network.EventWebSocketClosed, 100000)
	webSocketWillSendHandshakeRequestChan := make(chan *network.EventWebSocketWillSendHandshakeRequest, 100000)
	webSocketHandshakeResponseReceivedChan := make(chan *network.EventWebSocketHandshakeResponseReceived, 100000)
	scriptParsedChan := make(chan *debugger.EventScriptParsed, 100000)

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
			log.Log.Fatal(err)
		}
	}

	// Create our temporary results directory within the user data directory
	resultsDir := path.Join(st.UserDataDirectory, st.RandomIdentifier)
	_, err = os.Stat(resultsDir)
	if err != nil {
		err = os.MkdirAll(resultsDir, 0755)
		if err != nil {
			log.Log.Fatal("Error creating results directory")
		}
	} else {
		log.Log.Error("Results directory already existed within user data directory")
	}

	if st.AllResources {
		// Create a subdirectory where we will store all the files
		_, err = os.Stat(path.Join(resultsDir, storage.DefaultFileSubdir))
		if err != nil {
			err = os.MkdirAll(path.Join(resultsDir, storage.DefaultFileSubdir), 0744)
			if err != nil {
				log.Log.Fatal(err)
			}
		}
	}

	if st.AllScripts {
		// Create a subdirectory where we will store all scripts parsed by browser
		_, err = os.Stat(path.Join(resultsDir, storage.DefaultScriptSubdir))
		if err != nil {
			err = os.MkdirAll(path.Join(resultsDir, storage.DefaultScriptSubdir), 0744)
			if err != nil {
				log.Log.Fatal(err)
			}
		}
	}

	// Set the output file where chrome stdout and stderr will be stored if we are gathering a JavaScript trace
	/* Commented out for the moment. TODO
	if st.JSTrace {
		midaBrowserOutfile, err := os.Create(path.Join(resultsDir, storage.DefaultBrowserLogFileName))
		if err != nil {
			log.Log.Fatal(err)
		}
		// This allows us to redirect the output from the browser to a file we choose.
		// TODO: Implement in new version of chromedp
		cxt = context.WithValue(cxt, "MIDA_Browser_Output_File", midaBrowserOutfile)
	}
	*/

	/* Leave out browser coverage for now -- engineering ongoing
	if st.BrowserCoverage {
		// Set environment variable for browser
		cxt = context.WithValue(cxt, "MIDA_LLVM_PROFILE_FILE", path.Join(resultsDir, storage.DefaultCoverageSubdir, "coverage-%4m.profraw"))

		// Create directory which will contain coverage files
		_, err = os.Stat(path.Join(resultsDir, storage.DefaultCoverageSubdir))
		if err != nil {
			err = os.MkdirAll(path.Join(resultsDir, storage.DefaultCoverageSubdir), 0744)
			if err != nil {
				log.Log.Fatal(err)
			}
		}

	}
	*/

	/* Leave out the network strace bit for now. We have pcap via docker
	if st.NetworkStrace {
		cxt = context.WithValue(cxt, "MIDA_STRACE_FILE", path.Join(resultsDir, storage.DefaultNetworkStraceFileName))
	}
	*/

	// Add these the port and the user data directory as arguments to the browser as we start it up
	// No other flags should be added here unless there is a good reason they can't be put in
	// the pipeline earlier.

	// Append browser options along with exe path
	var opts []chromedp.ExecAllocatorOption
	for _, flagString := range st.BrowserFlags {
		name, val, err := util.FormatFlag(flagString)
		if err != nil {
			log.Log.Error(err)
			continue
		}
		opts = append(opts, chromedp.Flag(name, val))
	}

	opts = append(opts, chromedp.Flag("user-data-dir", st.UserDataDirectory))
	opts = append(opts, chromedp.ExecPath(st.BrowserBinary))

	if st.JSTrace {
		midaBrowserOutfile, err := os.Create(path.Join(resultsDir, storage.DefaultBrowserLogFileName))
		if err != nil {
			log.Log.Fatal(err)
		}
		// This allows us to redirect the output from the browser to a file we choose.
		opts = append(opts, chromedp.CombinedOutput(midaBrowserOutfile))
	}

	// Spawn the browser
	allocContext, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	cxt, cancel := chromedp.NewContext(allocContext)
	defer cancel()

	// Event Demux - just receive the events and stick them in the applicable channels
	chromedp.ListenTarget(cxt, func(ev interface{}) {
		switch ev.(type) {
		// Page Domain Events
		case *page.EventLoadEventFired:
			loadEventFiredChan <- ev.(*page.EventLoadEventFired)
		case *page.EventDomContentEventFired:
			domContentEventFiredChan <- ev.(*page.EventDomContentEventFired)

		// General Network Domain Events
		case *network.EventRequestWillBeSent:
			requestWillBeSentChan <- ev.(*network.EventRequestWillBeSent)
		case *network.EventResponseReceived:
			responseReceivedChan <- ev.(*network.EventResponseReceived)
		case *network.EventLoadingFinished:
			loadingFinishedChan <- ev.(*network.EventLoadingFinished)

		// Websocket Network Domain Events
		case *network.EventWebSocketCreated:
			webSocketCreatedChan <- ev.(*network.EventWebSocketCreated)
		case *network.EventWebSocketFrameSent:
			webSocketFrameSentChan <- ev.(*network.EventWebSocketFrameSent)
		case *network.EventWebSocketFrameReceived:
			webSocketFrameReceivedChan <- ev.(*network.EventWebSocketFrameReceived)
		case *network.EventWebSocketFrameError:
			webSocketFrameErrorChan <- ev.(*network.EventWebSocketFrameError)
		case *network.EventWebSocketClosed:
			webSocketClosedChan <- ev.(*network.EventWebSocketClosed)
		case *network.EventWebSocketWillSendHandshakeRequest:
			webSocketWillSendHandshakeRequestChan <- ev.(*network.EventWebSocketWillSendHandshakeRequest)
		case *network.EventWebSocketHandshakeResponseReceived:
			webSocketHandshakeResponseReceivedChan <- ev.(*network.EventWebSocketHandshakeResponseReceived)

		// Debugger Domain Events
		case *debugger.EventScriptParsed:
			scriptParsedChan <- ev.(*debugger.EventScriptParsed)

		}
	})

	// Ensure the correct domains are enabled/disabled
	err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
		err = runtime.Disable().Do(cxt)
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

		return nil
	}))
	if err != nil {
		cancel()

		close(loadEventFiredChan)
		close(domContentEventFiredChan)
		close(requestWillBeSentChan)
		close(responseReceivedChan)
		close(loadingFinishedChan)
		close(webSocketCreatedChan)
		close(webSocketFrameSentChan)
		close(webSocketFrameReceivedChan)
		close(webSocketFrameErrorChan)
		close(webSocketClosedChan)
		close(webSocketWillSendHandshakeRequestChan)
		close(webSocketHandshakeResponseReceivedChan)
		close(scriptParsedChan)

		rawResultLock.Lock()
		rawResult.SanitizedTask.TaskFailed = true
		rawResult.SanitizedTask.FailureCode = err.Error()
		rawResultLock.Unlock()

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
			log.Log.Fatal(err)
		}
		rawResult.CrawlHostInfo.HostName = hostname
		rawResultLock.Unlock()

		return nil
	}))
	if err != nil {
		cancel()

		close(loadEventFiredChan)
		close(domContentEventFiredChan)
		close(requestWillBeSentChan)
		close(responseReceivedChan)
		close(loadingFinishedChan)
		close(webSocketCreatedChan)
		close(webSocketFrameSentChan)
		close(webSocketFrameReceivedChan)
		close(webSocketFrameErrorChan)
		close(webSocketClosedChan)
		close(webSocketWillSendHandshakeRequestChan)
		close(webSocketHandshakeResponseReceivedChan)
		close(scriptParsedChan)

		rawResultLock.Lock()
		rawResult.SanitizedTask.TaskFailed = true
		rawResult.SanitizedTask.FailureCode = err.Error()
		rawResultLock.Unlock()

		return rawResult, nil
	}

	/*
		rawResultLock.Lock()
		rawResult.Stats.Timing.BrowserOpen = time.Now()
		rawResultLock.Unlock()

		rawResultLock.Lock()
		rawResult.Stats.Timing.DevtoolsConnect = time.Now()
		rawResultLock.Unlock()

		// Get browser info from DevTools
		err = c.Run(cxt, chromedp.ActionFunc(func(ctxt context.Context, h cdp.Executor) error {
			protocolVersion, product, revision, userAgent, jsVersion, err := browser.GetVersion().Do(cxt, h)
			rawResultLock.Lock()
			rawResult.CrawlHostInfo.DevToolsVersion = protocolVersion
			rawResult.CrawlHostInfo.Browser = product
			rawResult.CrawlHostInfo.V8Version = jsVersion
			rawResult.CrawlHostInfo.BrowserVersion = revision
			rawResult.CrawlHostInfo.UserAgent = userAgent
			hostname, err := os.Hostname()
			if err != nil {
				log.Log.Fatal(err)
			}
			rawResult.CrawlHostInfo.HostName = hostname
			rawResultLock.Unlock()
			return err
		}))
		if err != nil {
			log.Log.Error(err)
			rawResultLock.Lock()
			rawResult.SanitizedTask.TaskFailed = true
			rawResult.SanitizedTask.FailureCode = err.Error()
			rawResultLock.Unlock()

			return rawResult, nil
		}
	*/

	// Event Handler : Page.loadEventFired
	go func() {
		for range loadEventFiredChan {
			rawResultLock.Lock()
			if rawResult.Stats.Timing.LoadEvent.IsZero() {
				rawResult.Stats.Timing.LoadEvent = time.Now()
			} else {
				log.Log.Warn("Duplicate load event")
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
		}
	}()

	// Event Handler: Page.domContentEventFired
	go func() {
		for range domContentEventFiredChan {
			rawResultLock.Lock()
			if rawResult.Stats.Timing.DOMContentEvent.IsZero() {
				rawResult.Stats.Timing.DOMContentEvent = time.Now()
			} else {
				log.Log.Warn("Duplicate DOMContentLoaded event")
			}
			rawResultLock.Unlock()
		}
	}()

	// Event Handler: Network.requestWillBeSent
	go func() {
		for data := range requestWillBeSentChan {
			rawResultLock.Lock()
			rawResult.Requests[data.RequestID.String()] = append(rawResult.Requests[data.RequestID.String()], *data)
			rawResultLock.Unlock()
		}
	}()

	// Event Handler: Network.responseReceived
	go func() {
		for data := range responseReceivedChan {
			rawResultLock.Lock()
			rawResult.Responses[data.RequestID.String()] = append(rawResult.Responses[data.RequestID.String()], *data)
			rawResultLock.Unlock()
		}
	}()

	// Event Handler: Network.loadingFinished
	go func() {
		for data := range loadingFinishedChan {
			if st.AllResources {
				rawResultLock.Lock()
				if _, ok := rawResult.Requests[data.RequestID.String()]; !ok {
					log.Log.Debug("Will not get response body for unknown RequestID")
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
					log.Log.Debug("Failed to get response Body for known resource: ", data.RequestID)
				} else {
					err = ioutil.WriteFile(path.Join(resultsDir, storage.DefaultFileSubdir, data.RequestID.String()), respBody, os.ModePerm)
					if err != nil {
						log.Log.Fatal(err)
					}
				}
			}
		}
	}()

	// Event Handler: Network.webSocketCreated
	go func() {
		for data := range webSocketCreatedChan {
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
		}
	}()

	// Event Handler: Network.webSocketFrameSent
	go func() {
		for data := range webSocketFrameSentChan {
			rawResultLock.Lock()
			if _, ok := rawResult.WebsocketData[data.RequestID.String()]; ok {
				// Create our new websocket connection
				rawResult.WebsocketData[data.RequestID.String()].FramesSent = append(
					rawResult.WebsocketData[data.RequestID.String()].FramesSent, data)
			}
			// Otherwise, we ignore a frame for a connection we don't know about
			rawResultLock.Unlock()
		}
	}()

	// Event Handler: Network.webSocketFrameReceived
	go func() {
		for data := range webSocketFrameReceivedChan {
			rawResultLock.Lock()
			if _, ok := rawResult.WebsocketData[data.RequestID.String()]; ok {
				// Create our new websocket connection
				rawResult.WebsocketData[data.RequestID.String()].FramesReceived = append(
					rawResult.WebsocketData[data.RequestID.String()].FramesReceived, data)
			}
			// Otherwise, we ignore a frame for a connection we don't know about
			rawResultLock.Unlock()
		}
	}()

	// Event Handler: Network.webSocketFrameError
	go func() {
		for data := range webSocketFrameErrorChan {
			rawResultLock.Lock()
			if _, ok := rawResult.WebsocketData[data.RequestID.String()]; ok {
				// Create our new websocket connection
				rawResult.WebsocketData[data.RequestID.String()].FrameErrors = append(
					rawResult.WebsocketData[data.RequestID.String()].FrameErrors, data)
			}
			// Otherwise, we ignore a frame for a connection we don't know about
			rawResultLock.Unlock()
		}
	}()

	// Event Handler: Network.webSocketClosed
	go func() {
		for data := range webSocketClosedChan {
			rawResultLock.Lock()
			if _, ok := rawResult.WebsocketData[data.RequestID.String()]; ok {
				// Create our new websocket connection
				rawResult.WebsocketData[data.RequestID.String()].TSClose = data.Timestamp.Time().String()
			}
			// Otherwise, we ignore a frame for a connection we don't know about
			rawResultLock.Unlock()
		}
	}()

	// Event Handler: Network.websocketWillSendHandshakeRequest
	go func() {
		for data := range webSocketWillSendHandshakeRequestChan {
			rawResultLock.Lock()
			if _, ok := rawResult.WebsocketData[data.RequestID.String()]; ok {
				// Create our new websocket connection
				rawResult.WebsocketData[data.RequestID.String()].HandshakeRequests = append(
					rawResult.WebsocketData[data.RequestID.String()].HandshakeRequests, data)
			}
			// Otherwise, we ignore a frame for a connection we don't know about
			rawResultLock.Unlock()
		}
	}()

	// Event Handler: Network.websocketHandshakeResponseReceived
	go func() {
		for data := range webSocketHandshakeResponseReceivedChan {
			rawResultLock.Lock()
			if _, ok := rawResult.WebsocketData[data.RequestID.String()]; ok {
				// Create our new websocket connection
				rawResult.WebsocketData[data.RequestID.String()].HandshakeResponses = append(
					rawResult.WebsocketData[data.RequestID.String()].HandshakeResponses, data)
			}
			// Otherwise, we ignore a frame for a connection we don't know about
			rawResultLock.Unlock()
		}
	}()

	// Event Handler: Debugger.scriptParsed
	go func() {
		for data := range scriptParsedChan {
			rawResultLock.Lock()
			rawResult.Scripts[data.ScriptID.String()] = *data
			rawResultLock.Unlock()
			if st.AllScripts {
				var source string
				err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
					source, err = debugger.GetScriptSource(data.ScriptID).Do(cxt)
					if err != nil {
						return err
					}

					return nil
				}))
				if err != nil && err.Error() != "context canceled" {
					log.Log.Error("Failed to get script source")
					log.Log.Error(err)
				} else {
					err = ioutil.WriteFile(path.Join(resultsDir, storage.DefaultScriptSubdir, data.ScriptID.String()), []byte(source), os.ModePerm)
					if err != nil {
						log.Log.Fatal(err)
					}
				}
			}
		}
	}()

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
			log.Log.Error("Nav error: ", err)
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
			log.Log.Error("Failed to navigate to site: ", err)
		}

		cancel()

		rawResultLock.Lock()
		rawResult.Stats.Timing.BrowserClose = time.Now()
		rawResultLock.Unlock()

		close(loadEventFiredChan)
		close(domContentEventFiredChan)
		close(requestWillBeSentChan)
		close(responseReceivedChan)
		close(loadingFinishedChan)
		close(webSocketCreatedChan)
		close(webSocketFrameSentChan)
		close(webSocketFrameReceivedChan)
		close(webSocketFrameErrorChan)
		close(webSocketClosedChan)
		close(webSocketWillSendHandshakeRequestChan)
		close(webSocketHandshakeResponseReceivedChan)
		close(scriptParsedChan)

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
				log.Log.Error("Unexpectedly received load event through channel on TimeoutOnly crawl")
			}
		case <-postCrawlActionsChan:
			// We are free to begin post crawl data gathering which requires the browser
			// Examples: Screenshot, DOM snapshot, code coverage, etc.
			// These actions may or may not finish -- We still have to observe the timeout
			/*
				if st.ResourceTree {
					go func() {

						var tree *page.FrameTree
						err = c.Run(cxt, chromedp.ActionFunc(func(ctxt context.Context, h cdp.Executor) error {
							ctxt, cancel := context.WithTimeout(ctxt, 2*time.Second)
							defer cancel()
							tree, err = page.GetFrameTree().Do(ctxt, h)
							return err
						}))
						if err != nil {
							log.Log.Error(err)
						}
						rawResultLock.Lock()
						rawResult.FrameTree = tree
						rawResultLock.Unlock()

					}()
				}
			*/
			<-timeoutChan
		case <-timeoutChan:

		}
	}

	cancel()

	rawResultLock.Lock()
	rawResult.Stats.Timing.BrowserClose = time.Now()
	rawResultLock.Unlock()

	close(loadEventFiredChan)
	close(domContentEventFiredChan)
	close(requestWillBeSentChan)
	close(responseReceivedChan)
	close(loadingFinishedChan)
	close(webSocketCreatedChan)
	close(webSocketFrameSentChan)
	close(webSocketFrameReceivedChan)
	close(webSocketFrameErrorChan)
	close(webSocketClosedChan)
	close(webSocketWillSendHandshakeRequestChan)
	close(webSocketHandshakeResponseReceivedChan)
	close(scriptParsedChan)

	return rawResult, nil

}
