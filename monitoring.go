package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"strconv"
	"time"
)

// Statistics from the execution of a single task, used for monitoring
// the performance of MIDA through Prometheus/Grafana
type TaskStats struct {

	///// GENERAL TASK METRICS /////
	TaskSucceeded bool
	SanitizedTask SanitizedMIDATask

	///// TIMING METRICS /////
	StartTime             time.Time
	TimeAfterStorage      time.Time
	TimeAfterBrowserClose time.Time
	TimeAfterValidation   time.Time

	///// RESULTS METRICS /////
	RawJSTraceSize uint // Size of raw JS trace (Log from browser) in bytes

}

func RunPrometheusClient(monitoringChan <-chan TaskStats, port int) {

	browserDurationHistogram := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "browser_duration_seconds",
			Help:    "A histogram of browser open durations",
			Buckets: prometheus.LinearBuckets(0, 2, 45),
		})
	prometheus.MustRegister(browserDurationHistogram)

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		for t := range monitoringChan {
			// Update all of our Prometheus metrics using the TaskStats object
			browserDurationHistogram.Observe(t.TimeAfterBrowserClose.Sub(t.StartTime).Seconds())
		}
	}()

	Log.Error(http.ListenAndServe(":"+strconv.Itoa(port), nil))

	return
}
