package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyCodexHeaderWireCasing_InstallationIDUsesWireKeyNotCanonicalDuplicate(t *testing.T) {
	h := make(http.Header)
	h.Set("x-codex-installation-id", "install-1")
	h.Set("x-codex-window-id", "window-1")
	h.Set("x-codex-turn-metadata", `{"sandbox":"seccomp"}`)
	h.Set("x-codex-turn-state", "active")
	h.Set("version", "0.146.0")
	h.Set("user-agent", "codex-tui/0.146.0")
	h.Set("authorization", "Bearer tok")
	h.Set("content-type", "application/json")
	h.Set("accept", "text/event-stream")
	setHeaderRaw(h, "originator", openai.CodexDefaultOriginator)

	require.Contains(t, h, "X-Codex-Installation-Id", "precondition: Go Set stores the canonical key")

	applyCodexHeaderWireCasing(h)

	require.Contains(t, h, "x-codex-installation-id")
	_, hasCanonical := h["X-Codex-Installation-Id"]
	require.False(t, hasCanonical, "wire apply must not leave a Go-canonical duplicate")
	require.Equal(t, "install-1", h["x-codex-installation-id"][0])

	require.Contains(t, h, "x-codex-window-id")
	_, hasWindowCanonical := h["X-Codex-Window-Id"]
	require.False(t, hasWindowCanonical)

	require.Contains(t, h, "originator")
	require.Equal(t, openai.CodexDefaultOriginator, h["originator"][0])
	_, hasXOriginator := h["X-Originator"]
	require.False(t, hasXOriginator)
	_, hasOriginatorTitle := h["Originator"]
	require.False(t, hasOriginatorTitle)
}

func TestApplyCodexHeaderWireCasing_DoesNotRewriteUncapturedKeys(t *testing.T) {
	h := make(http.Header)
	h.Set("conversation_id", "conv-1")
	h.Set("chatgpt-account-id", "acc-1")
	h.Set("content-type", "application/json")
	h.Set("accept", "text/event-stream")
	h.Set("version", "0.146.0")
	require.Contains(t, h, "Conversation_id")
	require.Contains(t, h, "Chatgpt-Account-Id")
	require.Contains(t, h, "Content-Type")
	require.Contains(t, h, "Accept")
	require.Contains(t, h, "Version")

	applyCodexHeaderWireCasing(h)

	require.Contains(t, h, "Conversation_id")
	require.Equal(t, "conv-1", h["Conversation_id"][0])
	_, hasConvLower := h["conversation_id"]
	require.False(t, hasConvLower, "uncaptured conversation_id must keep the builder's emitted key")

	require.Contains(t, h, "Chatgpt-Account-Id")
	require.Equal(t, "acc-1", h["Chatgpt-Account-Id"][0])
	_, hasChatLower := h["chatgpt-account-id"]
	require.False(t, hasChatLower, "uncaptured chatgpt-account-id must keep the builder's emitted key")

	require.Contains(t, h, "Content-Type")
	require.Equal(t, "application/json", h["Content-Type"][0])
	_, hasCTLower := h["content-type"]
	require.False(t, hasCTLower, "Go Set already emits Content-Type; do not rewrite to lowercase")

	require.Contains(t, h, "Accept")
	_, hasAcceptLower := h["accept"]
	require.False(t, hasAcceptLower, "Go Set already emits Accept; do not rewrite to lowercase")

	require.Contains(t, h, "Version")
	_, hasVersionLower := h["version"]
	require.False(t, hasVersionLower, "Go Set already emits Version; do not rewrite to lowercase")
}

func TestApplyCodexHeaderWireCasing_OriginatorStaysLowercaseRawKey(t *testing.T) {
	h := make(http.Header)
	h.Set("X-Originator", "legacy")
	setHeaderRaw(h, "originator", "codex-tui")

	applyCodexHeaderWireCasing(h)

	require.Contains(t, h, "originator")
	require.Equal(t, "codex-tui", h["originator"][0])
	require.Equal(t, "codex-tui", getHeaderRaw(h, "originator"))
	_, hasXOriginator := h["X-Originator"]
	require.False(t, hasXOriginator)
}

