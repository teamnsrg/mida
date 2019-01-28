package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

func RunPrometheusClient() error {

	http.Handle("/metrics", promhttp.Handler())

	log.Info("Running Prom client server")
	log.Fatal(http.ListenAndServe(":8080", nil))
	return nil
}
