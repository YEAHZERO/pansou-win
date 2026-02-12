package util

import (
	"testing"
)

func TestKeywordMatcher(t *testing.T) {
	tests := []struct {
		keyword string
		text    string
		want    bool
		desc    string
	}{
		// 完全匹配
		{
			keyword: "这个江湖因我而存在",
			text:    "这个江湖因我而存在 全集",
			want:    true,
			desc:    "完全匹配",
		},
		// 部分匹配（相似标题）
		{
			keyword: "这个江湖因我而存在",
			text:    "这个江湖因我变得奇怪（80集）",
			want:    true,
			desc:    "部分匹配 - 相似标题",
		},
		// 分词匹配
		{
			keyword: "速度与激情10",
			text:    "速度与激情 第10部 全集",
			want:    true,
			desc:    "分词匹配",
		},
		// 核心词匹配
		{
			keyword: "庆余年第二季",
			text:    "庆余年第二季 全46集",
			want:    true,
			desc:    "核心词匹配",
		},
		// 不匹配
		{
			keyword: "这个江湖因我而存在",
			text:    "那个世界因你而精彩",
			want:    false,
			desc:    "不匹配",
		},
		// 英文匹配
		{
			keyword: "The Matrix",
			text:    "The Matrix Resurrections",
			want:    true,
			desc:    "英文匹配",
		},
		// 混合匹配
		{
			keyword: "复仇者联盟4",
			text:    "复仇者联盟：终局之战",
			want:    true,
			desc:    "混合匹配",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			matcher := NewKeywordMatcher(tt.keyword)
			got := matcher.Match(tt.text)
			if got != tt.want {
				t.Errorf("Match() = %v, want %v\n  keyword: %s\n  text: %s\n  tokens: %v",
					got, tt.want, tt.keyword, tt.text, matcher.tokens)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{
			input: "这个江湖因我而存在",
			desc:  "纯中文",
		},
		{
			input: "速度与激情10",
			desc:  "中文+数字",
		},
		{
			input: "The Matrix Resurrections",
			desc:  "纯英文",
		},
		{
			input: "复仇者联盟4：终局之战",
			desc:  "混合",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			tokens := tokenize(tt.input)
			t.Logf("Input: %s\nTokens: %v", tt.input, tokens)
		})
	}
}

func TestMatchScore(t *testing.T) {
	matcher := NewKeywordMatcher("这个江湖因我而存在")

	tests := []struct {
		text string
		desc string
	}{
		{
			text: "这个江湖因我而存在 全集",
			desc: "完全匹配",
		},
		{
			text: "这个江湖因我变得奇怪（80集）",
			desc: "部分匹配",
		},
		{
			text: "那个世界因你而精彩",
			desc: "不匹配",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			score := matcher.MatchScore(tt.text)
			t.Logf("Text: %s\nScore: %d", tt.text, score)
		})
	}
}
