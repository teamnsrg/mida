package browser

import (
	"context"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/sirupsen/logrus"
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
			loadEventChan <- true // Signal that a load event has fired

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
				rawResult.DevTools.Network.RequestWillBeSent[ev.RequestID.String()] = make([]network.EventRequestWillBeSent, 0)
			}
			rawResult.DevTools.Network.RequestWillBeSent[ev.RequestID.String()] = append(
				rawResult.DevTools.Network.RequestWillBeSent[ev.RequestID.String()], *ev)
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
			rawResult.DevTools.Network.ResponseReceived[ev.RequestID.String()] = *ev
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
