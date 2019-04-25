package jstrace

import (
	"bufio"
	"github.com/teamnsrg/mida/log"
	"io"
	"os"
	"strings"
)

// ParseTraceFromFile parses a file created as raw output from an
// instrumented version of the browser.
func ParseTraceFromFile(fname string) (*JSTrace, error) {

	var trace JSTrace
	trace.Isolates = make(map[string]*Isolate)
	file, err := os.Open(fname)
	if err != nil {
		return &trace, err
	}
	r := bufio.NewReader(file)

	// Stores the current call for each isolate
	iActiveCalls := make(map[string]*Call)

	// Unknown scripts: We see calls for them but never saw a beginning/base URL
	// These likely appear due to oversights in our browser instrumentation, but
	// until we can figure that out, we just grab them and try to get the base URL
	// from the script metadata gathered from DevTools
	trace.UnknownScripts = make(map[string]map[string]bool)

	// Trace metadata
	lineNum := 0

	for {
		// Get the next lineBytes from our trace
		isPrefix := true
		var lineBytes, tmpLine []byte
		for isPrefix && err == nil {
			tmpLine, isPrefix, err = r.ReadLine()
			lineBytes = append(lineBytes, tmpLine...)
		}
		if err != nil {
			if err != io.EOF {
				log.Log.Error(err)
			}
			break
		}

		lineNum += 1
		l := processLine(string(lineBytes))

		if l.LT == ErrorLine || l.LT == UnknownLine {
			log.Log.Info(lineNum, " : ", string(lineBytes))
		}

		// Ignore non-MIDA lines
		if l.LT == OtherLine {
			continue
		}

		// Now starts the state machine for building the trace line-by-line
		if l.LT == ControlLine {
			// If there's an active call for this isolate, we need to complete it
			// and add it to the applicable script
			if iActiveCalls[l.Isolate] != nil {
				// Make sure that we have seen this isolate before
				if _, ok := trace.Isolates[l.Isolate]; ok {
					// Add this call to the applicable script
					if _, ok := trace.Isolates[l.Isolate].Scripts[iActiveCalls[l.Isolate].ScriptId]; !ok {
						newScript := Script{
							ScriptId: l.ScriptId,
							BaseUrl:  "(UNKNOWN)",
							Calls:    make([]Call, 0),
						}
						trace.Isolates[l.Isolate].Scripts[iActiveCalls[l.Isolate].ScriptId] = &newScript
						if _, ok := trace.UnknownScripts[l.Isolate]; !ok {
							trace.UnknownScripts[l.Isolate] = make(map[string]bool)
						}
						trace.UnknownScripts[l.Isolate][iActiveCalls[l.Isolate].ScriptId] = true
					}

					trace.Isolates[l.Isolate].Scripts[iActiveCalls[l.Isolate].ScriptId].Calls = append(
						trace.Isolates[l.Isolate].Scripts[iActiveCalls[l.Isolate].ScriptId].Calls, *iActiveCalls[l.Isolate])
					trace.StoredCalls += 1
				} else {
					trace.IgnoredCalls += 1
				}

				// No active call anymore
				iActiveCalls[l.Isolate] = nil
			}

			if l.IsBegin && !l.IsCallback {
				// The beginning of a call or callback
				// If we haven't seen the isolate before, create an entry for it
				if _, ok := trace.Isolates[l.Isolate]; !ok {
					trace.Isolates[l.Isolate] = new(Isolate)
					trace.Isolates[l.Isolate].Scripts = make(map[string]*Script)
				}

				if _, ok := trace.Isolates[l.Isolate].Scripts[l.ScriptId]; !ok {
					newScript := Script{
						ScriptId: l.ScriptId,
						BaseUrl:  l.BaseURL,
						Calls:    make([]Call, 0),
					}
					trace.Isolates[l.Isolate].Scripts[l.ScriptId] = &newScript

				}
			} else {
				// This is the end of a call or callback
				// We ignore this since we are not concerned about individual executions in this implementation
			}
		} else if l.LT == CallLine {
			// If there's no entry for this isolate in the active calls map,
			// create one
			if _, ok := iActiveCalls[l.Isolate]; !ok {
				iActiveCalls[l.Isolate] = nil
			}

			// If there's an active call for this isolate, we need to complete it
			// and add it to the applicable script
			if iActiveCalls[l.Isolate] != nil {
				// Make sure that we have seen this isolate before
				if _, ok := trace.Isolates[l.Isolate]; ok {
					// Add this call to the applicable script
					if _, ok := trace.Isolates[l.Isolate].Scripts[iActiveCalls[l.Isolate].ScriptId]; !ok {
						newScript := Script{
							ScriptId: l.ScriptId,
							BaseUrl:  "(UNKNOWN)",
							Calls:    make([]Call, 0),
						}
						trace.Isolates[l.Isolate].Scripts[iActiveCalls[l.Isolate].ScriptId] = &newScript
						if _, ok := trace.UnknownScripts[l.Isolate]; !ok {
							trace.UnknownScripts[l.Isolate] = make(map[string]bool)
						}
						trace.UnknownScripts[l.Isolate][iActiveCalls[l.Isolate].ScriptId] = true
					}

					trace.Isolates[l.Isolate].Scripts[iActiveCalls[l.Isolate].ScriptId].Calls = append(
						trace.Isolates[l.Isolate].Scripts[iActiveCalls[l.Isolate].ScriptId].Calls, *iActiveCalls[l.Isolate])
					trace.StoredCalls += 1
				} else {
					trace.IgnoredCalls += 1
				}

				// No active call anymore
				iActiveCalls[l.Isolate] = nil
			}

			// Create our new call, set as active for this isolate
			iActiveCalls[l.Isolate] = new(Call)
			iActiveCalls[l.Isolate].T = l.CallType
			iActiveCalls[l.Isolate].C = l.CallClass
			iActiveCalls[l.Isolate].F = l.CallFunc
			iActiveCalls[l.Isolate].Args = make([]Arg, 0)
			iActiveCalls[l.Isolate].ScriptId = l.ScriptId
		} else if l.LT == ArgLine {
			// If there's no entry for this isolate in the active calls map,
			// create one
			if _, ok := iActiveCalls[l.Isolate]; !ok {
				iActiveCalls[l.Isolate] = nil
			}

			if iActiveCalls[l.Isolate] == nil {
				// No active call for this argument, so we must ignore
				log.Log.WithField("LineNum", lineNum).Error("No active call for argument")
				continue
			}
			var a Arg
			a.T = l.ArgType
			a.Val = l.ArgVal
			iActiveCalls[l.Isolate].Args = append(iActiveCalls[l.Isolate].Args, a)
		} else if l.LT == RetLine {
			if iActiveCalls[l.Isolate] == nil {
				// No active call for this return value, so we must ignore
				log.Log.WithField("LineNum", lineNum).Error("No active call for return value")
				continue
			}
			var a Arg
			a.T = l.ArgType
			a.Val = l.ArgVal
			iActiveCalls[l.Isolate].Ret = a
		}
	}

	return &trace, nil
}

