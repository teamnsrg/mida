package main

import (
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetReportCaller(true)

	rootCmd := BuildCommands()
	err := rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}
