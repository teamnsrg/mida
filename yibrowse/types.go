package yibrowse

// A single argument (or return value) from an API call
type Arg struct {
	T   string `json:"type"`
	Val string `json:"val"`
}

// A single API call
type Call struct {
	T        string `json:"t"`
	C        string `json:"c"`
	F        string `json:"f"`
	Args     []Arg  `json:"a"`
	Ret      Arg    `json:"r"`
	ScriptId string `json:"-"`
}

// A single script, identified by a unique script ID
type Script struct {
	ScriptId string `json:"script_id"`
	Url      string `json:"base_url"`
	Calls    []Call `json:"calls"`
	SHA1     string `json:"sha1"`
	Length   int    `json:"length"`

}

// The trace from a single isolate. Script IDs are only
// guaranteed unique per-isolate
type Isolate struct {
	Scripts map[string]*Script `json:"scripts"`

	// MongoDB-use only fields
	ID       int64   `json:"-"`
	Parent   int64   `json:"-"`
	Children []int64 `json:"-"`
}

// A full trace, parsed and ready to be stored or processed further
type RawJSTrace struct {
	Isolates map[string]*Isolate `json:"isolates,omitempty"`

	// Scripts for which we saw calls but never saw an initial declaration
	// We store this for use in repairing the trace using script metadata
	// UnknownScripts[isolate][scriptId] = true
	UnknownScripts map[string]map[string]bool `json:"-"`

	// Parsing data
	IgnoredCalls int `json:"ignored_calls"`
	StoredCalls  int `json:"stored_calls"`
}

type CleanedJSTrace struct {
	Url     string             `json:"url,omitempty"`
	Scripts map[string]*Script `json:"scripts,omitempty"`
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
