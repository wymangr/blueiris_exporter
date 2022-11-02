package main

import (
	"fmt"
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

	for _, m := range e.blueIrisServerMetrics {

		if m.Collect {
			name := m.Name
			wg.Add(1)
			go CollectMetrics(&wg, ch, m, name, cameras, e.logpath)
		}
	}

	wg.Wait()
}

func start(cameras string, logpath string, metricsPath string, port string) error {

	var finalPort string
	var finalLogpath string

	if strings.HasSuffix(logpath, `\`) {
		finalLogpath = logpath
	} else if strings.HasSuffix(logpath, `/`) {
		finalLogpath = logpath
	} else {
		if strings.Contains(logpath, `\`) {
			finalLogpath = logpath + `\`
		} else if strings.Contains(logpath, `/`) {
			finalLogpath = logpath + `/`
		}
	}

	exporterBlueIris, _ := NewExporterBlueIris(blueIrisServerMetrics, cameras, finalLogpath)
	blueIrisReg := prometheus.NewRegistry()
	blueIrisReg.MustRegister(exporterBlueIris, promcollectors.NewGoCollector())

	http.Handle(metricsPath, promhttp.HandlerFor(blueIrisReg, promhttp.HandlerOpts{}))

	if strings.Contains(port, ":") {
		finalPort = port
	} else {
		finalPort = ":" + port
	}
	err := http.ListenAndServe(finalPort, nil)
	if err != nil {
		return err
	}
	return nil
}

func main() {

	const svcName = "blueiris_exporter"
	const svcNameLong = "Blue Iris Exporter"

	err := IsService(svcName)
	if err != nil {
		common.BIlogger(err.Error(), "error")
		return
	}

	var (
		install = kingpin.Flag(
			"service.install",
			"Install as windows Service",
		).Bool()
		uninstall = kingpin.Flag(
			"service.uninstall",
			"Uninstall as windows Service",
		).Bool()
		serviceStart = kingpin.Flag(
			"service.start",
			"Start Blue Iris Exporter Windows Service",
		).Bool()
		serviceStop = kingpin.Flag(
			"service.stop",
			"Stop Blue Iris Exporter Windows Service",
		).Bool()
		servicePause = kingpin.Flag(
			"service.pause",
			"Pause Blue Iris Exporter Windows Service",
		).Bool()
		serviceContinue = kingpin.Flag(
			"service.continue",
			"Continue Blue Iris Exporter Windows Service",
		).Bool()
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

	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	if *install {
		err := installService(svcName, svcNameLong, *cameras, *logpath, *metricsPath, *port)
		if err != nil {
			common.BIlogger(err.Error(), "error")
		}
	} else if *uninstall {
		err := removeService(svcName)
		if err != nil {
			common.BIlogger(err.Error(), "error")
		}
		return
	} else if *serviceStart {
		err := startService(svcName)
		if err != nil {
			common.BIlogger(err.Error(), "error")
		}
	} else if *serviceStop {
		err = controlService(svcName, "Stop")
		if err != nil {
			common.BIlogger(err.Error(), "error")
		}
	} else if *servicePause {
		err = controlService(svcName, "Pause")
		if err != nil {
			common.BIlogger(err.Error(), "error")
		}
	} else if *serviceContinue {
		err = controlService(svcName, "Continue")
		if err != nil {
			common.BIlogger(err.Error(), "error")
		}
	} else {
		var a string = fmt.Sprintf(`starting Blue Iris Exporter with the following:
		Cameras: %v
		Log Path: %v
		Metric Path: %v
		Port: %v`, *cameras, *logpath, *metricsPath, *port)
		common.BIlogger(a, "info")

		err := start(*cameras, *logpath, *metricsPath, *port)
		if err != nil {
			common.BIlogger(fmt.Sprintf("Error starting blueiris_exporter. err: %v", err), "error")
		}
	}

}
