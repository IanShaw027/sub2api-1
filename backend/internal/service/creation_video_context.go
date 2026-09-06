package service

import (
	"context"
	"time"
)

const (
	CreationVideoSubmissionTimeout            = 90 * time.Second
	CreationVideoAcceptancePersistenceTimeout = 10 * time.Second
	CreationVideoBindTimeout                  = 5 * time.Second
)

type creationVideoTaskContextKey struct{}

// Only the local-video bridge supplies this marker. Ordinary gateway requests
// keep their existing upstream-detachment behavior.
func WithCreationVideoTaskContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, creationVideoTaskContextKey{}, true)
}

func NewCreationVideoSubmissionContext(browser context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(WithCreationVideoTaskContext(context.WithoutCancel(browser)), CreationVideoSubmissionTimeout)
}

func grokMediaUpstreamContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx != nil && ctx.Value(creationVideoTaskContextKey{}) == true {
		return ctx, func() {}
	}
	return detachUpstreamContext(ctx)
}

// Once upstream returned a task ID, persist its owner/account and pricing
// snapshot even if the submission deadline or browser request just ended.
func CreationVideoAcceptanceContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx != nil && ctx.Value(creationVideoTaskContextKey{}) == true {
		return context.WithTimeout(context.WithoutCancel(ctx), CreationVideoAcceptancePersistenceTimeout)
	}
	return ctx, func() {}
}
