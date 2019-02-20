package main

import (
	"time"
)

type ResourceTreeNode struct {
	RequestID string
	URL       string
	Type      string
	Children  []*ResourceTreeNode
}

func PostprocessResult(rawResultChan <-chan RawMIDAResult, finalResultChan chan<- FinalMIDAResult) {
	for rawResult := range rawResultChan {
		finalResult := FinalMIDAResult{
			SanitizedTask: rawResult.SanitizedTask,
			Stats:         rawResult.Stats,
			Timing:        rawResult.Timing,
		}

		Log.Info("Requests Made: ", len(rawResult.Requests))
		Log.Info("Responses Received: ", len(rawResult.Responses))
		Log.Info("Scripts Parsed: ", len(rawResult.Scripts))

		finalResult.Stats.TimeAfterValidation = time.Now()
		finalResultChan <- finalResult
	}

	// All Postprocessed results have been sent so close the channel
	close(finalResultChan)
}
