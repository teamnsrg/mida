package main

import (
	"context"
	"errors"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/phayes/freeport"
	"github.com/prometheus/common/log"
	"github.com/teamnsrg/chromedp"
	"github.com/teamnsrg/chromedp/runner"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"
)

func CrawlerInstance(sanitizedTaskChan <-chan SanitizedMIDATask, rawResultChan chan<- RawMIDAResult, retryChan <-chan SanitizedMIDATask, crawlerWG *sync.WaitGroup) {

	for sanitizedTaskChan != nil {
		select {
		case st, ok := <-retryChan:
			if !ok {
				retryChan = nil
			} else {
				rawResult, err := ProcessSanitizedTask(st)
				if err != nil {
					Log.Fatal(err)
				}
				// Put our raw crawl result into the Raw Result Channel, where it will be validated and post-processed
				rawResultChan <- rawResult
			}
		case st, ok := <-sanitizedTaskChan:
			if !ok {
				sanitizedTaskChan = nil
			} else {
				rawResult, err := ProcessSanitizedTask(st)
				if err != nil {
					Log.Fatal(err)
				}
				// Put our raw crawl result into the Raw Result Channel, where it will be validated and post-processed
				rawResultChan <- rawResult
			}
		}
	}

	// RawMIDAResult channel is closed once all crawlers have exited, where they are first created
	crawlerWG.Done()

	return
}

