package main

import "github.com/prometheus/common/log"

// Takes validated results and stores them as the task specifies, either locally, remotely, or both
func StoreResults(c chan FinalMIDAResult, mConfig MIDAConfig) {

	// TODO: Graceful exit using a signal from the main thread
	for r := range c {
		log.Info("Store results here", r, mConfig)
	}
}
