package main

type OutputSettings struct {
	SaveToLocalFS  bool   `json:"local"`
	SaveToRemoteFS bool   `json:"remote_fs"`
	LocalPath      string `json:"local_path"`
	RemotePath     string `json:"remote_path"`
}

type CompletionSettings struct {
	CompleteOnLoadEvent bool `json:"complete_on_load_event"`
	CompleteOnTimeout   bool `json:"complete_on_timeout"`
	Timeout             uint `json:"timeout"`
}

type BrowserSettings struct {
	BrowserBinary      string   `json:"browser_binary"`
	AddBrowserFlags    []string `json:"add_browser_flags"`
	RemoveBrowserFlags []string `json:"remove_browser_flags"`
	SetBrowserFlags    []string `json:"set_browser_flags"`
}

type MIDA_Task struct {
	Protocol string `json:"protocol"`
	Port     uint   `json:"port"`
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

func InitTask() MIDA_Task {
	t := MIDA_Task{
		Protocol: "http",
		Port:     80,
		URL:      "",
		Browser: BrowserSettings{
			BrowserBinary:      "",
			AddBrowserFlags:    []string{},
			RemoveBrowserFlags: []string{},
			SetBrowserFlags:    []string{},
		},
		Output: OutputSettings{},
	}

	return t
}
