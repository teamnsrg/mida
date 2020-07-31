package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/monitor"
	"github.com/teamnsrg/mida/sanitize"
	"github.com/teamnsrg/mida/storage"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
)

// InitPipeline is the main MIDA pipeline, used whenever MIDA uses a browser to visit websites.
// It consists of five main stages: RawTask stage1, RawTask Sanitize, Site Visit, stage4, and Results Storage.
func InitPipeline(cmd *cobra.Command, args []string) {
	rawTaskChan := make(chan *b.RawTask)           // channel connecting stages 1 and 2
	sanitizedTaskChan := make(chan *b.TaskWrapper) // channel connecting stages 2 and 3
	rawResultChan := make(chan *b.RawResult)       // channel connecting stages 3 and 4
	finalResultChan := make(chan *b.FinalResult)   // channel connection stages 4 and 5
	monitorChan := make(chan *b.TaskSummary)

	var crawlerWG sync.WaitGroup  // Tracks active crawler workers
	var storageWG sync.WaitGroup  // Tracks active storage workers
	var pipelineWG sync.WaitGroup // Tracks tasks currently in pipeline

	// Start our virtual display, if needed
	xvfb, err := cmd.Flags().GetBool("xvfb")
	if err != nil {
		log.Log.Error(err)
		return
	}
	xvfbCommand := exec.Command("Xvfb", ":99", "-screen", "0", "1920x1080x16")
	if xvfb {
		if runtime.GOOS != "linux" {
			log.Log.Error("virtual display (Xvfb) is only available on linux")
			return
		}

		// Required so we can catch SIGTERM/SIGINT gracefully without closing Xvfb immediately
		xvfbCommand.SysProcAttr = &syscall.SysProcAttr{
			Setpgid:                    true,
		}

		err := xvfbCommand.Start()
		if err != nil {
			log.Log.Error(err)
			return
		}

		err = os.Setenv("DISPLAY", ":99")
	}

	// Start goroutine that runs the Prometheus monitoring HTTP server
	if viper.GetBool("monitor") {
		go monitor.RunPrometheusClient(monitorChan, viper.GetInt("prom_port"))
	}

	// Start goroutine(s) that handles crawl results storage
	numStorers := viper.GetInt("storers")
	storageWG.Add(numStorers)
	for i := 0; i < viper.GetInt("storers"); i++ {
		go stage5(finalResultChan, monitorChan, &storageWG, &pipelineWG)
	}

	// Start goroutine that handles crawl results sanitization
	go stage4(rawResultChan, finalResultChan)

	// Start site visitors(s) which take sanitized tasks as arguments
	numCrawlers := viper.GetInt("crawlers")
	crawlerWG.Add(numCrawlers)
	for i := 0; i < numCrawlers; i++ {
		go stage3(sanitizedTaskChan, rawResultChan, &crawlerWG)
	}

	// Start goroutine which sanitizes input tasks
	go stage2(rawTaskChan, sanitizedTaskChan, &pipelineWG)

	// Start the goroutine responsible for getting our tasks
	go stage1(rawTaskChan, cmd, args)

	// Wait for all of our crawlers to finish, and then allow them to exit
	crawlerWG.Wait()
	close(rawResultChan)

	// Wait for all of our storers to exit. We do not need to close the channel
	// going to storers -- the channel close will ripple through the pipeline
	storageWG.Wait()

	// Close connections to databases or storage servers
	err = storage.CleanupConnections()
	if err != nil {
		log.Log.Error(err)
	}

	// Cleanup any remaining temporary files before we exit
	err = os.RemoveAll(sanitize.ExpandPath(b.DefaultTempDir))
	if err != nil {
		log.Log.Error(err)
	}

	// Shut down our xvfb server, if running
	if xvfb {
		err = xvfbCommand.Process.Kill()
		if err != nil {
			log.Log.Error(err)
		}
	}

	return
}
