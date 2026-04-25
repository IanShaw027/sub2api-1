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
