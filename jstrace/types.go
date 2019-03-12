package jstrace

// A single argument (or return value) from an API call
type Arg struct {
	t   string
	val string
}

// A single API call
type call struct {
	T    string // Type
	C    string // Class
	F    string // Function
	Args []string
	Ret  Arg
}

// A single execution of a single script. A script may
// have multiple executions through callbacks
type Execution struct {
	Calls []call
}

// A single script, identified by a unique script ID
type Script struct {
	ScriptId   string
	BaseUrl    string
	Executions []Execution
}

// The trace from a single isolate. Script IDs are only
// guaranteed unique per-isolate
type Isolate struct {
	Scripts map[string]*Script
}

// A full trace, parsed and ready to be stored or processed further
type JSTrace struct {
	Isolates map[string]*Isolate
}

type LineType int

const (
	ErrorLine   LineType = -1
	UnknownLine LineType = 0
	CallLine    LineType = 1
	ArgLine     LineType = 2
	RetLine     LineType = 3
	OtherLine   LineType = 4
	ControlLine LineType = 5
)

type Line struct {
	LT         LineType
	LineNum    int
	Isolate    string
	IsRet      bool
	ArgType    string
	ArgVal     string
	CallType   string
	CallClass  string
	CallFunc   string
	BaseURL    string
	ScriptId   string
	IsCallback bool
	IsBegin    bool
}
