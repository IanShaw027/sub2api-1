package service

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestKiroOAuthServiceExchangeCallbackReturnsActionableSessionExpiredError(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	_, err := svc.ExchangeCallback(context.Background(), &KiroExchangeCallbackInput{
		SessionID:   "missing-session",
		CallbackURL: "http://localhost:3128/oauth/callback?code=code-1&state=state-1&login_option=github",
	})
	if err == nil {
		t.Fatal("expected session expired error")
	}
	if !strings.Contains(err.Error(), "重新生成授权链接") {
		t.Fatalf("expected actionable session expired message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "30 分钟") {
		t.Fatalf("expected session expired message to mention ttl, got %q", err.Error())
	}
}

func TestKiroOAuthServiceExchangeCallbackReturnsActionableStateMismatchError(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	svc.sessionStore.Set("session-state-mismatch", &KiroOAuthSession{
		State:           "expected-state",
		CodeVerifier:    "verifier-1",
		RedirectURI:     "http://localhost:3128",
		CallbackBaseURL: "http://localhost:3128",
		CreatedAt:       time.Now(),
	})

	_, err := svc.ExchangeCallback(context.Background(), &KiroExchangeCallbackInput{
		SessionID:   "session-state-mismatch",
		CallbackURL: "http://localhost:3128/oauth/callback?code=code-1&state=other-state&login_option=github",
	})
	if err == nil {
		t.Fatal("expected state mismatch error")
	}
	if !strings.Contains(err.Error(), "不属于当前这次授权流程") {
		t.Fatalf("expected actionable state mismatch message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "不要刷新页面") {
		t.Fatalf("expected state mismatch message to include next-step guidance, got %q", err.Error())
	}
}

func TestKiroOAuthServiceExchangeCallbackRejectsNilInput(t *testing.T) {
	svc := NewKiroOAuthService(&kiroDefaultProxyRepoStub{}, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	_, err := svc.ExchangeCallback(context.Background(), nil)
	if err == nil {
		t.Fatal("expected nil input error")
	}
	if !strings.Contains(err.Error(), "callback input is required") {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

func TestKiroOAuthServiceGenerateAuthURLFailsCleanlyWhenProxyRepoMissing(t *testing.T) {
	svc := NewKiroOAuthService(nil, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	proxyID := int64(1001)
	_, err := svc.GenerateAuthURL(context.Background(), &proxyID)
	if err == nil {
		t.Fatal("expected proxy repository error")
	}
	if !strings.Contains(err.Error(), "proxy repository is unavailable") {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

func TestKiroOAuthServiceExchangeCallbackFailsCleanlyWhenProxyRepoMissing(t *testing.T) {
	svc := NewKiroOAuthService(nil, nil, nil, nil)
	svc.usageService = nil
	defer svc.Stop()

	svc.sessionStore.Set("session-1", &KiroOAuthSession{
		State:           "state-1",
		CodeVerifier:    "verifier-1",
		RedirectURI:     "http://localhost:3128",
		CallbackBaseURL: "http://localhost:3128",
		CreatedAt:       time.Now(),
	})

	proxyID := int64(1002)
	_, err := svc.ExchangeCallback(context.Background(), &KiroExchangeCallbackInput{
		SessionID:   "session-1",
		CallbackURL: "http://localhost:3128/oauth/callback?code=code-1&state=state-1",
		ProxyID:     &proxyID,
	})
	if err == nil {
		t.Fatal("expected proxy repository error")
	}
	if !strings.Contains(err.Error(), "proxy repository is unavailable") {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}
