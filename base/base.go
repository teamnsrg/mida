// This package contains the base/root components of MIDA. Other MIDA packages import this package, but this package
// should not depend on any other MIDA packages
package base

import (
	"encoding/json"
	"errors"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"sync"
	"time"
)

// Settings describing how MIDA will interact with a page
type InteractionSettings struct {
	LockNavigation        *bool `json:"lock_navigation"`
	BasicInteraction      *bool `json:"basic_interaction"`
	Gremlins              *bool `json:"gremlins"`
	TriggerEventListeners *bool `json:"event_listeners"`
}

// Settings describing the way in which a browser will be opened
type BrowserSettings struct {
	BrowserBinary       *string              `json:"browser_binary,omitempty"`       // The binary for the browser (e.g., "/path/to/chrome.exe")
	UserDataDirectory   *string              `json:"user_data_directory,omitempty"`  // Path to user data directory to use
	AddBrowserFlags     *[]string            `json:"add_browser_flags,omitempty"`    // Flags to be added to default browser flags
	RemoveBrowserFlags  *[]string            `json:"remove_browser_flags,omitempty"` // Flags to be removed from default browser flags
	SetBrowserFlags     *[]string            `json:"set_browser_flags,omitempty"`    // Flags to use to override default browser flags
	Extensions          *[]string            `json:"extensions,omitempty"`           // Paths to browser extensions to be used for the crawl
	InteractionSettings *InteractionSettings `json:"interaction_settings"`           // Settings describing how the browser will interact with the page
}

// Conditions under which a crawl will complete successfully
type CompletionCondition string

const (
	TimeoutOnly   CompletionCondition = "TimeoutOnly"   // Complete only when the timeout is reached
	TimeAfterLoad CompletionCondition = "TimeAfterLoad" // Wait a given number of seconds after the load event
	LoadEvent     CompletionCondition = "LoadEvent"     // Terminate crawl immediately when load event fires
)

var CompletionConditions = [...]CompletionCondition{TimeoutOnly, TimeAfterLoad, LoadEvent}

// Settings describing how a particular crawl will terminate
type CompletionSettings struct {
	CompletionCondition *CompletionCondition `json:"completion_condition"`      // Condition under which crawl will complete
	Timeout             *int                 `json:"timeout,omitempty"`         // Maximum amount of time the browser will remain open
	TimeAfterLoad       *int                 `json:"time_after_load,omitempty"` // Maximum amount of time the browser will remain open after page load
}

// Settings describing which data MIDA will capture from the crawl
type DataSettings struct {
	AllResources     *bool `json:"all_resources,omitempty"`     // Save all resource files
	AllScripts       *bool `json:"all_scripts,omitempty"`       // Save all scripts parsed by browser
	Cookies          *bool `json:"cookies,omitempty"`           // Save cookies set by page
	DOM              *bool `json:"dom,omitempty"`               // Collect JSON representation of the DOM
	ResourceMetadata *bool `json:"resource_metadata,omitempty"` // Save extensive metadata about each resource
	Screenshot       *bool `json:"screenshot,omitempty"`        // Save a screenshot from the web page
	ScriptMetadata   *bool `json:"script_metadata,omitempty"`   // Save metadata on scripts parsed by browser
	BrowserCoverage  *bool `json:"browser_coverage"`            // Whether to gather code coverage data from the browser
}

// Settings describing output of results to the local filesystem
type LocalOutputSettings struct {
	Enable *bool         `json:"enable,omitmepty"`        // Whether this storage method is enabled
	Path   *string       `json:"path,omitempty"`          // Path over the overarching results directory to be written
	DS     *DataSettings `json:"data_settings,omitempty"` // Data settings for output to local filesystem
}

// Settings describing results output via SSH/SFTP
type SftpOutputSettings struct {
	Enable         *bool         `json:"enable,omitempty"`           // Whether this storage method is enabled
	Host           *string       `json:"host,omitempty"`             // IP address or domain name of host to store to
	Port           *int          `json:"port,omitempty"`             // Port to initiate SSH/SFTP connection
	Path           *string       `json:"path,omitempty"`             // Path of the overarching results directory to be written
	UserName       *string       `json:"user_name,omitempty"`        // User name we should use for accessing the host
	PrivateKeyFile *string       `json:"private_key_file,omitempty"` // Path to the private key file we should use for accessing the host
	DS             *DataSettings `json:"data_settings,omitempty"`    // Data settings for output via SSH/SFTP
}

