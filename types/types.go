package types

import (
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/pmurley/mida/jstrace"
	"github.com/teamnsrg/chromedp/runner"
	"golang.org/x/crypto/ssh"
	"sync"
	"time"
)

type BrowserSettings struct {
	BrowserBinary      *string   `json:"browser_binary"`
	UserDataDirectory  *string   `json:"user_data_directory"`
	AddBrowserFlags    *[]string `json:"add_browser_flags"`
	RemoveBrowserFlags *[]string `json:"remove_browser_flags"`
	SetBrowserFlags    *[]string `json:"set_browser_flags"`
	Extensions         *[]string `json:"extensions"`
}

type CompletionSettings struct {
	CompletionCondition *string `json:"completion_condition"`
	Timeout             *int    `json:"timeout"`
	TimeAfterLoad       *int    `json:"time_after_load"`
}

type DataSettings struct {
	AllResources     *bool `json:"all_files"`
	AllScripts       *bool `json:"all_scripts"`
	JSTrace          *bool `json:"js_trace"`
	SaveRawTrace     *bool `json:"save_raw_trace"`
	ResourceMetadata *bool `json:"resource_metadata"`
	ScriptMetadata   *bool `json:"script_metadata"`
	ResourceTree     *bool `json:"resource_tree"`
	WebsocketTraffic *bool `json:"websocket_traffic"`
}

type OutputSettings struct {
	Path    *string `json:"path"`
	GroupID *string `json:"group_id"`
}

type MIDATask struct {
	URL *string `json:"url"`

	Browser    *BrowserSettings    `json:"browser"`
	Completion *CompletionSettings `json:"completion"`
	Data       *DataSettings       `json:"data"`
	Output     *OutputSettings     `json:"output"`

	// Track how many times we will attempt this task
	MaxAttempts *int `json:"max_attempts"`
}

type MIDATaskSet []MIDATask

type CompressedMIDATaskSet struct {
	URL *[]string `json:"url"`

	Browser    *BrowserSettings    `json:"browser"`
	Completion *CompletionSettings `json:"completion"`
	Data       *DataSettings       `json:"data"`
	Output     *OutputSettings     `json:"output"`

	// Track how many times we will attempt this task
	MaxAttempts *int `json:"max_attempts"`
}

// Crawl Completion Conditions
type CompletionCondition int

// Single, flat struct without pointers, containing
// all info required to complete a task
type SanitizedMIDATask struct {
	Url string

	// Browser settings
	BrowserBinary     string
	UserDataDirectory string
	BrowserFlags      []runner.CommandLineOption

	// Completion Settings
	CCond         CompletionCondition
	Timeout       int
	TimeAfterLoad int

	// Data settings
	AllResources     bool
	AllScripts       bool
	JSTrace          bool
	SaveRawTrace     bool
	ResourceMetadata bool
	ScriptMetadata   bool
	ResourceTree     bool
	WebsocketTraffic bool

	// Output Settings
	OutputPath       string
	GroupID          string // For identifying experiments
	RandomIdentifier string // Randomly generated task identifier

	// Parameters for retrying a task if it fails to complete
	MaxAttempts      int
	CurrentAttempt   int
	TaskFailed       bool   // Nothing else should be done on the task once this flag is set
	FailureCode      string // Should be appended whenever a task is set to fail
	PastFailureCodes []string
}

type WSConnection struct {
	Url                string                                             `json:"url"`
	Initiator          *network.Initiator                                 `json:"initiator"`
	HandshakeRequests  []*network.EventWebSocketWillSendHandshakeRequest  `json:"handshake_requests"`
	HandshakeResponses []*network.EventWebSocketHandshakeResponseReceived `json:"handshake_responses"`
	FramesSent         []*network.EventWebSocketFrameSent                 `json:"frames_sent"`
	FramesReceived     []*network.EventWebSocketFrameReceived             `json:"frames_received"`
	FrameErrors        []*network.EventWebSocketFrameError                `json:"frame_errors"`
	TSOpen             string                                             `json:"ts_open"`
	TSClose            string                                             `json:"ts_close"`
}

type RawMIDAResult struct {
	SanitizedTask SanitizedMIDATask
	Stats         TaskStats
	Requests      map[string][]network.EventRequestWillBeSent
	Responses     map[string][]network.EventResponseReceived
	Scripts       map[string]debugger.EventScriptParsed
	FrameTree     *page.FrameTree
	WebsocketData map[string]*WSConnection
}

type ResourceNode struct {
	RequestID   string
	FrameID     string
	IsFrameRoot bool
	Url         string
	Parent      *ResourceNode
	Children    []*ResourceNode
}

type Resource struct {
	Requests  []network.EventRequestWillBeSent `json:"requests"`
	Responses []network.EventResponseReceived  `json:"responses"`
}

type FinalMIDAResult struct {
	ResourceMetadata map[string]Resource
	SanitizedTask    SanitizedMIDATask
	ScriptMetadata   map[string]debugger.EventScriptParsed
	Stats            TaskStats
	JSTrace          *jstrace.JSTrace
	WebsocketData    map[string]*WSConnection
}

type TaskTiming struct {
	BeginCrawl            time.Time
	BrowserOpen           time.Time
	DevtoolsConnect       time.Time
	ConnectionEstablished time.Time
	LoadEvent             time.Time
	DOMContentEvent       time.Time
	BrowserClose          time.Time
	EndCrawl              time.Time
	BeginPostprocess      time.Time
	EndPostprocess        time.Time
	BeginStorage          time.Time
	EndStorage            time.Time
}

// Statistics from the execution of a single task, used for monitoring
// the performance of MIDA through Prometheus/Grafana
type TaskStats struct {
	///// GENERAL TASK METRICS /////
	TaskSucceeded bool
	SanitizedTask SanitizedMIDATask

	///// TIMING METRICS /////
	Timing TaskTiming

	///// RESULTS METRICS /////
	RawJSTraceSize uint // Size of raw JS trace (Log from browser) in bytes
}

// Holds information about an SSH session to another host,
// used for storing results
type SSHConn struct {
	sync.Mutex
	Client *ssh.Client
}
