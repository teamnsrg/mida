package monitor

import (
	"github.com/pmurley/mida/log"
	t "github.com/pmurley/mida/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"strconv"
)

func RunPrometheusClient(monitoringChan <-chan t.TaskStats, port int) {

	browserDurationHistogram := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "browser_duration_seconds",
			Help:    "A histogram of browser open durations",
			Buckets: prometheus.LinearBuckets(0, 2, 45),
		})
	prometheus.MustRegister(browserDurationHistogram)

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		for taskStats := range monitoringChan {
			// Update all of our Prometheus metrics using the TaskStats object
			browserDurationHistogram.Observe(taskStats.Timing.EndStorage.Sub(taskStats.Timing.BeginCrawl).Seconds())
		}
	}()

	log.Log.Error(http.ListenAndServe(":"+strconv.Itoa(port), nil))

	return
}
