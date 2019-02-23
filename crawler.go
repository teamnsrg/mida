package main

import (
	"context"
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
	"github.com/phayes/freeport"
	"github.com/spf13/viper"
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
		Requests:  make(map[string][]network.EventRequestWillBeSent),
		Responses: make(map[string][]network.EventResponseReceived),
		Scripts:   make(map[string]debugger.EventScriptParsed),
	}
	var rawResultLock sync.Mutex // Should be used any time this object is updated

	var requestMapLock sync.Mutex
	var responseMapLock sync.Mutex
	var scriptsMapLock sync.Mutex

	rawResultLock.Lock()
	rawResult.Stats.Timing.BeginCrawl = time.Now()
	rawResult.SanitizedTask = st
	rawResultLock.Unlock()

	// Create our context and browser
	cxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create our user data directory, if it does not yet exist
	if st.UserDataDirectory == "" {
		st.UserDataDirectory = path.Join(viper.GetString("TempDir"), st.RandomIdentifier)
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

	if st.AllFiles {
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
		Log.Fatal(err)
	}
	rawResultLock.Lock()
	rawResult.Stats.Timing.DevtoolsConnect = time.Now()
	rawResultLock.Unlock()

	// Set up required listeners and timers
	err = c.Run(cxt, chromedp.CallbackFunc("Page.loadEventFired", func(param interface{}, handler *chromedp.TargetHandler) {
		rawResultLock.Lock()
		if rawResult.Stats.Timing.LoadEvent.IsZero() {
			rawResult.Stats.Timing.LoadEvent = time.Now()
			Log.Info("Load Event")
		} else {
			Log.Warn("Duplicate load event")
		}
		rawResultLock.Unlock()
	}))
	if err != nil {
		Log.Fatal(err)
	}

	// Set up required listeners and timers
	err = c.Run(cxt, chromedp.CallbackFunc("Page.domContentEventFired", func(param interface{}, handler *chromedp.TargetHandler) {
		rawResultLock.Lock()
		if rawResult.Stats.Timing.DOMContentEvent.IsZero() {
			rawResult.Stats.Timing.DOMContentEvent = time.Now()
			Log.Info("DOMContentLoaded Event")
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
		// Log.Info(data.Frame.URL, " : ", data.Frame.ID," : ", data.Frame.Name," : ", data.Frame.State.String())
	}))
	if err != nil {
		Log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Network.requestWillBeSent", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*network.EventRequestWillBeSent)
		requestMapLock.Lock()
		rawResult.Requests[data.RequestID.String()] = append(rawResult.Requests[data.RequestID.String()], *data)
		requestMapLock.Unlock()
	}))
	if err != nil {
		Log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Network.responseReceived", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*network.EventResponseReceived)
		responseMapLock.Lock()
		rawResult.Responses[data.RequestID.String()] = append(rawResult.Responses[data.RequestID.String()], *data)
		responseMapLock.Unlock()
	}))
	if err != nil {
		Log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Network.loadingFinished", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*network.EventLoadingFinished)
		if st.AllFiles {
			respBody, err := network.GetResponseBody(data.RequestID).Do(cxt, handler)
			if err != nil {
				// The browser was unable to provide the content of this particular resource
				// TODO: Count how many times this happens, figure out what types of resources it is happening for
				Log.Warn("Failed to get response Body for resource: ", data.RequestID)
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
		Log.Info("Loading Failed: ", data.Type, " : ", data.BlockedReason, " : ", data.ErrorText)
	}))
	if err != nil {
		Log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Debugger.scriptParsed", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*debugger.EventScriptParsed)
		scriptsMapLock.Lock()
		rawResult.Scripts[data.ScriptID.String()] = *data
		scriptsMapLock.Unlock()
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
		} else {
			Log.Info("Not enabled")
		}

	}))
	if err != nil {
		Log.Fatal(err)
	}

	// Navigate to specified URL, timing out if no connection to the site
	// can be made
	navChan := make(chan error, 1)
	go func() {
		navChan <- c.Run(cxt, chromedp.Navigate(st.Url))
	}()
	select {
	case err = <-navChan:
		Log.Debug("Connection Established")
		rawResult.Stats.Timing.ConnectionEstablished = time.Now()
	case <-time.After(DefaultNavTimeout * time.Second):
		Log.Warn("Navigation timeout")
		// TODO: Handle navigation errors, build a corresponding result, etc.
	}
	if err != nil {
		if err.Error() == "net::ERR_NAME_NOT_RESOLVED" {
			Log.Warn("DNS did not resolve")
		} else if err.Error() == "net::ERR_INVALID_HTTP_RESPONSE" {
			Log.Warn("Received invalid HTTP response")
		} else {
			Log.Warn("Unknown navigation error: ", err.Error())
		}
	}

	// Wait for specified termination condition. This logic is dependent on
	// the completion condition specified in the task.
	err = c.Run(cxt, chromedp.Sleep(time.Duration(st.Timeout)*time.Second))
	if err != nil {
		Log.Fatal(err)
	}

	err = c.Shutdown(cxt)
	if err != nil {
		Log.Fatal("Client Shutdown:", err)
	}
	rawResult.Stats.Timing.BrowserClose = time.Now()

	rawResult.Stats.Timing.EndCrawl = time.Now()

	return rawResult, nil

}
