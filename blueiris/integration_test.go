//go:build LINUX

package blueiris

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/wymangr/blueiris_exporter/common"
)

// copyTestLog copies src to dst.
func copyTestLog(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("copyTestLog open %s: %v", src, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("copyTestLog create %s: %v", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatalf("copyTestLog copy: %v", err)
	}
}

// setupIntegrationDir copies both integration test log files into a new temp
// directory and sets the prior file mtime to one hour before the main file.
// Returns the directory path with a trailing slash.
func setupIntegrationDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	copyTestLog(t,
		filepath.Join("testdata", "integration", "integration_test_prior.log"),
		filepath.Join(dir, "integration_test_prior.log"),
	)
	copyTestLog(t,
		filepath.Join("testdata", "integration", "integration_test.log"),
		filepath.Join(dir, "integration_test.log"),
	)
	priorTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "integration_test_prior.log"), priorTime, priorTime); err != nil {
		t.Fatalf("Chtimes prior: %v", err)
	}
	return filepath.ToSlash(dir) + "/"
}

// fullSecondaryMetrics returns a []common.MetricInfo covering every secondary
// metric name that BlueIris() handles.
func fullSecondaryMetrics() []common.MetricInfo {
	return []common.MetricInfo{
		newTestMetric("ai_count", []string{"camera", "type"}),
		newTestMetric("ai_duration_distinct", []string{"camera", "type", "object", "detail"}),
		newTestMetric("ai_error", []string{}),
		newTestMetric("ai_starting", []string{}),
		newTestMetric("ai_started", []string{}),
		newTestMetric("ai_restarted", []string{}),
		newTestMetric("ai_timeout", []string{}),
		newTestMetric("ai_servererror", []string{}),
		newTestMetric("ai_notresponding", []string{}),
		newTestMetric("triggers", []string{"camera"}),
		newTestMetric("folder_disk_free", []string{"folder"}),
		newTestMetric("folder_used", []string{"folder"}),
		newTestMetric("hours_used", []string{"folder"}),
		newTestMetric("push_notifications", []string{"camera", "status", "detail"}),
		newTestMetric("profile", []string{"profile"}),
		newTestMetric("parse_errors", []string{"line"}),
		newTestMetric("parse_errors_total", []string{}),
		newTestMetric("logerror", []string{"error"}),
		newTestMetric("logerror_total", []string{}),
		newTestMetric("logwarning", []string{"warning"}),
		newTestMetric("logwarning_total", []string{}),
		newTestMetric("camera_status", []string{"camera", "detail"}),
	}
}

