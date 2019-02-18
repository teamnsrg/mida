package main

import (
	"encoding/json"
	"errors"
	"github.com/teamnsrg/chromedp/runner"
	"io/ioutil"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
)

type BrowserSettings struct {
	BrowserBinary      string   `json:"browser_binary"`
	UserDataDirectory  string   `json:"user_data_directory"`
	AddBrowserFlags    []string `json:"add_browser_flags"`
	RemoveBrowserFlags []string `json:"remove_browser_flags"`
	SetBrowserFlags    []string `json:"set_browser_flags"`
	Extensions         []string `json:"extensions"`
}

type CompletionSettings struct {
	CompletionCondition string `json:"completion_condition"`
	Timeout             int    `json:"timeout"`
}

type DataSettings struct {
	AllFiles     bool `json:"all_files"`
	AllScripts   bool `json:"all_scripts"`
	JSTrace      bool `json:"js_trace"`
	Screenshot   bool `json:"screenshot"`
	Cookies      bool `json:"cookies"`
	Certificates bool `json:"certificates"`
	CodeCoverage bool `json:"code_coverage"`
}

type OutputSettings struct {
	Path    string `json:"path"`
	GroupID string `json:"group_id"`
}

type MIDATask struct {
	URL string `json:"url"`

	Browser    BrowserSettings    `json:"browser"`
	Completion CompletionSettings `json:"completion"`
	Data       DataSettings       `json:"data"`
	Output     OutputSettings     `json:"output"`

	// Track how many times we will attempt this task
	MaxAttempts int `json:"max_attempts"`
}

type MIDATaskSet []MIDATask

type CompressedMIDATaskSet struct {
	URL []string `json:"url"`

	Browser    BrowserSettings    `json:"browser"`
	Completion CompletionSettings `json:"completion"`
	Data       DataSettings       `json:"data"`
	Output     OutputSettings     `json:"output"`

	// Track how many times we will attempt this task
	MaxAttempts int `json:"max_attempts"`
}

type SanitizedMIDATask struct {
	Url string

	// Browser settings
	BrowserBinary     string
	UserDataDirectory string
	BrowserFlags      []runner.CommandLineOption

	// Completion Settings
	CCond   CompletionCondition
	Timeout int

	// Data settings
	AllFiles     bool
	AllScripts   bool
	JSTrace      bool
	Screenshot   bool
	Cookies      bool
	Certificates bool
	CodeCoverage bool

	// Output Settings
	OutputPath string
	GroupID    string

	// Parameters for retrying a task if it fails to complete
	MaxAttempts    int
	CurrentAttempt int
	TaskFailed     bool
}

// Wrapper function that reads single tasks, full task sets,
// or compressed task sets from file
func ReadTasksFromFile(fName string) ([]MIDATask, error) {
	tasks := make(MIDATaskSet, 0)

	data, err := ioutil.ReadFile(fName)
	if err != nil {
		return tasks, err
	}

	err = json.Unmarshal(data, &tasks)
	if err == nil {
		Log.Debug("Parsed MIDATaskSet from file")
		return tasks, nil
	}

	singleTask := MIDATask{}
	err = json.Unmarshal(data, &singleTask)
	if err == nil {
		Log.Debug("Parsed single MIDATask from file")
		return append(tasks, singleTask), nil
	}

	compressedTaskSet := CompressedMIDATaskSet{}
	err = json.Unmarshal(data, &compressedTaskSet)
	if err == nil {
		// Decompress by iterating through URL
		for _, v := range compressedTaskSet.URL {
			newTask := MIDATask{
				URL:         v,
				Browser:     compressedTaskSet.Browser,
				Completion:  compressedTaskSet.Completion,
				Data:        compressedTaskSet.Data,
				Output:      compressedTaskSet.Output,
				MaxAttempts: compressedTaskSet.MaxAttempts,
			}
			tasks = append(tasks, newTask)
		}

		Log.Debug("Parsed CompressedMIDATaskSet from file")
		return tasks, nil

	}

	return tasks, errors.New("failed to unmarshal task file")
}

