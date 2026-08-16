//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type recordingHTTPUpstreamStub struct {
	accountConcurrency int
	calls              int
}

func (s *recordingHTTPUpstreamStub) Do(_ *http.Request, _ string, _ int64, accountConcurrency int) (*http.Response, error) {
	s.calls++
	s.accountConcurrency = accountConcurrency
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Location": {"/backend-api/codex/call_test"}},
		Body:       io.NopCloser(strings.NewReader("v=0\r\n")),
	}, nil
}

func (s *recordingHTTPUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, proxyURL, accountID, accountConcurrency)
}

type recordingLiveLeaseCache struct {
	schedulerTestConcurrencyCache
	accountMax int
}

func (c *recordingLiveLeaseCache) AcquireLiveLease(
	_ context.Context,
	_ int64,
	accountMax int,
	_ int64,
	_ int,
	_ int64,
	_ string,
	_ bool,
) (bool, error) {
	c.accountMax = accountMax
	return true, nil
}

func (c *recordingLiveLeaseCache) RefreshLiveLease(context.Context, int64, int64, int64, string) (bool, error) {
	return true, nil
}

func (c *recordingLiveLeaseCache) ReleaseLiveLease(context.Context, int64, int64, int64, string) error {
	return nil
}

func TestCreateLiveCall_OAuthZeroConcurrencyPassesEffectiveTwelve(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()

	groupID := int64(91077)
	account := Account{
		ID:          77001,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 0,
		Credentials: map[string]any{
			"access_token":       "test-access-token",
			"chatgpt_account_id": "acct_test",
		},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	cfg.JWT.Secret = "live-effective-concurrency-secret"

	upstream := &recordingHTTPUpstreamStub{}
	leaseCache := &recordingLiveLeaseCache{}
	store := &liveTestStore{GatewayCache: &schedulerTestGatewayCache{}}
	svc := &OpenAIGatewayService{
		accountRepo:           schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:                 store,
		cfg:                   cfg,
		concurrencyService:    NewConcurrencyService(leaseCache),
		httpUpstream:          upstream,
		liveAttestation:       liveAttestationStub{header: `{"v":1,"s":0,"t":"v1.test"}`},
		liveAttestationCipher: newLiveAttestationCipher(cfg),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	created, err := svc.CreateLiveCall(ctx, &LiveCallRequest{
		SDP:     "v=offer\r\n",
		Session: json.RawMessage(`{"model":"gpt-live-test"}`),
	}, LiveCallIdentity{
		APIKeyID: 22,
		UserID:   33,
		GroupID:  &groupID,
	}, 4)
	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, 12, leaseCache.accountMax, "live lease max must use EffectiveConcurrency, not stored 0")
	require.Equal(t, 12, upstream.accountConcurrency, "live upstream Do must use EffectiveConcurrency, not stored 0")
}

func TestCreateUpstreamLiveCall_OAuthZeroConcurrencyPassesEffectiveTwelve(t *testing.T) {
	t.Parallel()

	upstream := &liveHTTPUpstreamStub{}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:          7,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 0,
		Credentials: map[string]any{
			"access_token":       "test-access-token",
			"chatgpt_account_id": "acct_test",
		},
	}

	created, err := svc.createUpstreamLiveCall(context.Background(), account, &LiveCallRequest{
		SDP:     "v=offer\r\n",
		Session: json.RawMessage(`{"model":"gpt-live-test"}`),
	}, `{"v":1,"s":0,"t":"v1.test"}`)
	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, 12, upstream.accountConcurrency, "live upstream Do must use EffectiveConcurrency, not stored 0")
}

func TestDoAccountHTTPUpstream_OAuthZeroConcurrencyPassesEffectiveTwelve(t *testing.T) {
	t.Parallel()

	upstream := &recordingHTTPUpstreamStub{}
	account := &Account{
		ID:          8,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 0,
	}
	req, err := http.NewRequest(http.MethodPost, "https://example.test/v1/messages", strings.NewReader(`{}`))
	require.NoError(t, err)

	resp, err := doAccountHTTPUpstream(context.Background(), upstream, req, "", account, nil, nil, "", "http", "messages")
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, 1, upstream.calls)
	require.Equal(t, 12, upstream.accountConcurrency)
}

func TestOpenAIWSConnPool_OAuthZeroConcurrencyIsNotUnlimited(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 32
	cfg.Gateway.OpenAIWS.DynamicMaxConnsByAccountConcurrencyEnabled = true
	cfg.Gateway.OpenAIWS.OAuthMaxConnsFactor = 1.0

	pool := newOpenAIWSConnPool(cfg)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 0}

	require.Equal(t, 12, pool.effectiveMaxConnsByAccount(account), "OAuth stored 0 must size the pool at EffectiveConcurrency 12, not the hard cap")
}
