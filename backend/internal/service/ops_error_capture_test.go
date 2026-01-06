package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyError_NetworkErrors(t *testing.T) {
	tests := []struct {
		name               string
		err                error
		wantType           string
		wantPhase          string
		wantSeverity       string
		wantIsRetryable    bool
		wantUpstreamDetail string
	}{
		{
			name:               "timeout error",
			err:                errors.New("timeout_error: context deadline exceeded"),
			wantType:           "timeout_error",
			wantPhase:          "upstream",
			wantSeverity:       "P1",
			wantIsRetryable:    true,
			wantUpstreamDetail: "context deadline exceeded",
		},
		{
			name:               "network error - connection refused",
			err:                errors.New("network_error: connection refused"),
			wantType:           "network_error",
			wantPhase:          "upstream",
			wantSeverity:       "P1",
			wantIsRetryable:    true,
			wantUpstreamDetail: "connection refused",
		},
		{
			name:               "network error - no such host",
			err:                errors.New("network_error: no such host"),
			wantType:           "network_error",
			wantPhase:          "upstream",
			wantSeverity:       "P1",
			wantIsRetryable:    true,
			wantUpstreamDetail: "no such host",
		},
		{
			name:               "timeout error with detailed message",
			err:                errors.New("timeout_error: request timeout after 30s"),
			wantType:           "timeout_error",
			wantPhase:          "upstream",
			wantSeverity:       "P1",
			wantIsRetryable:    true,
			wantUpstreamDetail: "request timeout after 30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyError("", "", 0, nil, tt.err)

			assert.Equal(t, tt.wantType, result.Type)
			assert.Equal(t, tt.wantPhase, result.Phase)
			assert.Equal(t, tt.wantSeverity, result.Severity)
			assert.Equal(t, tt.wantIsRetryable, result.IsRetryable)

			if tt.wantUpstreamDetail != "" {
				assert.NotNil(t, result.UpstreamErrorDetail)
				assert.Equal(t, tt.wantUpstreamDetail, *result.UpstreamErrorDetail)
			}
		})
	}
}

func TestClassifyError_OtherErrors(t *testing.T) {
	tests := []struct {
		name        string
		errorType   string
		errorPhase  string
		err         error
		wantSource  string
		wantOwner   string
	}{
		{
			name:       "database error",
			errorType:  "database_error",
			errorPhase: "internal",
			err:        errors.New("connection pool exhausted"),
			wantSource: "infrastructure",
			wantOwner:  "infrastructure",
		},
		{
			name:       "authentication error",
			errorType:  "authentication_error",
			errorPhase: "auth",
			err:        errors.New("invalid credentials"),
			wantSource: "downstream_business",
			wantOwner:  "client",
		},
		{
			name:       "upstream business error",
			errorType:  "rate_limit_error",
			errorPhase: "upstream",
			err:        errors.New("rate limit exceeded"),
			wantSource: "upstream_business",
			wantOwner:  "provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyError(tt.errorType, tt.errorPhase, 0, nil, tt.err)

			assert.Equal(t, tt.wantSource, result.ErrorSource)
			assert.Equal(t, tt.wantOwner, result.ErrorOwner)
		})
	}
}
