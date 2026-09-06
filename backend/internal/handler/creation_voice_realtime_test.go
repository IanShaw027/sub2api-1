package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCreationVoiceRequiresSameOriginWebSocket(t *testing.T) {
	for _, test := range []struct {
		origin, forwarded string
		allowed           bool
	}{
		{"http://localhost:8080", "", true}, {"https://localhost:8080", "https", true},
		{"https://other.example", "https", false}, {"null", "", false},
		{"", "", false}, {"http://localhost:8080", "https", false},
		{"http://localhost:8080/path", "", false},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/v1/creation/audio/realtime", nil)
		c.Request.Header.Set("Origin", test.origin)
		c.Request.Header.Set("X-Forwarded-Proto", test.forwarded)
		require.Equal(t, test.allowed, creationVoiceSameOrigin(c), test.origin)
	}
}

func TestCreationVoiceTicketIsAcceptedOnlyInExpectedSubprotocol(t *testing.T) {
	ticket := strings.Repeat("a", 43)
	require.Equal(t, ticket, creationVoiceTicketFromProtocols("creation-voice, ticket."+ticket))
	for _, protocols := range []string{"", "jwt.secret", "ticket." + ticket, "creation-voice, ticket.short", "creation-voice, ticket." + ticket + ", ticket." + ticket} {
		require.Empty(t, creationVoiceTicketFromProtocols(protocols))
	}
}

func TestCreationVoiceAuthDoesNotAcceptQueryTokensOrCrossOrigin(t *testing.T) {
	for _, origin := range []string{"http://localhost:8080", "http://attacker.example"} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodGet, "http://localhost:8080/api/v1/creation/audio/realtime?ticket="+strings.Repeat("a", 43), nil)
		c.Request.Header.Set("Origin", origin)
		c.Request.Header.Set("Upgrade", "websocket")
		c.Request.Header.Set("Connection", "Upgrade")
		(&CreationHandler{}).RealtimeVoiceAuth(c)
		require.True(t, c.IsAborted())
		require.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, recorder.Code)
	}
}
