package main

import (
	"fmt"
	"github.com/spf13/viper"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/browser"
	"github.com/teamnsrg/mida/sanitize"
	"os"
	"path"
	"testing"
)

func testCleanup() {
	err := os.RemoveAll(viper.GetString("tempdir"))
	if err != nil {
		fmt.Println(err)
	}
}

func TestDismissJSDialog(t *testing.T) {
	t.Parallel()
	t.Cleanup(testCleanup)

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	testSiteDir := "file://" + path.Join(wd, "test")
	url := path.Join(testSiteDir, "jsdialog.html")

	cc := b.LoadEvent
	browserFlags := []string{"headless", "disable-gpu"}

	rt := b.RawTask{
		URL: &url,
		Browser: &b.BrowserSettings{
			AddBrowserFlags: &browserFlags,
		},
		Completion: &b.CompletionSettings{
			CompletionCondition: &cc,
		},
	}

	tw, err := sanitize.Task(&rt)
	if err != nil {
		t.Fatal(err)
	}

	rawResult, err := browser.VisitPageDevtoolsProtocol(&tw)
	if err != nil {
		t.Fatal(err)
	}

	success := false
	for _, v := range rawResult.DevTools.Network.RequestWillBeSent {
		for _, req := range v {
			if req.Request.URL == "http://www.example.org/example.txt" {
				success = true
			}
		}
	}

	if !success {
		t.Fatal("failed to pass through javascript dialogs")
	}
}
