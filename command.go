package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func BuildCommands() *cobra.Command {

	var cmdBuild = &cobra.Command{
		Use:   "build",
		Short: "Build a MIDA Task File",
		Long:  `Create and save a task file using flags or CLI`,
		Args:  cobra.OnlyValidArgs,
		Run: func(cmd *cobra.Command, args []string) {
			BuildTask(cmd)
		},
	}

	// Variables storing options for the build command
	var (
		urlfile     string
		maxAttempts int

		// Browser settings
		browser            string
		userDataDir        string
		addBrowserFlags    []string
		removeBrowserFlags []string
		setBrowserFlags    []string
		extensions         []string

		// Completion settings
		completionCondition string
		timeout             int

		// Output settings
		resultsOutputPath string // Results from task path
		groupID           string

		outputPath string // Task file path
		overwrite  bool
	)

	cmdBuild.Flags().StringVarP(&urlfile, "urlfile", "f",
		"", "File containing URL to visit (1 per line)")
	cmdBuild.Flags().IntVarP(&maxAttempts, "attempts", "a", DefaultTimeout,
		"Maximum attempts for a task before it fails")

	cmdBuild.Flags().StringVarP(&browser, "browser", "b",
		"", "Path to browser binary to use for this task")
	cmdBuild.Flags().StringVarP(&userDataDir, "user-data-dir", "d",
		"", "User Data Directory used for this task.")
	cmdBuild.Flags().StringSliceP("add-browser-flags", "", addBrowserFlags,
		"Flags to add to browser launch (comma-separated, no '--')")
	cmdBuild.Flags().StringSliceP("remove-browser-flags", "", removeBrowserFlags,
		"Flags to remove from browser launch (comma-separated, no '--')")
	cmdBuild.Flags().StringSliceP("set-browser-flags", "", setBrowserFlags,
		"Overrides default browser flags (comma-separated, no '--')")
	cmdBuild.Flags().StringSliceP("extensions", "e", extensions,
		"Full paths to browser extensions to use (comma-separated, no'--')")

	cmdBuild.Flags().StringVarP(&completionCondition, "completion", "c", "CompleteOnTimeoutOnly",
		"Completion condition for tasks (CompleteOnTimeoutOnly, CompleteOnLoadEvent, CompleteOnTimeoutAfterLoad")
	cmdBuild.Flags().IntVarP(&timeout, "timeout", "t", DefaultTimeout,
		"Timeout (in seconds) after which the browser will close and the task will complete")

	cmdBuild.Flags().StringVarP(&resultsOutputPath, "results-output-path", "p", DefaultLocalOutputPath,
		"Path (local or remote) to store results in. A new directory will be created inside this one for each task.")

	cmdBuild.Flags().StringVarP(&outputPath, "outfile", "o", DefaultTaskLocation,
		"Path to write the newly-created JSON task file")
	cmdBuild.Flags().BoolVarP(&overwrite, "overwrite", "x", false,
		"Allow overwriting of an existing task file")
	cmdBuild.Flags().StringVarP(&groupID, "group", "n", DefaultGroupID,
		"Group ID used for identifying experiments")

	_ = cmdBuild.MarkFlagRequired("urlfile")
	_ = cmdBuild.MarkFlagFilename("urlfile")

	var cmdFile = &cobra.Command{
		Use:   "file",
		Short: "Read and execute tasks from file",
		Long: `MIDA reads and executes tasks from a pre-created task
file, exiting when all tasks in the file are completed.`,
		Run: func(cmd *cobra.Command, args []string) {
			InitPipeline(cmd)
		},
	}

	var (
		taskfile    string
		monitor     bool
		promPort    int
		numCrawlers int
		numStorers  int
	)

	cmdFile.Flags().StringVarP(&taskfile, "taskfile", "f", DefaultTaskLocation,
		"Task file to process")
	cmdFile.Flags().BoolVarP(&monitor, "monitor", "m", false,
		"Enable monitoring via Prometheus by hosting a HTTP server")
	cmdFile.Flags().IntVarP(&promPort, "prom-port", "p", DefaultPrometheusPort,
		"Port used for hosting metrics for a Prometheus server")
	cmdFile.Flags().IntVarP(&numCrawlers, "crawlers", "c", DefaultNumCrawlers,
		"Number of parallel browser instances to use for crawling")
	cmdFile.Flags().IntVarP(&numStorers, "storers", "s", DefaultNumStorers,
		"Number of parallel goroutines working to store task results")

	_ = cmdBuild.MarkFlagFilename("taskfile")

	var cmdEnqueue = &cobra.Command{
		Use:   "load",
		Short: "Load tasks into RabbitMQ",
		Long:  `Read tasks from a file and enqueue these tasks using AMPQ`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("Running enqueue.")
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
			InitPipeline(cmd)
		},
	}

	var cmdRoot = &cobra.Command{Use: "mida"}

	cmdRoot.AddCommand(cmdEnqueue)
	cmdRoot.AddCommand(cmdClient)
	cmdRoot.AddCommand(cmdFile)
	cmdRoot.AddCommand(cmdBuild)

	return cmdRoot
}
