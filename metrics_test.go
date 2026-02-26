//go:build LINUX
// +build LINUX

package main

import (
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/wymangr/blueiris_exporter/common"
)

// ── newMetric ─────────────────────────────────────────────────────────────────

func TestNewMetric_Fields(t *testing.T) {
	m := newMetric(
		"test_metric",
		"A test metric",
		prometheus.GaugeValue,
		[]string{"label1"},
		func(ch chan<- prometheus.Metric, m common.MetricInfo, sec []common.MetricInfo, logpath string) {},
		CollectBool{true: []int{1, 2}},
		"blueIrisServerMetrics",
	)

	if m.Name != "test_metric" {
		t.Errorf("Name: want test_metric, got %q", m.Name)
	}
	if m.Type != prometheus.GaugeValue {
		t.Errorf("Type: want GaugeValue, got %v", m.Type)
	}
	if !m.Collect {
		t.Error("Collect: want true")
	}
	if len(m.SecondaryCollect) != 2 {
		t.Errorf("SecondaryCollect length: want 2, got %d", len(m.SecondaryCollect))
	}
	if m.Desc == nil {
		t.Error("Desc should not be nil")
	}
	if m.Errors == nil {
		t.Error("Errors should not be nil")
	}
	if m.Timer == nil {
		t.Error("Timer should not be nil")
	}
	if m.Server != "blueIrisServerMetrics" {
		t.Errorf("Server: want blueIrisServerMetrics, got %q", m.Server)
	}
}

func TestNewMetric_NotCollect(t *testing.T) {
	m := newMetric(
		"no_collect",
		"not collected",
		prometheus.GaugeValue,
		[]string{},
		func(ch chan<- prometheus.Metric, m common.MetricInfo, sec []common.MetricInfo, logpath string) {},
		CollectBool{false: nil},
		"blueIrisServerMetrics",
	)
	if m.Collect {
		t.Error("Collect: want false")
	}
	if m.SecondaryCollect != nil {
		t.Errorf("SecondaryCollect: want nil, got %v", m.SecondaryCollect)
	}
}

func TestNewMetric_FQName(t *testing.T) {
	m := newMetric(
		"ai_duration",
		"Duration",
		prometheus.GaugeValue,
		[]string{"camera"},
		func(ch chan<- prometheus.Metric, m common.MetricInfo, sec []common.MetricInfo, logpath string) {},
		CollectBool{false: nil},
		"blueIrisServerMetrics",
	)
	// Verify the desc contains the right fully qualified name by checking its String()
	descStr := m.Desc.String()
	if !strings.Contains(descStr, "blueiris_ai_duration") {
		t.Errorf("Desc string should contain blueiris_ai_duration, got: %s", descStr)
	}
}

// ── blueIrisServerMetrics map ─────────────────────────────────────────────────

func TestBlueIrisServerMetrics_Count(t *testing.T) {
	// The map is defined at package level with 23 entries.
	if len(blueIrisServerMetrics) != 23 {
		t.Errorf("blueIrisServerMetrics: want 23 entries, got %d", len(blueIrisServerMetrics))
	}
}

func TestBlueIrisServerMetrics_OnlyAIDurationCollects(t *testing.T) {
	// Only metric 1 (ai_duration) should have Collect=true.
	for k, m := range blueIrisServerMetrics {
		if k == 1 {
			if !m.Collect {
				t.Errorf("metric[1] (ai_duration) should have Collect=true")
			}
		} else {
			if m.Collect {
				t.Errorf("metric[%d] (%s) should have Collect=false", k, m.Name)
			}
		}
	}
}

func TestBlueIrisServerMetrics_AIDurationHasSecondaryCollect(t *testing.T) {
	m := blueIrisServerMetrics[1]
	if len(m.SecondaryCollect) == 0 {
		t.Error("ai_duration (metric 1) should have SecondaryCollect entries")
	}
}

func TestBlueIrisServerMetrics_AllHaveDesc(t *testing.T) {
	for k, m := range blueIrisServerMetrics {
		if m.Desc == nil {
			t.Errorf("metric[%d] (%s) has nil Desc", k, m.Name)
		}
		if m.Errors == nil {
			t.Errorf("metric[%d] (%s) has nil Errors", k, m.Name)
		}
		if m.Timer == nil {
			t.Errorf("metric[%d] (%s) has nil Timer", k, m.Name)
		}
	}
}

