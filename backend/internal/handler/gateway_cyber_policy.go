package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type cyberSessionBlockMarker interface {
	MarkCyberSessionBlocked(context.Context, string)
}

func markCyberSessionBlockedBeforeAsync(c *gin.Context, marker cyberSessionBlockMarker, cyberBlockKey string) {
	if marker == nil || cyberBlockKey == "" {
		return
	}
	parent := context.Background()
	if c != nil && c.Request != nil {
		parent = context.WithoutCancel(c.Request.Context())
	}
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	marker.MarkCyberSessionBlocked(ctx, cyberBlockKey)
}

func (h *GatewayHandler) recordGatewayCyberPolicyIfMarked(c *gin.Context, apiKey *service.APIKey, account *service.Account, subscription *service.UserSubscription, model string, forwardErrored bool, cyberBlockKey string, channelFields service.ChannelUsageFields, requestProtocol string, requestBody []byte) {
	mark := service.GetOpsCyberPolicy(c)
	if mark == nil {
		return
	}
	if c.GetBool(cyberPolicyRecordedKey) {
		return
	}
	c.Set(cyberPolicyRecordedKey, true)

	cmSvc := h.contentModerationService
	gwSvc := h.gatewayService
	opsSvc := h.opsService
	apiKeySvc := h.apiKeyService

	var userID, apiKeyID int64
	var userEmail, apiKeyName, groupName string
	var groupID *int64
	if apiKey != nil {
		apiKeyID = apiKey.ID
		apiKeyName = apiKey.Name
		groupID = apiKey.GroupID
		if apiKey.User != nil {
			userID = apiKey.User.ID
			userEmail = apiKey.User.Email
		}
		if apiKey.Group != nil {
			groupName = apiKey.Group.Name
		}
	}

	requestID := c.Writer.Header().Get("X-Request-Id")
	requestPath := ""
	if c.Request != nil && c.Request.URL != nil {
		requestPath = c.Request.URL.Path
	}
	inboundEndpoint := strings.TrimSpace(GetInboundEndpoint(c))
	if inboundEndpoint == "" {
		inboundEndpoint = requestPath
	}
	upstreamEndpoint := ""
	if account != nil {
		upstreamEndpoint = GetUpstreamEndpoint(c, account.Platform)
	}
	stream := false
	if v, ok := c.Get(opsStreamKey); ok {
		if b, ok := v.(bool); ok {
			stream = b
		}
	}
	var accountID int64
	if account != nil {
		accountID = account.ID
	}
	platform := resolveOpsPlatform(apiKey, guessPlatformFromPath(requestPath))
	var clientRequestID, userAgent, clientIPStr string
	if c.Request != nil {
		clientRequestID, _ = c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		userAgent = c.GetHeader("User-Agent")
		clientIPStr = strings.TrimSpace(ip.GetClientIP(c))
	}
	apiKeyPrefix := ""
	if apiKey != nil {
		apiKeyPrefix = keyPrefix(apiKey.Key, 8)
	}
	requestBodyCopy := append([]byte(nil), requestBody...)
	requestPayloadHash := service.HashUsageRequestPayload(requestBodyCopy)
	quotaPlatform := ""
	if c.Request != nil {
		quotaPlatform = service.QuotaPlatform(c.Request.Context(), apiKey)
	}

	if cmSvc != nil {
		hashParent := context.Background()
		if c.Request != nil {
			hashParent = context.WithoutCancel(c.Request.Context())
		}
		hashCtx, cancel := context.WithTimeout(hashParent, 5*time.Second)
		cmSvc.RecordCyberPolicyFlaggedHashes(hashCtx, requestProtocol, requestBodyCopy, service.ContentModerationHashMeta{})
		cancel()
	}
	opsMeta := cyberPolicyOpsErrorMeta{
		RequestID:       requestID,
		ClientRequestID: clientRequestID,
		Platform:        platform,
		Model:           model,
		RequestPath:     requestPath,
		Stream:          stream,
		InboundEndpoint: inboundEndpoint,
		UserAgent:       userAgent,
		APIKeyPrefix:    apiKeyPrefix,
		UserID:          userID,
		APIKeyID:        apiKeyID,
		AccountID:       accountID,
		GroupID:         groupID,
		ClientIP:        clientIPStr,
		CreatedAt:       time.Now(),
	}
	markCyberSessionBlockedBeforeAsync(c, gwSvc, cyberBlockKey)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if cmSvc != nil {
			cmSvc.RecordCyberPolicyEvent(ctx, service.CyberPolicyRecordInput{
				RequestID:       requestID,
				UserID:          userID,
				UserEmail:       userEmail,
				APIKeyID:        apiKeyID,
				APIKeyName:      apiKeyName,
				GroupID:         groupID,
				GroupName:       groupName,
				Endpoint:        inboundEndpoint,
				Model:           model,
				RequestProtocol: requestProtocol,
				RequestBody:     requestBodyCopy,
				UpstreamMessage: mark.Message,
				UpstreamBody:    mark.Body,
				UpstreamStatus:  mark.UpstreamStatus,
				UpstreamInTok:   mark.UpstreamInTok,
				UpstreamOutTok:  mark.UpstreamOutTok,
				SkipHashRecord:  true,
			})
		}
		if forwardErrored && gwSvc != nil {
			gwSvc.RecordCyberPolicyUsageLog(ctx, service.GatewayCyberPolicyUsageInput{
				APIKey:             apiKey,
				User:               nil,
				Account:            account,
				Subscription:       subscription,
				RequestID:          requestID,
				Model:              model,
				Stream:             stream,
				InputTokens:        mark.UpstreamInTok,
				OutputTokens:       mark.UpstreamOutTok,
				InboundEndpoint:    inboundEndpoint,
				UpstreamEndpoint:   upstreamEndpoint,
				UserAgent:          userAgent,
				IPAddress:          clientIPStr,
				RequestPayloadHash: requestPayloadHash,
				APIKeyService:      apiKeySvc,
				QuotaPlatform:      quotaPlatform,
				ChannelUsageFields: channelFields,
			})
		}
		if opsSvc != nil {
			enqueueOpsErrorLog(opsSvc, buildCyberPolicyOpsErrorEntry(opsMeta, mark))
		}
	}()
}

