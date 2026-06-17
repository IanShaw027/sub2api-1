package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAccountTestServiceSendErrorAndEnd_RecordsUpstreamOpsError(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	var captured *OpsInsertErrorLogInput
	repo := &opsRepoMock{
		InsertErrorLogFn: func(ctx context.Context, input *OpsInsertErrorLogInput) (int64, error) {
			captured = input
			return 1, nil
		},
	}
	opsSvc := NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/42/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.RequestID, "req-account-test"))
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.ClientRequestID, "client-account-test"))
	c.Request = req
	c.Set(accountTestOpsAccountIDKey, int64(42))
	c.Set(accountTestOpsPlatformKey, PlatformKiro)
	c.Set(accountTestOpsTypeKey, AccountTypeOAuth)
	c.Set(accountTestOpsNameKey, "Kiro OAuth")
	c.Set(accountTestOpsModelKey, "claude-3-5-sonnet")

	svc := &AccountTestService{opsService: opsSvc}
	err := svc.sendErrorAndEnd(c, "Kiro API returned 400: invalid_request: selected model is not available for this account")

	require.Error(t, err)
	require.NotNil(t, captured)
	require.Equal(t, "req-account-test", captured.RequestID)
	require.Equal(t, "client-account-test", captured.ClientRequestID)
	require.NotNil(t, captured.AccountID)
	require.EqualValues(t, 42, *captured.AccountID)
	require.Equal(t, PlatformKiro, captured.Platform)
	require.Equal(t, "claude-3-5-sonnet", captured.Model)
	require.Equal(t, "/api/v1/admin/accounts/42/test", captured.RequestPath)
	require.Equal(t, "/api/v1/admin/accounts/42/test", captured.InboundEndpoint)
	require.True(t, captured.Stream)
	require.NotNil(t, captured.RequestType)
	require.EqualValues(t, RequestTypeStream, *captured.RequestType)
	require.Equal(t, "upstream", captured.ErrorPhase)
	require.Equal(t, "invalid_request_error", captured.ErrorType)
	require.Equal(t, 400, captured.StatusCode)
	require.NotNil(t, captured.UpstreamStatusCode)
	require.Equal(t, 400, *captured.UpstreamStatusCode)
	require.NotNil(t, captured.UpstreamErrorMessage)
	require.Contains(t, *captured.UpstreamErrorMessage, "selected model is not available")
	require.NotNil(t, captured.UpstreamErrorDetail)
	require.Contains(t, *captured.UpstreamErrorDetail, "invalid_request")
}

func TestAccountTestServiceSendErrorAndEnd_RecordsAuthOpsError(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	var captured *OpsInsertErrorLogInput
	repo := &opsRepoMock{
		InsertErrorLogFn: func(ctx context.Context, input *OpsInsertErrorLogInput) (int64, error) {
			captured = input
			return 1, nil
		},
	}
	opsSvc := NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/7/test", nil)
	c.Set(accountTestOpsAccountIDKey, int64(7))
	c.Set(accountTestOpsPlatformKey, PlatformKiro)

	svc := &AccountTestService{opsService: opsSvc}
	err := svc.sendErrorAndEnd(c, "No Kiro access token available")

	require.Error(t, err)
	require.NotNil(t, captured)
	require.Equal(t, "auth", captured.ErrorPhase)
	require.Equal(t, "authentication_error", captured.ErrorType)
	require.Equal(t, http.StatusUnauthorized, captured.StatusCode)
	require.Equal(t, "account_credentials", captured.ErrorSource)
	require.False(t, captured.IsRetryable)
}

