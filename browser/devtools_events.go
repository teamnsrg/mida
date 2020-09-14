package browser

import (
	"context"
	"errors"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/sirupsen/logrus"
	"github.com/teamnsrg/chromedp"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"io/ioutil"
	"path"
	"sync"
	"time"
)

// PageLoadEventFired is the event handler for the Page.LoadEventFired event
func PageLoadEventFired(eventChan chan *page.EventLoadEventFired, loadEventChan chan<- bool, rawResult *b.RawResult, wg *sync.WaitGroup, ctxt context.Context) {
	done := false
	sentLoadEvent := false

	for {
		select {
		case _, ok := <-eventChan:
			if !ok { // Channel closed
				done = true
				break
			}

			rawResult.Lock()
			rawResult.TaskSummary.TaskTiming.LoadEvent = time.Now()
			rawResult.Unlock()

			log.Log.WithField("URL", rawResult.TaskSummary.TaskWrapper.SanitizedTask.URL).Debug("load event fired")
			rawResult.TaskSummary.TaskWrapper.Log.Debug("load event fired")

			// Ensure we only send one load event per page visit. Subsequent load events will be ignored
			if !sentLoadEvent {
				loadEventChan <- true // Signal that a load event has fired
				sentLoadEvent = true
			}

		case <-ctxt.Done(): // Context canceled, browser closed
			done = true
			break
		}

		if done {
			break
		}
	}

	wg.Done()
}

func PageFrameNavigated(eventChan chan *page.EventFrameNavigated, devtoolsState *DTState, wg *sync.WaitGroup, ctxt context.Context) {
	done := false
	for {
		select {
		case ev, ok := <-eventChan:
			if !ok { // Channel closed
				done = true
				break
			}

			// Keep track of the frame at the top level, so we can block navigations when needed
			if ev.Frame.ParentID == "" {
				devtoolsState.Lock()
				devtoolsState.mainFrameLoaderId = ev.Frame.ID.String()
				devtoolsState.Unlock()
			}

		case <-ctxt.Done(): // Context canceled, browser closed
			done = true
			break
		}

		if done {
			break
		}
	}

	wg.Done()
}

// PageJavaScriptDialogOpening handles JavaScript dialog events, for now simply dismissing them so data collection can continue
func PageJavaScriptDialogOpening(eventChan chan *page.EventJavascriptDialogOpening, wg *sync.WaitGroup, ctxt context.Context, log *logrus.Logger) {
	done := false
	for {
		select {
		case _, ok := <-eventChan:
			if !ok { // Channel closed
				done = true
				break
			}

			err := chromedp.Run(ctxt, chromedp.ActionFunc(func(cxt context.Context) error {
				err := page.HandleJavaScriptDialog(false).Do(cxt)
				if err != nil {
					return errors.New("failed to dismiss javascript dialog" + err.Error())
				}
				return nil
			}))
			if err != nil {
				log.Error(err)
			}

		case <-ctxt.Done(): // Context canceled, browser closed
			done = true
			break
		}

		if done {
			break
		}
	}

	wg.Done()
}

// NetworkRequestWillBeSent is the event handler for the Network.RequestWillBeSent event
func NetworkRequestWillBeSent(eventChan chan *network.EventRequestWillBeSent, rawResult *b.RawResult, wg *sync.WaitGroup, ctxt context.Context) {
	done := false
	for {
		select {
		case ev, ok := <-eventChan:
			if !ok { // Channel closed
				done = true
				break
			}

			rawResult.Lock()
			if _, ok := rawResult.DevTools.Network.RequestWillBeSent[ev.RequestID.String()]; !ok {
				rawResult.DevTools.Network.RequestWillBeSent[ev.RequestID.String()] = make([]*network.EventRequestWillBeSent, 0)
			}
			rawResult.DevTools.Network.RequestWillBeSent[ev.RequestID.String()] = append(
				rawResult.DevTools.Network.RequestWillBeSent[ev.RequestID.String()], ev)
			rawResult.Unlock()

		case <-ctxt.Done(): // Context canceled, browser closed
			done = true
			break
		}

		if done {
			break
		}
	}

	wg.Done()
}

// NetworkResponseReceived is the event handler for Network.ResponseReceived events
func NetworkResponseReceived(eventChan chan *network.EventResponseReceived, rawResult *b.RawResult, wg *sync.WaitGroup, ctxt context.Context) {
	done := false
	for {
		select {
		case ev, ok := <-eventChan:
			if !ok { // Channel closed
				done = true
				break
			}

			rawResult.Lock()
			rawResult.DevTools.Network.ResponseReceived[ev.RequestID.String()] = ev
			rawResult.Unlock()
		case <-ctxt.Done(): // Context canceled, browser closed
			done = true
			break
		}

		if done {
			break
		}
	}

	wg.Done()
}

