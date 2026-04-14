// Package logger 提供 Polyant 项目的日志工具
// 支持多级别日志、文件输出和基于大小的日志轮转
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/daifei0527/polyant/pkg/i18n"
)

// 日志级别常量
const (
	LevelDebug = 0 // 调试级别
	LevelInfo  = 1 // 信息级别
	LevelWarn  = 2 // 警告级别
	LevelError = 3 // 错误级别
)

// 日志级别名称映射
var levelNames = map[int]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
}

// LoggerConfig 日志配置结构体
type LoggerConfig struct {
	Level      int    // 日志级别：0=Debug, 1=Info, 2=Warn, 3=Error
	FilePath   string // 日志文件路径，为空则仅输出到 stdout
	MaxSizeMB  int    // 单个日志文件最大大小（MB），0 表示不限制
	MaxBackups int    // 保留的备份日志文件数量，0 表示不保留备份
	// 多语言支持
	Lang      string // 日志语言
	Bilingual bool   // 是否双语模式
}

// Logger 自定义日志结构体
// 支持多级别日志输出和文件轮转
type Logger struct {
	level      int
	filePath   string
	maxSize    int64 // 最大文件大小（字节）
	maxBackups int
	mu         sync.Mutex
	file       *os.File
	fileSize   int64
	logger     *log.Logger
	// 多语言支持
	lang      i18n.Lang
	bilingual bool
}

// NewLogger 根据配置创建新的 Logger 实例
// 如果配置了文件路径，会自动打开文件进行写入
func NewLogger(config *LoggerConfig) *Logger {
	if config == nil {
		config = &LoggerConfig{
			Level: LevelInfo,
		}
	}

	// 解析语言（验证有效性）
	lang := i18n.LangZhCN
	if config.Lang != "" {
		if i18n.IsValidLang(config.Lang) {
			lang = i18n.Lang(config.Lang)
		} else {
			fmt.Fprintf(os.Stderr, "警告: 无效的语言配置 '%s', 使用默认语言 zh-CN\n", config.Lang)
		}
	}

	l := &Logger{
		level:      config.Level,
		filePath:   config.FilePath,
		maxSize:    int64(config.MaxSizeMB) * 1024 * 1024,
		maxBackups: config.MaxBackups,
		lang:       lang,
		bilingual:  config.Bilingual,
	}

	// 如果配置了文件路径，初始化文件输出
	if config.FilePath != "" {
		l.initFile()
	}

	// 设置默认输出目标
	var output io.Writer = os.Stdout
	if l.file != nil {
		output = io.MultiWriter(os.Stdout, l.file)
	}

	l.logger = log.New(output, "", 0)
	return l
}

// initFile 初始化日志文件
// 如果文件已存在，获取当前文件大小用于轮转判断
func (l *Logger) initFile() {
	// 确保目录存在
	dir := filepath.Dir(l.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建日志目录失败: %v\n", err)
		return
	}

	// 打开文件（追加模式）
	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开日志文件失败: %v\n", err)
		return
	}

	// 获取当前文件大小
	info, err := f.Stat()
	if err == nil {
		l.fileSize = info.Size()
	}

	l.file = f
}

// rotate 检查并执行日志文件轮转
// 当文件大小超过限制时，将当前文件重命名为备份文件
func (l *Logger) rotate() error {
	if l.maxSize <= 0 || l.file == nil {
		return nil
	}

	info, err := l.file.Stat()
	if err != nil {
		return err
	}

	if info.Size() < l.maxSize {
		return nil
	}

	// 关闭当前文件
	l.file.Close()

	// 执行备份轮转
	if l.maxBackups > 0 {
		// 删除最旧的备份
		oldestBackup := fmt.Sprintf("%s.%d", l.filePath, l.maxBackups)
		os.Remove(oldestBackup)

		// 将现有备份依次后移
		for i := l.maxBackups - 1; i >= 1; i-- {
			oldName := fmt.Sprintf("%s.%d", l.filePath, i)
			newName := fmt.Sprintf("%s.%d", l.filePath, i+1)
			os.Rename(oldName, newName)
		}

		// 将当前日志文件重命名为第一个备份
		os.Rename(l.filePath, l.filePath+".1")
	}

	// 重新打开日志文件
	f, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("重新打开日志文件失败: %w", err)
	}

	l.file = f
	l.fileSize = 0

	// 更新 logger 的输出目标
	var output io.Writer = os.Stdout
	output = io.MultiWriter(os.Stdout, l.file)
	l.logger = log.New(output, "", 0)

	return nil
}

// log 输出日志的内部方法
// 根据配置的级别过滤日志消息
func (l *Logger) log(level int, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 检查是否需要轮转
	if err := l.rotate(); err != nil {
		fmt.Fprintf(os.Stderr, "日志轮转失败: %v\n", err)
	}

	levelName := "UNKNOWN"
	if name, ok := levelNames[level]; ok {
		levelName = name
	}

	msg := fmt.Sprintf(format, args...)
	l.logger.Printf("[%s] %s", levelName, msg)
}

// Debug 输出调试级别的日志
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

// Info 输出信息级别的日志
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

// Warn 输出警告级别的日志
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(LevelWarn, format, args...)
}

// Error 输出错误级别的日志
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// DebugI18n 输出调试级别的日志（带多语言支持）
func (l *Logger) DebugI18n(code string, args map[string]interface{}) {
	l.logI18n(LevelDebug, code, args)
}

