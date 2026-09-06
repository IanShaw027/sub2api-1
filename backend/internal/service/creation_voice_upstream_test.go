package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type creationVoiceHeaderDialer struct{ headers http.Header }

func (d *creationVoiceHeaderDialer) Dial(_ context.Context, _ string, headers http.Header, _ string) (openAIWSClientConn, int, http.Header, error) {
	d.headers = headers.Clone()
	return nil, http.StatusForbidden, nil, errors.New("test stops before network access")
}

func TestCreationVoiceUpstreamDoesNotForwardBrowserTicketHeaders(t *testing.T) {
	dialer := &creationVoiceHeaderDialer{}
	svc := &OpenAIGatewayService{openaiWSPassthroughDialer: dialer}
	account := &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"header_override_enabled": true,
		"header_overrides":        map[string]any{"Sec-WebSocket-Protocol": "creation-voice,ticket.secret", "Authorization": "Bearer user-jwt"},
	}}
	_, err := svc.OpenGrokRealtime(context.Background(), account, "upstream-account-token", "grok-voice-latest")
	require.Error(t, err)
	require.Equal(t, "Bearer upstream-account-token", dialer.headers.Get("Authorization"))
	require.Empty(t, dialer.headers.Get("Sec-WebSocket-Protocol"))
	for _, values := range dialer.headers {
		for _, value := range values {
			require.NotContains(t, value, "ticket.secret")
			require.NotContains(t, value, "user-jwt")
		}
	}
}
