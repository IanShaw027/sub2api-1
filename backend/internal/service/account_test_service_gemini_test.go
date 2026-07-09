//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCreateGeminiTestPayload_ImageModel(t *testing.T) {
	t.Parallel()

	payload := createGeminiTestPayload("gemini-2.5-flash-image", "draw a tiny robot")

	var parsed struct {
		Contents []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"contents"`
		GenerationConfig struct {
			ResponseModalities []string `json:"responseModalities"`
			ImageConfig        struct {
				AspectRatio string `json:"aspectRatio"`
			} `json:"imageConfig"`
		} `json:"generationConfig"`
	}

	require.NoError(t, json.Unmarshal(payload, &parsed))
	require.Len(t, parsed.Contents, 1)
	require.Len(t, parsed.Contents[0].Parts, 1)
	require.Equal(t, "draw a tiny robot", parsed.Contents[0].Parts[0].Text)
	require.Equal(t, []string{"TEXT", "IMAGE"}, parsed.GenerationConfig.ResponseModalities)
	require.Equal(t, "1:1", parsed.GenerationConfig.ImageConfig.AspectRatio)
}

func TestProcessGeminiStream_EmitsImageEvent(t *testing.T) {
	t.Parallel()
	setGinTestMode()

	ctx, recorder := newTestContext()
	svc := &AccountTestService{}

	stream := strings.NewReader("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"ok\"},{\"inlineData\":{\"mimeType\":\"image/png\",\"data\":\"QUJD\"}}]}}]}\n\ndata: [DONE]\n\n")

	err := svc.processGeminiStream(ctx, stream)
	require.NoError(t, err)

	body := recorder.Body.String()
	require.Contains(t, body, "\"type\":\"content\"")
	require.Contains(t, body, "\"text\":\"ok\"")
	require.Contains(t, body, "\"type\":\"image\"")
	require.Contains(t, body, "\"image_url\":\"data:image/png;base64,QUJD\"")
	require.Contains(t, body, "\"mime_type\":\"image/png\"")
}

func TestAccount_GeminiOAuthType_DoesNotInferFromProjectID(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"project_id": "project-1",
		},
	}

	require.Equal(t, "", account.GeminiOAuthType())
	require.False(t, account.IsGeminiCodeAssist())
	require.False(t, account.HasExplicitGeminiOAuthType())
}

type testGeminiTokenProvider struct {
	token string
	err   error
}

func (p *testGeminiTokenProvider) GetAccessToken(_ context.Context, _ *Account) (string, error) {
	if p.err != nil {
		return "", p.err
	}
	return p.token, nil
}

func TestAccountTestService_BuildGeminiOAuthRequest_UnknownProjectIDStaysAIStudio(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{
		geminiTokenProvider: &testGeminiTokenProvider{token: "gemini-token"},
		cfg:                 &config.Config{},
	}
	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"project_id": "legacy-project",
			"base_url":   "https://generativelanguage.googleapis.com",
		},
	}

	req, err := svc.buildGeminiOAuthRequest(context.Background(), account, "gemini-2.5-pro", []byte(`{"contents":[]}`))
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Contains(t, req.URL.String(), "/v1beta/models/gemini-2.5-pro:streamGenerateContent")
	require.Equal(t, "Bearer gemini-token", req.Header.Get("Authorization"))
}

func TestAccountTestService_BuildGeminiOAuthRequest_NilConfigUsesSafeURLValidationDefaults(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{
		geminiTokenProvider: &testGeminiTokenProvider{token: "gemini-token"},
	}
	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"base_url": "https://generativelanguage.googleapis.com",
		},
	}

	req, err := svc.buildGeminiOAuthRequest(context.Background(), account, "gemini-2.5-pro", []byte(`{"contents":[]}`))
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Contains(t, req.URL.String(), "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro:streamGenerateContent")
	require.Equal(t, "Bearer gemini-token", req.Header.Get("Authorization"))
}

func TestAccountTestService_BuildGeminiOAuthRequest_ExplicitCodeAssistRequiresProjectID(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{
		geminiTokenProvider: &testGeminiTokenProvider{token: "gemini-token"},
		cfg:                 &config.Config{},
	}
	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"oauth_type": "code_assist",
		},
	}

	req, err := svc.buildGeminiOAuthRequest(context.Background(), account, "gemini-2.5-pro", []byte(`{"contents":[]}`))
	require.Error(t, err)
	require.Nil(t, req)
	require.Contains(t, err.Error(), "project_id not configured")
}

func TestAccountTestService_BuildGeminiOAuthRequest_ExplicitCodeAssistUsesCodeAssistEndpoint(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{
		geminiTokenProvider: &testGeminiTokenProvider{token: "gemini-token"},
		cfg:                 &config.Config{},
	}
	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"oauth_type": "code_assist",
			"project_id": "proj-1",
		},
	}

	req, err := svc.buildGeminiOAuthRequest(context.Background(), account, "gemini-2.5-pro", []byte(`{"contents":[]}`))
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Contains(t, req.URL.String(), "cloudcode-pa.googleapis.com")
}

func TestAccountTestService_BuildGeminiOAuthRequest_ExplicitGoogleOneWithoutProjectIDUsesAIStudioEndpoint(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{
		geminiTokenProvider: &testGeminiTokenProvider{token: "gemini-token"},
		cfg:                 &config.Config{},
	}
	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"oauth_type": "google_one",
		},
	}

	req, err := svc.buildGeminiOAuthRequest(context.Background(), account, "gemini-2.5-pro", []byte(`{"contents":[]}`))
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Contains(t, req.URL.String(), "/v1beta/models/gemini-2.5-pro:streamGenerateContent")
	require.NotContains(t, req.URL.String(), "cloudcode-pa.googleapis.com")
	require.Equal(t, "Bearer gemini-token", req.Header.Get("Authorization"))
}

func TestAccountTestService_BuildGeminiOAuthRequest_ExplicitGoogleOneWithProjectIDUsesAIStudioEndpoint(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{
		geminiTokenProvider: &testGeminiTokenProvider{token: "gemini-token"},
		cfg:                 &config.Config{},
	}
	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"oauth_type": "google_one",
			"project_id": "proj-1",
		},
	}

	req, err := svc.buildGeminiOAuthRequest(context.Background(), account, "gemini-2.5-pro", []byte(`{"contents":[]}`))
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Contains(t, req.URL.String(), "/v1beta/models/gemini-2.5-pro:streamGenerateContent")
	require.NotContains(t, req.URL.String(), "cloudcode-pa.googleapis.com")
	require.Equal(t, "Bearer gemini-token", req.Header.Get("Authorization"))
}

func TestAccountTestService_BuildGeminiOAuthRequest_TokenErrorBubbles(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{
		geminiTokenProvider: &testGeminiTokenProvider{err: fmt.Errorf("token failed")},
		cfg:                 &config.Config{},
	}
	account := &Account{
		Platform: PlatformGemini,
		Type:     AccountTypeOAuth,
	}

	req, err := svc.buildGeminiOAuthRequest(context.Background(), account, "gemini-2.5-pro", []byte(`{"contents":[]}`))
	require.Error(t, err)
	require.Nil(t, req)
	require.Contains(t, err.Error(), "failed to get access token")
}

var _ interface {
	GetAccessToken(context.Context, *Account) (string, error)
} = (*testGeminiTokenProvider)(nil)
