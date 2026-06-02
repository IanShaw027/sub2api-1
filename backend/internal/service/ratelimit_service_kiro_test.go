//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type kiroRateLimitRepoStub struct {
	mockAccountRepoForGemini
	rateLimitedIDs []int64
	rateLimitUntil []time.Time
	tempIDs        []int64
	tempUntil      []time.Time
	tempReasons    []string
}

func (r *kiroRateLimitRepoStub) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	r.rateLimitedIDs = append(r.rateLimitedIDs, id)
	r.rateLimitUntil = append(r.rateLimitUntil, resetAt)
	return nil
}

func (r *kiroRateLimitRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	r.tempIDs = append(r.tempIDs, id)
	r.tempUntil = append(r.tempUntil, until)
	r.tempReasons = append(r.tempReasons, reason)
	return nil
}

type kiroTempUnschedCacheRecorder struct {
	accountIDs []int64
	states     []*TempUnschedState
}

func (c *kiroTempUnschedCacheRecorder) SetTempUnsched(ctx context.Context, accountID int64, state *TempUnschedState) error {
	c.accountIDs = append(c.accountIDs, accountID)
	c.states = append(c.states, state)
	return nil
}

func (c *kiroTempUnschedCacheRecorder) GetTempUnsched(ctx context.Context, accountID int64) (*TempUnschedState, error) {
	return nil, nil
}

func (c *kiroTempUnschedCacheRecorder) DeleteTempUnsched(ctx context.Context, accountID int64) error {
	return nil
}

func TestRateLimitService_HandleUpstreamError_Kiro429WithoutWindowDoesNotPersistCooldown(t *testing.T) {
	repo := &kiroRateLimitRepoStub{}
	cache := &kiroTempUnschedCacheRecorder{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, cache)
	account := &Account{ID: 42, Platform: PlatformKiro, Type: AccountTypeOAuth}

	shouldDisable := svc.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{}, []byte(`{"message":"Too many requests, please wait before trying again.","reason":null}`))

	require.False(t, shouldDisable)
	require.Empty(t, repo.rateLimitedIDs)
	require.Empty(t, repo.tempIDs)
	require.Empty(t, cache.accountIDs)
	require.Empty(t, cache.states)
}

func TestRateLimitService_HandleUpstreamError_Kiro429ShortRetryAfterUsesTempUnschedulable(t *testing.T) {
	repo := &kiroRateLimitRepoStub{}
	cache := &kiroTempUnschedCacheRecorder{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, cache)
	account := &Account{ID: 43, Platform: PlatformKiro, Type: AccountTypeOAuth}
	headers := http.Header{"Retry-After": []string{"12"}}
	before := time.Now()

	shouldDisable := svc.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, headers, []byte(`{"message":"Too many requests, please wait before trying again.","reason":null}`))

	require.False(t, shouldDisable)
	require.Empty(t, repo.rateLimitedIDs)
	require.Equal(t, []int64{43}, repo.tempIDs)
	require.Len(t, repo.tempUntil, 1)
	require.WithinDuration(t, before.Add(12*time.Second), repo.tempUntil[0], 2*time.Second)
	require.Equal(t, []int64{43}, cache.accountIDs)
	require.Len(t, cache.states, 1)
	require.WithinDuration(t, before.Add(12*time.Second), time.Unix(cache.states[0].UntilUnix, 0), 2*time.Second)
}

func TestRateLimitService_HandleUpstreamError_Kiro429LongRetryAfterUsesRateLimitedPath(t *testing.T) {
	repo := &kiroRateLimitRepoStub{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{ID: 44, Platform: PlatformKiro, Type: AccountTypeOAuth}
	headers := http.Header{"Retry-After": []string{"900"}}
	before := time.Now()
	body := []byte(`{"message":"Too many requests, please wait before trying again.","reason":null}`)

	shouldDisable := svc.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, headers, body)

	require.False(t, shouldDisable)
	require.Empty(t, repo.tempIDs)
	require.Equal(t, []int64{44}, repo.rateLimitedIDs)
	require.Len(t, repo.rateLimitUntil, 1)
	require.WithinDuration(t, before.Add(15*time.Minute), repo.rateLimitUntil[0], 2*time.Second)
}

func TestRateLimitService_HandleUpstreamError_KiroQuotaExhausted429KeepsRateLimitedPath(t *testing.T) {
	repo := &kiroRateLimitRepoStub{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{ID: 45, Platform: PlatformKiro, Type: AccountTypeOAuth}
	before := time.Now()
	body := []byte(`{"message":"monthly quota exceeded for this subscription"}`)

	shouldDisable := svc.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{}, body)

	require.False(t, shouldDisable)
	require.Empty(t, repo.tempIDs)
	require.Equal(t, []int64{45}, repo.rateLimitedIDs)
	require.Len(t, repo.rateLimitUntil, 1)
	require.WithinDuration(t, before.Add(5*time.Second), repo.rateLimitUntil[0], 2*time.Second)
}
