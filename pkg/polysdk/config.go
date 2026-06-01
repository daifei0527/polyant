// Package polysdk 提供 Polyant API 的 Go SDK 客户端。
// 支持知识条目的搜索、创建、更新、删除和评分等操作。
package polysdk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config Polyant SDK 配置
type Config struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key,omitempty"`
	KeyDir  string `json:"key_dir,omitempty"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		BaseURL: "http://localhost:8080",
		KeyDir:  filepath.Join(homeDir, ".polyant", "keys"),
	}
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	return &config, nil
}

// SaveConfig 保存配置到文件
func SaveConfig(config *Config, path string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// LoadConfigOrDefault 加载配置或返回默认配置
func LoadConfigOrDefault(path string) *Config {
	config, err := LoadConfig(path)
	if err != nil {
		return DefaultConfig()
	}
	return config
}

// NewClientFromConfig 从配置创建客户端
func NewClientFromConfig(config *Config) *Client {
	client := NewClient(config.BaseURL)
	if config.APIKey != "" {
		client.SetAPIKey(config.APIKey)
	}
	return client
}

// DefaultConfigPath 返回默认配置文件路径
func DefaultConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".polyant", "config.json")
}
