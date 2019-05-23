package main

import (
	"errors"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/storage"
	t "github.com/teamnsrg/mida/types"
	"github.com/teamnsrg/mida/util"
	"os"
	"path"
	"runtime"
	"sync"
)

// Takes raw tasks from input channel and produces sanitized tasks for the output channel
func SanitizeTasks(rawTaskChan <-chan t.MIDATask, sanitizedTaskChan chan<- t.SanitizedMIDATask, pipelineWG *sync.WaitGroup) {
	for r := range rawTaskChan {
		st, err := SanitizeTask(r)
		if err != nil {
			log.Log.Fatal(err)
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
func SanitizeTask(mt t.MIDATask) (t.SanitizedMIDATask, error) {

	var st t.SanitizedMIDATask
	var err error

	// Generate our random identifier for this task
	st.RandomIdentifier = util.GenRandomIdentifier()

	///// BEGIN SANITIZE AND BUILD URL /////
	if mt.URL == nil || *mt.URL == "" {
		return st, errors.New("no URL to crawl given in task")
	}

	// Do what we can to ensure a valid URL
	st.Url, err = util.ValidateURL(*mt.URL)
	if err != nil {
		return st, err
	}

	///// END SANITIZE AND BUILD URL /////
	///// BEGIN SANITIZE TASK COMPLETION SETTINGS

	if mt.Completion == nil {
		mt.Completion = new(t.CompletionSettings)
	}

	if mt.Completion.CompletionCondition == nil {
		st.CCond = DefaultCompletionCondition
	} else if *mt.Completion.CompletionCondition == "CompleteOnTimeoutOnly" {
		st.CCond = CompleteOnTimeoutOnly
	} else if *mt.Completion.CompletionCondition == "CompleteOnLoadEvent" {
		st.CCond = CompleteOnLoadEvent
	} else if *mt.Completion.CompletionCondition == "CompleteOnTimeoutAfterLoad" {
		st.CCond = CompleteOnTimeoutAfterLoad
	} else {
		return st, errors.New("invalid completion condition value given")
	}

	// If we don't get a value for timeout (or get zero), and we NEED that
	// value, just set it to the default
	if mt.Completion.Timeout == nil && st.CCond != CompleteOnLoadEvent {
		log.Log.Debug("No timeout value given in task. Setting to default value of ", DefaultTimeout)
		st.Timeout = DefaultTimeout
	} else if *mt.Completion.Timeout < 0 {
		return st, errors.New("invalid negative value for task timeout")
	} else {
		st.Timeout = *mt.Completion.Timeout
	}

	if mt.Completion.TimeAfterLoad == nil {
		if st.CCond == CompleteOnTimeoutAfterLoad {
			return st, errors.New("TimeoutAfterLoad specified but no value given")
		}
	} else if *mt.Completion.TimeAfterLoad < 0 {
		return st, errors.New("invalid value for TimeoutAfterLoad")
	} else {
		st.TimeAfterLoad = *mt.Completion.TimeAfterLoad
	}

	///// END SANITIZE TASK COMPLETION SETTINGS /////
	///// BEGIN SANITIZE BROWSER PARAMETERS /////

	if mt.Browser == nil {
		mt.Browser = new(t.BrowserSettings)
	}

	// Make sure we have a valid browser binary path, or select a default one
	if mt.Browser.BrowserBinary == nil || *mt.Browser.BrowserBinary == "" {
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
			log.Log.Fatal("Failed to locate Chrome or Chromium on your system")
		}
	} else {
		// Validate that this binary exists
		if _, err := os.Stat(*mt.Browser.BrowserBinary); err != nil {
			// We won't crawl if the user specified a browser that does not exist
			log.Log.Fatal("No such browser binary: ", *mt.Browser.BrowserBinary)
		} else {
			st.BrowserBinary = *mt.Browser.BrowserBinary
		}
	}

	// Sanitize user data directory to use
	if mt.Browser.UserDataDirectory == nil || *mt.Browser.UserDataDirectory == "" {
		st.UserDataDirectory = path.Join(storage.TempDir, st.RandomIdentifier)
	} else {
		// Chrome will create any directories required
		st.UserDataDirectory = *mt.Browser.UserDataDirectory
	}

	// Sanitize browser flags/command line options
	if mt.Browser.SetBrowserFlags == nil {
		mt.Browser.SetBrowserFlags = new([]string)
	}
	if mt.Browser.AddBrowserFlags == nil {
		mt.Browser.AddBrowserFlags = new([]string)
	}
	if mt.Browser.RemoveBrowserFlags == nil {
		mt.Browser.RemoveBrowserFlags = new([]string)
	}
	if mt.Browser.Extensions == nil {
		mt.Browser.Extensions = new([]string)
	}

	if len(*mt.Browser.Extensions) != 0 {
		// Check that each extension exists
		for _, e := range *mt.Browser.Extensions {
			x, err := os.Stat(e)
			if err != nil {
				return st, err
			}
			if !x.IsDir() {
				return st, errors.New("given extension [ " + e + " ] is not a directory")
			}
		}

		// Create the extensions flag
		extensionsFlag := "--disable-extensions-except="
		extensionsFlag += (*mt.Browser.Extensions)[0]
		if len(*mt.Browser.Extensions) > 1 {
			for _, e := range (*mt.Browser.Extensions)[1:] {
				extensionsFlag += ","
				extensionsFlag += e
			}
		}

		*mt.Browser.AddBrowserFlags = append(*mt.Browser.AddBrowserFlags, extensionsFlag)

		// Remove the --incognito and --disable-extensions (both prevent extensions)
		*mt.Browser.RemoveBrowserFlags = append(*mt.Browser.RemoveBrowserFlags, "--incognito")
		*mt.Browser.RemoveBrowserFlags = append(*mt.Browser.RemoveBrowserFlags, "--disable-extensions")
	}

	if len(*mt.Browser.SetBrowserFlags) != 0 {
		if len(*mt.Browser.AddBrowserFlags) != 0 {
			log.Log.Warn("SetBrowserFlags option is overriding AddBrowserFlags option")
		}
		if len(*mt.Browser.RemoveBrowserFlags) != 0 {
			log.Log.Warn("SetBrowserFlags option is overriding RemoveBrowserFlags option")
		}

		for _, flag := range *mt.Browser.SetBrowserFlags {
			st.BrowserFlags = append(st.BrowserFlags, flag)
		}
	} else {
		// Add flags, checking to see that they have not been removed
		for _, flag := range append(DefaultBrowserFlags, *mt.Browser.AddBrowserFlags...) {
			if util.IsRemoved(*mt.Browser.RemoveBrowserFlags, flag) {
				continue
			}
			st.BrowserFlags = append(st.BrowserFlags, flag)
		}
	}

	///// END SANITIZE BROWSER PARAMETERS /////
	///// BEGIN SANITIZE DATA GATHERING PARAMETERS /////

	if mt.Data == nil {
		mt.Data = new(t.DataSettings)
	}

	// Check if a value was provided. If not, set to default
	if mt.Data.AllResources != nil {
		st.AllResources = *mt.Data.AllResources
	} else {
		st.AllResources = DefaultAllResources
	}
	if mt.Data.AllScripts != nil {
		st.AllScripts = *mt.Data.AllScripts
	} else {
		st.AllScripts = DefaultAllScripts
	}
	if mt.Data.JSTrace != nil {
		st.JSTrace = *mt.Data.JSTrace
	} else {
		st.JSTrace = DefaultJSTrace
	}
	if mt.Data.SaveRawTrace != nil {
		st.SaveRawTrace = *mt.Data.SaveRawTrace
	} else {
		st.SaveRawTrace = DefaultSaveRawTrace
	}
	if mt.Data.ResourceMetadata != nil {
		st.ResourceMetadata = *mt.Data.ResourceMetadata
	} else {
		st.ResourceMetadata = DefaultResourceMetadata
	}
	if mt.Data.ScriptMetadata != nil {
		st.ScriptMetadata = *mt.Data.ScriptMetadata
	} else {
		st.ScriptMetadata = DefaultScriptMetadata
	}
	if mt.Data.ResourceTree != nil {
		st.ResourceTree = *mt.Data.ResourceTree
	} else {
		st.ResourceTree = DefaultResourceTree
	}
	if mt.Data.WebsocketTraffic != nil {
		st.WebsocketTraffic = *mt.Data.WebsocketTraffic
	} else {
		st.WebsocketTraffic = DefaultWebsocketTraffic
	}
	if mt.Data.NetworkTrace != nil {
		st.NetworkTrace = *mt.Data.NetworkTrace
	} else {
		st.NetworkTrace = DefaultNetworkStrace
	}
	if mt.Data.OpenWPMChecks != nil {
		st.OpenWPMChecks = *mt.Data.OpenWPMChecks
	} else {
		st.OpenWPMChecks = DefaultOpenWPMChecks
	}
	if mt.Data.BrowserCoverage != nil {
		st.BrowserCoverage = *mt.Data.BrowserCoverage
	} else {
		st.BrowserCoverage = DefaultBrowserCoverage
	}

	///// END SANITIZE DATA GATHERING PARAMETERS /////
	///// BEGIN SANITIZE OUTPUT PARAMETERS /////

	if mt.Output == nil {
		mt.Output = new(t.OutputSettings)
	}

	if mt.Output.Path == nil {
		st.OutputPath = storage.DefaultOutputPath
	} else {
		st.OutputPath = *mt.Output.Path
	}

	if mt.Output.GroupID == nil || *mt.Output.GroupID == "" {
		st.GroupID = DefaultGroupID
	} else {
		st.GroupID = *mt.Output.GroupID
	}

	if mt.Output.MongoURI == nil || *mt.Output.MongoURI == "" {
		st.MongoURI = ""
	} else {
		st.MongoURI = *mt.Output.MongoURI
	}

	///// END SANITIZE OUTPUT PARAMETERS /////

	if mt.MaxAttempts == nil {
		st.MaxAttempts = DefaultTaskAttempts
	} else if *mt.MaxAttempts <= DefaultTaskAttempts {
		st.MaxAttempts = DefaultTaskAttempts
	} else {
		st.MaxAttempts = *mt.MaxAttempts
	}
	st.CurrentAttempt = 1

	return st, nil
}
