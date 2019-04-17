package main

import (
	"github.com/teamnsrg/mida/log"
	_ "net/http/pprof"
)

// Sets up logging and hands off control to command.go, which is responsible
// for parsing args/flags and initiating the appropriate functionality
func main() {
	InitConfig()
	log.InitLogger()

	rootCmd := BuildCommands()

	err := rootCmd.Execute()
	if err != nil {
		log.Log.Debug(err)
	}

	log.Log.Info("MIDA exiting")
}
