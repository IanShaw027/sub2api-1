//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type grokTestAccountRepo struct {
	stubOpenAIAccountRepo
	updatedExtra          map[string]any
	tempUnschedCalls      int
	lastTempUnschedID     int64
	lastTempUnschedUntil  time.Time
	lastTempUnschedReason string
	setRateLimitedCalls   int
}

func (r *grokTestAccountRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	if updates != nil {
		r.updatedExtra = make(map[string]any, len(updates))
		for k, v := range updates {
			r.updatedExtra[k] = v
		}
	}
	return nil
}

func (r *grokTestAccountRepo) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	r.tempUnschedCalls++
	r.lastTempUnschedID = id
	r.lastTempUnschedUntil = until
	r.lastTempUnschedReason = reason
	return nil
}

func (r *grokTestAccountRepo) SetRateLimited(_ context.Context, _ int64, _ time.Time) error {
	r.setRateLimitedCalls++
	return nil
}

func TestAccountTestService_TestAccountConnection_GrokOAuthUsesXAIResponses(t *testing.T) {
	setGinTestMode()

	account := Account{
		ID:          73977,
		Name:        "grok-oauth",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "grok-access-token",
			"refresh_token": "grok-refresh-token",
		},
	}
	repo := stubOpenAIAccountRepo{accounts: []Account{account}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n" +
				"data: {\"type\":\"response.completed\"}\n\n" +
				"data: [DONE]\n\n",
		)),
	}}
	svc := &AccountTestService{
		accountRepo:       repo,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		httpUpstream:      upstream,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/73977/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "grok-4.3", "hello grok", "")
	require.NoError(t, err)

	require.NotNil(t, upstream.lastReq)
	require.Equal(t, xai.DefaultBaseURL+"/responses", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer grok-access-token", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "application/json, text/event-stream", upstream.lastReq.Header.Get("Accept"))
	require.Equal(t, "sub2api-grok/1.0", upstream.lastReq.Header.Get("User-Agent"))
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(upstream.lastReq.Context()))
	require.Equal(t, "grok-4.3", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "hi", gjson.GetBytes(upstream.lastBody, "input.0.content.0.text").String())
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.False(t, gjson.GetBytes(upstream.lastBody, "messages").Exists())
	require.Contains(t, rec.Body.String(), `"text":"ok"`)
	require.Contains(t, rec.Body.String(), `"success":true`)
}

func TestAccountTestService_TestAccountConnection_GrokOAuth429UsesGrokReconcile(t *testing.T) {
	setGinTestMode()

	account := Account{
		ID:          73978,
		Name:        "grok-oauth-429",
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":  "grok-access-token",
			"refresh_token": "grok-refresh-token",
		},
	}
	repo := &grokTestAccountRepo{stubOpenAIAccountRepo: stubOpenAIAccountRepo{accounts: []Account{account}}}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header: http.Header{
			"Content-Type":                   []string{"application/json"},
			"x-ratelimit-reset-requests":     []string{"30"},
			"x-ratelimit-limit-requests":     []string{"10"},
			"x-ratelimit-remaining-requests": []string{"0"},
		},
		Body: io.NopCloser(strings.NewReader(`{"error":"rate limited"}`)),
	}}
	svc := &AccountTestService{
		accountRepo:       repo,
		grokTokenProvider: NewGrokTokenProvider(repo, nil),
		httpUpstream:      upstream,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/73978/test", bytes.NewReader(nil))

	err := svc.TestAccountConnection(c, account.ID, "grok-4.3", "hello grok", "")
	require.Error(t, err)
	require.Equal(t, 1, repo.tempUnschedCalls)
	require.Equal(t, account.ID, repo.lastTempUnschedID)
	require.Equal(t, "grok rate limited", repo.lastTempUnschedReason)
	require.NotZero(t, repo.lastTempUnschedUntil)
	require.Zero(t, repo.setRateLimitedCalls)
	require.Contains(t, rec.Body.String(), "429")
}
