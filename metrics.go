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
	cameras               string
	logpath               string
}

var (
	namespace string = "blueiris"

	blueIrisServerMetrics = metrics{
		1: newMetric("ai_duration", "Duration of Blue Iris AI analysis", prometheus.GaugeValue, []string{"camera", "type", "object", "detail"}, blueiris.BlueIris, CollectBool{true: []int{2, 3, 4, 5, 6}}, "blueIrisServerMetrics"),
		2: newMetric("ai_count", "Count of Blue Iris AI analysis", prometheus.GaugeValue, []string{"camera", "type"}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		3: newMetric("ai_duration_distinct", "Duration of Blue Iris AI analysis once", prometheus.GaugeValue, []string{"camera", "type", "object", "detail"}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		4: newMetric("ai_restarted", "Times BlueIris restarted Deepstack", prometheus.GaugeValue, []string{}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		5: newMetric("logerror", "Count of unique errors in the logs", prometheus.GaugeValue, []string{"error"}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
		6: newMetric("logerror_total", "Count all errors in the logs", prometheus.GaugeValue, []string{}, blueiris.BlueIris, CollectBool{false: nil}, "blueIrisServerMetrics"),
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
	f func(ch chan<- prometheus.Metric, m common.MetricInfo, SecMet []common.MetricInfo, cameras []string, logpath string),
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
	cameras []string,
	logpath string,
) {

	defer wg.Done()

	if m.SecondaryCollect == nil {
		m.Function(ch, m, nil, cameras, logpath)
	} else {
		var secMet []common.MetricInfo
		secMet = nil
		for _, i := range m.SecondaryCollect {
			secMet = append(secMet, blueIrisServerMetrics[i])
		}
		m.Function(ch, m, secMet, cameras, logpath)
	}
}
