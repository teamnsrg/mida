package postprocess

import (
	"github.com/chromedp/cdproto/debugger"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/vv8"
	"io/ioutil"
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
	tempDir := rr.TaskSummary.TaskWrapper.TempDir

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

	if *st.DS.VV8 {
		files, err := ioutil.ReadDir(tempDir)
		if err != nil {
			log.Log.Error(err)
		} else {
			for _, f := range files {
				if strings.HasPrefix(f.Name(), "vv8") && strings.HasSuffix(f.Name(), "log") {
					isolateMap, err := vv8.ProcessLogFile(path.Join(tempDir, f.Name()))
					if err != nil {
						log.Log.Error(err)
					} else {
						// TODO: Need to handle multiple valid VV8 log files here
						// Maybe ProcessLogFile should be ProcessLogFiles() ???
						finalResult.DTVV8IsolateMap = isolateMap
					}
				}
			}
		}
	}

	finalResult.Summary.NumResources = len(rr.DevTools.Network.RequestWillBeSent)

	finalResult.Summary.TaskTiming.EndPostprocess = time.Now()

	return finalResult, nil
}
