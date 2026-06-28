package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestGetStickySessionAccountID_FallbackToLegacyKey(t *testing.T) {
	beforeFallbackTotal, beforeFallbackHit, _ := openAIStickyCompatStats()

	cache := &stubGatewayCache{
		sessionBindings: map[string]int64{
			"openai:legacy-hash": 42,
		},
	}
	svc := &OpenAIGatewayService{
		cache: cache,
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				OpenAIWS: config.GatewayOpenAIWSConfig{
					SessionHashReadOldFallback: true,
				},
			},
		},
	}

	ctx := withOpenAILegacySessionHash(context.Background(), "legacy-hash")
	accountID, err := svc.getStickySessionAccountID(ctx, nil, "new-hash")
	require.NoError(t, err)
	require.Equal(t, int64(42), accountID)

	afterFallbackTotal, afterFallbackHit, _ := openAIStickyCompatStats()
	require.Equal(t, beforeFallbackTotal+1, afterFallbackTotal)
	require.Equal(t, beforeFallbackHit+1, afterFallbackHit)
}

func TestGetStickySessionAccountIDWithSource_PrimaryHit(t *testing.T) {
	cache := &stubGatewayCache{
		sessionBindings: map[string]int64{
			"openai:new-hash": 42,
		},
		sessionTTLs: map[string]time.Duration{
			"openai:new-hash": 58 * time.Second,
		},
	}
	svc := &OpenAIGatewayService{
		cache: cache,
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				OpenAIWS: config.GatewayOpenAIWSConfig{
					SessionHashReadOldFallback: true,
				},
			},
		},
	}

	ctx := withOpenAILegacySessionHash(context.Background(), "legacy-hash")
	lookup := svc.getStickySessionAccountIDWithSource(ctx, nil, "new-hash")
	svc.populateOpenAIStickySessionLookupTTLs(ctx, nil, &lookup)

	require.Equal(t, int64(42), lookup.AccountID)
	require.Equal(t, "redis_current", lookup.Source)
	require.Equal(t, "openai:new-hash", lookup.PrimaryKey)
	require.Equal(t, int64(58000), lookup.PrimaryTTLMS)
	require.Empty(t, lookup.LegacyKey)
	require.True(t, lookup.PrimaryHit)
	require.False(t, lookup.LegacyFallbackAttempted)
	require.False(t, lookup.LegacyFallbackHit)
	require.NoError(t, lookup.Err)
}

func TestGetStickySessionAccountIDWithSource_LegacyFallbackHit(t *testing.T) {
	cache := &stubGatewayCache{
		sessionBindings: map[string]int64{
			"openai:legacy-hash": 42,
		},
		sessionTTLs: map[string]time.Duration{
			"openai:new-hash":    -2 * time.Millisecond,
			"openai:legacy-hash": 9 * time.Minute,
		},
	}
	svc := &OpenAIGatewayService{
		cache: cache,
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				OpenAIWS: config.GatewayOpenAIWSConfig{
					SessionHashReadOldFallback: true,
				},
			},
		},
	}

	ctx := withOpenAILegacySessionHash(context.Background(), "legacy-hash")
	lookup := svc.getStickySessionAccountIDWithSource(ctx, nil, "new-hash")
	svc.populateOpenAIStickySessionLookupTTLs(ctx, nil, &lookup)

	require.Equal(t, int64(42), lookup.AccountID)
	require.Equal(t, "redis_legacy_fallback", lookup.Source)
	require.Equal(t, "openai:new-hash", lookup.PrimaryKey)
	require.Equal(t, "openai:legacy-hash", lookup.LegacyKey)
	require.Equal(t, int64(-2), lookup.PrimaryTTLMS)
	require.Equal(t, int64(540000), lookup.LegacyTTLMS)
	require.False(t, lookup.PrimaryHit)
	require.NotEmpty(t, lookup.PrimaryError)
	require.True(t, lookup.LegacyFallbackEnabled)
	require.True(t, lookup.LegacyFallbackAttempted)
	require.True(t, lookup.LegacyFallbackHit)
	require.Empty(t, lookup.LegacyError)
	require.NoError(t, lookup.Err)
}

