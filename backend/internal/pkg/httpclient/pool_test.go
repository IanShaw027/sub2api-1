package httpclient

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
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
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
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

	require.Equal(t, "93.184.216.34:443", dialedAddr)
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
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.35")}}, nil
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
	require.Equal(t, []string{"93.184.216.35:443", "93.184.216.35:443"}, dialed)
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
			{IP: net.ParseIP("93.184.216.36")},
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
				return []net.IPAddr{{IP: net.ParseIP("93.184.216.37")}}, nil
			default:
				return []net.IPAddr{{IP: net.ParseIP("93.184.216.38")}}, nil
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

	require.Equal(t, []string{"93.184.216.37:443", "93.184.216.38:443"}, dialed)
}

func TestResolveValidatedIPBlocksSpecialUseNetworks(t *testing.T) {
	tests := []string{
		"100.64.0.1",
		"198.18.0.1",
		"224.0.0.1",
		"240.0.0.1",
		"255.255.255.255",
	}
	for _, ip := range tests {
		t.Run(ip, func(t *testing.T) {
			_, err := resolveValidatedIP(context.Background(), ip, nil)
			require.Error(t, err)
		})
	}
}

func TestResolveValidatedIPBlocksIPv6SpecialUseNetworks(t *testing.T) {
	tests := []string{
		"64:ff9b::0a00:1",
		"64:ff9b:1::1",
		"100:0:0:1::1",
		"2001:2::1",
		"3fff::1",
		"5f00::1",
		"::ffff:93.184.216.34",
	}
	for _, ip := range tests {
		t.Run(ip, func(t *testing.T) {
			_, err := resolveValidatedIP(context.Background(), ip, nil)
			require.Error(t, err)
		})
	}
}

func TestWriteProxyRequestUsesStandardProxySemantics(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "http://api.openai.com/v1/chat", nil)
	require.NoError(t, err)
	req.Host = "tenant.example.com"
	req.Body = http.NoBody
	req.ContentLength = 0
	req.Header.Set("Content-Type", "application/json")

	outboundURL, err := url.Parse("http://93.184.216.34/v1/chat")
	require.NoError(t, err)
	var buf bytes.Buffer
	err = writeProxyRequest(&buf, req, outboundURL, req.URL.Host, "Basic dXNlcjpwYXNz")
	require.NoError(t, err)

	raw := buf.String()
	require.Contains(t, raw, "POST http://93.184.216.34/v1/chat HTTP/1.1\r\n")
	require.Contains(t, raw, "Host: tenant.example.com\r\n")
	require.Contains(t, raw, "Proxy-Authorization: Basic dXNlcjpwYXNz\r\n")
	require.Contains(t, raw, "Content-Length: 0\r\n")
	require.NotContains(t, raw, "Transfer-Encoding: chunked\r\n")
}

func TestValidatedHTTPProxyRoundTripperRejectsUnsafeTargetBeforeProxyDial(t *testing.T) {
	originalLookup := validatedLookupIPAddr
	originalBaseDial := validatedBaseDialContext
	defer func() {
		validatedLookupIPAddr = originalLookup
		validatedBaseDialContext = originalBaseDial
	}()

	validatedLookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		require.Equal(t, "api.openai.com", host)
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}

	called := false
	validatedBaseDialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
		called = true
		return nil, errors.New("unexpected proxy dial")
	}

	client, err := buildClient(Options{
		ProxyURL:           "http://proxy.example.com:8080",
		ValidateResolvedIP: true,
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, "http://api.openai.com/v1/models", nil)
	require.NoError(t, err)
	_, err = client.Do(req)
	require.Error(t, err)
	require.False(t, called, "unsafe target resolution must not reach the proxy dialer")
}

func TestValidatedHTTPProxyRoundTripperDialsResolvedTargetIPAndPreservesHost(t *testing.T) {
	originalLookup := validatedLookupIPAddr
	originalBaseDial := validatedBaseDialContext
	defer func() {
		validatedLookupIPAddr = originalLookup
		validatedBaseDialContext = originalBaseDial
	}()

	validatedLookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		require.Equal(t, "api.openai.com", host)
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}

	requestLineSeen := make(chan string, 1)
	hostSeen := make(chan string, 1)
	validatedBaseDialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
		require.Equal(t, "tcp", network)
		require.Equal(t, "proxy.example.com:8080", addr)

		clientConn, serverConn := net.Pipe()
		go func() {
			defer func() { _ = serverConn.Close() }()
			reader := bufio.NewReader(serverConn)
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			requestLineSeen <- strings.TrimSpace(line)

			for {
				line, err = reader.ReadString('\n')
				if err != nil {
					return
				}
				if strings.HasPrefix(strings.ToLower(line), "host:") {
					hostSeen <- strings.TrimSpace(strings.TrimPrefix(line, "Host:"))
				}
				if line == "\r\n" {
					_, _ = io.WriteString(serverConn, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 2\r\n\r\nok")
					return
				}
			}
		}()
		return clientConn, nil
	}

	client, err := buildClient(Options{
		ProxyURL:           "http://proxy.example.com:8080",
		ValidateResolvedIP: true,
	})
	require.NoError(t, err)

	resp, err := client.Get("http://api.openai.com/v1/models")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, "GET http://93.184.216.34/v1/models HTTP/1.1", <-requestLineSeen)
	require.Equal(t, "api.openai.com", <-hostSeen)
	require.Equal(t, "api.openai.com", resp.Request.URL.Host)
	require.Equal(t, "ok", string(body))
}
