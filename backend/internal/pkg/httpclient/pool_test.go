package httpclient

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidatedDialContext_PinsResolvedIPAddress(t *testing.T) {
	originalLookup := validatedLookupIPAddr
	originalBaseDial := validatedBaseDialContext
	defer func() {
		validatedLookupIPAddr = originalLookup
		validatedBaseDialContext = originalBaseDial
	}()

	var lookupCalls int32
	validatedLookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		lookupCalls++
		require.Equal(t, "api.openai.com", host)
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}}, nil
	}

	var dialedAddr string
	validatedBaseDialContext = func(_ context.Context, _, addr string) (net.Conn, error) {
		dialedAddr = addr
		left, right := net.Pipe()
		go func() { _ = right.Close() }()
		return left, nil
	}

	transport, err := buildTransport(Options{ValidateResolvedIP: true})
	require.NoError(t, err)

	conn, err := transport.DialContext(context.Background(), "tcp", "api.openai.com:443")
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	require.Equal(t, "203.0.113.10:443", dialedAddr)
	require.EqualValues(t, 1, lookupCalls)
}

func TestValidatedDialContext_CachesResolvedIPAddress(t *testing.T) {
	originalLookup := validatedLookupIPAddr
	originalBaseDial := validatedBaseDialContext
	defer func() {
		validatedLookupIPAddr = originalLookup
		validatedBaseDialContext = originalBaseDial
	}()

	var lookupCalls int32
	validatedLookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		lookupCalls++
		require.Equal(t, "api.openai.com", host)
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.11")}}, nil
	}

	var dialed []string
	validatedBaseDialContext = func(_ context.Context, _, addr string) (net.Conn, error) {
		dialed = append(dialed, addr)
		left, right := net.Pipe()
		go func() { _ = right.Close() }()
		return left, nil
	}

	transport, err := buildTransport(Options{ValidateResolvedIP: true})
	require.NoError(t, err)

	conn, err := transport.DialContext(context.Background(), "tcp", "api.openai.com:443")
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	conn, err = transport.DialContext(context.Background(), "tcp", "api.openai.com:443")
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	require.EqualValues(t, 1, lookupCalls)
	require.Equal(t, []string{"203.0.113.11:443", "203.0.113.11:443"}, dialed)
}

func TestValidatedDialContext_RejectsUnsafeResolvedIP(t *testing.T) {
	originalLookup := validatedLookupIPAddr
	originalBaseDial := validatedBaseDialContext
	defer func() {
		validatedLookupIPAddr = originalLookup
		validatedBaseDialContext = originalBaseDial
	}()

	validatedLookupIPAddr = func(_ context.Context, _ string) ([]net.IPAddr, error) {
		return []net.IPAddr{
			{IP: net.ParseIP("203.0.113.12")},
			{IP: net.ParseIP("127.0.0.1")},
		}, nil
	}

	called := false
	validatedBaseDialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
		called = true
		return nil, errors.New("unexpected dial")
	}

	transport, err := buildTransport(Options{ValidateResolvedIP: true})
	require.NoError(t, err)

	_, err = transport.DialContext(context.Background(), "tcp", "api.openai.com:443")
	require.Error(t, err)
	require.False(t, called)
}

func TestValidatedDialContext_ExpiredCacheRevalidates(t *testing.T) {
	dialed := make([]string, 0, 2)
	now := time.Unix(1730000000, 0)

	vt := &validatedTransport{
		base: func(_ context.Context, _, addr string) (net.Conn, error) {
			dialed = append(dialed, addr)
			left, right := net.Pipe()
			go func() { _ = right.Close() }()
			return left, nil
		},
		lookup: func(_ context.Context, host string) ([]net.IPAddr, error) {
			require.Equal(t, "api.openai.com", host)
			switch len(dialed) {
			case 0:
				return []net.IPAddr{{IP: net.ParseIP("203.0.113.20")}}, nil
			default:
				return []net.IPAddr{{IP: net.ParseIP("203.0.113.21")}}, nil
			}
		},
		now: func() time.Time { return now },
	}

	conn, err := vt.DialContext(context.Background(), "tcp", "api.openai.com:443")
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	now = now.Add(validatedHostTTL + time.Second)

	conn, err = vt.DialContext(context.Background(), "tcp", "api.openai.com:443")
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	require.Equal(t, []string{"203.0.113.20:443", "203.0.113.21:443"}, dialed)
}
