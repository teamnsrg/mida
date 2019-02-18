package main

import (
	"bufio"
	"encoding/json"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
)

func BuildTask(cmd *cobra.Command) {

	var t CompressedMIDATaskSet

	// Get URL from URL file
	fname, err := cmd.Flags().GetString("urlfile")
	if err != nil {
		Log.Fatal(err)
	}

	urlfile, err := os.Open(fname)
	if err != nil {
		Log.Fatal(err)
	}
	defer urlfile.Close()

	scanner := bufio.NewScanner(urlfile)
	for scanner.Scan() {
		// TODO: Validate URL here
		t.URL = append(t.URL, scanner.Text())
	}

	// Fill in browser settings
	t.Browser.BrowserBinary, err = cmd.Flags().GetString("browser")
	if err != nil {
		Log.Fatal(err)
	}
	t.Browser.UserDataDirectory, err = cmd.Flags().GetString("user-data-dir")
	if err != nil {
		Log.Fatal(err)
	}
	t.Browser.AddBrowserFlags, err = cmd.Flags().GetStringSlice("add-browser-flags")
	if err != nil {
		Log.Fatal(err)
	}
	t.Browser.RemoveBrowserFlags, err = cmd.Flags().GetStringSlice("remove-browser-flags")
	if err != nil {
		Log.Fatal(err)
	}
	t.Browser.SetBrowserFlags, err = cmd.Flags().GetStringSlice("set-browser-flags")
	if err != nil {
		Log.Fatal(err)
	}
	t.Browser.Extensions, err = cmd.Flags().GetStringSlice("extensions")
	if err != nil {
		Log.Fatal(err)
	}

	// Fill in completion settings
	t.Completion.Timeout, err = cmd.Flags().GetInt("timeout")
	if err != nil {
		Log.Fatal(err)
	}
	t.Completion.CompletionCondition, err = cmd.Flags().GetString("completion")
	if err != nil {
		Log.Fatal(err)
	}

	// Fill in data settings
	// TODO: Allow cmdline option for data gathering settings somehow
	t.Data.AllFiles = DefaultAllFiles
	t.Data.AllScripts = DefaultAllScripts
	t.Data.JSTrace = DefaultJSTrace
	t.Data.Certificates = DefaultCertificates
	t.Data.Cookies = DefaultCookies
	t.Data.CodeCoverage = DefaultCodeCoverage
	t.Data.Screenshot = DefaultScreenshot

	// Fill in output settings
	t.Output.Path, err = cmd.Flags().GetString("results-output-path")
	if err != nil {
		Log.Fatal(err)
	}
	t.Output.GroupID, err = cmd.Flags().GetString("group")
	if err != nil {
		Log.Fatal(err)
	}

	// Fill in miscellaneous other settings
	t.MaxAttempts, err = cmd.Flags().GetInt("attempts")
	if err != nil {
		Log.Fatal(err)
	}

	// Check whether output file exists. Error if it does and overwrite is not set.
	fname, err = cmd.Flags().GetString("outfile")

	if err != nil {
		Log.Fatal(err)
	}
	overwrite, err := cmd.Flags().GetBool("overwrite")
	if err != nil {
		Log.Fatal(err)
	}
	_, err = os.Stat(fname)
	if err == nil && !overwrite {
		Log.Error("Task file '", fname, "' already exists")
		Log.Fatal("Use '-x' to overwrite")
	}

	// Write output JSON file
	outData, err := json.Marshal(t)
	if err != nil {
		Log.Fatal(err)
	}

	err = ioutil.WriteFile(fname, outData, 0644)

	return
}
