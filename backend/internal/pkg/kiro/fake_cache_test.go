package kiro

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildFakeCachePlanRequiresSessionID(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4",
		"messages":[
			{"role":"user","content":"hello"}
		]
	}`)

	plan, err := BuildFakeCachePlan(body, 42, "claude-sonnet-4")
	require.NoError(t, err)
	require.Nil(t, plan)
}

func TestBuildFakeCachePlanBuildsStableScopedKeys(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174000"},
		"system":"You are a helpful assistant.",
		"tools":[{"name":"search","description":"find docs"}],
		"messages":[
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"hi"},
			{"role":"user","content":"please continue"}
		]
	}`)

	plan, err := BuildFakeCachePlan(body, 7, "claude-sonnet-4")
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.NotEmpty(t, plan.CurrentKey)
	require.NotEmpty(t, plan.PreviousKey)
	require.NotEqual(t, plan.CurrentKey, plan.PreviousKey)
	require.Greater(t, plan.CurrentCacheableTokens, plan.PreviousCacheableTokens)
	require.Contains(t, plan.CurrentKey, "acct:7:model:claude-sonnet-4:session:123e4567-e89b-12d3-a456-426614174000")
}

func TestFakeCachePlanResolveUsage(t *testing.T) {
	plan := &FakeCachePlan{
		PreviousCacheableTokens: 60,
		CurrentCacheableTokens:  100,
	}

	miss := plan.ResolveUsage(140, false)
	require.Equal(t, FakeCacheUsage{
		InputTokens:              140,
		CacheCreationInputTokens: 100,
		CacheReadInputTokens:     0,
	}, miss)

	hit := plan.ResolveUsage(140, true)
	require.Equal(t, FakeCacheUsage{
		InputTokens:              80,
		CacheCreationInputTokens: 40,
		CacheReadInputTokens:     60,
	}, hit)
}

func TestFakeCachePlanResolveUsageClampsOversizedCache(t *testing.T) {
	plan := &FakeCachePlan{
		PreviousCacheableTokens: 120,
		CurrentCacheableTokens:  160,
	}

	hit := plan.ResolveUsage(90, true)
	require.Equal(t, FakeCacheUsage{
		InputTokens:              0,
		CacheCreationInputTokens: 0,
		CacheReadInputTokens:     90,
	}, hit)
}
