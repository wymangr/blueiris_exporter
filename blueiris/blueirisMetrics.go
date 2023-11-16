package blueiris

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/wymangr/blueiris_exporter/common"
)

type aidata struct {
	camera     string
	duration   float64
	object     string
	alertcount float64
	detail     string
	latest     string
}

var latestai = make(map[string]string)
var camerastatus = make(map[string]map[string]interface{})
var diskStats = make(map[string]map[string]float64)

// var pushes = make(map[string]map[string]interface{})

var (
	timeoutcount        float64
	servererrorcount    float64
	notrespondingcount  float64
	errorMetricsTotal   float64
	warningMetricsTotal float64
	parseErrorsTotal    float64
	restartCount        float64
	aiRestartingCount   float64
	aiRestartedCount    float64
	triggerCount        map[string]float64
	pushCount           map[string]float64
	errorMetrics        map[string]float64
	warningMetrics      map[string]float64
	parseErrors         map[string]float64
)

func BlueIris(ch chan<- prometheus.Metric, m common.MetricInfo, SecMet []common.MetricInfo, logpath string) {

	scrapeTime := time.Now()
	aiMetrics := make(map[string]aidata)
	errorMetrics = make(map[string]float64)
	warningMetrics = make(map[string]float64)
	triggerCount = make(map[string]float64)
	pushCount = make(map[string]float64)
	errorMetricsTotal = 0
	warningMetricsTotal = 0
	parseErrorsTotal = 0
	restartCount = 0
	aiRestartingCount = 0
	aiRestartedCount = 0
	timeoutcount = 0
	servererrorcount = 0
	notrespondingcount = 0
	parseErrors = make(map[string]float64)

	dir := logpath
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		common.BIlogger(fmt.Sprintf("BlueIris - Error reading blue_iris log directory. Error: %v", err), "error")
		ch <- prometheus.MustNewConstMetric(m.Errors.WithLabelValues("BlueIris").Desc(), prometheus.CounterValue, 1, "BlueIris")
		return
	}
	var newestFile string
	var newestTime int64 = 0
	for _, f := range files {
		fi, err := os.Stat(dir + f.Name())
		if err != nil {
			common.BIlogger(err.Error(), "error")
		}
		currTime := fi.ModTime().Unix()
		if currTime > newestTime {
			newestTime = currTime
			newestFile = f.Name()
		}
	}

	file, err := os.Open(dir + newestFile)
	if err != nil {
		common.BIlogger(fmt.Sprintf("BlueIris - Error opening latest log file. Error: %v", err), "error")
		ch <- prometheus.MustNewConstMetric(m.Errors.WithLabelValues("BlueIris").Desc(), prometheus.CounterValue, 1, "BlueIris")
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {

		match, r, matchType := findObject(scanner.Text())
		if (matchType == "alert") || (matchType == "cancelled") {
			cameraMatch := r.SubexpIndex("camera")
			durationMatch := r.SubexpIndex("duration")
			objectMatch := r.SubexpIndex("object")
			detailMatch := r.SubexpIndex("detail")

			camera := match[cameraMatch]
			duration, err := strconv.ParseFloat(match[durationMatch], 64)
			if err != nil {
				common.BIlogger(fmt.Sprintf("BlueIris - Error parsing duration float. Err: %v", err), "error")
				ch <- prometheus.MustNewConstMetric(m.Errors.WithLabelValues(err.Error()).Desc(), prometheus.CounterValue, 1, "BlueIris")
				continue
			}

			alertcount := aiMetrics[camera+matchType].alertcount
			alertcount++

			makeMap(camera, camerastatus)
			camerastatus[camera]["status"] = 0.0
			camerastatus[camera]["detail"] = "object"
			aiMetrics[camera+matchType] = aidata{
				camera:     camera,
				duration:   duration,
				object:     match[objectMatch],
				alertcount: alertcount,
				detail:     match[detailMatch],
				latest:     scanner.Text(),
			}

		}
	}

	for k, a := range aiMetrics {
		if strings.Contains(k, "alert") {
			ch <- prometheus.MustNewConstMetric(m.Desc, m.Type, a.duration, a.camera, "alert", a.object, a.detail)
		} else if strings.Contains(k, "cancelled") {
			ch <- prometheus.MustNewConstMetric(m.Desc, m.Type, a.duration, a.camera, "cancelled", a.object, a.detail)
		}
	}

	for _, sm := range SecMet {
		switch sm.Name {
		case "ai_count":
			for k, a := range aiMetrics {
				if strings.Contains(k, "alert") {
					ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, a.alertcount, a.camera, "alert")
				} else if strings.Contains(k, "cancelled") {
					ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, a.alertcount, a.camera, "cancelled")
				}
			}
		case "ai_duration_distinct":
			for k, a := range aiMetrics {
				if strings.Contains(k, "alert") {
					if a.latest != latestai[a.camera+"alert"] {
						ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, a.duration, a.camera, "alert", a.object, a.detail)
						latestai[a.camera+"alert"] = a.latest
					}
				} else if strings.Contains(k, "cancelled") {
					if a.latest != latestai[a.camera+"cancelled"] {
						ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, a.duration, a.camera, "cancelled", a.object, a.detail)
						latestai[a.camera+"cancelled"] = a.latest
					}
				}
			}
		case "ai_starting":
			ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, aiRestartingCount)
		case "ai_started":
			ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, aiRestartedCount)
		case "ai_restarted":
			ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, restartCount)
		case "ai_timeout":
			ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, timeoutcount)
		case "ai_servererror":
			ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, servererrorcount)
		case "ai_notresponding":
			ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, notrespondingcount)
		case "triggers":
			for c, v := range triggerCount {
				ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, v, c)
			}
		case "folder_disk_free":
			for f, v := range diskStats {
				ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, v["diskfree"], f)
			}
		case "folder_used":
			for f, v := range diskStats {
				ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, v["hourPercent"], f)
			}
		case "hours_used":
			for f, v := range diskStats {
				ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, v["sizePercent"], f)
			}
		case "push_notifications":

			for c, v := range pushCount {
				details := strings.Split(c, "|")
				camera := details[0]
				status := details[1]
				detail := details[2]
				ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, v, camera, status, detail)
			}

		case "parse_errors":
			if len(parseErrors) == 0 {
				ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, 0, "")
			} else {
				for parse_line, va := range parseErrors {
					ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, va, parse_line)
				}
			}
		case "parse_errors_total":
			ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, parseErrorsTotal)
		case "logerror":
			if len(errorMetrics) == 0 {
				ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, 0, "")
			} else {
				for er, va := range errorMetrics {
					ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, va, er)
				}
			}
		case "logerror_total":
			ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, errorMetricsTotal)
		case "logwarning":
			if len(warningMetrics) == 0 {
				ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, 0, "")
			} else {
				for er, va := range warningMetrics {
					ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, va, er)
				}
			}
		case "logwarning_total":
			ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, warningMetricsTotal)
		case "camera_status":
			status := 1.0
			for k, a := range camerastatus {

				switch i := a["status"].(type) {
				case float64:
					status = float64(i)
					detail, ok := a["detail"].(string)
					if !ok {
						common.BIlogger(fmt.Sprintf("Invalid type for camera_status detail: \n%v", ok), "error")
					}
					ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, status, k, detail)

				default:
					common.BIlogger(fmt.Sprintf("Invalid type for camera_status status: \n%v", i), "error")
				}
			}
		}
	}

	ch <- prometheus.MustNewConstMetric(m.Errors.WithLabelValues("BlueIris").Desc(), prometheus.CounterValue, 0, "BlueIris")
	ch <- prometheus.MustNewConstMetric(m.Timer, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "BlueIris")

}

