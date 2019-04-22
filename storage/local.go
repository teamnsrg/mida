package storage

import (
	"encoding/json"
	"errors"
	"github.com/teamnsrg/mida/log"
	t "github.com/teamnsrg/mida/types"
	"io/ioutil"
	"os"
	"path"
)

// Given a valid FinalMIDAResult, stores it according to the output
// path specified in the sanitized task within the result
func StoreResultsLocalFS(r *t.FinalMIDAResult, outpath string) error {
	_, err := os.Stat(outpath)
	if err != nil {
		err = os.MkdirAll(outpath, 0755)
		if err != nil {
			log.Log.Error("Failed to create local output directory")
			return errors.New("failed to create local output directory")
		}
	} else {
		log.Log.Error("Output directory for task already exists")
		return errors.New("output directory for task already exists")
	}

	// Store metadata from this task to a JSON file
	data, err := json.Marshal(r.Metadata)
	if err != nil {
		log.Log.Error(err)
	} else {
		err = ioutil.WriteFile(path.Join(outpath, DefaultCrawlMetadataFile), data, 0644)
		if err != nil {
			log.Log.Error(err)
		}
	}

	// Store resource metadata from crawl (DevTools requestWillBeSent and responseReceived data)
	if r.SanitizedTask.ResourceMetadata {
		data, err := json.Marshal(r.ResourceMetadata)
		if err != nil {
			log.Log.Error(err)
		} else {
			err = ioutil.WriteFile(path.Join(outpath, DefaultResourceMetadataFile), data, 0644)
			if err != nil {
				log.Log.Error(err)
			}
		}
	}

	// Store raw resources downloaded during crawl (named for their request IDs)
	if r.SanitizedTask.AllResources {
		_, err = os.Stat(path.Join(r.SanitizedTask.UserDataDirectory, r.SanitizedTask.RandomIdentifier, DefaultFileSubdir))
		if err != nil {
			log.Log.Error("AllResources requested but no files directory exists within temporary results directory")
			log.Log.Error("Files will not be stored")
		} else {
			err = os.Rename(path.Join(r.SanitizedTask.UserDataDirectory, r.SanitizedTask.RandomIdentifier, DefaultFileSubdir),
				path.Join(outpath, DefaultFileSubdir))
			if err != nil {
				log.Log.Fatal(err)
			}
		}
	}

	if r.SanitizedTask.ScriptMetadata {
		data, err := json.Marshal(r.ScriptMetadata)
		if err != nil {
			log.Log.Error(err)
		} else {
			err = ioutil.WriteFile(path.Join(outpath, DefaultScriptMetadataFile), data, 0644)
			if err != nil {
				log.Log.Error(err)
			}
		}
	}

	// Store raw scripts parsed by the browser during crawl (named by hashes)
	if r.SanitizedTask.AllScripts {
		_, err = os.Stat(path.Join(r.SanitizedTask.UserDataDirectory, r.SanitizedTask.RandomIdentifier, DefaultScriptSubdir))
		if err != nil {
			log.Log.Error("AllScripts requested but no files directory exists within temporary results directory")
			log.Log.Error("Scripts will not be stored")
		} else {
			err = os.Rename(path.Join(r.SanitizedTask.UserDataDirectory, r.SanitizedTask.RandomIdentifier, DefaultScriptSubdir),
				path.Join(outpath, DefaultScriptSubdir))
			if err != nil {
				log.Log.Fatal(err)
			}
		}
	}

	if r.SanitizedTask.NetworkTrace {
		_, err = os.Stat(path.Join(r.SanitizedTask.UserDataDirectory, r.SanitizedTask.RandomIdentifier, DefaultNetworkStraceFileName))
		if err != nil {
			log.Log.Error("Expected to find network strace file, but did not")
		} else {
			err = os.Rename(path.Join(r.SanitizedTask.UserDataDirectory, r.SanitizedTask.RandomIdentifier, DefaultNetworkStraceFileName),
				path.Join(outpath, DefaultNetworkStraceFileName))
			if err != nil {
				log.Log.Error(err)
			}
		}
	}

	if r.SanitizedTask.JSTrace {
		data, err := json.Marshal(r.JSTrace)
		if err != nil {
			log.Log.Error(err)
		} else {
			err = ioutil.WriteFile(path.Join(outpath, DefaultJSTracePath), data, 0644)
			if err != nil {
				log.Log.Error(err)
			}
		}

		if r.SanitizedTask.SaveRawTrace {
			err = os.Rename(path.Join(r.SanitizedTask.UserDataDirectory, r.SanitizedTask.RandomIdentifier, DefaultBrowserLogFileName),
				path.Join(outpath, DefaultBrowserLogFileName))
			if err != nil {
				log.Log.Error(err)
			}
		}
	}

	if r.SanitizedTask.ResourceTree {
		data, err := json.Marshal(r.RTree)
		if err != nil {
			log.Log.Error(err)
		} else {
			err = ioutil.WriteFile(path.Join(outpath, DefaultResourceTreePath), data, 0644)
			if err != nil {
				log.Log.Error(err)
			}
		}
	}

	// Store Websocket data (if specified)
	if r.SanitizedTask.WebsocketTraffic {
		data, err := json.Marshal(r.WebsocketData)
		if err != nil {
			log.Log.Error(err)
		} else {
			err = ioutil.WriteFile(path.Join(outpath, DefaultWebSocketTrafficFile), data, 0644)
			if err != nil {
				log.Log.Error(err)
			}
		}
	}

	if r.SanitizedTask.BrowserCoverage {
		_, err = os.Stat(path.Join(r.SanitizedTask.UserDataDirectory, r.SanitizedTask.RandomIdentifier, DefaultCoverageSubdir))
		if err != nil {
			log.Log.Error("Coverage Data requested but no coverage directory exists within temporary results directory")
			log.Log.Error("Coverage data will not be stored")
		} else {
			err = os.Rename(path.Join(r.SanitizedTask.UserDataDirectory, r.SanitizedTask.RandomIdentifier, DefaultCoverageSubdir),
				path.Join(outpath, DefaultCoverageSubdir))
			if err != nil {
				log.Log.Fatal(err)
			}
		}
	}

	return nil
}
