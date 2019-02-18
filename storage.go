package main

import (
	"os"
	"sync"
	"time"
)

// Takes validated results and stores them as the task specifies, either locally, remotely, or both
func StoreResults(finalResultChan <-chan FinalMIDAResult, mConfig MIDAConfig, monitoringChan chan<- TaskStats, retryChan chan<- SanitizedMIDATask, storageWG *sync.WaitGroup, pipelineWG *sync.WaitGroup) {
	for r := range finalResultChan {

		if !r.SanitizedTask.TaskFailed {
			// Store results here from a successfully completed task
		}

		// Remove all data from crawl
		// TODO: Add ability to save user data directory (without saving crawl data inside it)
		err := os.RemoveAll(r.SanitizedTask.UserDataDirectory)
		if err != nil {
			Log.Fatal(err)
		}

		if r.SanitizedTask.TaskFailed {
			if r.SanitizedTask.CurrentAttempt >= r.SanitizedTask.MaxAttempts {
				// We are abandoning trying this task. Too bad.
				Log.Error("Task failed after ", r.SanitizedTask.MaxAttempts, " attempts.")

			} else {
				// "Squash" task results and put the task back at the beginning of the pipeline
				Log.Debug("Retrying task...")
				r.SanitizedTask.CurrentAttempt++
				pipelineWG.Add(1)
				retryChan <- r.SanitizedTask
			}
		}

		// Send stats to Prometheus
		if mConfig.EnableMonitoring {
			r.Stats.TimeAfterStorage = time.Now()
			monitoringChan <- r.Stats
		}

		pipelineWG.Done()
	}

	storageWG.Done()
}