func makeMap(camera string, m map[string]map[string]interface{}) {
	if _, ok := m[camera]; !ok {
		m[camera] = make(map[string]interface{})
	}
}

func convertStrFloat(s string) (f float64, err error) {

	if s, err := strconv.ParseFloat(s, 64); err == nil {
		return s, err
	} else {
		return 0, err
	}
}

func convertBytes(s string) (f float64, err error) {
	var bytefl float64
	var errConv error
	var convertedFloat float64
	if strings.HasSuffix(s, "B") {
		bytefl, errConv = convertStrFloat(strings.Replace(s, "B", "", 1))
	} else if strings.HasSuffix(s, "K") {
		convertedFloat, errConv = convertStrFloat(strings.Replace(s, "K", "", 1))
		bytefl = convertedFloat * 1024
	} else if strings.HasSuffix(s, "M") {
		convertedFloat, errConv = convertStrFloat(strings.Replace(s, "M", "", 1))
		bytefl = convertedFloat * 1024 * 1024
	} else if strings.HasSuffix(s, "G") {
		convertedFloat, errConv = convertStrFloat(strings.Replace(s, "G", "", 1))
		bytefl = convertedFloat * 1024 * 1024 * 1024
	} else if strings.HasSuffix(s, "T") {
		convertedFloat, errConv = convertStrFloat(strings.Replace(s, "T", "", 1))
		bytefl = convertedFloat * 1024 * 1024 * 1024 * 1024
	} else {
		errConv = errors.New("unable to determine sting type (B, K, M, G, T)")
	}
	return bytefl, errConv
}

