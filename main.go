package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"
)

// Sets up logging and hands off control to command.go, which is responsible
// for parsing args/flags and initiating the appropriate functionality
func main() {
	InitConfig()
	InitLogger()

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	Log.Info("MIDA Starting")

	rootCmd := BuildCommands()
	err := rootCmd.Execute()
	if err != nil {
		Log.Debug(err)
	}

	Log.Info("MIDA exiting")
}
