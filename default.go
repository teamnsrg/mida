package main

import "github.com/spf13/viper"

// Crawl Completion Conditions
type CompletionCondition int

const (
	CompleteOnLoadEvent        CompletionCondition = 0
	CompleteOnTimeoutOnly      CompletionCondition = 1
	CompleteOnTimeoutAfterLoad CompletionCondition = 2
)

func InitConfig() {
	// Initialize the defaults below
	SetDefaults()

	// We will read environment variables with this prefix
	viper.SetEnvPrefix("MIDA")
	viper.AutomaticEnv()
}

func SetDefaults() {
	// MIDA-Wide Configuration Defaults
	viper.SetDefault("crawlers", 1)
	viper.SetDefault("storers", 1)
	viper.SetDefault("promport", 8001)
	viper.SetDefault("monitor", false)
	viper.SetDefault("taskfile", "examples/MIDA_task.json")
	viper.SetDefault("rabbitmqurl", "localhost:5672")
	viper.SetDefault("rabbitmquser", "")
	viper.SetDefault("rabbitmqpass", "")
	viper.SetDefault("rabbitmqtaskqueue", "tasks")
	viper.SetDefault("rabbitmqbroadcastqueue", "broadcast")
}

const (
	// MIDA Configuration Defaults

	DefaultTaskAttempts         = 2
	DefaultNavTimeout           = 7
	DefaultSSHBackoffMultiplier = 5

	// Browser-Related Parameters
	DefaultOSXChromePath      = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	DefaultOSXChromiumPath    = "/Applications/Chromium.app/Contents/MacOS/Chromium"
	DefaultLinuxChromePath    = "/usr/bin/google-chrome-stable"
	DefaultLinuxChromiumPath  = "/usr/bin/chromium-browser"
	DefaultBrowserLogFileName = "browser.log"
	DefaultProtocolPrefix     = "http://"

	// Output Parameters
	DefaultOutputPath           = "results"
	DefaultFileSubdir           = "files"
	DefaultScriptSubdir         = "scripts"
	DefaultResourceMetadataFile = "resource_metadata.json"
	DefaultScriptMetadataFile   = "script_metadata.json"
	DefaultJSTracePath          = "js_trace.json"
	DefaultGroupID              = "default"

	// Task completion
	DefaultTimeAfterLoad       = 0
	DefaultTimeout             = 5 // Default time (in seconds) to remain on a page before exiting browser
	DefaultCompletionCondition = CompleteOnTimeoutOnly

	// Defaults for data gathering settings
	DefaultAllResources     = true
	DefaultAllScripts       = true
	DefaultJSTrace          = true
	DefaultSaveRawTrace		= false
	DefaultResourceMetadata = true
	DefaultScriptMetadata   = true
	DefaultResourceTree     = true

	// RabbitMQDefaults

	// Other/Util
	AlphaNumChars           = "ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
	DefaultIdentifierLength = 16 // Random identifier for each crawl
	MIDALogFile             = "mida.log"
	TempDir                 = ".tmp"
)

var DefaultBrowserFlags = []string{
	"--disable-background-networking",
	"--disable-background-timer-throttling",
	"--disable-backgrounding-occluded-windows",
	"--disable-client-side-phishing-detection",
	"--disable-extensions",
	"--disable-features=IsolateOrigins,site-per-process",
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
