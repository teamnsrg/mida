package main

import (
	"github.com/prometheus/common/log"
	"sync"
)

// Takes validated results and stores them as the task specifies, either locally, remotely, or both
func StoreResults(frc <-chan FinalMIDAResult, mConfig MIDAConfig, mc chan<- TaskStats, storageWG *sync.WaitGroup) {
	for r := range frc {
		log.Info("Store results here", r, mConfig)
		if mConfig.EnableMonitoring {
			// Send statistics for Prometheus monitoring
			mc <- r.stats
		}
	}

	storageWG.Done()
}
