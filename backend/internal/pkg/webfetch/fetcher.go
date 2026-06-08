package webfetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyutil"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"golang.org/x/net/html"
)

const (
	defaultMaxRedirects   = 3
	defaultMaxContentSize = 64 * 1024
	defaultRequestTimeout = 20 * time.Second
	defaultDialTimeout    = 5 * time.Second
	defaultTLSHandshake   = 5 * time.Second
)

var errTooManyRedirects = errors.New("webfetch: too many redirects")

type checkRedirectFunc func(req *http.Request, via []*http.Request) error

type clientFactoryFunc func(proxyURL string, checkRedirect checkRedirectFunc) (*http.Client, error)

// Fetcher executes outbound web fetches with bounded redirects/content.
type Fetcher struct {
	clientFactory clientFactoryFunc
}

// NewFetcher creates a fetcher with the default proxy-aware HTTP client factory.
func NewFetcher() *Fetcher {
	return &Fetcher{clientFactory: newHTTPClient}
}

// Fetch executes a single GET request and returns a structured success/error result.
func (f *Fetcher) Fetch(ctx context.Context, req FetchRequest) *FetchResult {
	result := &FetchResult{RequestedURL: req.URL}

	normalizedURL, host, err := validateFetchURL(req)
	if err != nil {
		result.Error = err
		return result
	}

	if matchesHost(host, req.BlockedHosts) {
		result.Error = &FetchError{
			Code:    ErrorCodeDomainBlocked,
			Message: fmt.Sprintf("host is blocked: %s", host),
		}
		return result
	}

	maxRedirects := req.MaxRedirects
	if maxRedirects < 0 {
		maxRedirects = 0
	}
	if maxRedirects == 0 {
		maxRedirects = defaultMaxRedirects
	}

	maxContentBytes := req.MaxContentBytes
	if maxContentBytes <= 0 {
		maxContentBytes = defaultMaxContentSize
	}

	factory := f.clientFactory
	if factory == nil {
		factory = newHTTPClient
	}

	client, clientErr := factory(req.ProxyURL, func(_ *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return errTooManyRedirects
		}
		return nil
	})
	if clientErr != nil {
		result.Error = &FetchError{
			Code:    ErrorCodeClientConfig,
			Message: clientErr.Error(),
		}
		return result
	}

	httpReq, buildErr := http.NewRequestWithContext(ctx, http.MethodGet, normalizedURL, nil)
	if buildErr != nil {
		result.Error = &FetchError{
			Code:    ErrorCodeInvalidURL,
			Message: buildErr.Error(),
		}
		return result
	}
	httpReq.Header.Set("Accept", "text/html, text/plain;q=0.9, */*;q=0.1")
	httpReq.Header.Set("User-Agent", "sub2api-webfetch/1.0")

	resp, doErr := client.Do(httpReq)
	if doErr != nil {
		result.Error = mapRequestError(doErr)
		return result
	}
	defer func() { _ = resp.Body.Close() }()

	result.FinalURL = normalizedURL
	if resp.Request != nil && resp.Request.URL != nil {
		result.FinalURL = resp.Request.URL.String()
	}
	result.StatusCode = resp.StatusCode
	result.ContentType = canonicalContentType(resp.Header.Get("Content-Type"))

	body, truncated, readErr := readBoundedBody(resp.Body, maxContentBytes)
	if readErr != nil {
		result.Error = &FetchError{
			Code:      ErrorCodeReadFailed,
			Message:   readErr.Error(),
			Retryable: true,
		}
		return result
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = &FetchError{
			Code:      ErrorCodeHTTPStatus,
			Message:   fmt.Sprintf("unexpected HTTP status %d", resp.StatusCode),
			Retryable: resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests,
		}
		return result
	}

	title, text := extractReadableContent(result.ContentType, body)
	if trimmedText, didTruncate := truncateUTF8String(text, maxContentBytes); didTruncate {
		text = trimmedText
		truncated = true
	}

	result.Title = title
	result.Text = text
	result.Truncated = truncated
	return result
}

func validateFetchURL(req FetchRequest) (normalizedURL, host string, fetchErr *FetchError) {
	normalizedURL, err := urlvalidator.ValidateHTTPURL(req.URL, req.AllowInsecureHTTP, urlvalidator.ValidationOptions{
		AllowedHosts: req.AllowedHosts,
		AllowPrivate: req.AllowPrivate,
	})
	if err != nil {
		code := ErrorCodeInvalidURL
		msg := err.Error()
		if strings.Contains(msg, "host is not allowed") || strings.Contains(msg, "allowlist") {
			code = ErrorCodeDomainBlocked
		}
		return "", "", &FetchError{Code: code, Message: msg}
	}

	parsed, err := url.Parse(normalizedURL)
	if err != nil || parsed.Hostname() == "" {
		return "", "", &FetchError{Code: ErrorCodeInvalidURL, Message: "invalid normalized url"}
	}
	return normalizedURL, strings.ToLower(parsed.Hostname()), nil
}

