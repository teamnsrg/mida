package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"github.com/spf13/cobra"
	"github.com/teamnsrg/mida/log"
	t "github.com/teamnsrg/mida/types"
	"github.com/teamnsrg/mida/util"
	"io/ioutil"
	"os"
	"strings"
)

func BuildCompressedTaskSet(cmd *cobra.Command, args []string) (t.CompressedMIDATaskSet, error) {
	ts := InitializeCompressedTaskSet()
	var err error

	if cmd.Name() == "go" {
		// Get URLs from arguments
		for _, arg := range args {
			pieces := strings.Split(arg, ",")
			for _, piece := range pieces {
				u, err := util.ValidateURL(piece)
				if err != nil {
					return ts, err
				}
				*ts.URL = append(*ts.URL, u)
			}
		}
	} else if cmd.Name() == "build" {
		// Get URL from URL file
		fname, err := cmd.Flags().GetString("urlfile")
		if err != nil {
			return ts, err
		}

		urlfile, err := os.Open(fname)
		if err != nil {
			return ts, err
		}
		defer urlfile.Close()

		scanner := bufio.NewScanner(urlfile)
		for scanner.Scan() {
			u, err := util.ValidateURL(scanner.Text())
			if err != nil {
				return ts, err
			}
			*ts.URL = append(*ts.URL, u)
		}
	} else {
		return ts, errors.New("unknown command passed to BuildCompressedTaskSet()")
	}

	// Fill in browser settings
	*ts.Browser.BrowserBinary, err = cmd.Flags().GetString("browser")
	if err != nil {
		return ts, err
	}
	*ts.Browser.UserDataDirectory, err = cmd.Flags().GetString("user-data-dir")
	if err != nil {
		return ts, err
	}
	*ts.Browser.AddBrowserFlags, err = cmd.Flags().GetStringSlice("add-browser-flags")
	if err != nil {
		return ts, err
	}
	*ts.Browser.RemoveBrowserFlags, err = cmd.Flags().GetStringSlice("remove-browser-flags")
	if err != nil {
		return ts, err
	}
	*ts.Browser.SetBrowserFlags, err = cmd.Flags().GetStringSlice("set-browser-flags")
	if err != nil {
		return ts, err
	}
	*ts.Browser.Extensions, err = cmd.Flags().GetStringSlice("extensions")
	if err != nil {
		return ts, err
	}

	// Fill in completion settings
	*ts.Completion.Timeout, err = cmd.Flags().GetInt("timeout")
	if err != nil {
		return ts, err
	}
	*ts.Completion.TimeAfterLoad, err = cmd.Flags().GetInt("time-after-load")
	if err != nil {
		return ts, err
	}
	*ts.Completion.CompletionCondition, err = cmd.Flags().GetString("completion")
	if err != nil {
		return ts, err
	}

	// Fill in data settings
	*ts.Data.AllResources, err = cmd.Flags().GetBool("all-resources")
	if err != nil {
		return ts, err
	}
	*ts.Data.AllScripts, err = cmd.Flags().GetBool("all-scripts")
	if err != nil {
		return ts, err
	}
	*ts.Data.JSTrace, err = cmd.Flags().GetBool("js-trace")
	if err != nil {
		return ts, err
	}
	*ts.Data.ResourceMetadata, err = cmd.Flags().GetBool("resource-metadata")
	if err != nil {
		return ts, err
	}
	*ts.Data.ResourceTree, err = cmd.Flags().GetBool("resource-tree")
	if err != nil {
		return ts, err
	}
	*ts.Data.ScriptMetadata, err = cmd.Flags().GetBool("script-metadata")
	if err != nil {
		return ts, err
	}
	*ts.Data.WebsocketTraffic, err = cmd.Flags().GetBool("websocket")
	if err != nil {
		return ts, err
	}
	*ts.Data.NetworkTrace, err = cmd.Flags().GetBool("network-strace")
	if err != nil {
		return ts, err
	}
	*ts.Data.SaveRawTrace, err = cmd.Flags().GetBool("save-raw-trace")
	if err != nil {
		return ts, err
	}
	*ts.Data.OpenWPMChecks, err = cmd.Flags().GetBool("openwpm-checks")
	if err != nil {
		return ts, err
	}
	*ts.Data.BrowserCoverage, err = cmd.Flags().GetBool("browser-coverage")
	if err != nil {
		return ts, err
	}

	// Fill in output settings
	*ts.Output.Path, err = cmd.Flags().GetString("results-output-path")
	if err != nil {
		return ts, err
	}
	*ts.Output.GroupID, err = cmd.Flags().GetString("group")
	if err != nil {
		return ts, err
	}

	// Fill in miscellaneous other settings
	*ts.MaxAttempts, err = cmd.Flags().GetInt("attempts")
	if err != nil {
		return ts, err
	}

	*ts.Priority, err = cmd.Flags().GetInt("priority")
	if err != nil {
		return ts, err
	}

	if cmd.Name() == "go" {
		return ts, nil
	} else if cmd.Name() == "build" {
		// Check whether output file exists. Error if it does and overwrite is not set.
		fname, err := cmd.Flags().GetString("outfile")

		if err != nil {
			return ts, err
		}
		overwrite, err := cmd.Flags().GetBool("overwrite")
		if err != nil {
			return ts, err
		}
		_, err = os.Stat(fname)
		if err == nil && !overwrite {
			log.Log.Error("Task file '", fname, "' already exists")
			return ts, errors.New("use '-x' to overwrite existing task file")
		}

		// Write output JSON file
		outData, err := json.Marshal(ts)
		if err != nil {
			return ts, err
		}

		err = ioutil.WriteFile(fname, outData, 0644)
		if err != nil {
			return ts, errors.New("failed to write task file")
		} else {
			log.Log.Info("Successfully wrote task file to ", fname)
			return ts, nil
		}
	} else {
		return ts, errors.New("unknown command passed to BuildCompressedTaskSet()")
	}

}

func InitializeCompressedTaskSet() t.CompressedMIDATaskSet {

	cts := t.CompressedMIDATaskSet{
		URL: new([]string),
		Browser: &t.BrowserSettings{
			BrowserBinary:      new(string),
			UserDataDirectory:  new(string),
			AddBrowserFlags:    new([]string),
			RemoveBrowserFlags: new([]string),
			SetBrowserFlags:    new([]string),
			Extensions:         new([]string),
		},
		Completion: &t.CompletionSettings{
			CompletionCondition: new(string),
			Timeout:             new(int),
			TimeAfterLoad:       new(int),
		},
		Data: &t.DataSettings{
			AllResources:     new(bool),
			AllScripts:       new(bool),
			JSTrace:          new(bool),
			SaveRawTrace:     new(bool),
			ResourceMetadata: new(bool),
			ScriptMetadata:   new(bool),
			ResourceTree:     new(bool),
			WebsocketTraffic: new(bool),
			NetworkTrace:     new(bool),
			OpenWPMChecks:    new(bool),
			BrowserCoverage:  new(bool),
		},
		Output: &t.OutputSettings{
			Path:     new(string),
			GroupID:  new(string),
			MongoURI: new(string),
		},
		MaxAttempts: new(int),
		Priority:    new(int),
	}
	return cts
}
