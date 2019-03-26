package storage

import (
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/sftp"
	"github.com/teamnsrg/mida/log"
	t "github.com/teamnsrg/mida/types"
	"github.com/teamnsrg/mida/util"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
)

// Stores a result directory (via SSH/SFTP) to a remote host, given
// an already active SSH connection. Ensure that you lock the relevant SSH connection
// Before calling this.
func StoreResultsSSH(r *t.FinalMIDAResult, activeConn *t.SSHConn, remotePath string) error {
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
	dirName, err := util.DirNameFromURL(r.SanitizedTask.Url)
	if err != nil {
		log.Log.Fatal(err)
	}
	err = CopyDirRemote(sftpClient, tempPath, path.Join(remotePath,
		dirName, r.SanitizedTask.RandomIdentifier))
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

func CreateRemoteConnection(host string) (*t.SSHConn, error) {
	// First, get our private key
	var c t.SSHConn

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

func CopyDirRemote(sftpConn *sftp.Client, localDirname string, remoteDirname string) error {
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
