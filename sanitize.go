package main

import (
	"errors"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
)

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

// Run a series of checks on a raw task to ensure it is valid for a crawl.
// Put the task in a new format ("SanitizedMIDATask") which is used for processing.
func SanitizeTask(t MIDATask) (SanitizedMIDATask, error) {

	var st SanitizedMIDATask

	// Generate our random identifier for this task
	st.RandomIdentifier = GenRandomIdentifier()

	///// BEGIN SANITIZE AND BUILD URL /////
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

	///// END SANITIZE AND BUILD URL /////
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
		st.UserDataDirectory = path.Join(TempDirectory, st.RandomIdentifier)
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
	///// BEGIN SANITIZE OUTPUT PARAMETERS /////

	st.OutputPath = t.Output.Path

	///// END SANITIZE OUTPUT PARAMETERS /////

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