func newHTTPClient(proxyURL string, redirect checkRedirectFunc) (*http.Client, error) {
	_, parsedProxy, err := proxyurl.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: defaultDialTimeout}).DialContext,
		TLSHandshakeTimeout:   defaultTLSHandshake,
		ResponseHeaderTimeout: defaultRequestTimeout / 2,
		ForceAttemptHTTP2:     true,
	}
	if err := proxyutil.ConfigureTransportProxy(transport, parsedProxy); err != nil {
		return nil, fmt.Errorf("configure proxy: %w", err)
	}

	return &http.Client{
		Transport:     transport,
		Timeout:       defaultRequestTimeout,
		CheckRedirect: redirect,
	}, nil
}

func mapRequestError(err error) *FetchError {
	switch {
	case errors.Is(err, errTooManyRedirects):
		return &FetchError{
			Code:      ErrorCodeTooManyRedirects,
			Message:   "redirect limit exceeded",
			Retryable: true,
		}
	case errors.Is(err, context.DeadlineExceeded):
		return &FetchError{
			Code:      ErrorCodeRequestFailed,
			Message:   err.Error(),
			Retryable: true,
		}
	default:
		return &FetchError{
			Code:      ErrorCodeRequestFailed,
			Message:   err.Error(),
			Retryable: !errors.Is(err, context.Canceled),
		}
	}
}

func readBoundedBody(r io.Reader, maxBytes int) ([]byte, bool, error) {
	limited, err := io.ReadAll(io.LimitReader(r, int64(maxBytes+1)))
	if err != nil {
		return nil, false, err
	}
	if len(limited) > maxBytes {
		return limited[:maxBytes], true, nil
	}
	return limited, false, nil
}

func canonicalContentType(raw string) string {
	if raw == "" {
		return "application/octet-stream"
	}
	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil || mediaType == "" {
		return raw
	}
	return strings.ToLower(mediaType)
}

func extractReadableContent(contentType string, body []byte) (title, text string) {
	switch {
	case contentType == "text/html" || contentType == "application/xhtml+xml":
		return extractHTMLText(body)
	case strings.HasPrefix(contentType, "text/"):
		return "", bytesToText(body)
	default:
		return "", bytesToText(body)
	}
}

func extractHTMLText(body []byte) (title, text string) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", normalizeWhitespace(bytesToText(body))
	}

	title = strings.TrimSpace(findTitle(doc))
	parts := make([]string, 0, 16)
	var walk func(*html.Node, bool)
	walk = func(n *html.Node, skip bool) {
		if n == nil {
			return
		}
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "head", "script", "style", "noscript", "template", "svg":
				skip = true
			case "br", "p", "div", "section", "article", "main", "li", "ul", "ol", "h1", "h2", "h3", "h4", "h5", "h6":
				parts = append(parts, "\n")
			}
		}
		if !skip && n.Type == html.TextNode {
			if cleaned := normalizeWhitespace(n.Data); cleaned != "" {
				parts = append(parts, cleaned)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child, skip)
		}
	}
	walk(doc, false)

	text = normalizeWhitespace(strings.Join(parts, " "))
	if text == "" {
		text = normalizeWhitespace(bytesToText(body))
	}
	return title, text
}

func findTitle(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.ElementNode && strings.EqualFold(n.Data, "title") {
		return extractNodeText(n)
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if title := findTitle(child); title != "" {
			return title
		}
	}
	return ""
}

func extractNodeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	var parts []string
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if text := extractNodeText(child); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

func bytesToText(body []byte) string {
	return string(bytes.ToValidUTF8(body, []byte{}))
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func truncateUTF8String(s string, maxBytes int) (string, bool) {
	if len(s) <= maxBytes {
		return s, false
	}
	cut := 0
	for idx := range s {
		if idx > maxBytes {
			break
		}
		cut = idx
	}
	if cut == 0 {
		return "", true
	}
	return s[:cut], true
}

func matchesHost(host string, patterns []string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	for _, raw := range patterns {
		pattern := normalizeHostPattern(raw)
		if pattern == "" {
			continue
		}
		if strings.HasPrefix(pattern, "*.") {
			suffix := strings.TrimPrefix(pattern, "*.")
			if host == suffix || strings.HasSuffix(host, "."+suffix) {
				return true
			}
			continue
		}
		if host == pattern {
			return true
		}
	}
	return false
}

func normalizeHostPattern(value string) string {
	pattern := strings.ToLower(strings.TrimSpace(value))
	if pattern == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(pattern); err == nil {
		pattern = host
	}
	return pattern
}
