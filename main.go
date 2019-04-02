package main

import (
	"github.com/teamnsrg/mida/log"
	"net/http"
	_ "net/http/pprof"
)

// Sets up logging and hands off control to command.go, which is responsible
// for parsing args/flags and initiating the appropriate functionality
func main() {
	InitConfig()
	log.InitLogger()

	log.Log.Info("MIDA Starting")

	go func() {
		log.Log.Info(http.ListenAndServe("localhost:8080", nil))
	}()

	rootCmd := BuildCommands()
	err := rootCmd.Execute()
	if err != nil {
		log.Log.Debug(err)
	}

	log.Log.Info("MIDA exiting")
}
