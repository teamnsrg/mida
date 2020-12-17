package browser

import (
	"context"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/sirupsen/logrus"
	"github.com/teamnsrg/chromedp"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"io/ioutil"
	"math/rand"
	"path"
	"strconv"
	"sync"
	"time"
)

// postLoadActions is triggered when a load event fires for a site. It is responsible for
// performing actions which will not take place until after that load event,
// such as interacting with the page and gathering screenshots. postLoadActions must be
// responsive to the cancellation of the context it is passed, as the main goroutine will
// wait for it to return before continuing. Because sites (especially complex ones) sometimes
// fail to fire load events for opaque reasons, this should be considered a "best-effort" function,
// and when something fails, it will generally just log a relevant message and press on.
func postLoadActions(cxt context.Context, tw *b.TaskWrapper, rawResult *b.RawResult, wg *sync.WaitGroup) {

	// This is a WaitGroup used for individual post load actions, and should not be
	// confused with the WaitGroup passed to this function.
	var individualActionsWG sync.WaitGroup

	// Enable request interception to block navigation (if specified)
	if *tw.SanitizedTask.IS.LockNavigation {
		err := chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
			err := fetch.Enable().Do(cxt)
			if err != nil {
				return err
			}

			bytes := new([]byte)
			err = chromedp.Evaluate(`((window, open) => {window.open = (url) => {};})(window, window.open);`,
				bytes).Do(cxt)

			return err
		}))
		if err != nil {
			tw.Log.Warn("could not lock navigation so did not complete post-load actions: " + err.Error())
		}

	}

	// Capture screenshot
	if *tw.SanitizedTask.DS.Screenshot {
		individualActionsWG.Add(1)
		go captureScreenshot(cxt, path.Join(tw.TempDir, b.DefaultScreenshotFileName), tw.Log, &individualActionsWG)
	}

	// Capture cookies set so far
	if *tw.SanitizedTask.DS.Cookies {
		individualActionsWG.Add(1)
		go getCookies(cxt, tw.Log, rawResult, &individualActionsWG)
	}

	// Capture the DOM
	if *tw.SanitizedTask.DS.DOM {
		individualActionsWG.Add(1)
		go getDOM(cxt, tw.Log, rawResult, &individualActionsWG)
	}

	if *tw.SanitizedTask.IS.BasicInteraction {
		individualActionsWG.Add(1)
		go basicInteraction(cxt, tw.Log, &individualActionsWG)
	} else if *tw.SanitizedTask.IS.Gremlins {
		go runGremlinsJS(cxt, tw.Log, &individualActionsWG)
		individualActionsWG.Add(1)
	}

	individualActionsWG.Wait()
	log.Log.Debug("post load actions completed")
	wg.Done()
	return
}

// basicInteraction runs a few simple interactions with the page. This is mostly useful for pages
// that look for a bit of mouse movement or scrolling before they reveal their full content.
func basicInteraction(cxt context.Context, taskLog *logrus.Logger, wg *sync.WaitGroup) {
	var err error

	err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
		var bytes = new([]byte)
		for i := 0; i < 10; i += 1 {
			pixels := 70 - rand.Intn(100)
			err = chromedp.EvaluateAsDevTools("window.scrollBy(0,"+strconv.Itoa(pixels)+");", bytes).Do(cxt)
			if err != nil {
				return err
			}
			err = cxtSleep(cxt, time.Duration(rand.Intn(1000))*time.Millisecond)
			if err != nil {
				return err
			}
		}
		return nil
	}))
	if err != nil {
		taskLog.Warn("basic interaction started but did not complete: " + err.Error())
	}

	wg.Done()
	return
}

// runGremlinsJS uses gremlins.js (https://github.com/marmelab/gremlins.js/) to interact with the page
// a LOT in a short period of time. It does lots of clicking, scrolling, form filling, etc. This is
// significantly more aggressive than the basic interaction.
func runGremlinsJS(cxt context.Context, taskLog *logrus.Logger, wg *sync.WaitGroup) {
	var err error
    gremlinsApplet := `javascript: (function() { function callback() { gremlins.createHorde({ species: [gremlins.species.clicker(),gremlins.species.toucher(),gremlins.species.formFiller(),gremlins.species.scroller(),gremlins.species.typer()], mogwais: [gremlins.mogwais.alert(),gremlins.mogwais.fps(),gremlins.mogwais.gizmo()], strategies: [gremlins.strategies.distribution(), gremlins.strategies.allTogether({ nb: 100000 })] }).unleash(); } var s = document.createElement("script"); s.src = "https://unpkg.com/gremlins.js"; if (s.addEventListener) { s.addEventListener("load", callback, false); } else if (s.readyState) { s.onreadystatechange = callback; } document.body.appendChild(s); })()`

	err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
		var bytes = new([]byte)
		err = chromedp.EvaluateAsDevTools(gremlinsApplet, bytes).Do(cxt)
		if err != nil {
			return err
		}

		return nil
	}))
	if err != nil {
		taskLog.Warn("gremlinsjs started but did not complete: " + err.Error())
	}

	wg.Done()
	return
}

// captureScreenshot uses an existing browser context to capture a screenshot, logging any error to both
// the global MIDA log and the task-specific log
func captureScreenshot(cxt context.Context, outputPath string, taskLog *logrus.Logger, wg *sync.WaitGroup) {
	var data []byte
	var err error
	err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
		data, err = page.CaptureScreenshot().Do(cxt)
		if err != nil {
			log.Log.Error(err)
			return err
		}
		return nil
	}))
	if err != nil {
		taskLog.Warn("error capturing screenshot: " + err.Error())
	}

	err = ioutil.WriteFile(outputPath, data, 0644)
	if err != nil {
		taskLog.Warn("error writing screenshot to file: " + err.Error())
	}

	wg.Done()
}

// getCookies grabs all cookies from the browser
func getCookies(cxt context.Context, taskLog *logrus.Logger, rawResult *b.RawResult, wg *sync.WaitGroup) {
	var cookies []*network.Cookie
	var err error
	err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
		cookies, err = network.GetAllCookies().Do(cxt)
		if err != nil {
			taskLog.Error(err)
			return err
		}

		return nil
	}))
	if err != nil {
		taskLog.Warn("error capturing cookies: " + err.Error())
	} else {
		rawResult.Lock()
		rawResult.DevTools.Cookies = cookies
		rawResult.Unlock()
	}

	wg.Done()
}

// getDOM grabs the current state of the DOM from the browser
func getDOM(cxt context.Context, taskLog *logrus.Logger, rawResult *b.RawResult, wg *sync.WaitGroup) {
	var domData *cdp.Node
	var err error
	err = chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
		domData, err = dom.GetDocument().WithDepth(-1).WithPierce(true).Do(cxt)
		if err != nil {
			return err
		}

		return nil
	}))
	if err != nil {
		taskLog.Warn("failed to get DOM: " + err.Error())
	} else {
		rawResult.Lock()
		rawResult.DevTools.DOM = domData
		rawResult.Unlock()
	}

	wg.Done()
}

// cxtSleep is just a wrapper around a sleep function to make it responsive
// to context cancellations
func cxtSleep(cxt context.Context, t time.Duration) error {
	select {
	case <-cxt.Done():
		return cxt.Err()
	case <-time.After(t):
		return nil
	}
}
