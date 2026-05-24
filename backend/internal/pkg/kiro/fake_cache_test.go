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

func TestBuildFakeCachePlanBuildsCacheControlCheckpoints(t *testing.T) {
	plan := requireFakeCachePlan(t, `{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174003"},
		"system":"Use concise answers.",
		"tools":[{"name":"search","description":"find docs"}],
		"messages":[
			{"role":"user","content":[
				{"type":"text","text":"first"},
				{"type":"text","text":"cache me","cache_control":{"type":"ephemeral"}}
			]},
			{"role":"assistant","content":[
				{"type":"tool_use","id":"toolu_1","name":"search","input":{"query":"alpha"}}
			]},
			{"role":"user","content":[
				{"type":"tool_result","tool_use_id":"toolu_1","content":[{"type":"text","text":"alpha result"}],"cache_control":{"type":"ephemeral"}}
			]}
		]
	}`)

	require.GreaterOrEqual(t, len(plan.Checkpoints), 3)
	require.NotEmpty(t, plan.SessionProgressKey)
	require.Equal(t, plan.Checkpoints[len(plan.Checkpoints)-1].Tokens, plan.CurrentCheckpointTokens())
	for idx := 1; idx < len(plan.Checkpoints); idx++ {
		require.Greater(t, plan.Checkpoints[idx].Tokens, plan.Checkpoints[idx-1].Tokens)
		require.NotEqual(t, plan.Checkpoints[idx].Key, plan.Checkpoints[idx-1].Key)
	}
}

func TestFakeCachePlanResolveUsage(t *testing.T) {
	plan := &FakeCachePlan{
		PreviousPrefixCacheableTokens: 60,
		CurrentPrefixCacheableTokens:  100,
		CurrentPrefixKey:              "prefix:current",
		PreviousPrefixKey:             "prefix:previous",
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
		PreviousPrefixCacheableTokens: 120,
		CurrentPrefixCacheableTokens:  160,
		CurrentPrefixKey:              "prefix:current",
		PreviousPrefixKey:             "prefix:previous",
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

	// With 95% hit rate:
	// - Independent: 20 tokens (hit)
	// - PreviousPrefix: 60 tokens (hit)
	// - CurrentPrefix: 80 tokens (20 new)
	// Total cache read before scaling: 80 (20 + 60)
	// After 95% scaling: 76 tokens
	// Cache write: 20 tokens (80 - 60, new content)
	// Regular input: 44 tokens (140 - 76 - 20)
	require.Equal(t, FakeCacheUsage{
		InputTokens:              44, // Increased due to 5% cache miss
		CacheCreationInputTokens: 20, // Only new content, not affected by hit rate
		CacheReadInputTokens:     76, // 95% of 80
	}, usage)
}

func TestFakeCachePlanResolveUsageWithConfig_ScalesCacheReadAt98Percent(t *testing.T) {
	plan := &FakeCachePlan{
		CurrentPrefixCacheableTokens:  100,
		PreviousPrefixCacheableTokens: 60,
		CurrentPrefixKey:              "prefix:current",
		PreviousPrefixKey:             "prefix:previous",
	}

	usage := plan.ResolveUsageWithConfig(140, FakeCacheHitState{
		Prefix: true,
	}, FakeCacheUsageConfig{
		HitRateScale: 98,
	})

	require.Equal(t, FakeCacheUsage{
		InputTokens:              42,
		CacheCreationInputTokens: 40,
		CacheReadInputTokens:     58, // 98% of 60
	}, usage)
}

func TestFakeCachePlanResolveUsageWithConfig_ZeroHitRateScaleIsValid(t *testing.T) {
	plan := &FakeCachePlan{
		CurrentPrefixCacheableTokens:  100,
		PreviousPrefixCacheableTokens: 60,
		CurrentPrefixKey:              "prefix:current",
		PreviousPrefixKey:             "prefix:previous",
	}

	usage := plan.ResolveUsageWithConfig(140, FakeCacheHitState{
		Prefix: true,
	}, FakeCacheUsageConfig{
		HitRateScale: 0,
	})

	// With 0% hit rate, all cache reads should become regular input tokens
	// Cache write should remain the same (40 tokens for the new content)
	require.Equal(t, FakeCacheUsage{
		InputTokens:              100, // 40 (non-cached) + 60 (failed cache read)
		CacheCreationInputTokens: 40,  // Only new content (100 - 60)
		CacheReadInputTokens:     0,   // No cache hits with 0% rate
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
		CacheCreationInputTokens: 20,
		CacheReadInputTokens:     60,
	}, usage)
}

func TestFakeCachePlanResolveUsageWithConfig_WritesSmallIncrementAfterEligiblePrefixHit(t *testing.T) {
	plan := &FakeCachePlan{
		IndependentCacheableTokens:    2600,
		CurrentPrefixCacheableTokens:  320,
		PreviousPrefixCacheableTokens: 200,
		IndependentKey:                "independent",
		CurrentPrefixKey:              "prefix:current",
		PreviousPrefixKey:             "prefix:previous",
	}

	usage := plan.ResolveUsageWithConfig(3300, FakeCacheHitState{
		Independent: true,
		Prefix:      true,
	}, FakeCacheUsageConfig{
		HitRateScale:   100,
		MinBlockTokens: 1024,
	})

	require.Equal(t, FakeCacheUsage{
		InputTokens:              380,
		CacheCreationInputTokens: 120,
		CacheReadInputTokens:     2800,
	}, usage)
}

func TestFakeCachePlanResolveUsageWithConfig_UsesLongestCheckpointHit(t *testing.T) {
	plan := &FakeCachePlan{
		Checkpoints: []FakeCacheCheckpoint{
			{Key: "checkpoint:system", Tokens: 2600},
			{Key: "checkpoint:previous", Tokens: 2800},
			{Key: "checkpoint:current", Tokens: 2920},
		},
	}

	usage := plan.ResolveUsageWithConfig(3300, FakeCacheHitState{
		CheckpointTokens: 2800,
	}, FakeCacheUsageConfig{
		HitRateScale:   100,
		MinBlockTokens: 1024,
	})

	require.Equal(t, FakeCacheUsage{
		InputTokens:              380,
		CacheCreationInputTokens: 120,
		CacheReadInputTokens:     2800,
	}, usage)
}

func TestFakeCachePlanResolveUsageWithConfig_NoSessionID(t *testing.T) {
	// When there's no session ID, all keys are empty
	// Should return all tokens as regular input, no cache simulation
	plan := &FakeCachePlan{
		CurrentCacheableTokens:  1000,
		PreviousCacheableTokens: 0,
		IndependentKey:          "",
		CurrentPrefixKey:        "",
		PreviousPrefixKey:       "",
	}

	usage := plan.ResolveUsageWithConfig(1000, FakeCacheHitState{
		Independent: false,
		Prefix:      false,
	}, FakeCacheUsageConfig{
		HitRateScale:   95,
		MinBlockTokens: 1024,
	})

	// Should NOT simulate cache when no session ID
	require.Equal(t, FakeCacheUsage{
		InputTokens:              1000,
		CacheCreationInputTokens: 0,
		CacheReadInputTokens:     0,
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