// TestIntegration_FullMetricScan calls BlueIris() against the integration test
// log file and asserts exact counter values for every metric code path covered
// by the log file.
func TestIntegration_FullMetricScan(t *testing.T) {
	resetState()
	dir := setupIntegrationDir(t)

	ch := make(chan prometheus.Metric, 2048)
	m := newTestMetric("ai_duration", []string{"camera", "type", "object", "detail"})
	BlueIris(ch, m, fullSecondaryMetrics(), dir)
	close(ch)
	metrics := drainChannel(ch)
	if len(metrics) == 0 {
		t.Fatal("expected at least one metric to be emitted")
	}

	// ── AI counters ───────────────────────────────────────────────────────────

	if timeoutcount != 1 {
		t.Errorf("timeoutcount: want 1, got %v", timeoutcount)
	}
	if aiErrorCount != 1 {
		t.Errorf("aiErrorCount: want 1, got %v", aiErrorCount)
	}
	if notrespondingcount != 1 {
		t.Errorf("notrespondingcount: want 1, got %v", notrespondingcount)
	}
	if restartCount != 1 {
		t.Errorf("restartCount: want 1, got %v", restartCount)
	}
	// "AI is being restarted" (1) + "AI: is being started" (1)
	if aiRestartingCount != 2 {
		t.Errorf("aiRestartingCount: want 2, got %v", aiRestartingCount)
	}
	// "AI has been started" (1) + "AI: has been started" (1)
	if aiRestartedCount != 2 {
		t.Errorf("aiRestartedCount: want 2, got %v", aiRestartedCount)
	}
	if servererrorcount != 1 {
		t.Errorf("servererrorcount: want 1, got %v", servererrorcount)
	}

	// ── AI alert metrics ──────────────────────────────────────────────────────

	// FD: "AI: [Objects]" (line 3) + "DeepStack: [Objects]" (line 23) = alertcount 2
	if aiMetrics["FDalert"].camera != "FD" {
		t.Errorf("aiMetrics[FDalert].camera: want FD, got %q", aiMetrics["FDalert"].camera)
	}
	if aiMetrics["FDalert"].alertcount != 2 {
		t.Errorf("aiMetrics[FDalert].alertcount: want 2, got %v", aiMetrics["FDalert"].alertcount)
	}
	if aiMetrics["BYalert"].camera != "BY" {
		t.Errorf("aiMetrics[BYalert].camera: want BY, got %q", aiMetrics["BYalert"].camera)
	}
	if aiMetrics["DWalert"].camera != "DW" {
		t.Errorf("aiMetrics[DWalert].camera: want DW, got %q", aiMetrics["DWalert"].camera)
	}
	if aiMetrics["GarageExterioralert"].camera != "GarageExterior" {
		t.Errorf("aiMetrics[GarageExterioralert].camera: want GarageExterior, got %q",
			aiMetrics["GarageExterioralert"].camera)
	}

	// ── AI canceled metrics ───────────────────────────────────────────────────

	if aiMetrics["GATE-MASKcanceled"].camera != "GATE-MASK" {
		t.Errorf("aiMetrics[GATE-MASKcanceled].camera: want GATE-MASK, got %q",
			aiMetrics["GATE-MASKcanceled"].camera)
	}
	if aiMetrics["EastNorthcanceled"].camera != "EastNorth" {
		t.Errorf("aiMetrics[EastNorthcanceled].camera: want EastNorth, got %q",
			aiMetrics["EastNorthcanceled"].camera)
	}
	// FD: two "Trigger: Alert canceled" lines → alertcount 2
	if aiMetrics["FDcanceled"].alertcount != 2 {
		t.Errorf("aiMetrics[FDcanceled].alertcount: want 2, got %v", aiMetrics["FDcanceled"].alertcount)
	}
	if aiMetrics["DWcanceled"].camera != "DW" {
		t.Errorf("aiMetrics[DWcanceled].camera: want DW, got %q", aiMetrics["DWcanceled"].camera)
	}

	// ── Trigger counts ────────────────────────────────────────────────────────

	// FD: MOTION_A + EXTERNAL + DIO + Trigger: Motion_A = 4
	if triggerCount["FD"] != 4 {
		t.Errorf("triggerCount[FD]: want 4, got %v", triggerCount["FD"])
	}
	if triggerCount["GarageExterior"] != 1 {
		t.Errorf("triggerCount[GarageExterior]: want 1, got %v", triggerCount["GarageExterior"])
	}
	if triggerCount["SouthEast"] != 1 {
		t.Errorf("triggerCount[SouthEast]: want 1, got %v", triggerCount["SouthEast"])
	}

	// ── Push notifications ────────────────────────────────────────────────────

	if pushCount["FD|OK|Garret's S21 Ultra"] != 1 {
		t.Errorf("pushCount[FD|OK|...]: want 1, got %v", pushCount["FD|OK|Garret's S21 Ultra"])
	}
	if pushCount["DW|Error|Garret's S21 Ultra"] != 1 {
		t.Errorf("pushCount[DW|Error|...]: want 1, got %v", pushCount["DW|Error|Garret's S21 Ultra"])
	}

	// ── Profile counts ────────────────────────────────────────────────────────

	// Log switches Day → Night; Night becomes 1 and Day is reset to 0.
	if profileCount["Night"] != 1 {
		t.Errorf("profileCount[Night]: want 1, got %v", profileCount["Night"])
	}
	if profileCount["Day"] != 0 {
		t.Errorf("profileCount[Day]: want 0 after switch to Night, got %v", profileCount["Day"])
	}

	// ── Camera status ─────────────────────────────────────────────────────────

	// DW: Signal restored (level-4) → 0.0, then network retry (level-1) → 1.0,
	//     then Socket closed (level-1) → 1.0. Final: 1.0.
	if camerastatus["DW"] == nil {
		t.Fatal("expected camerastatus[DW] to be set")
	}
	if camerastatus["DW"]["status"] != 1.0 {
		t.Errorf("camerastatus[DW][status]: want 1.0, got %v", camerastatus["DW"]["status"])
	}

	// BY: AI alert sets 0.0, then Signal: link down (level-4, not "restored")
	//     → HasPrefix("4") branch → 0.0. Final: 0.0.
	if camerastatus["BY"] == nil {
		t.Fatal("expected camerastatus[BY] to be set")
	}
	if camerastatus["BY"]["status"] != 0.0 {
		t.Errorf("camerastatus[BY][status]: want 0.0, got %v", camerastatus["BY"]["status"])
	}

	// FD: various triggers/alerts set 0.0, then Signal: restored (level-0,
	//     contains "restored") → Contains("restored") branch → 0.0. Final: 0.0.
	if camerastatus["FD"] == nil {
		t.Fatal("expected camerastatus[FD] to be set")
	}
	if camerastatus["FD"]["status"] != 0.0 {
		t.Errorf("camerastatus[FD][status]: want 0.0, got %v", camerastatus["FD"]["status"])
	}

	// ── Log errors / warnings ─────────────────────────────────────────────────

	// Level-2 lines not caught by earlier branches:
	//   "Your license is not authorized..." + "Connection refused" = 2
	if errorMetricsTotal != 2 {
		t.Errorf("errorMetricsTotal: want 2, got %v", errorMetricsTotal)
	}
	// Level-1 lines not caught by earlier branches:
	//   "[Maintenance and support plan expired...]" + "HW decode could not start" = 2
	if warningMetricsTotal != 2 {
		t.Errorf("warningMetricsTotal: want 2, got %v", warningMetricsTotal)
	}

	// ── Disk stats ────────────────────────────────────────────────────────────

	if len(diskStats) == 0 {
		t.Fatal("expected diskStats to be populated")
	}
	// Verify each folder whose Delete line exercises a distinct regex path.
	for _, folder := range []string{"Data Drive", "Stored", "Alerts", "Aux 7", "New", "Clips", "Stored2"} {
		found := false
		for k := range diskStats {
			if strings.Contains(k, folder) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected diskStats entry containing %q; got keys=%v", folder, diskStats)
		}
	}
}