// An aggregation of the output settings for a task or task-set
type OutputSettings struct {
	LocalOut  *LocalOutputSettings `json:"local_output_settings,omitempty"` // Output settings for the local filesystem
	SftpOut   *SftpOutputSettings  `json:"sftp_output_settings,omitempty"`  // Output settings for the remote filesystem
	PostQueue *string              `json:"post_queue,omitempty"`            // AMQP queue in which we should put metadata for crawl once complete
}

// A raw MIDA task. This is the struct that is read from/written to file when tasks are stored as JSON.
type RawTask struct {
	URL *string `json:"url"` // The URL to be visited

	Browser    *BrowserSettings    `json:"browser_settings"`    // Settings for launching the browser
	Completion *CompletionSettings `json:"completion_settings"` // Settings for when the site visit will complete
	Data       *DataSettings       `json:"data_settings"`       // Settings for what data will be collected from the site
	Output     *OutputSettings     `json:"output_settings"`     // Settings for what/how results will be saved
}

// Internal type built from the process of sanitizing a RawTask. Should contain all the parameters needed for a crawl
// without the need to re-access the raw task. SanitizedTask should not contain information that cannot be deduced
// based on the raw task (and system parameters).
type SanitizedTask struct {
	URL string

	BrowserBinaryPath string   // Full path to the browser binary we use for the crawl
	BrowserFlags      []string // List of flags we will use when opening the browser (does not include --remote-debugging-port or similar)
	UserDataDirectory string   // Full path to the user data directory for the task

	CS  CompletionSettings  // Task completion settings for the task
	DS  DataSettings        // Data Gathering Settings for the task
	IS  InteractionSettings // Settings on how the browser will interact with the page
	OPS OutputSettings      // Output settings for the task
}

// A slice of MIDA tasks, ready to be enqueued
type TaskSet []RawTask

// A grouping of tasks for multiple URLs that may be repeated
type CompressedTaskSet struct {
	URL *[]string `json:"url"` // List of URLs to be visited

	Browser    *BrowserSettings    `json:"browser_settings"`    // Settings for launching the browser
	Completion *CompletionSettings `json:"completion_settings"` // Settings for when the site visit will complete
	Data       *DataSettings       `json:"data_settings"`       // Settings for what data will be collected from the site
	Output     *OutputSettings     `json:"output_settings"`     // Settings for what/how results will be saved

	Repeat *int `json:"repeat"` // Number of times to repeat the crawl after it finishes successfully
}

// Wrapper struct which contains a task, along with some dynamic metadata. This is an internal struct only --
// It should not be exported/stored.
type TaskWrapper struct {
	RawTask       RawTask       // A pointer to a MIDA task
	SanitizedTask SanitizedTask // A sanitized MIDA task

	UUID    uuid.UUID
	TempDir string // Temporary directory where results are stored. Can be the same as the UserDataDir in some cases.

	// Dynamic fields
	Log     *logrus.Logger
	LogFile *os.File
}

// TaskTiming contains timing data for the processing of a particular task
type TaskTiming struct {
	BrowserOpen           time.Time `json:"browser_open"`
	ConnectionEstablished time.Time `json:"connection_established"`
	LoadEvent             time.Time `json:"load_event"`
	BrowserClose          time.Time `json:"browser_close"`
	BeginPostprocess      time.Time `json:"begin_postprocess"`
	EndPostprocess        time.Time `json:"end_postprocess"`
	BeginStorage          time.Time `json:"begin_storage"`
	EndStorage            time.Time `json:"-"`
}

// Statistics gathered about a specific task
type TaskSummary struct {
	NavURL string `json:"nav_url"`
	UUID   string `json:"uuid"`

	Success       bool   `json:"success"`                  // True if the task did not fail
	FailureReason string `json:"failure_reason,omitempty"` // Holds the failure code for the task

	TaskWrapper *TaskWrapper `json:"-"`            // Wrapper containing the full task
	TaskTiming  TaskTiming   `json:"task_timing"`  // Timing data for the task
	CrawlerInfo CrawlerInfo  `json:"crawler_info"` // Information about the infrastructure used to visit the site

	OutputHost string `json:"output_host,omitempty"` // Host to which results were stored via SFTP
	OutputPath string `json:"output_path,omitempty"` // Path to the results of the crawl on the applicable host (after storage)

	NumResources int `json:"num_resources"` // Number of resources the browser loaded

	NavHistory []page.NavigationEntry `json:"nav_history"`
}

// Information about the infrastructure used to perform the crawl
type CrawlerInfo struct {
	Browser        string `json:"browser"`         // Name of the browser itself
	BrowserVersion string `json:"browser_version"` // Version of the browser we are using
	UserAgent      string `json:"user_agent"`      // User agent we are using
	JSVersion      string `json:"js_version"`      // JS version
}

