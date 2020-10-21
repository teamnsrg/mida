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

	// Store metadata for every crawl
	data, err := json.Marshal(finalResult.Summary)
	if err != nil {
		return errors.New("failed to marshal metadata for storage: " + err.Error())
	}
	err = ioutil.WriteFile(path.Join(outPath, b.DefaultMetadataFile), data, 0644)
	if err != nil {
		return errors.New("failed to write metadata file: " + err.Error())
	}

	if *dataSettings.ResourceMetadata {
		data, err := json.Marshal(finalResult.DTResourceMetadata)
		if err != nil {
			return errors.New("failed to marshal resource data for storage: " + err.Error())
		}

		err = ioutil.WriteFile(path.Join(outPath, b.DefaultResourceMetadataFile), data, 0644)
		if err != nil {
			return errors.New("failed to write resource metadata file: " + err.Error())
		}
	}

	if *dataSettings.ScriptMetadata {
		data, err := json.Marshal(finalResult.DTScriptMetadata)
		if err != nil {
			return errors.New("failed to marshal script metadata for storage: " + err.Error())
		}

		err = ioutil.WriteFile(path.Join(outPath, b.DefaultScriptMetadataFile), data, 0644)
		if err != nil {
			return errors.New("failed to write script metadata file: " + err.Error())
		}
	}

	if *dataSettings.AllResources {
		err = os.Rename(path.Join(tw.TempDir, b.DefaultResourceSubdir), path.Join(outPath, b.DefaultResourceSubdir))
		if err != nil {
			tw.Log.Error("failed to copy resources directory into results directory: " + err.Error())
			log.Log.Error("failed to copy resources directory into results directory: " + err.Error())
		}
	}

	if *dataSettings.AllScripts {
		err = os.Rename(path.Join(tw.TempDir, b.DefaultScriptSubdir), path.Join(outPath, b.DefaultScriptSubdir))
		if err != nil {
			tw.Log.Error("failed to copy scripts directory into results directory: " + err.Error())
			log.Log.Error("failed to copy scripts directory into results directory: " + err.Error())
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
			return errors.New("failed to marshal cookies for storage")
		}

		err = ioutil.WriteFile(path.Join(outPath, b.DefaultCookieFileName), data, 0644)
		if err != nil {
			return errors.New("failed to write cookie json to file")
		}
	}

	if *dataSettings.DOM {
		data, err := json.Marshal(finalResult.DTDOM)
		if err != nil {
			return errors.New("failed to marshal dom for storage")
		}

		err = ioutil.WriteFile(path.Join(outPath, b.DefaultDomFileName), data, 0644)
		if err != nil {
			return errors.New("failed to write DOM json to file")
		}
	}

	if *dataSettings.VV8 {
		data, err := json.Marshal(finalResult.DTVV8IsolateMap)
		if err != nil {
			return errors.New("failed to marshal vv8 data")
		}

		err = ioutil.WriteFile(path.Join(outPath, b.DefaultVV8FileName), data, 0644)
		if err != nil {
			return errors.New("failed to write vv8 file")
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
