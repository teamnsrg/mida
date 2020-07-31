package main

import (
	"github.com/spf13/viper"
	t "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/storage"
	"sync"
	"time"
)

func stage5(finalResultChan <-chan *t.FinalResult, monitoringChan chan<- *t.TaskSummary,
	storageWG *sync.WaitGroup, pipelineWG *sync.WaitGroup) {

	for fr := range finalResultChan {

		fr.Summary.TaskTiming.BeginStorage = time.Now()
		err := storage.StoreAll(fr)
		if err != nil {
			log.Log.Error(err)
		}
		fr.Summary.TaskTiming.EndStorage = time.Now()

		if viper.GetBool("monitor") {
			monitoringChan <- &(fr.Summary)
		}

		err = storage.CleanupTask(fr)
		if err != nil {
			log.Log.Error(err)
		}

		pipelineWG.Done()
	}

	storageWG.Done()
}
