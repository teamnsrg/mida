package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/teamnsrg/mida/amqp"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
)

// getRootCommand returns the root cobra command which will be executed based on arguments passed to the program
func getRootCommand() *cobra.Command {
	var err error
	var cmdRoot = &cobra.Command{Use: "mida"}

	var (
		numCrawlers    int
		numStorers     int
		monitor        bool
		promPort       int
		logLevel       int
		virtualDisplay bool
	)

	cmdRoot.PersistentFlags().IntVarP(&numCrawlers, "crawlers", "c", viper.GetInt("crawlers"),
		"Number of parallel browser instances to use for crawling")
	cmdRoot.PersistentFlags().IntVarP(&numStorers, "storers", "s", viper.GetInt("storers"),
		"Number of parallel goroutines working to store task results")
	cmdRoot.PersistentFlags().BoolVarP(&monitor, "monitor", "m", false,
		"Enable monitoring via Prometheus by hosting a HTTP server")
	cmdRoot.PersistentFlags().IntVarP(&promPort, "prom-port", "z", viper.GetInt("prom_port"),
		"Port used for hosting metrics for a Prometheus server")
	cmdRoot.PersistentFlags().IntVarP(&logLevel, "log-level", "l", viper.GetInt("log_level"),
		"Log Level for MIDA (0=Error, 1=Warn, 2=Info, 3=Debug)")
	cmdRoot.PersistentFlags().BoolVarP(&virtualDisplay, "xvfb", "", false,
		"Use Xvfb virtual display (for non-headless, monitor-less crawls on Linux)")

	err = viper.BindPFlags(cmdRoot.PersistentFlags())
	if err != nil {
		log.Log.Fatal("viper failed to bind pflags")
	}

	cmdRoot.AddCommand(getBuildCommand())
	cmdRoot.AddCommand(getClientCommand())
	cmdRoot.AddCommand(getFileCommand())
	cmdRoot.AddCommand(getLoadCommand())
	cmdRoot.AddCommand(getGoCommand())

	return cmdRoot
}