// InfoI18n 输出信息级别的日志（带多语言支持）
func (l *Logger) InfoI18n(code string, args map[string]interface{}) {
	l.logI18n(LevelInfo, code, args)
}

// WarnI18n 输出警告级别的日志（带多语言支持）
func (l *Logger) WarnI18n(code string, args map[string]interface{}) {
	l.logI18n(LevelWarn, code, args)
}

// ErrorI18n 输出错误级别的日志（带多语言支持）
func (l *Logger) ErrorI18n(code string, args map[string]interface{}) {
	l.logI18n(LevelError, code, args)
}

// logI18n 内部日志方法（带多语言支持）
func (l *Logger) logI18n(level int, code string, args map[string]interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.rotate(); err != nil {
		fmt.Fprintf(os.Stderr, "日志轮转失败: %v\n", err)
	}

	levelName := "UNKNOWN"
	if name, ok := levelNames[level]; ok {
		levelName = name
	}

	if l.bilingual {
		msgZh := i18n.Tc(i18n.LangZhCN, code, args)
		msgEn := i18n.Tc(i18n.LangEnUS, code, args)
		l.logger.Printf("[%s] %s | %s", levelName, msgZh, msgEn)
	} else {
		msg := i18n.Tc(l.lang, code, args)
		l.logger.Printf("[%s] %s", levelName, msg)
	}
}

// Close 关闭日志文件句柄，释放资源
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

// SetLevel 动态设置日志级别
func (l *Logger) SetLevel(level int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel 获取当前日志级别
func (l *Logger) GetLevel() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}

// SetLang 动态设置日志语言
func (l *Logger) SetLang(lang i18n.Lang) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lang = lang
}

// GetLang 获取当前日志语言
func (l *Logger) GetLang() i18n.Lang {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lang
}

// SetBilingual 动态设置双语模式
func (l *Logger) SetBilingual(bilingual bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.bilingual = bilingual
}

// IsBilingual 检查是否为双语模式
func (l *Logger) IsBilingual() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.bilingual
}

// ==================== 全局日志实例 ====================

var (
	// globalLogger 全局日志实例
	globalLogger *Logger
)

// Init 初始化全局日志实例
// dir: 日志目录
// level: 日志级别（debug, info, warn, error）
func Init(dir, level string) {
	// 解析日志级别
	levelMap := map[string]int{
		"debug": LevelDebug,
		"info":  LevelInfo,
		"warn":  LevelWarn,
		"error": LevelError,
	}

	logLevel, ok := levelMap[level]
	if !ok {
		logLevel = LevelInfo
	}

	// 创建日志文件路径
	filePath := filepath.Join(dir, "polyant.log")

	// 创建日志配置
	config := &LoggerConfig{
		Level:      logLevel,
		FilePath:   filePath,
		MaxSizeMB:  100,  // 100MB 轮转
		MaxBackups: 5,     // 保留5个备份
	}

	// 初始化全局日志
	globalLogger = NewLogger(config)

	// 输出初始化信息
	globalLogger.Info("日志系统初始化完成，级别: %s, 文件: %s", level, filePath)
}

// InitWithConfig 使用完整配置初始化全局日志实例
func InitWithConfig(dir, level, lang string, bilingual bool) {
	// 解析日志级别
	levelMap := map[string]int{
		"debug": LevelDebug,
		"info":  LevelInfo,
		"warn":  LevelWarn,
		"error": LevelError,
	}

	logLevel, ok := levelMap[level]
	if !ok {
		logLevel = LevelInfo
	}

	// 创建日志文件路径
	filePath := filepath.Join(dir, "polyant.log")

	// 创建日志配置
	config := &LoggerConfig{
		Level:      logLevel,
		FilePath:   filePath,
		MaxSizeMB:  100,  // 100MB 轮转
		MaxBackups: 5,     // 保留5个备份
		Lang:       lang,
		Bilingual:  bilingual,
	}

	// 初始化全局日志
	globalLogger = NewLogger(config)

	// 输出初始化信息
	globalLogger.Info("日志系统初始化完成，级别: %s, 语言: %s, 文件: %s", level, lang, filePath)
}

// Close 关闭全局日志实例
func Close() error {
	if globalLogger != nil {
		return globalLogger.Close()
	}
	return nil
}

// Debug 输出调试级别的全局日志
func Debug(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Debug(format, args...)
	}
}

// Info 输出信息级别的全局日志
func Info(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Info(format, args...)
	}
}

// Warn 输出警告级别的全局日志
func Warn(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Warn(format, args...)
	}
}

// Error 输出错误级别的全局日志
func Error(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Error(format, args...)
	}
}

// DebugI18n 输出调试级别的全局日志（带多语言支持）
func DebugI18n(code string, args map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.DebugI18n(code, args)
	}
}

// InfoI18n 输出信息级别的全局日志（带多语言支持）
func InfoI18n(code string, args map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.InfoI18n(code, args)
	}
}

// WarnI18n 输出警告级别的全局日志（带多语言支持）
func WarnI18n(code string, args map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.WarnI18n(code, args)
	}
}

// ErrorI18n 输出错误级别的全局日志（带多语言支持）
func ErrorI18n(code string, args map[string]interface{}) {
	if globalLogger != nil {
		globalLogger.ErrorI18n(code, args)
	}
}

// Fatal 输出错误级别的全局日志并退出程序
func Fatalf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Error(format, args...)
	}
	os.Exit(1)
}