// TestIntegration_NewestFileSelected verifies that BlueIris() selects the
// file with the most recent mtime, not the alphabetically last file.
func TestIntegration_NewestFileSelected(t *testing.T) {
	resetState()
	dir := setupIntegrationDir(t)

	ch := make(chan prometheus.Metric, 256)
	m := newTestMetric("ai_duration", []string{"camera", "type", "object", "detail"})
	BlueIris(ch, m, nil, dir)
	close(ch)
	_ = drainChannel(ch)

	if lastLogFile != "integration_test.log" {
		t.Errorf("lastLogFile: want integration_test.log, got %q", lastLogFile)
	}
}

// TestIntegration_IncrementalScan verifies that a repeat call does not
// re-process lines already seen, and that appended lines are picked up.
func TestIntegration_IncrementalScan(t *testing.T) {
	resetState()
	dir := setupIntegrationDir(t)
	dirPath := filepath.FromSlash(strings.TrimSuffix(dir, "/"))

	m := newTestMetric("ai_duration", []string{"camera", "type", "object", "detail"})
	smTimeout := newTestMetric("ai_timeout", []string{})

	// First call – scans all lines in the main file.
	ch1 := make(chan prometheus.Metric, 256)
	BlueIris(ch1, m, []common.MetricInfo{smTimeout}, dir)
	close(ch1)
	_ = drainChannel(ch1)
	countAfterFirst := timeoutcount // 1 from integration_test.log

	// Second call – file unchanged; timeoutcount must not grow.
	ch2 := make(chan prometheus.Metric, 256)
	BlueIris(ch2, m, []common.MetricInfo{smTimeout}, dir)
	close(ch2)
	_ = drainChannel(ch2)
	if timeoutcount != countAfterFirst {
		t.Errorf("second call without new lines: timeoutcount changed from %v to %v",
			countAfterFirst, timeoutcount)
	}

	// Append a new timeout line and verify a third call picks it up.
	mainFile := filepath.Join(dirPath, "integration_test.log")
	f, err := os.OpenFile(mainFile, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("open for append: %v", err)
	}
	_, writeErr := f.WriteString("2 \t1/1/2024 1:00:00.000 AM\tDW                  \tAI: timeout\r\n")
	f.Close()
	if writeErr != nil {
		t.Fatalf("write append: %v", writeErr)
	}

	ch3 := make(chan prometheus.Metric, 256)
	BlueIris(ch3, m, []common.MetricInfo{smTimeout}, dir)
	close(ch3)
	_ = drainChannel(ch3)
	if timeoutcount != countAfterFirst+1 {
		t.Errorf("after append: timeoutcount: want %v, got %v", countAfterFirst+1, timeoutcount)
	}
}

