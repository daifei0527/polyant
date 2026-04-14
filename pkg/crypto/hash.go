// Package crypto 提供 Polyant 项目的加密工具函数
// 包含哈希计算、验证和 UUID 生成等功能
package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
)

// ComputeSHA256 计算给定数据的 SHA256 哈希值
// 返回小写十六进制编码的哈希字符串
func ComputeSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// VerifySHA256 验证给定数据的 SHA256 哈希值是否与期望的哈希匹配
// hash 参数应为小写十六进制编码的字符串
func VerifySHA256(data []byte, hash string) bool {
	computed := ComputeSHA256(data)
	return computed == hash
}

// ComputeFileSHA256 计算文件的 SHA256 哈希值
// 返回小写十六进制编码的哈希字符串
func ComputeFileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer f.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// GenerateUUID 生成一个基于时间戳和随机数的 UUID v4 格式字符串
// 格式: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
func GenerateUUID() string {
	// 使用 crypto/rand 生成随机字节
	b := make([]byte, 16)
	n, err := rand.Read(b)
	if err != nil || n != 16 {
		// 回退方案：使用时间戳
		return generateFallbackUUID()
	}

	// 设置版本号 (v4) 和变体位
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// generateFallbackUUID 当随机数生成失败时的回退 UUID 生成方案
// 基于时间戳和进程 ID 生成伪 UUID
func generateFallbackUUID() string {
	now := time.Now()
	ts := now.UnixNano()
	pid := os.Getpid()

	// 使用时间戳和 PID 混合生成伪随机数据
	data := fmt.Sprintf("%d-%d-%d", ts, pid, now.UnixNano())

	hash := sha256.Sum256([]byte(data))
	b := hash[:16]

	// 设置版本号和变体位
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
