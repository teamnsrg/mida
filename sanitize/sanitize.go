package sanitize

import (
	"errors"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	b "github.com/teamnsrg/mida/base"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// Task takes a raw tasks, checks it for validity, adds default values as needed,
// and creates a TaskWrapper object for it so it can be passed on for the site visit.
func Task(rt *b.RawTask) (b.TaskWrapper, error) {
	var tw b.TaskWrapper
	var err error

	// Each task gets its own UUID
	tw.UUID = uuid.New()

	tmpDir, err := filepath.Abs(path.Join(ExpandPath(viper.GetString("tempdir")), tw.UUID.String()))
	if err != nil {
		return b.TaskWrapper{}, err
	}
	tw.TempDir = tmpDir

	err = os.MkdirAll(tw.TempDir, 0755)
	if err != nil {
		return b.TaskWrapper{}, errors.New("failed to create temporary directory for task: " + err.Error())
	}

	// Create our log for this specific site visit
	tw.Log = logrus.New()
	tw.Log.SetLevel(logrus.DebugLevel)
	tw.LogFile, err = os.OpenFile(path.Join(tw.TempDir, b.DefaultTaskLogFile), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return b.TaskWrapper{}, errors.New("failed to create log file: " + err.Error())
	}
	tw.Log.SetOutput(tw.LogFile)

	if rt.URL == nil || *rt.URL == "" {
		return b.TaskWrapper{}, errors.New("missing or empty URL for task")
	}

	tw.SanitizedTask.URL, err = ValidateURL(*rt.URL)
	if err != nil {
		return b.TaskWrapper{}, err
	}
	tw.Log.Infof("initiated log for: %s", tw.SanitizedTask.URL)

	tw.SanitizedTask.BrowserBinaryPath, err = getBrowserBinaryPath(rt, tw.Log)
	if err != nil {
		return b.TaskWrapper{}, err
	}

	tw.SanitizedTask.BrowserFlags, err = getBrowserFlags(rt, tw.Log)
	if err != nil {
		return b.TaskWrapper{}, err
	}

	tw.SanitizedTask.UserDataDirectory, err = getUserDataDirectory(rt, tw.TempDir)
	if err != nil {
		return b.TaskWrapper{}, err
	} else {
		tw.Log.Debugf("set user data directory: %s", tw.SanitizedTask.UserDataDirectory)
	}

	tw.SanitizedTask.IS, err = InteractionSettings(rt)
	if err != nil {
		return b.TaskWrapper{}, err
	}

	tw.SanitizedTask.CS, err = CompletionSettings(rt.Completion)
	if err != nil {
		return b.TaskWrapper{}, err
	}

	tw.SanitizedTask.DS, err = DataSettings(rt.Data, nil)
	if err != nil {
		return b.TaskWrapper{}, err
	}

	tw.SanitizedTask.OPS, err = OutputSettings(rt.Output, &tw.SanitizedTask.DS)
	if err != nil {
		return b.TaskWrapper{}, err
	}

	return tw, nil
}

func InteractionSettings(rt *b.RawTask) (b.InteractionSettings, error) {
	result := b.AllocateNewInteractionSettings()

	if rt == nil || rt.Browser == nil || rt.Browser.InteractionSettings == nil {
		return *result, nil
	}

	is := rt.Browser.InteractionSettings

	if is.LockNavigation != nil {
		*result.LockNavigation = *is.LockNavigation
	}

	if is.BasicInteraction != nil {
		*result.BasicInteraction = *is.BasicInteraction
	}

	if is.Gremlins != nil {
		*result.Gremlins = *is.Gremlins
	}

	if is.TriggerEventListeners != nil {
		*result.TriggerEventListeners = *is.TriggerEventListeners
	}

	return *result, nil

}

// getBrowserBinaryPath uses input from the task to sanitize and set the full path to the browser
// binary we will use for this crawl. If an invalid path is provided, it returns an error. If no
// path is provided, it attempts to select a default.
// Order of preference:
//   1. Chromium
//   2. Chrome
func getBrowserBinaryPath(rt *b.RawTask, log *logrus.Logger) (string, error) {

	if rt.Browser == nil || rt.Browser.BrowserBinary == nil || *rt.Browser.BrowserBinary == "" {
		if runtime.GOOS == "darwin" {
			if _, err := os.Stat(b.DefaultOSXChromiumPath); err == nil {
				log.Debugf("no browser set, defaulting to to %s", b.DefaultOSXChromiumPath)
				return b.DefaultOSXChromiumPath, nil
			} else if _, err := os.Stat(b.DefaultOSXChromePath); err == nil {
				log.Debugf("no browser set, defaulting to to %s", b.DefaultOSXChromePath)
				return b.DefaultOSXChromePath, nil
			} else {
				return "", errors.New("no browser binary provided and could not find a default")
			}
		} else if runtime.GOOS == "linux" {
			if _, err := os.Stat(b.DefaultLinuxChromiumPath); err == nil {
				log.Debugf("no browser set, defaulting to to %s", b.DefaultLinuxChromiumPath)
				return b.DefaultLinuxChromiumPath, nil
			} else if _, err := os.Stat(b.DefaultLinuxChromePath); err == nil {
				log.Debugf("no browser set, defaulting to to %s", b.DefaultLinuxChromePath)
				return b.DefaultLinuxChromePath, nil
			} else {
				return "", errors.New("no browser binary provided and could not find a default")
			}
		} else if runtime.GOOS == "windows" {
			if _, err := os.Stat(b.DefaultWindowsChromiumPath); err == nil {
				log.Debugf("no browser set, defaulting to to %s", b.DefaultWindowsChromiumPath)
				return b.DefaultWindowsChromiumPath, nil
			} else if _, err := os.Stat(b.DefaultWindowsChromePath); err == nil {
				log.Debugf("no browser set, defaulting to to %s", b.DefaultWindowsChromePath)
				return b.DefaultWindowsChromePath, nil
			} else {
				return "", errors.New("no browser binary provided and could not find a default")
			}
		} else {
			return "", errors.New("this operating system is not supported by MIDA (MIDA supports Windows, Linux, Mac OS)")
		}
	} else {
		_, err := os.Stat(*rt.Browser.BrowserBinary)
		if err == nil {
			log.Debugf("set browser to %s", *rt.Browser.BrowserBinary)
			return *rt.Browser.BrowserBinary, nil
		}

		// We were given a browser string that is not a path to a file that exists
		// We offer some shortcuts for popular browsers
		if strings.ToLower(*rt.Browser.BrowserBinary) == "chrome" {
			if _, err := os.Stat(b.DefaultOSXChromePath); err == nil && runtime.GOOS == "darwin" {
				log.Debugf("set browser to %s", b.DefaultOSXChromePath)
				return b.DefaultOSXChromePath, nil
			} else if _, err := os.Stat(b.DefaultLinuxChromePath); err == nil && runtime.GOOS == "linux" {
				log.Debugf("set browser to %s", b.DefaultLinuxChromePath)
				return b.DefaultLinuxChromePath, nil
			} else if _, err := os.Stat(b.DefaultWindowsChromePath); err == nil && runtime.GOOS == "windows" {
				log.Debugf("set browser to %s", b.DefaultWindowsChromePath)
				return b.DefaultWindowsChromePath, nil
			} else {
				return "", errors.New("could not find chrome on the system")
			}
		} else if strings.ToLower(*rt.Browser.BrowserBinary) == "chromium" ||
			strings.ToLower(*rt.Browser.BrowserBinary) == "chromium-browser" {
			if _, err := os.Stat(b.DefaultOSXChromiumPath); err == nil && runtime.GOOS == "darwin" {
				log.Debugf("set browser to %s", b.DefaultOSXChromiumPath)
				return b.DefaultOSXChromiumPath, nil
			} else if _, err := os.Stat(b.DefaultLinuxChromiumPath); err == nil && runtime.GOOS == "linux" {
				log.Debugf("set browser to %s", b.DefaultLinuxChromiumPath)
				return b.DefaultLinuxChromiumPath, nil
			} else if _, err := os.Stat(b.DefaultWindowsChromiumPath); err == nil && runtime.GOOS == "windows" {
				log.Debugf("set browser to %s", b.DefaultWindowsChromiumPath)
				return b.DefaultWindowsChromiumPath, nil
			} else {
				return "", errors.New("could not find chrome on the system")
			}
		} else {
			return "", errors.New("could not find browser: " + *rt.Browser.BrowserBinary)
		}
	}
}

// getBrowserFlags uses the flag and extension settings passed in in the RawTask to create a single string
// slice with the flags we will use for our browser. Note that this slice will not include the specific
// flag which allows remote control of the browser. This flag will be added in Stage 3.
func getBrowserFlags(rt *b.RawTask, log *logrus.Logger) ([]string, error) {
	result := make([]string, 0)

	if rt.Browser == nil {
		return b.DefaultChromiumBrowserFlags, nil
	}

	// We make copies of these two so we can manipulate them without altering the raw task
	abf := make([]string, 0)
	if rt.Browser.AddBrowserFlags != nil {
		abf = append(abf, *rt.Browser.AddBrowserFlags...)
	}
	rbf := make([]string, 0)
	if rt.Browser.RemoveBrowserFlags != nil {
		rbf = append(rbf, *rt.Browser.RemoveBrowserFlags...)
	}

	if rt.Browser.Extensions != nil && len(*rt.Browser.Extensions) != 0 {
		// Check that each extension exists
		for _, e := range *rt.Browser.Extensions {
			x, err := os.Stat(e)
			if err != nil {
				return []string{}, err
			}
			if !x.IsDir() {
				return []string{}, errors.New("given extension [ " + e + " ] is not a directory")
			}
		}

		// Create the extensions flag
		extensionsFlag := "--disable-extensions-except="
		extensionsFlag += (*rt.Browser.Extensions)[0]
		if len(*rt.Browser.Extensions) > 1 {
			for _, e := range (*rt.Browser.Extensions)[1:] {
				extensionsFlag += ","
				extensionsFlag += e
				log.Debugf("adding extension: %s", e)
			}
		}

		abf = append(abf, extensionsFlag)

		// Remove the --incognito and --disable-extensions (both prevent extensions)
		rbf = append(rbf, "--incognito")
		rbf = append(rbf, "--disable-extensions")
	}

	if rt.Browser.SetBrowserFlags != nil && len(*rt.Browser.SetBrowserFlags) != 0 {

		for _, flag := range *rt.Browser.SetBrowserFlags {
			result = append(result, flag)
		}
	} else {
		// Add flags, checking to see that they have not been removed
		for _, flag := range append(b.DefaultChromiumBrowserFlags, abf...) {
			found := false
			for _, excluded := range rbf {
				if flag == excluded {
					found = true
				}
			}
			if !found {
				result = append(result, flag)
			}
		}
	}

	return result, nil
}

// getUserDataDirectory reads a raw task. If the task specifies a valid user data directory, it is
// returned. Otherwise, getUserDataDirectory selects a default directory based on the task UUID
func getUserDataDirectory(rt *b.RawTask, tempDir string) (string, error) {
	if rt.Browser != nil && rt.Browser.UserDataDirectory != nil && *rt.Browser.UserDataDirectory != "" {
		return *rt.Browser.UserDataDirectory, nil
	} else {
		// Use the first 8 characters of the uuid for temporary directories by default
		return tempDir + "-udd", nil
	}
}

// CompletionSettings takes a raw CompletionSettings struct and sanitizes it
func CompletionSettings(cs *b.CompletionSettings) (b.CompletionSettings, error) {
	result := b.AllocateNewCompletionSettings()

	if cs == nil {
		*result.CompletionCondition = b.DefaultCompletionCondition
		*result.Timeout = b.DefaultTimeout
		*result.TimeAfterLoad = b.DefaultTimeAfterLoad
		return *result, nil
	}

	if cs.CompletionCondition == nil {
		*result.CompletionCondition = b.DefaultCompletionCondition
	} else {
		for _, cc := range b.CompletionConditions {
			if cc == *cs.CompletionCondition {
				*result.CompletionCondition = *cs.CompletionCondition
			}
		}

		if *result.CompletionCondition == "" {
			return b.CompletionSettings{}, errors.New("invalid completion condition")
		}
	}

	if cs.Timeout == nil {
		*result.Timeout = b.DefaultTimeout
	} else {
		if *cs.Timeout <= 0 {
			return b.CompletionSettings{}, errors.New("timeout value must be positive")
		} else {
			*result.Timeout = *cs.Timeout
		}
	}

	if cs.TimeAfterLoad == nil {
		*result.TimeAfterLoad = b.DefaultTimeAfterLoad
	} else {
		if *cs.TimeAfterLoad < 0 {
			return b.CompletionSettings{}, errors.New("time_after_load value must be non-negative")
		} else {
			*result.TimeAfterLoad = *cs.TimeAfterLoad
		}
	}

	return *result, nil
}

// DataSettings allocates and sanitizes a  new DataSettings object by searching
func DataSettings(rawDataSettings *b.DataSettings, parentSettings *b.DataSettings) (b.DataSettings, error) {
	result := b.AllocateNewDataSettings()

	*result.ResourceMetadata = b.DefaultResourceMetadata
	if parentSettings != nil && parentSettings.ResourceMetadata != nil {
		*result.ResourceMetadata = *parentSettings.ResourceMetadata
	}
	if rawDataSettings != nil && rawDataSettings.ResourceMetadata != nil {
		*result.ResourceMetadata = *rawDataSettings.ResourceMetadata
	}

	*result.ScriptMetadata = b.DefaultScriptMetadata
	if parentSettings != nil && parentSettings.ScriptMetadata != nil {
		*result.ScriptMetadata = *parentSettings.ScriptMetadata
	}
	if rawDataSettings != nil && rawDataSettings.ScriptMetadata != nil {
		*result.ScriptMetadata = *rawDataSettings.ScriptMetadata
	}

	*result.AllResources = b.DefaultAllResources
	if parentSettings != nil && parentSettings.AllResources != nil {
		*result.AllResources = *parentSettings.AllResources
	}
	if rawDataSettings != nil && rawDataSettings.AllResources != nil {
		*result.AllResources = *rawDataSettings.AllResources
	}

	*result.AllScripts = b.DefaultAllScripts
	if parentSettings != nil && parentSettings.AllScripts != nil {
		*result.AllScripts = *parentSettings.AllScripts
	}
	if rawDataSettings != nil && rawDataSettings.AllScripts != nil {
		*result.AllScripts = *rawDataSettings.AllScripts
	}

	*result.Screenshot = b.DefaultScreenshot
	if parentSettings != nil && parentSettings.Screenshot != nil {
		*result.Screenshot = *parentSettings.Screenshot
	}
	if rawDataSettings != nil && rawDataSettings.Screenshot != nil {
		*result.Screenshot = *rawDataSettings.Screenshot
	}

	*result.Cookies = b.DefaultCookies
	if parentSettings != nil && parentSettings.Cookies != nil {
		*result.Cookies = *parentSettings.Cookies
	}
	if rawDataSettings != nil && rawDataSettings.Cookies != nil {
		*result.Cookies = *rawDataSettings.Cookies
	}

	*result.DOM = b.DefaultDOM
	if parentSettings != nil && parentSettings.DOM != nil {
		*result.DOM = *parentSettings.DOM
	}
	if rawDataSettings != nil && rawDataSettings.DOM != nil {
		*result.DOM = *rawDataSettings.DOM
	}

	*result.JavaScriptCoverage = b.DefaultJavaScriptCoverage
	if parentSettings != nil && parentSettings.JavaScriptCoverage != nil {
		*result.JavaScriptCoverage = *parentSettings.JavaScriptCoverage
	}
	if rawDataSettings != nil && rawDataSettings.JavaScriptCoverage != nil {
		*result.JavaScriptCoverage = *rawDataSettings.JavaScriptCoverage
	}

	*result.BrowserCoverage = b.DefaultBrowserCoverage
	if parentSettings != nil && parentSettings.BrowserCoverage != nil {
		*result.BrowserCoverage = *parentSettings.BrowserCoverage
	}
	if rawDataSettings != nil && rawDataSettings.BrowserCoverage != nil {
		*result.BrowserCoverage = *rawDataSettings.BrowserCoverage
	}

	return *result, nil
}

// OutputSettings takes in a set of output settings, along with some default data
// settings, ensures validity, and returns a newly/fully allocated set of sanitized OutputSettings
func OutputSettings(ops *b.OutputSettings, ds *b.DataSettings) (b.OutputSettings, error) {
	result := b.AllocateNewOutputSettings()
	var err error

	// If no output settings are provided, we default to providing local filesystem output only
	if ops == nil {
		*result.LocalOut.Enable = true
		*result.LocalOut.Path = b.DefaultLocalOutputPath
		*result.LocalOut.DS, err = DataSettings(nil, ds)
		if err != nil {
			return b.OutputSettings{}, err
		}
		return *result, nil
	}

	result.LocalOut, err = LocalOutputSettings(ops.LocalOut, ds)
	if err != nil {
		return b.OutputSettings{}, err
	}
	result.SftpOut, err = SftpOutputSettings(ops.SftpOut, ds)
	if err != nil {
		return b.OutputSettings{}, err
	}

	if ops.PostQueue == nil {
		*result.PostQueue = ""
	} else {
		*result.PostQueue = *ops.PostQueue
	}

	return *result, nil
}

func LocalOutputSettings(los *b.LocalOutputSettings, defaultSettings *b.DataSettings) (*b.LocalOutputSettings, error) {
	var err error
	result := b.AllocateNewLocalOutputSettings()
	if los == nil {
		*result.Enable = false
		return result, nil
	}

	if los.Enable != nil {
		*(result.Enable) = *los.Enable
	} else {
		*(result.Enable) = *los.Enable
	}

	if los.Path != nil {
		*result.Path = ExpandPath(*los.Path)
	} else {
		*result.Path = b.DefaultLocalOutputPath
	}

	*result.DS, err = DataSettings(los.DS, defaultSettings)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func SftpOutputSettings(sos *b.SftpOutputSettings, defaultSettings *b.DataSettings) (*b.SftpOutputSettings, error) {
	var err error
	result := b.AllocateNewSftpOutputSettings()

	// Sftp will not be used, just return a disabled SftpOutputSettings object
	if sos == nil || sos.Enable == nil || *sos.Enable == false {
		*(result.Enable) = false
		return result, nil
	}

	*(result.Enable) = *(sos.Enable)

	if sos.Path == nil || sos.Host == nil {
		return nil, errors.New("required field for SFTP output not specified")
	}
	*(result.Path) = *(sos.Path)
	*(result.Host) = *(sos.Host)

	if sos.Port == nil {
		*(result.Port) = 22
	} else {
		*(result.Port) = *(sos.Port)
	}

	if sos.UserName == nil {
		u, err := user.Current()
		if err != nil {
			return nil, errors.New("failed to determine current user")
		}
		*(result.UserName) = u.Username
	} else {
		*(result.UserName) = *(sos.UserName)
	}

	if sos.PrivateKeyFile == nil {
		*(result.PrivateKeyFile) = b.DefaultSftpPrivKeyFile
	} else {
		*(result.PrivateKeyFile) = ExpandPath(*(sos.PrivateKeyFile))
	}

	*result.DS, err = DataSettings(sos.DS, defaultSettings)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ValidateURL makes a best-effort pass at validating/fixing URLs
func ValidateURL(s string) (string, error) {
	var result string
	u, err := url.ParseRequestURI(s)
	if err != nil {
		if !strings.Contains(s, "://") {
			u, err = url.ParseRequestURI(b.DefaultProtocolPrefix + s)
			if err != nil {
				return result, errors.New("bad url: " + s)
			}
		} else {
			return result, errors.New("bad url: " + s)
		}
	}

	return u.String(), nil
}

func ExpandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		u, err := user.Current()
		if err != nil {
			return filepath.FromSlash(p)
		}
		return filepath.FromSlash(path.Join(u.HomeDir, p[2:]))

	} else {
		return filepath.FromSlash(p)
	}
}
