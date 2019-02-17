package main

import (
	"bufio"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

func BuildTask(cmd *cobra.Command) {

	// Get URLs from URL file
	var urls []string
	fname, err := cmd.Flags().GetString("urlfile")
	if err != nil {
		log.Fatal(err)
	}

	urlfile, err := os.Open(fname)
	if err != nil {
		log.Fatal(err)
	}
	defer urlfile.Close()

	scanner := bufio.NewScanner(urlfile)
	for scanner.Scan() {
		// TODO: Validate URLs here
		urls = append(urls, scanner.Text())
	}

}
