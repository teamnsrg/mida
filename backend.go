package main

import (
	"errors"
	"github.com/spf13/viper"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/storage"
	t "github.com/teamnsrg/mida/types"
	"github.com/teamnsrg/mida/util"
	"net/url"
	"os"
	"path"
	"sync"
	"time"
)

// Backend takes validated results and stores them as the task specifies, either locally, remotely, or both
func Backend(finalResultChan <-chan t.FinalMIDAResult, monitoringChan chan<- t.TaskStats,
	retryChan chan<- t.SanitizedMIDATask, storageWG *sync.WaitGroup, pipelineWG *sync.WaitGroup,
	connInfo *ConnInfo) {

	// Iterate over channel of rawResults until it is closed
	for r := range finalResultChan {
		r.Stats.Timing.BeginStorage = time.Now()
		r.Metadata.Timing.BeginStorage = time.Now()
		if !r.SanitizedTask.TaskFailed {
			// Store results here from a successfully completed task
			if r.SanitizedTask.OutputPath != "" {
				outputPathURL, err := url.Parse(r.SanitizedTask.OutputPath)
				if err != nil {
					log.Log.WithField("URL", r.SanitizedTask.Url).Error(err)
				} else {
					if outputPathURL.Host == "" {
						dirName, err := util.DirNameFromURL(r.SanitizedTask.Url)
						if err != nil {
							log.Log.WithField("URL", r.SanitizedTask.Url).Fatal(err)
						}
						outpath := path.Join(r.SanitizedTask.OutputPath, dirName, r.SanitizedTask.RandomIdentifier)
						err = storage.StoreResultsLocalFS(&r, outpath)
						if err != nil {
							log.Log.WithField("URL", r.SanitizedTask.Url).Error("Failed to store results: ", err)
						}
					} else {
						err := StoreOverSSH(&r, connInfo, outputPathURL)
						if err != nil {
							log.Log.WithField("URL", r.SanitizedTask.Url).Error(err)
						}
					}
				}
			}

			// Store data to Mongo, if you are in to that sort of thing
			// For now, we create a new connection on every single trace
			if r.SanitizedTask.MongoURI != "" {
				err := StoreToMongo(&r)
				if err != nil {
					log.Log.WithField("URL", r.SanitizedTask.Url).Error(err)
				}
			}

			if r.SanitizedTask.PostgresURI != "" {
				err := StoreToPostgres(&r)
				if err != nil {
					log.Log.WithField("URL", r.SanitizedTask.Url).Error(err)
				}
			}
		} else if r.SanitizedTask.CurrentAttempt >= r.SanitizedTask.MaxAttempts {
			// We are abandoning trying this task. Too bad.
			log.Log.WithField("URL", r.SanitizedTask.Url).Error("Task failed after ", r.SanitizedTask.MaxAttempts, " attempts.")
			log.Log.WithField("URL", r.SanitizedTask.Url).Errorf("Failure Code: [ %s ]", r.SanitizedTask.FailureCode)
			r.SanitizedTask.PastFailureCodes = append(r.SanitizedTask.PastFailureCodes, r.SanitizedTask.FailureCode)
		}

		// Remove all data from crawl
		// TODO: Add ability to save user data directory (without saving crawl data inside it)

		// There's an issue where os.RemoveAll throws an error while trying to delete the Chromium
		// User Data Directory sometimes. It's still unclear exactly why.
		err := os.RemoveAll(r.SanitizedTask.UserDataDirectory)
		if err != nil {
			log.Log.Error("Retrying in 1 sec...")
			time.Sleep(time.Second)
			err = os.RemoveAll(r.SanitizedTask.UserDataDirectory)
			if err != nil {
				log.Log.WithField("URL", r.SanitizedTask.Url).Error("Failure Deleting UDD on second try")
				log.Log.WithField("URL", r.SanitizedTask.Url).Fatal(err)
			} else {
				log.Log.WithField("URL", r.SanitizedTask.Url).Info("Deleted UDD on second try")
			}
		}

		// If this task failed but it still has tries left, retry it
		if r.SanitizedTask.TaskFailed && r.SanitizedTask.CurrentAttempt < r.SanitizedTask.MaxAttempts {
			// Squash task results and put the task back at the beginning of the pipeline
			r.SanitizedTask.CurrentAttempt++
			r.SanitizedTask.TaskFailed = false
			r.SanitizedTask.PastFailureCodes = append(r.SanitizedTask.PastFailureCodes, r.SanitizedTask.FailureCode)
			r.SanitizedTask.FailureCode = ""
			pipelineWG.Add(1)
			retryChan <- r.SanitizedTask
		} else if viper.GetBool("monitor") {
			r.Stats.Timing.EndStorage = time.Now()
			monitoringChan <- r.Stats
		}

		pipelineWG.Done()
	}

	storageWG.Done()
}

