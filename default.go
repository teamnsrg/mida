package main

import "github.com/spf13/viper"

const (
	CompleteOnLoadEvent        = 0
	CompleteOnTimeoutOnly      = 1
	CompleteOnTimeoutAfterLoad = 2
)

func initConfig() {
	// Initialize the defaults below
	setDefaults()

	// We will read environment variables with this prefix
	viper.SetEnvPrefix("MIDA")
	viper.AutomaticEnv()
}

func setDefaults() {
	// MIDA-Wide Configuration Defaults
	viper.SetDefault("crawlers", 1)
	viper.SetDefault("storers", 1)
	viper.SetDefault("promport", 8001)
	viper.SetDefault("monitor", false)
	viper.SetDefault("log-level", 2)
	viper.SetDefault("taskfile", "examples/MIDA_task.json")
	viper.SetDefault("rabbitmqurl", "localhost:5672")
	viper.SetDefault("rabbitmquser", "")
	viper.SetDefault("rabbitmqpass", "")
	viper.SetDefault("rabbitmqtaskqueue", "tasks")
	viper.SetDefault("rabbitmqbroadcastqueue", "broadcast")
	viper.SetDefault("mongourl", "localhost:27017")
	viper.SetDefault("mongouser", "")
	viper.SetDefault("mongopass", "")
	viper.SetDefault("mongodatabase", "")
}

const (
	// MIDA Configuration Defaults

	DefaultTaskAttempts         = 2
	DefaultNavTimeout           = 7
	DefaultSSHBackoffMultiplier = 5
	DefaultTaskPriority         = 5

	// Browser-Related Parameters
	DefaultOSXChromePath     = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	DefaultOSXChromiumPath   = "/Applications/Chromium.app/Contents/MacOS/Chromium"
	DefaultLinuxChromePath   = "/usr/bin/google-chrome-stable"
	DefaultLinuxChromiumPath = "/usr/bin/chromium-browser"

	DefaultGroupID = "default"

	// Task completion
	DefaultTimeAfterLoad       = 0
	DefaultTimeout             = 5 // Default time (in seconds) to remain on a page before exiting browser
	DefaultCompletionCondition = CompleteOnTimeoutOnly

	// Defaults for data gathering settings
	DefaultAllResources     = false
	DefaultAllScripts       = false
	DefaultJSTrace          = false
	DefaultSaveRawTrace     = false
	DefaultResourceMetadata = true
	DefaultScriptMetadata   = true
	DefaultResourceTree     = false
	DefaultWebsocketTraffic = false
	DefaultNetworkStrace    = false
	DefaultOpenWPMChecks    = false
	DefaultBrowserCoverage  = false

	// Other/Util

)

var DefaultBrowserFlags = []string{
	"--disable-background-networking",
	"--disable-background-timer-throttling",
	"--disable-backgrounding-occluded-windows",
	"--disable-client-side-phishing-detection",
	"--disable-extensions",
	"--disable-features=IsolateOrigins,site-per-process",
	"--disable-hang-monitor",
	"--disable-ipc-flooding-protection",
	"--disable-infobars",
	"--disable-popup-blocking",
	"--disable-prompt-on-repost",
	"--disable-renderer-backgrounding",
	"--disable-sync",
	"--disk-cache-size=0",
	"--incognito",
	"--new-window",
	"--no-default-browser-check",
	"--no-first-run",
	"--no-sandbox",
	"--safebrowsing-disable-auto-update",
}
