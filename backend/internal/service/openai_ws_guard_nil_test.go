package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type openAIWSGuardHTTPUpstreamStub struct{}

func (openAIWSGuardHTTPUpstreamStub) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return nil, nil
}

func (openAIWSGuardHTTPUpstreamStub) DoWithTLS(_ *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return nil, nil
}

func TestNewOpenAIWSTLSFingerprintHTTPClient_RequiresProfile(t *testing.T) {
	client, err := newOpenAIWSTLSFingerprintHTTPClient("", nil)
	require.Nil(t, client)
	require.EqualError(t, err, "tls fingerprint profile is required")
}

func TestCoderOpenAIWSClientDialerProxyHTTPClient_RequiresDialer(t *testing.T) {
	client, err := (*coderOpenAIWSClientDialer)(nil).proxyHTTPClient("http://proxy.example.com")
	require.Nil(t, client)
	require.EqualError(t, err, "openai ws dialer is required")
}

func TestOpenAIWSConnPoolDialConn_RequiresClientDialer(t *testing.T) {
	conn, err := (*openAIWSConnPool)(nil).dialConn(context.Background(), openAIWSAcquireRequest{})
	require.Nil(t, conn)
	require.EqualError(t, err, "openai ws client dialer is required")
}

func TestProxyOpenAIWSHTTPBridgeTurn_RequiresService(t *testing.T) {
	result, err := (*OpenAIGatewayService)(nil).proxyOpenAIWSHTTPBridgeTurn(
		context.Background(),
		nil,
		nil,
		"",
		nil,
		0,
		"",
		"",
		"",
		"",
		0,
		func([]byte) error { return nil },
	)
	require.Nil(t, result)
	require.EqualError(t, err, "service is required")
}

func TestProxyOpenAIWSHTTPBridgeTurn_RequiresHTTPUpstream(t *testing.T) {
	result, err := (&OpenAIGatewayService{}).proxyOpenAIWSHTTPBridgeTurn(
		context.Background(),
		nil,
		nil,
		"",
		nil,
		0,
		"",
		"",
		"",
		"",
		0,
		func([]byte) error { return nil },
	)
	require.Nil(t, result)
	require.EqualError(t, err, "openai http upstream is required")
}

func TestProxyOpenAIWSHTTPBridgeTurn_RequiresClientWriter(t *testing.T) {
	result, err := (&OpenAIGatewayService{httpUpstream: openAIWSGuardHTTPUpstreamStub{}}).proxyOpenAIWSHTTPBridgeTurn(
		context.Background(),
		nil,
		&Account{ID: 1},
		"",
		nil,
		0,
		"",
		"",
		"",
		"",
		0,
		nil,
	)
	require.Nil(t, result)
	require.EqualError(t, err, "client websocket writer is required")
}

func TestProxyResponsesWebSocketV2Passthrough_RequiresService(t *testing.T) {
	err := (*OpenAIGatewayService)(nil).proxyResponsesWebSocketV2Passthrough(
		context.Background(),
		nil,
		nil,
		nil,
		"",
		nil,
		nil,
		OpenAIWSProtocolDecision{},
	)
	require.EqualError(t, err, "service is required")
}

func TestProxyResponsesWebSocketV2Passthrough_RequiresClientWebsocket(t *testing.T) {
	err := (&OpenAIGatewayService{}).proxyResponsesWebSocketV2Passthrough(
		context.Background(),
		nil,
		nil,
		nil,
		"",
		nil,
		nil,
		OpenAIWSProtocolDecision{},
	)
	require.EqualError(t, err, "client websocket is required")
}

func TestRequireOpenAIWSHTTPBridgeTurnResult_RequiresResult(t *testing.T) {
	err := requireOpenAIWSHTTPBridgeTurnResult(nil)
	require.EqualError(t, err, "websocket http bridge turn result is required")

	require.NoError(t, requireOpenAIWSHTTPBridgeTurnResult(&OpenAIForwardResult{}))
}

func TestRequireOpenAIWSLease_RequiresLease(t *testing.T) {
	err := requireOpenAIWSLease(nil)
	require.EqualError(t, err, "upstream websocket lease is required")

	require.NoError(t, requireOpenAIWSLease(&openAIWSConnLease{}))
}

func TestRequireOpenAIWSTurnResult_RequiresResult(t *testing.T) {
	err := requireOpenAIWSTurnResult(nil)
	require.EqualError(t, err, "websocket turn result is required")

	require.NoError(t, requireOpenAIWSTurnResult(&OpenAIForwardResult{}))
}
