//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAICyberPolicyAccountRepo struct {
	mockAccountRepoForGemini
	setErrorID  int64
	setErrorMsg string
}

func (r *openAICyberPolicyAccountRepo) SetError(_ context.Context, id int64, errorMsg string) error {
	r.setErrorID = id
	r.setErrorMsg = errorMsg
	return nil
}

func TestOpenAINonStreamingSSECyberPolicyMarksAccountError(t *testing.T) {
	repo := &openAICyberPolicyAccountRepo{}
	rateLimitSvc := NewRateLimitService(repo, nil, nil, nil, nil)
	svc := &OpenAIGatewayService{rateLimitService: rateLimitSvc}
	account := &Account{ID: 321, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	body := []byte("event: response.failed\n" +
		`data: {"type":"response.failed","response":{"error":{"code":"cyber_policy","message":"policy denied"}}}` +
		"\n\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)

	_, err := svc.handleSSEToJSON(resp, c, account, body, "gpt-5.4", "gpt-5.4")

	require.Error(t, err)
	require.Equal(t, account.ID, repo.setErrorID)
	require.Contains(t, repo.setErrorMsg, "被风控命中(cyber_policy)")
	require.Contains(t, repo.setErrorMsg, "policy denied")
}

func TestOpenAICyberPolicySkipsPoolModeWithoutCustomErrorCodes(t *testing.T) {
	repo := &openAICyberPolicyAccountRepo{}
	rateLimitSvc := NewRateLimitService(repo, nil, nil, nil, nil)
	account := &Account{
		ID:       654,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":   "sk-test",
			"pool_mode": true,
		},
	}

	disabled := rateLimitSvc.HandleUpstreamError(
		context.Background(),
		account,
		http.StatusForbidden,
		nil,
		[]byte(`{"error":{"code":"cyber_policy","message":"policy denied"}}`),
	)

	require.False(t, disabled)
	require.Zero(t, repo.setErrorID)
	require.Empty(t, repo.setErrorMsg)
}
