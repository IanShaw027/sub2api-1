//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type codexInviteResetAdminServiceStub struct {
	AdminService
	mu             sync.Mutex
	account        *Account
	proxy          *Proxy
	extraUpdates   []map[string]any
	extraUpdateIDs []int64
}

func (s *codexInviteResetAdminServiceStub) GetAccount(ctx context.Context, id int64) (*Account, error) {
	return s.account, nil
}

func (s *codexInviteResetAdminServiceStub) GetProxy(ctx context.Context, id int64) (*Proxy, error) {
	return s.proxy, nil
}

func (s *codexInviteResetAdminServiceStub) UpdateAccountExtra(ctx context.Context, id int64, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make(map[string]any, len(updates))
	for k, v := range updates {
		copied[k] = v
	}
	s.extraUpdateIDs = append(s.extraUpdateIDs, id)
	s.extraUpdates = append(s.extraUpdates, copied)
	return nil
}

// codexInviteResetHistoryRepoStub 记录写入的历史条目，供断言。
type codexInviteResetHistoryRepoStub struct {
	mu      sync.Mutex
	entries []*CodexInviteResetHistoryEntry
}

func (r *codexInviteResetHistoryRepoStub) Create(ctx context.Context, entry *CodexInviteResetHistoryEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entry)
	return nil
}

func (r *codexInviteResetHistoryRepoStub) ListByAccount(ctx context.Context, accountID int64, params pagination.PaginationParams) ([]CodexInviteResetHistoryEntry, *pagination.PaginationResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]CodexInviteResetHistoryEntry, 0, len(r.entries))
	for _, e := range r.entries {
		if e.AccountID == accountID {
			items = append(items, *e)
		}
	}
	return items, &pagination.PaginationResult{Total: int64(len(items)), Page: params.Page, PageSize: params.PageSize}, nil
}

type codexInviteResetHTTPUpstreamStub struct {
	mu sync.Mutex
	// responsesByPath 按 URL path 路由响应，用于并发请求场景（GetStatus）。
	responsesByPath map[string]*http.Response
	// responses 按调用顺序弹出，用于单请求场景（SendInvite/Consume）。
	responses []*http.Response
	requests  []*http.Request
	bodies    []string
	profiles  []*tlsfingerprint.Profile
}

func (s *codexInviteResetHTTPUpstreamStub) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return s.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (s *codexInviteResetHTTPUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	body := ""
	if req.Body != nil {
		payload, _ := io.ReadAll(req.Body)
		body = string(payload)
		req.Body = io.NopCloser(strings.NewReader(body))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, req)
	s.bodies = append(s.bodies, body)
	s.profiles = append(s.profiles, profile)
	if s.responsesByPath != nil {
		if resp, ok := s.responsesByPath[req.URL.Path]; ok {
			return resp, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	}
	if len(s.responses) == 0 {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	}
	resp := s.responses[0]
	s.responses = s.responses[1:]
	return resp, nil
}

// requestByPath 在记录的请求中按 URL path 查找（并发请求顺序不确定时使用）。
func (s *codexInviteResetHTTPUpstreamStub) requestByPath(path string) *http.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, req := range s.requests {
		if req.URL.Path == path {
			return req
		}
	}
	return nil
}

func TestCodexInviteResetServiceGetStatusAggregatesDesktopEndpoints(t *testing.T) {
	account := &Account{
		ID:          42,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 3,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}
	upstream := &codexInviteResetHTTPUpstreamStub{responsesByPath: map[string]*http.Response{
		"/backend-api/referrals/invite/eligibility":     codexInviteResetJSONResponse(`{"requires_explicit_confirmation":true}`),
		"/backend-api/wham/referrals/eligibility_rules": codexInviteResetJSONResponse(`{"rules":[{"text":"friend must send first Codex message"}]}`),
		"/backend-api/wham/rate-limit-reset-credits":    codexInviteResetJSONResponse(`{"available_count":2,"credits":[{"id":"credit-1","status":"available","title":"Reset"},{"id":"credit-2","status":"available"}]}`),
	}}
	adminSvc := &codexInviteResetAdminServiceStub{account: account}
	svc := NewCodexInviteResetService(adminSvc, upstream, nil, nil, &codexInviteResetHistoryRepoStub{})

	status, err := svc.GetStatus(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, codexInviteResetReferralKey, status.ReferralKey)
	require.Equal(t, 2, status.AvailableCount)
	require.True(t, status.RequiresConsent)
	require.Len(t, status.Credits, 2)
	require.Equal(t, "friend must send first Codex message", status.EligibilityRules[0])

	require.Len(t, upstream.requests, 3)
	// 三个请求并发执行，到达顺序不确定，按 path 查找而非索引断言。
	eligibilityReq := upstream.requestByPath("/backend-api/referrals/invite/eligibility")
	require.NotNil(t, eligibilityReq)
	require.Equal(t, codexInviteResetReferralKey, eligibilityReq.URL.Query().Get("referral_key"))
	require.NotNil(t, upstream.requestByPath("/backend-api/wham/referrals/eligibility_rules"))
	require.NotNil(t, upstream.requestByPath("/backend-api/wham/rate-limit-reset-credits"))
	require.Equal(t, "Bearer oauth-token", eligibilityReq.Header.Get("Authorization"))
	require.Equal(t, "Codex Desktop", eligibilityReq.Header.Get("originator"))
	require.Equal(t, codexInviteResetDefaultUserAgent, eligibilityReq.Header.Get("User-Agent"))
	require.Equal(t, "1", eligibilityReq.Header.Get("X-OpenAI-Attach-Auth"))
	require.Equal(t, "1", eligibilityReq.Header.Get("X-OpenAI-Attach-Integrity-State"))
	require.Equal(t, "chatgpt-acc", eligibilityReq.Header.Get("chatgpt-account-id"))
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(eligibilityReq.Context()))

	require.Len(t, adminSvc.extraUpdates, 1)
	require.Equal(t, account.ID, adminSvc.extraUpdateIDs[0])
	require.Equal(t, 2, adminSvc.extraUpdates[0]["codex_invite_reset_available_count"])
	require.Equal(t, []string{"credit-1", "credit-2"}, adminSvc.extraUpdates[0]["codex_invite_reset_credit_ids"])
	require.NotEmpty(t, adminSvc.extraUpdates[0]["codex_invite_reset_updated_at"])
}

