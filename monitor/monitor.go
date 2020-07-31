package monitor

import (
	"github.com/montanaflynn/stats"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	b "github.com/teamnsrg/mida/base"
	"github.com/teamnsrg/mida/log"
	"net/http"
	"strconv"
)

// RunPrometheusClient is responsible for running a client which will
// be scraped by a Prometheus server
func RunPrometheusClient(monitoringChan <-chan *b.TaskSummary, port int) {

	http.Handle("/metrics", promhttp.Handler())

	var browserOpenTimeBuffer []float64
	browserOpenTime := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mida_browser_open_time_seconds",
		Help: "Number of seconds browser remains open (Median of last 5)",
	})
	prometheus.MustRegister(browserOpenTime)

	var storageTimeBuffer []float64
	storageTime := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mida_storage_time_seconds",
		Help: "Amount of time spent storing task results (Median of last 5)",
	})
	prometheus.MustRegister(storageTime)


	go func() {
		for ts := range monitoringChan {
			// Update all of our Prometheus metrics using the TaskSummary object

			var reading float64
			var median  float64

			// Update browser open time
			reading = ts.TaskTiming.BrowserClose.Sub(ts.TaskTiming.BrowserOpen).Seconds()
			median, browserOpenTimeBuffer = updateMemorySlice(browserOpenTimeBuffer, reading, 5)
			browserOpenTime.Set(median)

			// Update time to store results
			reading = ts.TaskTiming.EndStorage.Sub(ts.TaskTiming.BeginStorage).Seconds()
			median, storageTimeBuffer = updateMemorySlice(storageTimeBuffer, reading, 5)
			storageTime.Set(median)
		}
	}()

	err := http.ListenAndServe(":"+strconv.Itoa(port), nil)
	if err != nil {
		log.Log.Error(err)
	}

	return

}


func updateMemorySlice(memory []float64, reading float64, length int) (float64, []float64) {
	newMemory := append([]float64{reading}, memory...)
	if len(newMemory) > length {
		newMemory = newMemory[:length]
	}

	med, err := stats.Median(newMemory)
	if err != nil {
		log.Log.Error(err)
		return 0, []float64{}
	}
	return med, newMemory
}