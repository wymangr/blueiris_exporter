//go:build LINUX
// +build LINUX

package common

import "testing"

func TestBIlogger_Info(t *testing.T) {
	err := BIlogger("test info message", "info")
	if err != nil {
		t.Errorf("BIlogger info: expected nil, got %v", err)
	}
}

func TestBIlogger_Error(t *testing.T) {
	err := BIlogger("test error message", "error")
	if err != nil {
		t.Errorf("BIlogger error: expected nil, got %v", err)
	}
}

func TestBIlogger_Console(t *testing.T) {
	err := BIlogger("test console message", "console")
	if err != nil {
		t.Errorf("BIlogger console: expected nil, got %v", err)
	}
}

func TestBIlogger_UnknownLevel(t *testing.T) {
	// Unknown levels should not panic and should return nil.
	err := BIlogger("unknown level message", "debug")
	if err != nil {
		t.Errorf("BIlogger unknown level: expected nil, got %v", err)
	}
}