func TestCodexInviteResetServiceUsesDesktopTLSRouter(t *testing.T) {
	account := &Account{
		ID:          44,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 3,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Extra: map[string]any{
			"enable_tls_fingerprint":    true,
			"tls_fingerprint_router_id": float64(10),
		},
	}
	upstream := &codexInviteResetHTTPUpstreamStub{responsesByPath: map[string]*http.Response{
		"/backend-api/referrals/invite/eligibility":     codexInviteResetJSONResponse(`{"requires_explicit_confirmation":true}`),
		"/backend-api/wham/referrals/eligibility_rules": codexInviteResetJSONResponse(`{"rules":[]}`),
		"/backend-api/wham/rate-limit-reset-credits":    codexInviteResetJSONResponse(`{"available_count":1}`),
	}}
	routerSvc := NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{
		{
			ID:      10,
			Name:    "openai clients",
			Enabled: true,
			Rules: []model.TLSFingerprintRouterRule{
				{
					Name:                    "desktop",
					Enabled:                 true,
					Transport:               model.TLSFingerprintRouterTransportHTTP,
					MatchType:               model.TLSFingerprintRouterMatchPrefix,
					Pattern:                 "Codex Desktop/",
					TLSFingerprintProfileID: 7,
					UpstreamUserAgent:       "Codex Desktop/26.616.71553 (Mac OS X 15.5; arm64)",
					UpstreamOriginator:      "Codex Desktop",
				},
			},
		},
	}}, nil)
	profileSvc := &TLSFingerprintProfileService{
		localCache: map[int64]*model.TLSFingerprintProfile{
			7: {
				ID:            7,
				Name:          "Codex Desktop Routed",
				ALPNProtocols: []string{"h2", "http/1.1"},
			},
		},
	}
	svc := NewCodexInviteResetService(&codexInviteResetAdminServiceStub{account: account}, upstream, nil, profileSvc, &codexInviteResetHistoryRepoStub{})
	svc.SetTLSFingerprintRouterService(routerSvc)

	_, err := svc.GetStatus(context.Background(), account.ID)
	require.NoError(t, err)

	require.Len(t, upstream.profiles, 3)
	for _, profile := range upstream.profiles {
		require.NotNil(t, profile)
		require.Equal(t, "Codex Desktop Routed", profile.Name)
	}
	req := upstream.requestByPath("/backend-api/wham/rate-limit-reset-credits")
	require.NotNil(t, req)
	require.Equal(t, "Codex Desktop/26.616.71553 (Mac OS X 15.5; arm64)", req.Header.Get("User-Agent"))
	require.Equal(t, "Codex Desktop", req.Header.Get("originator"))
}

