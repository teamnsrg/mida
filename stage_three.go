package main

import (
	t "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/browser"
	"sync"
)

func stage3(taskWrapperChan <-chan *t.TaskWrapper, rawResultChan chan<- *t.RawResult, crawlerWG *sync.WaitGroup) {

	for tw := range taskWrapperChan {
		rawResult, err := browser.VisitPageDevtoolsProtocol(tw)
		if err != nil {
			if rawResult != nil {
				rawResult.TaskSummary.Success = false
				rawResult.TaskSummary.FailureReason = err.Error()
			} else {
				// Something is majorly broken, so we need to just close
				break
			}
		}

		rawResultChan <- rawResult
	}

	crawlerWG.Done()
}