// TestIntegration_LogRotation verifies that BlueIris() detects when the
// newest file in the directory changes and starts scanning from the new file.
func TestIntegration_LogRotation(t *testing.T) {
	resetState()
	dir := setupIntegrationDir(t)
	dirPath := filepath.FromSlash(strings.TrimSuffix(dir, "/"))

	m := newTestMetric("ai_duration", []string{"camera", "type", "object", "detail"})
	smTimeout := newTestMetric("ai_timeout", []string{})

	// First call – processes integration_test.log.
	ch1 := make(chan prometheus.Metric, 256)
	BlueIris(ch1, m, []common.MetricInfo{smTimeout}, dir)
	close(ch1)
	_ = drainChannel(ch1)
	countAfterFirst := timeoutcount

	// Write a new log file and make it the newest by setting a future mtime.
	newFile := filepath.Join(dirPath, "integration_test_new.log")
	newContent := "level\ttime\tobject\tmessage\r\n" +
		"firstline\r\n" +
		"2 \t1/1/2024 2:00:00.000 AM\tDW                  \tAI: timeout\r\n" +
		"2 \t1/1/2024 2:01:00.000 AM\tDW                  \tAI: timeout\r\n"
	if err := os.WriteFile(newFile, []byte(newContent), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	futureTime := time.Now().Add(time.Hour)
	if err := os.Chtimes(newFile, futureTime, futureTime); err != nil {
		t.Fatalf("Chtimes new file: %v", err)
	}

	// Second call – must detect the new file and scan it from the beginning.
	ch2 := make(chan prometheus.Metric, 256)
	BlueIris(ch2, m, []common.MetricInfo{smTimeout}, dir)
	close(ch2)
	_ = drainChannel(ch2)

	if lastLogFile != "integration_test_new.log" {
		t.Errorf("after rotation: lastLogFile: want integration_test_new.log, got %q", lastLogFile)
	}
	// The new file has two timeout lines on top of the retained global state.
	if timeoutcount != countAfterFirst+2 {
		t.Errorf("after rotation: timeoutcount: want %v, got %v",
			countAfterFirst+2, timeoutcount)
	}
}