// NetworkLoadingFinished is the event handler for the Network.LoadingFinished event
func NetworkLoadingFinished(eventChan chan *network.EventLoadingFinished, rawResult *b.RawResult, wg *sync.WaitGroup, ctxt context.Context, log *logrus.Logger) {
	var err error
	done := false
	resourceDownloadSuccessCounter := 0
	resourceDownloadAttemptCounter := 0
	for {
		select {
		case ev, ok := <-eventChan:
			if !ok { // Channel closed
				done = true
				break
			}

			// Skip downloading the resource if we aren't gathering them
			if !*rawResult.TaskSummary.TaskWrapper.SanitizedTask.DS.AllResources {
				break
			}

			rawResult.Lock()
			if _, ok := rawResult.DevTools.Network.RequestWillBeSent[ev.RequestID.String()]; !ok {
				// Skipping downloading a resource we have not seen a request for
				rawResult.Unlock()
				break
			}
			resourceDownloadAttemptCounter += 1
			var respBody []byte
			err = chromedp.Run(ctxt, chromedp.ActionFunc(func(ctxt context.Context) error {
				respBody, err = network.GetResponseBody(ev.RequestID).Do(ctxt)
				return err
			}))
			if err == nil {
				err = ioutil.WriteFile(path.Join(rawResult.TaskSummary.TaskWrapper.TempDir,
					b.DefaultResourceSubdir, ev.RequestID.String()), respBody, 0644)
				if err != nil {
					log.Errorf("failed to write resource (%s) to results directory", ev.RequestID.String())
				} else {
					resourceDownloadSuccessCounter += 1
				}
			}

			rawResult.Unlock()
		case <-ctxt.Done(): // Context canceled
			done = true
			break
		}

		if done {
			break
		}
	}

	if *rawResult.TaskSummary.TaskWrapper.SanitizedTask.DS.AllResources {
		log.Debugf("successfully downloaded %d out of %d resources",
			resourceDownloadSuccessCounter, resourceDownloadAttemptCounter)
	}

	wg.Done()
}

// FetchRequestPaused is the event handler for network requests which have been paused
func FetchRequestPaused(eventChan chan *fetch.EventRequestPaused, rawResult *b.RawResult, devtoolsState *DTState, wg *sync.WaitGroup, ctxt context.Context) {
	done := false
	tw := rawResult.TaskSummary.TaskWrapper
	for {
		select {
		case ev, ok := <-eventChan:
			if !ok { // Channel closed
				done = true
				break
			}

			err := chromedp.Run(ctxt, chromedp.ActionFunc(func(cxt context.Context) error {
				devtoolsState.Lock()
				mainFrame := devtoolsState.mainFrameLoaderId
				devtoolsState.Unlock()

				var err error
				if ev.ResourceType == network.ResourceTypeDocument {
					var frameId string
					rawResult.Lock()
					if _, ok := rawResult.DevTools.Network.RequestWillBeSent[ev.NetworkID.String()]; ok {
						rArr := rawResult.DevTools.Network.RequestWillBeSent[ev.NetworkID.String()]
						frameId = rArr[len(rArr)-1].FrameID.String()
					}
					rawResult.Unlock()

					if frameId == mainFrame {
						err = fetch.FailRequest(ev.RequestID, network.ErrorReasonAborted).Do(cxt)
						log.Log.Debug("denying navigation to " + ev.Request.URL)
					} else {
						err = fetch.ContinueRequest(ev.RequestID).Do(cxt)
					}
				} else {
					err = fetch.ContinueRequest(ev.RequestID).Do(cxt)
				}

				return err
			}))
			if err != nil {
				tw.Log.Error("failed to continue a paused request: " + err.Error())
			}

		case <-ctxt.Done(): // Context canceled, browser closed
			done = true
			break
		}

		if done {
			break
		}
	}

	wg.Done()
}

func TargetTargetCreated(eventChan chan *target.EventTargetCreated, wg *sync.WaitGroup, ctxt context.Context) {
	done := false
	for {
		select {
		case ev, ok := <-eventChan:
			if !ok { // Channel closed
				done = true
				break
			}

			// Prevent new tabs from opening up
			if ev.TargetInfo.URL != "about:blank" && ev.TargetInfo.Type == "page" {
				err := chromedp.Run(ctxt, chromedp.ActionFunc(func(cxt context.Context) error {
					log.Log.Debug("closing newly opened target " + ev.TargetInfo.URL)
					success, err := target.CloseTarget(ev.TargetInfo.TargetID).Do(cxt)
					if err != nil || !success {
						errString := "failed to close new target"
						if err != nil {
							errString += ": " + err.Error()
						}
						return errors.New(errString)
					}
					return err
				}))
				if err != nil {
					log.Log.Error(err)
				}
			}

		case <-ctxt.Done(): // Context canceled
			done = true
			break
		}

		if done {
			break
		}
	}

	wg.Done()
}
