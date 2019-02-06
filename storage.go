package main

import (
	"github.com/prometheus/common/log"
	"sync"
)

// Takes validated results and stores them as the task specifies, either locally, remotely, or both
func StoreResults(c chan FinalMIDAResult, mConfig MIDAConfig, storageWG *sync.WaitGroup) {
	for r := range c {
		log.Info("Store results here", r, mConfig)
	}

	storageWG.Done()
}
