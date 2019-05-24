package main

import (
	"fmt"
	"github.com/teamnsrg/mida/log"
	"net/http"
	_ "net/http/pprof"
)

// Sets up logging and hands off control to command.go, which is responsible
// for parsing args/flags and initiating the appropriate functionality
func main() {
	initConfig()
	log.InitLogger()

	go func() {
		fmt.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	rootCmd := buildCommands()

	err := rootCmd.Execute()
	if err != nil {
		log.Log.Debug(err)
	}
}
