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
	percent    string
	latest     string
}

var latestai = make(map[string]string)

func BlueIris(ch chan<- prometheus.Metric, m common.MetricInfo, SecMet []common.MetricInfo, cameras []string, logpath string) {

	scrapeTime := time.Now()
	aiMetrics := make(map[string]aidata)
	errorMetrics := make(map[string]float64)
	var restartCount float64 = 0

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
		if strings.Contains(scanner.Text(), "AI:") {
			r := regexp.MustCompile(`^.+(?P<camera>DW|BY|FD)(\s*AI:\s)(Alert|\[Objects\])\s(?P<object>[aA-zZ]*|cancelled)(:|\s)(?P<percent>[0-9]*)(%\s\[|\[)(?P<reason>.+).+\s(?P<duration>[0-9]*)ms`)
			if strings.Contains(scanner.Text(), "cancelled") {
				match := r.FindStringSubmatch(scanner.Text())
				cameraMatch := r.SubexpIndex("camera")
				durationMatch := r.SubexpIndex("duration")
				objectMatch := r.SubexpIndex("object")
				reasonMatch := r.SubexpIndex("reason")

				camera := match[cameraMatch]
				duration, err := strconv.ParseFloat(match[durationMatch], 64)
				if err != nil {
					common.BIlogger(fmt.Sprintf("BlueIris - Error parsing duration float. Err: %v", err), "error")
					ch <- prometheus.MustNewConstMetric(m.Errors.WithLabelValues(err.Error()).Desc(), prometheus.CounterValue, 1, "BlueIris")
					continue
				}
				alertcount := aiMetrics[camera+"cancelled"].alertcount
				alertcount++

				aiMetrics[camera+"cancelled"] = aidata{
					camera:     camera,
					duration:   duration,
					object:     match[objectMatch],
					alertcount: alertcount,
					percent:    match[reasonMatch],
					latest:     scanner.Text(),
				}

			} else if strings.Contains(scanner.Text(), "Objects") {
				match := r.FindStringSubmatch(scanner.Text())
				cameraMatch := r.SubexpIndex("camera")
				durationMatch := r.SubexpIndex("duration")
				objectMatch := r.SubexpIndex("object")
				percentMatch := r.SubexpIndex("percent")

				camera := match[cameraMatch]
				duration, err := strconv.ParseFloat(match[durationMatch], 64)
				if err != nil {
					common.BIlogger(fmt.Sprintf("BlueIris - Error parsing duration float. Err: %v", err), "error")
					ch <- prometheus.MustNewConstMetric(m.Errors.WithLabelValues(err.Error()).Desc(), prometheus.CounterValue, 1, "BlueIris")
					continue
				}
				alertcount := aiMetrics[camera+"alert"].alertcount
				alertcount++

				aiMetrics[camera+"alert"] = aidata{
					camera:     camera,
					duration:   duration,
					object:     match[objectMatch],
					alertcount: alertcount,
					percent:    match[percentMatch],
					latest:     scanner.Text(),
				}

			}
		} else if strings.Contains(scanner.Text(), "AI has been restarted") {
			restartCount++
		} else if strings.HasPrefix(scanner.Text(), "2") {
			rError := regexp.MustCompile(`.*\s\s\s(?P<error>.*)`)
			matchError := rError.FindStringSubmatch(scanner.Text())
			ErrorMatch := rError.SubexpIndex("error")
			e := matchError[ErrorMatch]
			if val, ok := errorMetrics[e]; ok {
				val++
				errorMetrics[e] = val
			} else {
				errorMetrics[e] = 1
			}
		}
	}

	for k, a := range aiMetrics {
		if strings.Contains(k, "alert") {
			ch <- prometheus.MustNewConstMetric(m.Desc, m.Type, a.duration, a.camera, "alert", a.object, a.percent)
		} else if strings.Contains(k, "cancelled") {
			ch <- prometheus.MustNewConstMetric(m.Desc, m.Type, a.duration, a.camera, "cancelled", a.object, a.percent)
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
						ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, a.duration, a.camera, "alert", a.object, a.percent)
						latestai[a.camera+"alert"] = a.latest
					}
				} else if strings.Contains(k, "cancelled") {
					if a.latest != latestai[a.camera+"cancelled"] {
						ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, a.duration, a.camera, "cancelled", a.object, a.percent)
						latestai[a.camera+"cancelled"] = a.latest
					}
				}
			}

		case "ai_restarted":
			ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, restartCount)
		case "logerror":
			if len(errorMetrics) == 0 {
				ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, 0, "")
			} else {
				for er, va := range errorMetrics {
					ch <- prometheus.MustNewConstMetric(sm.Desc, sm.Type, va, er)
				}
			}
		}
	}

	ch <- prometheus.MustNewConstMetric(m.Errors.WithLabelValues("BlueIris").Desc(), prometheus.CounterValue, 0, "BlueIris")
	ch <- prometheus.MustNewConstMetric(m.Timer, prometheus.GaugeValue, time.Since(scrapeTime).Seconds(), "BlueIris")

}
