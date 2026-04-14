// +build !cgo

package index

import (
	"log"
)

// JiebaWrapper gojieba 分词器的桩实现（无 CGO 支持时使用）
type JiebaWrapper struct{}

// NewJiebaWrapper 创建分词器（无 CGO 时返回空实现）
func NewJiebaWrapper() *JiebaWrapper {
	log.Println("警告: gojieba 需要 CGO 支持，已降级到简单分词器")
	return &JiebaWrapper{}
}

// Cut 简单分词实现
func (w *JiebaWrapper) Cut(text string, hmm bool) []string {
	return simpleTokenize(text)
}

// Close 空操作
func (w *JiebaWrapper) Close() {}
