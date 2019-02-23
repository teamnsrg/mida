package main

import (
	"time"
)

func PostprocessResult(rawResultChan <-chan RawMIDAResult, finalResultChan chan<- FinalMIDAResult) {
	for rawResult := range rawResultChan {
		finalResult := FinalMIDAResult{
			SanitizedTask: rawResult.SanitizedTask,
			Stats:         rawResult.Stats,
		}

		finalResult.Stats.Timing.BeginPostprocess = time.Now()

		// Ignore any requests/responses which do not have a matching request/response
		finalResult.ResourceMetadata = make(map[string]Resource)

		for k := range rawResult.Requests {
			if _, ok := rawResult.Responses[k]; ok {
				finalResult.ResourceMetadata[k] = Resource{
					rawResult.Requests[k],
					rawResult.Responses[k],
				}
			}
		}

		finalResult.ScriptMetadata = rawResult.Scripts

		Log.Info("Requests Made: ", len(rawResult.Requests))
		Log.Info("Responses Received: ", len(rawResult.Responses))
		Log.Info("Scripts Parsed: ", len(rawResult.Scripts))

		finalResult.Stats.Timing.EndPostprocess = time.Now()
		finalResultChan <- finalResult
	}

	// All Postprocessed results have been sent so close the channel
	close(finalResultChan)
}