func TestBuildUpstreamRequest_OAuthForwardAppliesCodexWireCasing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := leftoverOAuthAccount(1940, nil)
	profile := leftoverValidProfile(account.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex-tui/0.146.0 (Mac OS X 14.0; arm64) iTerm", "mid-wire")
	installLeftoverOutboundProfile(t, profile)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("session-id", "client-session")
	c.Request.Header.Set("x-codex-turn-state", "active")

	applyCodexForwardRequestIdentity(context.Background(), c, map[string]any{}, account, c.Request.Header)

	svc := &OpenAIGatewayService{}
	req, err := svc.buildUpstreamRequest(context.Background(), c, account, []byte(`{"model":"gpt-5.4"}`), "oauth-token", true, "cache-key", true)
	require.NoError(t, err)
	require.NotNil(t, req)
	applyCodexHeaderWireCasing(req.Header)

	require.Contains(t, req.Header, "x-codex-installation-id")
	_, hasCanonical := req.Header["X-Codex-Installation-Id"]
	require.False(t, hasCanonical)
	require.Equal(t, profile.InstallationID, req.Header["x-codex-installation-id"][0])

	require.Contains(t, req.Header, "originator")
	require.Equal(t, strings.ToLower(getHeaderRaw(req.Header, "originator")), getHeaderRaw(req.Header, "originator"))
	_, hasXOriginator := req.Header["X-Originator"]
	require.False(t, hasXOriginator)

	// leftover 5 session/full still overwrites native /responses session_id
	// with account-stable empty-tail DeriveSessionIDs.
	require.Equal(t, resolveConvergedSessionID(profile.SessionNamespace), getHeaderRaw(req.Header, "session_id"))
	require.Equal(t, resolveConvergedSessionID(profile.SessionNamespace), getHeaderRaw(req.Header, "session-id"))
	require.Contains(t, req.Header, "session-id")
}

func TestBuildUpstreamRequest_DoesNotApplyCodexWireCasing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := leftoverOAuthAccount(1941, nil)
	profile := leftoverValidProfile(account.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex-tui/0.146.0 (Mac OS X 14.0; arm64) iTerm", "mid-wire-late")
	installLeftoverOutboundProfile(t, profile)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("session-id", "client-session")
	applyCodexForwardRequestIdentity(context.Background(), c, map[string]any{}, account, c.Request.Header)

	svc := &OpenAIGatewayService{}
	req, err := svc.buildUpstreamRequest(context.Background(), c, account, []byte(`{"model":"gpt-5.4"}`), "oauth-token", true, "cache-key", true)
	require.NoError(t, err)

	require.Contains(t, req.Header, "X-Codex-Installation-Id", "apply must wait until after callers Header.Set/Get")
	_, hasWire := req.Header["x-codex-installation-id"]
	require.False(t, hasWire)
	require.NotEmpty(t, req.Header.Get("conversation_id"))
}

func TestImagesBridgePostBuildSetHasSingleJSONContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := leftoverOAuthAccount(1942, nil)
	profile := leftoverValidProfile(account.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex-tui/0.146.0 (Mac OS X 14.0; arm64) iTerm", "mid-img")
	installLeftoverOutboundProfile(t, profile)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=abc")
	c.Request.Header.Set("session-id", "client-session")
	applyCodexForwardRequestIdentity(context.Background(), c, map[string]any{}, account, c.Request.Header)

	svc := &OpenAIGatewayService{}
	req, err := svc.buildUpstreamRequest(context.Background(), c, account, []byte(`{"model":"gpt-5.4"}`), "oauth-token", true, "cache-key", false)
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	keys, values := headerValuesFold(req.Header, "content-type")
	require.Equal(t, 1, keys, "image bridge must not keep multipart raw content-type beside Content-Type")
	require.Equal(t, []string{"application/json"}, values)
	require.NotContains(t, values, "multipart/form-data; boundary=abc")

	applyCodexHeaderWireCasing(req.Header)
	keys, values = headerValuesFold(req.Header, "content-type")
	require.Equal(t, 1, keys)
	require.Equal(t, []string{"application/json"}, values)
}

func TestMessagesBridgePostBuildGetDoesNotDuplicateTurnState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := leftoverOAuthAccount(1943, nil)
	profile := leftoverValidProfile(account.ID, PlatformOpenAI, ClientFamilyCodexCLI, "codex-tui/0.146.0 (Mac OS X 14.0; arm64) iTerm", "mid-msg")
	installLeftoverOutboundProfile(t, profile)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Request.Header.Set("x-codex-turn-state", "from-client")
	c.Request.Header.Set("conversation_id", "client-conv")
	c.Request.Header.Set("session-id", "client-session")
	applyCodexForwardRequestIdentity(context.Background(), c, map[string]any{}, account, c.Request.Header)

	svc := &OpenAIGatewayService{}
	req, err := svc.buildUpstreamRequest(context.Background(), c, account, []byte(`{"model":"gpt-5.4"}`), "oauth-token", true, "cache-key", false)
	require.NoError(t, err)
	require.NotEmpty(t, req.Header.Get("conversation_id"), "Header.Get(conversation_id) must still see the isolated value")

	if req.Header.Get("x-codex-turn-state") == "" {
		req.Header.Set("x-codex-turn-state", "from-session")
	}

	keys, values := headerValuesFold(req.Header, "x-codex-turn-state")
	require.Equal(t, 1, keys)
	require.Equal(t, []string{"from-client"}, values)

	applyCodexHeaderWireCasing(req.Header)
	keys, values = headerValuesFold(req.Header, "x-codex-turn-state")
	require.Equal(t, 1, keys)
	require.Equal(t, []string{"from-client"}, values)
}

func headerValuesFold(h http.Header, name string) (keys int, values []string) {
	for k, vs := range h {
		if strings.EqualFold(k, name) {
			keys++
			values = append(values, vs...)
		}
	}
	return keys, values
}
