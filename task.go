package main

import (
	"encoding/json"
	"github.com/influxdata/platform/kit/errors"
	log "github.com/sirupsen/logrus"
	"github.com/teamnsrg/chromedp/runner"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"
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
}

type SanitizedMIDATask struct {
	Url string

	BrowserBinary string
	BrowserFlags  []runner.CommandLineOption

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
}

func ReadTaskFromFile(fname string) (RawMIDATask, error) {
	t := InitTask()

	data, err := ioutil.ReadFile(fname)
	if err != nil {
		return t, err
	}

	err = json.Unmarshal(data, &t)
	if err != nil {
		return t, err
	} else {
		return t, nil
	}
}

// Initialize raw task with default values. Note that some values (URL, etc.) must be specified elsewhere
func InitTask() RawMIDATask {
	t := RawMIDATask{
		Protocol: "http",
		Port:     0,
		URL:      "",
		Browser: BrowserSettings{
			BrowserBinary:      "",
			AddBrowserFlags:    []string{},
			RemoveBrowserFlags: []string{},
			SetBrowserFlags:    []string{},
		},
		Completion: CompletionSettings{
			CompletionCondition: "CompleteOnTimeoutOnly",
			Timeout:             10,
		},
		Output: OutputSettings{
			SaveToLocalFS:  true,
			SaveToRemoteFS: false,
			LocalPath:      DefaultLocalOutputPath,
			RemotePath:     DefaultRemoteOutputPath,
		},
	}

	return t
}

// Run a series of checks on a raw task to ensure it is valid for a crawl.
// Put the task in a new format ("SanitizedMIDATask") which is used for processing.
func SanitizeTask(t RawMIDATask) (SanitizedMIDATask, error) {

	var st SanitizedMIDATask

	///// BEGIN SANITIZE AND BUILD URL
	if t.URL == "" {
		return st, errors.New("No URL to crawl given in task")
	}

	if t.Protocol != "http" && t.Protocol != "https" {
		return st, errors.New("Protocol should be 'http' or 'https'")
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
	} else {
		return st, errors.New("Invalid completion condition value given")
	}

	st.Timeout = t.Completion.Timeout

	///// BEGIN SANITIZE BROWSER PARAMETERS /////

	// Make sure we have a valid browser binary path, or select a default one
	if t.Browser.BrowserBinary == "" {
		// Set to system default.
		if runtime.GOOS == "darwin" {
			// OS X
			if _, err := os.Stat(DefaultOSXChromiumPath); err == nil {
				st.BrowserBinary = DefaultOSXChromiumPath
			} else if _, err := os.Stat(DefaultOSXChromePath); err == nil {
				st.BrowserBinary = DefaultOSXChromePath
			}
		} else if runtime.GOOS == "linux" {
			// Linux
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
