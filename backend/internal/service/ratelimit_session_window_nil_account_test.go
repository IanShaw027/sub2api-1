//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateSessionWindowNilAccountDoesNotPanicOrPersist(t *testing.T) {
	repo := &sessionWindowMockRepo{}
	svc := newRateLimitServiceForTest(repo)

	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-5h-status", "allowed")
	headers.Set("anthropic-ratelimit-unified-5h-reset", "1735689600")

	require.NotPanics(t, func() {
		svc.UpdateSessionWindow(context.Background(), nil, headers)
	})
	require.Empty(t, repo.sessionWindowCalls)
	require.Empty(t, repo.updateExtraCalls)
}
