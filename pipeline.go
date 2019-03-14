package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"sync"
)

type ConnInfo struct {
	sync.Mutex
	SSHConnInfo map[string]*SSHConn
}

func InitPipeline(cmd *cobra.Command, args []string) {

	// Create channels for the pipeline
	monitoringChan := make(chan TaskStats)
	finalResultChan := make(chan FinalMIDAResult)
	rawResultChan := make(chan RawMIDAResult)
	sanitizedTaskChan := make(chan SanitizedMIDATask)
	rawTaskChan := make(chan MIDATask)
	retryChan := make(chan SanitizedMIDATask, viper.GetInt("crawlers")+viper.GetInt("storers")+2)

	var crawlerWG sync.WaitGroup  // Tracks active crawler workers
	var storageWG sync.WaitGroup  // Tracks active storage workers
	var pipelineWG sync.WaitGroup // Tracks tasks currently in pipeline

	// Initialize directory for SSH connections, which are effectively global
	var connInfo ConnInfo
	connInfo.SSHConnInfo = make(map[string]*SSHConn)

	// Start goroutine that runs the Prometheus monitoring HTTP server
	if viper.GetBool("monitor") {
		go RunPrometheusClient(monitoringChan, viper.GetInt("promport"))
	}

	// Start goroutine(s) that handles crawl results storage
	storageWG.Add(viper.GetInt("storers"))
	for i := 0; i < viper.GetInt("storers"); i++ {
		go StoreResults(finalResultChan, monitoringChan, retryChan, &storageWG, &pipelineWG, &connInfo)
	}

	// Start goroutine that handles crawl results sanitization
	go PostprocessResult(rawResultChan, finalResultChan)

	// Start crawler(s) which take sanitized tasks as arguments
	crawlerWG.Add(viper.GetInt("crawlers"))
	for i := 0; i < viper.GetInt("crawlers"); i++ {
		go CrawlerInstance(sanitizedTaskChan, rawResultChan, retryChan, &crawlerWG)
	}

	// Start goroutine which sanitizes input tasks
	go SanitizeTasks(rawTaskChan, sanitizedTaskChan, &pipelineWG)

	go TaskIntake(rawTaskChan, cmd, args)

	// Once all crawlers have completed, we can close the Raw Result Channel
	crawlerWG.Wait()
	close(rawResultChan)

	// We are done when all storage has completed
	storageWG.Wait()

	// Nicely close any SSH connections open
	connInfo.Lock()
	for k, v := range connInfo.SSHConnInfo {
		v.Lock()
		err := v.Client.Close()
		if err != nil {
			Log.Error(err)
		}
		Log.Info("Closed SSH connection to: ", k)
		v.Unlock()
	}
	connInfo.Unlock()

	// Cleanup remaining artifacts
	err := os.RemoveAll(TempDir)
	if err != nil {
		Log.Warn(err)
	}

	return

}
