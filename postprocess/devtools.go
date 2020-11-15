package postprocess

import (
	"github.com/chromedp/cdproto/debugger"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/yibrowse"
	"path"
	"strings"
	"time"
)

func DevTools(rr *b.RawResult) (b.FinalResult, error) {
	finalResult := b.FinalResult{
		Summary:            rr.TaskSummary,
		DTResourceMetadata: make(map[string]b.DTResource),
		DTScriptMetadata:   make(map[string]*debugger.EventScriptParsed),
	}

	finalResult.Summary.TaskTiming.BeginPostprocess = time.Now()

	// For brevity
	st := rr.TaskSummary.TaskWrapper.SanitizedTask

	// Ignore any requests/responses which do not have a matching request/response
	if *st.DS.ResourceMetadata {
		for k := range rr.DevTools.Network.RequestWillBeSent {
			if _, ok := rr.DevTools.Network.ResponseReceived[k]; ok {

				/*
					var tdl int64 = -1
					if _, okData := rr.DataLengths[k]; okData {
						tdl = rawResult.DataLengths[k]
					}
				*/

				finalResult.DTResourceMetadata[k] = b.DTResource{
					Requests: rr.DevTools.Network.RequestWillBeSent[k],
					Response: rr.DevTools.Network.ResponseReceived[k],
					// TotalDataLength: tdl,
				}

			}
		}
	}

	if *st.DS.ScriptMetadata {
		for _, v := range rr.DevTools.Scripts {
			if _, ok := finalResult.DTScriptMetadata[v.ScriptID.String()]; ok {
				rr.TaskSummary.TaskWrapper.Log.Warnf("found duplicate scriptId: %s", v.ScriptID.String())
			} else {
				finalResult.DTScriptMetadata[v.ScriptID.String()] = v
			}
		}
	}

	if *st.DS.Cookies {
		finalResult.DTCookies = rr.DevTools.Cookies
	}

	if *st.DS.DOM {
		finalResult.DTDOM = rr.DevTools.DOM
	}

	if *st.DS.YiBrowse {
		trace, err := yibrowse.ParseTraceFromFile(path.Join(rr.TaskSummary.TaskWrapper.TempDir, b.DefaultBrowserLogFileName))
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
			for _, scriptParsedEvent := range rr.DevTools.Scripts {
				if _, ok := isolate.Scripts[scriptParsedEvent.ScriptID.String()]; ok {
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
				if _, ok := finalResult.DTScriptMetadata[scriptId]; ok {
					numInMetadata += 1
				}
			}

			log.Log.Debugf("[ %s ] Best isolate (%s) covered %d of %d scripts from scriptParsed event",
				st.URL, bestIsolate, bestNumCovered, len(finalResult.DTScriptMetadata))
			log.Log.Debugf("[ %s ] scriptParsed events covered %d of %d scripts in that isolate",
				st.URL, numInMetadata, len(trace.Isolates[bestIsolate].Scripts))

			// Create our cleaned trace for our final result
			finalResult.DTYibrowseCleanedTrace.Scripts = trace.Isolates[bestIsolate].Scripts
			finalResult.DTYibrowseCleanedTrace.Url = st.URL
		} else {
			log.Log.Error("yibrowse could not find a valid isolate with a nonzero number of calls")
		}
	}

	finalResult.Summary.NumResources = len(rr.DevTools.Network.RequestWillBeSent)
	finalResult.Summary.TaskTiming.EndPostprocess = time.Now()

	return finalResult, nil
}
