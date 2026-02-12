package util

import (
	"strings"
	"unicode"
)

// KeywordMatcher 关键词匹配器
type KeywordMatcher struct {
	keyword      string
	lowerKeyword string
	tokens       []string // 分词后的关键词
}

// NewKeywordMatcher 创建关键词匹配器
func NewKeywordMatcher(keyword string) *KeywordMatcher {
	matcher := &KeywordMatcher{
		keyword:      keyword,
		lowerKeyword: strings.ToLower(keyword),
	}

	// 分词
	matcher.tokens = tokenize(keyword)

	return matcher
}

// Match 检查文本是否匹配关键词
// 使用多种匹配策略：
// 1. 完全匹配（最高优先级）
// 2. 部分匹配（80%以上的词匹配）
// 3. 核心词匹配（去除常见词后匹配）
func (m *KeywordMatcher) Match(text string) bool {
	if text == "" {
		return false
	}

	lowerText := strings.ToLower(text)

	// 策略1：完全匹配
	if strings.Contains(lowerText, m.lowerKeyword) {
		return true
	}

	// 策略2：分词匹配
	// 如果关键词只有1-2个词，要求全部匹配
	// 如果关键词有3个以上的词，要求80%以上匹配
	if len(m.tokens) > 0 {
		matchCount := 0
		for _, token := range m.tokens {
			if strings.Contains(lowerText, strings.ToLower(token)) {
				matchCount++
			}
		}

		// 计算匹配率
		matchRate := float64(matchCount) / float64(len(m.tokens))

		// 根据关键词长度调整匹配阈值
		threshold := 0.8 // 默认80%
		if len(m.tokens) <= 2 {
			threshold = 1.0 // 短关键词要求100%匹配
		} else if len(m.tokens) >= 5 {
			threshold = 0.6 // 长关键词降低到60%
		}

		if matchRate >= threshold {
			return true
		}
	}

	// 策略3：核心词匹配
	// 去除常见词后，检查核心词是否匹配
	coreTokens := filterCommonWords(m.tokens)
	if len(coreTokens) > 0 {
		coreMatchCount := 0
		for _, token := range coreTokens {
			if strings.Contains(lowerText, strings.ToLower(token)) {
				coreMatchCount++
			}
		}

		// 核心词要求至少50%匹配
		if len(coreTokens) > 0 && float64(coreMatchCount)/float64(len(coreTokens)) >= 0.5 {
			return true
		}
	}

	return false
}

// MatchScore 计算匹配分数（0-100）
func (m *KeywordMatcher) MatchScore(text string) int {
	if text == "" {
		return 0
	}

	lowerText := strings.ToLower(text)

	// 完全匹配：100分
	if strings.Contains(lowerText, m.lowerKeyword) {
		return 100
	}

	// 分词匹配：根据匹配率计算分数
	if len(m.tokens) > 0 {
		matchCount := 0
		for _, token := range m.tokens {
			if strings.Contains(lowerText, strings.ToLower(token)) {
				matchCount++
			}
		}

		matchRate := float64(matchCount) / float64(len(m.tokens))
		return int(matchRate * 80) // 最高80分
	}

	return 0
}

// tokenize 中文分词（简单实现）
// 将字符串分割成有意义的词组
func tokenize(text string) []string {
	if text == "" {
		return nil
	}

	var tokens []string
	var currentToken strings.Builder
	var lastType runeType

	for _, r := range text {
		currentType := getRuneType(r)

		// 类型变化时，保存当前token
		if lastType != runeTypeUnknown && currentType != lastType {
			if currentToken.Len() > 0 {
				token := currentToken.String()
				if len(token) > 0 {
					tokens = append(tokens, token)
				}
				currentToken.Reset()
			}
		}

		// 跳过空格和标点
		if currentType != runeTypeSpace && currentType != runeTypePunct {
			currentToken.WriteRune(r)
		}

		lastType = currentType
	}

	// 保存最后一个token
	if currentToken.Len() > 0 {
		token := currentToken.String()
		if len(token) > 0 {
			tokens = append(tokens, token)
		}
	}

	// 对于中文，进一步分割成2-3字的词组
	var finalTokens []string
	for _, token := range tokens {
		if isChinese(token) && len([]rune(token)) > 3 {
			// 分割成2-3字的词组
			runes := []rune(token)
			for i := 0; i < len(runes); {
				// 优先取3字词
				if i+3 <= len(runes) {
					finalTokens = append(finalTokens, string(runes[i:i+3]))
					i += 2 // 重叠分词
				} else if i+2 <= len(runes) {
					finalTokens = append(finalTokens, string(runes[i:i+2]))
					i += 2
				} else {
					i++
				}
			}
		} else {
			finalTokens = append(finalTokens, token)
		}
	}

	return finalTokens
}

// runeType 字符类型
type runeType int

const (
	runeTypeUnknown runeType = iota
	runeTypeChinese
	runeTypeEnglish
	runeTypeNumber
	runeTypeSpace
	runeTypePunct
)

// getRuneType 获取字符类型
func getRuneType(r rune) runeType {
	if unicode.Is(unicode.Han, r) {
		return runeTypeChinese
	}
	if unicode.IsLetter(r) {
		return runeTypeEnglish
	}
	if unicode.IsNumber(r) {
		return runeTypeNumber
	}
	if unicode.IsSpace(r) {
		return runeTypeSpace
	}
	if unicode.IsPunct(r) || unicode.IsSymbol(r) {
		return runeTypePunct
	}
	return runeTypeUnknown
}

// isChinese 检查字符串是否主要是中文
func isChinese(s string) bool {
	chineseCount := 0
	totalCount := 0

	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			chineseCount++
		}
		totalCount++
	}

	return totalCount > 0 && float64(chineseCount)/float64(totalCount) > 0.5
}

// filterCommonWords 过滤常见词
func filterCommonWords(tokens []string) []string {
	commonWords := map[string]bool{
		"的": true, "了": true, "在": true, "是": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
		"都": true, "一": true, "一个": true, "上": true, "也": true,
		"很": true, "到": true, "说": true, "要": true, "去": true,
		"你": true, "会": true, "着": true, "没有": true, "看": true,
		"好": true, "自己": true, "这": true, "那": true, "个": true,
		"们": true, "为": true, "与": true, "及": true, "等": true,
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"of": true, "to": true, "in": true, "for": true, "on": true,
	}

	var filtered []string
	for _, token := range tokens {
		if !commonWords[strings.ToLower(token)] {
			filtered = append(filtered, token)
		}
	}

	return filtered
}
