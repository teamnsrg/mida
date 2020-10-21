package main

import (
	"bufio"
	"errors"
	"github.com/spf13/cobra"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/sanitize"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func BuildCompressedTaskSet(cmd *cobra.Command, args []string) (*b.CompressedTaskSet, error) {
	ts := b.AllocateNewCompressedTaskSet()
	var err error

	fName, err := cmd.Flags().GetString("url-file")
	if err != nil {
		return nil, err
	}
	maxUrls, err := cmd.Flags().GetInt("max-urls")
	if err != nil {
		return nil, err
	}

	if len(args) > 0 && fName != "" {
		return nil, errors.New("cannot use both arguments and file for URLs")
	}

	if len(args) > 0 {
		// Get URLs from arguments
		for _, arg := range args {
			pieces := strings.Split(arg, ",")
			for _, piece := range pieces {
				u, err := sanitize.ValidateURL(piece)
				if err != nil {
					return ts, err
				}
				*ts.URL = append(*ts.URL, u)
			}
		}
	} else if fName != "" {
		// Get URL from URL file
		var fileName string
		download := false
		if strings.HasPrefix(fName, "http://") || strings.HasPrefix(fName, "https://") {
			download = true
			resp, err := http.Get(fName)
			if err != nil {
				return nil, err
			}

			parts := strings.Split(fName, "/")
			shortFName := parts[len(parts)-1]
			fileName = shortFName
			out, err := os.Create(fileName)
			if err != nil {
				return nil, err
			}

			_, err = io.Copy(out, resp.Body)
			if err != nil {
				return nil, err
			}

			err = resp.Body.Close()
			if err != nil {
				return nil, err
			}

			err = out.Close()
			if err != nil {
				return nil, err
			}

		} else {
			fileName = fName
		}

		urlFile, err := os.Open(fileName)
		if err != nil {
			return nil, err
		}

		scanner := bufio.NewScanner(urlFile)
		for scanner.Scan() && maxUrls != 0 {
			u, err := sanitize.ValidateURL(scanner.Text())
			if err != nil {
				return nil, err
			}
			*ts.URL = append(*ts.URL, u)
			maxUrls -= 1
		}

		err = urlFile.Close()
		if err != nil {
			return nil, err
		}

		if download {
			err = os.Remove(fileName)
			if err != nil {
				return nil, err
			}
		}
	} else {
		return nil, errors.New("no urls specified (use arguments or URL file option)")
	}

	*ts.Browser.BrowserBinary, err = cmd.Flags().GetString("browser")
	if err != nil {
		return nil, err
	}
	*ts.Browser.UserDataDirectory, err = cmd.Flags().GetString("user-data-dir")
	if err != nil {
		return nil, err
	}
	*ts.Browser.AddBrowserFlags, err = cmd.Flags().GetStringSlice("add-browser-flags")
	if err != nil {
		return nil, err
	}
	*ts.Browser.RemoveBrowserFlags, err = cmd.Flags().GetStringSlice("remove-browser-flags")
	if err != nil {
		return nil, err
	}
	*ts.Browser.SetBrowserFlags, err = cmd.Flags().GetStringSlice("set-browser-flags")
	if err != nil {
		return nil, err
	}
	*ts.Browser.Extensions, err = cmd.Flags().GetStringSlice("extensions")
	if err != nil {
		return nil, err
	}

	*ts.Browser.InteractionSettings.LockNavigation, err = cmd.Flags().GetBool("nav-lock")
	if err != nil {
		return nil, err
	}
	*ts.Browser.InteractionSettings.BasicInteraction, err = cmd.Flags().GetBool("basic-interaction")
	if err != nil {
		return nil, err
	}
	*ts.Browser.InteractionSettings.Gremlins, err = cmd.Flags().GetBool("gremlins")
	if err != nil {
		return nil, err
	}
	*ts.Browser.InteractionSettings.TriggerEventListeners, err = cmd.Flags().GetBool("trigger-event-listeners")
	if err != nil {
		return nil, err
	}

	*ts.Completion.Timeout, err = cmd.Flags().GetInt("timeout")
	if err != nil {
		return nil, err
	}
	*ts.Completion.TimeAfterLoad, err = cmd.Flags().GetInt("time-after-load")
	if err != nil {
		return nil, err
	}
	CCString, err := cmd.Flags().GetString("completion")
	if err != nil {
		return nil, err
	}
	*ts.Completion.CompletionCondition = b.CompletionCondition(CCString)

	*ts.Data.AllResources, err = cmd.Flags().GetBool("all-resources")
	if err != nil {
		return ts, err
	}
	*ts.Data.AllScripts, err = cmd.Flags().GetBool("all-scripts")
	if err != nil {
		return ts, err
	}
	*ts.Data.Cookies, err = cmd.Flags().GetBool("cookies")
	if err != nil {
		return nil, err
	}
	*ts.Data.DOM, err = cmd.Flags().GetBool("dom")
	if err != nil {
		return nil, err
	}
	*ts.Data.ResourceMetadata, err = cmd.Flags().GetBool("resource-metadata")
	if err != nil {
		return nil, err
	}
	*ts.Data.Screenshot, err = cmd.Flags().GetBool("screenshot")
	if err != nil {
		return nil, err
	}
	*ts.Data.ScriptMetadata, err = cmd.Flags().GetBool("script-metadata")
	if err != nil {
		return nil, err
	}
	*ts.Data.VV8, err = cmd.Flags().GetBool("vv8")
	if err != nil {
		return nil, err
	}

	// Output settings, either local or remote
	resultsOutputPath, err := cmd.Flags().GetString("results-output-path")
	if err != nil {
		return nil, err
	}

	if resultsOutputPath == "none" {
		*ts.Output.LocalOut.Enable = false
		*ts.Output.SftpOut.Enable = false
	} else if strings.Contains(resultsOutputPath, "ssh://") {
		*ts.Output.SftpOut.Enable = true
		remoteUrl, err := url.Parse(resultsOutputPath)
		if err != nil {
			return nil, err
		}
		// Url library includes port in host, we want to remove it here
		*ts.Output.SftpOut.Host = strings.Split(remoteUrl.Host, ":")[0]
		if remoteUrl.Port() == "" {
			*ts.Output.SftpOut.Port = 22
		} else {
			*ts.Output.SftpOut.Port, err = strconv.Atoi(remoteUrl.Port())
		}
		if err != nil {
			return nil, err
		}
		*ts.Output.SftpOut.UserName = remoteUrl.User.String() //Blank if not specified, will be set to default later
		*ts.Output.SftpOut.Path = remoteUrl.Path
		*ts.Output.SftpOut.DS = *ts.Data
	} else {
		*ts.Output.LocalOut.Enable = true
		*ts.Output.LocalOut.Path = resultsOutputPath
		*ts.Output.LocalOut.DS = *ts.Data
	}

	*ts.Repeat, err = cmd.Flags().GetInt("repeat")
	if err != nil {
		return nil, err
	}

	return ts, nil
}
