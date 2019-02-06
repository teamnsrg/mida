package main

import (
	"github.com/prometheus/common/log"
	"os"
	"sync"
	"time"
)

// Takes validated results and stores them as the task specifies, either locally, remotely, or both
func StoreResults(frc <-chan FinalMIDAResult, mConfig MIDAConfig, mc chan<- TaskStats, storageWG *sync.WaitGroup) {
	for r := range frc {
		if mConfig.EnableMonitoring {
			// Send statistics for Prometheus monitoring
			r.stats.TimeAfterStorage = time.Now()
			mc <- r.stats
		}

		err := os.RemoveAll(r.sanitizedTask.UserDataDirectory)
		if err != nil {
			log.Fatal(err)
		}
	}

	storageWG.Done()
}
