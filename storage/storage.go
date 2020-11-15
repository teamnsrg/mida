package storage

import (
	"errors"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"golang.org/x/crypto/ssh"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

type SftpConn struct {
	sync.Mutex
	Client *ssh.Client
}

type ConnInfo struct {
	sync.Mutex
	SSHConnInfo map[string]*SftpConn
}

func newConnInfo() *ConnInfo {
	c := new(ConnInfo)
	c.SSHConnInfo = make(map[string]*SftpConn)
	return c
}

// Holds info about connections to remote storage entities
var connInfo = newConnInfo()

func StoreAll(finalResult *b.FinalResult) error {
	// For brevity
	st := finalResult.Summary.TaskWrapper.SanitizedTask

	if *st.OPS.LocalOut.Enable {
		// Build our output path
		dirName, err := DirNameFromURL(st.URL)
		if err != nil {
			return errors.New("failed to extract directory name from URL: " + err.Error())
		}
		outPath := path.Join(*st.OPS.LocalOut.Path, dirName, finalResult.Summary.TaskWrapper.UUID.String())

		err = Local(finalResult, st.OPS.LocalOut.DS, outPath)
		if err != nil {
			return err
		}
	}

	if *st.OPS.SftpOut.Enable {
		err := Sftp(finalResult)
		if err != nil {
			log.Log.Error(err)
		}
	}

	return nil
}

// CleanupConnections should be called when MIDA exits to ensure that any existing connections
// to remote hosts or databases are closed gracefully
func CleanupConnections() error {
	// Nicely close any connections open
	connInfo.Lock()
	for k, v := range connInfo.SSHConnInfo {
		v.Lock()
		err := v.Client.Close()
		if err != nil {
			log.Log.Error(err)
		}
		log.Log.WithField("host", k).Info("Closed SSH connection")
		v.Unlock()
	}

	connInfo.Unlock()
	return nil
}

// CleanupTask handles deleting temporary files created as tasks are processed. It should only
// be called once results have been totally stored.
func CleanupTask(fr *b.FinalResult) error {

	tw := fr.Summary.TaskWrapper

	// Chrome sometimes won't allow the user data directory to be deleted on the first try,
	// so we loop until we can successfully remove it
	_ = os.RemoveAll(tw.SanitizedTask.UserDataDirectory)
	for {
		if _, err := os.Stat(tw.SanitizedTask.UserDataDirectory); err == nil {
			time.Sleep(1 * time.Second)
			err = os.RemoveAll(tw.SanitizedTask.UserDataDirectory)
			if err != nil {
				log.Log.Error("failed to remove user data directory")
			}
		} else {
			break
		}
	}

	// Remove our temporary results directory
	err := os.RemoveAll(tw.TempDir)
	if err != nil {
		return err
	}

	return nil
}

// DirNameFromURL takes a URL and sanitizes/escapes it so it can safely be used as a filename
func DirNameFromURL(s string) (string, error) {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return "", err
	}

	// Replace all disallowed file path characters (both Windows and Unix) so we can safely use URL as directory name
	disallowedChars := []string{"/", "\\", ">", "<", ":", "|", "?", "*"}
	result := u.Host + u.EscapedPath()
	for _, c := range disallowedChars {
		result = strings.Replace(result, c, "-", -1)
	}
	return result, nil
}
