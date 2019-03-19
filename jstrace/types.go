package jstrace

// A single argument (or return value) from an API call
type Arg struct {
	T   string `json:"type"`
	Val string `json:"val"`
}

// A single API call
type Call struct {
	ID   int64  `json:"_id,omitempty"`
	T    string `json:"type"`
	C    string `json:"class"`
	F    string `json:"func"`
	Args []Arg  `json:"args"`
	Ret  Arg    `json:"ret"`
}

// A single execution of a single script. A script may
// have multiple executions through callbacks
type Execution struct {
	ID       int64   `json:"_id,omitempty"`
	Isolate  string  `json:"isolate"`
	ScriptId string  `json:"script_id"`
	TS       string  `json:"timestamp"`
	Calls    []*Call `json:"calls"`
}

type ExecutionStack []Execution

// A single script, identified by a unique script ID
type Script struct {
	ID         int64       `json:"_id,omitempty"`
	ScriptId   string      `json:"script_id"`
	BaseUrl    string      `json:"base_url"`
	Executions []Execution `json:"executions"`
}

// The trace from a single isolate. Script IDs are only
// guaranteed unique per-isolate
type Isolate struct {
	ID      int64              `json:"_id,omitempty"`
	Scripts map[string]*Script `json:"scripts"`
}

// A full trace, parsed and ready to be stored or processed further
type JSTrace struct {
	ID       int64               `json:"_id,omitempty"`
	Isolates map[string]*Isolate `json:"isolates"`
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
	TS         string // Timestamp
}
