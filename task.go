package main

import (
	"encoding/json"
	"errors"
	"github.com/teamnsrg/chromedp/runner"
	"io/ioutil"
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
	AllResources     bool `json:"all_files"`
	AllScripts       bool `json:"all_scripts"`
	JSTrace          bool `json:"js_trace"`
	ResourceMetadata bool `json:"resource_metadata"`
	ScriptMetadata   bool `json:"script_metadata"`
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
	AllFiles         bool
	AllScripts       bool
	JSTrace          bool
	ResourceMetadata bool
	ScriptMetadata   bool

	// Output Settings
	OutputPath       string
	GroupID          string // For identifying experiments
	RandomIdentifier string // Randomly generated task identifier

	// Parameters for retrying a task if it fails to complete
	MaxAttempts    int
	CurrentAttempt int
	TaskFailed     bool   // Nothing else should be done on the task once this flag is set
	FailureCode    string // Should be appended whenever a task is set to fail
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
