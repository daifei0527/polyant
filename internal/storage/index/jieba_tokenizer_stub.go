// Package index 提供全文搜索功能
//go:build !cgo
// +build !cgo

package index

import (
	"log"
)

// JiebaTokenizer 结巴分词器桩实现（无CGO支持时使用）
type JiebaTokenizer struct {
	initialized bool
}

// NewJiebaTokenizer 创建结巴分词器（无CGO时返回nil）
func NewJiebaTokenizer() *JiebaTokenizer {
	log.Println("警告: gojieba 需要 CGO 支持，已降级到简单分词器")
	return nil
}

// Tokenize 简单分词
func (t *JiebaTokenizer) Tokenize(text string) []string {
	return Tokenize(text)
}

// Close 关闭分词器
func (t *JiebaTokenizer) Close() {}
