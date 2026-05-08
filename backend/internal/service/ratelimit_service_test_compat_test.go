//go:build !unit

package service

import (
	"context"
	"time"
)

type rateLimitAccountRepoStub struct {
	AccountRepository
	setErrorCalls          int
	tempCalls              int
	rateLimitedCalls       int
	updateCredentialsCalls int
	lastCredentials        map[string]any
	lastErrorMsg           string
	lastTempReason         string
	lastRateLimitedAt      time.Time
	tempErr                error
}

func (r *rateLimitAccountRepoStub) SetError(_ context.Context, _ int64, errorMsg string) error {
	r.setErrorCalls++
	r.lastErrorMsg = errorMsg
	return nil
}

func (r *rateLimitAccountRepoStub) SetTempUnschedulable(_ context.Context, _ int64, until time.Time, reason string) error {
	r.tempCalls++
	r.lastTempReason = reason
	r.lastRateLimitedAt = until
	return r.tempErr
}

func (r *rateLimitAccountRepoStub) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	r.rateLimitedCalls++
	r.lastRateLimitedAt = resetAt
	return nil
}

func (r *rateLimitAccountRepoStub) UpdateCredentials(_ context.Context, _ int64, credentials map[string]any) error {
	r.updateCredentialsCalls++
	r.lastCredentials = cloneCredentials(credentials)
	return nil
}
