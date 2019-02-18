package main

// Crawl Completion Conditions
type CompletionCondition int

const (
	CompleteOnLoadEvent        CompletionCondition = 0
	CompleteOnTimeoutOnly      CompletionCondition = 1
	CompleteOnTimeoutAfterLoad CompletionCondition = 2
)

const (
	// MIDA Configuration Defaults
	DefaultNumCrawlers    = 1
	DefaultNumStorers     = 1
	DefaultTaskLocation   = "examples/MIDA_task.json"
	DefaultPrometheusPort = 8001

	DefaultMaximumTaskAttempts = 10
	DefaultNavTimeout          = 7

	// Browser-Related Parameters
	DefaultOSXChromePath     = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	DefaultOSXChromiumPath   = "/Applications/Chromium.app/Contents/MacOS/Chromium"
	DefaultLinuxChromePath   = "/usr/bin/google-chrome-stable"
	DefaultLinuxChromiumPath = "/usr/bin/chromium-browser"
	DefaultLogFileName       = "chrome.log"

	// Output Parameters
	DefaultLocalOutputPath  = "results"
	DefaultRemoteOutputPath = ""
	TempDirectory           = ".tmp"
	DefaultFileSubdir       = "files"
	DefaultGroupID          = "default"

	// Task completion
	DefaultProtocolPrefix      = "http://"
	DefaultTimeout             = 5 // Default time (in seconds) to remain on a page before exiting browser
	DefaultCompletionCondition = CompleteOnTimeoutOnly

	// Defaults for data gathering settings
	DefaultAllFiles     = true
	DefaultAllScripts   = true
	DefaultJSTrace      = true
	DefaultCertificates = true
	DefaultCookies      = true
	DefaultCodeCoverage = true
	DefaultScreenshot   = true

	// Other/Util
	AlphaNumChars           = "ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
	DefaultIdentifierLength = 16 // Random identifier for each crawl
)

var DefaultBrowserFlags = []string{
	"--disable-background-networking",
	"--disable-background-timer-throttling",
	"--disable-backgrounding-occluded-windows",
	"--disable-client-side-phishing-detection",
	"--disable-extensions",
	"--disable-ipc-flooding-protection",
	"--disable-popup-blocking",
	"--disable-prompt-on-repost",
	"--disable-renderer-backgrounding",
	"--disable-sync",
	"--disk-cache-size=0",
	"--incognito",
	"--new-window",
	"--no-default-browser-check",
	"--no-first-run",
	"--safebrowsing-disable-auto-update",
}