func StoreOverSSH(r *t.FinalMIDAResult, connInfo *ConnInfo, outputPathURL *url.URL) error {
	// Begin remote storage
	// Check if connection info exists already for host
	var activeConn *t.SSHConn
	connInfo.Lock()
	if _, ok := connInfo.SSHConnInfo[outputPathURL.Host]; !ok {
		newConn, err := storage.CreateRemoteConnection(outputPathURL.Host)
		connInfo.Unlock()
		backoff := 1
		for err != nil {
			log.Log.WithField("URL", r.SanitizedTask.Url).WithField("Backoff", backoff).Error(err)
			time.Sleep(time.Duration(backoff) * time.Second)
			connInfo.Lock()
			newConn, err = storage.CreateRemoteConnection(outputPathURL.Host)
			connInfo.Unlock()
			backoff *= DefaultSSHBackoffMultiplier
		}

		connInfo.SSHConnInfo[outputPathURL.Host] = newConn
		activeConn = newConn
		log.Log.WithField("host", outputPathURL.Host).Info("Created new SSH connection")
	} else {
		activeConn = connInfo.SSHConnInfo[outputPathURL.Host]
		connInfo.Unlock()
	}

	if activeConn == nil {
		log.Log.WithField("URL", r.SanitizedTask.Url).Error("Failed to correctly set activeConn")
		return errors.New("failed to correctly set activeConn")
	}

	// Now that our new connection is in place, proceed with storage
	activeConn.Lock()
	backOff := 1
	err := storage.StoreResultsSSH(r, activeConn, outputPathURL.Path)
	for err != nil {
		log.Log.WithField("URL", r.SanitizedTask.Url).WithField("BackOff", backOff).Error(err)
		time.Sleep(time.Duration(backOff) * time.Second)
		err = storage.StoreResultsSSH(r, activeConn, outputPathURL.Path)
		backOff *= DefaultSSHBackoffMultiplier
	}
	activeConn.Unlock()
	return nil
}

func StoreToMongo(r *t.FinalMIDAResult) error {
	mongoConn, err := storage.CreateMongoDBConnection(r.SanitizedTask.MongoURI, r.SanitizedTask.GroupID)
	if err != nil {
		return err
	}
	// Store metadata
	_, err = mongoConn.StoreMetadata(r)
	if err != nil {
		return err
	}

	// Store resource info to Mongo, if requested
	if r.SanitizedTask.ResourceMetadata {
		_, err := mongoConn.StoreResources(r)
		if err != nil {
			return err
		}
	}

	// Store websocket data to Mongo, if requested
	if r.SanitizedTask.WebsocketTraffic {
		_, err = mongoConn.StoreWebSocketData(r)
		if err != nil {
			return err
		}
	}

	// Close our connection to MongoDB nicely
	err = mongoConn.TeardownConnection()

	if err != nil {
		return err
	}

	return nil
}

func StoreToPostgres(r *t.FinalMIDAResult) error {
	// First, check and see if we have an existing connection for this database
	db, callNameMap, err := storage.CreatePostgresConnection(r.SanitizedTask.PostgresURI, "54330",
		r.SanitizedTask.PostgresDB)
	if err != nil {
		log.Log.Fatal(err)
	}

	// Now we store our js trace to postgres, if specified
	if r.SanitizedTask.JSTrace {
		err := storage.StoreJSTraceToDB(db,
			callNameMap, r)
		if err != nil {
			log.Log.WithField("URL", r.SanitizedTask.Url).Error(err)
		}
	}

	return db.Close()
}
