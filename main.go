package main

import (
	"github.com/teamnsrg/mida/log"
	_ "net/http/pprof"
)

// Sets up logging and hands off control to command.go, which is responsible
// for parsing args/flags and initiating the appropriate functionality
func main() {
	initConfig()
	log.InitLogger()

	rootCmd := buildCommands()

	err := rootCmd.Execute()
	if err != nil {
		log.Log.Debug(err)
	}
}
