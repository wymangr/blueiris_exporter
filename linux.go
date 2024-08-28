//go:build LINUX
// +build LINUX

package main

import (
	"errors"
)

func IsService(name string) error {
	return nil
}

func removeService(name string) error {
	err := errors.New("--service.remove is not supprted in Linux!")
	return err
}

func startService(name string) error {
	err := errors.New("--service.start is not supprted in Linux!")
	return err
}

func controlService(name string, command string) error {
	err := errors.New("--service." + command + "is not supprted in Linux!")
	return err
}

func installService(name, desc string, logpath string, metricsPath string, port string) error {
	err := errors.New("--service.install is not supprted in Linux!")
	return err
}
