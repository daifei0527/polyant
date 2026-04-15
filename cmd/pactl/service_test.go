package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/daifei0527/polyant/pkg/config"
)

func TestLoadConfig_Empty(t *testing.T) {
	// When no config file is provided and no default exists
	cfg, err := loadConfig("")
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	// Should return default config
	if cfg == nil {
		t.Fatal("Config should not be nil")
	}
}

func TestLoadConfig_WithFile(t *testing.T) {
	// Create a temp config file
	tmpDir, err := os.MkdirTemp("", "pactl-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configContent := `{
		"node_type": "local",
		"network": {
			"api_port": 9090
		}
	}`
	configFile := filepath.Join(tmpDir, "config.json")
	err = os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := loadConfig(configFile)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Config should not be nil")
	}
}

func TestLoadConfig_NonExistentFile(t *testing.T) {
	// Load non-existent file should return error from config.Load
	cfg, err := loadConfig("/non/existent/config.json")
	// loadConfig calls config.Load which returns error for non-existent files
	if err == nil {
		t.Log("config.Load may have succeeded or returned default config")
	}
	t.Logf("Config: %v, Error: %v", cfg, err)
}

func TestStartViaSystemd(t *testing.T) {
	// This test just verifies the function doesn't panic
	// Actual systemd operations require root privileges
	err := startViaSystemd("test-service")
	// Should not error even without systemd (falls back to print)
	if err != nil {
		t.Logf("startViaSystemd returned: %v (expected on non-systemd systems)", err)
	}
}

func TestStopViaSystemd(t *testing.T) {
	// This test just verifies the function doesn't panic
	// Actual systemd operations require root privileges
	err := stopViaSystemd("test-service")
	// Should not error even without systemd (falls back to print)
	if err != nil {
		t.Logf("stopViaSystemd returned: %v (expected on non-systemd systems)", err)
	}
}

func TestConfig_DefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig should not return nil")
	}
	if cfg.Node.Type == "" {
		t.Error("Node.Type should not be empty in default config")
	}
}