type DevToolsNetworkRawData struct {
	RequestWillBeSent map[string][]*network.EventRequestWillBeSent
	ResponseReceived  map[string]*network.EventResponseReceived
}

type DevToolsScriptRawData []*debugger.EventScriptParsed

type DevToolsRawData struct {
	Network DevToolsNetworkRawData
	Cookies []*network.Cookie
	DOM     *cdp.Node
	Scripts DevToolsScriptRawData
}

// The results MIDA gathers before they are post-processed
type RawResult struct {
	TaskSummary TaskSummary     // Summary information about the task, not necessarily complete in RawResult
	DevTools    DevToolsRawData // Struct Containing Raw Data gathered from a DevTools site visit
	sync.Mutex
}

type DTResource struct {
	Requests []*network.EventRequestWillBeSent `json:"requests"`  // All requests sent for this particular request
	Response *network.EventResponseReceived    `json:"responses"` // All responses received for this particular request
}

type FinalResult struct {
	Summary            TaskSummary                            `json:"stats"`   // Statistics on timing and resource usage for the crawl
	DTCookies          []*network.Cookie                      `json:"cookies"` // Cookies collected from DevTools protocol
	DTDOM              *cdp.Node                              `json:"dom"`
	DTResourceMetadata map[string]DTResource                  `json:"resource_metadata"` // Metadata on each resource loaded
	DTScriptMetadata   map[string]*debugger.EventScriptParsed `json:"script_metadata"`   // Metadata on each script parsed
}

func AllocateNewCompressedTaskSet() *CompressedTaskSet {
	var cts = new(CompressedTaskSet)
	cts.URL = new([]string)
	cts.Browser = AllocateNewBrowserSettings()
	cts.Completion = AllocateNewCompletionSettings()
	cts.Data = AllocateNewDataSettings()
	cts.Output = AllocateNewOutputSettings()
	cts.Repeat = new(int)
	return cts
}

// AllocateNewTask allocates a new RawTask struct, initializing everything to zero values
func AllocateNewTask() *RawTask {
	var task = new(RawTask)
	task.URL = new(string)

	task.Browser = AllocateNewBrowserSettings()
	task.Completion = AllocateNewCompletionSettings()
	task.Data = AllocateNewDataSettings()
	task.Output = AllocateNewOutputSettings()

	return task
}

// AllocateNewInteractionSettings allocates a new InteractionSettings struct specifying if/how the
// browser will interact with pages it visits as part of the task
func AllocateNewInteractionSettings() *InteractionSettings {
	var is = new(InteractionSettings)
	is.LockNavigation = new(bool)
	is.BasicInteraction = new(bool)
	is.TriggerEventListeners = new(bool)
	is.Gremlins = new(bool)

	*is.LockNavigation = DefaultNavLockAfterLoad
	*is.BasicInteraction = DefaultBasicInteraction
	*is.Gremlins = DefaultGremlins
	*is.TriggerEventListeners = DefaultTriggerEventListeners

	return is
}

// AllocateNewBrowserSettings allocates a new BrowserSettings struct, initializing everything to zero values
func AllocateNewBrowserSettings() *BrowserSettings {
	var bs = new(BrowserSettings)
	bs.BrowserBinary = new(string)
	bs.AddBrowserFlags = new([]string)
	bs.RemoveBrowserFlags = new([]string)
	bs.SetBrowserFlags = new([]string)
	bs.Extensions = new([]string)
	bs.UserDataDirectory = new(string)
	bs.InteractionSettings = AllocateNewInteractionSettings()

	return bs
}

// AllocateNewCompletionSettings allocates a new CompletionSettings struct, initializing everything to zero values
func AllocateNewCompletionSettings() *CompletionSettings {
	var cs = new(CompletionSettings)
	cs.TimeAfterLoad = new(int)
	cs.Timeout = new(int)
	cs.CompletionCondition = new(CompletionCondition)

	return cs
}

// AllocateNewDataSettings allocates a new DataSettings struct, initializing everything to zero values
func AllocateNewDataSettings() *DataSettings {
	var ds = new(DataSettings)
	ds.AllResources = new(bool)
	ds.AllScripts = new(bool)
	ds.Cookies = new(bool)
	ds.DOM = new(bool)
	ds.ResourceMetadata = new(bool)
	ds.Screenshot = new(bool)
	ds.ScriptMetadata = new(bool)
	ds.BrowserCoverage = new(bool)

	return ds
}

