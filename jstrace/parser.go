package jstrace

import (
	"bufio"
	"github.com/pkg/errors"
	"github.com/prometheus/common/log"
	"io"
	"os"
	"strings"
)

func ParseTraceFromFile(fname string) (*JSTrace, error) {

	var trace JSTrace
	trace.Isolates = make(map[string]*Isolate)
	file, err := os.Open(fname)
	if err != nil {
		return &trace, err
	}
	r := bufio.NewReader(file)

	// Stacks of executions for each isolate
	iStacks := make(map[string]ExecutionStack)

	// Stores the current call for each isolate
	iActiveCalls := make(map[string]*Call)

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
				log.Error(err)
			}
			break
		}

		lineNum += 1
		l := ProcessLine(string(lineBytes))

		if l.LT == ErrorLine || l.LT == UnknownLine {
			log.Info(lineNum, " : ", string(lineBytes))
		}

		// Ignore non-MIDA lines
		if l.LT == OtherLine {
			continue
		}

		// Now starts the state machine for building the trace line-by-line
		if l.LT == ControlLine {
			// First, check if there is a call in progress to be completed
			// If there's an active call for this isolate, we need to complete it
			// and add it to the current execution for this isolate
			if iActiveCalls[l.Isolate] != nil {
				if _, ok := iStacks[l.Isolate]; !ok {
					iStacks[l.Isolate] = make(ExecutionStack, 0)
				}

				if len(iStacks[l.Isolate]) == 0 {
					// No active executions
					// We ignore this call, since we cannot attribute it to a particular script
					continue
				}

				// Otherwise, we can add the call to the execution
				iStacks[l.Isolate][len(iStacks[l.Isolate])-1].Calls = append(
					iStacks[l.Isolate][len(iStacks[l.Isolate])-1].Calls, iActiveCalls[l.Isolate])

				// No active call anymore
				iActiveCalls[l.Isolate] = nil
			}

			if l.IsBegin {
				// The beginning of a call or callback
				// If we haven't seen the isolate before, create an entry for it
				if _, ok := iStacks[l.Isolate]; !ok {
					iStacks[l.Isolate] = make(ExecutionStack, 0)
				}

				if _, ok := trace.Isolates[l.Isolate]; !ok {
					trace.Isolates[l.Isolate] = new(Isolate)
					trace.Isolates[l.Isolate].Scripts = make(map[string]*Script)
				}

				// Create a new execution and push it onto the stack
				e := Execution{
					Calls:    make([]*Call, 0),
					Isolate:  l.Isolate,
					ScriptId: l.ScriptId,
					TS:       l.TS,
				}
				iStacks[l.Isolate] = iStacks[l.Isolate].Push(e)

				if _, ok := trace.Isolates[l.Isolate].Scripts[e.ScriptId]; !ok {
					newScript := Script{
						ScriptId:   l.ScriptId,
						BaseUrl:    l.BaseURL,
						Executions: make([]Execution, 0),
					}
					trace.Isolates[l.Isolate].Scripts[e.ScriptId] = &newScript

				} else {
					if !l.IsCallback {
						// Update the Base URL if it's a normal call
						// Weird case because somehow we have already seen this script
						// before in this isolate
						trace.Isolates[l.Isolate].Scripts[e.ScriptId].BaseUrl = l.BaseURL
					}
				}
			} else {
				// This is the end of a call or callback
				// If we haven't seen the isolate before, create an entry for it
				if _, ok := iStacks[l.Isolate]; !ok {
					iStacks[l.Isolate] = make(ExecutionStack, 0)
				}

				// First, try the simple case where stuff works as intended
				e, err := iStacks[l.Isolate].Peek(1)
				if err != nil {
					// Stack is empty and we can't do anything
					continue
				}

				if e.ScriptId == l.ScriptId && e.TS == l.TS {
					// Woo Hoo! Worked as expected.
					// Pop the execution off the stack and put in in our trace
					trace.Isolates[l.Isolate].Scripts[e.ScriptId].Executions = append(
						trace.Isolates[l.Isolate].Scripts[e.ScriptId].Executions, e)
					iStacks[l.Isolate], _, _ = iStacks[l.Isolate].Pop()
					continue
				}

				// If they didn't match up, we will check up to 3 deep in our execution stack
				// to try to find the scriptID/Timestamp

				e, err = iStacks[l.Isolate].Peek(2)
				if err != nil {
					// Stack is empty and we can't do anything
					continue
				}

				if e.ScriptId == l.ScriptId && e.TS == l.TS {
					// Found it on the second try
					// Pop the execution off the stack and put in in our trace
					trace.Isolates[l.Isolate].Scripts[e.ScriptId].Executions = append(
						trace.Isolates[l.Isolate].Scripts[e.ScriptId].Executions, e)
					_, _, _ = iStacks[l.Isolate].Pop()
					iStacks[l.Isolate], _, _ = iStacks[l.Isolate].Pop()
					continue
				}

				e, err = iStacks[l.Isolate].Peek(3)
				if err != nil {
					// Stack is empty and we can't do anything
					continue
				}

				if e.ScriptId == l.ScriptId && e.TS == l.TS {
					// Found it on the third try
					// Pop the execution off the stack and put in in our trace
					trace.Isolates[l.Isolate].Scripts[e.ScriptId].Executions = append(
						trace.Isolates[l.Isolate].Scripts[e.ScriptId].Executions, e)
					_, _, _ = iStacks[l.Isolate].Pop()
					_, _, _ = iStacks[l.Isolate].Pop()
					iStacks[l.Isolate], _, _ = iStacks[l.Isolate].Pop()
					continue
				}

				// We failed to find the execution on the stack, so we just ignore this
				// line and hope for the best
			}
		} else if l.LT == CallLine {
			// If there's no entry for this isolate in the active calls map,
			// create one
			if _, ok := iActiveCalls[l.Isolate]; !ok {
				iActiveCalls[l.Isolate] = nil
			}

			// If there's an active call for this isolate, we need to complete it
			// and add it to the current execution for this isolate
			if iActiveCalls[l.Isolate] != nil {
				if _, ok := iStacks[l.Isolate]; !ok {
					iStacks[l.Isolate] = make(ExecutionStack, 0)
				}

				if len(iStacks[l.Isolate]) == 0 {
					// No active executions
					// We ignore this call, since we cannot attribute it to a particular script
					continue
				}

				// Otherwise, we can add the call to the execution
				iStacks[l.Isolate][len(iStacks[l.Isolate])-1].Calls = append(
					iStacks[l.Isolate][len(iStacks[l.Isolate])-1].Calls, iActiveCalls[l.Isolate])

				// No active call anymore
				iActiveCalls[l.Isolate] = nil
			}

			// Create our new call, set as active for this isolate
			iActiveCalls[l.Isolate] = new(Call)
			iActiveCalls[l.Isolate].T = l.CallType
			iActiveCalls[l.Isolate].C = l.CallClass
			iActiveCalls[l.Isolate].F = l.CallFunc
			iActiveCalls[l.Isolate].Args = make([]Arg, 0)
		} else if l.LT == ArgLine {
			// If there's no entry for this isolate in the active calls map,
			// create one
			if _, ok := iActiveCalls[l.Isolate]; !ok {
				iActiveCalls[l.Isolate] = nil
			}

			if iActiveCalls[l.Isolate] == nil {
				// No active call for this argument, so we must ignore
				log.Error("No active call for argument")
				continue
			}
			var a Arg
			a.T = l.ArgType
			a.Val = l.ArgVal
			iActiveCalls[l.Isolate].Args = append(iActiveCalls[l.Isolate].Args, a)
		} else if l.LT == RetLine {
			if iActiveCalls[l.Isolate] == nil {
				// No active call for this return value, so we must ignore
				log.Error("No active call for return value")
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

func (es ExecutionStack) Push(e Execution) ExecutionStack {
	return append(es, e)
}

func (es ExecutionStack) Pop() (ExecutionStack, Execution, error) {
	l := len(es)
	if l == 0 {
		return es, Execution{}, errors.New("popped empty stack")
	}
	return es[:l-1], es[l-1], nil
}

func (es ExecutionStack) Peek(depth int) (Execution, error) {
	if len(es) < depth {
		return Execution{}, errors.New("stack not large enough")
	}
	return es[len(es)-depth], nil
}

func ProcessLine(s string) Line {
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
		if len(fields) != 5 {
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
