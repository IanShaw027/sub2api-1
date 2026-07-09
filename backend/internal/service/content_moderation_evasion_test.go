package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMatchBlockedKeyword_FullWidthEvasion 全角/兼容字符经 NFKC 折叠后仍应命中关键词。
func TestMatchBlockedKeyword_FullWidthEvasion(t *testing.T) {
	kw, hit := matchBlockedKeyword("请看这个 ｃｔｆ 挑战", []string{"ctf"}, nil, contentModerationDefaultProximityWindow)
	require.True(t, hit)
	require.Equal(t, "ctf", kw)
}

// TestMatchBlockedKeyword_ZeroWidthEvasion 关键词内插入零宽字符（ZWSP/ZWNJ）应被剔除后命中。
func TestMatchBlockedKeyword_ZeroWidthEvasion(t *testing.T) {
	text := "attack c\u200bt\u200cf here"
	kw, hit := matchBlockedKeyword(text, []string{"ctf"}, nil, contentModerationDefaultProximityWindow)
	require.True(t, hit)
	require.Equal(t, "ctf", kw)
}

// TestMatchBlockedKeyword_ZeroWidthEvasion_AndRule &&组合词各 term 内夹零宽字符也应命中。
func TestMatchBlockedKeyword_ZeroWidthEvasion_AndRule(t *testing.T) {
	text := "这个 c\u200btf 比赛需要 re\u200bverse 工程"
	kw, hit := matchBlockedKeyword(text, []string{"reverse&&ctf"}, nil, contentModerationDefaultProximityWindow)
	require.True(t, hit)
	require.Equal(t, "reverse&&ctf", kw)
}

// TestMatchBlockedKeyword_FullWidthException 例外短语同样按归一化比较，全角例外应豁免命中。
func TestMatchBlockedKeyword_FullWidthException(t *testing.T) {
	_, hit := matchBlockedKeyword("ｃｔｆ", []string{"ctf"}, []string{"ctf"}, contentModerationDefaultProximityWindow)
	require.False(t, hit)
}

// TestMatchBlockedKeyword_ConfusableCyrillicEvasion 同形字（如西里尔 i）应折叠后命中。
func TestMatchBlockedKeyword_ConfusableCyrillicEvasion(t *testing.T) {
	kw, hit := matchBlockedKeyword("please kіll the process", []string{"kill"}, nil, contentModerationDefaultProximityWindow)
	require.True(t, hit)
	require.Equal(t, "kill", kw)
}

// TestMatchBlockedKeyword_SpacedASCIIEvasion 短 ASCII 关键词被空白拆开时仍应命中。
func TestMatchBlockedKeyword_SpacedASCIIEvasion(t *testing.T) {
	kw, hit := matchBlockedKeyword("please k i l l the process", []string{"kill"}, nil, contentModerationDefaultProximityWindow)
	require.True(t, hit)
	require.Equal(t, "kill", kw)
}

// TestMatchBlockedKeyword_SpacedASCIIEvasion_AndRule && 组合词各 term 被空白拆开时也应命中。
func TestMatchBlockedKeyword_SpacedASCIIEvasion_AndRule(t *testing.T) {
	kw, hit := matchBlockedKeyword("这个 r e v e r s e 教程配合 c t f 练习", []string{"reverse&&ctf"}, nil, contentModerationDefaultProximityWindow)
	require.True(t, hit)
	require.Equal(t, "reverse&&ctf", kw)
}

// TestContentModerationInput_KeywordScanCoversFullText 违规词被推到 12000 runes 之后，
// 送审 Text 被截断但 KeywordScanText 覆盖全文，本地关键词仍应命中。
func TestContentModerationInput_KeywordScanCoversFullText(t *testing.T) {
	filler := strings.Repeat("a", maxModerationInputRunes+500)
	in := ContentModerationInput{Text: filler + " ctf"}
	in.Normalize()

	// Text 被截断，扫描截断文本无法命中。
	_, hitTruncated := matchBlockedKeyword(in.Text, []string{"ctf"}, nil, contentModerationDefaultProximityWindow)
	require.False(t, hitTruncated)

	// 全文扫描命中。
	kw, hitFull := matchBlockedKeyword(in.KeywordScanText(), []string{"ctf"}, nil, contentModerationDefaultProximityWindow)
	require.True(t, hitFull)
	require.Equal(t, "ctf", kw)
}

// TestContentModerationInput_KeywordScanBounded 本地扫描全文有上界：超过 maxModerationKeywordScanRunes
// 的填充后再放违规词，会被截断掉、KeywordScanText 不再无界，避免大请求体放大成 DoS。
func TestContentModerationInput_KeywordScanBounded(t *testing.T) {
	filler := strings.Repeat("a", maxModerationKeywordScanRunes+1000)
	in := ContentModerationInput{Text: filler + " ctf"}
	in.Normalize()

	require.LessOrEqual(t, len([]rune(in.KeywordScanText())), maxModerationKeywordScanRunes)
	_, hit := matchBlockedKeyword(in.KeywordScanText(), []string{"ctf"}, nil, contentModerationDefaultProximityWindow)
	require.False(t, hit)
}

// TestContentModerationInput_HashNormalizesZeroWidthEvasion hash/pre-check 与本地关键词匹配应共享零宽字符归一化口径。
func TestContentModerationInput_HashNormalizesZeroWidthEvasion(t *testing.T) {
	base := ContentModerationInput{Text: "attack ctf here"}
	base.Normalize()

	evasion := ContentModerationInput{Text: "attack c\u200bt\u200cf here"}
	evasion.Normalize()

	require.Equal(t, base.Hash(), evasion.Hash())
}

// TestContentModerationInput_HashNormalizesFullWidthEvasion 全角/兼容字符变体与原文应落到同一 hash。
func TestContentModerationInput_HashNormalizesFullWidthEvasion(t *testing.T) {
	base := ContentModerationInput{Text: "please review ctf notes"}
	base.Normalize()

	evasion := ContentModerationInput{Text: "please review ｃｔｆ notes"}
	evasion.Normalize()

	require.Equal(t, base.Hash(), evasion.Hash())
}
