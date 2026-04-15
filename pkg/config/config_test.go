// Package config_test 提供配置管理包的单元测试
package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/daifei0527/polyant/pkg/config"
)

// ==================== DefaultConfig 测试 ====================

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig 返回 nil")
	}

	// 验证节点配置默认值
	if cfg.Node.Type != "local" {
		t.Errorf("默认节点类型 = %q, want 'local'", cfg.Node.Type)
	}
	if cfg.Node.Name == "" {
		t.Error("默认节点名称不应为空")
	}

	// 验证网络配置默认值
	if cfg.Network.ListenPort <= 0 {
		t.Error("默认监听端口应大于 0")
	}
	if cfg.Network.APIPort <= 0 {
		t.Error("默认 API 端口应大于 0")
	}

	// 验证同步配置默认值
	if !cfg.Sync.AutoSync {
		t.Error("默认应启用自动同步")
	}
}

// TestDefaultConfigPortsDifferent 测试默认端口不同
func TestDefaultConfigPortsDifferent(t *testing.T) {
	cfg := config.DefaultConfig()

	if cfg.Network.ListenPort == cfg.Network.APIPort {
		t.Error("监听端口和 API 端口不应相同")
	}
}

// ==================== Validate 测试 ====================

// TestValidateValidConfig 测试有效配置验证
func TestValidateValidConfig(t *testing.T) {
	cfg := config.DefaultConfig()

	if err := config.Validate(cfg); err != nil {
		t.Errorf("验证默认配置失败: %v", err)
	}
}

// TestValidateNilConfig 测试空配置
func TestValidateNilConfig(t *testing.T) {
	err := config.Validate(nil)
	if err == nil {
		t.Error("验证 nil 配置应返回错误")
	}
}

// TestValidateInvalidNodeType 测试无效节点类型
func TestValidateInvalidNodeType(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Node.Type = "invalid"

	err := config.Validate(cfg)
	if err == nil {
		t.Error("验证无效节点类型应返回错误")
	}
}

// TestValidateEmptyNodeName 测试空节点名称
func TestValidateEmptyNodeName(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Node.Name = ""

	err := config.Validate(cfg)
	if err == nil {
		t.Error("验证空节点名称应返回错误")
	}
}

// TestValidateInvalidLogLevel 测试无效日志级别
func TestValidateInvalidLogLevel(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Node.LogLevel = "trace"

	err := config.Validate(cfg)
	if err == nil {
		t.Error("验证无效日志级别应返回错误")
	}
}

// TestValidateInvalidPorts 测试无效端口
func TestValidateInvalidPorts(t *testing.T) {
	tests := []struct {
		name        string
		listenPort  int
		apiPort     int
		shouldError bool
	}{
		{"有效端口", 1000, 2000, false},
		{"端口为0", 0, 2000, true},
		{"端口为负数", -1, 2000, true},
		{"端口超出范围", 70000, 2000, true},
		{"相同端口", 1000, 1000, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Network.ListenPort = tc.listenPort
			cfg.Network.APIPort = tc.apiPort

			err := config.Validate(cfg)
			if tc.shouldError && err == nil {
				t.Error("应返回错误")
			}
			if !tc.shouldError && err != nil {
				t.Errorf("不应返回错误: %v", err)
			}
		})
	}
}

// TestValidateInvalidSyncInterval 测试无效同步间隔
func TestValidateInvalidSyncInterval(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Sync.IntervalSeconds = -1

	err := config.Validate(cfg)
	if err == nil {
		t.Error("验证负数同步间隔应返回错误")
	}
}

// TestValidateInvalidCompression 测试无效压缩算法
func TestValidateInvalidCompression(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Sync.Compression = "invalid"

	err := config.Validate(cfg)
	if err == nil {
		t.Error("验证无效压缩算法应返回错误")
	}
}

// TestValidateSMTPEnabled 测试 SMTP 启用时的验证
func TestValidateSMTPEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.SMTP.Enabled = true
	cfg.SMTP.Host = ""
	cfg.SMTP.Port = 587
	cfg.SMTP.From = ""

	err := config.Validate(cfg)
	if err == nil {
		t.Error("启用 SMTP 但缺少配置应返回错误")
	}

	// 配置完整
	cfg.SMTP.Host = "smtp.example.com"
	cfg.SMTP.From = "noreply@example.com"
	err = config.Validate(cfg)
	if err != nil {
		t.Errorf("完整 SMTP 配置验证失败: %v", err)
	}
}

// ==================== Load 测试 ====================

// TestLoadNonExistent 测试加载不存在的配置文件
func TestLoadNonExistent(t *testing.T) {
	cfg, err := config.Load("/nonexistent/path/config.json")
	if err != nil {
		t.Errorf("加载不存在的文件应返回默认配置: %v", err)
	}
	if cfg == nil {
		t.Error("返回的配置不应为 nil")
	}
}