func TestOpenAIWSStickySelectDiagLogMessageIncludesRedisTTL(t *testing.T) {
	msg := openAIWSStickySelectDiagLogMessage(openAIWSStickySelectDiagLog{
		GroupID:                    22,
		APIKeyID:                   441,
		SessionHash:                "abcdef1234567890",
		Source:                     "redis_legacy_fallback",
		RedisSource:                "redis_legacy_fallback",
		PrimaryKey:                 "openai:new-hash",
		PrimaryTTLMS:               -2,
		LegacyKey:                  "openai:legacy-hash",
		LegacyTTLMS:                540000,
		AccountID:                  73972,
		ConnID:                     "oa_ws_73972_6",
		RedisAccountID:             73971,
		SessionContextAccountID:    73972,
		SessionContextConnID:       "oa_ws_73972_6",
		SessionContextAccountMatch: true,
		SessionContextFallback:     true,
		SessionContextBound:        true,
		PrimaryHit:                 false,
		PrimaryError:               "redis_nil",
		LegacyFallbackEnabled:      true,
		LegacyFallbackAttempted:    true,
		LegacyFallbackHit:          true,
		LegacyError:                "",
		RedisError:                 "redis_nil",
	})

	require.Contains(t, msg, "openai_ws_sticky_select_diag")
	require.Contains(t, msg, "primary_ttl_ms=-2")
	require.Contains(t, msg, "legacy_ttl_ms=540000")
	require.Contains(t, msg, "source=redis_legacy_fb")
	require.Contains(t, msg, "redis_account_id=73971")
	require.Contains(t, msg, "session_context_account_id=73972")
	require.Contains(t, msg, "session_context_conn_id=oa_ws_73972_6")
	require.Contains(t, msg, "session_context_account_match=true")
	require.Contains(t, msg, "session_context_fallback=true")
	require.Contains(t, msg, "session_context_bound=true")
}

func TestSetStickySessionAccountID_DualWriteOldEnabled(t *testing.T) {
	_, _, beforeDualWriteTotal := openAIStickyCompatStats()

	cache := &stubGatewayCache{sessionBindings: map[string]int64{}}
	svc := &OpenAIGatewayService{
		cache: cache,
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				OpenAIWS: config.GatewayOpenAIWSConfig{
					SessionHashDualWriteOld: true,
				},
			},
		},
	}

	ctx := withOpenAILegacySessionHash(context.Background(), "legacy-hash")
	err := svc.setStickySessionAccountID(ctx, nil, "new-hash", 9, openaiStickySessionTTL)
	require.NoError(t, err)
	require.Equal(t, int64(9), cache.sessionBindings["openai:new-hash"])
	require.Equal(t, int64(9), cache.sessionBindings["openai:legacy-hash"])

	_, _, afterDualWriteTotal := openAIStickyCompatStats()
	require.Equal(t, beforeDualWriteTotal+1, afterDualWriteTotal)
}

func TestSetStickySessionAccountID_DualWriteOldDisabled(t *testing.T) {
	cache := &stubGatewayCache{sessionBindings: map[string]int64{}}
	svc := &OpenAIGatewayService{
		cache: cache,
		cfg: &config.Config{
			Gateway: config.GatewayConfig{
				OpenAIWS: config.GatewayOpenAIWSConfig{
					SessionHashDualWriteOld: false,
				},
			},
		},
	}

	ctx := withOpenAILegacySessionHash(context.Background(), "legacy-hash")
	err := svc.setStickySessionAccountID(ctx, nil, "new-hash", 9, openaiStickySessionTTL)
	require.NoError(t, err)
	require.Equal(t, int64(9), cache.sessionBindings["openai:new-hash"])
	_, exists := cache.sessionBindings["openai:legacy-hash"]
	require.False(t, exists)
}

func TestSnapshotOpenAICompatibilityFallbackMetrics(t *testing.T) {
	before := SnapshotOpenAICompatibilityFallbackMetrics()

	ctx := context.WithValue(context.Background(), ctxkey.ThinkingEnabled, true)
	_, _ = ThinkingEnabledFromContext(ctx)

	after := SnapshotOpenAICompatibilityFallbackMetrics()
	require.GreaterOrEqual(t, after.MetadataLegacyFallbackTotal, before.MetadataLegacyFallbackTotal+1)
	require.GreaterOrEqual(t, after.MetadataLegacyFallbackThinkingEnabledTotal, before.MetadataLegacyFallbackThinkingEnabledTotal+1)
}
