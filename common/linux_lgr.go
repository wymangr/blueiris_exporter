// +build LINUX

package common

import (
	"fmt"
	"time"
)

func BIlogger(message string, level string) error {
	currentTime := time.Now().Format("2006.01.02 15:04:05")
	fmt.Printf("%v - [%v] - %v\n", currentTime, level, message)
	return nil
}