// TestLoadValidJSON 测试加载有效 JSON
func TestLoadValidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试配置文件
	configPath := filepath.Join(tmpDir, "config.json")
	configContent := `{
		"node": {
			"type": "seed",
			"name": "test-node",
			"data_dir": "/data"
		},
		"network": {
			"listen_port": 9000,
			"api_port": 8080
		}
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if cfg.Node.Type != "seed" {
		t.Errorf("节点类型 = %q, want 'seed'", cfg.Node.Type)
	}
	if cfg.Node.Name != "test-node" {
		t.Errorf("节点名称 = %q, want 'test-node'", cfg.Node.Name)
	}
	if cfg.Network.ListenPort != 9000 {
		t.Errorf("监听端口 = %d, want 9000", cfg.Network.ListenPort)
	}
}

// TestLoadInvalidJSON 测试加载无效 JSON
func TestLoadInvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}

	_, err = config.Load(configPath)
	if err == nil {
		t.Error("加载无效 JSON 应返回错误")
	}
}

// TestLoadPartialJSON 测试加载部分 JSON（使用默认值填充）
func TestLoadPartialJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	configContent := `{
		"node": {
			"type": "local"
		}
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 部分字段应使用默认值
	if cfg.Sync.IntervalSeconds == 0 {
		t.Error("同步间隔应使用默认值")
	}
}

// ==================== Save 测试 ====================

// TestSaveConfig 测试保存配置
func TestSaveConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.DefaultConfig()
	cfg.Node.Name = "save-test-node"

	configPath := filepath.Join(tmpDir, "config.json")

	if err := config.Save(cfg, configPath); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("配置文件未创建")
	}

	// 重新加载验证
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("重新加载配置失败: %v", err)
	}

	if loaded.Node.Name != "save-test-node" {
		t.Errorf("节点名称 = %q, want 'save-test-node'", loaded.Node.Name)
	}
}

// TestSaveNilConfig 测试保存空配置
func TestSaveNilConfig(t *testing.T) {
	err := config.Save(nil, "/tmp/test.json")
	if err == nil {
		t.Error("保存 nil 配置应返回错误")
	}
}

// TestSaveToNestedPath 测试保存到嵌套路径
func TestSaveToNestedPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-test-")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.DefaultConfig()
	nestedPath := filepath.Join(tmpDir, "a", "b", "c", "config.json")

	if err := config.Save(cfg, nestedPath); err != nil {
		t.Fatalf("保存到嵌套路径失败: %v", err)
	}

	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Error("嵌套路径配置文件未创建")
	}
}

// ==================== LoadWithEnv 测试 ====================

// TestLoadWithEnvNodeType 测试环境变量覆盖节点类型
func TestLoadWithEnvNodeType(t *testing.T) {
	os.Setenv("POLYANT_NODE_TYPE", "seed")
	defer os.Unsetenv("POLYANT_NODE_TYPE")

	cfg := config.DefaultConfig()
	cfg = config.LoadWithEnv(cfg)

	if cfg.Node.Type != "seed" {
		t.Errorf("节点类型 = %q, want 'seed'", cfg.Node.Type)
	}
}

// TestLoadWithEnvPorts 测试环境变量覆盖端口
func TestLoadWithEnvPorts(t *testing.T) {
	os.Setenv("POLYANT_NETWORK_LISTEN_PORT", "9999")
	os.Setenv("POLYANT_NETWORK_API_PORT", "8888")
	defer os.Unsetenv("POLYANT_NETWORK_LISTEN_PORT")
	defer os.Unsetenv("POLYANT_NETWORK_API_PORT")

	cfg := config.DefaultConfig()
	cfg = config.LoadWithEnv(cfg)

	if cfg.Network.ListenPort != 9999 {
		t.Errorf("监听端口 = %d, want 9999", cfg.Network.ListenPort)
	}
	if cfg.Network.APIPort != 8888 {
		t.Errorf("API 端口 = %d, want 8888", cfg.Network.APIPort)
	}
}

// TestLoadWithEnvSeedNodes 测试环境变量覆盖种子节点
func TestLoadWithEnvSeedNodes(t *testing.T) {
	os.Setenv("POLYANT_NETWORK_SEED_NODES", "node1:9000,node2:9000")
	defer os.Unsetenv("POLYANT_NETWORK_SEED_NODES")

	cfg := config.DefaultConfig()
	cfg = config.LoadWithEnv(cfg)

	if len(cfg.Network.SeedNodes) != 2 {
		t.Errorf("种子节点数量 = %d, want 2", len(cfg.Network.SeedNodes))
	}
}

// TestLoadWithEnvBool 测试环境变量布尔值解析
func TestLoadWithEnvBool(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"1", true},
		{"yes", true},
		{"on", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"off", false},
	}

	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			os.Setenv("POLYANT_SYNC_AUTO_SYNC", tc.value)
			defer os.Unsetenv("POLYANT_SYNC_AUTO_SYNC")

			cfg := config.DefaultConfig()
			cfg.Sync.AutoSync = !tc.expected // 设置相反值
			cfg = config.LoadWithEnv(cfg)

			if cfg.Sync.AutoSync != tc.expected {
				t.Errorf("AutoSync = %v, want %v", cfg.Sync.AutoSync, tc.expected)
			}
		})
	}
}