func TestCodexInviteResetServiceRouterRejectsWebSocketOnlyProfileForHTTP(t *testing.T) {
	account := &Account{
		ID:          45,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 3,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Extra: map[string]any{
			"enable_tls_fingerprint":     true,
			"tls_fingerprint_router_id":  float64(10),
			"tls_fingerprint_profile_id": float64(8),
		},
	}
	upstream := &codexInviteResetHTTPUpstreamStub{responsesByPath: map[string]*http.Response{
		"/backend-api/referrals/invite/eligibility":     codexInviteResetJSONResponse(`{"requires_explicit_confirmation":true}`),
		"/backend-api/wham/referrals/eligibility_rules": codexInviteResetJSONResponse(`{"rules":[]}`),
		"/backend-api/wham/rate-limit-reset-credits":    codexInviteResetJSONResponse(`{"available_count":1}`),
	}}
	routerSvc := NewTLSFingerprintRouterService(&tlsFingerprintRouterRepoStub{routers: []*model.TLSFingerprintRouter{
		{
			ID:      10,
			Name:    "openai clients",
			Enabled: true,
			Rules: []model.TLSFingerprintRouterRule{
				{
					Name:                    "desktop",
					Enabled:                 true,
					Transport:               model.TLSFingerprintRouterTransportHTTP,
					MatchType:               model.TLSFingerprintRouterMatchPrefix,
					Pattern:                 "Codex Desktop/",
					TLSFingerprintProfileID: 7,
					UpstreamUserAgent:       "Codex Desktop/26.616.71553 (Mac OS X 15.5; arm64)",
					UpstreamOriginator:      "Codex Desktop",
				},
			},
		},
	}}, nil)
	profileSvc := &TLSFingerprintProfileService{
		localCache: map[int64]*model.TLSFingerprintProfile{
			7: {
				ID:        7,
				Name:      "WebSocket Only",
				Platform:  PlatformOpenAI,
				Transport: model.TLSFingerprintRouterTransportWSH2,
			},
			8: {
				ID:               8,
				Name:             "HTTP Account Default",
				Platform:         PlatformOpenAI,
				Transport:        model.TLSFingerprintRouterTransportH2,
				UserAgent:        "Account Default HTTP UA",
				Originator:       "Account Default",
				HTTP2Fingerprint: "1:65536|2:0|3:1000|4:6291456|6:262144|8:0|9:1|:method,:authority,:scheme,:path",
				ALPNProtocols:    []string{"h2", "http/1.1"},
			},
		},
	}
	svc := NewCodexInviteResetService(&codexInviteResetAdminServiceStub{account: account}, upstream, nil, profileSvc, &codexInviteResetHistoryRepoStub{})
	svc.SetTLSFingerprintRouterService(routerSvc)

	_, err := svc.GetStatus(context.Background(), account.ID)
	require.NoError(t, err)

	require.Len(t, upstream.profiles, 3)
	for _, profile := range upstream.profiles {
		require.NotNil(t, profile)
		require.Equal(t, "HTTP Account Default", profile.Name)
	}
	req := upstream.requestByPath("/backend-api/wham/rate-limit-reset-credits")
	require.NotNil(t, req)
	require.Equal(t, "Account Default HTTP UA", req.Header.Get("User-Agent"))
	require.Equal(t, "Account Default", req.Header.Get("originator"))
}

func TestCodexInviteResetServiceGetStatusFallsBackToUsageCount(t *testing.T) {
	account := &Account{
		ID:          43,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 3,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
	}
	upstream := &codexInviteResetHTTPUpstreamStub{responsesByPath: map[string]*http.Response{
		"/backend-api/referrals/invite/eligibility":     codexInviteResetErrorResponse(http.StatusServiceUnavailable, `cf challenge`),
		"/backend-api/wham/referrals/eligibility_rules": codexInviteResetErrorResponse(http.StatusBadGateway, `bad gateway`),
		"/backend-api/wham/rate-limit-reset-credits":    codexInviteResetErrorResponse(http.StatusNotFound, `not found`),
		"/backend-api/wham/usage":                       codexInviteResetJSONResponse(`{"rate_limit_reset_credits":{"available_count":4}}`),
	}}
	adminSvc := &codexInviteResetAdminServiceStub{account: account}
	svc := NewCodexInviteResetService(adminSvc, upstream, nil, nil, &codexInviteResetHistoryRepoStub{})

	status, err := svc.GetStatus(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, 4, status.AvailableCount)
	require.Empty(t, status.Credits)
	require.True(t, status.RequiresConsent)
	require.NotNil(t, upstream.requestByPath("/backend-api/wham/usage"))

	require.Len(t, adminSvc.extraUpdates, 1)
	require.Equal(t, 4, adminSvc.extraUpdates[0]["codex_invite_reset_available_count"])
}

