package handler

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

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
		cmSvc.RecordCyberPolicyFlaggedHashes(hashCtx, requestProtocol, requestBodyCopy)
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
		if gwSvc != nil && cyberBlockKey != "" {
			gwSvc.MarkCyberSessionBlocked(ctx, cyberBlockKey)
		}
		if opsSvc != nil {
			enqueueOpsErrorLog(opsSvc, buildCyberPolicyOpsErrorEntry(opsMeta, mark))
		}
	}()
}
