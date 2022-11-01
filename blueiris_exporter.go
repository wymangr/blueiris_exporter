package main

import (
	"net/http"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	promcollectors "github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wymangr/blueiris_exporter/common"
	"gopkg.in/alecthomas/kingpin.v2"
)

func NewExporterBlueIris(selectedServerMetrics map[int]common.MetricInfo, cameras string, logpath string) (*ExporterBlueIris, error) {
	return &ExporterBlueIris{
		blueIrisServerMetrics: selectedServerMetrics,
		cameras:               cameras,
		logpath:               logpath,
	}, nil
}

func (e *ExporterBlueIris) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range blueIrisServerMetrics {
		ch <- m.Desc
		ch <- m.Timer
	}
}

func (e *ExporterBlueIris) Collect(ch chan<- prometheus.Metric) {
	var wg sync.WaitGroup

	cameras := strings.Split(e.cameras, ",")
	var logpath string
	if strings.HasSuffix(e.logpath, `\`) {
		logpath = e.logpath
	} else if strings.HasSuffix(e.logpath, `/`) {
		logpath = e.logpath
	} else {
		if strings.Contains(e.logpath, `\`) {
			logpath = e.logpath + `\`
		} else if strings.Contains(e.logpath, `/`) {
			logpath = e.logpath + `/`
		}
	}

	for _, m := range e.blueIrisServerMetrics {

		if m.Collect {
			name := m.Name
			wg.Add(1)
			go CollectMetrics(&wg, ch, m, name, cameras, logpath)
		}
	}

	wg.Wait()
}

func main() {

	var (
		cameras = kingpin.Flag(
			"cameras",
			"Comma-separated list of camera shot names",
		).String()

		port = kingpin.Flag(
			"telemetry.addr",
			"Addresses on which to expose metrics",
		).Default(":2112").String()

		logpath = kingpin.Flag(
			"logpath",
			"Directory path to the Blue Iris Logs",
		).Default(`C:\BlueIris\log\`).String()

		metricsPath = kingpin.Flag(
			"telemetry.path",
			"URL path for surfacing collected metrics.",
		).Default("/metrics").String()
	)

	var finalPort string

	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	exporterBlueIris, _ := NewExporterBlueIris(blueIrisServerMetrics, *cameras, *logpath)
	blueIrisReg := prometheus.NewRegistry()
	blueIrisReg.MustRegister(exporterBlueIris, promcollectors.NewGoCollector())

	http.Handle(*metricsPath, promhttp.HandlerFor(blueIrisReg, promhttp.HandlerOpts{}))

	if strings.Contains(*port, ":") {
		finalPort = *port
	} else {
		finalPort = ":" + *port
	}
	http.ListenAndServe(finalPort, nil)
}