func processLine(s string) Line {
	var l Line

	fields := strings.Fields(s)

	if len(fields) < 4 {
		l.LT = OtherLine
		return l
	}

	if fields[0] != "[MIDA]" {
		l.LT = OtherLine
		return l
	}

	// Read isolate
	if !strings.HasPrefix(fields[1], "[") && strings.HasSuffix(fields[1], "]") {
		l.LT = ErrorLine
		return l
	}
	l.Isolate = fields[1][1 : len(fields[1])-1]

	if fields[3] == "[get]" || fields[3] == "[set]" || fields[3] == "[call]" || fields[3] == "[cons]" {
		if len(fields) != 6 {
			l.LT = ErrorLine
			return l
		}

		pieces := strings.Split(fields[4], "::")
		if len(pieces) != 2 {
			l.LT = ErrorLine
			return l
		}

		l.LT = CallLine
		l.CallType = fields[3][1 : len(fields[3])-1]
		l.CallClass = pieces[0]
		l.CallFunc = pieces[1]

		if strings.HasPrefix(fields[5], "[") && strings.HasSuffix(fields[5], "]") {
			l.ScriptId = fields[5][1 : len(fields[5])-1]
		} else {
			l.LT = ErrorLine
			return l
		}

		return l

	} else if fields[2] == "[arg]" {
		if len(fields) < 4 {
			l.LT = ErrorLine
			return l
		}

		if !strings.HasPrefix(fields[3], "[") && strings.HasSuffix(fields[3], "]") {
			l.LT = ErrorLine
			return l
		}

		l.LT = ArgLine
		l.ArgType = fields[3][1 : len(fields[3])-1]
		if len(fields) > 4 {
			// Get the argument value as the remainder of the string
			idx := strings.Index(s, "["+l.ArgType+"]")
			l.ArgVal = s[idx+len(l.ArgType)+3:]
		}

	} else if fields[2] == "[ret]" {
		if len(fields) < 4 {
			l.LT = ErrorLine
			return l
		}

		if !strings.HasPrefix(fields[3], "[") && strings.HasSuffix(fields[3], "]") {
			l.LT = ErrorLine
			return l
		}

		l.LT = RetLine
		l.ArgType = fields[3][1 : len(fields[3])-1]
		if len(fields) > 4 {
			// Get the argument value as the remainder of the string
			idx := strings.Index(s, "["+l.ArgType+"]")
			l.ArgVal = s[idx+len(l.ArgType)+3:]
		}

	} else if fields[3] == "BEGINCALL" {
		if len(fields) != 6 {
			l.LT = ErrorLine
			return l
		}

		l.LT = ControlLine
		l.IsBegin = true
		l.IsCallback = false
		l.TS = fields[2]

		// Get Script ID
		if !strings.HasPrefix(fields[4], "[") && strings.HasSuffix(fields[4], "]") {
			l.LT = ErrorLine
			return l
		}
		l.ScriptId = fields[4][1 : len(fields[4])-1]

		// Get Base URL
		if !strings.HasPrefix(fields[5], "[\"") && strings.HasSuffix(fields[5], "\"]") {
			l.LT = ErrorLine
			return l
		}
		l.BaseURL = fields[5][2 : len(fields[5])-2]

	} else if fields[3] == "ENDCALL" {
		if len(fields) != 5 {
			l.LT = ErrorLine
			return l
		}

		l.LT = ControlLine
		l.IsBegin = false
		l.IsCallback = false
		l.TS = fields[2]

		// Get Script ID
		if !strings.HasPrefix(fields[4], "[") && strings.HasSuffix(fields[4], "]") {
			l.LT = ErrorLine
			return l
		}
		l.ScriptId = fields[4][1 : len(fields[4])-1]

	} else if fields[3] == "BEGINCALLBACK" {
		if len(fields) != 5 {
			l.LT = ErrorLine
			return l
		}

		l.LT = ControlLine
		l.IsBegin = true
		l.IsCallback = true
		l.TS = fields[2]

		// Get Script ID
		if !strings.HasPrefix(fields[4], "[") && strings.HasSuffix(fields[4], "]") {
			l.LT = ErrorLine
			return l
		}
		l.ScriptId = fields[4][1 : len(fields[4])-1]

	} else if fields[3] == "ENDCALLBACK" {
		if len(fields) != 5 {
			l.LT = ErrorLine
			return l
		}

		l.LT = ControlLine
		l.IsBegin = false
		l.IsCallback = true
		l.TS = fields[2]

		// Get Script ID
		if !strings.HasPrefix(fields[4], "[") && strings.HasSuffix(fields[4], "]") {
			l.LT = ErrorLine
			return l
		}
		l.ScriptId = fields[4][1 : len(fields[4])-1]

	} else {
		l.LT = ErrorLine
	}

	return l
}
