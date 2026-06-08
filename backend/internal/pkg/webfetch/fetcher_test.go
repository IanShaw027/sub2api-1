package webfetch

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetcherFetch_HTMLExtractsTitleAndReadableText(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	called := false
	fetcher := &Fetcher{
		clientFactory: func(_ string) (*http.Client, error) {
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
	t.Parallel()

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
	t.Parallel()

	fetcher := &Fetcher{
		clientFactory: func(_ string) (*http.Client, error) {
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
	t.Parallel()

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
	t.Parallel()

	roundTrips := 0
	fetcher := &Fetcher{
		clientFactory: func(_ string) (*http.Client, error) {
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
	t.Parallel()

	roundTrips := 0
	fetcher := &Fetcher{
		clientFactory: func(_ string) (*http.Client, error) {
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

func TestFetcherFetch_RejectsResolvedPrivateIPBeforeRequest(t *testing.T) {
	t.Parallel()

	originalValidateResolvedHost := validateResolvedFetchHost
	validateResolvedFetchHost = func(host string) error {
		require.Equal(t, "public.example.com", host)
		return errors.New("resolved ip 127.0.0.1 is not allowed")
	}
	t.Cleanup(func() { validateResolvedFetchHost = originalValidateResolvedHost })

	called := false
	fetcher := &Fetcher{
		clientFactory: func(_ string) (*http.Client, error) {
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

func TestFetcherFetch_ExposesHTTPStatusCodeInError(t *testing.T) {
	t.Parallel()

	fetcher := &Fetcher{
		clientFactory: func(_ string) (*http.Client, error) {
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
