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
		Run: func(cmd *cobra.Command, args []string) {
			log.Info("Building Task File")
		},
	}

	var rootCmd = &cobra.Command{Use: "mida"}
	rootCmd.AddCommand(cmdEnqueue)
	rootCmd.AddCommand(cmdClient)
	rootCmd.AddCommand(cmdFile)
	rootCmd.AddCommand(cmdBuild)
	err := rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}

}
