package browser

import (
	"context"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/sirupsen/logrus"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"io/ioutil"
	"path"
	"sync"
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

	// Enable request interception to block navigation
	err := chromedp.Run(cxt, chromedp.ActionFunc(func(cxt context.Context) error {
		err := fetch.Enable().Do(cxt)
		return err
	}))
	if err != nil {
		log.Log.Error(err)
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

	individualActionsWG.Wait()
	log.Log.Debug("post load actions completed")
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
