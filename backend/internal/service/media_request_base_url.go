package service

import (
	"context"
	"strings"
)

type requestBaseURLContextKey struct{}

// WithRequestBaseURL injects the inbound request's base URL (scheme://host)
// into ctx so downstream services can build absolute URLs that point back at
// the backend the caller actually reached, instead of a statically configured
// public base URL.
func WithRequestBaseURL(ctx context.Context, baseURL string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return ctx
	}
	return context.WithValue(ctx, requestBaseURLContextKey{}, baseURL)
}

// requestBaseURLFromContext resolves the inbound request base URL from ctx.
// Returns an empty string when no request context is available (e.g. async
// email sending), in which case callers fall back to a configured base URL.
func requestBaseURLFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	baseURL, _ := ctx.Value(requestBaseURLContextKey{}).(string)
	return strings.TrimRight(strings.TrimSpace(baseURL), "/")
}
