package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/queue"
	"github.com/teamnsrg/mida/storage"
)

func buildCommands() *cobra.Command {

	// Variables storing options for the build command
	var (
		// Root Command Flags
		numCrawlers int
		numStorers  int
		monitor     bool
		promPort    int
		logLevel    int

		urlFile     string
		maxAttempts int
		priority    int

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
		timeAfterLoad       int

		// Data Gathering settings
		resourceMetadata bool
		scriptMetadata   bool
		jsTrace          bool
		saveRawTrace     bool
		allResources     bool
		allScripts       bool
		resourceTree     bool
		webSocket        bool
		networkTrace     bool
		openWPMChecks    bool
		browserCoverage	 bool

		// Output settings
		resultsOutputPath string // Results from task path
		groupID           string

		outputPath string // Task file path
		overwrite  bool
	)

	var cmdBuild = &cobra.Command{
		Use:   "build",
		Short: "Build a MIDA Task File",
		Long:  `Create and save a task file using flags or CLI`,
		Args:  cobra.OnlyValidArgs,
		Run: func(cmd *cobra.Command, args []string) {
			ll, err := cmd.Flags().GetInt("log-level")
			if err != nil {
				log.Log.Fatal(err)
			}
			err = log.ConfigureLogging(ll)
			if err != nil {
				log.Log.Fatal(err)
			}
			_, err = BuildCompressedTaskSet(cmd, args)
			if err != nil {
				log.Log.Error(err)
			}
		},
	}

	cmdBuild.Flags().StringVarP(&urlFile, "urlfile", "f",
		"", "File containing URL to visit (1 per line)")
	cmdBuild.Flags().IntVarP(&maxAttempts, "attempts", "a", DefaultTimeout,
		"Maximum attempts for a task before it fails")
	cmdBuild.Flags().IntVarP(&priority, "priority", "", DefaultTaskPriority,
		"Task priority (when loaded into RabbitMQ")

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
	cmdBuild.Flags().IntVarP(&timeAfterLoad, "time-after-load", "", DefaultTimeAfterLoad,
		"Time after load event to remain on page (overridden by timeout if reached first)")

	cmdBuild.Flags().BoolVarP(&allResources, "all-resources", "", DefaultAllResources,
		"Gather and store all resources downloaded by browser")
	cmdBuild.Flags().BoolVarP(&allScripts, "all-scripts", "", DefaultAllScripts,
		"Gather and store source code for all scripts parsed by the browser")
	cmdBuild.Flags().BoolVarP(&jsTrace, "js-trace", "", DefaultJSTrace,
		"Gather and store a trace of JavaScript API calls (requires instrumented browser)")
	cmdBuild.Flags().BoolVarP(&saveRawTrace, "save-raw-trace", "", DefaultSaveRawTrace,
		"Save the raw JavaScript trace (as output by the browser)")
	cmdBuild.Flags().BoolVarP(&resourceMetadata, "resource-metadata", "", DefaultResourceMetadata,
		"Gather and store metadata about all resources downloaded by browser")
	cmdBuild.Flags().BoolVarP(&scriptMetadata, "script-metadata", "", DefaultResourceMetadata,
		"Gather and store metadata about all scripts parsed by browser")
	cmdBuild.Flags().BoolVarP(&resourceTree, "resource-tree", "", DefaultResourceTree,
		"Construct and store a best-effort dependency tree for resources encountered during crawl")
	cmdBuild.Flags().BoolVarP(&webSocket, "websocket", "", DefaultWebsocketTraffic,
		"Gather and store data and metadata on websocket messages")
	cmdBuild.Flags().BoolVarP(&networkTrace, "network-strace", "", DefaultNetworkStrace,
		"Gather a raw trace of all networking system calls made by the browser")
	cmdBuild.Flags().BoolVarP(&openWPMChecks, "openwpm-checks", "", DefaultOpenWPMChecks,
		"Run OpenWPM fingerprinting checks on JavaScript trace")
	cmdBuild.Flags().BoolVarP(&browserCoverage, "browser-coverage", "", DefaultBrowserCoverage,
		"Gather browser coverage data (requires browser instrumented for coverage)")

	cmdBuild.Flags().StringVarP(&resultsOutputPath, "results-output-path", "r", storage.DefaultOutputPath,
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
			ll, err := cmd.Flags().GetInt("log-level")
			if err != nil {
				log.Log.Fatal(err)
			}
			err = log.ConfigureLogging(ll)
			if err != nil {
				log.Log.Fatal(err)
			}
			InitPipeline(cmd, args)
		},
	}

	cmdGo.Flags().StringVarP(&urlFile, "urlfile", "f",
		"", "File containing URL to visit (1 per line)")
	cmdGo.Flags().IntVarP(&maxAttempts, "attempts", "a", DefaultTaskAttempts,
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
	cmdGo.Flags().IntVarP(&timeAfterLoad, "time-after-load", "", DefaultTimeAfterLoad,
		"Time after load event to remain on page (overridden by timeout if reached first)")

	cmdGo.Flags().BoolVarP(&allResources, "all-resources", "", DefaultAllResources,
		"Gather and store all resources downloaded by browser")
	cmdGo.Flags().BoolVarP(&allScripts, "all-scripts", "", DefaultAllScripts,
		"Gather and store source code for all scripts parsed by the browser")
	cmdGo.Flags().BoolVarP(&jsTrace, "js-trace", "", DefaultJSTrace,
		"Gather and store a trace of JavaScript API calls (requires instrumented browser)")
	cmdGo.Flags().BoolVarP(&saveRawTrace, "save-raw-trace", "", DefaultSaveRawTrace,
		"Save the raw JavaScript trace (as output by the browser)")
	cmdGo.Flags().BoolVarP(&resourceMetadata, "resource-metadata", "", DefaultResourceMetadata,
		"Gather and store metadata about all resources downloaded by browser")
	cmdGo.Flags().BoolVarP(&resourceMetadata, "script-metadata", "", DefaultResourceMetadata,
		"Gather and store metadata about all scripts parsed by browser")
	cmdGo.Flags().BoolVarP(&resourceTree, "resource-tree", "", DefaultResourceTree,
		"Construct and store a best-effort dependency tree for resources encountered during crawl")
	cmdGo.Flags().BoolVarP(&webSocket, "websocket", "", DefaultWebsocketTraffic,
		"Gather and store data and metadata on websocket messages")
	cmdGo.Flags().BoolVarP(&networkTrace, "network-strace", "", DefaultNetworkStrace,
		"Gather a raw trace of all networking system calls made by the browser")
	cmdGo.Flags().BoolVarP(&openWPMChecks, "openwpm-checks", "", DefaultOpenWPMChecks,
		"Run OpenWPM fingerprinting checks on JavaScript trace")
	cmdGo.Flags().BoolVarP(&browserCoverage, "browser-coverage", "", DefaultBrowserCoverage,
		"Gather browser coverage data (requires browser instrumented for coverage)")
	cmdGo.Flags().IntVarP(&priority, "priority", "", DefaultTaskPriority,
		"Task priority (when loaded into RabbitMQ")

	cmdGo.Flags().StringVarP(&resultsOutputPath, "results-output-path", "r", storage.DefaultOutputPath,
		"Path (local or remote) to store results in. A new directory will be created inside this one for each task.")

	cmdGo.Flags().StringVarP(&groupID, "group", "n", DefaultGroupID,
		"Group ID used for identifying experiments")

	var cmdFile = &cobra.Command{
		Use:   "file",
		Short: "Read and execute tasks from file",
		Long: `MIDA reads and executes tasks from a pre-created task
file, exiting when all tasks in the file are completed.`,
		Run: func(cmd *cobra.Command, args []string) {
			ll, err := cmd.Flags().GetInt("log-level")
			if err != nil {
				log.Log.Fatal(err)
			}
			err = log.ConfigureLogging(ll)
			if err != nil {
				log.Log.Fatal(err)
			}
			InitPipeline(cmd, args)
		},
	}

	var taskfile string

	cmdFile.Flags().StringVarP(&taskfile, "taskfile", "f", viper.GetString("taskfile"),
		"Task file to process")
	err := viper.BindPFlag("taskfile", cmdFile.Flags().Lookup("taskfile"))
	if err != nil {
		log.Log.Fatal(err)
	}

	_ = cmdFile.MarkFlagFilename("taskfile")

	var cmdLoad = &cobra.Command{
		Use:   "load",
		Short: "Load/Enqueue tasks into an AMQP instance",
		Long:  `Read tasks from a file and enqueue these tasks using AMQP`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ll, err := cmd.Flags().GetInt("log-level")
			if err != nil {
				log.Log.Fatal(err)
			}
			err = log.ConfigureLogging(ll)
			if err != nil {
				log.Log.Fatal(err)
			}
			tasks, err := ReadTasksFromFile(args[0])
			if err != nil {
				log.Log.Fatal(err)
			}
			numTasksLoaded, err := queue.AMQPLoadTasks(tasks)
			if err != nil {
				log.Log.Fatal(err)
			}
			log.Log.Infof("Loaded %d tasks into queue.", numTasksLoaded)
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
			ll, err := cmd.Flags().GetInt("log-level")
			if err != nil {
				log.Log.Fatal(err)
			}
			err = log.ConfigureLogging(ll)
			if err != nil {
				log.Log.Fatal(err)
			}
			InitPipeline(cmd, args)
		},
	}

	var cmdRoot = &cobra.Command{Use: "mida"}

	cmdRoot.PersistentFlags().IntVarP(&numCrawlers, "crawlers", "c", viper.GetInt("crawlers"),
		"Number of parallel browser instances to use for crawling")
	cmdRoot.PersistentFlags().IntVarP(&numStorers, "storers", "s", viper.GetInt("storers"),
		"Number of parallel goroutines working to store task results")
	cmdRoot.PersistentFlags().BoolVarP(&monitor, "monitor", "m", false,
		"Enable monitoring via Prometheus by hosting a HTTP server")
	cmdRoot.PersistentFlags().IntVarP(&promPort, "prom-port", "p", viper.GetInt("prom-port"),
		"Port used for hosting metrics for a Prometheus server")
	cmdRoot.PersistentFlags().IntVarP(&logLevel, "log-level", "l", viper.GetInt("log-level"),
		"Log Level for MIDA (0=Error, 1=Warn, 2=Info, 3=Debug)")

	err = viper.BindPFlags(cmdRoot.PersistentFlags())
	if err != nil {
		log.Log.Fatal(err)
	}

	cmdRoot.AddCommand(cmdLoad)
	cmdRoot.AddCommand(cmdClient)
	cmdRoot.AddCommand(cmdFile)
	cmdRoot.AddCommand(cmdBuild)
	cmdRoot.AddCommand(cmdGo)

	return cmdRoot
}
