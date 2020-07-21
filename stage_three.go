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
			break
		}

		rawResultChan <- rawResult
	}

	crawlerWG.Done()
}
