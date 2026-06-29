package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMatchBlockedKeyword_AndRuleProximityWindow 测试&&组合关键词的邻近窗口限制
func TestMatchBlockedKeyword_AndRuleProximityWindow(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		keywords []string
		want     bool
		wantKW   string
	}{
		{
			name:     "邻近共现_应该命中",
			text:     "这个CTF比赛需要reverse工程技能",
			keywords: []string{"Reverse&&CTF"},
			want:     true,
			wantKW:   "Reverse&&CTF",
		},
		{
			name:     "远距离共现_不应命中",
			text:     buildLongText("Reverse proxy configuration", "CTF competition", 250),
			keywords: []string{"Reverse&&CTF"},
			want:     false,
		},
		{
			name:     "窗口边界_刚好在窗口内_应该命中",
			text:     buildTextWithDistance("reverse", "ctf", 140), // 调整到窗口内
			keywords: []string{"Reverse&&CTF"},
			want:     true,
			wantKW:   "Reverse&&CTF",
		},
		{
			name:     "窗口边界_超过200字符_不应命中",
			text:     buildTextWithDistance("reverse", "ctf", 220),
			keywords: []string{"Reverse&&CTF"},
			want:     false,
		},
		{
			name: "Claude_Code_system_prompt误判场景",
			text: `You are Claude Code, Anthropic's official CLI for Claude. ` +
				strings.Repeat("Additional context and instructions. ", 20) +
				`Reverse the order of operations. ` +
				strings.Repeat("More instructions here. ", 20) +
				`Handle CTF format responses appropriately.`,
			keywords: []string{"Reverse&&CTF"},
			want:     false, // 距离太远,不应命中
		},
		{
			name:     "真实CTF逆向场景_应该命中",
			text:     "这道CTF题目需要reverse engineering来分析二进制文件",
			keywords: []string{"Reverse&&CTF"},
			want:     true,
			wantKW:   "Reverse&&CTF",
		},
		{
			name:     "单纯reverse_proxy_无CTF_不应命中",
			text:     "配置nginx作为reverse proxy用于负载均衡",
			keywords: []string{"Reverse&&CTF"},
			want:     false,
		},
		{
			name:     "多个term都在窗口内_应该命中",
			text:     "账号撞库和密码撞库是常见攻击",
			keywords: []string{"撞库&&账号", "撞库&&密码"},
			want:     true,
			wantKW:   "撞库&&账号",
		},
		{
			name: "第一个term多次出现_找到最近的匹配",
			text: "reverse the order. " +
				strings.Repeat("padding ", 30) +
				"reverse engineering in CTF challenges is common",
			keywords: []string{"Reverse&&CTF"},
			want:     true,
			wantKW:   "Reverse&&CTF",
		},
		{
			name:     "例外短语在窗口内_应该放行",
			text:     "防范勒索病毒攻击很重要",
			keywords: []string{"勒索"},
			want:     false, // 虽然单个词测试,但测例外短语逻辑
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exceptions := []string{"勒索病毒", "勒索软件"}
			kw, hit := matchBlockedKeyword(tt.text, tt.keywords, exceptions)
			require.Equal(t, tt.want, hit, "命中判断不符合预期")
			if tt.want {
				require.Equal(t, tt.wantKW, kw, "命中的关键词不符合预期")
			}
		})
	}
}

// buildLongText 构造两个词距离超过窗口的长文本
func buildLongText(part1, part2 string, distanceChars int) string {
	padding := strings.Repeat("x", distanceChars)
	return part1 + padding + part2
}

// buildTextWithDistance 构造两个词之间有指定距离的文本
func buildTextWithDistance(word1, word2 string, distanceChars int) string {
	if distanceChars < 0 {
		distanceChars = 0
	}
	padding := strings.Repeat(".", distanceChars)
	return word1 + padding + word2
}

// TestMatchBlockedKeyword_ProximityWithMultipleOccurrences 测试多次出现时的窗口匹配
func TestMatchBlockedKeyword_ProximityWithMultipleOccurrences(t *testing.T) {
	// reverse出现3次,只有第3次和CTF在窗口内
	text := "reverse order " +
		strings.Repeat("padding ", 40) + // 超过窗口
		"reverse proxy " +
		strings.Repeat("padding ", 40) + // 超过窗口
		"reverse engineering for CTF challenges"

	kw, hit := matchBlockedKeyword(text, []string{"Reverse&&CTF"}, nil)
	require.True(t, hit, "应该匹配最后一次reverse和CTF的邻近共现")
	require.Equal(t, "Reverse&&CTF", kw)
}