// AllocateNewOutputSettings allocates a new OutputSettings struct, initializing everything to zero values
func AllocateNewOutputSettings() *OutputSettings {
	var ops = new(OutputSettings)
	ops.LocalOut = AllocateNewLocalOutputSettings()
	ops.SftpOut = AllocateNewSftpOutputSettings()
	ops.PostQueue = new(string)

	return ops
}

func AllocateNewLocalOutputSettings() *LocalOutputSettings {
	var los = new(LocalOutputSettings)
	los.Enable = new(bool)
	los.Path = new(string)
	los.DS = AllocateNewDataSettings()

	return los
}

func AllocateNewSftpOutputSettings() *SftpOutputSettings {
	var sos = new(SftpOutputSettings)
	sos.Enable = new(bool)
	sos.UserName = new(string)
	sos.Host = new(string)
	sos.Port = new(int)
	sos.Path = new(string)
	sos.PrivateKeyFile = new(string)
	sos.DS = AllocateNewDataSettings()

	return sos
}

// ReadTasksFromFile is a wrapper function that reads single tasks, full task sets,
// or compressed task sets from file.
func ReadTasksFromFile(filename string) ([]RawTask, error) {
	tasks := make(TaskSet, 0)

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return tasks, errors.New("failed to read task file: " + filename)
	}

	tasks, err = ReadTasksFromBytes(data)
	if err != nil {
		return tasks, err
	}

	return tasks, nil
}

// WriteTaskSliceToFile takes a RawTask slice and writes it out as a JSON file to a given filename.
func WriteTaskSliceToFile(tasks []RawTask, filename string) error {
	taskBytes, err := WriteTaskSliceToBytes(tasks)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filename, taskBytes, 0644)
	return err
}

// WriteCompressedTaskSetToFile takes a CompressedTaskSet and writes a JSON representation
// of it out to a file
func WriteCompressedTaskSetToFile(cts *CompressedTaskSet, filename string, overwrite bool) error {
	_, err := os.Stat(filename)
	if err == nil && !overwrite {
		return errors.New("use '-x' to overwrite existing task file")
	}

	// Write output JSON file
	outData, err := json.Marshal(cts)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filename, outData, 0644)
	if err != nil {
		return errors.New("failed to write task file")
	}

	return nil
}

// ExpandCompressedTaskSet takes a CompressedTaskSet object and converts it into a slice
// of regular Tasks.
func ExpandCompressedTaskSet(ts CompressedTaskSet) []RawTask {
	var rawTasks []RawTask

	repeats := 1
	if ts.Repeat != nil && *ts.Repeat > 0 {
		repeats = *ts.Repeat
	}
	for i := 0; i < repeats; i += 1 {
		for _, singleUrl := range *ts.URL {
			var url = singleUrl
			newTask := RawTask{
				URL:        &url,
				Browser:    ts.Browser,
				Completion: ts.Completion,
				Data:       ts.Data,
				Output:     ts.Output,
			}
			rawTasks = append(rawTasks, newTask)
		}
	}
	return rawTasks
}

// ReadTasksFromBytes reads in tasks from a byte array. It will read them whether they
// are formatted as individual tasks or as a CompressedTaskSet.
func ReadTasksFromBytes(data []byte) ([]RawTask, error) {
	tasks := make(TaskSet, 0)
	err := json.Unmarshal(data, &tasks)
	if err == nil {
		return tasks, nil
	}

	var singleTask RawTask
	err = json.Unmarshal(data, &singleTask)
	if err == nil {
		return append(tasks, singleTask), nil
	}

	compressedTaskSet := CompressedTaskSet{}
	err = json.Unmarshal(data, &compressedTaskSet)
	if err != nil {
		return tasks, errors.New("failed to unmarshal tasks: [ " + err.Error() + " ]")
	}

	if compressedTaskSet.URL == nil || len(*compressedTaskSet.URL) == 0 {
		return tasks, errors.New("no URLs given in task set")
	}
	tasks = ExpandCompressedTaskSet(compressedTaskSet)

	return tasks, nil

}

// WriteTaskSliceToBytes takes a slice of tasks and converts it to corresponding JSON bytes to transfer somewhere.
func WriteTaskSliceToBytes(tasks []RawTask) ([]byte, error) {
	taskBytes, err := json.Marshal(tasks)
	if err != nil {
		return nil, err
	}

	return taskBytes, nil
}

// WriteCompressedTaskSetToBytes takes a CompressedTaskSet and converts it to corresponding JSON bytes to transfer somewhere.
func WriteCompressedTaskSetToBytes(tasks CompressedTaskSet) ([]byte, error) {
	taskBytes, err := json.Marshal(tasks)
	if err != nil {
		return nil, err
	}

	return taskBytes, nil
}
