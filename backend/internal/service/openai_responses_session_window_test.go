package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIResponsesSessionWindowStore_BindAndGetViaSharedCache(t *testing.T) {
	cache := &stubGatewayCache{}
	writer := NewOpenAIWSStateStore(cache)
	reader := NewOpenAIWSStateStore(cache)

	window := openAIResponsesSessionWindow{
		LatestResponseID: "resp_latest_1",
		PromptCacheKey:   "pcache_1",
		ReplayInputRaw:   []byte(`[{"type":"input_text","text":"history"}]`),
	}

	require.NoError(t, writer.BindSessionWindow(context.Background(), 7, 11, "session_hash_1", window, time.Hour))

	got, ok := reader.GetSessionWindow(context.Background(), 7, 11, "session_hash_1")
	require.True(t, ok)
	require.Equal(t, "resp_latest_1", got.LatestResponseID)
	require.Equal(t, "pcache_1", got.PromptCacheKey)
	require.JSONEq(t, string(window.ReplayInputRaw), string(got.ReplayInputRaw))
}
