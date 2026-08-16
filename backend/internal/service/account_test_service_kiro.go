package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/gin-gonic/gin"
)

func (s *AccountTestService) testKiroAccountConnection(c *gin.Context, account *Account, modelID string) error {
	ctx := c.Request.Context()
	testModelID := modelID
	if strings.TrimSpace(testModelID) == "" {
		testModelID = "claude-sonnet-4-5-20250929"
	}
	convertedModelID, err := resolveKiroRequestedModel(account, testModelID)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Unsupported Kiro model: %s", testModelID))
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	s.sendEvent(c, TestEvent{Type: "test_start", Model: testModelID})

	accessToken := account.GetCredential("access_token")
	if account.Type == AccountTypeAPIKey {
		accessToken = account.GetCredential("api_key")
	}
	expiresAt := account.GetCredentialAsTime("expires_at")
	if account.Type == AccountTypeOAuth && (accessToken == "" || expiresAt == nil || expiresAt.Before(time.Now().Add(3*time.Minute))) {
		if s.kiroTokenProvider == nil {
			return s.sendErrorAndEnd(c, "Kiro token provider is not configured")
		}
		refreshedToken, err := s.kiroTokenProvider.GetAccessToken(ctx, account)
		if err != nil {
			return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to refresh Kiro token: %s", err.Error()))
		}
		accessToken = refreshedToken
	}
	if accessToken == "" {
		return s.sendErrorAndEnd(c, "No Kiro access token available")
	}

	payload := map[string]any{
		"model": convertedModelID,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": "hi"},
				},
			},
		},
		"max_tokens": 128,
		"stream":     true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create Kiro test payload")
	}
	body = injectKiroProfileARNIntoAnthropicBody(body, account)
	converted, err := kiropkg.ConvertAnthropicRequestWithModel(body, convertedModelID)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to convert Kiro payload: %s", err.Error()))
	}

	req, err := buildKiroGenerateAssistantRequest(ctx, account, converted.Body, accessToken, s.resolveKiroRuntimeSettings(ctx))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create Kiro request")
	}

	if s.httpUpstream == nil {
		return s.sendErrorAndEnd(c, "HTTP upstream is not configured")
	}
	resp, err := s.httpUpstream.Do(req, accountProxyURL(account), account.ID, account.EffectiveConcurrency())
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Kiro request failed: %s", err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return s.sendErrorAndEnd(c, fmt.Sprintf("Kiro API returned %d", resp.StatusCode))
		}
		return s.sendErrorAndEnd(c, kiroHTTPStatusErrorMessage("Kiro API", resp.StatusCode, body))
	}
	frames, err := readAllKiroFrames(resp.Body)
	if err != nil {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Failed to decode Kiro response: %s", err.Error()))
	}
	for _, frame := range frames {
		if failureErr := kiroFrameFailure(frame); failureErr != nil {
			return s.sendErrorAndEnd(c, failureErr.Error())
		}
	}

	assistantText, hasAssistantResponse := collectKiroAssistantResponseText(frames)
	if !hasAssistantResponse {
		return s.sendErrorAndEnd(c, "Kiro upstream returned no assistant content")
	}
	if trimmed := strings.TrimSpace(assistantText); trimmed != "" {
		s.sendEvent(c, TestEvent{Type: "content", Text: trimmed})
	}
	s.sendEvent(c, TestEvent{Type: "test_complete", Success: true})
	return nil
}

func (s *AccountTestService) resolveKiroRuntimeSettings(ctx context.Context) *KiroRuntimeSettings {
	if s != nil && s.settingService != nil {
		return s.settingService.GetKiroRuntimeSettings(ctx)
	}
	return DefaultKiroRuntimeSettings()
}
