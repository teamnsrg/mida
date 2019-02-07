package main

import (
	"time"
)

type RawMIDAResult struct {
	sanitizedTask SanitizedMIDATask
	stats         TaskStats
}

type FinalMIDAResult struct {
	sanitizedTask SanitizedMIDATask
	stats         TaskStats
}

func PostprocessResult(rawResultChan <-chan RawMIDAResult, finalResultChan chan<- FinalMIDAResult) {
	for rawResult := range rawResultChan {
		finalResult := FinalMIDAResult{
			sanitizedTask: rawResult.sanitizedTask,
			stats:         rawResult.stats,
		}
		finalResult.stats.TimeAfterValidation = time.Now()
		finalResultChan <- finalResult
	}

	// All Postprocessed results have been sent so close the channel
	close(finalResultChan)
}
