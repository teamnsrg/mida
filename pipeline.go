package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/monitor"
	"github.com/teamnsrg/mida/storage"
	t "github.com/teamnsrg/mida/types"
	"os"
	"sync"
)

type ConnInfo struct {
	sync.Mutex
	SSHConnInfo map[string]*t.SSHConn
}

func InitPipeline(cmd *cobra.Command, args []string) {

	// Create channels for the pipeline
	monitoringChan := make(chan t.TaskStats)
	finalResultChan := make(chan t.FinalMIDAResult)
	rawResultChan := make(chan t.RawMIDAResult)
	sanitizedTaskChan := make(chan t.SanitizedMIDATask)
	rawTaskChan := make(chan t.MIDATask)

	// Important to size this channel correctly to avoid deadlock when retries are very common
	retryChan := make(chan t.SanitizedMIDATask, viper.GetInt("crawlers")+viper.GetInt("storers")+2)

	var crawlerWG sync.WaitGroup  // Tracks active crawler workers
	var storageWG sync.WaitGroup  // Tracks active storage workers
	var pipelineWG sync.WaitGroup // Tracks tasks currently in pipeline

	// Initialize directory for SSH connections, which are effectively global
	var connInfo ConnInfo
	connInfo.SSHConnInfo = make(map[string]*t.SSHConn)

	// Start goroutine that runs the Prometheus monitoring HTTP server
	if viper.GetBool("monitor") {
		go monitor.RunPrometheusClient(monitoringChan, viper.GetInt("promport"))
	}

	// Start goroutine(s) that handles crawl results storage
	storageWG.Add(viper.GetInt("storers"))
	for i := 0; i < viper.GetInt("storers"); i++ {
		go Backend(finalResultChan, monitoringChan, retryChan, &storageWG, &pipelineWG, &connInfo)
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
			log.Log.Error(err)
		}
		log.Log.Info("Closed SSH connection to: ", k)
		v.Unlock()
	}
	connInfo.Unlock()

	// Cleanup remaining artifacts
	err := os.RemoveAll(storage.TempDir)
	if err != nil {
		log.Log.Warn(err)
	}

	return
}
