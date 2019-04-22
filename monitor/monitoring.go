package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/teamnsrg/mida/log"
	t "github.com/teamnsrg/mida/types"
	"net/http"
	"strconv"
)

// RunPrometheusClient is responsible for running a client which will
// be scraped by a Prometheus server
func RunPrometheusClient(monitoringChan <-chan t.TaskStats, port int) {

	browserDurationHistogram := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "browser_duration_seconds",
			Help:    "A histogram of browser open durations",
			Buckets: prometheus.LinearBuckets(0, 2, 45),
		})
	prometheus.MustRegister(browserDurationHistogram)

	postprocessDurationHistogram := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "postprocess_duration_seconds",
			Help:    "A histogram of postprocessing durations",
			Buckets: prometheus.LinearBuckets(0, 2, 45),
		})
	prometheus.MustRegister(postprocessDurationHistogram)

	storageDurationHistogram := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "storage_duration_seconds",
			Help:    "A histogram of results storage durations",
			Buckets: prometheus.LinearBuckets(0, 2, 45),
		})
	prometheus.MustRegister(storageDurationHistogram)

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		for taskStats := range monitoringChan {
			// Update all of our Prometheus metrics using the TaskStats object
			browserDurationHistogram.Observe(taskStats.Timing.BrowserClose.Sub(taskStats.Timing.BrowserOpen).Seconds())
			postprocessDurationHistogram.Observe(taskStats.Timing.EndPostprocess.Sub(taskStats.Timing.BeginPostprocess).Seconds())
			storageDurationHistogram.Observe(taskStats.Timing.EndStorage.Sub(taskStats.Timing.BeginStorage).Seconds())
		}
	}()

	log.Log.Error(http.ListenAndServe(":"+strconv.Itoa(port), nil))

	return
}
