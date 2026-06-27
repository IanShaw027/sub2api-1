package service

import (
	"context"
	"testing"

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

	require.Equal(t, int64(42), lookup.AccountID)
	require.Equal(t, "redis_current", lookup.Source)
	require.Equal(t, "openai:new-hash", lookup.PrimaryKey)
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

	require.Equal(t, int64(42), lookup.AccountID)
	require.Equal(t, "redis_legacy_fallback", lookup.Source)
	require.Equal(t, "openai:new-hash", lookup.PrimaryKey)
	require.Equal(t, "openai:legacy-hash", lookup.LegacyKey)
	require.False(t, lookup.PrimaryHit)
	require.NotEmpty(t, lookup.PrimaryError)
	require.True(t, lookup.LegacyFallbackEnabled)
	require.True(t, lookup.LegacyFallbackAttempted)
	require.True(t, lookup.LegacyFallbackHit)
	require.Empty(t, lookup.LegacyError)
	require.NoError(t, lookup.Err)
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