func ProcessSanitizedTask(st SanitizedMIDATask) (RawMIDAResult, error) {

	rawResult := RawMIDAResult{
		Requests:      make(map[string][]network.EventRequestWillBeSent),
		Responses:     make(map[string][]network.EventResponseReceived),
		Scripts:       make(map[string]debugger.EventScriptParsed),
		SanitizedTask: st,
	}
	var rawResultLock sync.Mutex // Should be used any time this object is updated

	navChan := make(chan error, 1)
	timeoutChan := time.After(time.Duration(st.Timeout) * time.Second)
	loadEventChan := make(chan bool, 5)
	postCrawlActionsChan := make(chan bool, 1)

	rawResultLock.Lock()
	rawResult.Stats.Timing.BeginCrawl = time.Now()
	rawResult.SanitizedTask = st
	rawResultLock.Unlock()

	// Create our context and browser
	cxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create our user data directory, if it does not yet exist
	if st.UserDataDirectory == "" {
		st.UserDataDirectory = path.Join(TempDir, st.RandomIdentifier)
	}

	_, err := os.Stat(st.UserDataDirectory)
	if err != nil {
		err = os.MkdirAll(st.UserDataDirectory, 0744)
		if err != nil {
			Log.Fatal(err)
		}
	}

	// Create our temporary results directory within the user data directory
	resultsDir := path.Join(st.UserDataDirectory, st.RandomIdentifier)
	_, err = os.Stat(resultsDir)
	if err != nil {
		err = os.MkdirAll(resultsDir, 0755)
		if err != nil {
			Log.Fatal("Error creating results directory")
		}
	} else {
		Log.Fatal("Results directory already existed within user data directory")
	}

	if st.AllResources {
		// Create a subdirectory where we will store all the files
		_, err = os.Stat(path.Join(resultsDir, DefaultFileSubdir))
		if err != nil {
			err = os.MkdirAll(path.Join(resultsDir, DefaultFileSubdir), 0744)
			if err != nil {
				Log.Fatal(err)
			}
		}
	}

	if st.AllScripts {
		// Create a subdirectory where we will store all scripts parsed by browser
		_, err = os.Stat(path.Join(resultsDir, DefaultScriptSubdir))
		if err != nil {
			err = os.MkdirAll(path.Join(resultsDir, DefaultScriptSubdir), 0744)
			if err != nil {
				Log.Fatal(err)
			}
		}
	}

	// Set the output file where chrome stdout and stderr will be stored if we are gathering a JavaScript trace
	if st.JSTrace {
		midaBrowserOutfile, err := os.Create(path.Join(resultsDir, DefaultBrowserLogFileName))
		if err != nil {
			Log.Fatal(err)
		}
		// This allows us to redirect the output from the browser to a file we choose.
		// This happens in github.com/teamnsrg/chromedp/runner.go
		cxt = context.WithValue(cxt, "MIDA_Browser_Output_File", midaBrowserOutfile)
	}

	// Remote Debugging Protocol (DevTools) will listen on this port
	port, err := freeport.GetFreePort()
	if err != nil {
		Log.Fatal(err)
	}

	// Add these the port and the user data directory as arguments to the browser as we start it up
	// No other flags should be added here unless there is a good reason they can't be put in
	// the pipeline earlier.
	runnerOpts := append(st.BrowserFlags, runner.ExecPath(st.BrowserBinary),
		runner.Flag("remote-debugging-port", port),
		runner.Flag("user-data-dir", st.UserDataDirectory),
	)

	r, err := runner.New(runnerOpts...)
	if err != nil {
		Log.Fatal(err)
	}
	err = r.Start(cxt)
	if err != nil {
		Log.Fatal(err)
	}
	rawResultLock.Lock()
	rawResult.Stats.Timing.BrowserOpen = time.Now()
	rawResultLock.Unlock()

	c, err := chromedp.New(cxt, chromedp.WithRunner(r))
	if err != nil {
		Log.Error(err)
		log.Error("If running without a display, preface command with \"xvfb-run\"")
		Log.Fatal("Exiting")
	}
	rawResultLock.Lock()
	rawResult.Stats.Timing.DevtoolsConnect = time.Now()
	rawResultLock.Unlock()

	// Set up required listeners and timers
	err = c.Run(cxt, chromedp.CallbackFunc("Page.loadEventFired", func(param interface{}, handler *chromedp.TargetHandler) {
		rawResultLock.Lock()
		if rawResult.Stats.Timing.LoadEvent.IsZero() {
			rawResult.Stats.Timing.LoadEvent = time.Now()
		} else {
			Log.Warn("Duplicate load event")
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
	}))
	if err != nil {
		Log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Page.domContentEventFired", func(param interface{}, handler *chromedp.TargetHandler) {
		rawResultLock.Lock()
		if rawResult.Stats.Timing.DOMContentEvent.IsZero() {
			rawResult.Stats.Timing.DOMContentEvent = time.Now()
		} else {
			Log.Warn("Duplicate DOMContentLoaded event")
		}
		rawResultLock.Unlock()
	}))
	if err != nil {
		Log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Page.frameNavigated", func(param interface{}, handler *chromedp.TargetHandler) {
		// data := param.(*page.EventFrameNavigated)
		// Log.Warn("FrameNavigated: ", data.Frame.URL, " : ", data.Frame.ID," : ", data.Frame.Name," : ", data.Frame.State.String())
	}))
	if err != nil {
		Log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Page.lifecycleEvent", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*page.EventLifecycleEvent)
		Log.Warn(data.Name, "    ", data.Timestamp.Time().String(), "    ", data.FrameID.String())
	}))
	if err != nil {
		Log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Network.requestWillBeSent", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*network.EventRequestWillBeSent)
		rawResultLock.Lock()
		rawResult.Requests[data.RequestID.String()] = append(rawResult.Requests[data.RequestID.String()], *data)
		rawResultLock.Unlock()
	}))
	if err != nil {
		Log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Network.responseReceived", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*network.EventResponseReceived)
		rawResultLock.Lock()
		rawResult.Responses[data.RequestID.String()] = append(rawResult.Responses[data.RequestID.String()], *data)
		rawResultLock.Unlock()
	}))
	if err != nil {
		Log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Network.loadingFinished", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*network.EventLoadingFinished)
		if st.AllResources {
			rawResultLock.Lock()
			if _, ok := rawResult.Requests[data.RequestID.String()]; !ok {
				Log.Debug("Will not get response body for unknown RequestID")
				rawResultLock.Unlock()
				return
			}
			rawResultLock.Unlock()
			respBody, err := network.GetResponseBody(data.RequestID).Do(cxt, handler)
			if err != nil {
				// The browser was unable to provide the content of this particular resource
				// TODO: Count how many times this happens, figure out what types of resources it is happening for
				Log.Warn("Failed to get response Body for known resource: ", data.RequestID)
			} else {
				err = ioutil.WriteFile(path.Join(resultsDir, DefaultFileSubdir, data.RequestID.String()), respBody, os.ModePerm)
				if err != nil {
					Log.Fatal(err)
				}
			}
		}

	}))
	if err != nil {
		Log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Network.loadingFailed", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*network.EventLoadingFailed)
		// TODO: Count how many times this happens, figure out what types of resources it is happening for
		Log.Debug("Loading Failed: ", data.Type, " : ", data.BlockedReason, " : ", data.ErrorText)
	}))
	if err != nil {
		Log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Debugger.scriptParsed", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*debugger.EventScriptParsed)
		rawResultLock.Lock()
		rawResult.Scripts[data.ScriptID.String()] = *data
		rawResultLock.Unlock()
		if st.AllScripts {
			source, err := debugger.GetScriptSource(data.ScriptID).Do(cxt, handler)
			if err != nil && err.Error() != "context canceled" {
				Log.Error("Failed to get script source")
				Log.Error(err)
			} else {
				err = ioutil.WriteFile(path.Join(resultsDir, DefaultScriptSubdir, data.ScriptID.String()), []byte(source), os.ModePerm)
				if err != nil {
					Log.Fatal(err)
				}
			}
		}
	}))
	if err != nil {
		Log.Fatal(err)
	}

	// Below is the MIDA navigation logic. Since MIDA offers several different
	// termination conditions (Terminate on timout, terminate on load event, etc.),
	// this logic gets a little complex.
	go func() {
		navChan <- c.Run(cxt, chromedp.Navigate(st.Url))
	}()
	select {
	case err = <-navChan:
		rawResult.Stats.Timing.ConnectionEstablished = time.Now()
	case <-time.After(DefaultNavTimeout * time.Second):
		// This usually happens because we successfully resolved DNS,
		// but we could not connect to the server
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

		err = c.Shutdown(cxt)
		if err != nil {
			Log.Fatal("Browser Shutdown Failed: ", err)
		}

		rawResultLock.Lock()
		rawResult.Stats.Timing.BrowserClose = time.Now()
		rawResult.Stats.Timing.EndCrawl = time.Now()
		rawResultLock.Unlock()

		return rawResult, nil

	} else {
		// We successfully connected to the site. At this point, we WILL gather results.
		// Wait for our termination condition.
		select {
		// This will only arrive if we are using a completion condition that requires load events
		case <-loadEventChan:
			var ccond CompletionCondition
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
				Log.Error("Unexpectedly received load event through channel on TimeoutOnly crawl")
			}
		case <-postCrawlActionsChan:
			// We are free to begin post crawl data gathering which requires the browser
			// Examples: Screenshot, DOM snapshot, code coverage, etc.
			// These actions may or may not finish -- We still have to observe the timeout
			go func() {
				var tree *page.FrameTree
				err = c.Run(cxt, chromedp.ActionFunc(func(ctxt context.Context, h cdp.Executor) error {
					ctxt, cancel := context.WithTimeout(ctxt, 2*time.Second)
					defer cancel()
					tree, err = page.GetFrameTree().Do(ctxt, h)
					return err
				}))
				if err != nil {
					Log.Error(err)
				}
				rawResultLock.Lock()
				rawResult.FrameTree = tree
				rawResultLock.Unlock()

			}()
			<-timeoutChan
		case <-timeoutChan:
			// Overall timeout, shut down now, no post-crawl data gathering for you
			Log.Info("Timeout (no post crawl activities")
		}
	}

	// Clean up
	err = c.Shutdown(cxt)
	if err != nil {
		Log.Fatal("Browser Shutdown Failed: ", err)
	}

	rawResultLock.Lock()
	rawResult.Stats.Timing.BrowserClose = time.Now()
	rawResult.Stats.Timing.EndCrawl = time.Now()
	rawResultLock.Unlock()

	return rawResult, nil

}