func findObject(line string) (match []string, r *regexp.Regexp, matchType string) {

	if strings.HasSuffix(line, "AI: timeout") {
		timeoutcount++

	} else if strings.Contains(line, "AI has been restarted") {
		restartCount++

	} else if strings.Contains(line, "AI: is being started") {
		aiRestartingCount++

	} else if strings.Contains(line, "AI: has been started") {
		aiRestartedCount++

	} else if strings.Contains(line, "DeepStack: Server error") {
		servererrorcount++

	} else if strings.HasSuffix(line, "AI: not responding") {
		notrespondingcount++

	} else if strings.Contains(line, "EXTERNAL") || strings.Contains(line, "MOTION") || strings.Contains(line, "DIO") {
		r := regexp.MustCompile(`(?P<camera>[^\s\\]*)(\s*(?P<motion>EXTERNAL|MOTION|DIO))`)
		match := r.FindStringSubmatch(line)
		if len(match) == 0 {
			parseErrors = appendCounterMap(parseErrors, line)
			parseErrorsTotal++
		} else {
			cameraMatch := r.SubexpIndex("camera")
			camera := match[cameraMatch]
			triggerCount[camera]++
			makeMap(camera, camerastatus)
			camerastatus[camera]["status"] = 0.0
			camerastatus[camera]["detail"] = "trigger"
		}

	} else if strings.Contains(line, "Push:") {
		r := regexp.MustCompile(`(?P<camera>[^\s\\]*)(\s*Push:\s)(?P<status>.+)(\sto\s)(?P<detail>.+)`)
		match := r.FindStringSubmatch(line)
		if len(match) == 0 {
			parseErrors = appendCounterMap(parseErrors, line)
			parseErrorsTotal++
		} else {
			cameraMatch := r.SubexpIndex("camera")
			statusMatch := r.SubexpIndex("status")
			detailMatch := r.SubexpIndex("detail")
			camera := match[cameraMatch]
			status := match[statusMatch]
			detail := match[detailMatch]

			pushCount[camera+"|"+status+"|"+detail]++
		}

	} else if strings.Contains(line, "AI:") || strings.Contains(line, "DeepStack:") {
		newLine := strings.Join(strings.Fields(line), " ")
		r := regexp.MustCompile(`(?P<camera>[^\s\\]*)(\sAI:\s|\sDeepStack:\s)(\[Objects\]\s|Alert\s|\[.+\]\s|)(?P<object>[aA-zZ]*|cancelled)(\s|:)(\[|)(?P<detail>[0-9]*|.*)(%|\])(\s)(\[.+\]\s|)(?P<duration>[0-9]*)ms`)
		match := r.FindStringSubmatch(newLine)

		if len(match) == 0 {
			r2 := regexp.MustCompile(`(?P<camera>[^\s\\]*)(\sAI:\s|\sDeepStack:\s)(\[Objects\]\s|Alert\s|\[.+\]\s|)(?P<object>[aA-zZ]*|cancelled)(\s|:)(\[|)(?P<detail>[0-9]*|.*)`)
			match2 := r2.FindStringSubmatch(newLine)
			if len(match2) == 0 {
				parseErrors = appendCounterMap(parseErrors, line)
				parseErrorsTotal++
			}
			return nil, nil, ""
		} else {
			if strings.Contains(newLine, "cancelled") {
				return match, r, "cancelled"
			} else {
				return match, r, "alert"
			}
		}

	} else if strings.Contains(line, "Signal:") {
		r := regexp.MustCompile(`(?P<camera>[^\s\\]*)(\s*Signal:\s)(?P<status>.+)`)
		match := r.FindStringSubmatch(line)
		if len(match) == 0 {
			parseErrors = appendCounterMap(parseErrors, line)
			parseErrorsTotal++
		} else {
			cameraMatch := r.SubexpIndex("camera")
			statusMatch := r.SubexpIndex("status")

			camera := match[cameraMatch]
			status := match[statusMatch]

			makeMap(camera, camerastatus)

			if strings.Contains(status, "restored") {
				camerastatus[camera]["status"] = 0.0
			} else if strings.HasPrefix(line, "4") {
				camerastatus[camera]["status"] = 0.0
			} else {
				camerastatus[camera]["status"] = 1.0
			}
			camerastatus[camera]["detail"] = status
		}
	} else if strings.Contains(line, "Delete: ") && strings.HasPrefix(line, "0 ") {
		r := regexp.MustCompile(`(?P<folder>[^\s\\]*)(\s*Delete:).+(\[(((?P<hoursused>[0-9]*)\/(?P<hourstotal>[0-9]*))\shrs,(\s(?P<sizeused>.+)\/(?P<sizelimit>.+)),\s(?P<diskfree>.+)\sfree)\])`)
		match := r.FindStringSubmatch(line)
		if len(match) == 0 {
			r1 := regexp.MustCompile(`(?P<folder>[^\s\\]*)(\s*Delete:).+(\[((?P<sizeused>.+)\/(?P<sizelimit>.+)),\s(?P<diskfree>.+)\sfree\])`)
			match1 := r1.FindStringSubmatch(line)
			if len(match1) == 0 {
				parseErrors = appendCounterMap(parseErrors, line)
				parseErrorsTotal++
				return nil, nil, ""
			}
			folderMatch1 := r1.SubexpIndex("folder")
			sizeusedMatch1 := r1.SubexpIndex("sizeused")
			sizelimitMatch1 := r1.SubexpIndex("sizelimit")
			diskfreeMatch1 := r1.SubexpIndex("diskfree")

			folder1 := match1[folderMatch1]
			sizeused1, err := convertBytes(match1[sizeusedMatch1])
			if err != nil {
				common.BIlogger(fmt.Sprintf("Unable to get convert sizeused Error: \n%v", err), "console")
				return nil, nil, ""
			}
			sizelimit1, err := convertBytes(match1[sizelimitMatch1])
			if err != nil {
				common.BIlogger(fmt.Sprintf("Unable to get convert sizelimit Error: \n%v", err), "console")
				return nil, nil, ""
			}
			diskfree1, err := convertBytes(match1[diskfreeMatch1])
			if err != nil {
				common.BIlogger(fmt.Sprintf("Unable to get convert diskfree Error: \n%v", err), "console")
				return nil, nil, ""
			}

			sizePercent1 := (sizeused1 / sizelimit1) * 100

			if _, ok := diskStats[folder1]; !ok {
				diskStats[folder1] = make(map[string]float64)
			}

			diskStats[folder1]["diskfree"] = diskfree1
			diskStats[folder1]["sizePercent"] = sizePercent1

		} else {
			folderMatch := r.SubexpIndex("folder")
			hoursusedMatch := r.SubexpIndex("hoursused")
			hourstotalMatch := r.SubexpIndex("hourstotal")
			sizeusedMatch := r.SubexpIndex("sizeused")
			sizelimitMatch := r.SubexpIndex("sizelimit")
			diskfreeMatch := r.SubexpIndex("diskfree")

			folder := match[folderMatch]
			hoursused, err := convertStrFloat(match[hoursusedMatch])
			if err != nil {
				common.BIlogger(fmt.Sprintf("Unable to convert hoursused string to float: \n%v", hoursused), "console")
				return nil, nil, ""
			}
			hourstotal, err := convertStrFloat(match[hourstotalMatch])
			if err != nil {
				common.BIlogger(fmt.Sprintf("Unable to convert hourstotal string to float: \n%v", hourstotal), "console")
				return nil, nil, ""
			}
			sizeused, err := convertBytes(match[sizeusedMatch])
			if err != nil {
				common.BIlogger(fmt.Sprintf("Unable to get convert sizeused Error: \n%v", err), "console")
				return nil, nil, ""
			}
			sizelimit, err := convertBytes(match[sizelimitMatch])
			if err != nil {
				common.BIlogger(fmt.Sprintf("Unable to get convert sizelimit Error: \n%v", err), "console")
				return nil, nil, ""
			}
			diskfree, err := convertBytes(match[diskfreeMatch])
			if err != nil {
				common.BIlogger(fmt.Sprintf("Unable to get convert diskfree Error: \n%v", err), "console")
				return nil, nil, ""
			}

			hourPercent := (hoursused / hourstotal) * 100
			sizePercent := (sizeused / sizelimit) * 100

			if _, ok := diskStats[folder]; !ok {
				diskStats[folder] = make(map[string]float64)
			}

			diskStats[folder]["diskfree"] = diskfree
			diskStats[folder]["hourPercent"] = hourPercent
			diskStats[folder]["sizePercent"] = sizePercent
		}

	} else if strings.HasPrefix(line, "2") {
		r := regexp.MustCompile(`.*\s\s\s(?P<error>.*)`)
		match := r.FindStringSubmatch(line)
		if len(match) == 0 {
			parseErrors = appendCounterMap(parseErrors, line)
			parseErrorsTotal++
			return nil, nil, ""
		} else {
			ErrorMatch := r.SubexpIndex("error")
			e := match[ErrorMatch]
			errorMetricsTotal++
			if val, ok := errorMetrics[e]; ok {
				val++
				errorMetrics[e] = val
			} else {
				errorMetrics[e] = 1
			}

		}
	} else if strings.HasPrefix(line, "1") && !strings.HasPrefix(line, "10") {
		r := regexp.MustCompile(`.*\s\s\s(?P<warning>.*)`)
		match := r.FindStringSubmatch(line)
		if len(match) == 0 {
			parseErrors = appendCounterMap(parseErrors, line)
			parseErrorsTotal++
			return nil, nil, ""
		} else {
			WarningMatch := r.SubexpIndex("warning")
			e := match[WarningMatch]
			warningMetricsTotal++
			if val, ok := warningMetrics[e]; ok {
				val++
				warningMetrics[e] = val
			} else {
				warningMetrics[e] = 1
			}
		}
	}
	return nil, nil, ""
}

func appendCounterMap(m map[string]float64, key string) map[string]float64 {
	if val, ok := m[key]; ok {
		val++
		m[key] = val
	} else {
		m[key] = 1
	}
	return m
}
