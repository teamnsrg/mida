package main

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp/runner"
	"github.com/teamnsrg/chromedp"
	"time"
)

func main() {

	cxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, err := chromedp.New(cxt, chromedp.WithRunnerOptions(
		runner.Flag("remote-debugging-port", 8088)))
	if err != nil {
		log.Fatal(err)
	}

	err = c.Run(cxt, chromedp.CallbackFunc("Network.requestWillBeSent", func(param interface{}, handler *chromedp.TargetHandler) {
		data := param.(*network.EventRequestWillBeSent)
		log.Info(data.Request.URL)
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

	err = c.Run(cxt, chromedp.Navigate("https://www.cnn.com"))
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