func (h *GatewayHandler) rejectIfCyberSessionBlocked(c *gin.Context, apiKey *service.APIKey, body []byte, model string, format cyberSessionBlockFormat) bool {
	if h == nil || h.gatewayService == nil || apiKey == nil {
		return false
	}
	if enabled, _ := h.gatewayService.CyberSessionBlockRuntime(c.Request.Context()); !enabled {
		return false
	}
	key := service.CyberSessionBlockKey(apiKey.ID, c, body)
	if key == "" {
		return false
	}
	if !h.gatewayService.IsCyberSessionBlocked(c.Request.Context(), key) {
		return false
	}
	switch format {
	case cyberBlockFormatAnthropic:
		c.JSON(http.StatusForbidden, gin.H{"type": "error", "error": gin.H{
			"type":    "permission_error",
			"message": cyberSessionBlockedClientMsg,
		}})
	default:
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{
			"type":    "permission_error",
			"code":    "session_blocked_by_cyber_policy",
			"message": cyberSessionBlockedClientMsg,
		}})
	}
	h.enqueueCyberSessionBlockedOpsEntry(c, apiKey, model, key)
	return true
}

func (h *GatewayHandler) enqueueCyberSessionBlockedOpsEntry(c *gin.Context, apiKey *service.APIKey, model string, sessionBlockKey string) {
	if h == nil || h.opsService == nil || c == nil {
		return
	}
	requestID := c.Writer.Header().Get("X-Request-Id")
	requestPath := ""
	if c.Request != nil && c.Request.URL != nil {
		requestPath = c.Request.URL.Path
	}
	inboundEndpoint := strings.TrimSpace(GetInboundEndpoint(c))
	if inboundEndpoint == "" {
		inboundEndpoint = requestPath
	}
	stream := false
	if v, ok := c.Get(opsStreamKey); ok {
		if b, ok := v.(bool); ok {
			stream = b
		}
	}
	var clientRequestID, userAgent, clientIPStr string
	if c.Request != nil {
		clientRequestID, _ = c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		userAgent = c.GetHeader("User-Agent")
		clientIPStr = strings.TrimSpace(ip.GetClientIP(c))
	}
	var userID, apiKeyID int64
	var groupID *int64
	apiKeyPrefix := ""
	if apiKey != nil {
		apiKeyID = apiKey.ID
		groupID = apiKey.GroupID
		apiKeyPrefix = keyPrefix(apiKey.Key, 8)
		if apiKey.User != nil {
			userID = apiKey.User.ID
		}
	}
	meta := cyberPolicyOpsErrorMeta{
		RequestID:       requestID,
		ClientRequestID: clientRequestID,
		Platform:        resolveOpsPlatform(apiKey, guessPlatformFromPath(requestPath)),
		Model:           model,
		RequestPath:     requestPath,
		Stream:          stream,
		InboundEndpoint: inboundEndpoint,
		UserAgent:       userAgent,
		APIKeyPrefix:    apiKeyPrefix,
		UserID:          userID,
		APIKeyID:        apiKeyID,
		GroupID:         groupID,
		ClientIP:        clientIPStr,
		CreatedAt:       time.Now(),
		SessionBlockKey: sessionBlockKey,
	}
	enqueueOpsErrorLog(h.opsService, buildCyberSessionBlockedOpsEntry(meta))
}
