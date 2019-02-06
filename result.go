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

func ValidateResult(rr <-chan RawMIDAResult, fr chan<- FinalMIDAResult) {
	for rawResult := range rr {
		finalResult := FinalMIDAResult{
			sanitizedTask: rawResult.sanitizedTask,
			stats:         rawResult.stats,
		}
		finalResult.stats.TimeAfterValidation = time.Now()
		fr <- finalResult
	}

	// All validated results have been sent so close the channel
	close(fr)
}
