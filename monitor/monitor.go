package monitor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	b "github.com/teamnsrg/mida/base"
	"net/http"
	"strconv"
)

// RunPrometheusClient is responsible for running a client which will
// be scraped by a Prometheus server
func RunPrometheusClient(monitoringChan <-chan *b.TaskSummary, port int) {

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
		for taskSum := range monitoringChan {
			// Update all of our Prometheus metrics using the TaskSummary object
			browserDurationHistogram.Observe(taskSum.TaskTiming.BrowserClose.Sub(taskSum.TaskTiming.BrowserOpen).Seconds())
			postprocessDurationHistogram.Observe(taskSum.TaskTiming.EndPostprocess.Sub(taskSum.TaskTiming.BeginPostprocess).Seconds())
			storageDurationHistogram.Observe(taskSum.TaskTiming.EndStorage.Sub(taskSum.TaskTiming.BeginStorage).Seconds())
		}
	}()

	http.ListenAndServe(":"+strconv.Itoa(port), nil)

	return
}
