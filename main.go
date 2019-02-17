package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func main() {
	var cmdEnqueue = &cobra.Command{
		Use:   "load",
		Short: "Load tasks into RabbitMQ",
		Long: `Read tasks from a file or build them from the command line.
Given an address and credentials, enqueue these tasks using AMPQ`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("Running enqueuer.")
		},
	}

	var cmdClient = &cobra.Command{
		Use:   "client",
		Short: "Act as AMPQ Client for tasks",
		Long: `MIDA acts as a client to a RabbitMQ/AMPQ server.
An address and credentials must be provided. MIDA will remain running until
it receives explicit instructions to close, or the connection to the queue is
lost.`,
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("Running client")
			InitPipeline()
		},
	}

	var cmdFile = &cobra.Command{
		Use:   "file",
		Short: "Read and execute tasks from file",
		Long: `MIDA reads and executes tasks from a pre-created task
file, exiting when all tasks in the file are completed.`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("Running file")
			InitPipeline()
		},
	}

	var cmdBuild = &cobra.Command{
		Use:   "build",
		Short: "Build a MIDA Task File",
		Long:  `Create and save a task file using flags or CLI`,
		Args:  cobra.OnlyValidArgs,
		Run: func(cmd *cobra.Command, args []string) {
			BuildTask(cmd)
		},
	}

	var rootCmd = &cobra.Command{Use: "mida"}

	// General settings
	var urlfile string
	var maxAttempts int

	// Browser settings
	var binary string
	var userDataDir string
	var addBrowserFlags []string
	var removeBrowserFlags []string
	var setBrowserFlags []string
	var extensions []string

	// Completion settings
	var completionCondition string
	var timeout int

	// Output settings
	var resultsOutputPath string

	var outputPath string
	var groupID string

	cmdBuild.Flags().StringVarP(&urlfile, "urlfile", "f",
		"", "File containing URLs to visit (1 per line)")
	cmdBuild.Flags().IntVarP(&maxAttempts, "attempts", "a", DefaultTimeout,
		"Maximum attempts for a task before it fails")

	cmdBuild.Flags().StringVarP(&binary, "binary", "b",
		"", "Path to browser binary to use for this task")
	cmdBuild.Flags().StringVarP(&userDataDir, "user-data-dir", "u",
		"", "User Data Directory used for this task.")
	cmdBuild.Flags().StringSliceP("add-browser-flags", "p", addBrowserFlags,
		"Flags to add to browser launch (comma-separated, no'--')")
	cmdBuild.Flags().StringSliceP("remove-browser-flags", "r", removeBrowserFlags,
		"Flags to remove from browser launch (comma-separated, no'--')")
	cmdBuild.Flags().StringSliceP("set-browser-flags", "s", setBrowserFlags,
		"Overrides default browser flags (comma-separated, no'--')")
	cmdBuild.Flags().StringSliceP("extensions", "e", extensions,
		"Full paths to browser extensions to use (comma-separated, no'--')")

	cmdBuild.Flags().StringVarP(&completionCondition, "completion", "c", "CompleteOnTimeoutOnly",
		"Completion condition for tasks (CompleteOnTimeoutOnly, CompleteOnLoadEvent, CompleteOnTimeoutAfterLoad")
	cmdBuild.Flags().IntVarP(&timeout, "timeout", "t", DefaultTimeout,
		"Timeout (in seconds) after which the browser will close and the task will complete")

	cmdBuild.Flags().StringVarP(&resultsOutputPath, "local-output-path", "l", DefaultLocalOutputPath,
		"Local path to use for storing task results")

	cmdBuild.Flags().StringVarP(&outputPath, "outfile", "o", DefaultTaskLocation,
		"Path to write the newly-created JSON task file")
	cmdBuild.Flags().StringVarP(&groupID, "group", "n", DefaultGroupID,
		"Group ID used for identifying experiments")

	_ = cmdBuild.MarkFlagRequired("urlfile")
	_ = cmdBuild.MarkFlagFilename("urlfile")

	rootCmd.AddCommand(cmdEnqueue)
	rootCmd.AddCommand(cmdClient)
	rootCmd.AddCommand(cmdFile)
	rootCmd.AddCommand(cmdBuild)
	err := rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}

}
