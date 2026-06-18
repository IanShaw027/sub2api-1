package webfetch

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetcherFetch_HTMLExtractsTitleAndReadableText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<!doctype html>
<html>
  <head>
    <title>Example Title</title>
    <style>.hidden { display:none; }</style>
    <script>var hidden = "ignore me";</script>
  </head>
  <body>
    <main>
      <h1>Readable Heading</h1>
      <p>Paragraph with <strong>bold</strong> text.</p>
    </main>
  </body>
</html>`)
	}))
	defer srv.Close()

	result := NewFetcher().Fetch(context.Background(), FetchRequest{
		URL:               srv.URL,
		AllowPrivate:      true,
		AllowInsecureHTTP: true,
	})

	require.Nil(t, result.Error)
	require.Equal(t, srv.URL, result.FinalURL)
	require.Equal(t, "Example Title", result.Title)
	require.Equal(t, "text/html", result.ContentType)
	require.Contains(t, result.Text, "Readable Heading")
	require.Contains(t, result.Text, "Paragraph with bold text.")
	require.NotContains(t, result.Text, "ignore me")
}

func TestFetcherFetch_PlainTextReturnsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "line one\nline two\n")
	}))
	defer srv.Close()

	result := NewFetcher().Fetch(context.Background(), FetchRequest{
		URL:               srv.URL,
		AllowPrivate:      true,
		AllowInsecureHTTP: true,
	})

	require.Nil(t, result.Error)
	require.Equal(t, "text/plain", result.ContentType)
	require.Empty(t, result.Title)
	require.Equal(t, "line one\nline two\n", result.Text)
	require.False(t, result.Truncated)
}

func TestFetcherFetch_ValidatesAllowedAndBlockedDomainsBeforeRequest(t *testing.T) {
	called := false
	fetcher := &Fetcher{
		clientFactory: func(_ FetchRequest) (*http.Client, error) {
			return &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					called = true
					return nil, errors.New("unexpected outbound request")
				}),
			}, nil
		},
	}

	result := fetcher.Fetch(context.Background(), FetchRequest{
		URL:               "http://blocked.example.com/page",
		AllowInsecureHTTP: true,
		AllowedHosts:      []string{"*.example.com"},
		BlockedHosts:      []string{"blocked.example.com"},
	})

	require.NotNil(t, result.Error)
	require.Equal(t, ErrorCodeDomainBlocked, result.Error.Code)
	require.False(t, called)
}

func TestFetcherFetch_TruncatesOversizedContent(t *testing.T) {
	const maxBytes = 32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, strings.Repeat("x", maxBytes+20))
	}))
	defer srv.Close()

	result := NewFetcher().Fetch(context.Background(), FetchRequest{
		URL:               srv.URL,
		AllowPrivate:      true,
		AllowInsecureHTTP: true,
		MaxContentBytes:   maxBytes,
	})

	require.Nil(t, result.Error)
	require.True(t, result.Truncated)
	require.Len(t, result.Text, maxBytes)
	require.Equal(t, strings.Repeat("x", maxBytes), result.Text)
}

func TestFetcherFetch_MapsRequestFailuresToStructuredError(t *testing.T) {
	stubResolvedFetchHostOK(t)

	fetcher := &Fetcher{
		clientFactory: func(_ FetchRequest) (*http.Client, error) {
			return &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, errors.New("dial tcp: i/o timeout")
				}),
			}, nil
		},
	}

	result := fetcher.Fetch(context.Background(), FetchRequest{
		URL:               "http://example.com/page",
		AllowInsecureHTTP: true,
	})

	require.NotNil(t, result.Error)
	require.Equal(t, ErrorCodeRequestFailed, result.Error.Code)
	require.Contains(t, result.Error.Message, "dial tcp")
	require.True(t, result.Error.Retryable)
}

func TestFetcherFetch_StopsAfterConfiguredRedirectLimit(t *testing.T) {
	redirects := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			redirects++
			http.Redirect(w, r, "/middle", http.StatusFound)
			return
		}
		if r.URL.Path == "/middle" {
			redirects++
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	result := NewFetcher().Fetch(context.Background(), FetchRequest{
		URL:               srv.URL + "/start",
		AllowPrivate:      true,
		AllowInsecureHTTP: true,
		MaxRedirects:      1,
	})

	require.NotNil(t, result.Error)
	require.Equal(t, ErrorCodeTooManyRedirects, result.Error.Code)
	require.Equal(t, 2, redirects)
}

func TestFetcherFetch_RejectsRedirectToBlockedHost(t *testing.T) {
	stubResolvedFetchHostOK(t)

	roundTrips := 0
	fetcher := &Fetcher{
		clientFactory: func(_ FetchRequest) (*http.Client, error) {
			return &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					roundTrips++
					return redirectResponse("http://blocked.example.com/final"), nil
				}),
			}, nil
		},
	}

	result := fetcher.Fetch(context.Background(), FetchRequest{
		URL:               "http://example.com/start",
		AllowedHosts:      []string{"*.example.com"},
		BlockedHosts:      []string{"blocked.example.com"},
		AllowInsecureHTTP: true,
	})

	require.NotNil(t, result.Error)
	require.Equal(t, ErrorCodeDomainBlocked, result.Error.Code)
	require.Equal(t, "blocked_host", result.Error.Reason)
	require.Equal(t, 1, roundTrips)
}

func TestFetcherFetch_RejectsRedirectToPrivateHost(t *testing.T) {
	stubResolvedFetchHostOK(t)

	roundTrips := 0
	fetcher := &Fetcher{
		clientFactory: func(_ FetchRequest) (*http.Client, error) {
			return &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					roundTrips++
					return redirectResponse("http://127.0.0.1/final"), nil
				}),
			}, nil
		},
	}

	result := fetcher.Fetch(context.Background(), FetchRequest{
		URL:               "http://example.com/start",
		AllowInsecureHTTP: true,
	})

	require.NotNil(t, result.Error)
	require.Equal(t, ErrorCodeDomainBlocked, result.Error.Code)
	require.Equal(t, "private_host", result.Error.Reason)
	require.Equal(t, 1, roundTrips)
}

func TestFetcherFetch_BlocksSpecialUseLiteralHostAsPrivateHost(t *testing.T) {
	result := NewFetcher().Fetch(context.Background(), FetchRequest{
		URL:               "http://100.64.0.1/page",
		AllowInsecureHTTP: true,
	})

	require.NotNil(t, result.Error)
	require.Equal(t, ErrorCodeDomainBlocked, result.Error.Code)
	require.Equal(t, "private_host", result.Error.Reason)
}

func TestFetcherFetch_RejectsResolvedPrivateIPBeforeRequest(t *testing.T) {
	originalValidateResolvedHost := validateResolvedFetchHost
	validateResolvedFetchHost = func(host string) error {
		require.Equal(t, "public.example.com", host)
		return errors.New("resolved ip 127.0.0.1 is not allowed")
	}
	t.Cleanup(func() { validateResolvedFetchHost = originalValidateResolvedHost })

	called := false
	fetcher := &Fetcher{
		clientFactory: func(_ FetchRequest) (*http.Client, error) {
			return &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					called = true
					return nil, errors.New("unexpected outbound request")
				}),
			}, nil
		},
	}

	result := fetcher.Fetch(context.Background(), FetchRequest{
		URL:               "http://public.example.com/final",
		AllowInsecureHTTP: true,
	})

	require.NotNil(t, result.Error)
	require.Equal(t, ErrorCodeDomainBlocked, result.Error.Code)
	require.Equal(t, "resolved_private_ip", result.Error.Reason)
	require.False(t, called)
}

func TestFetcherFetch_RejectsPrivateIPResolvedAtDialTime(t *testing.T) {
	originalValidateResolvedHost := validateResolvedFetchHost
	validateResolvedFetchHost = func(string) error { return nil }
	t.Cleanup(func() { validateResolvedFetchHost = originalValidateResolvedHost })

	originalLookup := fetchLookupIPAddr
	fetchLookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		require.Equal(t, "public.example.com", host)
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	t.Cleanup(func() { fetchLookupIPAddr = originalLookup })

	originalDial := fetchBaseDialContext
	var dialed atomic.Bool
	fetchBaseDialContext = func(context.Context, string, string) (net.Conn, error) {
		dialed.Store(true)
		return nil, errors.New("unexpected dial")
	}
	t.Cleanup(func() { fetchBaseDialContext = originalDial })

	result := NewFetcher().Fetch(context.Background(), FetchRequest{
		URL:               "http://public.example.com/private-after-rebind",
		AllowInsecureHTTP: true,
	})

	require.NotNil(t, result.Error)
	require.Equal(t, ErrorCodeDomainBlocked, result.Error.Code)
	require.Equal(t, "resolved_private_ip", result.Error.Reason)
	require.False(t, dialed.Load(), "private dial-time resolution must not reach the base dialer")
}

func TestFetcherFetch_DialsValidatedIPAndPreservesHostHeader(t *testing.T) {
	originalValidateResolvedHost := validateResolvedFetchHost
	validateResolvedFetchHost = func(string) error { return nil }
	t.Cleanup(func() { validateResolvedFetchHost = originalValidateResolvedHost })

	originalLookup := fetchLookupIPAddr
	fetchLookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		require.Equal(t, "public.example.com", host)
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}
	t.Cleanup(func() { fetchLookupIPAddr = originalLookup })

	hostSeen := make(chan string, 1)
	originalDial := fetchBaseDialContext
	fetchBaseDialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
		require.Equal(t, "tcp", network)
		require.Equal(t, "93.184.216.34:80", addr)
		clientConn, serverConn := net.Pipe()
		go func() {
			defer func() { _ = serverConn.Close() }()
			reader := bufio.NewReader(serverConn)
			for {
				line, err := reader.ReadString('\n')
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
	t.Cleanup(func() { fetchBaseDialContext = originalDial })

	result := NewFetcher().Fetch(context.Background(), FetchRequest{
		URL:               "http://public.example.com/page",
		AllowInsecureHTTP: true,
	})

	require.Nil(t, result.Error)
	require.Equal(t, "ok", result.Text)
	require.Equal(t, "public.example.com", <-hostSeen)
}

func TestFetcherFetch_ProxyRejectsPrivateIPResolvedAtRoundTrip(t *testing.T) {
	originalValidateResolvedHost := validateResolvedFetchHost
	validateResolvedFetchHost = func(string) error { return nil }
	t.Cleanup(func() { validateResolvedFetchHost = originalValidateResolvedHost })

	originalLookup := fetchLookupIPAddr
	fetchLookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		require.Equal(t, "public.example.com", host)
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	t.Cleanup(func() { fetchLookupIPAddr = originalLookup })

	originalDial := fetchBaseDialContext
	var dialed atomic.Bool
	fetchBaseDialContext = func(context.Context, string, string) (net.Conn, error) {
		dialed.Store(true)
		return nil, errors.New("unexpected proxy dial")
	}
	t.Cleanup(func() { fetchBaseDialContext = originalDial })

	result := NewFetcher().Fetch(context.Background(), FetchRequest{
		URL:               "http://public.example.com/proxied-rebind",
		ProxyURL:          "http://proxy.example.com:8080",
		AllowInsecureHTTP: true,
	})

	require.NotNil(t, result.Error)
	require.Equal(t, ErrorCodeDomainBlocked, result.Error.Code)
	require.Equal(t, "resolved_private_ip", result.Error.Reason)
	require.False(t, dialed.Load(), "unsafe target resolution must be blocked before dialing the proxy")
}

func TestFetcherFetch_HTTPProxyDialsValidatedTargetIPAndPreservesResultURL(t *testing.T) {
	originalValidateResolvedHost := validateResolvedFetchHost
	validateResolvedFetchHost = func(string) error { return nil }
	t.Cleanup(func() { validateResolvedFetchHost = originalValidateResolvedHost })

	originalLookup := fetchLookupIPAddr
	fetchLookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		require.Equal(t, "public.example.com", host)
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}
	t.Cleanup(func() { fetchLookupIPAddr = originalLookup })

	requestLineSeen := make(chan string, 1)
	hostSeen := make(chan string, 1)
	originalDial := fetchBaseDialContext
	fetchBaseDialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
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
	t.Cleanup(func() { fetchBaseDialContext = originalDial })

	result := NewFetcher().Fetch(context.Background(), FetchRequest{
		URL:               "http://public.example.com/page",
		ProxyURL:          "http://proxy.example.com:8080",
		AllowInsecureHTTP: true,
	})

	require.Nil(t, result.Error)
	require.Equal(t, "GET http://93.184.216.34/page HTTP/1.1", <-requestLineSeen)
	require.Equal(t, "public.example.com", <-hostSeen)
	require.Equal(t, "http://public.example.com/page", result.FinalURL)
	require.Equal(t, "ok", result.Text)
}

func TestResolveFetchValidatedIPBlocksSpecialUseNetworks(t *testing.T) {
	tests := []string{
		"100.64.0.1",
		"198.18.0.1",
		"224.0.0.1",
		"240.0.0.1",
		"255.255.255.255",
	}
	for _, ip := range tests {
		t.Run(ip, func(t *testing.T) {
			_, err := resolveFetchValidatedIP(context.Background(), ip, nil)
			require.Error(t, err)
		})
	}
}

func TestFetcherFetch_ExposesHTTPStatusCodeInError(t *testing.T) {
	stubResolvedFetchHostOK(t)

	fetcher := &Fetcher{
		clientFactory: func(_ FetchRequest) (*http.Client, error) {
			return &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusTooManyRequests,
						Header:     http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
						Body:       io.NopCloser(strings.NewReader("slow down")),
					}, nil
				}),
			}, nil
		},
	}

	result := fetcher.Fetch(context.Background(), FetchRequest{
		URL:               "http://example.com/page",
		AllowInsecureHTTP: true,
	})

	require.NotNil(t, result.Error)
	require.Equal(t, ErrorCodeHTTPStatus, result.Error.Code)
	require.Equal(t, 429, result.Error.StatusCode)
	require.Equal(t, "http_status", result.Error.Reason)
	require.True(t, result.Error.Retryable)
	require.Equal(t, 429, result.StatusCode)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func stubResolvedFetchHostOK(t *testing.T) {
	t.Helper()
	original := validateResolvedFetchHost
	validateResolvedFetchHost = func(string) error { return nil }
	t.Cleanup(func() { validateResolvedFetchHost = original })
}

func redirectResponse(location string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusFound,
		Header: http.Header{
			"Location": []string{location},
		},
		Body:    io.NopCloser(strings.NewReader("")),
		Request: &http.Request{URL: mustParseURL(location)},
	}
}

func mustParseURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return parsed
}
