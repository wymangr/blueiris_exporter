package common

import (
	"github.com/prometheus/client_golang/prometheus"
)

type MetricInfo struct {
	Desc             *prometheus.Desc
	Type             prometheus.ValueType
	Name             string
	Collect          bool
	SecondaryCollect []int
	Function         func(ch chan<- prometheus.Metric, m MetricInfo, SecMet []MetricInfo, cameras []string, logpath string)
	Server           string
	Errors           *prometheus.CounterVec
	Timer            *prometheus.Desc
}
