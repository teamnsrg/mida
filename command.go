package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func BuildCommands() *cobra.Command {

	var cmdBuild = &cobra.Command{
		Use:   "build",
		Short: "Build a MIDA Task File",
		Long:  `Create and save a task file using flags or CLI`,
		Args:  cobra.OnlyValidArgs,
		Run: func(cmd *cobra.Command, args []string) {
			_, err := BuildCompressedTaskSet(cmd, args)
			if err != nil {
				Log.Error(err)
			}
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

	cmdBuild.Flags().StringVarP(&completionCondition, "completion", "y", "CompleteOnTimeoutOnly",
		"Completion condition for tasks (CompleteOnTimeoutOnly, CompleteOnLoadEvent, CompleteOnTimeoutAfterLoad")
	cmdBuild.Flags().IntVarP(&timeout, "timeout", "t", DefaultTimeout,
		"Timeout (in seconds) after which the browser will close and the task will complete")

	cmdBuild.Flags().StringVarP(&resultsOutputPath, "results-output-path", "r", DefaultOutputPath,
		"Path (local or remote) to store results in. A new directory will be created inside this one for each task.")

	cmdBuild.Flags().StringVarP(&outputPath, "outfile", "o", viper.GetString("taskfile"),
		"Path to write the newly-created JSON task file")
	cmdBuild.Flags().BoolVarP(&overwrite, "overwrite", "x", false,
		"Allow overwriting of an existing task file")
	cmdBuild.Flags().StringVarP(&groupID, "group", "n", DefaultGroupID,
		"Group ID used for identifying experiments")

	_ = cmdBuild.MarkFlagRequired("urlfile")
	_ = cmdBuild.MarkFlagFilename("urlfile")

	var cmdGo = &cobra.Command{
		Use:   "go",
		Short: "Start a crawl here and now, using flags to set params",
		Long: `MIDA flags and URL(s) from the input command and immediately begins
to crawl, using default parameters where not specified`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			InitPipeline(cmd, args)
		},
	}

	cmdGo.Flags().StringVarP(&urlfile, "urlfile", "f",
		"", "File containing URL to visit (1 per line)")
	cmdGo.Flags().IntVarP(&maxAttempts, "attempts", "a", DefaultTimeout,
		"Maximum attempts for a task before it fails")

	cmdGo.Flags().StringVarP(&browser, "browser", "b",
		"", "Path to browser binary to use for this task")
	cmdGo.Flags().StringVarP(&userDataDir, "user-data-dir", "d",
		"", "User Data Directory used for this task.")
	cmdGo.Flags().StringSliceP("add-browser-flags", "", addBrowserFlags,
		"Flags to add to browser launch (comma-separated, no '--')")
	cmdGo.Flags().StringSliceP("remove-browser-flags", "", removeBrowserFlags,
		"Flags to remove from browser launch (comma-separated, no '--')")
	cmdGo.Flags().StringSliceP("set-browser-flags", "", setBrowserFlags,
		"Overrides default browser flags (comma-separated, no '--')")
	cmdGo.Flags().StringSliceP("extensions", "e", extensions,
		"Full paths to browser extensions to use (comma-separated, no'--')")

	cmdGo.Flags().StringVarP(&completionCondition, "completion", "y", "CompleteOnTimeoutOnly",
		"Completion condition for tasks (CompleteOnTimeoutOnly, CompleteOnLoadEvent, CompleteOnTimeoutAfterLoad")
	cmdGo.Flags().IntVarP(&timeout, "timeout", "t", DefaultTimeout,
		"Timeout (in seconds) after which the browser will close and the task will complete")

	cmdGo.Flags().StringVarP(&resultsOutputPath, "results-output-path", "r", DefaultOutputPath,
		"Path (local or remote) to store results in. A new directory will be created inside this one for each task.")

	cmdGo.Flags().StringVarP(&groupID, "group", "n", DefaultGroupID,
		"Group ID used for identifying experiments")

	// TODO: Look into combining 'go' and 'build' flags somehow - maybe just unify under root

	var cmdFile = &cobra.Command{
		Use:   "file",
		Short: "Read and execute tasks from file",
		Long: `MIDA reads and executes tasks from a pre-created task
file, exiting when all tasks in the file are completed.`,
		Run: func(cmd *cobra.Command, args []string) {
			InitPipeline(cmd, args)
		},
	}

	var taskfile string

	cmdFile.Flags().StringVarP(&taskfile, "taskfile", "f", viper.GetString("taskfile"),
		"Task file to process")
	err := viper.BindPFlag("taskfile", cmdFile.Flags().Lookup("taskfile"))
	if err != nil {
		Log.Fatal(err)
	}

	_ = cmdFile.MarkFlagFilename("taskfile")

	var cmdLoad = &cobra.Command{
		Use:   "load",
		Short: "Load/Enqueue tasks into an AMQP instance",
		Long:  `Read tasks from a file and enqueue these tasks using AMQP`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			Log.Info("Running enqueue.")
		},
	}

	var cmdClient = &cobra.Command{
		Use:   "client",
		Short: "Act as AMQP Client for tasks",
		Long: `MIDA acts as a client to a AMQP server.
An address and credentials must be provided. MIDA will remain running until
it receives explicit instructions to close, or the connection to the queue is
lost.`,
		Run: func(cmd *cobra.Command, args []string) {
			InitPipeline(cmd, args)
		},
	}

	var cmdRoot = &cobra.Command{Use: "mida"}

	var (
		numCrawlers int
		numStorers  int
		monitor     bool
		promPort    int
	)

	cmdRoot.PersistentFlags().IntVarP(&numCrawlers, "crawlers", "c", viper.GetInt("crawlers"),
		"Number of parallel browser instances to use for crawling")
	cmdRoot.PersistentFlags().IntVarP(&numStorers, "storers", "s", viper.GetInt("storers"),
		"Number of parallel goroutines working to store task results")
	cmdRoot.PersistentFlags().BoolVarP(&monitor, "monitor", "m", false,
		"Enable monitoring via Prometheus by hosting a HTTP server")
	cmdRoot.PersistentFlags().IntVarP(&promPort, "prom-port", "p", viper.GetInt("prom-port"),
		"Port used for hosting metrics for a Prometheus server")

	err = viper.BindPFlags(cmdRoot.PersistentFlags())
	if err != nil {
		Log.Fatal(err)
	}

	cmdRoot.AddCommand(cmdLoad)
	cmdRoot.AddCommand(cmdClient)
	cmdRoot.AddCommand(cmdFile)
	cmdRoot.AddCommand(cmdBuild)
	cmdRoot.AddCommand(cmdGo)

	return cmdRoot
}
