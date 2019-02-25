package main

// Sets up logging and hands off control to command.go, which is responsible
// for parsing args/flags and initiating the appropriate functionality
func main() {
	InitConfig()
	InitLogger()

	Log.Info("MIDA Starting")

	rootCmd := BuildCommands()
	err := rootCmd.Execute()
	if err != nil {
		Log.Debug(err)
	}

	Log.Info("MIDA exiting")
}