// TestLoadWithEnvNil 测试空配置的环境变量加载
func TestLoadWithEnvNil(t *testing.T) {
	os.Setenv("POLYANT_NODE_TYPE", "user")
	defer os.Unsetenv("POLYANT_NODE_TYPE")

	cfg := config.LoadWithEnv(nil)

	if cfg == nil {
		t.Fatal("LoadWithEnv(nil) 返回 nil")
	}
	if cfg.Node.Type != "user" {
		t.Errorf("节点类型 = %q, want 'user'", cfg.Node.Type)
	}
}

// ==================== ToJSON 测试 ====================

// TestConfigToJSON 测试配置序列化为 JSON
func TestConfigToJSON(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Node.Name = "json-test"

	jsonStr, err := cfg.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON 失败: %v", err)
	}

	if jsonStr == "" {
		t.Error("JSON 字符串不应为空")
	}

	// 验证可以解析
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Errorf("JSON 解析失败: %v", err)
	}
}

// ==================== String 测试 ====================

// TestConfigString 测试配置字符串表示
func TestConfigString(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Node.Name = "string-test"

	str := cfg.String()

	if str == "" {
		t.Error("String() 返回空字符串")
	}

	// 应包含 JSON 格式
	if str[0] != '{' {
		t.Error("String() 应返回 JSON 格式")
	}
}

// ==================== parseBool 测试 ====================

// TestParseBool 测试布尔值解析函数
func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"1", true},
		{"yes", true},
		{"YES", true},
		{"on", true},
		{"ON", true},
		{"false", false},
		{"FALSE", false},
		{"0", false},
		{"no", false},
		{"off", false},
		{"", false},
		{"invalid", false},
		{"  true  ", true}, // 带空格
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			// parseBool 是内部函数，通过 LoadWithEnv 测试
			os.Setenv("POLYANT_SYNC_AUTO_SYNC", tc.input)
			defer os.Unsetenv("POLYANT_SYNC_AUTO_SYNC")

			cfg := &config.Config{}
			cfg.Sync.AutoSync = false
			cfg = config.LoadWithEnv(cfg)

			if cfg.Sync.AutoSync != tc.expected {
				t.Errorf("parseBool(%q) = %v, want %v", tc.input, cfg.Sync.AutoSync, tc.expected)
			}
		})
	}
}

// ==================== SeedConfig 测试 ====================

// TestSeedConfigValidation 测试种子节点配置验证
func TestSeedConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.SeedConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid seed config",
			config: &config.SeedConfig{
				Domain:  "seed.example.com",
				TLSCert: "/path/to/cert.pem",
				TLSKey:  "/path/to/key.pem",
			},
			wantErr: false,
		},
		{
			name: "missing domain",
			config: &config.SeedConfig{
				TLSCert: "/path/to/cert.pem",
				TLSKey:  "/path/to/key.pem",
			},
			wantErr: true,
			errMsg:  "domain is required",
		},
		{
			name: "missing tls cert",
			config: &config.SeedConfig{
				Domain: "seed.example.com",
				TLSKey: "/path/to/key.pem",
			},
			wantErr: true,
			errMsg:  "tls_cert is required",
		},
		{
			name: "missing tls key",
			config: &config.SeedConfig{
				Domain:  "seed.example.com",
				TLSCert: "/path/to/cert.pem",
			},
			wantErr: true,
			errMsg:  "tls_key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error message = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestValidateSeedNodeConfig 测试种子节点类型的配置验证
func TestValidateSeedNodeConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *config.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "seed node with valid seed config",
			config: &config.Config{
				Node: config.NodeConfig{
					Type:     "seed",
					Name:     "seed-node-1",
					LogLevel: "info",
				},
				Network: config.NetworkConfig{
					ListenPort: 18530,
					APIPort:    18531,
				},
				Seed: config.SeedConfig{
					Domain:  "seed.example.com",
					TLSCert: "/path/to/cert.pem",
					TLSKey:  "/path/to/key.pem",
				},
			},
			wantErr: false,
		},
		{
			name: "seed node missing domain",
			config: &config.Config{
				Node: config.NodeConfig{
					Type:     "seed",
					Name:     "seed-node-1",
					LogLevel: "info",
				},
				Network: config.NetworkConfig{
					ListenPort: 18530,
					APIPort:    18531,
				},
				Seed: config.SeedConfig{
					TLSCert: "/path/to/cert.pem",
					TLSKey:  "/path/to/key.pem",
				},
			},
			wantErr: true,
			errMsg:  "domain is required",
		},
		{
			name: "local node does not require seed config",
			config: &config.Config{
				Node: config.NodeConfig{
					Type:     "local",
					Name:     "local-node-1",
					LogLevel: "info",
				},
				Network: config.NetworkConfig{
					ListenPort: 18530,
					APIPort:    18531,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.Validate(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("error message = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// containsString 检查字符串是否包含子串
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
