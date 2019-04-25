package main

import (
	"github.com/teamnsrg/mida/jstrace"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/resourcetree"
	"github.com/teamnsrg/mida/storage"
	t "github.com/teamnsrg/mida/types"
	"path"
	"time"
)

func PostprocessResult(rawResultChan <-chan t.RawMIDAResult, finalResultChan chan<- t.FinalMIDAResult) {
	for rawResult := range rawResultChan {
		finalResult := t.FinalMIDAResult{
			SanitizedTask: rawResult.SanitizedTask,
			Stats:         rawResult.Stats,
		}

		finalResult.Stats.Timing.BeginPostprocess = time.Now()

		// Ignore any requests/responses which do not have a matching request/response
		if rawResult.SanitizedTask.ResourceMetadata {
			finalResult.ResourceMetadata = make(map[string]t.Resource)
			for k := range rawResult.Requests {
				if _, ok := rawResult.Responses[k]; ok {
					finalResult.ResourceMetadata[k] = t.Resource{
						Requests:  rawResult.Requests[k],
						Responses: rawResult.Responses[k],
					}
				}
			}
		}

		// Store script metadata
		if rawResult.SanitizedTask.ScriptMetadata {
			finalResult.ScriptMetadata = rawResult.Scripts
		}

		if rawResult.SanitizedTask.JSTrace {
			trace, err := jstrace.ParseTraceFromFile(path.Join(rawResult.SanitizedTask.UserDataDirectory,
				rawResult.SanitizedTask.RandomIdentifier, storage.DefaultBrowserLogFileName))
			if err != nil {
				log.Log.Info(err)
			} else {
				finalResult.JSTrace = trace
			}

			// Try to fix up JS trace using script metadata we gathered
			if rawResult.SanitizedTask.ScriptMetadata {
				for isolate, scriptIds := range trace.UnknownScripts {
					for scriptId := range scriptIds {
						if _, ok := rawResult.Scripts[scriptId]; ok {
							trace.Isolates[isolate].Scripts[scriptId].BaseUrl = rawResult.Scripts[scriptId].URL
						}
					}
				}
			}

			// Fingerprinting checks using trace data
			if rawResult.SanitizedTask.OpenWPMChecks {
				err = jstrace.OpenWPMCheckTraceForFingerprinting(finalResult.JSTrace)
				if err != nil {
					log.Log.Error(err)
				}
			}
		}

		if rawResult.SanitizedTask.WebsocketTraffic {
			finalResult.WebsocketData = rawResult.WebsocketData
		}

		if rawResult.SanitizedTask.ResourceTree {
			rootNode, orphans, err := resourcetree.BuildResourceTree(&rawResult)
			if err != nil {
				log.Log.Error(err)
			}
			finalResult.RTree = &t.ResourceTree{
				RootNode: rootNode,
				Orphans:  orphans,
			}
		}

		// Passthroughs - These raw results just get copied into the final result
		finalResult.CrawlHostInfo = rawResult.CrawlHostInfo

		// Send our final results on for storage
		finalResult.Stats.Timing.EndPostprocess = time.Now()

		// Now fill in the metadata
		finalResult.Metadata = BuildMetadata(&finalResult)

		finalResultChan <- finalResult
	}

	// All PostProcessed results have been sent so close the channel
	close(finalResultChan)
}

// Using the full results, construct the metadata object for this task
func BuildMetadata(r *t.FinalMIDAResult) *t.CrawlMetadata {

	metadata := new(t.CrawlMetadata)
	metadata.Task = r.SanitizedTask
	metadata.Timing = r.Stats.Timing
	metadata.CrawlHostInfo = r.CrawlHostInfo
	metadata.Failed = r.SanitizedTask.TaskFailed
	metadata.FailureCodes = r.SanitizedTask.PastFailureCodes

	metadata.NumResources = len(r.ResourceMetadata)
	metadata.NumScripts = len(r.ScriptMetadata)
	metadata.NumWSConnections = len(r.WebsocketData)

	return metadata
}
