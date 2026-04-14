// Package config 提供 Polyant 项目的配置管理功能
// 支持从 JSON 文件加载配置、环境变量覆盖和配置验证
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

// ==================== 配置结构体定义 ====================

// NodeConfig 节点配置
type NodeConfig struct {
	Type     string `json:"type"`      // 节点类型: "local" 或 "seed"
	Name     string `json:"name"`      // 节点名称
	DataDir  string `json:"data_dir"`  // 数据存储目录
	LogDir   string `json:"log_dir"`   // 日志存储目录
	LogLevel string `json:"log_level"` // 日志级别: debug, info, warn, error
}

// NetworkConfig 网络配置
type NetworkConfig struct {
	ListenPort int      `json:"listen_port"` // P2P 监听端口
	APIPort    int      `json:"api_port"`    // API 服务端口
	SeedNodes  []string `json:"seed_nodes"`  // 种子节点列表
	DHTEnabled bool     `json:"dht_enabled"` // 是否启用 DHT
	MDNSEnabled bool    `json:"mdns_enabled"` // 是否启用 mDNS 发现
}

// SyncConfig 同步配置
type SyncConfig struct {
	AutoSync         bool     `json:"auto_sync"`          // 是否自动同步
	IntervalSeconds  int      `json:"interval_seconds"`   // 同步间隔（秒）
	MirrorCategories []string `json:"mirror_categories"`  // 镜像的分类列表
	MaxLocalSizeMB   int      `json:"max_local_size_mb"`  // 本地最大存储大小（MB）
	Compression      string   `json:"compression"`        // 压缩算法: gzip, zlib, none
}

// SharingConfig 共享配置
type SharingConfig struct {
	AllowMirror      bool `json:"allow_mirror"`       // 是否允许被镜像
	BandwidthLimitMB int  `json:"bandwidth_limit_mb"` // 带宽限制（MB/s）
	MaxConcurrent    int  `json:"max_concurrent"`     // 最大并发连接数
}

// UserConfig 用户配置
type UserConfig struct {
	PrivateKeyPath string `json:"private_key_path"` // 私钥文件路径
	Email         string `json:"email"`            // 用户邮箱
	AutoRegister  bool   `json:"auto_register"`    // 是否自动注册
}

// SMTPConfig 邮件服务配置
type SMTPConfig struct {
	Enabled  bool   `json:"enabled"`  // 是否启用邮件服务
	Host     string `json:"host"`     // SMTP 服务器地址
	Port     int    `json:"port"`     // SMTP 服务器端口
	Username string `json:"username"` // SMTP 用户名
	Password string `json:"password"` // SMTP 密码
	From     string `json:"from"`     // 发件人地址
}

// APIConfig API 服务配置
type APIConfig struct {
	Enabled bool `json:"enabled"` // 是否启用 API 服务
	CORS    bool `json:"cors"`    // 是否启用 CORS
}

// StorageConfig 存储配置
type StorageConfig struct {
	KVType     string `json:"kv_type"`     // KV 存储类型: pebble, badger
	SearchType string `json:"search_type"` // 搜索引擎类型: bleve, memory
}

// I18nConfig 国际化配置
type I18nConfig struct {
	DefaultLang    string   `json:"default_lang"`    // 默认语言
	AvailableLangs []string `json:"available_langs"` // 可用语言列表
	LogBilingual   bool     `json:"log_bilingual"`   // 日志双语模式
}

// Config 顶层配置结构体
// 包含所有子模块的配置
type Config struct {
	Node    NodeConfig    `json:"node"`
	Network NetworkConfig `json:"network"`
	Sync    SyncConfig    `json:"sync"`
	Sharing SharingConfig `json:"sharing"`
	User    UserConfig    `json:"user"`
	SMTP    SMTPConfig    `json:"smtp"`
	API     APIConfig     `json:"api"`
	Storage StorageConfig `json:"storage"`
	I18n    I18nConfig    `json:"i18n"`
}

// ==================== 配置管理函数 ====================

// DefaultConfig 返回包含所有默认值的配置实例
func DefaultConfig() *Config {
	return &Config{
		Node: NodeConfig{
			Type:     "local",
			Name:     "polyant-node-1",
			DataDir:  "./data",
			LogDir:   "./logs",
			LogLevel: "info",
		},
		Network: NetworkConfig{
			ListenPort:  18530,
			APIPort:     18531,
			SeedNodes:   []string{},
			DHTEnabled:  true,
			MDNSEnabled: true,
		},
		Sync: SyncConfig{
			AutoSync:         true,
			IntervalSeconds:  300,
			MirrorCategories: []string{},
			MaxLocalSizeMB:   1024,
			Compression:      "gzip",
		},
		Sharing: SharingConfig{
			AllowMirror:      true,
			BandwidthLimitMB: 100,
			MaxConcurrent:    10,
		},
		User: UserConfig{
			PrivateKeyPath: "./data/keys",
			Email:          "",
			AutoRegister:   true,
		},
		SMTP: SMTPConfig{
			Enabled:  false,
			Host:     "",
			Port:     587,
			Username: "",
			Password: "",
			From:     "",
		},
		API: APIConfig{
			Enabled: true,
			CORS:    true,
		},
		Storage: StorageConfig{
			KVType:     "pebble",
			SearchType: "bleve",
		},
		I18n: I18nConfig{
			DefaultLang:    "zh-CN",
			AvailableLangs: []string{"zh-CN", "en-US"},
			LogBilingual:   false,
		},
	}
}

