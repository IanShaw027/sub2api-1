//go:build unit

package service

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestIsPrivateOrLoopbackHost_LocalhostBlocked(t *testing.T) {
	blocked, err := isPrivateOrLoopbackHost(context.Background(), "localhost")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !blocked {
		t.Fatal("localhost must be blocked")
	}
}

func TestIsPrivateOrLoopbackHost_PublicIPAllowed(t *testing.T) {
	blocked, err := isPrivateOrLoopbackHost(context.Background(), "8.8.8.8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if blocked {
		t.Fatal("public literal IP should be allowed")
	}
}

func TestIsPrivateIP_BlocksSpecialUseNetworks(t *testing.T) {
	for _, raw := range []string{
		"198.18.0.1",
		"192.0.2.1",
		"198.51.100.1",
		"203.0.113.1",
		"240.0.0.1",
		"64:ff9b::0a00:1",
		"2001:2::1",
	} {
		if !isPrivateIP(net.ParseIP(raw)) {
			t.Fatalf("expected %s to be blocked", raw)
		}
	}
}

func TestValidateEndpoint_BlocksSpecialUseLiteralIP(t *testing.T) {
	if err := validateEndpoint("https://198.18.0.1"); !errors.Is(err, ErrChannelMonitorEndpointPrivate) {
		t.Fatalf("expected private endpoint error, got %v", err)
	}
}

func TestSafeDialContext_BlocksPrivateLiteralIP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := safeDialContext(ctx, "tcp", "127.0.0.1:443")
	if err == nil {
		t.Fatal("expected private literal IP to be blocked")
	}
	var addrErr *net.AddrError
	if ok := errors.As(err, &addrErr); !ok {
		t.Fatalf("expected *net.AddrError, got %T (%v)", err, err)
	}
}
