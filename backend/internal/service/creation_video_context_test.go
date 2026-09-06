//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreationVideoSubmissionAndAcceptanceBudgetsSurviveBrowserCancellation(t *testing.T) {
	type identityKey struct{}
	browser, stopBrowser := context.WithCancel(context.WithValue(context.Background(), identityKey{}, "identity"))
	submission, stopSubmission := NewCreationVideoSubmissionContext(browser)
	defer stopSubmission()
	stopBrowser()
	require.NoError(t, submission.Err())
	require.Equal(t, "identity", submission.Value(identityKey{}))
	deadline, ok := submission.Deadline()
	require.True(t, ok)
	require.InDelta(t, CreationVideoSubmissionTimeout.Seconds(), time.Until(deadline).Seconds(), 1)
	upstream, release := grokMediaUpstreamContext(submission)
	defer release()
	upstreamDeadline, ok := upstream.Deadline()
	require.True(t, ok)
	require.Equal(t, deadline, upstreamDeadline)
	stopSubmission()
	require.ErrorIs(t, upstream.Err(), context.Canceled)
	acceptance, stopAcceptance := CreationVideoAcceptanceContext(submission)
	defer stopAcceptance()
	require.NoError(t, acceptance.Err(), "an already accepted task still needs durable owner/pricing writes")
	acceptedDeadline, ok := acceptance.Deadline()
	require.True(t, ok)
	require.InDelta(t, CreationVideoAcceptancePersistenceTimeout.Seconds(), time.Until(acceptedDeadline).Seconds(), 1)
	// Include the reservation round trip itself, not only the upstream request.
	require.Less(t, 5*time.Second+CreationVideoSubmissionTimeout+CreationVideoAcceptancePersistenceTimeout+CreationVideoBindTimeout, CreationVideoReservationTTL)
}

func TestCreationVideoObserverCancellationStopsAnUpstreamBodyAfterHeaders(t *testing.T) {
	releaseServer := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-releaseServer:
		}
	}))
	defer server.Close()
	defer close(releaseServer)
	poll, stop := context.WithCancel(WithCreationVideoTaskContext(context.Background()))
	defer stop()
	upstream, cleanup := grokMediaUpstreamContext(poll)
	defer cleanup()
	request, err := http.NewRequestWithContext(upstream, http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	response, err := server.Client().Do(request)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()
	result := make(chan error, 1)
	go func() { _, readErr := io.ReadAll(response.Body); result <- readErr }()
	stop()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("cancelled observer still occupies its upstream connection")
	}
}

func TestCreationVideoContextDoesNotChangeNativeGatewayDetachment(t *testing.T) {
	native, cancel := context.WithCancel(context.Background())
	upstream, release := grokMediaUpstreamContext(native)
	defer release()
	acceptance, finish := CreationVideoAcceptanceContext(native)
	defer finish()
	cancel()
	require.NoError(t, upstream.Err())
	require.ErrorIs(t, acceptance.Err(), context.Canceled)
}