// getFileCommand returns the command for the "mida file" version of the program
func getFileCommand() *cobra.Command {
	var cmdFile = &cobra.Command{
		Use:   "file",
		Short: "Read and execute tasks from file",
		Long: `MIDA reads and executes tasks from a pre-created task
file, exiting when all tasks in the file are completed.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ll, err := cmd.Flags().GetInt("log-level")
			if err != nil {
				log.Log.Fatal(err)
			}
			err = log.ConfigureLogging(ll)
			if err != nil {
				log.Log.Fatal(err)
			}
			log.Log.Debug("MIDA Starts (Mode: file)")

			InitPipeline(cmd, args)
		},
	}

	var (
		shuffle bool
	)

	cmdFile.Flags().BoolVarP(&shuffle, "shuffle", "", b.DefaultShuffle,
		"Randomize processing order for tasks")

	return cmdFile
}

func getLoadCommand() *cobra.Command {
	var cmdLoad = &cobra.Command{
		Use:   "load",
		Short: "Load tasks from file into queue",
		Long:  `Read tasks from a JSON-formatted file, parse them, and load them into the specified queue instance`,
		Args:  cobra.ExactArgs(1), // the filename containing tasks to read
		Run: func(cmd *cobra.Command, args []string) {
			ll, err := cmd.Flags().GetInt("log-level")
			if err != nil {
				log.Log.Fatal(err)
			}
			err = log.ConfigureLogging(ll)
			if err != nil {
				log.Log.Fatal(err)
			}
			log.Log.Debug("MIDA Starts (Mode: load)")

			tasks, err := b.ReadTasksFromFile(args[0])
			if err != nil {
				log.Log.Fatal(err)
			}

			var params = amqp.ConnParams{
				User: viper.GetString("amqp_user"),
				Pass: viper.GetString("amqp_pass"),
				Uri:  viper.GetString("amqp_uri"),
			}

			queue, err := cmd.Flags().GetString("queue")
			if err != nil {
				log.Log.Fatal(err)
			}

			priority, err := cmd.Flags().GetUint8("priority")
			if err != nil {
				log.Log.Fatal(err)
			}

			shuffle, err := cmd.Flags().GetBool("shuffle")
			if err != nil {
				log.Log.Fatal(err)
			}

			numTasksLoaded, err := amqp.LoadTasks(tasks, params, queue,
				priority, shuffle)
			if err != nil {
				log.Log.Fatal(err)
			}

			log.Log.Infof("Loaded %d tasks into queue \"%s\" with priority %d",
				numTasksLoaded, queue, priority)
		},
	}

	var (
		shuffle  bool
		queue    string
		priority uint8
	)

	cmdLoad.Flags().StringVarP(&queue, "queue", "", amqp.DefaultTaskQueue,
		"AMQP queue into which we will load tasks")
	cmdLoad.Flags().BoolVarP(&shuffle, "shuffle", "", b.DefaultShuffle,
		"Randomize loading order for tasks")
	cmdLoad.Flags().Uint8VarP(&priority, "priority", "", amqp.DefaultPriority,
		"Priority of tasks we are loaded (AMQP: x-max-priority setting)")

	return cmdLoad
}

func getBuildCommand() *cobra.Command {
	// Variables storing options for the build command
	var (
		urlFile  string
		priority int

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
		allResources     bool

		// Output settings
		resultsOutputPath string // Results from task path

		outputPath string // Task file path
		overwrite  bool

		// How many times a task should be repeated
		repeat int
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
			log.Log.Debug("MIDA Starts (Mode: build)")

			cts, err := BuildCompressedTaskSet(cmd, args)
			if err != nil {
				log.Log.Error(err)
				return
			}
			outfile, err := cmd.Flags().GetString("outfile")
			if err != nil {
				log.Log.Error(err)
				return
			}

			overwrite, err := cmd.Flags().GetBool("overwrite")
			if err != nil {
				log.Log.Error(err)
				return
			}

			err = b.WriteCompressedTaskSetToFile(cts, outfile, overwrite)
			if err != nil {
				log.Log.Error(err)
			} else {
				log.Log.Infof("Wrote %d tasks to %s", len(*cts.URL), outfile)
			}
		},
	}

	cmdBuild.Flags().StringVarP(&urlFile, "url-file", "f",
		"", "File containing URL to visit (1 per line)")
	cmdBuild.Flags().IntVarP(&priority, "priority", "", b.DefaultTaskPriority,
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

	cmdBuild.Flags().StringVarP(&completionCondition, "completion", "y", string(b.DefaultCompletionCondition),
		"Completion condition for tasks (CompleteOnTimeoutOnly, CompleteOnLoadEvent, CompleteOnTimeoutAfterLoad")
	cmdBuild.Flags().IntVarP(&timeout, "timeout", "t", b.DefaultTimeout,
		"Timeout (in seconds) after which the browser will close and the task will complete")
	cmdBuild.Flags().IntVarP(&timeAfterLoad, "time-after-load", "", b.DefaultTimeAfterLoad,
		"Time after load event to remain on page (overridden by timeout if reached first)")

	cmdBuild.Flags().BoolVarP(&allResources, "all-resources", "", b.DefaultAllResources,
		"Gather and store all resources downloaded by browser")
	cmdBuild.Flags().BoolVarP(&resourceMetadata, "resource-metadata", "", b.DefaultResourceMetadata,
		"Gather and store metadata about all resources downloaded by browser")

	cmdBuild.Flags().StringVarP(&resultsOutputPath, "results-output-path", "r", b.DefaultLocalOutputPath,
		"Path (local or remote) to store results in. A new directory will be created inside this one for each task.")

	cmdBuild.Flags().StringVarP(&outputPath, "outfile", "o", viper.GetString("task_file"),
		"Path to write the newly-created JSON task file")
	cmdBuild.Flags().BoolVarP(&overwrite, "overwrite", "x", false,
		"Allow overwriting of an existing task file")

	cmdBuild.Flags().IntVarP(&repeat, "repeat", "", 1,
		"How many times to repeat a given task")

	_ = cmdBuild.MarkFlagRequired("url-file")
	_ = cmdBuild.MarkFlagFilename("url-file")

	return cmdBuild
}

func getGoCommand() *cobra.Command {
	var (
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
		allResources     bool

		// Output settings
		resultsOutputPath string // Results from task path

		outputPath string // Task file path
		overwrite  bool

		// How many times a task should be repeated
		repeat int
	)

	var cmdGo = &cobra.Command{
		Use:   "go",
		Short: "Crawl from the command line",
		Long:  `Start a crawl right here and now, normally specifying urls on the command line`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ll, err := cmd.Flags().GetInt("log-level")
			if err != nil {
				log.Log.Fatal(err)
			}
			err = log.ConfigureLogging(ll)
			if err != nil {
				log.Log.Fatal(err)
			}
			log.Log.Debug("MIDA Starts (Mode: go)")

			InitPipeline(cmd, args)
		},
	}

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

	cmdGo.Flags().StringVarP(&completionCondition, "completion", "y", string(b.DefaultCompletionCondition),
		"Completion condition for tasks (CompleteOnTimeoutOnly, CompleteOnLoadEvent, CompleteOnTimeoutAfterLoad")
	cmdGo.Flags().IntVarP(&timeout, "timeout", "t", b.DefaultTimeout,
		"Timeout (in seconds) after which the browser will close and the task will complete")
	cmdGo.Flags().IntVarP(&timeAfterLoad, "time-after-load", "", b.DefaultTimeAfterLoad,
		"Time after load event to remain on page (overridden by timeout if reached first)")

	cmdGo.Flags().BoolVarP(&allResources, "all-resources", "", b.DefaultAllResources,
		"Gather and store all resources downloaded by browser")
	cmdGo.Flags().BoolVarP(&resourceMetadata, "resource-metadata", "", b.DefaultResourceMetadata,
		"Gather and store metadata about all resources downloaded by browser")

	cmdGo.Flags().StringVarP(&resultsOutputPath, "results-output-path", "r", b.DefaultLocalOutputPath,
		"Path (local or remote) to store results in. A new directory will be created inside this one for each task.")

	cmdGo.Flags().StringVarP(&outputPath, "outfile", "o", viper.GetString("task_file"),
		"Path to write the newly-created JSON task file")
	cmdGo.Flags().BoolVarP(&overwrite, "overwrite", "x", false,
		"Allow overwriting of an existing task file")

	cmdGo.Flags().IntVarP(&repeat, "repeat", "", 1,
		"How many times to repeat a given task")

	return cmdGo
}

func getClientCommand() *cobra.Command {
	var cmdClient = &cobra.Command{
		Use:   "client",
		Short: "Act as AMQP Client for tasks",
		Long: `MIDA acts as a client to a AMQP server.
An address and credentials must be provided. MIDA will remain running until
it receives explicit instructions to close, or the connection to AMQP server is lost.`,
		Run: func(cmd *cobra.Command, args []string) {
			ll, err := cmd.Flags().GetInt("log-level")
			if err != nil {
				log.Log.Fatal(err)
			}
			err = log.ConfigureLogging(ll)
			if err != nil {
				log.Log.Fatal(err)
			}
			log.Log.Debug("MIDA Starts (Mode: client)")

			user, err := cmd.Flags().GetString("user")
			if err != nil {
				log.Log.Fatal(err)
			}

			if user != "" {
				viper.Set("amqp_user", user)
			}

			pass, err := cmd.Flags().GetString("pass")
			if err != nil {
				log.Log.Fatal(err)
			}
			if pass != "" {
				viper.Set("amqp_pass", pass)
			}

			uri, err := cmd.Flags().GetString("uri")
			if err != nil {
				log.Log.Fatal(err)
			}
			if uri != "" {
				viper.Set("amqp_uri", uri)
			}

			InitPipeline(cmd, args)
		},
	}

	var (
		queue string
		user  string
		pass  string
		uri   string
	)

	cmdClient.Flags().StringVarP(&queue, "queue", "", "",
		"AMQP queue into which we will load tasks")
	cmdClient.Flags().StringVarP(&user, "user", "", "",
		"AMQP User")
	cmdClient.Flags().StringVarP(&pass, "pass", "", "",
		"AMQP Password")
	cmdClient.Flags().StringVarP(&uri, "uri", "", "",
		"AMQP URI")

	return cmdClient
}
