package main

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

// Statistics from the execution of a single task, used for monitoring
// the performance of MIDA through Prometheus/Grafana
type TaskStats struct {

	///// GENERAL TASK METRICS /////
	TaskSucceeded bool
	SanitizedTask SanitizedMIDATask

	///// TIMING METRICS /////
	StartTime            float64 //
	TotalTime            float64 // Time from receipt of raw task to completion of storage
	TaskSanitizationTime float64 // Time to sanitize task
	BrowserTime          float64 // Time the browser is open for this task
	ValidationTime       float64 // Time spent in results validation

	///// RESULTS METRICS /////
	RawJSTraceSize uint // Size of raw JS trace (log from browser) in bytes

}

func RunPrometheusClient(c chan TaskStats, port int) {

	http.Handle("/metrics", promhttp.Handler())
	log.Info("Running Prom client")

	go func() {
		for t := range c {
			log.Info("Update metrics here", t)
		}
	}()

	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))

	return
}
