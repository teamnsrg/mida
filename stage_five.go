package main

import (
	t "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/storage"
	"sync"
)

func stage5(finalResultChan <-chan *t.FinalResult, monitoringChan chan<- *t.TaskSummary,
	storageWG *sync.WaitGroup, pipelineWG *sync.WaitGroup) {

	for fr := range finalResultChan {
		err := storage.StoreAll(fr)
		if err != nil {
			log.Log.Error(err)
		}

		err = storage.CleanupTask(fr)
		if err != nil {
			log.Log.Error(err)
		}

		pipelineWG.Done()
	}

	storageWG.Done()
}
