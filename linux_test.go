//go:build LINUX
// +build LINUX

package main

import (
	"strings"
	"testing"
)

func TestIsService_ReturnsNil(t *testing.T) {
	err := IsService("any-service")
	if err != nil {
		t.Errorf("IsService: expected nil, got %v", err)
	}
}

func TestRemoveService_ReturnsError(t *testing.T) {
	err := removeService("any-service")
	if err == nil {
		t.Error("removeService: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not supprted in Linux") {
		t.Errorf("removeService: unexpected error message: %v", err)
	}
}

func TestStartService_ReturnsError(t *testing.T) {
	err := startService("any-service")
	if err == nil {
		t.Error("startService: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not supprted in Linux") {
		t.Errorf("startService: unexpected error message: %v", err)
	}
}

func TestControlService_Stop_ReturnsError(t *testing.T) {
	err := controlService("any-service", "Stop")
	if err == nil {
		t.Error("controlService: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not supprted in Linux") {
		t.Errorf("controlService: unexpected error message: %v", err)
	}
}

func TestControlService_Pause_ReturnsError(t *testing.T) {
	err := controlService("any-service", "Pause")
	if err == nil {
		t.Error("controlService(Pause): expected error, got nil")
	}
}

func TestInstallService_ReturnsError(t *testing.T) {
	err := installService("any-service", "Any Description", "/logs/", "/metrics", "2112")
	if err == nil {
		t.Error("installService: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not supprted in Linux") {
		t.Errorf("installService: unexpected error message: %v", err)
	}
}
