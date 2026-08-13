package kiro

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccurateTokenCount_English(t *testing.T) {
	text := "Hello, world! This is a test message."
	count := AccurateTokenCount(text)
	require.Greater(t, count, 0)
	require.Less(t, count, len(text))
}

func TestAccurateTokenCount_Chinese(t *testing.T) {
	text := "你好，世界！这是一个测试消息。"
	count := AccurateTokenCount(text)
	require.Greater(t, count, 0)
	// Chinese characters should result in more tokens than rough estimation
	roughCount := roughTokenCount(text)
	require.Greater(t, count, roughCount)
}

func TestAccurateTokenCount_Mixed(t *testing.T) {
	text := "Hello 你好 world 世界"
	count := AccurateTokenCount(text)
	require.Greater(t, count, 0)
}

func TestAccurateTokenCount_Code(t *testing.T) {
	text := `func main() {
		fmt.Println("Hello, world!")
	}`
	count := AccurateTokenCount(text)
	require.Greater(t, count, 0)
}

func TestAccurateTokenCount_Empty(t *testing.T) {
	require.Equal(t, 0, AccurateTokenCount(""))
	require.Equal(t, 0, AccurateTokenCount("   "))
}

func TestRoughTokenCount_Fallback(t *testing.T) {
	text := "Hello, world!"
	count := roughTokenCount(text)
	require.Greater(t, count, 0)
	// Rough count should be approximately text length / 4
	expected := (len([]rune(text)) + 3) / 4
	require.Equal(t, expected, count)
}
