package types

import (
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/teamnsrg/mida/jstrace"
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
	NetworkTrace     *bool `json:"network_trace"`
	OpenWPMChecks    *bool `json:"open_wpm_checks"`
	BrowserCoverage  *bool `json:"browser_coverage"`
}

type OutputSettings struct {
	Path     *string `json:"path"`
	GroupID  *string `json:"group_id"`
	MongoURI *string `json:"mongo_uri,omitempty"`
}

type MIDATask struct {
	URL *string `json:"url"`

	Browser    *BrowserSettings    `json:"browser"`
	Completion *CompletionSettings `json:"completion"`
	Data       *DataSettings       `json:"data"`
	Output     *OutputSettings     `json:"output"`

	// Track how many times we will attempt this task
	MaxAttempts *int `json:"max_attempts"`

	// Integer between one and ten (inclusive), sets priority in queue, defaults to 5
	Priority *int `json:"priority,omitempty"`
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

	// Integer between one and ten (inclusive), sets priority in queue, defaults to 5
	Priority *int `json:"priority,omitempty"`
}

// Crawl Completion Conditions
type CompletionCondition int

// Single, flat struct without pointers, containing
// all info required to complete a task
type SanitizedMIDATask struct {
	Url string

	// Browser settings
	BrowserBinary     string   `json:"browser_binary"`
	UserDataDirectory string   `json:"user_data_directory"`
	BrowserFlags      []string `json:"-" bson:"-"`

	// Completion Settings
	CCond         CompletionCondition `json:"completion_condition"`
	Timeout       int                 `json:"timeout"`
	TimeAfterLoad int                 `json:"time_after_load"`

	// Data settings
	AllResources     bool `json:"all_resources"`
	AllScripts       bool `json:"all_scripts"`
	JSTrace          bool `json:"js_trace"`
	SaveRawTrace     bool `json:"save_raw_trace"`
	ResourceMetadata bool `json:"resource_metadata"`
	ScriptMetadata   bool `json:"script_metadata"`
	ResourceTree     bool `json:"resource_tree"`
	WebsocketTraffic bool `json:"websocket_traffic"`
	NetworkTrace     bool `json:"network_trace"`
	OpenWPMChecks    bool `json:"open_wpm_checks"`
	BrowserCoverage  bool `json:"browser_coverage"`

	// Output Settings
	OutputPath       string `json:"output_path"`
	GroupID          string `json:"group_id"`
	RandomIdentifier string `json:"random_identifier"`
	MongoURI         string `json:"mongo_uri,omitempty"`

	// Parameters for retrying a task if it fails to complete
	MaxAttempts      int      `json:"max_attempts"`
	CurrentAttempt   int      `json:"-"`
	TaskFailed       bool     `json:"-"`
	FailureCode      string   `json:"-"`
	PastFailureCodes []string `json:"-"`
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

	// MongoDB use only
	ID    int64  `json:"-" bson:"_id"`
	Crawl int64  `json:"-" bson:"crawl"`
	Type  string `json:"-" bson:"type"`
}

type HostInfo struct {
	HostName string `json:"host_name"`

	// Browser Info
	Browser         string `json:"browser"`
	BrowserVersion  string `json:"browser_version"`
	UserAgent       string `json:"user_agent"`
	V8Version       string `json:"v8_version"`
	DevToolsVersion string `json:"devtools_version"`
}

type RawMIDAResult struct {
	CrawlHostInfo HostInfo
	SanitizedTask SanitizedMIDATask
	Stats         TaskStats
	Requests      map[string][]network.EventRequestWillBeSent
	Responses     map[string][]network.EventResponseReceived
	Scripts       map[string]debugger.EventScriptParsed
	FrameTree     *page.FrameTree
	WebsocketData map[string]*WSConnection
}

type ResourceNode struct {
	RequestID   string          `json:"request_id"`
	FrameID     string          `json:"frame_id"`
	IsFrameRoot bool            `json:"is_frame_root"`
	Url         string          `json:"url"`
	Children    []*ResourceNode `json:"children"`
}

type Resource struct {
	Requests  []network.EventRequestWillBeSent `json:"requests"`
	Responses []network.EventResponseReceived  `json:"responses"`

	// MongoDB use only
	ID    int64  `json:"-" bson:"_id"`
	Crawl int64  `json:"-" bson:"crawl"`
	Type  string `json:"-" bson:"type"`
}

type ResourceTree struct {
	RootNode *ResourceNode   `json:"root_node"`
	Orphans  []*ResourceNode `json:"orphans"`
}

// Metadata corresponding to a single site visit
type CrawlMetadata struct {
	Task          SanitizedMIDATask `json:"task"`
	Timing        TaskTiming        `json:"timing"`
	CrawlHostInfo HostInfo          `json:"host_info"`
	Failed        bool              `json:"failed"`
	FailureCodes  []string          `json:"failure_codes"`

	NumResources     int `json:"num_resources,omitempty"`
	NumScripts       int `json:"num_scripts,omitempty"`
	NumWSConnections int `json:"num_ws_connections,omitempty"`

	// For MongoDB storage
	ID   int64  `json:"-" bson:"_id"`
	Type string `json:"-" bson:"type"`
}

type FinalMIDAResult struct {
	Metadata         *CrawlMetadata
	CrawlHostInfo    HostInfo
	ResourceMetadata map[string]Resource
	SanitizedTask    SanitizedMIDATask
	ScriptMetadata   map[string]debugger.EventScriptParsed
	Stats            TaskStats
	JSTrace          *jstrace.JSTrace
	WebsocketData    map[string]*WSConnection
	RTree            *ResourceTree
}

type TaskTiming struct {
	BeginCrawl            time.Time `json:"begin_crawl"`
	BrowserOpen           time.Time `json:"browser_open"`
	DevtoolsConnect       time.Time `json:"devtools_connect"`
	ConnectionEstablished time.Time `json:"connection_established"`
	LoadEvent             time.Time `json:"load_event"`
	DOMContentEvent       time.Time `json:"dom_content_event"`
	BrowserClose          time.Time `json:"browser_close"`
	BeginPostprocess      time.Time `json:"begin_postprocess"`
	EndPostprocess        time.Time `json:"end_postprocess"`
	BeginStorage          time.Time `json:"begin_storage"`
	EndStorage            time.Time `json:"end_storage"`
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
