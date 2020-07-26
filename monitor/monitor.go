package monitor

import (
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

	go func() {
		for _ = range monitoringChan {
			// Update all of our Prometheus metrics using the TaskSummary object

		}
	}()

	err := http.ListenAndServe(":"+strconv.Itoa(port), nil)
	if err != nil {
		log.Log.Error(err)
	}

	return

}
