package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"strings"
)

func BuildCompressedTaskSet(cmd *cobra.Command, args []string) (CompressedMIDATaskSet, error) {
	t := InitializeCompressedTaskSet()
	var err error

	if cmd.Name() == "go" {
		// Get URLs from arguments
		for _, arg := range args {
			pieces := strings.Split(arg, ",")
			for _, piece := range pieces {
				u, err := ValidateURL(piece)
				if err != nil {
					return t, err
				}
				*t.URL = append(*t.URL, u)
			}
		}
	} else if cmd.Name() == "build" {
		// Get URL from URL file
		fname, err := cmd.Flags().GetString("urlfile")
		if err != nil {
			return t, err
		}

		urlfile, err := os.Open(fname)
		if err != nil {
			return t, err
		}
		defer urlfile.Close()

		scanner := bufio.NewScanner(urlfile)
		for scanner.Scan() {
			u, err := ValidateURL(scanner.Text())
			if err != nil {
				return t, err
			}
			*t.URL = append(*t.URL, u)
		}
	} else {
		return t, errors.New("unknown command passed to BuildCompressedTaskSet()")
	}

	// Fill in browser settings
	*t.Browser.BrowserBinary, err = cmd.Flags().GetString("browser")
	if err != nil {
		return t, err
	}
	*t.Browser.UserDataDirectory, err = cmd.Flags().GetString("user-data-dir")
	if err != nil {
		return t, err
	}
	*t.Browser.AddBrowserFlags, err = cmd.Flags().GetStringSlice("add-browser-flags")
	if err != nil {
		return t, err
	}
	*t.Browser.RemoveBrowserFlags, err = cmd.Flags().GetStringSlice("remove-browser-flags")
	if err != nil {
		return t, err
	}
	*t.Browser.SetBrowserFlags, err = cmd.Flags().GetStringSlice("set-browser-flags")
	if err != nil {
		return t, err
	}
	*t.Browser.Extensions, err = cmd.Flags().GetStringSlice("extensions")
	if err != nil {
		return t, err
	}

	// Fill in completion settings
	*t.Completion.Timeout, err = cmd.Flags().GetInt("timeout")
	if err != nil {
		return t, err
	}
	*t.Completion.TimeAfterLoad, err = cmd.Flags().GetInt("time-after-load")
	if err != nil {
		return t, err
	}
	*t.Completion.CompletionCondition, err = cmd.Flags().GetString("completion")
	if err != nil {
		return t, err
	}

	// Fill in data settings
	// TODO: Allow cmdline option for data gathering settings somehow
	*t.Data.AllResources = DefaultAllResources
	*t.Data.AllScripts = DefaultAllScripts
	*t.Data.JSTrace = DefaultJSTrace
	*t.Data.SaveRawTrace = DefaultSaveRawTrace
	*t.Data.ResourceMetadata = DefaultResourceMetadata
	*t.Data.ScriptMetadata = DefaultScriptMetadata
	*t.Data.ResourceTree = DefaultResourceTree

	// Fill in output settings
	*t.Output.Path, err = cmd.Flags().GetString("results-output-path")
	if err != nil {
		return t, err
	}
	*t.Output.GroupID, err = cmd.Flags().GetString("group")
	if err != nil {
		return t, err
	}

	// Fill in miscellaneous other settings
	*t.MaxAttempts, err = cmd.Flags().GetInt("attempts")
	if err != nil {
		return t, err
	}

	if cmd.Name() == "go" {
		return t, nil
	} else if cmd.Name() == "build" {
		// Check whether output file exists. Error if it does and overwrite is not set.
		fname, err := cmd.Flags().GetString("outfile")

		if err != nil {
			return t, err
		}
		overwrite, err := cmd.Flags().GetBool("overwrite")
		if err != nil {
			return t, err
		}
		_, err = os.Stat(fname)
		if err == nil && !overwrite {
			Log.Error("Task file '", fname, "' already exists")
			return t, errors.New("use '-x' to overwrite existing task file")
		}

		// Write output JSON file
		outData, err := json.Marshal(t)
		if err != nil {
			return t, err
		}

		err = ioutil.WriteFile(fname, outData, 0644)
		if err != nil {
			return t, errors.New("failed to write task file")
		} else {
			Log.Info("Successfully wrote task file to ", fname)
			return t, nil
		}
	} else {
		return t, errors.New("unknown command passed to BuildCompressedTaskSet()")
	}

}

func InitializeCompressedTaskSet() CompressedMIDATaskSet {

	t := CompressedMIDATaskSet{
		URL: new([]string),
		Browser: &BrowserSettings{
			BrowserBinary:      new(string),
			UserDataDirectory:  new(string),
			AddBrowserFlags:    new([]string),
			RemoveBrowserFlags: new([]string),
			SetBrowserFlags:    new([]string),
			Extensions:         new([]string),
		},
		Completion: &CompletionSettings{
			CompletionCondition: new(string),
			Timeout:             new(int),
			TimeAfterLoad:       new(int),
		},
		Data: &DataSettings{
			AllResources:     new(bool),
			AllScripts:       new(bool),
			JSTrace:          new(bool),
			SaveRawTrace:     new(bool),
			ResourceMetadata: new(bool),
			ScriptMetadata:   new(bool),
			ResourceTree:     new(bool),
			WebsocketTraffic: new(bool),
		},
		Output: &OutputSettings{
			Path:    new(string),
			GroupID: new(string),
		},
		MaxAttempts: new(int),
	}
	return t
}
