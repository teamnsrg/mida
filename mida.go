package main

import (
	"flag"
	log "github.com/Sirupsen/logrus"
)

func main() {

	taskSrcFlag := flag.String("tasksrc", "", "Source for task(s)")
	taskFileFlag := flag.String("taskfile", "", "File containing tasks to complete")
	monitorFlag := flag.Bool("monitor", false, "Run Prometheus monitoring")
	instanceNumFlag := flag.Int("instances", 1, "Number of concurrent instances to run")

	flag.Parse()

	go RunPrometheusClient()

	sampleTask := InitTask()

	sampleTask.URL = "murley.io"

	ProcessSanitizedTask(sampleTask)

	log.Info(*taskSrcFlag, *monitorFlag, *instanceNumFlag, *taskFileFlag)

	// Parse config file (if it exists)

	// Construct necessary channels

	// Start goroutine that runs the Prometheus monitoring HTTP server

	// Start goroutine that handles crawl results

	// Start crawler master goroutine (which spawns individual crawler(s)

	// Start goroutine which handles input tasks

	// Await message that all tasks have been completed

	return

}
