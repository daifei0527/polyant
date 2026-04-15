// Package model 定义了 Polyant 系统的核心数据模型。
// 包含知识条目、用户、评分、分类、节点信息等结构体定义。
// 本文件补充 models.go 中未定义的类型和常量。
package model

import "time"

// NowMillis 返回当前时间的Unix毫秒时间戳
func NowMillis() int64 {
	return time.Now().UnixMilli()
}

// GetLevelWeight 根据用户层级获取评分权重
func GetLevelWeight(level int32) float64 {
	switch level {
	case 0:
		return 0.0 // Lv0 基础用户
	case 1:
		return 1.0 // Lv1 正式用户
	case 2:
		return 1.2 // Lv2 活跃贡献者
	case 3:
		return 1.5 // Lv3 资深贡献者
	case 4:
		return 2.0 // Lv4 专家贡献者
	case 5:
		return 3.0 // Lv5 核心维护者
	default:
		return 0.0
	}
}

// UserStatusActive 用户正常状态
const UserStatusActive = "active"

// UserStatusBanned 用户封禁状态
const UserStatusBanned = "banned"

// UserStatusSuspended 用户暂停状态
const UserStatusSuspended = "suspended"

// BanType 封禁类型
type BanType string

const (
	BanTypeFull     BanType = "full"     // 完全禁止访问
	BanTypeReadonly BanType = "readonly" // 只读模式
)
