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
	"sync"
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

var lastLogLine string = ""
var lastLogFile string = ""

var (
	timeoutcount        float64                           = 0
	servererrorcount    float64                           = 0
	notrespondingcount  float64                           = 0
	errorMetricsTotal   float64                           = 0
	warningMetricsTotal float64                           = 0
	parseErrorsTotal    float64                           = 0
	restartCount        float64                           = 0
	aiErrorCount        float64                           = 0
	aiRestartingCount   float64                           = 0
	aiRestartedCount    float64                           = 0
	triggerCount        map[string]float64                = make(map[string]float64)
	pushCount           map[string]float64                = make(map[string]float64)
	errorMetrics        map[string]float64                = make(map[string]float64)
	warningMetrics      map[string]float64                = make(map[string]float64)
	parseErrors         map[string]float64                = make(map[string]float64)
	profileCount        map[string]float64                = make(map[string]float64)
	aiMetrics           map[string]aidata                 = make(map[string]aidata)
	diskStats           map[string]map[string]float64     = make(map[string]map[string]float64)
	camerastatus        map[string]map[string]interface{} = make(map[string]map[string]interface{})
	latestai            map[string]string                 = make(map[string]string)
)

var mutex = sync.RWMutex{}

func BlueIris(ch chan<- prometheus.Metric, m common.MetricInfo, SecMet []common.MetricInfo, logpath string) {

	scrapeTime := time.Now()
	mutex.Lock()

	parseErrors = make(map[string]float64)
	startScanning := false

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

	if lastLogFile != "" && lastLogFile != newestFile {
		startScanning = true
	}
	lastLogFile = newestFile

	scanner := bufio.NewScanner(file)
	scanner.Scan()

	for scanner.Scan() {
		if lastLogLine == scanner.Text() || lastLogLine == "" {
			startScanning = true
			if lastLogLine == "" {
				lastLogLine = scanner.Text()
			}
			continue
		}

		if startScanning {
			lastLogLine = scanner.Text()
			match, r, matchType := findObject(scanner.Text())
			if (matchType == "alert") || (matchType == "canceled") {
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
	}

	for k, a := range aiMetrics {
		if strings.Contains(k, "alert") {
			ch <- prometheus.MustNewConstMetric(m.Desc, m.Type, a.duration, a.camera, "alert", a.object, a.detail)
		} else if strings.Contains(k, "canceled") {
			ch <- prometheus.MustNewConstMetric(m.Desc, m.Type, a.duration, a.camera, "canceled", a.object, a.detail)
		}
	}

	for _, sm := range SecMet {
		switch sm.Name {
		case "ai_count":
			for k, a := range aiMetrics {
				if strings.Contains(k, "alert") {
					ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, a.alertcount, a.camera, "alert")
				} else if strings.Contains(k, "canceled") {
					ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, a.alertcount, a.camera, "canceled")
				}
			}
		case "ai_duration_distinct":
			for k, a := range aiMetrics {
				if strings.Contains(k, "alert") {
					if a.latest != latestai[a.camera+"alert"] {
						ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, a.duration, a.camera, "alert", a.object, a.detail)
						latestai[a.camera+"alert"] = a.latest
					}
				} else if strings.Contains(k, "canceled") {
					if a.latest != latestai[a.camera+"canceled"] {
						ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, a.duration, a.camera, "canceled", a.object, a.detail)
						latestai[a.camera+"canceled"] = a.latest
					}
				}
			}
		case "ai_error":
			ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, aiErrorCount)
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
				ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, v["sizePercent"], f)
			}
		case "hours_used":
			for f, v := range diskStats {
				ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, v["hourPercent"], f)
			}
		case "push_notifications":

			for c, v := range pushCount {
				details := strings.Split(c, "|")
				camera := details[0]
				status := details[1]
				detail := details[2]
				ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, v, camera, status, detail)
			}
		case "profile":
			for f, v := range profileCount {
				ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, v, f)
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

	mutex.Unlock()
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

func convertBytes(s string, unit string) (f float64, err error) {
	var bytefl float64
	var errConv error
	var convertedFloat float64
	if strings.Compare(unit, "B") == 0 {
		bytefl, errConv = convertStrFloat(s)
	} else if strings.Compare(unit, "KB") == 0 || strings.Compare(unit, "K") == 0 {
		convertedFloat, errConv = convertStrFloat(s)
		bytefl = convertedFloat * 1000
	} else if strings.Compare(unit, "MB") == 0 || strings.Compare(unit, "M") == 0 {
		convertedFloat, errConv = convertStrFloat(s)
		bytefl = convertedFloat * 1000 * 1000
	} else if strings.Compare(unit, "GB") == 0 || strings.Compare(unit, "G") == 0 {
		convertedFloat, errConv = convertStrFloat(s)
		bytefl = convertedFloat * 1000 * 1000 * 1000
	} else if strings.Compare(unit, "TB") == 0 || strings.Compare(unit, "T") == 0 {
		convertedFloat, errConv = convertStrFloat(s)
		bytefl = convertedFloat * 1000 * 1000 * 1000 * 1000
	} else {
		common.BIlogger(fmt.Sprintf("s %s , Unit %s", s, unit), "console")
		errConv = errors.New("unable to determine sting type (B, K, M, G, T)")
	}
	return bytefl, errConv
}

