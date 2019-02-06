package main

import (
	"context"
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
	"github.com/phayes/freeport"
	log "github.com/sirupsen/logrus"
	"github.com/teamnsrg/chromedp"
	"io/ioutil"
	"path"
	"sync"

	//"github.com/teamnsrg/chromedp/client"
	"github.com/teamnsrg/chromedp/runner"
	"math/rand"
	"os"
	"time"
)

func CrawlerInstance(tc <-chan SanitizedMIDATask, rc chan<- RawMIDAResult, mConfig MIDAConfig, crawlerWG *sync.WaitGroup) {
	for st := range tc {
		rawResult, err := ProcessSanitizedTask(st)
		if err != nil {
			log.Fatal(err)
		}
		// Put our raw crawl result into the Raw Result Channel, where it will be validated and post-processed
		rc <- rawResult
	}

	// RawMIDAResult channel is closed once all crawlers have exited, where they are first created
	crawlerWG.Done()

	return
}

func ProcessSanitizedTask(st SanitizedMIDATask) (RawMIDAResult, error) {

	rawResult := RawMIDAResult{}
	rawResult.stats.StartTime = time.Now()

	// Create our context and browser
	cxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Generates a random identifier which will be used to name the user data directory, if not given
	// Set the length of this identifier with DefaultIdentifierLength in default.go
	randomIdentifier := GenRandomIdentifier()

	// Create our user data directory, if it does not yet exist
	if st.UserDataDirectory == "" {
		st.UserDataDirectory = path.Join(TemporaryDirectory, randomIdentifier)
	}

	_, err := os.Stat(st.UserDataDirectory)
	if err != nil {
		err = os.MkdirAll(st.UserDataDirectory, 0744)
		if err != nil {
			log.Fatal(err)
		}
	}

	if st.AllFiles {
		// Create a subdirectory where we will store all the files
		_, err = os.Stat(path.Join(st.UserDataDirectory, DefaultFileSubdir))
		if err != nil {
			err = os.MkdirAll(path.Join(st.UserDataDirectory, DefaultFileSubdir), 0744)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	// Set the output file where chrome stdout and stderr will be stored if we are gathering a JavaScript trace
	if st.JSTrace {
		midaBrowserOutfile, err := os.Create(path.Join(st.UserDataDirectory, DefaultLogFileName))
		if err != nil {
			log.Fatal(err)
		}
		// This allows us to redirect the output from the browser to a file we choose.
		// This happens in github.com/teamnsrg/chromedp/runner.go
		cxt = context.WithValue(cxt, "MIDA_Browser_Output_File", midaBrowserOutfile)
	}

	// Remote Debugging Protocol (DevTools) will listen on this port
	port, err := freeport.GetFreePort()
	if err != nil {
		log.Fatal(err)
	}

	// Add these the port and the user data directory as arguments to the browser as we start it up
	runnerOpts := append(st.BrowserFlags, runner.ExecPath(st.BrowserBinary),
		runner.Flag("remote-debugging-port", port),
		runner.Flag("user-data-dir", st.UserDataDirectory),
	)

	r, err := runner.New(runnerOpts...)
	if err != nil {
		log.Fatal(err)
	}
	err = r.Start(cxt)
	if err != nil {
		log.Fatal(err)
	}

	//c, err := chromedp.New(cxt, chromedp.WithClient(cxt, client.New(client.URL("http://localhost:9555/json"))))
	c, err := chromedp.New(cxt, chromedp.WithRunner(r))
	if err != nil {
		log.Fatal(err)
	}

	// Set up required listeners and timers
	err = c.Run(cxt, chromedp.CallbackFunc("Network.requestWillBeSent", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*network.EventRequestWillBeSent)
		log.Debug(data.Request.URL)
	}))
	if err != nil {
		log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Network.loadingFinished", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*network.EventLoadingFinished)
		respBody, err := network.GetResponseBody(data.RequestID).Do(cxt, handler)
		if err != nil {
			// TODO: Count how many times this happens, figure out what types of resources it is happening for
		} else {
			err = ioutil.WriteFile(path.Join(st.UserDataDirectory, DefaultFileSubdir, data.RequestID.String()), respBody, os.ModePerm)
			if err != nil {
				log.Fatal(err)
			}
		}

	}))
	if err != nil {
		log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Network.loadingFailed", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*network.EventLoadingFailed)
		// TODO: Count how many times this happens, figure out what types of resources it is happening for
		log.Debug(data.BlockedReason)
	}))
	if err != nil {
		log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Debugger.scriptParsed", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*debugger.EventScriptParsed)
		_, _ = debugger.GetScriptSource(data.ScriptID).Do(cxt, handler)

	}))
	if err != nil {
		log.Fatal(err)
	}

	// Navigate to specified URL and wait for termination condition
	err = c.Run(cxt, chromedp.Navigate(st.Url))
	if err != nil {
		log.Fatal(err)
	}

	// Ensure that events have stopped and shut down the browser
	err = c.Run(cxt, chromedp.Sleep(time.Duration(st.Timeout)*time.Second))
	if err != nil {
		log.Fatal(err)
	}

	err = c.Shutdown(cxt)
	if err != nil {
		log.Fatal("Client Shutdown:", err)
	}

	// Record how long the browser was open
	rawResult.stats.TimeAfterBrowserClose = time.Now()

	return rawResult, nil

}

func GenRandomIdentifier() string {
	// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
	b := ""
	rand.Seed(time.Now().UTC().UnixNano())
	for i := 0; i < DefaultIdentifierLength; i++ {
		b = b + string(AlphaNumChars[rand.Intn(len(AlphaNumChars))])
	}
	return b
}
