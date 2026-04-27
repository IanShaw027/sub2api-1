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
	require.NotEmpty(t, plan.IndependentKey)
	require.NotEmpty(t, plan.CurrentPrefixKey)
	require.NotEmpty(t, plan.PreviousPrefixKey)
	require.NotEqual(t, plan.CurrentKey, plan.PreviousKey)
	require.Greater(t, plan.CurrentCacheableTokens, plan.PreviousCacheableTokens)
	require.Contains(t, plan.CurrentKey, "acct:7:model:claude-sonnet-4:session:123e4567-e89b-12d3-a456-426614174000")
}

func TestBuildFakeCachePlanAcceptsJSONMetadataUserID(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"{\"device_id\":\"device\",\"account_uuid\":\"\",\"session_id\":\"123e4567-e89b-12d3-a456-426614174000\"}"},
		"messages":[
			{"role":"user","content":"hello"}
		]
	}`)

	plan, err := BuildFakeCachePlan(body, 7, "claude-sonnet-4")
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Contains(t, plan.CurrentKey, "session:123e4567-e89b-12d3-a456-426614174000")
}

func TestBuildFakeCachePlanBindsPrefixKeysToIndependentChain(t *testing.T) {
	base := requireFakeCachePlan(t, `{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174001"},
		"system":"Use concise answers.",
		"tools":[{
			"name":"search",
			"description":"find docs",
			"input_schema":{"type":"object","properties":{"query":{"type":"string"}}}
		}],
		"messages":[
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"hi"},
			{"role":"user","content":"please continue"}
		]
	}`)

	systemChanged := requireFakeCachePlan(t, `{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174001"},
		"system":"Use detailed answers.",
		"tools":[{
			"name":"search",
			"description":"find docs",
			"input_schema":{"type":"object","properties":{"query":{"type":"string"}}}
		}],
		"messages":[
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"hi"},
			{"role":"user","content":"please continue"}
		]
	}`)

	toolChanged := requireFakeCachePlan(t, `{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174001"},
		"system":"Use concise answers.",
		"tools":[{
			"name":"search",
			"description":"find source docs",
			"input_schema":{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer"}}}
		}],
		"messages":[
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"hi"},
			{"role":"user","content":"please continue"}
		]
	}`)

	require.NotEqual(t, base.IndependentKey, systemChanged.IndependentKey)
	require.NotEqual(t, base.CurrentPrefixKey, systemChanged.CurrentPrefixKey)
	require.NotEqual(t, base.PreviousPrefixKey, systemChanged.PreviousPrefixKey)
	require.NotEqual(t, base.IndependentKey, toolChanged.IndependentKey)
	require.NotEqual(t, base.CurrentPrefixKey, toolChanged.CurrentPrefixKey)
	require.NotEqual(t, base.PreviousPrefixKey, toolChanged.PreviousPrefixKey)
}

func TestBuildFakeCachePlanPrefixKeysIncludeAssistantAndToolHistory(t *testing.T) {
	base := requireFakeCachePlan(t, `{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174002"},
		"messages":[
			{"role":"user","content":"find the docs"},
			{"role":"assistant","content":[
				{"type":"text","text":"I will search."},
				{"type":"tool_use","id":"toolu_1","name":"search","input":{"query":"alpha"}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"alpha result"}]}
			]},
			{"role":"assistant","content":"alpha summary"},
			{"role":"user","content":"continue"}
		]
	}`)

	cases := map[string]string{
		"assistant_text": `{
			"model":"claude-sonnet-4",
			"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174002"},
			"messages":[
				{"role":"user","content":"find the docs"},
				{"role":"assistant","content":[
					{"type":"text","text":"I will inspect the cache."},
					{"type":"tool_use","id":"toolu_1","name":"search","input":{"query":"alpha"}}
				]},
				{"role":"user","content":[
					{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"alpha result"}]}
				]},
				{"role":"assistant","content":"alpha summary"},
				{"role":"user","content":"continue"}
			]
		}`,
		"assistant_tool_use": `{
			"model":"claude-sonnet-4",
			"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174002"},
			"messages":[
				{"role":"user","content":"find the docs"},
				{"role":"assistant","content":[
					{"type":"text","text":"I will search."},
					{"type":"tool_use","id":"toolu_1","name":"search","input":{"query":"beta"}}
				]},
				{"role":"user","content":[
					{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"alpha result"}]}
				]},
				{"role":"assistant","content":"alpha summary"},
				{"role":"user","content":"continue"}
			]
		}`,
		"user_tool_result": `{
			"model":"claude-sonnet-4",
			"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174002"},
			"messages":[
				{"role":"user","content":"find the docs"},
				{"role":"assistant","content":[
					{"type":"text","text":"I will search."},
					{"type":"tool_use","id":"toolu_1","name":"search","input":{"query":"alpha"}}
				]},
				{"role":"user","content":[
					{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"beta result"}]}
				]},
				{"role":"assistant","content":"alpha summary"},
				{"role":"user","content":"continue"}
			]
		}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			changed := requireFakeCachePlan(t, body)
			require.NotEqual(t, base.CurrentPrefixKey, changed.CurrentPrefixKey)
			require.NotEqual(t, base.PreviousPrefixKey, changed.PreviousPrefixKey)
		})
	}
}

