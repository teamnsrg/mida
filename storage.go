package main

import (
	"encoding/json"
	"errors"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/sftp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Holds information about an SSH session to another host,
// used for storing results
type SSHConn struct {
	sync.Mutex
	Client *ssh.Client
}

// Takes validated results and stores them as the task specifies, either locally, remotely, or both
func StoreResults(finalResultChan <-chan FinalMIDAResult, monitoringChan chan<- TaskStats,
	retryChan chan<- SanitizedMIDATask, storageWG *sync.WaitGroup, pipelineWG *sync.WaitGroup,
	connInfo *ConnInfo) {
	for r := range finalResultChan {

		r.Stats.Timing.BeginStorage = time.Now()

		if !r.SanitizedTask.TaskFailed {
			// Store results here from a successfully completed task
			outputPathURL, err := url.Parse(r.SanitizedTask.OutputPath)
			if err != nil {
				Log.Error(err)
			} else {
				if outputPathURL.Host == "" {
					dirName, err := DirNameFromURL(r.SanitizedTask.Url)
					if err != nil {
						Log.Fatal(err)
					}
					outpath := path.Join(r.SanitizedTask.OutputPath, dirName,
						r.SanitizedTask.RandomIdentifier)
					err = StoreResultsLocalFS(r, outpath)
					if err != nil {
						log.Error("Failed to store results: ", err)
					}
				} else {
					// Begin remote storage
					// Check if connection info exists already for host
					var activeConn *SSHConn
					connInfo.Lock()
					if _, ok := connInfo.SSHConnInfo[outputPathURL.Host]; !ok {
						newConn, err := CreateRemoteConnection(outputPathURL.Host)
						connInfo.Unlock()
						backoff := 1
						for err != nil {
							Log.WithField("Backoff", backoff).Error(err)
							time.Sleep(time.Duration(backoff) * time.Second)
							connInfo.Lock()
							newConn, err = CreateRemoteConnection(outputPathURL.Host)
							connInfo.Unlock()
							backoff *= DefaultSSHBackoffMultiplier
						}

						connInfo.SSHConnInfo[outputPathURL.Host] = newConn
						activeConn = newConn
						Log.WithField("host", outputPathURL.Host).Info("Created new SSH connection")
					} else {
						activeConn = connInfo.SSHConnInfo[outputPathURL.Host]
						connInfo.Unlock()
					}

					if activeConn == nil {
						Log.Error("Failed to correctly set activeConn")
					}

					// Now that our new connection is in place, proceed with storage
					activeConn.Lock()
					exponentialBackoff := 1
					err = StoreResultsSSH(r, activeConn, outputPathURL.Path)
					for err != nil {
						Log.WithField("Backoff", exponentialBackoff).Error(err)
						time.Sleep(time.Duration(exponentialBackoff) * time.Second)
						err = StoreResultsSSH(r, activeConn, outputPathURL.Path)
						exponentialBackoff *= DefaultSSHBackoffMultiplier
					}
					activeConn.Unlock()
				}
			}
		}

		// Remove all data from crawl
		// TODO: Add ability to save user data directory (without saving crawl data inside it)
		// TODO: Also fix this nonsense
		// There's an issue where os.RemoveAll throws an error while trying to delete the Chromium
		// User Data Directory sometimes. It's still unclear exactly why.
		err := os.RemoveAll(r.SanitizedTask.UserDataDirectory)
		if err != nil {
			Log.WithFields(log.Fields{
				"URL": r.SanitizedTask.Url,
			}).Errorf("Issue deleting User Data Dir: %s", r.SanitizedTask.UserDataDirectory)
			Log.Error("Retrying in 1 sec...")
			time.Sleep(time.Second)
			err = os.RemoveAll(r.SanitizedTask.UserDataDirectory)
			if err != nil {
				Log.Error("Failure")
				Log.Fatal(err)
			} else {
				Log.Error("Success!")
			}
		}

		if r.SanitizedTask.TaskFailed {
			if r.SanitizedTask.CurrentAttempt >= r.SanitizedTask.MaxAttempts {
				// We are abandoning trying this task. Too bad.
				Log.WithField("URL", r.SanitizedTask.Url).Error("Task failed after ", r.SanitizedTask.MaxAttempts, " attempts.")
				Log.WithField("URL", r.SanitizedTask.Url).Errorf("Failure Code: [ %s ]", r.SanitizedTask.FailureCode)
			} else {
				// "Squash" task results and put the task back at the beginning of the pipeline
				Log.WithFields(log.Fields{
					"URL": r.SanitizedTask.Url,
				}).Debug("Retrying task...")
				r.SanitizedTask.CurrentAttempt++
				r.SanitizedTask.TaskFailed = false
				r.SanitizedTask.PastFailureCodes = append(r.SanitizedTask.PastFailureCodes, r.SanitizedTask.FailureCode)
				r.SanitizedTask.FailureCode = ""
				pipelineWG.Add(1)
				retryChan <- r.SanitizedTask
				Log.WithFields(log.Fields{
					"URL": r.SanitizedTask.Url,
				}).Debug("Added to retry channel")
			}
		}

		r.Stats.Timing.EndStorage = time.Now()

		// Send stats to Prometheus
		if viper.GetBool("monitoring") {
			r.Stats.Timing.EndStorage = time.Now()
			monitoringChan <- r.Stats
		}

		pipelineWG.Done()
	}

	storageWG.Done()
}

// Stores a result directory (via SSH/SFTP) to a remote host, given
// an already active SSH connection. Ensure that you lock the relevant SSH connection
// Before calling this.
func StoreResultsSSH(r FinalMIDAResult, activeConn *SSHConn, remotePath string) error {
	// We store all the results to the local file system first in a temporary directory
	tempPath := path.Join(TempDir, r.SanitizedTask.RandomIdentifier+"-results")
	err := StoreResultsLocalFS(r, tempPath)
	if err != nil {
		return err
	}

	sftpClient, err := sftp.NewClient(activeConn.Client)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	// Walk the temporary results directory and write everything to our new remote file location
	dirName, err := DirNameFromURL(r.SanitizedTask.Url)
	if err != nil {
		Log.Fatal(err)
	}
	err = CopyDirRemote(sftpClient, tempPath, path.Join(remotePath,
		dirName, r.SanitizedTask.RandomIdentifier))
	if err != nil {
		err = os.RemoveAll(tempPath)
		if err != nil {
			Log.Error(err)
		}
		return err
	}

	err = os.RemoveAll(tempPath)
	if err != nil {
		Log.Error(err)
	}

	return nil
}

func CopyDirRemote(sftpConn *sftp.Client, localDirname string, remoteDirname string) error {
	err := sftpConn.MkdirAll(remoteDirname)
	if err != nil {
		Log.Error(err)
	}
	err = filepath.Walk(localDirname, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if p == localDirname {
			return nil
		}
		localizedPath := strings.TrimPrefix(p, localDirname+"/")

		if info.IsDir() {
			err = sftpConn.MkdirAll(path.Join(remoteDirname, info.Name()))
			if err != nil {
				Log.Debug(err)
				return err
			}

		} else {
			srcFile, err := os.Open(p)
			if err != nil {
				Log.Debug(err)
				return err
			}
			defer srcFile.Close()

			dstFile, err := sftpConn.Create(path.Join(remoteDirname, localizedPath))
			if err != nil {
				Log.Debug(err)
				return err
			}
			defer dstFile.Close()

			_, err = io.Copy(dstFile, srcFile)
			if err != nil {
				Log.Debug(err)
				return err
			}
		}

		return nil
	})

	return err
}

