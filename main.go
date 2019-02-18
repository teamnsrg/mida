package main

import (
	log "github.com/sirupsen/logrus"
)

func main() {
	rootCmd := BuildCommands()
	err := rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}
