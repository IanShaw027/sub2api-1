package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIEmbeddings_SelectionFailure_ReturnsSupportingModelMessage(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/embeddings", `{"model":"text-embedding-3-large","input":"hello"}`)
	h := newOpenAISelectionErrorTestHandler(t, nil)

	h.Embeddings(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), `"message":"No available accounts supporting model: text-embedding-3-large"`)
}

func TestOpenAIEmbeddings_LocalExclusionStillReturnsSupportingModelMessage(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/embeddings", `{"model":"text-embedding-3-large","input":"hello"}`)
	h := newOpenAISelectionErrorTestHandler(t, []service.Account{
		{
			ID:       1,
			Platform: service.PlatformOpenAI,
			Type:     service.AccountTypeAPIKey,
			Name:     "slot-retry",
		},
	})
	h.cfg.Gateway.Scheduling.FallbackWaitTimeout = time.Millisecond
	cache := &concurrencyCacheMock{
		acquireUserSlotFn: func(_ context.Context, _ int64, _ int, _ string) (bool, error) {
			return true, nil
		},
		acquireAccountSlotFn: func(_ context.Context, _ int64, _ int, _ string) (bool, error) {
			return false, nil
		},
	}
	h.concurrencyHelper = NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second)

	h.Embeddings(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"message":"No available accounts supporting model: text-embedding-3-large"`)
}
