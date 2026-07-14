package service

import (
	"context"
	"strings"
)

type tlsFingerprintInboundUserAgentContextKey struct{}

// WithTLSFingerprintInboundUserAgent keeps the client identity available to
// detached auxiliary requests that no longer have a Gin context.
func WithTLSFingerprintInboundUserAgent(ctx context.Context, userAgent string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, tlsFingerprintInboundUserAgentContextKey{}, strings.TrimSpace(userAgent))
}

func tlsFingerprintInboundUserAgentFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	userAgent, _ := ctx.Value(tlsFingerprintInboundUserAgentContextKey{}).(string)
	return strings.TrimSpace(userAgent)
}
