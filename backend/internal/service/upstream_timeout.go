package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	defaultUpstreamTimeout          = 30 * time.Second
	defaultStreamingUpstreamTimeout = 5 * time.Minute
)

func withUpstreamTimeout(ctx context.Context, cfg *config.Config, isStream bool) (context.Context, context.CancelFunc) {
	timeout := upstreamTimeout(cfg, isStream)
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func upstreamTimeout(cfg *config.Config, isStream bool) time.Duration {
	if cfg == nil {
		if isStream {
			return defaultStreamingUpstreamTimeout
		}
		return defaultUpstreamTimeout
	}

	if isStream {
		if cfg.Gateway.StreamingUpstreamTimeout <= 0 {
			return 0
		}
		return time.Duration(cfg.Gateway.StreamingUpstreamTimeout) * time.Second
	}

	if cfg.Gateway.UpstreamTimeout <= 0 {
		return 0
	}
	return time.Duration(cfg.Gateway.UpstreamTimeout) * time.Second
}
