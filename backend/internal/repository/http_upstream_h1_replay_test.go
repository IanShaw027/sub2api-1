package repository

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHTTP1HeaderReplayRoundTripper_PreservesConfiguredHeaderOrderAndCase(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	requestBytes := make(chan string, 1)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, acceptErr := listener.Accept()
		require.NoError(t, acceptErr)
		defer func() { _ = conn.Close() }()

		reader := bufio.NewReader(conn)
		var raw strings.Builder
		for {
			line, readErr := reader.ReadString('\n')
			require.NoError(t, readErr)
			raw.WriteString(line)
			if raw.String() == "" {
				continue
			}
			if strings.Contains(raw.String(), "\r\n\r\n") {
				break
			}
		}
		body := make([]byte, 2)
		_, _ = io.ReadFull(reader, body)
		requestBytes <- raw.String() + string(body)
		_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nok")
	}()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://"+listener.Addr().String()+"/v1/responses?foo=1", strings.NewReader("{}"))
	require.NoError(t, err)
	req.Host = "chatgpt.com"
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("User-Agent", "codex_cli_rs/0.125.0")
	req.Header.Set("Content-Type", "application/json")

	rt := &http1HeaderReplayRoundTripper{
		headerOrder: []string{"authorization", "openai-beta", "user-agent", "content-type"},
	}
	resp, err := rt.RoundTrip(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "ok", string(body))

	var raw string
	select {
	case raw = <-requestBytes:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for raw request bytes")
	}
	<-serverDone

	require.Contains(t, raw, "POST /v1/responses?foo=1 HTTP/1.1\r\n")
	require.Contains(t, raw, "Host: chatgpt.com\r\n")
	idxAuth := strings.Index(raw, "\r\nauthorization: Bearer test-token\r\n")
	idxBeta := strings.Index(raw, "\r\nopenai-beta: responses=experimental\r\n")
	idxUA := strings.Index(raw, "\r\nuser-agent: codex_cli_rs/0.125.0\r\n")
	idxCT := strings.Index(raw, "\r\ncontent-type: application/json\r\n")
	require.NotEqual(t, -1, idxAuth)
	require.NotEqual(t, -1, idxBeta)
	require.NotEqual(t, -1, idxUA)
	require.NotEqual(t, -1, idxCT)
	require.Less(t, idxAuth, idxBeta)
	require.Less(t, idxBeta, idxUA)
	require.Less(t, idxUA, idxCT)
	require.Contains(t, raw, "\r\nContent-Length: 2\r\n")
	require.True(t, strings.HasSuffix(raw, "\r\n\r\n{}"))
}