// Load 从指定路径加载 JSON 配置文件
// 如果文件不存在或读取失败，返回默认配置
func Load(path string) (*Config, error) {
	// 先创建默认配置作为基础
	cfg := DefaultConfig()

	// 尝试读取配置文件
	data, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// 配置文件不存在，返回默认配置
			return cfg, nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析 JSON 配置
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return cfg, nil
}

// LoadWithEnv 使用环境变量覆盖配置值
// 支持的环境变量前缀为 POLYANT_，使用下划线分隔层级
// 例如: POLYANT_NODE_TYPE, POLYANT_NETWORK_LISTEN_PORT
func LoadWithEnv(config *Config) *Config {
	if config == nil {
		config = DefaultConfig()
	}

	// 节点配置环境变量
	if v := os.Getenv("POLYANT_NODE_TYPE"); v != "" {
		config.Node.Type = v
	}
	if v := os.Getenv("POLYANT_NODE_NAME"); v != "" {
		config.Node.Name = v
	}
	if v := os.Getenv("POLYANT_NODE_DATA_DIR"); v != "" {
		config.Node.DataDir = v
	}
	if v := os.Getenv("POLYANT_NODE_LOG_DIR"); v != "" {
		config.Node.LogDir = v
	}
	if v := os.Getenv("POLYANT_NODE_LOG_LEVEL"); v != "" {
		config.Node.LogLevel = v
	}

	// 网络配置环境变量
	if v := os.Getenv("POLYANT_NETWORK_LISTEN_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			config.Network.ListenPort = port
		}
	}
	if v := os.Getenv("POLYANT_NETWORK_API_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			config.Network.APIPort = port
		}
	}
	if v := os.Getenv("POLYANT_NETWORK_SEED_NODES"); v != "" {
		config.Network.SeedNodes = strings.Split(v, ",")
	}
	if v := os.Getenv("POLYANT_NETWORK_DHT_ENABLED"); v != "" {
		config.Network.DHTEnabled = parseBool(v)
	}
	if v := os.Getenv("POLYANT_NETWORK_MDNS_ENABLED"); v != "" {
		config.Network.MDNSEnabled = parseBool(v)
	}

	// 同步配置环境变量
	if v := os.Getenv("POLYANT_SYNC_AUTO_SYNC"); v != "" {
		config.Sync.AutoSync = parseBool(v)
	}
	if v := os.Getenv("POLYANT_SYNC_INTERVAL_SECONDS"); v != "" {
		if sec, err := strconv.Atoi(v); err == nil {
			config.Sync.IntervalSeconds = sec
		}
	}
	if v := os.Getenv("POLYANT_SYNC_MIRROR_CATEGORIES"); v != "" {
		config.Sync.MirrorCategories = strings.Split(v, ",")
	}
	if v := os.Getenv("POLYANT_SYNC_MAX_LOCAL_SIZE_MB"); v != "" {
		if size, err := strconv.Atoi(v); err == nil {
			config.Sync.MaxLocalSizeMB = size
		}
	}
	if v := os.Getenv("POLYANT_SYNC_COMPRESSION"); v != "" {
		config.Sync.Compression = v
	}

	// 共享配置环境变量
	if v := os.Getenv("POLYANT_SHARING_ALLOW_MIRROR"); v != "" {
		config.Sharing.AllowMirror = parseBool(v)
	}
	if v := os.Getenv("POLYANT_SHARING_BANDWIDTH_LIMIT_MB"); v != "" {
		if limit, err := strconv.Atoi(v); err == nil {
			config.Sharing.BandwidthLimitMB = limit
		}
	}
	if v := os.Getenv("POLYANT_SHARING_MAX_CONCURRENT"); v != "" {
		if max, err := strconv.Atoi(v); err == nil {
			config.Sharing.MaxConcurrent = max
		}
	}

	// 用户配置环境变量
	if v := os.Getenv("POLYANT_USER_PRIVATE_KEY_PATH"); v != "" {
		config.User.PrivateKeyPath = v
	}
	if v := os.Getenv("POLYANT_USER_EMAIL"); v != "" {
		config.User.Email = v
	}
	if v := os.Getenv("POLYANT_USER_AUTO_REGISTER"); v != "" {
		config.User.AutoRegister = parseBool(v)
	}

	// SMTP 配置环境变量
	if v := os.Getenv("POLYANT_SMTP_ENABLED"); v != "" {
		config.SMTP.Enabled = parseBool(v)
	}
	if v := os.Getenv("POLYANT_SMTP_HOST"); v != "" {
		config.SMTP.Host = v
	}
	if v := os.Getenv("POLYANT_SMTP_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			config.SMTP.Port = port
		}
	}
	if v := os.Getenv("POLYANT_SMTP_USERNAME"); v != "" {
		config.SMTP.Username = v
	}
	if v := os.Getenv("POLYANT_SMTP_PASSWORD"); v != "" {
		config.SMTP.Password = v
	}
	if v := os.Getenv("POLYANT_SMTP_FROM"); v != "" {
		config.SMTP.From = v
	}

	// API 配置环境变量
	if v := os.Getenv("POLYANT_API_ENABLED"); v != "" {
		config.API.Enabled = parseBool(v)
	}
	if v := os.Getenv("POLYANT_API_CORS"); v != "" {
		config.API.CORS = parseBool(v)
	}

	// I18n 配置环境变量
	if v := os.Getenv("POLYANT_I18N_DEFAULT_LANG"); v != "" {
		config.I18n.DefaultLang = v
	}
	if v := os.Getenv("POLYANT_I18N_AVAILABLE_LANGS"); v != "" {
		config.I18n.AvailableLangs = strings.Split(v, ",")
	}
	if v := os.Getenv("POLYANT_I18N_LOG_BILINGUAL"); v != "" {
		config.I18n.LogBilingual = parseBool(v)
	}

	return config
}

