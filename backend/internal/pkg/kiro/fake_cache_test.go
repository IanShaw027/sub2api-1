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

	plan, err := BuildFakeCachePlan(body, FakeCacheScope{AccountID: 42, UserID: 1, APIKeyID: 2}, "claude-sonnet-4")
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

	plan, err := BuildFakeCachePlan(body, FakeCacheScope{AccountID: 7, UserID: 1, APIKeyID: 2}, "claude-sonnet-4")
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.NotEmpty(t, plan.CurrentKey)
	require.NotEmpty(t, plan.PreviousKey)
	require.NotEmpty(t, plan.IndependentKey)
	require.NotEmpty(t, plan.CurrentPrefixKey)
	require.NotEmpty(t, plan.PreviousPrefixKey)
	require.NotEqual(t, plan.CurrentKey, plan.PreviousKey)
	require.Greater(t, plan.CurrentCacheableTokens, plan.PreviousCacheableTokens)
	require.Contains(t, plan.CurrentKey, "user:1:key:2:model:claude-sonnet-4:session:123e4567-e89b-12d3-a456-426614174000")
}

func TestBuildFakeCachePlanAcceptsJSONMetadataUserID(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"{\"device_id\":\"device\",\"account_uuid\":\"\",\"session_id\":\"123e4567-e89b-12d3-a456-426614174000\"}"},
		"messages":[
			{"role":"user","content":"hello"}
		]
	}`)

	plan, err := BuildFakeCachePlan(body, FakeCacheScope{AccountID: 7, UserID: 1, APIKeyID: 2}, "claude-sonnet-4")
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Contains(t, plan.CurrentKey, "session:123e4567-e89b-12d3-a456-426614174000")
}

func TestBuildFakeCachePlanAcceptsJSONMetadataUserIDWithNonUUIDSessionID(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"{\"device_id\":\"device\",\"account_uuid\":\"\",\"session_id\":\"kiro-recheck-nonuuid-session\"}"},
		"messages":[
			{"role":"user","content":"hello"}
		]
	}`)

	plan, err := BuildFakeCachePlan(body, FakeCacheScope{AccountID: 7, UserID: 1, APIKeyID: 2}, "claude-sonnet-4")
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Contains(t, plan.CurrentKey, "session:kiro-recheck-nonuuid-session")
}

func TestBuildFakeCachePlanReusesKeysAcrossAccountsForSameUserAndAPIKey(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-4266141740aa"},
		"messages":[
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"hi"},
			{"role":"user","content":"please continue"}
		]
	}`)

	first, err := BuildFakeCachePlan(body, FakeCacheScope{AccountID: 101, UserID: 7, APIKeyID: 11}, "claude-sonnet-4")
	require.NoError(t, err)
	second, err := BuildFakeCachePlan(body, FakeCacheScope{AccountID: 202, UserID: 7, APIKeyID: 11}, "claude-sonnet-4")
	require.NoError(t, err)

	require.NotNil(t, first)
	require.NotNil(t, second)
	require.Equal(t, first.IndependentKey, second.IndependentKey)
	require.Equal(t, first.CurrentPrefixKey, second.CurrentPrefixKey)
	require.Equal(t, first.PreviousPrefixKey, second.PreviousPrefixKey)
	require.Equal(t, first.SessionProgressKey, second.SessionProgressKey)
	require.Contains(t, first.CurrentKey, "user:7:key:11:model:claude-sonnet-4:session:123e4567-e89b-12d3-a456-4266141740aa")
}

func TestBuildFakeCachePlanSeparatesDifferentUsersAndAPIKeys(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-4266141740ab"},
		"messages":[
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"hi"},
			{"role":"user","content":"please continue"}
		]
	}`)

	base, err := BuildFakeCachePlan(body, FakeCacheScope{AccountID: 101, UserID: 7, APIKeyID: 11}, "claude-sonnet-4")
	require.NoError(t, err)
	otherUser, err := BuildFakeCachePlan(body, FakeCacheScope{AccountID: 202, UserID: 8, APIKeyID: 11}, "claude-sonnet-4")
	require.NoError(t, err)
	otherKey, err := BuildFakeCachePlan(body, FakeCacheScope{AccountID: 303, UserID: 7, APIKeyID: 12}, "claude-sonnet-4")
	require.NoError(t, err)

	require.NotNil(t, base)
	require.NotNil(t, otherUser)
	require.NotNil(t, otherKey)
	require.NotEqual(t, base.CurrentKey, otherUser.CurrentKey)
	require.NotEqual(t, base.CurrentKey, otherKey.CurrentKey)
	require.NotEqual(t, base.SessionProgressKey, otherUser.SessionProgressKey)
	require.NotEqual(t, base.SessionProgressKey, otherKey.SessionProgressKey)
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

