package service

import (
	"context"
	"encoding/json"
	"mime"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func (s *OpenAIGatewayService) normalizeOpenAICompatibleFingerprintJSONBody(ctx context.Context, c *gin.Context, account *Account, body []byte, endpoint string) []byte {
	if s == nil || s.fingerprintNormalizer == nil || len(strings.TrimSpace(string(body))) == 0 {
		return body
	}
	if !openAICompatibleRequestAllowsJSONBodyNormalization(c) || !json.Valid(body) {
		return body
	}
	ua := ""
	if c != nil && c.Request != nil {
		ua = c.Request.Header.Get("User-Agent")
	}
	canonical := s.fingerprintNormalizer.ResolveCanonical(ctx, account, ua)
	if canonical == nil {
		return body
	}
	_, normalizedBody, err := s.fingerprintNormalizer.ApplyToRequest(nil, body, canonical)
	if err != nil {
		logger.L().Debug("openai fingerprint body normalization skipped",
			zap.String("endpoint", endpoint),
			zap.Error(err),
		)
		return body
	}
	if len(normalizedBody) == 0 {
		return body
	}
	return normalizedBody
}

func openAICompatibleRequestAllowsJSONBodyNormalization(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return true
	}
	contentType := strings.TrimSpace(c.GetHeader("Content-Type"))
	if contentType == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return strings.Contains(strings.ToLower(contentType), "json")
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}
