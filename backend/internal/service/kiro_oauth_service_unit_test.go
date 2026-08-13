//go:build unit

package service

import (
	"context"
	"net/http"
	"runtime"
	"testing"
	"time"
)

func TestKiroOAuthServiceCompleteDeviceAuthorizationLocksContinuationDuringSlowDown(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	originalComplete := kiroIDCCompleteDeviceAuthorizationFunc
	t.Cleanup(func() {
		kiroIDCCompleteDeviceAuthorizationFunc = originalComplete
	})
	kiroIDCCompleteDeviceAuthorizationFunc = func(ctx context.Context, input kiroIDCCompleteDeviceAuthorizationInput) (map[string]any, error) {
		return nil, &kiroIDCAPIError{
			Operation:        "kiro idc create token",
			StatusCode:       http.StatusBadRequest,
			ErrorCode:        "slow_down",
			ErrorDescription: "poll more slowly",
		}
	}

	session := &KiroOAuthSession{
		State:           "state-1",
		CodeVerifier:    "verifier-1",
		RedirectURI:     "http://localhost:3128",
		CallbackBaseURL: "http://localhost:3128",
		CreatedAt:       time.Now(),
		IDCContinuation: &KiroIDCContinuationSession{
			LoginOption:     "awsidc",
			ClientID:        "client-1",
			ClientSecret:    "secret-1",
			DeviceCode:      "device-code-1",
			Region:          "us-east-1",
			IssuerURL:       "https://oidc.us-east-1.amazonaws.com",
			Scopes:          []string{"openid", "profile"},
			LoginHint:       "dev@example.com",
			IntervalSeconds: 5,
			ExpiresAt:       time.Now().Add(10 * time.Minute),
			CreatedAt:       time.Now(),
		},
	}
	svc.sessionStore.Set("session-device-slow-down-race", session)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			session.mu.Lock()
			if session.IDCContinuation != nil {
				session.IDCContinuation.IntervalSeconds++
				session.IDCContinuation.ExpiresAt = time.Now().Add(10 * time.Minute)
			}
			session.mu.Unlock()
			runtime.Gosched()
		}
	}()
	defer func() {
		close(stop)
		<-done
	}()

	for i := 0; i < 200; i++ {
		progress, err := svc.CompleteDeviceAuthorization(context.Background(), &KiroDeviceCompleteInput{
			SessionID: "session-device-slow-down-race",
		})
		if err != nil {
			t.Fatalf("CompleteDeviceAuthorization returned error: %v", err)
		}
		if progress == nil || progress.Continuation == nil {
			t.Fatal("expected continuation result")
		}
		if progress.Continuation.Status != "authorization_pending" {
			t.Fatalf("continuation status = %q, want authorization_pending", progress.Continuation.Status)
		}
	}
}
