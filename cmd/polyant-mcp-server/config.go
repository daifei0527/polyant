package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config MCP 服务器配置
type Config struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key,omitempty"`
	KeyDir  string `json:"key_dir,omitempty"`
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
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:8080"
	}
	return &config, nil
}
