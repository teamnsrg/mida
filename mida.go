package main

import (
	"flag"
	log "github.com/sirupsen/logrus"
)

func main() {

	taskSrcFlag := flag.String("tasksrc", "", "Source for task(s)")
	taskFileFlag := flag.String("taskfile", "", "File containing tasks to complete")
	monitorFlag := flag.Bool("monitor", false, "Run Prometheus monitoring")
	instanceNumFlag := flag.Int("instances", 1, "Number of concurrent instances to run")

	flag.Parse()

	go RunPrometheusClient()

	sampleTask, err := ReadTaskFromFile("/home/pmurley/go/src/github.com/teamnsrg/mida/examples/exampleTask.json")
	if err != nil {
		log.Fatal(err)
	}

	sanitizedSampleTask, err := SanitizeTask(sampleTask)
	if err != nil {
		log.Fatal(err)
	}
	ProcessSanitizedTask(sanitizedSampleTask)

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