func TestBuildFakeCachePlanPrefixKeysIncludeNativeWebToolHistory(t *testing.T) {
	base := requireFakeCachePlan(t, `{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174020"},
		"messages":[
			{"role":"user","content":"search go docs"},
			{"role":"assistant","content":[
				{"type":"server_tool_use","id":"srvtoolu_search_1","name":"web_search","input":{"query":"golang"}}
			]},
			{"role":"user","content":[
				{"type":"web_search_tool_result","tool_use_id":"srvtoolu_search_1","content":[{"type":"url","url":"https://go.dev","title":"The Go Programming Language"}]}
			]},
			{"role":"assistant","content":[
				{"type":"server_tool_use","id":"srvtoolu_fetch_1","name":"web_fetch","input":{"url":"https://go.dev"}}
			]},
			{"role":"user","content":[
				{"type":"web_fetch_tool_result","tool_use_id":"srvtoolu_fetch_1","content":{"type":"web_fetch_result","url":"https://go.dev","text":"Go docs"}}
			]},
			{"role":"user","content":"continue"}
		]
	}`)

	cases := map[string]string{
		"server_tool_use_input": `{
			"model":"claude-sonnet-4",
			"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174020"},
			"messages":[
				{"role":"user","content":"search go docs"},
				{"role":"assistant","content":[
					{"type":"server_tool_use","id":"srvtoolu_search_1","name":"web_search","input":{"query":"rust"}}
				]},
				{"role":"user","content":[
					{"type":"web_search_tool_result","tool_use_id":"srvtoolu_search_1","content":[{"type":"url","url":"https://go.dev","title":"The Go Programming Language"}]}
				]},
				{"role":"assistant","content":[
					{"type":"server_tool_use","id":"srvtoolu_fetch_1","name":"web_fetch","input":{"url":"https://go.dev"}}
				]},
				{"role":"user","content":[
					{"type":"web_fetch_tool_result","tool_use_id":"srvtoolu_fetch_1","content":{"type":"web_fetch_result","url":"https://go.dev","text":"Go docs"}}
				]},
				{"role":"user","content":"continue"}
			]
		}`,
		"web_search_result": `{
			"model":"claude-sonnet-4",
			"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174020"},
			"messages":[
				{"role":"user","content":"search go docs"},
				{"role":"assistant","content":[
					{"type":"server_tool_use","id":"srvtoolu_search_1","name":"web_search","input":{"query":"golang"}}
				]},
				{"role":"user","content":[
					{"type":"web_search_tool_result","tool_use_id":"srvtoolu_search_1","content":[{"type":"url","url":"https://pkg.go.dev","title":"Go Packages"}]}
				]},
				{"role":"assistant","content":[
					{"type":"server_tool_use","id":"srvtoolu_fetch_1","name":"web_fetch","input":{"url":"https://go.dev"}}
				]},
				{"role":"user","content":[
					{"type":"web_fetch_tool_result","tool_use_id":"srvtoolu_fetch_1","content":{"type":"web_fetch_result","url":"https://go.dev","text":"Go docs"}}
				]},
				{"role":"user","content":"continue"}
			]
		}`,
		"web_fetch_result": `{
			"model":"claude-sonnet-4",
			"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174020"},
			"messages":[
				{"role":"user","content":"search go docs"},
				{"role":"assistant","content":[
					{"type":"server_tool_use","id":"srvtoolu_search_1","name":"web_search","input":{"query":"golang"}}
				]},
				{"role":"user","content":[
					{"type":"web_search_tool_result","tool_use_id":"srvtoolu_search_1","content":[{"type":"url","url":"https://go.dev","title":"The Go Programming Language"}]}
				]},
				{"role":"assistant","content":[
					{"type":"server_tool_use","id":"srvtoolu_fetch_1","name":"web_fetch","input":{"url":"https://go.dev"}}
				]},
				{"role":"user","content":[
					{"type":"web_fetch_tool_result","tool_use_id":"srvtoolu_fetch_1","content":{"type":"web_fetch_result","url":"https://go.dev","text":"Updated Go docs"}}
				]},
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

func TestBuildFakeCachePlanCalibratesCacheableTokens(t *testing.T) {
	body := []byte(`{
		"model":"claude-sonnet-4",
		"metadata":{"user_id":"user_x_account__session_123e4567-e89b-12d3-a456-426614174111"},
		"system":"You are a helpful coding assistant. Always be concise and accurate.",
		"tools":[
			{"name":"search","description":"find docs in the knowledge base","input_schema":{"type":"object","properties":{"query":{"type":"string","description":"the search query"}},"required":["query"]}},
			{"name":"write_file","description":"write content to a file path","input_schema":{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}}
		],
		"messages":[
			{"role":"user","content":"Please help me refactor the authentication module to use the new token provider."},
			{"role":"assistant","content":"Sure, let me look at the current implementation first."},
			{"role":"user","content":"Go ahead, the files are in internal/auth."}
		]
	}`)

	plan, err := BuildFakeCachePlan(body, FakeCacheScope{AccountID: 7, UserID: 1, APIKeyID: 2}, "claude-sonnet-4")
	require.NoError(t, err)
	require.NotNil(t, plan)

	// The independent block carries the 493-token tool system scaffold plus the
	// per-tool and schema×1.75 calibration, so it must dwarf a raw tiktoken count.
	require.Greater(t, plan.IndependentCacheableTokens, kiroTokenEstimateToolSystemBase)

	// A full cache hit (independent + previous prefix cached) must drive billable
	// input down near the residual: only the final user turn's calibrated content
	// stays uncached.
	total := EstimateInputTokens(body)
	usage := plan.ResolveUsageWithConfig(total, FakeCacheHitState{
		Independent: true,
		Prefix:      true,
	}, FakeCacheUsageConfig{HitRateScale: 100})

	require.Equal(t, total, usage.InputTokens+usage.CacheReadInputTokens+usage.CacheCreationInputTokens)
	// Before the calibration fix the residual was content-proportional and grew
	// without bound; now the cached portion offsets the calibrated weight, so the
	// leftover billable input is a small fraction of the total.
	require.Less(t, usage.InputTokens, total/2)
	require.Greater(t, usage.CacheReadInputTokens, 0)
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
	// - ideal read: Independent 20 + PreviousPrefix 60 = 80
	// - currentTokens: Independent 20 + CurrentPrefix 80 = 100
	// - existing read remains 80
	// - new growth is 20, 95% of it is written: 19
	// - the unwritten growth token remains input
	require.Equal(t, FakeCacheUsage{
		InputTokens:              41,
		CacheCreationInputTokens: 19,
		CacheReadInputTokens:     80,
	}, usage)
}

func TestFakeCachePlanResolveUsageWithConfig_ScalesNewGrowthIntoInput(t *testing.T) {
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
		HitRateScale: 95,
	})

	require.Equal(t, 140, usage.InputTokens+usage.CacheCreationInputTokens+usage.CacheReadInputTokens)
	require.Equal(t, 80, usage.CacheReadInputTokens)
	require.Equal(t, 19, usage.CacheCreationInputTokens)
	require.Equal(t, 41, usage.InputTokens)
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
		InputTokens:              41,
		CacheCreationInputTokens: 39,
		CacheReadInputTokens:     60,
	}, usage)
	require.Equal(t, usage.CacheReadInputTokens+usage.CacheCreationInputTokens, plan.RecordedEffectiveCachedTokens)
}

func TestFakeCachePlanResolveUsageWithConfig_ChainsEffectiveCacheAcrossTurns(t *testing.T) {
	turn1 := &FakeCachePlan{
		CurrentPrefixCacheableTokens: 1000,
		CurrentPrefixKey:             "prefix:current",
	}
	u1 := turn1.ResolveUsageWithConfig(1000, FakeCacheHitState{}, FakeCacheUsageConfig{HitRateScale: 95})
	require.Equal(t, 950, u1.CacheCreationInputTokens)
	require.Equal(t, 0, u1.CacheReadInputTokens)
	require.Equal(t, 50, u1.InputTokens)
	require.Equal(t, u1.CacheReadInputTokens+u1.CacheCreationInputTokens, turn1.RecordedEffectiveCachedTokens)

	turn2 := &FakeCachePlan{
		CurrentPrefixCacheableTokens:  1200,
		PreviousPrefixCacheableTokens: 1000,
		CurrentPrefixKey:              "prefix:current",
		PreviousPrefixKey:             "prefix:previous",
	}
	u2 := turn2.ResolveUsageWithConfig(1200, FakeCacheHitState{
		Prefix:                true,
		EffectiveCachedTokens: turn1.RecordedEffectiveCachedTokens,
	}, FakeCacheUsageConfig{HitRateScale: 95})
	require.Equal(t, 950, u2.CacheReadInputTokens)
	require.Equal(t, 237, u2.CacheCreationInputTokens)
	require.Equal(t, 13, u2.InputTokens)
	require.Equal(t, u2.CacheReadInputTokens+u2.CacheCreationInputTokens, turn2.RecordedEffectiveCachedTokens)

	turn3 := &FakeCachePlan{
		CurrentPrefixCacheableTokens:  1300,
		PreviousPrefixCacheableTokens: 1200,
		CurrentPrefixKey:              "prefix:current",
		PreviousPrefixKey:             "prefix:previous",
	}
	u3 := turn3.ResolveUsageWithConfig(1300, FakeCacheHitState{
		Prefix:                true,
		EffectiveCachedTokens: turn2.RecordedEffectiveCachedTokens,
	}, FakeCacheUsageConfig{HitRateScale: 95})
	require.Equal(t, u2.CacheReadInputTokens+u2.CacheCreationInputTokens, u3.CacheReadInputTokens)
	require.Equal(t, 107, u3.CacheCreationInputTokens)
	require.Equal(t, 6, u3.InputTokens)
	require.Equal(t, u3.CacheReadInputTokens+u3.CacheCreationInputTokens, turn3.RecordedEffectiveCachedTokens)
}

func TestFakeCachePlanResolveUsageWithConfig_UsesSessionProgressWhenPrefixKeyMisses(t *testing.T) {
	plan := &FakeCachePlan{
		IndependentCacheableTokens:   34000,
		CurrentPrefixCacheableTokens: 76000,
		IndependentKey:               "independent",
		CurrentPrefixKey:             "prefix:current",
	}

	usage := plan.ResolveUsageWithConfig(110000, FakeCacheHitState{
		Independent:           true,
		EffectiveCachedTokens: 100000,
	}, FakeCacheUsageConfig{
		HitRateScale: 95,
	})

	require.Equal(t, FakeCacheUsage{
		InputTokens:              500,
		CacheCreationInputTokens: 9500,
		CacheReadInputTokens:     100000,
	}, usage)
	require.Equal(t, usage.CacheReadInputTokens+usage.CacheCreationInputTokens, plan.RecordedEffectiveCachedTokens)
}

func TestFakeCachePlanResolveUsageWithConfig_CapsLongerRealHitToSessionProgress(t *testing.T) {
	plan := &FakeCachePlan{
		CurrentPrefixKey: "prefix:current",
		Checkpoints: []FakeCacheCheckpoint{
			{Key: "checkpoint:previous", Tokens: 2800},
			{Key: "checkpoint:current", Tokens: 2920},
		},
	}

	usage := plan.ResolveUsageWithConfig(3300, FakeCacheHitState{
		CheckpointTokens:      2800,
		EffectiveCachedTokens: 1000,
	}, FakeCacheUsageConfig{
		HitRateScale:   100,
		MinBlockTokens: 1024,
	})

	require.Equal(t, FakeCacheUsage{
		InputTokens:              380,
		CacheCreationInputTokens: 1920,
		CacheReadInputTokens:     1000,
	}, usage)
	require.Equal(t, 2920, plan.RecordedEffectiveCachedTokens)
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

	// With 0% hit rate, existing cache remains readable but no new growth is
	// written. The unwritten growth is reported as input.
	require.Equal(t, FakeCacheUsage{
		InputTokens:              80,
		CacheCreationInputTokens: 0,
		CacheReadInputTokens:     60,
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

func TestFakeCachePlanResolveUsageWithConfig_CheckpointsStillWriteCurrentPrefixDelta(t *testing.T) {
	plan := &FakeCachePlan{
		IndependentCacheableTokens:    2600,
		CurrentPrefixCacheableTokens:  320,
		PreviousPrefixCacheableTokens: 200,
		Checkpoints: []FakeCacheCheckpoint{
			{Key: "checkpoint:system", Tokens: 2600},
			{Key: "checkpoint:previous", Tokens: 2800},
		},
	}

	usage := plan.ResolveUsageWithConfig(3300, FakeCacheHitState{
		Independent:      true,
		Prefix:           true,
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

	plan, err := BuildFakeCachePlan([]byte(body), FakeCacheScope{AccountID: 7, UserID: 1, APIKeyID: 2}, "claude-sonnet-4")
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.NotEmpty(t, plan.CurrentPrefixKey)
	require.NotEmpty(t, plan.PreviousPrefixKey)
	return plan
}
