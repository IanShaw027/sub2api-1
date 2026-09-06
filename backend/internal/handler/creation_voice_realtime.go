package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const creationVoiceProtocol = "creation-voice"

func (h *CreationHandler) RealtimeTicket(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		key, ok := middleware2.GetAPIKeyFromContext(c)
		if !ok || key == nil || key.Group == nil || key.Group.Platform != service.PlatformGrok {
			imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "Voice requires a Grok group")
			return
		}
		auth := strings.Fields(c.GetHeader("Authorization"))
		if len(auth) != 2 || !strings.EqualFold(auth[0], "Bearer") {
			imageTaskJSONError(c, http.StatusUnauthorized, "authentication_error", "User authentication required")
			return
		}
		ticket, expires, err := h.voiceTickets.Issue(c.Request.Context(), auth[1], key.Group.ID, creationVoiceFingerprint(c))
		if err != nil {
			writeCreationVoiceTicketError(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, gin.H{"ticket": ticket, "expires_at": expires.Unix()})
	})
}

// RealtimeVoiceAuth is registered outside JWT middleware: browsers present an
// in-memory, one-use ticket via the WebSocket subprotocol header instead.
func (h *CreationHandler) RealtimeVoiceAuth(c *gin.Context) {
	if !isOpenAIWSUpgradeRequest(c.Request) || !creationVoiceSameOrigin(c) {
		imageTaskJSONError(c, http.StatusForbidden, "permission_error", "Same-origin WebSocket connection required")
		c.Abort()
		return
	}
	ticket := creationVoiceTicketFromProtocols(c.GetHeader("Sec-WebSocket-Protocol"))
	if ticket == "" {
		writeCreationVoiceTicketError(c, service.ErrCreationVoiceTicketInvalid)
		c.Abort()
		return
	}
	user, record, err := h.voiceTickets.Consume(c.Request.Context(), ticket, creationVoiceFingerprint(c))
	if err != nil {
		writeCreationVoiceTicketError(c, err)
		c.Abort()
		return
	}
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: user.ID, Concurrency: user.Concurrency})
	c.Set(string(middleware2.ContextKeyUserRole), user.Role)
	c.Set(middleware2.ContextKeyAuthEmail, user.Email)
	c.Set(middleware2.ContextKeySessionID, record.SessionID)
	// Group/model selection comes only from the redeemed ticket, never WS query parameters.
	c.Request.URL.RawQuery = ""
	c.Request.Header.Set("X-Group-Id", strconv.FormatInt(record.GroupID, 10))
	c.Request.Header.Set("Sec-WebSocket-Protocol", creationVoiceProtocol)
	c.Request.Header.Del("Authorization")
	c.Next()
}

func (h *CreationHandler) RealtimeVoice(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		key, ok := middleware2.GetAPIKeyFromContext(c)
		if !ok || key == nil || key.Group == nil || key.Group.Platform != service.PlatformGrok {
			imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "Voice requires a Grok group")
			return
		}
		if h.openAI == nil {
			imageTaskJSONError(c, http.StatusServiceUnavailable, "api_error", "Voice gateway unavailable")
			return
		}
		c.Header("Sec-WebSocket-Protocol", creationVoiceProtocol)
		h.openAI.GrokRealtime(c)
	})
}

func creationVoiceSameOrigin(c *gin.Context) bool {
	origin, err := url.Parse(c.GetHeader("Origin"))
	if err != nil || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return false
	}
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return origin.Scheme == scheme && strings.EqualFold(origin.Host, c.Request.Host)
}

func creationVoiceTicketFromProtocols(protocols string) string {
	var ticket string
	protocolFound := false
	for _, raw := range strings.Split(protocols, ",") {
		value := strings.TrimSpace(raw)
		if value == creationVoiceProtocol {
			protocolFound = true
		} else if strings.HasPrefix(value, "ticket.") {
			if ticket != "" {
				return ""
			}
			ticket = strings.TrimPrefix(value, "ticket.")
		}
	}
	if !protocolFound || len(ticket) != 43 {
		return ""
	}
	return ticket
}

func creationVoiceFingerprint(c *gin.Context) string {
	hash := sha256.Sum256([]byte(ip.GetClientIP(c) + "\x00" + c.GetHeader("User-Agent")))
	return hex.EncodeToString(hash[:])
}

func writeCreationVoiceTicketError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrCreationVoiceTicketUnavailable) {
		imageTaskJSONError(c, http.StatusServiceUnavailable, "api_error", "Voice authentication temporarily unavailable")
		return
	}
	imageTaskJSONError(c, http.StatusUnauthorized, "authentication_error", "Voice ticket is invalid or expired")
}
