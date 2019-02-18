package main

import (
	"time"
)

type TimingResult struct {
	BrowserOpen           time.Time
	DevtoolsConnect       time.Time
	ConnectionEstablished time.Time
	LoadEvent             time.Time
	DOMContentEvent       time.Time
	BrowserClose          time.Time
}

type RawMIDAResult struct {
	SanitizedTask SanitizedMIDATask
	Stats         TaskStats
	Timing        TimingResult
}

type FinalMIDAResult struct {
	SanitizedTask SanitizedMIDATask
	Stats         TaskStats
	Timing        TimingResult
}

func PostprocessResult(rawResultChan <-chan RawMIDAResult, finalResultChan chan<- FinalMIDAResult) {
	for rawResult := range rawResultChan {
		finalResult := FinalMIDAResult{
			SanitizedTask: rawResult.SanitizedTask,
			Stats:         rawResult.Stats,
			Timing:        rawResult.Timing,
		}
		finalResult.Stats.TimeAfterValidation = time.Now()
		finalResultChan <- finalResult
	}

	// All Postprocessed results have been sent so close the channel
	close(finalResultChan)
}
