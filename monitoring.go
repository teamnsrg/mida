package main

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func RunPrometheusClient() {

	http.Handle("/metrics", promhttp.Handler())

	log.Info("Running Prom client server")
	log.Fatal(http.ListenAndServe(":8080", nil))
	return
}