// Given a valid FinalMIDAResult, stores it according to the output
// path specified in the sanitized task within the result
func StoreResultsLocalFS(r FinalMIDAResult, outpath string) error {
	_, err := os.Stat(outpath)
	if err != nil {
		err = os.MkdirAll(outpath, 0755)
		if err != nil {
			Log.Error("Failed to create local output directory")
			return errors.New("failed to create local output directory")
		}
	} else {
		Log.Error("Output directory for task already exists")
		return errors.New("output directory for task already exists")
	}

	// Store resource metadata from crawl (DevTools requestWillBeSent and responseReceived data)
	if r.SanitizedTask.ResourceMetadata {
		data, err := json.Marshal(r.ResourceMetadata)
		if err != nil {
			Log.Error(err)
		}
		err = ioutil.WriteFile(path.Join(outpath, DefaultResourceMetadataFile), data, 0644)
		if err != nil {
			Log.Error(err)
		}

	}

	// Store raw resources downloaded during crawl (named for their request IDs)
	if r.SanitizedTask.AllResources {
		_, err = os.Stat(path.Join(r.SanitizedTask.UserDataDirectory, r.SanitizedTask.RandomIdentifier, DefaultFileSubdir))
		if err != nil {
			Log.Error("AllResources requested but no files directory exists within temporary results directory")
			Log.Error("Files will not be stored")
			return errors.New("files temporary directory does not exist")
		} else {
			err = os.Rename(path.Join(r.SanitizedTask.UserDataDirectory, r.SanitizedTask.RandomIdentifier, DefaultFileSubdir),
				path.Join(outpath, DefaultFileSubdir))
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	if r.SanitizedTask.ScriptMetadata {
		data, err := json.Marshal(r.ScriptMetadata)
		if err != nil {
			Log.Error(err)
		}
		err = ioutil.WriteFile(path.Join(outpath, DefaultScriptMetadataFile), data, 0644)
		if err != nil {
			Log.Error(err)
		}
	}

	// Store raw scripts parsed by the browser during crawl (named by hashes)
	if r.SanitizedTask.AllScripts {
		_, err = os.Stat(path.Join(r.SanitizedTask.UserDataDirectory, r.SanitizedTask.RandomIdentifier, DefaultScriptSubdir))
		if err != nil {
			Log.Error("AllScripts requested but no files directory exists within temporary results directory")
			Log.Error("Scripts will not be stored")
			return errors.New("scripts temporary directory does not exist")
		} else {
			err = os.Rename(path.Join(r.SanitizedTask.UserDataDirectory, r.SanitizedTask.RandomIdentifier, DefaultScriptSubdir),
				path.Join(outpath, DefaultScriptSubdir))
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	if r.SanitizedTask.JSTrace {
		data, err := json.Marshal(r.JSTrace)
		if err != nil {
			Log.Error(err)
		}
		err = ioutil.WriteFile(path.Join(outpath, DefaultJSTracePath), data, 0644)
		if err != nil {
			Log.Error(err)
		}

		if r.SanitizedTask.SaveRawTrace {
			err = os.Rename(path.Join(r.SanitizedTask.UserDataDirectory, r.SanitizedTask.RandomIdentifier, DefaultBrowserLogFileName),
				path.Join(outpath, DefaultBrowserLogFileName))
		}
	}

	// Store Websocket data (if specified)
	if r.SanitizedTask.WebsocketTraffic {
		data, err := json.Marshal(r.WebsocketData)
		if err != nil {
			Log.Error(err)
		}
		err = ioutil.WriteFile(path.Join(outpath, DefaultWebSocketTrafficFile), data, 0644)
		if err != nil {
			Log.Error(err)
		}
	}

	return nil
}

func CreateRemoteConnection(host string) (*SSHConn, error) {
	// First, get our private key
	var c SSHConn

	h, err := homedir.Dir()
	if err != nil {
		return &c, err
	}
	privateKeyBytes, err := ioutil.ReadFile(h + "/.ssh/id_rsa") // TODO
	if err != nil {
		return &c, err
	}

	privateKey, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return &c, err
	}

	// Get current username for use in ssh
	u, err := user.Current()
	if err != nil {
		return &c, err
	}

	config := &ssh.ClientConfig{
		User: u.Username, // TODO
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(privateKey),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	c.Client, err = ssh.Dial("tcp", host+":22", config)
	if err != nil {
		return &c, err
	}

	return &c, nil
}
