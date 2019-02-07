package main

import (
	"encoding/json"
	"errors"
	log "github.com/sirupsen/logrus"
	"github.com/teamnsrg/chromedp/runner"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type OutputSettings struct {
	SaveToLocalFS  bool   `json:"local"`
	SaveToRemoteFS bool   `json:"remote_fs"`
	LocalPath      string `json:"local_path"`
	RemotePath     string `json:"remote_path"`
}

type CompletionSettings struct {
	CompletionCondition string `json:"completion_condition"`
	Timeout             int    `json:"timeout"`
}

type BrowserSettings struct {
	BrowserBinary      string   `json:"browser_binary"`
	UserDataDirectory  string   `json:"user_data_directory"`
	AddBrowserFlags    []string `json:"add_browser_flags"`
	RemoveBrowserFlags []string `json:"remove_browser_flags"`
	SetBrowserFlags    []string `json:"set_browser_flags"`
}

type RawMIDATask struct {
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
	URL      string `json:"url"`

	Browser    BrowserSettings    `json:"browser"`
	Output     OutputSettings     `json:"output"`
	Completion CompletionSettings `json:"completion"`

	// Data gathering options
	AllFiles     bool `json:"all_files"`
	AllScripts   bool `json:"all_scripts"`
	JSTrace      bool `json:"js_trace"`
	Screenshot   bool `json:"screenshot"`
	Cookies      bool `json:"cookies"`
	Certificates bool `json:"certificates"`
	CodeCoverage bool `json:"code_coverage"`

	// Track how many times we will attempt this task
	MaxAttempts int `json:"max_attempts"`
}

type SanitizedMIDATask struct {
	Url string

	BrowserBinary     string
	UserDataDirectory string
	BrowserFlags      []runner.CommandLineOption

	LocalOutputPath  string
	RemoteOutputPath string

	CCond   CompletionCondition
	Timeout int

	AllFiles     bool
	AllScripts   bool
	JSTrace      bool
	Screenshot   bool
	Cookies      bool
	Certificates bool
	CodeCoverage bool

	// Parameters for retrying a task if it fails to complete
	MaxAttempts    int
	CurrentAttempt int
	TaskFailed     bool
}

// Wrapper function that basically just unmarshals a JSON task
// file, which may contain one or more tasks.
func ReadTasksFromFile(fName string) ([]RawMIDATask, error) {
	tasks := make([]RawMIDATask, 0)

	data, err := ioutil.ReadFile(fName)
	if err != nil {
		return tasks, err
	}

	err = json.Unmarshal(data, &tasks)
	if err != nil {
		singleTask := RawMIDATask{}
		err = json.Unmarshal(data, &singleTask)
		if err != nil {
			return tasks, err
		} else {
			return append(tasks, singleTask), nil
		}
	} else {
		return tasks, nil
	}
}

// Takes raw tasks from input channel and produces sanitized tasks for the output channel
func SanitizeTasks(rawTaskChan <-chan RawMIDATask, sanitizedTaskChan chan<- SanitizedMIDATask, mConfig MIDAConfig, pipelineWG *sync.WaitGroup) {
	for r := range rawTaskChan {
		st, err := SanitizeTask(r)
		if err != nil {
			log.Fatal(err)
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
func TaskIntake(rtc chan<- RawMIDATask, mConfig MIDAConfig) {
	if mConfig.UseAMPQForTasks {
		log.Info("AMPQ not yet supported")
	} else {
		rawTasks, err := ReadTasksFromFile(mConfig.TaskLocation)
		if err != nil {
			log.Fatal(err)
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
func SanitizeTask(t RawMIDATask) (SanitizedMIDATask, error) {

	var st SanitizedMIDATask

	///// BEGIN SANITIZE AND BUILD URL
	if t.URL == "" {
		return st, errors.New("no URL to crawl given in task")
	}

	if t.Protocol == "" {
		t.Protocol = DefaultProtocol
	}

	port := ""
	if t.Port == 80 && t.Protocol == "http" {
		// Ignore port
		port = ""
	} else if t.Port == 443 && t.Protocol == "https" {
		port = ""
	} else if t.Port == 0 {
		port = ""
	} else if t.Port > 0 && t.Port < 65536 {
		port = ":" + strconv.Itoa(t.Port)
	} else {
		log.Fatal("Invalid port")
	}

	// Build the actual URL we will visit
	st.Url = t.Protocol + "://" + t.URL + port

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
		log.Debug("No timeout value given in task. Setting to default value of ", DefaultTimeout)
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
			log.Fatal("Failed to locate Chrome or Chromium on your system")
		}
	} else {
		// Validate that this binary exists
		if _, err := os.Stat(t.Browser.BrowserBinary); err != nil {
			// We won't crawl if the user specified a browser that does not exist
			log.Fatal("No such browser binary: ", t.Browser.BrowserBinary)
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
			log.Warn("SetBrowserFlags option is overriding AddBrowserFlags option")
		}
		if len(t.Browser.RemoveBrowserFlags) != 0 {
			log.Warn("SetBrowserFlags option is overriding RemoveBrowserFlags option")
		}

		for _, flag := range t.Browser.SetBrowserFlags {
			ff, err := FormatFlag(flag)
			if err != nil {
				log.Warn(err)
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
				log.Warn(err)
			} else {
				st.BrowserFlags = append(st.BrowserFlags, ff)
			}
		}
	}

	///// END SANITIZE BROWSER PARAMETERS /////

	///// BEGIN SANITIZE DATA GATHERING PARAMETERS /////

	// For now, these are just bools and we will just copy them
	st.AllFiles = t.AllFiles
	st.AllScripts = t.AllScripts
	st.JSTrace = t.JSTrace
	st.Screenshot = t.Screenshot
	st.Cookies = t.Cookies
	st.Certificates = t.Certificates
	st.CodeCoverage = t.CodeCoverage

	///// END SANITIZE DATA GATHERING PARAMETERS /////

	if t.MaxAttempts <= 1 {
		st.MaxAttempts = 1
	} else if t.MaxAttempts > MaximumTaskAttempts {
		log.Fatal("A task may not have more than ", MaximumTaskAttempts, " attempts")
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
