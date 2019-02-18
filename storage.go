package main

import (
	"os"
	"sync"
	"time"
)

// Takes validated results and stores them as the task specifies, either locally, remotely, or both
func StoreResults(finalResultChan <-chan FinalMIDAResult, mConfig MIDAConfig, monitoringChan chan<- TaskStats, retryChan chan<- SanitizedMIDATask, storageWG *sync.WaitGroup, pipelineWG *sync.WaitGroup) {
	for r := range finalResultChan {

		if !r.sanitizedTask.TaskFailed {
			// Store results here from a successfully completed task
		}

		// Remove all data from crawl
		// TODO: Add ability to save user data directory (without saving crawl data inside it)
		err := os.RemoveAll(r.sanitizedTask.UserDataDirectory)
		if err != nil {
			Log.Fatal(err)
		}

		if r.sanitizedTask.TaskFailed {
			if r.sanitizedTask.CurrentAttempt >= r.sanitizedTask.MaxAttempts {
				// We are abandoning trying this task. Too bad.
				Log.Error("Task failed after ", r.sanitizedTask.MaxAttempts, " attempts.")

			} else {
				// "Squash" task results and put the task back at the beginning of the pipeline
				Log.Debug("Retrying task...")
				r.sanitizedTask.CurrentAttempt++
				pipelineWG.Add(1)
				retryChan <- r.sanitizedTask
			}
		}

		// Send stats to Prometheus
		if mConfig.EnableMonitoring {
			r.stats.TimeAfterStorage = time.Now()
			monitoringChan <- r.stats
		}

		pipelineWG.Done()
	}

	storageWG.Done()
}
