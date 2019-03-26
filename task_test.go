package main

import (
	"github.com/teamnsrg/mida/storage"
	"path"
	"strings"
	"testing"
)

func TestRead(t *testing.T) {
	tasks, err := ReadTasksFromFile("examples/exampleTask.json")
	if err != nil {
		t.Error("Failed to read exampleTask.json: ", err)
	}

	if len(tasks) != 3 {
		t.Fatal("Wrong length for tasks read from file")
	}

	if *tasks[1].Output.Path != "example_results" {
		t.Fatal("Incorrect output path setting for task")
	}
}

func TestSanitizeOne(t *testing.T) {
	taskOneData := []byte(`
{
  "url": "murley.io"
}`)
	tasks, err := ReadTasks(taskOneData)
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 1 {
		t.Fatal("Wrong length of MIDATasks")
	}

	st1, err := SanitizeTask(tasks[0])
	if err != nil {
		t.Fatal(err)
	}

	if st1.Url != "http://murley.io" {
		t.Error("Incorrect URL")
	}

	// Browser Options
	if st1.BrowserBinary == "" {
		t.Fatal("Did not set binary")
	}
	if st1.UserDataDirectory != path.Join(storage.TempDir, st1.RandomIdentifier) {
		t.Fatal("Incorrect user data dir")
	}
	if len(st1.BrowserFlags) != len(DefaultBrowserFlags) {
		t.Fatal("Incorrect browser flags")
	}

	// Completion Condition options
	if st1.CCond != DefaultCompletionCondition {
		t.Fatal("Incorrect completion condition")
	}
	if st1.Timeout != DefaultTimeout {
		t.Fatal("Incorrect timeout")
	}

	// Data gathering options
	if st1.ScriptMetadata != DefaultScriptMetadata ||
		st1.ResourceMetadata != DefaultResourceMetadata ||
		st1.JSTrace != DefaultJSTrace ||
		st1.AllResources != DefaultAllResources ||
		st1.AllScripts != DefaultAllScripts {
		t.Fatal("Defaults for data collection set incorrectly")
	}

	if st1.MaxAttempts != DefaultTaskAttempts {
		t.Fatal("Incorrect task attempts")
	}

}

func TestSanitizeTwo(t *testing.T) {
	taskTwoData := []byte(`
{
  "url": [
    "cnn.com",
    "cbs.com",
    "murley.io"
  ],
  "browser": {
    "browser_binary": "",
    "user_data_directory": "",
    "add_browser_flags": [],
    "remove_browser_flags": [],
    "set_browser_flags": [],
    "extensions": []
  },
  "completion": {
    "completion_condition": "CompleteOnTimeoutOnly",
    "timeout": 5
  },
  "data": {
    "all_files": true,
    "all_scripts": true,
    "js_trace": true,
    "resource_metadata": true,
    "script_metadata": true
  },
  "output": {
    "path": "example_results",
    "group_id": "default"
  },
  "max_attempts": 5
}`)

	tasks, err := ReadTasks(taskTwoData)
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 3 {
		t.Fatal("Wrong length of tasks (Should be 3, one for each URL")
	}

	st, err := SanitizeTask(tasks[0])
	if err != nil {
		t.Fatal("Task sanitization test failed: ", err)
	}

	if !strings.HasPrefix(st.Url, "http://") {
		t.Fatal("Did not append http:// to start of address in sanitized task")
	}

}

func TestSanitizeThree(t *testing.T) {
	taskThreeData := []byte(`
[
	{
	  "url": "http://apple.com",
	  "browser": {
	    "browser_binary": "",
	    "user_data_directory": "",
	    "add_browser_flags": [],
	    "remove_browser_flags": [],
	    "set_browser_flags": [],
	    "extensions": []
	  },
	  "completion": {
	    "completion_condition": "CompleteOnTimeoutOnly",
	    "timeout": 5
	  },
	  "data": {
	    "all_files": true,
	    "all_scripts": true,
	    "js_trace": true,
	    "resource_metadata": true
	  },
	  "output": {
	    "path": "example_results",
	    "group_id": "default"
	  },
	  "max_attempts": 5
	},
	{
	  "url": "https://apple.com",
	  "browser": {
	    "browser_binary": "",
	    "user_data_directory": "",
	    "add_browser_flags": [],
	    "remove_browser_flags": [],
	    "set_browser_flags": [],
	    "extensions": []
	  },
	  "completion": {
	    "completion_condition": "CompleteOnTimeoutOnly",
	    "timeout": 5
	  },
	  "data": {
	    "all_files": true,
	    "all_scripts": true,
	    "js_trace": true,
	    "resource_metadata": true,
	    "script_metadata": true
	  },
	  "output": {
	    "path": "example_results_2",
	    "group_id": "default"
	  },
	  "max_attempts": 5
	}
]`)

	tasks, err := ReadTasks(taskThreeData)
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 2 {
		t.Fatal("Wrong length of tasks (Should be 2, one for each task in array")
	}

	st1, err := SanitizeTask(tasks[0])
	if err != nil {
		t.Fatal("First task sanitization test failed: ", err)
	}

	st2, err := SanitizeTask(tasks[1])
	if err != nil {
		t.Fatal("Second task sanitization test failed: ", err)
	}

	if st1.Url == st2.Url {
		t.Fatal("Failed to set urls correctly")
	}
	if st1.OutputPath == st2.OutputPath {
		t.Fatal("Failed to set output paths correctly")
	}
	if st1.ScriptMetadata != DefaultScriptMetadata {
		t.Fatal("Failed to set default script metadata")
	}

}