// Validate 验证配置值的合法性
// 返回第一个发现的验证错误
func Validate(config *Config) error {
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	// 验证节点配置
	validNodeTypes := map[string]bool{"local": true, "seed": true, "user": true}
	if !validNodeTypes[config.Node.Type] {
		return fmt.Errorf("无效的节点类型: %s，必须是 'local'、'seed' 或 'user'", config.Node.Type)
	}
	if config.Node.Name == "" {
		return fmt.Errorf("节点名称不能为空")
	}
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[config.Node.LogLevel] {
		return fmt.Errorf("无效的日志级别: %s，必须是 debug/info/warn/error", config.Node.LogLevel)
	}

	// 验证网络配置
	if config.Network.ListenPort < 1 || config.Network.ListenPort > 65535 {
		return fmt.Errorf("无效的监听端口: %d，必须在 1-65535 之间", config.Network.ListenPort)
	}
	if config.Network.APIPort < 1 || config.Network.APIPort > 65535 {
		return fmt.Errorf("无效的 API 端口: %d，必须在 1-65535 之间", config.Network.APIPort)
	}
	if config.Network.ListenPort == config.Network.APIPort {
		return fmt.Errorf("监听端口和 API 端口不能相同: %d", config.Network.ListenPort)
	}

	// 验证同步配置
	if config.Sync.IntervalSeconds < 0 {
		return fmt.Errorf("同步间隔不能为负数: %d", config.Sync.IntervalSeconds)
	}
	if config.Sync.MaxLocalSizeMB < 0 {
		return fmt.Errorf("本地最大存储大小不能为负数: %d", config.Sync.MaxLocalSizeMB)
	}
	validCompression := map[string]bool{"gzip": true, "zlib": true, "none": true, "": true}
	if !validCompression[config.Sync.Compression] {
		return fmt.Errorf("无效的压缩算法: %s，必须是 gzip/zlib/none", config.Sync.Compression)
	}

	// 验证共享配置
	if config.Sharing.BandwidthLimitMB < 0 {
		return fmt.Errorf("带宽限制不能为负数: %d", config.Sharing.BandwidthLimitMB)
	}
	if config.Sharing.MaxConcurrent < 0 {
		return fmt.Errorf("最大并发数不能为负数: %d", config.Sharing.MaxConcurrent)
	}

	// 验证 SMTP 配置（仅在启用时验证）
	if config.SMTP.Enabled {
		if config.SMTP.Host == "" {
			return fmt.Errorf("SMTP 已启用但未配置主机地址")
		}
		if config.SMTP.Port < 1 || config.SMTP.Port > 65535 {
			return fmt.Errorf("无效的 SMTP 端口: %d", config.SMTP.Port)
		}
		if config.SMTP.From == "" {
			return fmt.Errorf("SMTP 已启用但未配置发件人地址")
		}
	}

	return nil
}

// Save 将配置保存为 JSON 文件
// 会自动创建目标目录
func Save(config *Config, path string) error {
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	// 确保目标目录存在
	dir := path[:strings.LastIndex(path, "/")]
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建配置目录失败: %w", err)
		}
	}

	// 序列化为格式化 JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 写入文件
	if err := ioutil.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// ToJSON 将配置序列化为 JSON 字符串
func (c *Config) ToJSON() (string, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化配置失败: %w", err)
	}
	return string(data), nil
}

// String 实现 Stringer 接口
func (c *Config) String() string {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Sprintf("Config{Node: %s, Network: %+v}", c.Node.Name, c.Network)
	}
	return string(data)
}

// parseBool 将字符串解析为布尔值
// 支持 "true", "1", "yes", "on" 为 true
// 支持 "false", "0", "no", "off" 为 false
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return false
	}
}
