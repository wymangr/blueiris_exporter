//go:build !LINUX
// +build !LINUX

package common

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

var elog debug.Log

func BIlogger(message string, level string) error {

	var err error

	currentTime := time.Now().Format("2006.01.02 15:04:05")
	fmt.Printf("%v - [%v] - %v\n", currentTime, level, message)

	if level != "console" {
		elog, err = eventlog.Open("blueiris_exporter")
		if err != nil {
			return err
		}
		defer elog.Close()

		switch level {
		case "info":
			elog.Info(1, message)
		case "error":
			elog.Error(1, message)
		}
	}

	return nil
}
