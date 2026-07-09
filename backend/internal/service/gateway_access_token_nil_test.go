//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatewayServiceGetAccessTokenRejectsNilAccount(t *testing.T) {
	t.Parallel()

	svc := &GatewayService{}

	token, tokenType, err := svc.GetAccessToken(context.Background(), nil)

	require.Empty(t, token)
	require.Empty(t, tokenType)
	require.EqualError(t, err, "account is required")
}

func TestOpenAIGatewayServiceGetAccessTokenRejectsNilAccount(t *testing.T) {
	t.Parallel()

	svc := &OpenAIGatewayService{}

	token, tokenType, err := svc.GetAccessToken(context.Background(), nil)

	require.Empty(t, token)
	require.Empty(t, tokenType)
	require.EqualError(t, err, "account is required")
}

func TestGatewayServiceDoGrokNativeResponsesJSONRejectsNilAccount(t *testing.T) {
	t.Parallel()

	svc := &GatewayService{httpUpstream: &httpUpstreamRecorder{}}

	body, err := svc.DoGrokNativeResponsesJSON(context.Background(), nil, nil, []byte(`{"model":"grok-4.3"}`))

	require.Nil(t, body)
	require.EqualError(t, err, "account is required")
}

func TestGatewayServiceDoGrokNativeResponsesJSONRejectsNonGrokAccount(t *testing.T) {
	t.Parallel()

	svc := &GatewayService{httpUpstream: &httpUpstreamRecorder{}}
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}

	body, err := svc.DoGrokNativeResponsesJSON(context.Background(), nil, account, []byte(`{"model":"grok-4.3"}`))

	require.Nil(t, body)
	require.EqualError(t, err, "grok account required")
}
