package main

import (
	"errors"
	"os"
	"path"
	"runtime"
	"sync"
)

// Takes raw tasks from input channel and produces sanitized tasks for the output channel
func SanitizeTasks(rawTaskChan <-chan MIDATask, sanitizedTaskChan chan<- SanitizedMIDATask, pipelineWG *sync.WaitGroup) {
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

// Run a series of checks on a raw task to ensure it is valid for a crawl.
// Put the task in a new format ("SanitizedMIDATask") which is used for processing.
func SanitizeTask(t MIDATask) (SanitizedMIDATask, error) {

	var st SanitizedMIDATask
	var err error

	// Generate our random identifier for this task
	st.RandomIdentifier = GenRandomIdentifier()

	///// BEGIN SANITIZE AND BUILD URL /////
	if t.URL == nil || *t.URL == "" {
		return st, errors.New("no URL to crawl given in task")
	}

	// Do what we can to ensure a valid URL
	st.Url, err = ValidateURL(*t.URL)
	if err != nil {
		return st, err
	}

	///// END SANITIZE AND BUILD URL /////
	///// BEGIN SANITIZE TASK COMPLETION SETTINGS

	if t.Completion == nil {
		t.Completion = new(CompletionSettings)
	}

	if t.Completion.CompletionCondition == nil {
		st.CCond = DefaultCompletionCondition
	} else if *t.Completion.CompletionCondition == "CompleteOnTimeoutOnly" {
		st.CCond = CompleteOnTimeoutOnly
	} else if *t.Completion.CompletionCondition == "CompleteOnLoadEvent" {
		st.CCond = CompleteOnLoadEvent
	} else if *t.Completion.CompletionCondition == "CompleteOnTimeoutAfterLoad" {
		st.CCond = CompleteOnTimeoutAfterLoad
	} else {
		return st, errors.New("invalid completion condition value given")
	}

	// If we don't get a value for timeout (or get zero), and we NEED that
	// value, just set it to the default
	if t.Completion.Timeout == nil && st.CCond != CompleteOnLoadEvent {
		Log.Debug("No timeout value given in task. Setting to default value of ", DefaultTimeout)
		st.Timeout = DefaultTimeout
	} else if *t.Completion.Timeout < 0 {
		return st, errors.New("invalid negative value for task timeout")
	} else {
		st.Timeout = *t.Completion.Timeout
	}

	if t.Completion.TimeAfterLoad == nil {
		if st.CCond == CompleteOnTimeoutAfterLoad {
			return st, errors.New("TimeoutAfterLoad specified but no value given")
		}
	} else if *t.Completion.TimeAfterLoad < 0 {
		return st, errors.New("invalid value for TimeoutAfterLoad")
	} else {
		st.TimeAfterLoad = *t.Completion.TimeAfterLoad
	}

	///// END SANITIZE TASK COMPLETION SETTINGS /////
	///// BEGIN SANITIZE BROWSER PARAMETERS /////

	if t.Browser == nil {
		t.Browser = new(BrowserSettings)
	}

	// Make sure we have a valid browser binary path, or select a default one
	if t.Browser.BrowserBinary == nil || *t.Browser.BrowserBinary == "" {
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
		if _, err := os.Stat(*t.Browser.BrowserBinary); err != nil {
			// We won't crawl if the user specified a browser that does not exist
			Log.Fatal("No such browser binary: ", t.Browser.BrowserBinary)
		} else {
			st.BrowserBinary = *t.Browser.BrowserBinary
		}
	}

	// Sanitize user data directory to use
	if t.Browser.UserDataDirectory == nil || *t.Browser.UserDataDirectory == "" {
		st.UserDataDirectory = path.Join(TempDir, st.RandomIdentifier)
	} else {
		// Chrome will create any directories required
		st.UserDataDirectory = *t.Browser.UserDataDirectory
	}

	// Sanitize browser flags/command line options
	if t.Browser.SetBrowserFlags == nil {
		t.Browser.SetBrowserFlags = new([]string)
	}
	if t.Browser.AddBrowserFlags == nil {
		t.Browser.AddBrowserFlags = new([]string)
	}
	if t.Browser.RemoveBrowserFlags == nil {
		t.Browser.RemoveBrowserFlags = new([]string)
	}
	if len(*t.Browser.SetBrowserFlags) != 0 {
		if len(*t.Browser.AddBrowserFlags) != 0 {
			Log.Warn("SetBrowserFlags option is overriding AddBrowserFlags option")
		}
		if len(*t.Browser.RemoveBrowserFlags) != 0 {
			Log.Warn("SetBrowserFlags option is overriding RemoveBrowserFlags option")
		}

		for _, flag := range *t.Browser.SetBrowserFlags {
			ff, err := FormatFlag(flag)
			if err != nil {
				Log.Warn(err)
			} else {
				st.BrowserFlags = append(st.BrowserFlags, ff)
			}
		}
	} else {
		// Add flags, checking to see that they have not been removed
		for _, flag := range append(DefaultBrowserFlags, *t.Browser.AddBrowserFlags...) {
			if IsRemoved(*t.Browser.RemoveBrowserFlags, flag) {
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

	// TODO: Extensions

	///// END SANITIZE BROWSER PARAMETERS /////
	///// BEGIN SANITIZE DATA GATHERING PARAMETERS /////

	if t.Data == nil {
		t.Data = new(DataSettings)
	}

	// Check if a value was provided. If not, set to default
	if t.Data.AllResources != nil {
		st.AllResources = *t.Data.AllResources
	} else {
		st.AllResources = DefaultAllResources
	}
	if t.Data.AllScripts != nil {
		st.AllScripts = *t.Data.AllScripts
	} else {
		st.AllScripts = DefaultAllScripts
	}
	if t.Data.JSTrace != nil {
		st.JSTrace = *t.Data.JSTrace
	} else {
		st.JSTrace = DefaultJSTrace
	}
	if t.Data.ResourceMetadata != nil {
		st.ResourceMetadata = *t.Data.ResourceMetadata
	} else {
		st.ResourceMetadata = DefaultResourceMetadata
	}
	if t.Data.ScriptMetadata != nil {
		st.ScriptMetadata = *t.Data.ScriptMetadata
	} else {
		st.ScriptMetadata = DefaultScriptMetadata
	}

	///// END SANITIZE DATA GATHERING PARAMETERS /////
	///// BEGIN SANITIZE OUTPUT PARAMETERS /////

	if t.Output == nil {
		t.Output = new(OutputSettings)
	}

	if t.Output.Path == nil || *t.Output.Path == "" {
		st.OutputPath = DefaultOutputPath
	} else {
		st.OutputPath = *t.Output.Path
	}

	if t.Output.GroupID == nil || *t.Output.GroupID == "" {
		st.GroupID = DefaultGroupID
	} else {
		st.GroupID = *t.Output.GroupID
	}

	///// END SANITIZE OUTPUT PARAMETERS /////

	if t.MaxAttempts == nil {
		st.MaxAttempts = DefaultTaskAttempts
	} else if *t.MaxAttempts <= DefaultTaskAttempts {
		st.MaxAttempts = DefaultTaskAttempts
	} else if *t.MaxAttempts > DefaultMaximumTaskAttempts {
		Log.Fatal("A task may not have more than ", DefaultMaximumTaskAttempts, " attempts")
	} else {
		st.MaxAttempts = *t.MaxAttempts
	}
	st.CurrentAttempt = 1

	return st, nil
}
