package blueiris

import (
	"bufio"
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

var (
	timeoutcount       float64
	servererrorcount   float64
	notrespondingcount float64
	errorMetricsTotal  float64
	restartCount       float64
	errorMetrics       map[string]float64
)

func BlueIris(ch chan<- prometheus.Metric, m common.MetricInfo, SecMet []common.MetricInfo, logpath string) {

	scrapeTime := time.Now()
	aiMetrics := make(map[string]aidata)
	errorMetrics = make(map[string]float64)
	errorMetricsTotal = 0
	restartCount = 0
	timeoutcount = 0
	servererrorcount = 0
	notrespondingcount = 0

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
		case "ai_restarted":
			ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, restartCount)
		case "ai_timeout":
			ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, timeoutcount)
		case "ai_servererror":
			ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, servererrorcount)
		case "ai_notresponding":
			ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, notrespondingcount)
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
		}
	}

	ch <- prometheus.MustNewConstMetric(m.Errors.WithLabelValues("BlueIris").Desc(), prometheus.CounterValue, 0, "BlueIris")
	ch <- prometheus.MustNewConstMetric(m.Timer, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "BlueIris")

}

func findObject(line string) (match []string, r *regexp.Regexp, matchType string) {

	if strings.HasSuffix(line, "AI: timeout") {
		timeoutcount++
		return nil, nil, ""

	} else if strings.Contains(line, "AI has been restarted") {
		restartCount++
		return nil, nil, ""

	} else if strings.Contains(line, "DeepStack: Server error") {
		servererrorcount++
		return nil, nil, ""

	} else if strings.HasSuffix(line, "AI: not responding") {
		notrespondingcount++
		return nil, nil, ""

	} else if strings.Contains(line, "AI:") || strings.Contains(line, "DeepStack:") {
		newLine := strings.Join(strings.Fields(line), " ")
		r := regexp.MustCompile(`(?P<camera>[^\s\\]*)(\sAI:\s|\sDeepStack:\s)(\[Objects\]\s|Alert\s|\[.+\]\s|)(?P<object>[aA-zZ]*|cancelled)(\s|:)(\[|)(?P<detail>[0-9]*|.*)(%|\])(\s)(\[.+\]\s|)(?P<duration>[0-9]*)ms`)
		match := r.FindStringSubmatch(newLine)

		if len(match) == 0 {
			common.BIlogger(fmt.Sprintf("Unable to parse log line: \n%v", newLine), "console")
			return nil, nil, ""
		} else {
			if strings.Contains(newLine, "cancelled") {
				return match, r, "cancelled"
			} else {
				return match, r, "alert"
			}
		}

	} else if strings.HasPrefix(line, "2") {
		r := regexp.MustCompile(`.*\s\s\s(?P<error>.*)`)
		match := r.FindStringSubmatch(line)
		if len(match) == 0 {
			common.BIlogger(fmt.Sprintf("Unable to parse log line: \n%v", line), "console")
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
			return nil, nil, ""
			// return match, r, "error"

		}
	}
	return nil, nil, ""
}