// Takes raw tasks from input channel and produces sanitized tasks for the output channel
func SanitizeTasks(rawTaskChan <-chan MIDATask, sanitizedTaskChan chan<- SanitizedMIDATask, mConfig MIDAConfig, pipelineWG *sync.WaitGroup) {
	for r := range rawTaskChan {
		st, err := SanitizeTask(r)
		if err != nil {
			Log.Fatal(err)
		}
		pipelineWG.Add(1)

		sanitizedTaskChan <- st
	}

	// Wait until the pipeline is clear before we close the sanitized task channel,
	// which will cause MIDA to shutdown
	pipelineWG.Wait()
	close(sanitizedTaskChan)
}

// Retrieves raw tasks, either from a queue or a file
func TaskIntake(rtc chan<- MIDATask, mConfig MIDAConfig) {
	if mConfig.UseAMPQForTasks {
		Log.Info("AMPQ not yet supported")
	} else {
		rawTasks, err := ReadTasksFromFile(mConfig.TaskLocation)
		if err != nil {
			Log.Fatal(err)
		}

		// Put raw tasks in the channel
		for _, rt := range rawTasks {
			rtc <- rt
		}
	}

	// Start the process of closing up the pipeline and exit
	close(rtc)
}

// Run a series of checks on a raw task to ensure it is valid for a crawl.
// Put the task in a new format ("SanitizedMIDATask") which is used for processing.
func SanitizeTask(t MIDATask) (SanitizedMIDATask, error) {

	var st SanitizedMIDATask

	///// BEGIN SANITIZE AND BUILD URL
	if t.URL == "" {
		return st, errors.New("no URL to crawl given in task")
	}

	// Do what we can to ensure a valid URL
	u, err := url.ParseRequestURI(t.URL)
	if err != nil {
		if !strings.Contains(t.URL, "://") {
			u, err = url.ParseRequestURI(DefaultProtocolPrefix + t.URL)
			if err != nil {
				Log.Fatal("Bad URL in task: ", t.URL)
			}
		} else {
			Log.Fatal("Bad URL in task: ", t.URL)
		}
	}

	st.Url = u.String()

	///// BEGIN SANITIZE TASK COMPLETION SETTINGS

	if t.Completion.CompletionCondition == "CompleteOnTimeoutOnly" {
		st.CCond = CompleteOnTimeoutOnly
	} else if t.Completion.CompletionCondition == "CompleteOnLoadEvent" {
		st.CCond = CompleteOnLoadEvent
	} else if t.Completion.CompletionCondition == "CompleteOnTimeoutAfterLoad" {
		st.CCond = CompleteOnTimeoutAfterLoad
	} else if t.Completion.CompletionCondition == "" {
		st.CCond = DefaultCompletionCondition
	} else {
		return st, errors.New("invalid completion condition value given")
	}

	// If we don't get a value for timeout (or get zero), and we NEED that
	// value, just set it to the default
	if t.Completion.Timeout == 0 && st.CCond != CompleteOnLoadEvent {
		Log.Debug("No timeout value given in task. Setting to default value of ", DefaultTimeout)
		st.Timeout = DefaultTimeout
	} else {
		st.Timeout = t.Completion.Timeout
	}

	///// END SANITIZE TASK COMPLETION SETTINGS /////

	///// BEGIN SANITIZE BROWSER PARAMETERS /////

	// Make sure we have a valid browser binary path, or select a default one
	if t.Browser.BrowserBinary == "" {
		// Set to system default.
		if runtime.GOOS == "darwin" {
			if _, err := os.Stat(DefaultOSXChromiumPath); err == nil {
				st.BrowserBinary = DefaultOSXChromiumPath
			} else if _, err := os.Stat(DefaultOSXChromePath); err == nil {
				st.BrowserBinary = DefaultOSXChromePath
			}
		} else if runtime.GOOS == "linux" {
			if _, err := os.Stat(DefaultLinuxChromiumPath); err == nil {
				st.BrowserBinary = DefaultLinuxChromiumPath
			} else if _, err := os.Stat(DefaultLinuxChromePath); err == nil {
				st.BrowserBinary = DefaultLinuxChromePath
			}
		} else {
			Log.Fatal("Failed to locate Chrome or Chromium on your system")
		}
	} else {
		// Validate that this binary exists
		if _, err := os.Stat(t.Browser.BrowserBinary); err != nil {
			// We won't crawl if the user specified a browser that does not exist
			Log.Fatal("No such browser binary: ", t.Browser.BrowserBinary)
		} else {
			st.BrowserBinary = t.Browser.BrowserBinary
		}
	}

	// Sanitize user data directory to use
	if t.Browser.UserDataDirectory == "" {
		st.UserDataDirectory = ""
	} else {
		// Chrome will create any directories required
		st.UserDataDirectory = t.Browser.UserDataDirectory
	}

	// Sanitize browser flags/command line options
	if len(t.Browser.SetBrowserFlags) != 0 {
		if len(t.Browser.AddBrowserFlags) != 0 {
			Log.Warn("SetBrowserFlags option is overriding AddBrowserFlags option")
		}
		if len(t.Browser.RemoveBrowserFlags) != 0 {
			Log.Warn("SetBrowserFlags option is overriding RemoveBrowserFlags option")
		}

		for _, flag := range t.Browser.SetBrowserFlags {
			ff, err := FormatFlag(flag)
			if err != nil {
				Log.Warn(err)
			} else {
				st.BrowserFlags = append(st.BrowserFlags, ff)
			}

		}
	} else {
		// Add flags, checking to see that they have not been removed
		for _, flag := range append(DefaultBrowserFlags, t.Browser.AddBrowserFlags...) {
			if IsRemoved(t.Browser.RemoveBrowserFlags, flag) {
				continue
			}
			ff, err := FormatFlag(flag)
			if err != nil {
				Log.Warn(err)
			} else {
				st.BrowserFlags = append(st.BrowserFlags, ff)
			}
		}
	}

	///// END SANITIZE BROWSER PARAMETERS /////

	///// BEGIN SANITIZE DATA GATHERING PARAMETERS /////

	// For now, these are just bools and we will just copy them
	st.AllFiles = t.Data.AllFiles
	st.AllScripts = t.Data.AllScripts
	st.JSTrace = t.Data.JSTrace
	st.Screenshot = t.Data.Screenshot
	st.Cookies = t.Data.Cookies
	st.Certificates = t.Data.Certificates
	st.CodeCoverage = t.Data.CodeCoverage

	///// END SANITIZE DATA GATHERING PARAMETERS /////

	if t.MaxAttempts <= 1 {
		st.MaxAttempts = 1
	} else if t.MaxAttempts > DefaultMaximumTaskAttempts {
		Log.Fatal("A task may not have more than ", DefaultMaximumTaskAttempts, " attempts")
	} else {
		st.MaxAttempts = t.MaxAttempts
	}

	st.CurrentAttempt = 1

	return st, nil
}

// Check to see if a flag has been removed by the RemoveBrowserFlags setting
func IsRemoved(toRemove []string, candidate string) bool {
	for _, x := range toRemove {
		if candidate == x {
			return true
		}
	}

	return false
}

// Takes a variety of possible flag formats and puts them
// in a format that chromedp understands (key/value)
func FormatFlag(f string) (runner.CommandLineOption, error) {
	if strings.HasPrefix(f, "--") {
		f = f[2:]
	}

	parts := strings.Split(f, "=")
	if len(parts) == 1 {
		return runner.Flag(parts[0], true), nil
	} else if len(parts) == 2 {
		return runner.Flag(parts[0], parts[1]), nil
	} else {
		return runner.Flag("", ""), errors.New("Invalid flag: " + f)
	}

}
