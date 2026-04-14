// +build cgo

package index

import (
	"github.com/yanyiwu/gojieba"
)

// JiebaWrapper 封装 gojieba 分词器
type JiebaWrapper struct {
	jieba *gojieba.Jieba
}

// NewJiebaWrapper 创建 gojieba 分词器
func NewJiebaWrapper() *JiebaWrapper {
	return &JiebaWrapper{
		jieba: gojieba.NewJieba(),
	}
}

// Cut 分词（精确模式）
func (w *JiebaWrapper) Cut(text string, hmm bool) []string {
	return w.jieba.Cut(text, hmm)
}

// Close 释放资源
func (w *JiebaWrapper) Close() {
	// gojieba 有 finalizer，会自动释放
}