func TestAccountTestServiceSendErrorAndEnd_ClassifiesKiroFrameFailureAsUpstream(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	var captured *OpsInsertErrorLogInput
	repo := &opsRepoMock{
		InsertErrorLogFn: func(ctx context.Context, input *OpsInsertErrorLogInput) (int64, error) {
			captured = input
			return 1, nil
		},
	}
	opsSvc := NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/kiro/test", nil)
	c.Set(accountTestOpsAccountIDKey, int64(66))
	c.Set(accountTestOpsPlatformKey, PlatformKiro)
	c.Set(accountTestOpsModelKey, "claude-sonnet-4-5-20250929")

	svc := &AccountTestService{opsService: opsSvc}
	err := svc.sendErrorAndEnd(c, "kiro upstream returned exception frame: upstream failed")

	require.Error(t, err)
	require.NotNil(t, captured)
	require.Equal(t, "upstream", captured.ErrorPhase)
	require.Equal(t, "upstream_error", captured.ErrorType)
	require.Equal(t, "upstream_http", captured.ErrorSource)
	require.Equal(t, "provider", captured.ErrorOwner)
	require.Equal(t, http.StatusBadGateway, captured.StatusCode)
	require.NotNil(t, captured.UpstreamStatusCode)
	require.Equal(t, http.StatusBadGateway, *captured.UpstreamStatusCode)
	require.NotNil(t, captured.UpstreamErrorMessage)
	require.Contains(t, *captured.UpstreamErrorMessage, "kiro upstream returned exception frame")
}

func TestAccountTestService_RunTestBackground_KiroDefaultModelRecordedInOpsError(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	var captured *OpsInsertErrorLogInput
	repo := &opsRepoMock{
		InsertErrorLogFn: func(ctx context.Context, input *OpsInsertErrorLogInput) (int64, error) {
			captured = input
			return 1, nil
		},
	}
	opsSvc := NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	upstream := &kiroHTTPUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(bytes.NewReader(buildKiroTestFrame(t, map[string]string{
				":event-type":     "exception",
				":exception-type": "RuntimeException",
			}, map[string]any{"message": "upstream failed"}))),
		},
	}
	accountRepo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			901: {
				ID:       901,
				Platform: PlatformKiro,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"api_key": "kiro-api-key",
				},
			},
		},
	}
	svc := &AccountTestService{
		accountRepo:  accountRepo,
		httpUpstream: upstream,
		opsService:   opsSvc,
	}

	result, err := svc.RunTestBackground(context.Background(), 901, "")

	require.Error(t, err)
	require.NotNil(t, result)
	require.Equal(t, "failed", result.Status)
	require.NotNil(t, captured)
	require.Equal(t, "claude-sonnet-4-5-20250929", captured.Model)
	require.Equal(t, "claude-sonnet-4-5-20250929", captured.RequestedModel)
}

func TestAccountTestServiceSendErrorAndEnd_SkipsIgnoredScheduledAccountNotFoundOpsError(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	inserted := 0
	repo := &opsRepoMock{
		InsertErrorLogFn: func(ctx context.Context, input *OpsInsertErrorLogInput) (int64, error) {
			inserted++
			return 1, nil
		},
	}
	settingRepo := newRuntimeSettingRepoStub()
	settingRepo.values[SettingKeyOpsAdvancedSettings] = `{"ignore_account_not_found_errors":true}`
	opsSvc := NewOpsService(repo, settingRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/internal/scheduled-tests/accounts/203/test", nil)
	c.Set(accountTestOpsAccountIDKey, int64(203))
	c.Set(accountTestOpsPlatformKey, PlatformOpenAI)

	svc := &AccountTestService{opsService: opsSvc}
	err := svc.sendErrorAndEnd(c, "Account not found")

	require.Error(t, err)
	require.Equal(t, 0, inserted)
}

func TestAccountTestService_TestAccountConnection_OpenAIDefaultModelRecordedInOpsError(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	var captured *OpsInsertErrorLogInput
	repo := &opsRepoMock{
		InsertErrorLogFn: func(ctx context.Context, input *OpsInsertErrorLogInput) (int64, error) {
			captured = input
			return 1, nil
		},
	}
	opsSvc := NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	accountRepo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			902: {
				ID:       902,
				Platform: PlatformOpenAI,
				Type:     AccountTypeOAuth,
			},
		},
	}
	svc := &AccountTestService{
		accountRepo: accountRepo,
		opsService:  opsSvc,
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/902/test", nil)

	err := svc.TestAccountConnection(c, 902, "", "", AccountTestModeDefault)

	require.Error(t, err)
	require.NotNil(t, captured)
	require.Equal(t, openai.DefaultTestModel, captured.Model)
	require.Equal(t, openai.DefaultTestModel, captured.RequestedModel)
}