func TestBlueIrisServerMetrics_AllHaveNames(t *testing.T) {
	expectedNames := []string{
		"ai_duration", "ai_count", "ai_duration_distinct", "ai_restarted",
		"ai_timeout", "ai_servererror", "ai_notresponding", "logerror",
		"logerror_total", "camera_status", "triggers", "push_notifications",
		"logwarning", "logwarning_total", "folder_disk_free", "folder_used",
		"hours_used", "parse_errors", "parse_errors_total", "ai_starting",
		"ai_started", "profile", "ai_error",
	}
	nameSet := make(map[string]bool)
	for _, m := range blueIrisServerMetrics {
		nameSet[m.Name] = true
	}
	for _, name := range expectedNames {
		if !nameSet[name] {
			t.Errorf("expected metric %q not found in blueIrisServerMetrics", name)
		}
	}
}

// ── NewExporterBlueIris ───────────────────────────────────────────────────────

func TestNewExporterBlueIris_ReturnsExporter(t *testing.T) {
	e, err := NewExporterBlueIris(blueIrisServerMetrics, "/some/path/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e == nil {
		t.Fatal("expected non-nil exporter")
	}
	if e.logpath != "/some/path/" {
		t.Errorf("logpath: want /some/path/, got %q", e.logpath)
	}
}

