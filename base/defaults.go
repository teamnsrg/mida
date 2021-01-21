package base

const (
	// Output Parameters
	DefaultLocalOutputPath      = "results"
	DefaultPostQueue            = ""
	DefaultResourceSubdir       = "resources"
	DefaultScriptSubdir         = "scripts"
	DefaultCoverageSubdir       = "coverage"
	DefaultScreenshotFileName   = "screenshot.png"
	DefaultCookieFileName       = "cookies.json"
	DefaultDomFileName          = "dom.json"
	DefaultMetadataFile         = "metadata.json"
	DefaultCovBVFileName        = "coverage.bv"
	DefaultResourceMetadataFile = "resource_metadata.json"
	DefaultScriptMetadataFile   = "script_metadata.json"
	DefaultSftpPrivKeyFile      = "~/.ssh/id_rsa"
	DefaultTaskLogFile          = "task.log"

	// MIDA Configuration Defaults

	DefaultNavTimeout           = 30 // How long to wait when connecting to a web server
	DefaultSSHBackoffMultiplier = 5  // Exponential increase in time between tries when connecting for SFTP storage
	DefaultTaskPriority         = 5  // Queue priority when creating new tasks -- Value should be 1-10

	DefaultEventChannelBufferSize = 10000

	// Browser-Related Parameters
	DefaultOSXChromePath       = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	DefaultOSXChromiumPath     = "/Applications/Chromium.app/Contents/MacOS/Chromium"
	DefaultLinuxChromePath     = "/usr/bin/google-chrome-stable"
	DefaultLinuxChromiumPath   = "/usr/bin/chromium-browser"
	DefaultWindowsChromePath   = "C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe"
	DefaultWindowsChromiumPath = "\\%LocalAppData%\\chromium\\Application\\chrome.exe"
	DefaultHeadless            = false

	// RawTask completion
	DefaultTimeAfterLoad       = 5  // Default time to stay on a page after load event is fired (in TimeAfterLoad mode)
	DefaultTimeout             = 10 // Default time (in seconds) to remain on a page before exiting browser
	DefaultCompletionCondition = TimeoutOnly

	// Default Interaction Settings
	DefaultNavLockAfterLoad      = true
	DefaultBasicInteraction      = false
	DefaultGremlins              = false
	DefaultTriggerEventListeners = false

	// Defaults for data gathering settings
	DefaultAllResources     = true
	DefaultAllScripts       = false
	DefaultCookies          = true
	DefaultDOM              = false
	DefaultResourceMetadata = true
	DefaultScreenshot       = true
	DefaultScriptMetadata   = false
	DefaultBrowserCoverage  = false

	DefaultShuffle = true // Whether to shuffle order of task processing

	DefaultProtocolPrefix = "https://" // If no protocol is provided, we use https for the crawl
)

var (

	// Flags we apply by default to Chrome/Chromium-based browsers
	DefaultChromiumBrowserFlags = []string{
		"--enable-features=NetworkService",
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
)
