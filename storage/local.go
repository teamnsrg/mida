package storage

import (
	"encoding/json"
	"errors"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"io/ioutil"
	"os"
	"path"
)

// Local stores the results of a site visit locally, returning the path
// to the results along with an error
func Local(finalResult *b.FinalResult, dataSettings *b.DataSettings, outPath string) error {

	// For brevity
	tw := finalResult.Summary.TaskWrapper

	_, err := os.Stat(outPath)
	if err != nil {
		err = os.MkdirAll(outPath, 0755)
		if err != nil {
			return errors.New("failed to create local output directory: " + err.Error())
		}
	} else {
		return errors.New("task local output directory exists")
	}

	if *dataSettings.ResourceMetadata {
		data, err := json.Marshal(finalResult.DTResourceMetadata)
		if err != nil {
			return errors.New("failed to marshal resource data for local storage: " + err.Error())
		}

		err = ioutil.WriteFile(path.Join(outPath, b.DefaultResourceMetadataFile), data, 0644)
		if err != nil {
			return errors.New("failed to write resource metadata file: " + err.Error())
		}
	}

	if *dataSettings.AllResources {
		err = os.Rename(path.Join(tw.TempDir, b.DefaultResourceSubdir), path.Join(outPath, b.DefaultResourceSubdir))
		if err != nil {
			tw.Log.Error("failed to copy resources directory into results directory: " + err.Error())
			log.Log.Error("failed to copy resources directory into results directory: " + err.Error())
		}
	}

	if *dataSettings.Screenshot {
		err = os.Rename(path.Join(tw.TempDir, b.DefaultScreenshotFileName), path.Join(outPath, b.DefaultScreenshotFileName))
		if err != nil {
			tw.Log.Warn("screenshot was not gathered -- load event probably never fired")
		}
	}

	if *dataSettings.Cookies {
		data, err := json.Marshal(finalResult.DTCookies)
		if err != nil {
			return errors.New("failed to marshal cookies for local storage")
		}

		err = ioutil.WriteFile(path.Join(outPath, b.DefaultCookieFileName), data, 0644)
		if err != nil {
			return errors.New("failed to write cookie json to file")
		}
	}

	// Store our log
	err = tw.LogFile.Close()
	if err != nil {
		log.Log.Error(err)
	}
	err = os.Rename(tw.LogFile.Name(), path.Join(outPath, b.DefaultTaskLogFile))
	if err != nil {
		log.Log.Error("failed to store log file")
	}

	return nil
}
