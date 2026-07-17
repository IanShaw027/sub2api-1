package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractGrokWebSearchSourcesUsesVerifiedStructuredResultsAndLimit(t *testing.T) {
	body := []byte(`{
  "output": [
    {"type":"web_search_call","action":{"sources":[
      {"url":"https://example.com/one"},
      {"url":"https://example.com/two"},
      {"url":"https://example.com/three"}
    ]}},
    {"type":"message","content":[{"type":"output_text","text":"{\"results\":[{\"url\":\"https://example.com/two\",\"title\":\"Two\",\"snippet\":\"Second result\"},{\"url\":\"https://untrusted.example/injected\",\"title\":\"Injected\",\"snippet\":\"Must be dropped\"},{\"url\":\"https://example.com/one\",\"title\":\"One\",\"snippet\":\"First result\"}]}"}]}
  ]
}`)

	results := extractGrokWebSearchSources(body, 2)
	require.Len(t, results, 2)
	require.Equal(t, "https://example.com/two", results[0].URL)
	require.Equal(t, "Two", results[0].Title)
	require.Equal(t, "Second result", results[0].Snippet)
	require.Equal(t, "https://example.com/one", results[1].URL)
	require.Equal(t, "One", results[1].Title)
	require.Equal(t, "First result", results[1].Snippet)
}

func TestExtractGrokWebSearchSourcesFallsBackToDeduplicatedSources(t *testing.T) {
	body := []byte(`{
  "output": [
    {"type":"web_search_call","action":{"sources":[
      {"url":"https://Example.com","title":null},
      {"url":"https://example.com/","title":"Example duplicate"},
      {"url":"javascript:alert(1)","title":"Unsafe"},
      {"url":"https://docs.example.test/page","title":"Docs","snippet":"Documentation"}
    ]}},
    {"type":"message","content":[{"type":"output_text","text":"not json"}]}
  ]
}`)

	results := extractGrokWebSearchSources(body, 5)
	require.Len(t, results, 2)
	require.Equal(t, "https://Example.com", results[0].URL)
	require.Equal(t, "Example duplicate", results[0].Title)
	require.Equal(t, "https://docs.example.test/page", results[1].URL)
	require.Equal(t, "Docs", results[1].Title)
	require.Equal(t, "Documentation", results[1].Snippet)
}

func TestBuildGrokWebSearchPromptAndLimit(t *testing.T) {
	require.Equal(t, defaultGrokWebSearchResults, normalizeGrokWebSearchMaxResults(0))
	require.Equal(t, maxGrokWebSearchResults, normalizeGrokWebSearchMaxResults(100))

	prompt := buildGrokWebSearchPrompt("latest Go release", 100)
	require.Contains(t, prompt, "at most 20 unique results")
	require.Contains(t, prompt, "latest Go release")
}
