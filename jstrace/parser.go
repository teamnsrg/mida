package jstrace

import (
	"bufio"
	"github.com/prometheus/common/log"
	"os"
	"strings"
)

func ParseTraceFromFile(fname string) (*JSTrace, error) {

	var trace JSTrace
	file, err := os.Open(fname)
	if err != nil {
		return &trace, err
	}
	r := bufio.NewReader(file)

	for {
		// Get the next lineBytes from our trace
		isPrefix := true
		var lineBytes, tmpLine []byte
		for isPrefix && err == nil {
			tmpLine, isPrefix, err = r.ReadLine()
			lineBytes = append(lineBytes, tmpLine...)
		}
		if err != nil {
			log.Info()
			break
		}

		l := ProcessLine(string(lineBytes))

		if l.LT == OtherLine {
			log.Info(string(lineBytes))
		} else if l.LT == ErrorLine {
			log.Info(string(lineBytes))
		}

	}

	return &trace, nil
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
		l.BaseURL = fields[4][2 : len(fields[4])-2]

	} else if fields[3] == "ENDCALL" {
		if len(fields) != 5 {
			l.LT = ErrorLine
			return l
		}

		l.LT = ControlLine
		l.IsBegin = false
		l.IsCallback = false

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
