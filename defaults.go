package main

const (
	// Browser-Related Parameters
	DefaultOSXChromePath     = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	DefaultOSXChromiumPath   = "/Applications/Chromium.app/Contents/MacOS/Chromium"
	DefaultLinuxChromePath   = "/usr/bin/google-chrome-stable"
	DefaultLinuxChromiumPath = "/usr/bin/chromium-browser"

	// Output Parameters
	DefaultLocalOutputPath  = "output/"
	DefaultRemoteOutputPath = ""

	// Task completion

	// Other/Util
	Letters                 = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	DefaultIdentifierLength = 16
)

type CompletionCondition int

const (
	CompleteOnLoadEvent        CompletionCondition = 0
	CompleteOnTimeoutOnly      CompletionCondition = 1
	CompleteOnTimeoutAfterLoad CompletionCondition = 2
)

var DefaultBrowserFlags = []string{
	"--disable-background-networking",
	"--disable-background-timer-throttling",
	"--disable-client-side-phishing-detection",
	"--disable-extensions",
	"--disable-popup-blocking",
	"--disable-prompt-on-repost",
	"--disable-extensions",
	"--disable-sync",
	"--incognito",
	"--new-window",
	"--no-default-browser-check",
	"--no-first-run",
	"--safebrowsing-disable-auto-update",
}