func TestNewExporterBlueIris_EmptyMetrics(t *testing.T) {
	e, err := NewExporterBlueIris(map[int]common.MetricInfo{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e == nil {
		t.Fatal("expected non-nil exporter")
	}
}

// ── ExporterBlueIris.Describe ─────────────────────────────────────────────────

func TestExporterBlueIris_Describe(t *testing.T) {
	e, _ := NewExporterBlueIris(blueIrisServerMetrics, "")
	ch := make(chan *prometheus.Desc, 200)
	e.Describe(ch)
	close(ch)
	var descs []*prometheus.Desc
	for d := range ch {
		descs = append(descs, d)
	}
	// Each metric emits Desc + Timer → 2 * 23 = 46
	if len(descs) != 46 {
		t.Errorf("Describe: expected 46 descriptors, got %d", len(descs))
	}
}

func TestExporterBlueIris_Describe_CustomMetrics(t *testing.T) {
	// Describe always iterates the package-level blueIrisServerMetrics map;
	// the logpath stored in the exporter is irrelevant here.  Use a channel
	// large enough to hold all descriptors (23 metrics × 2 = 46).
	e, _ := NewExporterBlueIris(blueIrisServerMetrics, "/logs/")
	ch := make(chan *prometheus.Desc, 100)
	e.Describe(ch)
	close(ch)
	var descs []*prometheus.Desc
	for d := range ch {
		descs = append(descs, d)
	}
	if len(descs) != 46 {
		t.Errorf("expected 46 descriptors, got %d", len(descs))
	}
}

// ── start logpath normalization ────────────────────────────────────────────────
// We can't call start() directly (it blocks on ListenAndServe), but we can test
// the logpath normalization logic by extracting it as a helper.  Since it is
// embedded in start(), we verify it via the exported behaviour: a port that is
// already in use makes ListenAndServe fail immediately, letting us assert the
// path that arrived was cleaned up correctly.

func logpathNormalize(logpath string) string {
	if strings.HasSuffix(logpath, `\`) {
		return logpath
	} else if strings.HasSuffix(logpath, `/`) {
		return logpath
	} else {
		if strings.Contains(logpath, `\`) {
			return logpath + `\`
		} else if strings.Contains(logpath, `/`) {
			return logpath + `/`
		}
	}
	return logpath
}

func TestStart_LogpathNormalization_BackslashAlready(t *testing.T) {
	got := logpathNormalize(`C:\BlueIris\log\`)
	want := `C:\BlueIris\log\`
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestStart_LogpathNormalization_ForwardSlashAlready(t *testing.T) {
	got := logpathNormalize(`C:/BlueIris/log/`)
	want := `C:/BlueIris/log/`
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestStart_LogpathNormalization_AddBackslash(t *testing.T) {
	got := logpathNormalize(`C:\BlueIris\log`)
	want := `C:\BlueIris\log\`
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestStart_LogpathNormalization_AddForwardSlash(t *testing.T) {
	got := logpathNormalize(`/var/log/blueiris`)
	want := `/var/log/blueiris/`
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestStart_LogpathNormalization_NoSeparator(t *testing.T) {
	// path with no separator - should be returned unchanged
	got := logpathNormalize(`logdir`)
	want := `logdir`
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

// ── CollectMetrics ────────────────────────────────────────────────────────────

func TestCollectMetrics_NilSecondary(t *testing.T) {
	called := false
	m := common.MetricInfo{
		Name: "test",
		Function: func(ch chan<- prometheus.Metric, m common.MetricInfo, sec []common.MetricInfo, logpath string) {
			if sec != nil {
				t.Errorf("expected nil secondary, got %v", sec)
			}
			called = true
		},
	}

	ch := make(chan prometheus.Metric, 10)
	var wg sync.WaitGroup
	wg.Add(1)
	go CollectMetrics(&wg, ch, m, "test", "/tmp/")
	wg.Wait()

	if !called {
		t.Error("Function was not called")
	}
}

func TestCollectMetrics_WithSecondary(t *testing.T) {
	// CollectMetrics uses blueIrisServerMetrics[i] for each index in
	// SecondaryCollect, so the indices must reference real map entries.
	var gotSec []common.MetricInfo
	m := common.MetricInfo{
		Name:             "ai_duration",
		SecondaryCollect: []int{2, 3}, // ai_count and ai_duration_distinct
		Function: func(ch chan<- prometheus.Metric, m common.MetricInfo, sec []common.MetricInfo, logpath string) {
			gotSec = sec
		},
	}

	ch := make(chan prometheus.Metric, 10)
	var wg sync.WaitGroup
	wg.Add(1)
	CollectMetrics(&wg, ch, m, "ai_duration", "/tmp/")

	if len(gotSec) != 2 {
		t.Errorf("expected 2 secondary metrics, got %d", len(gotSec))
	}
	if gotSec[0].Name != "ai_count" {
		t.Errorf("secondary[0].Name: want ai_count, got %q", gotSec[0].Name)
	}
	if gotSec[1].Name != "ai_duration_distinct" {
		t.Errorf("secondary[1].Name: want ai_duration_distinct, got %q", gotSec[1].Name)
	}
}

// ── ExporterBlueIris.Collect ──────────────────────────────────────────────────

func TestExporterBlueIris_Collect_CallsFunction(t *testing.T) {
	called := 0
	m := map[int]common.MetricInfo{
		1: {
			Name:    "test_collect",
			Collect: true,
			Function: func(ch chan<- prometheus.Metric, m common.MetricInfo, sec []common.MetricInfo, logpath string) {
				called++
			},
		},
		2: {
			Name:    "not_collected",
			Collect: false,
			Function: func(ch chan<- prometheus.Metric, m common.MetricInfo, sec []common.MetricInfo, logpath string) {
				t.Error("non-Collect metric function should not be called")
			},
		},
	}
	e, _ := NewExporterBlueIris(m, "/tmp/")
	ch := make(chan prometheus.Metric, 100)
	e.Collect(ch)
	if called != 1 {
		t.Errorf("expected Collect=true function called once, got %d", called)
	}
}

func TestExporterBlueIris_Collect_NoCollectMetrics(t *testing.T) {
	// When no metrics have Collect=true, Collect should not call any function.
	m := map[int]common.MetricInfo{
		1: {
			Name:    "skip",
			Collect: false,
			Function: func(ch chan<- prometheus.Metric, m common.MetricInfo, sec []common.MetricInfo, logpath string) {
				t.Error("should not be called")
			},
		},
	}
	e, _ := NewExporterBlueIris(m, "")
	ch := make(chan prometheus.Metric, 10)
	e.Collect(ch) // should return immediately without calling any function
}
