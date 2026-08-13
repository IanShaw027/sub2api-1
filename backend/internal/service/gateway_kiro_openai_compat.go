package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/gin-gonic/gin"
)

func (s *GatewayService) forwardKiroAnthropicCapture(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	sourceBody []byte,
	anthropicBody []byte,
	model string,
) (*http.Response, *ForwardResult, error) {
	if s == nil || s.kiroGatewayService == nil {
		return nil, nil, errors.New("kiro gateway service is not configured")
	}

	parsed, err := ParseGatewayRequest(NewRequestBodyRef(anthropicBody), domain.PlatformAnthropic)
	if err != nil || parsed == nil {
		parsed = &ParsedRequest{}
	}
	parsed.Model = strings.TrimSpace(model)
	parsed.Body = NewRequestBodyRef(anthropicBody)
	parsed.Stream = true
	applyKiroRequestScopeFromContext(c, parsed)
	anthropicBody, parsed = ensureKiroOpenAICompatSessionMetadata(sourceBody, c, parsed, anthropicBody)

	rec := httptest.NewRecorder()
	tempCtx, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/v1/messages", bytes.NewReader(anthropicBody))
	if c != nil && c.Request != nil {
		req.Header = c.Request.Header.Clone()
	}
	tempCtx.Request = req

	result, forwardErr := s.kiroGatewayService.Forward(ctx, tempCtx, account, parsed)

	headers := rec.Header().Clone()
	if result != nil && strings.TrimSpace(result.RequestID) != "" {
		headers.Set("x-request-id", result.RequestID)
		headers.Set("x-amzn-requestid", result.RequestID)
	}

	statusCode := rec.Code
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	resp := &http.Response{
		StatusCode: statusCode,
		Header:     headers,
		Body:       io.NopCloser(bytes.NewReader(rec.Body.Bytes())),
	}

	return resp, result, forwardErr
}

func kiroUpstreamModel(result *ForwardResult, fallback string) string {
	if result != nil && strings.TrimSpace(result.UpstreamModel) != "" {
		return strings.TrimSpace(result.UpstreamModel)
	}
	return strings.TrimSpace(fallback)
}

func ensureKiroOpenAICompatSessionMetadata(
	sourceBody []byte,
	c *gin.Context,
	parsed *ParsedRequest,
	anthropicBody []byte,
) ([]byte, *ParsedRequest) {
	if parsed == nil {
		parsed = &ParsedRequest{}
	}
	sessionSeed := KiroExplicitSessionSeed(
		sourceBody,
		headerValueFromContext(c, "X-Claude-Code-Session-Id"),
		headerValueFromContext(c, "session_id"),
		headerValueFromContext(c, "conversation_id"),
	)
	if sessionSeed == "" {
		return anthropicBody, parsed
	}
	updatedBody, metadataUserID, changed := EnsureKiroMetadataUserIDForSession(anthropicBody, parsed.MetadataUserID, sessionSeed)
	if !changed {
		return anthropicBody, parsed
	}
	parsed.Body = NewRequestBodyRef(updatedBody)
	parsed.MetadataUserID = metadataUserID
	return updatedBody, parsed
}

func applyKiroRequestScopeFromContext(c *gin.Context, parsed *ParsedRequest) {
	if c == nil || parsed == nil {
		return
	}
	value, ok := c.Get("api_key")
	if !ok {
		return
	}
	apiKey, ok := value.(*APIKey)
	if !ok || apiKey == nil {
		return
	}
	if apiKey.UserID > 0 {
		parsed.UserID = apiKey.UserID
	} else if apiKey.User != nil {
		parsed.UserID = apiKey.User.ID
	}
	if apiKey.ID > 0 {
		parsed.APIKeyID = apiKey.ID
	}
}

func headerValueFromContext(c *gin.Context, key string) string {
	if c == nil || c.Request == nil {
		return ""
	}
	return c.Request.Header.Get(key)
}
