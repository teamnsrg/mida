package main

import (
	"flag"
	log "github.com/sirupsen/logrus"
)

// A configuration for running MIDA
type MIDAConfig struct {
	// Number of simultaneous browser instances
	NumWorkers int

	// Where should MIDA pull tasks from. Can be an AMPQ (RabbitMQ) address
	// or a file. If task parameters are given via command line, a file
	// (default: "mida_task.json") will be created containing the task.
	TaskInputSource string

	// Monitoring parameters
	EnableMonitoring bool
	PrometheusPort   int

	// Note that results configuration parameters are set on a per-task basis
}

func main() {

	taskSrcFlag := flag.String("tasksrc", "", "Source for task(s)")
	taskFileFlag := flag.String("taskfile", "", "File containing tasks to complete")
	monitorFlag := flag.Bool("monitor", false, "Run Prometheus monitoring")
	instanceNumFlag := flag.Int("instances", 1, "Number of concurrent instances to run")

	flag.Parse()

	go RunPrometheusClient()

	sampleTask, err := ReadTaskFromFile("examples/exampleTask.json")
	if err != nil {
		log.Fatal(err)
	}

	sanitizedSampleTask, err := SanitizeTask(sampleTask)
	if err != nil {
		log.Fatal(err)
	}
	ProcessSanitizedTask(sanitizedSampleTask)

	log.Info(*taskSrcFlag, *monitorFlag, *instanceNumFlag, *taskFileFlag)

	// Construct necessary channels

	// Start goroutine that runs the Prometheus monitoring HTTP server

	// Start goroutine that handles crawl results

	// Start crawler master goroutine (which spawns individual crawler(s)

	// Start goroutine which handles input tasks

	// Await message that all tasks have been completed

	return

}
