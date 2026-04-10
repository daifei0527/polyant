// Package linkparser 提供 Markdown 内部链接解析功能
// 用于解析知识条目中的内部链接，支持反向链接索引构建
package linkparser

import (
	"regexp"
	"strings"
)

var (
	// 匹配 [[内部链接]] 格式
	// 支持 [[entry-id|显示文本]] 格式
	wikiLinkRegex = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)

	// 匹配 Markdown 标准链接 [显示文本](entry://entry-id)
	// 用于支持条目标题内部链接
	entrySchemeRegex = regexp.MustCompile(`\[([^\]]+)\]\((entry://([^)]+))\)`)

	// 匹配 Markdown 标准链接 [显示文本](/#/entry/entry-id)
	// 用于 UI 路由格式的内部链接
	hashRouteRegex = regexp.MustCompile(`\[([^\]]+)\]\((/#/entry/([^)]+))\)`)
)

// ParseLinks 从 Markdown 内容中解析所有内部链接条目ID
// 返回去重后的链接条目ID列表
func ParseLinks(content string) []string {
	linkMap := make(map[string]bool)

	// 解析 [[内部链接]] 格式
	matches := wikiLinkRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			entryID := strings.TrimSpace(match[1])
			if entryID != "" {
				linkMap[entryID] = true
			}
		}
	}

	// 解析 entry:// 格式链接
	matches = entrySchemeRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 4 {
			entryID := strings.TrimSpace(match[3])
			if entryID != "" {
				linkMap[entryID] = true
			}
		}
	}

	// 解析 /#/entry/ 格式路由链接
	matches = hashRouteRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 4 {
			entryID := strings.TrimSpace(match[3])
			if entryID != "" {
				linkMap[entryID] = true
			}
		}
	}

	// 转换为切片
	result := make([]string, 0, len(linkMap))
	for entryID := range linkMap {
		result = append(result, entryID)
	}
	return result
}

// ParseLinksWithText 从 Markdown 内容中解析所有内部链接，返回包含显示文本的结果
func ParseLinksWithText(content string) []LinkInfo {
	linkMap := make(map[string]LinkInfo)

	// 解析 [[内部链接]] 格式
	matches := wikiLinkRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			entryID := strings.TrimSpace(match[1])
			if entryID == "" {
				continue
			}
			displayText := entryID
			if len(match) >= 3 && match[2] != "" {
				displayText = strings.TrimSpace(match[2])
			}
			linkMap[entryID] = LinkInfo{
				EntryID:     entryID,
				DisplayText: displayText,
			}
		}
	}

	// 解析 entry:// 格式链接
	matches = entrySchemeRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 4 {
			entryID := strings.TrimSpace(match[3])
			if entryID == "" {
				continue
			}
			displayText := strings.TrimSpace(match[1])
			if _, ok := linkMap[entryID]; !ok {
				linkMap[entryID] = LinkInfo{
					EntryID:     entryID,
					DisplayText: displayText,
				}
			}
		}
	}

	// 解析 /#/entry/ 格式路由链接
	matches = hashRouteRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 4 {
			entryID := strings.TrimSpace(match[3])
			if entryID == "" {
				continue
			}
			displayText := strings.TrimSpace(match[1])
			if _, ok := linkMap[entryID]; !ok {
				linkMap[entryID] = LinkInfo{
					EntryID:     entryID,
					DisplayText: displayText,
				}
			}
		}
	}

	// 转换为切片
	result := make([]LinkInfo, 0, len(linkMap))
	for _, info := range linkMap {
		result = append(result, info)
	}
	return result
}

// LinkInfo 链接信息
type LinkInfo struct {
	EntryID     string // 目标条目ID
	DisplayText string // 显示文本
}
