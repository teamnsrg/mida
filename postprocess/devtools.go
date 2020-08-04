package postprocess

import (
	b "github.com/teamnsrg/mida/base"
	"time"
)

func DevTools(rr *b.RawResult) (b.FinalResult, error) {
	finalResult := b.FinalResult{
		Summary:            rr.TaskSummary,
		DTResourceMetadata: make(map[string]b.DTResource),
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

	if *st.DS.Cookies {
		finalResult.DTCookies = rr.DevTools.Cookies
	}

	finalResult.Summary.TaskTiming.EndPostprocess = time.Now()

	return finalResult, nil
}