func TestCodexInviteResetServiceSendInviteNormalizesEmails(t *testing.T) {
	account := &Account{
		ID:          7,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "oauth-token"},
	}
	upstream := &codexInviteResetHTTPUpstreamStub{responses: []*http.Response{
		codexInviteResetJSONResponse(`{"invites":[{"email":"a@example.com"}],"message":"ok"}`),
	}}
	historyRepo := &codexInviteResetHistoryRepoStub{}
	svc := NewCodexInviteResetService(&codexInviteResetAdminServiceStub{account: account}, upstream, nil, nil, historyRepo)

	operatorID := int64(1001)
	result, err := svc.SendInvite(context.Background(), account.ID, []string{"a@example.com, b@example.com", "A@example.com"}, &operatorID)
	require.NoError(t, err)
	require.Equal(t, "ok", result.Message)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "/backend-api/wham/referrals/invite", upstream.requests[0].URL.Path)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(upstream.bodies[0]), &payload))
	require.Equal(t, codexInviteResetReferralKey, payload["referral_key"])
	require.Equal(t, []any{"a@example.com", "b@example.com"}, payload["emails"])

	// 成功邀请写入一条 invite 历史，记录操作人和归一化后的邮箱。
	require.Len(t, historyRepo.entries, 1)
	entry := historyRepo.entries[0]
	require.Equal(t, CodexInviteResetActionInvite, entry.ActionType)
	require.True(t, entry.Success)
	require.Equal(t, []string{"a@example.com", "b@example.com"}, entry.Emails)
	require.NotNil(t, entry.OperatorUserID)
	require.Equal(t, operatorID, *entry.OperatorUserID)
}

func TestCodexInviteResetServiceConsumeSendsRedeemRequestID(t *testing.T) {
	account := &Account{
		ID:          9,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "oauth-token"},
	}
	upstream := &codexInviteResetHTTPUpstreamStub{responses: []*http.Response{
		codexInviteResetJSONResponse(`{"code":"reset","available_count":0}`),
	}}
	historyRepo := &codexInviteResetHistoryRepoStub{}
	svc := NewCodexInviteResetService(&codexInviteResetAdminServiceStub{account: account}, upstream, nil, nil, historyRepo)

	result, err := svc.Consume(context.Background(), account.ID, "credit-1", nil)
	require.NoError(t, err)
	require.Equal(t, "reset", result.Code)
	require.Equal(t, "credit-1", result.CreditID)
	require.NotEmpty(t, result.RedeemRequestID)
	require.NotNil(t, result.AvailableCount)
	require.Equal(t, 0, *result.AvailableCount)

	var payload map[string]string
	require.NoError(t, json.Unmarshal([]byte(upstream.bodies[0]), &payload))
	require.Equal(t, "credit-1", payload["credit_id"])
	require.Equal(t, result.RedeemRequestID, payload["redeem_request_id"])

	// 重置写入一条 consume 历史，code=reset 视为成功；operator 为 nil（系统触发）。
	require.Len(t, historyRepo.entries, 1)
	entry := historyRepo.entries[0]
	require.Equal(t, CodexInviteResetActionConsume, entry.ActionType)
	require.Equal(t, "credit-1", entry.CreditID)
	require.Equal(t, "reset", entry.ResultCode)
	require.True(t, entry.Success)
	require.Nil(t, entry.OperatorUserID)
}

func TestCodexInviteResetServiceConsumeAllowsEmptyCreditID(t *testing.T) {
	account := &Account{
		ID:          10,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "oauth-token"},
	}
	upstream := &codexInviteResetHTTPUpstreamStub{responses: []*http.Response{
		codexInviteResetJSONResponse(`{"code":"reset","windows_reset":2,"available_count":1}`),
	}}
	historyRepo := &codexInviteResetHistoryRepoStub{}
	svc := NewCodexInviteResetService(&codexInviteResetAdminServiceStub{account: account}, upstream, nil, nil, historyRepo)

	result, err := svc.Consume(context.Background(), account.ID, "", nil)
	require.NoError(t, err)
	require.Equal(t, "reset", result.Code)
	require.Empty(t, result.CreditID)
	require.NotEmpty(t, result.RedeemRequestID)
	require.NotNil(t, result.AvailableCount)
	require.Equal(t, 1, *result.AvailableCount)

	var payload map[string]string
	require.NoError(t, json.Unmarshal([]byte(upstream.bodies[0]), &payload))
	require.Empty(t, payload["credit_id"])
	require.NotEmpty(t, payload["redeem_request_id"])

	require.Len(t, historyRepo.entries, 1)
	entry := historyRepo.entries[0]
	require.Equal(t, CodexInviteResetActionConsume, entry.ActionType)
	require.Empty(t, entry.CreditID)
	require.Equal(t, "reset", entry.ResultCode)
	require.True(t, entry.Success)
}

func TestNormalizeCodexInviteEmailsRejectsInvalidAndTooMany(t *testing.T) {
	_, err := normalizeCodexInviteEmails([]string{"bad-email"})
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))

	_, err = normalizeCodexInviteEmails([]string{"a@x.com,b@x.com,c@x.com,d@x.com,e@x.com,f@x.com"})
	require.Error(t, err)
	require.Equal(t, "CODEX_INVITE_RESET_EMAIL_LIMIT", infraerrors.Reason(err))
}

func codexInviteResetJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func codexInviteResetErrorResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
