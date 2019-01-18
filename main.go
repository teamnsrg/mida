package main

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
	"github.com/teamnsrg/chromedp/runner"
	"github.com/teamnsrg/chromedp"
	"time"
    "os"
)

func main() {

	cxt, cancel := context.WithCancel(context.Background())
	defer cancel()

    // Set the output file where chrome stdout and stderr will be stored
    mida_browser_outfile, err := os.Create("/home/pmurley/mida_browser_output.log")
    cxt = context.WithValue(cxt, "MIDA_Browser_Output_File", mida_browser_outfile)


	c, err := chromedp.New(cxt, chromedp.WithRunnerOptions(
		runner.Flag("remote-debugging-port", 8088),
		runner.Flag("headless", true),
		runner.Flag("disable-gpu", true),
        runner.ExecPath("/home/pmurley/build_4/chrome")))
	if err != nil {
		log.Fatal(err)
	}

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
}