// TestMatchBlockedKeyword_ProximityWithCJK 测试中文&&组合的邻近窗口
func TestMatchBlockedKeyword_ProximityWithCJK(t *testing.T) {
	// 中文也应用窗口限制
	farText := "撞库攻击" + strings.Repeat("是常见的安全威胁。", 30) + "保护账号安全很重要"
	_, hit := matchBlockedKeyword(farText, []string{"撞库&&账号"}, nil)
	require.False(t, hit, "中文词距离太远不应命中")

	nearText := "撞库攻击获取账号信息"
	kw, hit := matchBlockedKeyword(nearText, []string{"撞库&&账号"}, nil)
	require.True(t, hit, "中文词邻近应该命中")
	require.Equal(t, "撞库&&账号", kw)
}

// TestMatchBlockedKeyword_ConfigurableProximityWindow 测试可配置的窗口大小
func TestMatchBlockedKeyword_ConfigurableProximityWindow(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		keywords []string
		want     bool
		wantKW   string
	}{
		{
			name:     "使用默认窗口200_超过则不命中",
			text:     buildTextWithDistance("reverse", "ctf", 180),
			keywords: []string{"Reverse&&CTF"},
			want:     false,
		},
		{
			name:     "配置窗口300_同样距离应命中",
			text:     buildTextWithDistance("reverse", "ctf", 180),
			keywords: []string{"Reverse&&CTF:300"},
			want:     true,
			wantKW:   "Reverse&&CTF:300",
		},
		{
			name:     "配置小窗口50_近距离不命中",
			text:     buildTextWithDistance("reverse", "ctf", 80),
			keywords: []string{"Reverse&&CTF:50"},
			want:     false,
		},
		{
			name:     "配置大窗口500_远距离命中",
			text:     buildTextWithDistance("reverse", "ctf", 350), // 调整距离在75%范围内
			keywords: []string{"Reverse&&CTF:500"},
			want:     true,
			wantKW:   "Reverse&&CTF:500",
		},
		{
			name:     "配置窗口与空格_正确解析",
			text:     buildTextWithDistance("reverse", "ctf", 80),
			keywords: []string{"Reverse && CTF : 150"},
			want:     true,
			wantKW:   "Reverse && CTF : 150",
		},
		{
			name:     "非法窗口配置_导致无法匹配",
			text:     buildTextWithDistance("reverse", "ctf", 120),
			keywords: []string{"Reverse&&CTF:abc"}, // 非法配置,:abc不会被移除,匹配失败
			want:     false,                        // 因为会尝试匹配"ctf:abc"而不是"ctf"
		},
		{
			name:     "超大窗口限制_最大2000",
			text:     buildTextWithDistance("reverse", "ctf", 1500),
			keywords: []string{"Reverse&&CTF:5000"}, // 超过2000会被限制
			want:     false,                         // 因为5000无效,回退到默认200,所以不命中
		},
		{
			name:     "合法大窗口2000_命中",
			text:     buildTextWithDistance("reverse", "ctf", 1400), // 在75%范围内
			keywords: []string{"Reverse&&CTF:2000"},
			want:     true,
			wantKW:   "Reverse&&CTF:2000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kw, hit := matchBlockedKeyword(tt.text, tt.keywords, nil)
			require.Equal(t, tt.want, hit, "命中判断不符合预期")
			if tt.want {
				require.Equal(t, tt.wantKW, kw, "命中的关键词不符合预期")
			}
		})
	}
}

// TestParseProximityWindow 测试窗口大小解析
func TestParseProximityWindow(t *testing.T) {
	tests := []struct {
		keyword string
		want    int
	}{
		{"Reverse&&CTF", 200},         // 默认
		{"Reverse&&CTF:300", 300},     // 自定义
		{"Reverse&&CTF:50", 50},       // 小窗口
		{"Reverse&&CTF:2000", 2000},   // 最大窗口
		{"Reverse&&CTF:5000", 200},    // 超过最大,使用默认
		{"Reverse&&CTF:abc", 200},     // 非法,使用默认
		{"Reverse&&CTF:", 200},        // 空值,使用默认
		{"Reverse&&CTF:0", 200},       // 零值,使用默认
		{"Reverse&&CTF:-100", 200},    // 负值,使用默认
		{"Reverse && CTF : 150", 150}, // 带空格
		{"keyword1&&sk-proj:123", 200},
		{"keyword1&&exploit:5000", 200},
	}

	for _, tt := range tests {
		t.Run(tt.keyword, func(t *testing.T) {
			got := parseProximityWindow(tt.keyword)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSplitBlockedKeywordAndTerms_ProximityWindowAmbiguity(t *testing.T) {
	tests := []struct {
		name    string
		keyword string
		want    []string
	}{
		{
			name:    "colon in second term is not window",
			keyword: "keyword1&&sk-proj:123",
			want:    []string{"keyword1", "sk-proj:123"},
		},
		{
			name:    "out of range numeric suffix is removed but default window applies",
			keyword: "keyword1&&exploit:5000",
			want:    []string{"keyword1", "exploit"},
		},
		{
			name:    "valid numeric suffix is removed from second term",
			keyword: "keyword1&&keyword2:300",
			want:    []string{"keyword1", "keyword2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, splitBlockedKeywordAndTerms(tt.keyword))
		})
	}
}
