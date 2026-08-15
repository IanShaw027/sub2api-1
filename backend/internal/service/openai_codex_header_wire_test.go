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

func TestSortCodexHeadersByWireOrder_ReturnsKnownKeysInDeclaredOrder(t *testing.T) {
	h := make(http.Header)
	h["x-codex-turn-state"] = []string{"active"}
	h["originator"] = []string{openai.CodexDefaultOriginator}
	h["User-Agent"] = []string{"codex-tui/0.146.0"}
	h["x-codex-installation-id"] = []string{"install-1"}
	h["version"] = []string{"0.146.0"}
	h["session-id"] = []string{"sess-1"}
	h["unknown-extra"] = []string{"keep"}

	got := sortCodexHeadersByWireOrder(h)

	require.Equal(t, []string{
		"User-Agent",
		"originator",
		"version",
		"session-id",
		"x-codex-installation-id",
		"x-codex-turn-state",
		"unknown-extra",
	}, got)
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
