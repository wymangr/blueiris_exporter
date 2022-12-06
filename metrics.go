package main

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/wymangr/blueiris_exporter/blueiris"
	"github.com/wymangr/blueiris_exporter/common"
)

type metrics map[int]common.MetricInfo

type CollectBool map[bool][]int

type ExporterBlueIris struct {
	blueIrisServerMetrics map[int]common.MetricInfo
	logpath               string
}

var (
	namespace string = "blueiris"

	blueIrisServerMetrics = metrics{
		1:  newMetric("ai_duration", "Duration of Blue Iris AI analysis", prometheus.GaugeValue, []string{"camera", "type", "object", "detail"}, blueiris.BlueIris, CollectBool{true: []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17}}, "blueIrisServerMetrics"),
		2:  newMetric("ai_count", "Count of Blue Iris AI analysis", prometheus.GaugeValue, []string{"camera", "type"}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		3:  newMetric("ai_duration_distinct", "Duration of Blue Iris AI analysis once", prometheus.GaugeValue, []string{"camera", "type", "object", "detail"}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		4:  newMetric("ai_restarted", "Times BlueIris restarted Deepstack", prometheus.GaugeValue, []string{}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		5:  newMetric("ai_timeout", "Count of AI timeouts in current logfile", prometheus.GaugeValue, []string{}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		6:  newMetric("ai_servererror", "Count of AI server not responding errors in current logfile", prometheus.GaugeValue, []string{}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		7:  newMetric("ai_notresponding", "Count of AI not responding errors in current logfile", prometheus.GaugeValue, []string{}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		8:  newMetric("logerror", "Count of unique errors in the logs", prometheus.GaugeValue, []string{"error"}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		9:  newMetric("logerror_total", "Count all errors in the logs", prometheus.GaugeValue, []string{}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		10: newMetric("camera_status", "Status of each camera. 0=up, 1=down", prometheus.GaugeValue, []string{"camera", "detail"}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		11: newMetric("triggers", "Count of triggers", prometheus.GaugeValue, []string{"camera"}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		12: newMetric("push_notifications", "Count of push notifications sent", prometheus.GaugeValue, []string{"camera", "status", "detail"}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		13: newMetric("logwarning", "Count of unique warnings in the logs", prometheus.GaugeValue, []string{"warning"}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		14: newMetric("logwarning_total", "Count all warnings in the logs", prometheus.GaugeValue, []string{}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		15: newMetric("folder_disk_free", "Free space of the disk the folder is using in bytes", prometheus.GaugeValue, []string{"folder"}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		16: newMetric("folder_used", "Percentage of folder bytes used based on limit", prometheus.GaugeValue, []string{"folder"}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		17: newMetric("hours_used", "Percentage of folder hours used based on limit", prometheus.GaugeValue, []string{"folder"}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
	}

	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "collector_duration_seconds"),
		"Collector time duration.",
		[]string{"collector"}, nil,
	)
)

func newMetric(
	metricName string,
	docString string,
	t prometheus.ValueType,
	labels []string,
	f func(ch chan<- prometheus.Metric, m common.MetricInfo, SecMet []common.MetricInfo, logpath string),
	collect CollectBool,
	ServerMetrics string) common.MetricInfo {

	var colBoo bool
	var secCol []int

	for k, v := range collect {
		colBoo = k
		secCol = v
	}
	return common.MetricInfo{
		Desc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", metricName),
			docString,
			labels,
			nil,
		),
		Type:             t,
		Name:             metricName,
		Collect:          colBoo,
		SecondaryCollect: secCol,
		Function:         f,
		Server:           ServerMetrics,
		Errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_errors",
			Help:      "blueiris_exporter errors",
		},
			[]string{"function"}),
		Timer: scrapeDurationDesc,
	}
}

func CollectMetrics(
	wg *sync.WaitGroup,
	ch chan<- prometheus.Metric,
	m common.MetricInfo,
	n string,
	logpath string,
) {

	defer wg.Done()

	if m.SecondaryCollect == nil {
		m.Function(ch, m, nil, logpath)
	} else {
		var secMet []common.MetricInfo
		secMet = nil
		for _, i := range m.SecondaryCollect {
			secMet = append(secMet, blueIrisServerMetrics[i])
		}
		m.Function(ch, m, secMet, logpath)
	}
}
