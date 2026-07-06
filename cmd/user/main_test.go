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
}

func TestLoadConfig_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "test-config.json")

	cfgContent := `{
		"node": {"type": "user", "name": "test-user", "data_dir": "` + tmpDir + `"},
		"network": {"listen_port": 0, "api_port": 8080}
	}`
	err := os.WriteFile(cfgPath, []byte(cfgContent), 0644) //nolint:gosec // test helper file, permissions irrelevant
	require.NoError(t, err)

	*configFile = cfgPath
	defer func() { *configFile = "" }()

	cfg, err := config.Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "test-user", cfg.Node.Name)
}

func TestParseSeedNodes(t *testing.T) {
	nodes := parseSeedNodes("/ip4/1.2.3.4/tcp/9000/p2p/abc,/ip4/5.6.7.8/tcp/9000/p2p/def", nil)
	assert.Len(t, nodes, 2)
	assert.Equal(t, "/ip4/1.2.3.4/tcp/9000/p2p/abc", nodes[0])
}

func TestParseSeedNodes_FromConfig(t *testing.T) {
	configNodes := []string{"/ip4/1.2.3.4/tcp/9000/p2p/abc"}
	nodes := parseSeedNodes("", configNodes)
	assert.Equal(t, configNodes, nodes)
}

func TestVersion(t *testing.T) {
	assert.Equal(t, "1.0.0", Version)
}
