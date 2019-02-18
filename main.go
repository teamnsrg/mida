package main

// Sets up logging and hands off control to command.go, which is responsible
// for parsing args/flags and initiating the appropriate functionality
func main() {
	InitLogger()

	Log.Info("MIDA starting")

	rootCmd := BuildCommands()
	err := rootCmd.Execute()
	if err != nil {
		Log.Error(err)
	}

	Log.Info("MIDA exiting")
}