func TestFakeCachePlanResolveUsage(t *testing.T) {
	plan := &FakeCachePlan{
		PreviousCacheableTokens: 60,
		CurrentCacheableTokens:  100,
	}

	miss := plan.ResolveUsage(140, false)
	require.Equal(t, FakeCacheUsage{
		InputTokens:              40,
		CacheCreationInputTokens: 100,
		CacheReadInputTokens:     0,
	}, miss)

	hit := plan.ResolveUsage(140, true)
	require.Equal(t, FakeCacheUsage{
		InputTokens:              40,
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

func TestFakeCachePlanResolveUsageWithConfig_ScalesCacheReadAndHonorsMinBlock(t *testing.T) {
	plan := &FakeCachePlan{
		IndependentCacheableTokens:    20,
		CurrentPrefixCacheableTokens:  80,
		PreviousPrefixCacheableTokens: 60,
		IndependentKey:                "independent",
		CurrentPrefixKey:              "prefix:current",
		PreviousPrefixKey:             "prefix:previous",
	}

	usage := plan.ResolveUsageWithConfig(140, FakeCacheHitState{
		Independent: true,
		Prefix:      true,
	}, FakeCacheUsageConfig{
		HitRateScale:   95,
		MinBlockTokens: 16,
	})

	require.Equal(t, FakeCacheUsage{
		InputTokens:              40,
		CacheCreationInputTokens: 24,
		CacheReadInputTokens:     76,
	}, usage)
}

func TestFakeCachePlanResolveUsageWithConfig_ZeroHitRateScaleIsValid(t *testing.T) {
	plan := &FakeCachePlan{
		CurrentCacheableTokens:  100,
		PreviousCacheableTokens: 60,
	}

	usage := plan.ResolveUsageWithConfig(140, FakeCacheHitState{
		Prefix: true,
	}, FakeCacheUsageConfig{
		HitRateScale: 0,
	})

	require.Equal(t, FakeCacheUsage{
		InputTokens:              40,
		CacheCreationInputTokens: 100,
		CacheReadInputTokens:     0,
	}, usage)
}

func TestFakeCachePlanResolveUsageWithConfig_DropsSmallBlocks(t *testing.T) {
	plan := &FakeCachePlan{
		IndependentCacheableTokens:    40,
		CurrentPrefixCacheableTokens:  40,
		PreviousPrefixCacheableTokens: 20,
		IndependentKey:                "independent",
		CurrentPrefixKey:              "prefix:current",
		PreviousPrefixKey:             "prefix:previous",
	}

	usage := plan.ResolveUsageWithConfig(140, FakeCacheHitState{
		Independent: true,
		Prefix:      true,
	}, FakeCacheUsageConfig{
		HitRateScale:   100,
		MinBlockTokens: 32,
	})

	require.Equal(t, FakeCacheUsage{
		InputTokens:              60,
		CacheCreationInputTokens: 40,
		CacheReadInputTokens:     40,
	}, usage)
}

func requireFakeCachePlan(t *testing.T, body string) *FakeCachePlan {
	t.Helper()

	plan, err := BuildFakeCachePlan([]byte(body), 7, "claude-sonnet-4")
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.NotEmpty(t, plan.CurrentPrefixKey)
	require.NotEmpty(t, plan.PreviousPrefixKey)
	return plan
}
