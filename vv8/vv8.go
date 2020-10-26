package vv8

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

type CallType int

const (
	Get      CallType = iota
	Set      CallType = iota
	New      CallType = iota
	Function CallType = iota
)

type Call struct {
	Id                  string
	CallType            CallType
	FunctionName        string   // Function, New
	OwningObject        string   // Function, Get, Set
	PositionalArguments []string // Function, New
	PropertyName        string   // Get, Set
	NewValue            string   // Set
}

type ScriptID string

type Script struct {
	ScriptID ScriptID
	Name     string
	Source   string // Check on hashing
	Calls    []Call
}

type Isolate map[ScriptID]*Script
type IsolateAddress string

// Unescaped find
func unescapedSplit(s string, delim byte) []string {
	var splat []string
	prevIdx := 0
	for i := 1; i < len(s); i++ {
		if s[i] == delim && s[i-1] != '\\' {
			splat = append(splat, s[prevIdx:i])
			prevIdx = i + 1
		}
	}
	splat = append(splat, s[prevIdx:])
	return splat
}

func readFullLine(reader *bufio.Reader) (string, error) {
	line := ""
	continueRead := true
	for continueRead {
		segment, isPrefix, err := reader.ReadLine()
		if err == io.EOF {
			return line, err
		}
		line += string(segment)
		continueRead = isPrefix
	}

	return line, nil
}

func ProcessLogFiles(filenames []string) (map[IsolateAddress]Isolate, error) {
	// Open file
	isolateMap := make(map[IsolateAddress]Isolate)

	for _, filename := range filenames {

		file, err := os.Open(filename)
		if err != nil {
			return isolateMap, err
		}
		defer file.Close()

		// Initialize map
		curIsolateID := IsolateAddress("")
		curScriptID := ScriptID("?")

		reader := bufio.NewReader(file)
		for {
			line, err := readFullLine(reader)
			if err != nil {
				break
			}

			splat := unescapedSplit(line[1:], ':')
			switch line[0] {
			case '~':
				newIsolateID := IsolateAddress(line[1:])
				isolateMap[newIsolateID] = make(Isolate)
				curIsolateID = newIsolateID
			case '@':
			case '$':
				scriptID := ScriptID(splat[0])
				if _, ok := isolateMap[curIsolateID][scriptID]; !ok {
					isolateMap[curIsolateID][scriptID] = &Script{
						Calls: []Call{},
					}
				}
				script := isolateMap[curIsolateID][scriptID]
				script.ScriptID = scriptID
				script.Name = splat[1]
				script.Source = splat[2]
			case '!':
				curScriptID = ScriptID(line[1:])
				if _, ok := isolateMap[curIsolateID][curScriptID]; !ok {
					isolateMap[curIsolateID][curScriptID] = &Script{
						Calls: []Call{},
					}
				}
			case 'c':
				call := Call{
					Id:                  splat[0],
					CallType:            Function,
					FunctionName:        splat[1],
					OwningObject:        splat[2],
					PositionalArguments: splat[3:],
				}
				isolateMap[curIsolateID][curScriptID].Calls = append(isolateMap[curIsolateID][curScriptID].Calls, call)

			case 'n':
				call := Call{
					Id:                  splat[0],
					CallType:            New,
					FunctionName:        splat[1],
					PositionalArguments: splat[2:],
				}
				isolateMap[curIsolateID][curScriptID].Calls = append(isolateMap[curIsolateID][curScriptID].Calls, call)

			case 'g':
				call := Call{
					Id:           splat[0],
					CallType:     Get,
					OwningObject: splat[1],
					PropertyName: splat[2],
				}
				isolateMap[curIsolateID][curScriptID].Calls = append(isolateMap[curIsolateID][curScriptID].Calls, call)

			case 's':
				call := Call{
					Id:           splat[0],
					CallType:     Set,
					OwningObject: splat[1],
					PropertyName: splat[2],
					NewValue:     splat[3],
				}
				isolateMap[curIsolateID][curScriptID].Calls = append(isolateMap[curIsolateID][curScriptID].Calls, call)

			default:
				fmt.Printf("Unknown line: %s\n", line)
			}
		}

	}

	return isolateMap, nil
}
