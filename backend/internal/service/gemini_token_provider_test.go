//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/stretchr/testify/require"
)

func TestGeminiTokenProvider_GetAccessToken_BackfillsProjectIDWhenAutoDetectFlagSet(t *testing.T) {
	t.Parallel()

	account := &Account{
		ID:       101,
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":           "access-token",
			"oauth_type":             "code_assist",
			"auto_detect_project_id": "true",
		},
	}

	repo := &refreshAPIAccountRepo{account: account}
	oauthService := &GeminiOAuthService{
		codeAssist: &mockGeminiCodeAssistClient{
			loadCodeAssistFunc: func(ctx context.Context, accessToken, proxyURL string, req *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
				return &geminicli.LoadCodeAssistResponse{
					CloudAICompanionProject: "detected-project",
					CurrentTier:             &geminicli.TierInfo{ID: "STANDARD"},
				}, nil
			},
		},
	}

	provider := NewGeminiTokenProvider(repo, nil, oauthService)

	token, err := provider.GetAccessToken(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, "access-token", token)
	require.Equal(t, "detected-project", account.GetCredential("project_id"))
	require.Equal(t, "STANDARD", account.GetCredential("tier_id"))
	require.Equal(t, 1, repo.updateCredentialsCalls)
}
