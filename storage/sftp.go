package storage

import (
	"errors"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/sftp"
	"github.com/spf13/viper"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/sanitize"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type SftpParams struct {
	User           string
	PrivateKeyFile string
	Host           string
	Port           int
	Path           string
}

func Sftp(r *b.FinalResult) error {
	sftpSettings := r.Summary.TaskWrapper.SanitizedTask.OPS.SftpOut
	params := SftpParams{
		User:           *sftpSettings.UserName,
		PrivateKeyFile: *sftpSettings.PrivateKeyFile,
		Host:           *sftpSettings.Host,
		Port:           *sftpSettings.Port,
		Path:           *sftpSettings.Path,
	}

	// Begin remote storage
	// Check if connection info exists already for host
	var activeConn *SftpConn
	connInfo.Lock()
	if _, ok := connInfo.SSHConnInfo[fullSSHUri(params)]; !ok {
		newConn, err := createRemoteConnection(params)
		connInfo.Unlock()
		backoff := 1
		for err != nil {
			log.Log.WithField("URL", r.Summary.TaskWrapper.SanitizedTask.URL).WithField("Backoff", backoff).Error(err)
			time.Sleep(time.Duration(backoff) * time.Second)
			connInfo.Lock()
			newConn, err = createRemoteConnection(params)
			connInfo.Unlock()
			backoff *= b.DefaultSSHBackoffMultiplier
		}

		connInfo.SSHConnInfo[fullSSHUri(params)] = newConn
		activeConn = newConn
		log.Log.WithField("host", fullSSHUri(params)).Info("Created new SSH connection")
	} else {
		activeConn = connInfo.SSHConnInfo[fullSSHUri(params)]
		connInfo.Unlock()
	}

	if activeConn == nil {
		log.Log.WithField("URL", r.Summary.TaskWrapper.SanitizedTask.URL).Error("Failed to correctly set activeConn")
		return errors.New("failed to correctly set activeConn")
	}

	// Now that our new connection is in place, proceed with storage
	activeConn.Lock()
	backOff := 1
	err := StoreResultsSSH(r, activeConn, params.Path)
	for err != nil {
		log.Log.WithField("URL", r.Summary.TaskWrapper.SanitizedTask.URL).WithField("BackOff", backOff).Error(err)
		time.Sleep(time.Duration(backOff) * time.Second)
		err = StoreResultsSSH(r, activeConn, params.Path)
		backOff *= b.DefaultSSHBackoffMultiplier
	}
	activeConn.Unlock()
	return nil
}

func StoreResultsSSH(r *b.FinalResult, activeConn *SftpConn, remotePath string) error {
	// We store all the results to the local file system first in a temporary directory
	// Ideally, we reuse what has already been stored locally, but we store it ourselves if required
	tw := r.Summary.TaskWrapper
	tempPath := path.Join(viper.GetString("tempdir"), tw.UUID.String()+"-sftpresults")
	err := Local(r, tw.SanitizedTask.OPS.SftpOut.DS, tempPath)
	if err != nil {
		return err
	}

	sftpClient, err := sftp.NewClient(activeConn.Client)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	// Walk the temporary results directory and write everything to our new remote file location
	dirName, err := DirNameFromURL(tw.SanitizedTask.URL)
	if err != nil {
		log.Log.Fatal(err)
	}
	err = copyDirRemote(sftpClient, tempPath, path.Join(remotePath,
		dirName, tw.UUID.String()))
	if err != nil {
		err = os.RemoveAll(tempPath)
		if err != nil {
			log.Log.Error(err)
		}
		return err
	}

	err = os.RemoveAll(tempPath)
	if err != nil {
		log.Log.Error(err)
	}

	return nil
}

func copyDirRemote(sftpConn *sftp.Client, localDirname string, remoteDirname string) error {
	err := sftpConn.MkdirAll(remoteDirname)
	if err != nil {
		return err
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
				return err
			}

		} else {
			srcFile, err := os.Open(p)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			dstFile, err := sftpConn.Create(path.Join(remoteDirname, localizedPath))
			if err != nil {
				return err
			}
			defer dstFile.Close()

			_, err = io.Copy(dstFile, srcFile)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

func createRemoteConnection(params SftpParams) (*SftpConn, error) {
	// First, get our private key
	var c SftpConn

	privateKeyFile := sanitize.ExpandPath(params.PrivateKeyFile)
	if params.PrivateKeyFile == "" {
		h, err := homedir.Dir()
		if err != nil {
			return nil, errors.New("no SSH private key file provided and could not determine default")
		}
		privateKeyFile = sanitize.ExpandPath(h + "/.ssh/id_rsa")
	}

	privateKeyBytes, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		return nil, err
	}

	privateKey, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return nil, err
	}

	// Get username for use in ssh
	username := params.User
	if username == "" {
		u, err := user.Current()
		if err != nil {
			return nil, err
		}
		username = u.Username
	}

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(privateKey),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	c.Client, err = ssh.Dial("tcp", params.Host+":"+strconv.Itoa(params.Port), config)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

func fullSSHUri(params SftpParams) string {
	return params.User + "@" + params.Host + ":" + strconv.Itoa(params.Port)
}
