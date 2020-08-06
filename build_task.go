package main

import (
	"bufio"
	"errors"
	"github.com/spf13/cobra"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/sanitize"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func BuildCompressedTaskSet(cmd *cobra.Command, args []string) (*b.CompressedTaskSet, error) {
	ts := b.AllocateNewCompressedTaskSet()
	var err error

	if cmd.Name() == "go" {
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
	} else if cmd.Name() == "build" {
		// Get URL from URL file
		fName, err := cmd.Flags().GetString("url-file")
		if err != nil {
			return nil, err
		}

		urlFile, err := os.Open(fName)
		if err != nil {
			return nil, err
		}
		defer urlFile.Close()

		scanner := bufio.NewScanner(urlFile)
		for scanner.Scan() {
			u, err := sanitize.ValidateURL(scanner.Text())
			if err != nil {
				return nil, err
			}
			*ts.URL = append(*ts.URL, u)
		}
	} else {
		return nil, errors.New("unknown command passed to BuildCompressedTaskSet()")
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
