package main

import (
	"github.com/teamnsrg/mida/jstrace"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/resourcetree"
	"github.com/teamnsrg/mida/storage"
	t "github.com/teamnsrg/mida/types"
	"path"
	"strings"
	"time"
)

func PostprocessResult(rawResultChan <-chan t.RawMIDAResult, finalResultChan chan<- t.FinalMIDAResult) {
	for rawResult := range rawResultChan {
		finalResult := t.FinalMIDAResult{
			SanitizedTask: rawResult.SanitizedTask,
			Stats:         rawResult.Stats,
			JSTrace:       new(jstrace.CleanedJSTrace),
		}

		finalResult.Stats.Timing.BeginPostprocess = time.Now()

		// Ignore any requests/responses which do not have a matching request/response
		if rawResult.SanitizedTask.ResourceMetadata {
			finalResult.ResourceMetadata = make(map[string]t.Resource)
			for k := range rawResult.Requests {
				if _, ok := rawResult.Responses[k]; ok {

					var tdl int64 = -1
					if _, okData := rawResult.DataLengths[k]; okData {
						tdl = rawResult.DataLengths[k]
					}

					finalResult.ResourceMetadata[k] = t.Resource{
						Requests:        rawResult.Requests[k],
						Responses:       rawResult.Responses[k],
						TotalDataLength: tdl,
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
			}

			// Try to fix up JS trace using script metadata we gathered. First, pick the isolate from
			// our trace that makes the most sense as the one devtools was attached to.

			// First, narrow off any isolates which contain scripts from Chrome extensions
			isolatesToDelete := make([]string, 0)

			for k, v := range trace.Isolates {
				extScripts := 0
				nonExtScripts := 0
				for _, scr := range v.Scripts {
					if strings.HasPrefix(scr.Url, "chrome-extension") {
						extScripts += 1
					} else {
						nonExtScripts += 1
					}
				}

				if extScripts > nonExtScripts {
					isolatesToDelete = append(isolatesToDelete, k)
				}
			}

			for _, i := range isolatesToDelete {
				delete(trace.Isolates, i)
			}

			// Now, figure out which isolate best covers the scripts we saw being parsed from
			// DevTools, and keep only that one
			bestIsolate := ""
			bestNumCovered := -1
			for isolateId, isolate := range trace.Isolates {
				numCovered := 0
				for scriptId := range rawResult.Scripts {
					if _, ok := isolate.Scripts[scriptId]; ok {
						numCovered += 1
					}
				}
				if numCovered > bestNumCovered {
					bestIsolate = isolateId
					bestNumCovered = numCovered
				}
			}

			if bestIsolate != "" && bestNumCovered > 0 {
				numInMetadata := 0
				for scriptId := range trace.Isolates[bestIsolate].Scripts {
					if _, ok := finalResult.ScriptMetadata[scriptId]; ok {
						numInMetadata += 1
					}
				}

				log.Log.Infof("[ %s ] Best isolate (%s) covered %d of %d scripts from scriptParsed event",
					rawResult.SanitizedTask.Url, bestIsolate, bestNumCovered, len(finalResult.ScriptMetadata))
				log.Log.Infof("[ %s ] scriptParsed events covered %d of %d scripts in that isolate",
					rawResult.SanitizedTask.Url, numInMetadata, len(trace.Isolates[bestIsolate].Scripts))

				// Fingerprinting checks using trace data
				if rawResult.SanitizedTask.OpenWPMChecks {
					err = jstrace.OpenWPMCheckTraceForFingerprinting(trace)
					if err != nil {
						log.Log.Error(err)
					}
				}

				// Create our cleaned trace for our final result
				finalResult.JSTrace.Scripts = trace.Isolates[bestIsolate].Scripts
				finalResult.JSTrace.Url = rawResult.SanitizedTask.Url
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
