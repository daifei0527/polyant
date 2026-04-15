package index

import (
	"strings"
	"unicode"
)

// chineseStopWords 中文停用词列表
var chineseStopWords = map[string]bool{
	"的": true, "了": true, "在": true, "是": true, "我": true,
	"有": true, "和": true, "就": true, "不": true, "人": true,
	"都": true, "一": true, "一个": true, "上": true, "也": true,
	"很": true, "到": true, "说": true, "要": true, "去": true,
	"你": true, "会": true, "着": true, "没有": true, "看": true,
	"好": true, "自己": true, "这": true, "他": true, "她": true,
	"它": true, "们": true, "那": true, "些": true, "什么": true,
	"吗": true, "吧": true, "啊": true, "呢": true, "嗯": true,
	"把": true, "被": true, "让": true, "给": true, "从": true,
	"对": true, "与": true, "为": true, "以": true, "及": true,
	"等": true, "但": true, "而": true, "或": true, "如": true,
	"之": true, "其": true, "中": true, "这个": true, "那个": true,
	"可以": true, "因为": true, "所以": true, "如果": true, "虽然": true,
	"但是": true, "然后": true, "已经": true, "还是": true, "只是": true,
}

// englishStopWords 英文停用词列表
var englishStopWords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true,
	"but": true, "in": true, "on": true, "at": true, "to": true,
	"for": true, "of": true, "with": true, "by": true, "from": true,
	"is": true, "are": true, "was": true, "were": true, "be": true,
	"been": true, "being": true, "have": true, "has": true, "had": true,
	"do": true, "does": true, "did": true, "will": true, "would": true,
	"could": true, "should": true, "may": true, "might": true,
	"can": true, "shall": true, "must": true, "it": true, "its": true,
	"this": true, "that": true, "these": true, "those": true,
	"i": true, "you": true, "he": true, "she": true, "we": true,
	"they": true, "me": true, "him": true, "her": true, "us": true,
	"them": true, "my": true, "your": true, "his": true, "our": true,
	"their": true, "not": true, "no": true, "nor": true, "so": true,
	"if": true, "then": true, "than": true, "too": true, "very": true,
	"just": true, "about": true, "up": true, "out": true, "as": true,
}

// Tokenize 对文本进行分词处理
// 支持中文bigram分词和英文单词分割
// 自动去除停用词和标点符号
func Tokenize(text string) []string {
	if text == "" {
		return nil
	}

	var tokens []string

	// 将文本按字符类型分段处理
	// 分为：连续CJK字符段、连续英文/数字段
	segments := segmentText(text)

	for _, segment := range segments {
		if isCJKSegment(segment) {
			// 中文段：使用bigram分词
			cjkTokens := tokenizeCJK(segment)
			tokens = append(tokens, cjkTokens...)
		} else {
			// 英文/数字段：按空格和标点分割
			engTokens := tokenizeEnglish(segment)
			tokens = append(tokens, engTokens...)
		}
	}

	// 去除停用词并转小写
	tokens = filterStopWords(tokens)

	return tokens
}

// segmentText 将文本按字符类型分段
func segmentText(text string) []string {
	var segments []string
	var current strings.Builder
	prevType := charTypeUnknown

	for _, r := range text {
		ct := getCharType(r)

		if ct == charTypeOther {
			// 标点符号等，结束当前段
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
			prevType = charTypeUnknown
			continue
		}

		if prevType != charTypeUnknown && ct != prevType {
			// 字符类型变化，结束当前段
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
		}

		current.WriteRune(r)
		prevType = ct
	}

	if current.Len() > 0 {
		segments = append(segments, current.String())
	}

	return segments
}

// tokenizeCJK 对中文文本进行bigram分词
// 同时也提取单个字符作为一元组
func tokenizeCJK(text string) []string {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}

	var tokens []string

	// 提取bigram（相邻两个字符的组合）
	for i := 0; i < len(runes)-1; i++ {
		bigram := string(runes[i : i+2])
		tokens = append(tokens, bigram)
	}

	// 如果只有一个字符，也作为token
	if len(runes) == 1 {
		tokens = append(tokens, string(runes[0]))
	}

	return tokens
}

// tokenizeEnglish 对英文文本进行分词
// 按空格分割，去除标点，转小写
func tokenizeEnglish(text string) []string {
	words := strings.Fields(text)
	var tokens []string

	for _, word := range words {
		// 去除首尾标点
		word = strings.TrimFunc(word, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})

		word = strings.ToLower(word)
		if len(word) > 1 {
			tokens = append(tokens, word)
		}
	}

	return tokens
}

// filterStopWords 过滤停用词
func filterStopWords(tokens []string) []string {
	var filtered []string

	for _, token := range tokens {
		// 跳过中文停用词
		if chineseStopWords[token] {
			continue
		}
		// 跳过英文停用词
		if englishStopWords[token] {
			continue
		}
		// 跳过纯数字
		if isAllDigits(token) {
			continue
		}
		// 跳过单字符（除非是CJK字符）
		if len(token) == 1 && !isCJKChar(token) {
			continue
		}

		filtered = append(filtered, token)
	}

	return filtered
}

// ==================== 字符类型判断 ====================

const (
	charTypeUnknown = iota
	charTypeCJK
	charTypeEnglish
	charTypeOther
)

// getCharType 判断字符类型
func getCharType(r rune) int {
	if isCJKRune(r) {
		return charTypeCJK
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return charTypeEnglish
	}
	return charTypeOther
}

// isCJKRune 判断是否为CJK字符
func isCJKRune(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK统一汉字
		(r >= 0x3400 && r <= 0x4DBF) || // CJK扩展A
		(r >= 0x20000 && r <= 0x2A6DF) || // CJK扩展B
		(r >= 0x3000 && r <= 0x303F) || // CJK标点符号
		(r >= 0xFF00 && r <= 0xFFEF) || // 全角字符
		(r >= 0xF900 && r <= 0xFAFF) // CJK兼容汉字
}

// isCJKSegment 判断字符串是否为CJK文本段
func isCJKSegment(s string) bool {
	for _, r := range s {
		if isCJKRune(r) {
			return true
		}
	}
	return false
}

// isCJKChar 判断字符串是否为单个CJK字符
func isCJKChar(s string) bool {
	runes := []rune(s)
	return len(runes) == 1 && isCJKRune(runes[0])
}

// isAllDigits 判断字符串是否全部为数字
func isAllDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
