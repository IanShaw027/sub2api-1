package repository

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type captureHTTPUpstreamFailureSink struct {
	mu       sync.Mutex
	failures []service.OpsUpstreamFailure
}

func (s *captureHTTPUpstreamFailureSink) EnqueueOpsUpstreamFailure(_ context.Context, failure service.OpsUpstreamFailure) {
	s.mu.Lock()
	s.failures = append(s.failures, failure)
	s.mu.Unlock()
}

func (s *captureHTTPUpstreamFailureSink) snapshot() []service.OpsUpstreamFailure {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]service.OpsUpstreamFailure(nil), s.failures...)
}

func TestHTTPUpstreamReportsEveryHTTPErrorAttempt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"overloaded"}`))
	}))
	defer server.Close()

	upstream := NewHTTPUpstream(nil)
	setter, ok := upstream.(service.HTTPUpstreamFailureSinkSetter)
	require.True(t, ok)
	sink := &captureHTTPUpstreamFailureSink{}
	setter.SetOpsUpstreamFailureSink(sink)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/v1/responses", nil)
	require.NoError(t, err)
	resp, err := upstream.Do(req, "", 123, 1)
	require.NoError(t, err)
	require.NotNil(t, resp)
	_, _ = io.Copy(io.Discard, resp.Body)
	require.NoError(t, resp.Body.Close())

	failures := sink.snapshot()
	require.Len(t, failures, 1)
	require.Equal(t, int64(123), failures[0].AccountID)
	require.Equal(t, http.StatusServiceUnavailable, failures[0].StatusCode)
	require.Equal(t, "http_attempt", failures[0].Kind)
	require.JSONEq(t, `{"error":"overloaded"}`, failures[0].ResponseBody)
}

func TestHTTPUpstreamReportsTransportErrorAttempt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	serverURL := server.URL
	server.Close()

	upstream := NewHTTPUpstream(nil)
	setter := upstream.(service.HTTPUpstreamFailureSinkSetter)
	sink := &captureHTTPUpstreamFailureSink{}
	setter.SetOpsUpstreamFailureSink(sink)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, serverURL+"/v1/models", nil)
	require.NoError(t, err)
	resp, err := upstream.Do(req, "", 456, 1)
	require.Error(t, err)
	require.Nil(t, resp)

	failures := sink.snapshot()
	require.Len(t, failures, 1)
	require.Equal(t, int64(456), failures[0].AccountID)
	require.Error(t, failures[0].Err)
}

func TestHTTPUpstreamUsesOpenAIProfileForCustomBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	upstream := NewHTTPUpstream(nil)
	setter := upstream.(service.HTTPUpstreamFailureSinkSetter)
	sink := &captureHTTPUpstreamFailureSink{}
	setter.SetOpsUpstreamFailureSink(sink)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/custom/responses", nil)
	require.NoError(t, err)
	req = req.WithContext(service.WithHTTPUpstreamProfile(req.Context(), service.HTTPUpstreamProfileOpenAI))
	resp, err := upstream.Do(req, "", 789, 1)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	failures := sink.snapshot()
	require.Len(t, failures, 1)
	require.Equal(t, service.PlatformOpenAI, failures[0].Platform)
}
