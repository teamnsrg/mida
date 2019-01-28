package main

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
	"github.com/teamnsrg/chromedp"
	"github.com/teamnsrg/chromedp/runner"
	"math/rand"
	"os"
	"time"
)

func ProcessSanitizedTask(t MIDA_Task) {

	// Create our context and browser
	cxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Generates a random identifier which will be used to name the user data directory, if not given
	// Set the length of this identifier with DEFAULT_IDENTIFIER_LENGTH in defaults.go

	// Set the output file where chrome stdout and stderr will be stored if we are gathering a JavaScript trace
	if t.JSTrace {
		midaBrowserOutfile, err := os.Create("/Users/pmurley/Desktop/chromelog.log")
		if err != nil {
			log.Fatal(err)
		}
		cxt = context.WithValue(cxt, "MIDA_Browser_Output_File", midaBrowserOutfile)
	}

	c, err := chromedp.New(cxt, chromedp.WithRunnerOptions(
		runner.Flag("remote-debugging-port", 8088),
		runner.Flag("headless", true),
		runner.Flag("disable-gpu", true)))
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

	err = c.Run(cxt, chromedp.CallbackFunc("Debugger.scriptParsed", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*debugger.EventScriptParsed)
		result, _ := debugger.GetScriptSource(data.ScriptID).Do(cxt, handler)
		log.Debug(result)

	}))
	if err != nil {
		log.Fatal(err)
	}

	// Ensure that events have stopped and shut down the browser
	// Navigate to specified URL and wait for termination condition
	err = c.Run(cxt, chromedp.Navigate("https://murley.io"))
	if err != nil {
		log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.Sleep(10*time.Second))
	if err != nil {
		log.Fatal(err)
	}

	err = c.Shutdown(cxt)
	if err != nil {
		log.Fatal(err)
	}

	// Send results through channel to results processor

	// Return true if processing was successful or false if failed

	log.Info(t)

	log.Info(cxt)

}

func GenRandomIdentifier() string {
	// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
	var letters = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, DEFAULT_IDENTIFER_LENGTH)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
