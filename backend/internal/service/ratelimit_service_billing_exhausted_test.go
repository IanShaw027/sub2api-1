package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestIsAPIKeyBillingExhausted(t *testing.T) {
	t.Parallel()

	apikey := &Account{Type: AccountTypeAPIKey, Platform: PlatformOpenAI}
	anthropic := &Account{Type: AccountTypeAPIKey, Platform: PlatformAnthropic}
	oauth := &Account{Type: AccountTypeOAuth, Platform: PlatformOpenAI}

	require.True(t, isAPIKeyBillingExhausted(apikey, http.StatusPaymentRequired, nil))
	require.True(t, isAPIKeyBillingExhausted(apikey, http.StatusForbidden, []byte(`{"error":{"code":"insufficient_balance","message":"余额不足"}}`)))
	require.True(t, isAPIKeyBillingExhausted(apikey, http.StatusForbidden, []byte(`insufficient balance`)))
	require.True(t, isAPIKeyBillingExhausted(anthropic, http.StatusBadRequest, []byte(`{"error":{"message":"Your credit balance is too low"}}`)))
	require.False(t, isAPIKeyBillingExhausted(apikey, http.StatusBadGateway, []byte(`insufficient balance`)))
	require.False(t, isAPIKeyBillingExhausted(oauth, http.StatusPaymentRequired, nil))
	require.False(t, isAPIKeyBillingExhausted(apikey, http.StatusForbidden, []byte(`permission denied`)))
}

func TestHandleUpstreamError_PoolModeBillingExhaustedStopsScheduling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		body       []byte
	}{
		{name: "402", statusCode: http.StatusPaymentRequired, body: []byte(`{"error":{"message":"insufficient balance"}}`)},
		{name: "403_balance", statusCode: http.StatusForbidden, body: []byte(`{"error":{"code":"insufficient_quota","message":"insufficient quota"}}`)},
		{name: "anthropic_400", statusCode: http.StatusBadRequest, body: []byte(`{"error":{"message":"credit balance too low"}}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &errorPolicyRepoStub{}
			svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			account := &Account{
				ID:       40,
				Type:     AccountTypeAPIKey,
				Platform: PlatformAnthropic,
				Credentials: map[string]any{
					"pool_mode": true,
				},
			}

			shouldDisable := svc.HandleUpstreamError(context.Background(), account, tt.statusCode, http.Header{}, tt.body)
			require.True(t, shouldDisable)
			require.Equal(t, 1, repo.setErrCalls)
			require.Equal(t, ErrorPolicyMatched, svc.CheckErrorPolicy(context.Background(), account, tt.statusCode, tt.body))
		})
	}
}

func TestHandleUpstreamError_PoolModeTransientErrorStillSkips(t *testing.T) {
	t.Parallel()

	repo := &errorPolicyRepoStub{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:       41,
		Type:     AccountTypeAPIKey,
		Platform: PlatformOpenAI,
		Credentials: map[string]any{
			"pool_mode": true,
		},
	}

	shouldDisable := svc.HandleUpstreamError(context.Background(), account, http.StatusBadGateway, http.Header{}, []byte(`bad gateway`))
	require.False(t, shouldDisable)
	require.Equal(t, 0, repo.setErrCalls)
	require.Equal(t, ErrorPolicySkipped, svc.CheckErrorPolicy(context.Background(), account, http.StatusBadGateway, []byte(`bad gateway`)))
}
