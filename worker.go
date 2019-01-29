package main

import (
	"context"
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
	"github.com/phayes/freeport"
	log "github.com/sirupsen/logrus"
	"github.com/teamnsrg/chromedp"
	"github.com/teamnsrg/chromedp/client"
	"github.com/teamnsrg/chromedp/runner"
	"math/rand"
	"os"
	"time"
)

func ProcessSanitizedTask(st SanitizedMIDATask) {

	numScriptsParsed := 0
	numScriptsSourceCode := 0
	numResources := 0

	// Create our context and browser
	cxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Generates a random identifier which will be used to name the user data directory, if not given
	// Set the length of this identifier with DefaultIdentifierLength in defaults.go
	randomIdentifier := GenRandomIdentifier()
	log.Info(randomIdentifier)

	// Set the output file where chrome stdout and stderr will be stored if we are gathering a JavaScript trace
	if st.JSTrace {
		midaBrowserOutfile, err := os.Create("/Users/pmurley/Desktop/chromelog.log")
		if err != nil {
			log.Fatal(err)
		}
		cxt = context.WithValue(cxt, "MIDA_Browser_Output_File", midaBrowserOutfile)
	}

	port, err := freeport.GetFreePort()
	if err != nil {
		log.Fatal(err)
	}

	port = 9556

	runnerOpts := append(st.BrowserFlags, runner.ExecPath(st.BrowserBinary),
		runner.Flag("remote-debugging-port", port))

	r, err := runner.New(runnerOpts...)
	if err != nil {
		log.Fatal(err)
	}
	err = r.Start(cxt)
	if err != nil {
		log.Fatal(err)
	}

	c, err := chromedp.New(cxt, chromedp.WithClient(cxt, client.New(client.URL("http://localhost:9555/json"))))
	//c, err := chromedp.New(cxt, chromedp.WithRunner(r))
	if err != nil {
		log.Fatal(err)
	}

	// Set up required listeners and timers
	err = c.Run(cxt, chromedp.CallbackFunc("Network.requestWillBeSent", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*network.EventRequestWillBeSent)
		log.Debug(data.Request.URL)
		numResources += 1
	}))
	if err != nil {
		log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Network.loadingFailed", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*network.EventLoadingFailed)
		log.Info(data)
		numResources += 1
	}))
	if err != nil {
		log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Debugger.scriptParsed", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*debugger.EventScriptParsed)
		numScriptsParsed += 1
		_, _ = debugger.GetScriptSource(data.ScriptID).Do(cxt, handler)
		numScriptsSourceCode += 1

	}))
	if err != nil {
		log.Fatal(err)
	}

	// Ensure that events have stopped and shut down the browser
	// Navigate to specified URL and wait for termination condition

	err = c.Run(cxt, chromedp.Navigate(st.Url))
	if err != nil {
		log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.Sleep(time.Duration(st.Timeout)*time.Second))
	if err != nil {
		log.Fatal(err)
	}

	err = c.Shutdown(cxt)
	if err != nil {
		log.Fatal(err)
	}

	err = r.Shutdown(cxt)
	if err != nil {
		log.Fatal(err)
	}

	// Send results through channel to results processor

	// Return true if processing was successful or false if failed

	log.Info("Scripts parsed/downloaded:", numScriptsParsed, numScriptsSourceCode)
	log.Info("Resources: ", numResources)
	log.Info(st)

	log.Info(cxt)

}

func GenRandomIdentifier() string {
	// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go

	b := ""
	for i := 0; i < DefaultIdentifierLength; i++ {
		b = b + string(Letters[rand.Intn(len(Letters))])
	}
	return b
}
