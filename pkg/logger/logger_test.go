package logger

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	// Test with nil config (should use defaults)
	l := NewLogger(nil)
	if l == nil {
		t.Fatal("NewLogger returned nil")
	}
	if l.level != LevelInfo {
		t.Errorf("Default level should be Info, got %d", l.level)
	}
}

func TestNewLogger_WithConfig(t *testing.T) {
	config := &LoggerConfig{
		Level:      LevelDebug,
		FilePath:   "",
		MaxSizeMB:  10,
		MaxBackups: 3,
	}

	l := NewLogger(config)
	if l.level != LevelDebug {
		t.Errorf("Expected level Debug, got %d", l.level)
	}
}

func TestNewLogger_WithFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "logger-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "test.log")

	config := &LoggerConfig{
		Level:      LevelDebug,
		FilePath:   logPath,
		MaxSizeMB:  1,
		MaxBackups: 2,
	}

	l := NewLogger(config)
	defer l.Close()

	if l.file == nil {
		t.Error("File should be opened")
	}

	// Write some log
	l.Info("Test message")

	// Verify file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file should exist")
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	// Capture output using a buffer
	var buf bytes.Buffer

	l := &Logger{
		level:  LevelInfo,
		logger: log.New(&buf, "", 0),
	}

	// Debug should be filtered
	buf.Reset()
	l.Debug("Debug message")
	if buf.Len() > 0 {
		t.Error("Debug message should be filtered at Info level")
	}

	// Info should pass
	buf.Reset()
	l.Info("Info message")
	if !strings.Contains(buf.String(), "Info message") {
		t.Error("Info message should be logged")
	}

	// Warn should pass
	buf.Reset()
	l.Warn("Warn message")
	if !strings.Contains(buf.String(), "Warn message") {
		t.Error("Warn message should be logged")
	}

	// Error should pass
	buf.Reset()
	l.Error("Error message")
	if !strings.Contains(buf.String(), "Error message") {
		t.Error("Error message should be logged")
	}
}

func TestLogger_LevelNames(t *testing.T) {
	tests := []struct {
		level    int
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
	}

	for _, tt := range tests {
		name, ok := levelNames[tt.level]
		if !ok {
			t.Errorf("Level %d not found in levelNames", tt.level)
			continue
		}
		if name != tt.expected {
			t.Errorf("Level %d: expected %s, got %s", tt.level, tt.expected, name)
		}
	}
}

func TestLogger_SetLevel(t *testing.T) {
	l := NewLogger(nil)

	l.SetLevel(LevelDebug)
	if l.GetLevel() != LevelDebug {
		t.Error("Level should be Debug")
	}

	l.SetLevel(LevelError)
	if l.GetLevel() != LevelError {
		t.Error("Level should be Error")
	}
}

func TestLogger_Close(t *testing.T) {
	// Without file
	l := NewLogger(nil)
	err := l.Close()
	if err != nil {
		t.Errorf("Close without file should not error: %v", err)
	}

	// With file
	tmpDir, _ := os.MkdirTemp("", "logger-test-")
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "test.log")
	l = NewLogger(&LoggerConfig{
		Level:    LevelInfo,
		FilePath: logPath,
	})

	err = l.Close()
	if err != nil {
		t.Errorf("Close with file should not error: %v", err)
	}

	if l.file != nil {
		t.Error("File should be nil after Close")
	}
}

func TestLogger_Rotate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "logger-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "test.log")

	config := &LoggerConfig{
		Level:      LevelInfo,
		FilePath:   logPath,
		MaxSizeMB:  0, // No rotation limit
		MaxBackups: 2,
	}

	l := NewLogger(config)
	defer l.Close()

	// With MaxSizeMB = 0, rotate should be no-op
	err = l.rotate()
	if err != nil {
		t.Errorf("Rotate with no size limit should not error: %v", err)
	}
}

func TestLogger_Init(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "logger-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Init global logger
	Init(tmpDir, "debug")

	if globalLogger == nil {
		t.Fatal("globalLogger should be initialized")
	}

	// Test global functions
	Debug("Debug test")
	Info("Info test")
	Warn("Warn test")
	Error("Error test")

	// Close
	Close()
}

func TestLogger_Fatalf(t *testing.T) {
	// Fatalf calls os.Exit(1), so we can't really test it
	// Just verify it doesn't panic with nil logger
	globalLogger = nil
	// Fatalf with nil logger should not panic
	// Note: We can't actually call Fatalf because it exits
}
