package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/daifei0527/polyant/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Default(t *testing.T) {
	*configFile = ""
	cfg := config.DefaultConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, "local", cfg.Node.Type)
}

func TestLoadConfig_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "test-config.json")

	cfgContent := `{
        "node": {"type": "seed", "name": "test-seed", "data_dir": "` + tmpDir + `"},
        "network": {"listen_port": 9000, "api_port": 8080},
        "seed": {"domain": "test.example.com"}
    }`
	err := os.WriteFile(cfgPath, []byte(cfgContent), 0644)
	require.NoError(t, err)

	*configFile = cfgPath
	defer func() { *configFile = "" }()

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "test-seed", cfg.Node.Name)
}

func TestSeedApp_Validation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Node.Type = "seed"
	cfg.Seed.Domain = ""

	err := cfg.Seed.Validate()
	assert.Error(t, err)
}

func TestVersion(t *testing.T) {
	assert.Equal(t, "1.0.0", Version)
}
