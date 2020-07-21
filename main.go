package main

import (
	"github.com/teamnsrg/mida/log"
)

func main() {
	initViperConfig()
	log.InitGlobalLogger("mida.log")

	rootCmd := getRootCommand()
	err := rootCmd.Execute()
	if err != nil {
		log.Log.Error(err)
	}

	log.Log.Debug("MIDA Exits")
	return
}