func findObject(line string) (match []string, r *regexp.Regexp, matchType string) {

	if strings.HasSuffix(line, "AI: timeout") {
		timeoutcount++

	} else if strings.Contains(line, "AI has been restarted") {
		restartCount++

	} else if strings.Contains(line, "AI: error") {
		aiErrorCount++

	} else if strings.Contains(line, "AI: is being started") || strings.Contains(line, "AI is being restarted") {
		aiRestartingCount++

	} else if strings.Contains(line, "AI: has been started") || strings.Contains(line, "AI has been started") {
		aiRestartedCount++

	} else if strings.Contains(line, "DeepStack: Server error") {
		servererrorcount++

	} else if strings.HasSuffix(line, "AI: not responding") {
		notrespondingcount++

	} else if strings.Contains(line, "EXTERNAL") || strings.Contains(line, "MOTION") || strings.Contains(line, "DIO") || strings.Contains(line, "Triggered") || strings.Contains(line, "Re-triggered") {
		r := regexp.MustCompile(`(?P<camera>[^\s\\]*)(\s*(?P<motion>EXTERNAL|MOTION|DIO|Triggered|Re-triggered))`)
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
		r := regexp.MustCompile(`(?P<camera>[^\s\\]*)(\sAI:\s|\sDeepStack:\s)(\[Objects\]\s|Alert\s|\[.+\]\s|)(?P<object>[aA-zZ]*|cancelled|canceled)(\s|:)(\[|)(?P<detail>[0-9]*|.*)(%|\])(\s)(\[.+\]\s|)(?P<duration>[0-9]*)ms`)
		match := r.FindStringSubmatch(newLine)

		if len(match) == 0 {
			r2 := regexp.MustCompile(`(?P<camera>[^\s\\]*)(\sAI:\s|\sDeepStack:\s)(\[Objects\]\s|Alert\s|\[.+\]\s|)(?P<object>[aA-zZ]*|cancelled|canceled)(\s|:)(\[|)(?P<detail>[0-9]*|.*)`)
			match2 := r2.FindStringSubmatch(newLine)
			if len(match2) == 0 {
				parseErrors = appendCounterMap(parseErrors, line)
				parseErrorsTotal++
			}
			return nil, nil, ""
		} else {
			if strings.Contains(newLine, "cancelled") || strings.Contains(newLine, "canceled") {
				return match, r, "canceled"
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
	} else if strings.Contains(line, "Current profile:") {
		r := regexp.MustCompile(`(App)(\s*Current profile:\s)(?P<profile>.+)`)
		match := r.FindStringSubmatch(line)
		if len(match) == 0 {
			parseErrors = appendCounterMap(parseErrors, line)
			parseErrorsTotal++
		} else {
			profileMatch := r.SubexpIndex("profile")
			profile := match[profileMatch]
			for f := range profileCount {
				profileCount[f] = 0
			}
			profileCount[profile] = 1
		}
	} else if strings.Contains(line, "Delete: ") && strings.HasPrefix(line, "0 ") {
		r := regexp.MustCompile(`[APM]{2}\s(?P<folder>.+?)\s+Delete.+(\s|\[)((((?P<hoursused>[0-9]*)\/(?P<hourstotal>[0-9]*))\shrs,(\s(?P<sizeused>[\d\.]+)(?P<sizeunit>\w*)\/(?P<sizelimit>[\d\.]+)(?P<sizelimitunit>\w+)),\s((?P<diskfree>[\d\.]+)(?P<freeunit>\w+))\sfree))`)
		match := r.FindStringSubmatch(line)
		if len(match) == 0 {
			r1 := regexp.MustCompile(`[APM]{2}\s(?P<folder>.+?)\s+Delete.+((((\s|\s\[)(?P<sizeused>\d+|\d+\.\d))(?P<sizeunit>\w*)\/(?P<sizelimit>[\d\.]+)(?P<sizelimitunit>\w+)),\s((?P<diskfree>[\d\.]+)(?P<freeunit>\w+))\sfree)`)
			match1 := r1.FindStringSubmatch(line)
			if len(match1) == 0 {
				r2 := regexp.MustCompile(`[APM]{2}\s(?P<folder>.+?)\s+(?P<ignore>Delete:\s\d+\sitems\s\d.+)`)
				match2 := r2.FindStringSubmatch(line)
				ignore := r2.SubexpIndex("ignore")
				if strings.Compare(match2[ignore], "") == 0 {
					parseErrors = appendCounterMap(parseErrors, line)
					parseErrorsTotal++
				}
				return nil, nil, ""
			}
			folderMatch1 := r1.SubexpIndex("folder")
			sizeusedMatch1 := r1.SubexpIndex("sizeused")
			sizelimitMatch1 := r1.SubexpIndex("sizelimit")
			diskfreeMatch1 := r1.SubexpIndex("diskfree")
			sizeunitMatch1 := r1.SubexpIndex("sizeunit")
			sizelimitunitMatch1 := r1.SubexpIndex("sizelimitunit")
			freeunitMatch1 := r1.SubexpIndex("freeunit")

			folder1 := match1[folderMatch1]

			sizelimit1, err := convertBytes(match1[sizelimitMatch1], match1[sizelimitunitMatch1])
			if err != nil {
				common.BIlogger(fmt.Sprintf("Unable to get convert sizelimit Error: \n%v", err), "console")
				return nil, nil, ""
			}
			diskfree1, err := convertBytes(match1[diskfreeMatch1], match1[freeunitMatch1])
			if err != nil {
				common.BIlogger(fmt.Sprintf("Unable to get convert diskfree Error: \n%v", err), "console")
				return nil, nil, ""
			}

			if strings.Compare(match1[sizeunitMatch1], "") == 0 {
				sizeused1, err := convertBytes(match1[sizeusedMatch1], match1[sizelimitunitMatch1])
				if err != nil {
					fmt.Println(line)
					common.BIlogger(fmt.Sprintf("Unable to get convert sizeused Match1/sizelimitunitMatch1 Error: \n%v", err), "console")
					return nil, nil, ""
				}
				sizePercent1 := (sizeused1 / sizelimit1) * 100
				if _, ok := diskStats[folder1]; !ok {
					diskStats[folder1] = make(map[string]float64)
				}
				diskStats[folder1]["diskfree"] = diskfree1
				diskStats[folder1]["sizePercent"] = sizePercent1
			} else {
				sizeused1, err := convertBytes(match1[sizeusedMatch1], match1[sizeunitMatch1])
				if err != nil {
					fmt.Println(line)
					common.BIlogger(fmt.Sprintf("Unable to get convert sizeused sizeusedMatch1/sizeunitMatch1 Error: \n%v", err), "console")
					return nil, nil, ""
				}
				sizePercent1 := (sizeused1 / sizelimit1) * 100
				if _, ok := diskStats[folder1]; !ok {
					diskStats[folder1] = make(map[string]float64)
				}
				diskStats[folder1]["diskfree"] = diskfree1
				diskStats[folder1]["sizePercent"] = sizePercent1
			}

		} else {
			folderMatch := r.SubexpIndex("folder")
			hoursusedMatch := r.SubexpIndex("hoursused")
			hourstotalMatch := r.SubexpIndex("hourstotal")
			sizeusedMatch := r.SubexpIndex("sizeused")
			sizelimitMatch := r.SubexpIndex("sizelimit")
			diskfreeMatch := r.SubexpIndex("diskfree")
			sizelimitunitMatch := r.SubexpIndex("sizelimitunit")
			sizeunitMatch := r.SubexpIndex("sizeunit")
			freeunitMatch := r.SubexpIndex("freeunit")

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
			sizelimit, err := convertBytes(match[sizelimitMatch], match[sizelimitunitMatch])
			if err != nil {
				common.BIlogger(fmt.Sprintf("Unable to get convert sizelimit Error: \n%v", err), "console")
				return nil, nil, ""
			}
			diskfree, err := convertBytes(match[diskfreeMatch], match[freeunitMatch])
			if err != nil {
				common.BIlogger(fmt.Sprintf("Unable to get convert diskfree Error: \n%v", err), "console")
				return nil, nil, ""
			}

			hourPercent := (hoursused / hourstotal) * 100

			if strings.Compare(match[sizeunitMatch], "") == 0 {
				sizeused, err := convertBytes(match[sizeusedMatch], match[sizelimitunitMatch])
				if err != nil {
					common.BIlogger(fmt.Sprintf("Unable to get convert sizeused sizeusedMatch/sizelimitunitMatch Error: \n%v", err), "console")
					return nil, nil, ""
				}
				sizePercent := (sizeused / sizelimit) * 100

				if _, ok := diskStats[folder]; !ok {
					diskStats[folder] = make(map[string]float64)
				}

				diskStats[folder]["diskfree"] = diskfree
				diskStats[folder]["hourPercent"] = hourPercent
				diskStats[folder]["sizePercent"] = sizePercent
			} else {
				sizeused, err := convertBytes(match[sizeusedMatch], match[sizeunitMatch])
				if err != nil {
					common.BIlogger(fmt.Sprintf("Unable to get convert sizeused sizeusedMatch/sizeunitMatch Error: \n%v", err), "console")
					return nil, nil, ""
				}
				sizePercent := (sizeused / sizelimit) * 100

				if _, ok := diskStats[folder]; !ok {
					diskStats[folder] = make(map[string]float64)
				}

				diskStats[folder]["diskfree"] = diskfree
				diskStats[folder]["hourPercent"] = hourPercent
				diskStats[folder]["sizePercent"] = sizePercent
			}
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
