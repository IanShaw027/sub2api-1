package service

import (
	"context"
	"strings"
)

const (
	opsSkipErrContextCanceled            = "context canceled"
	opsSkipErrNoAvailableAccounts        = "no available accounts"
	opsSkipErrInvalidAPIKey              = "invalid_api_key"
	opsSkipErrAPIKeyRequired             = "api_key_required"
	opsSkipErrInsufficientBalance        = "insufficient balance"
	opsSkipErrInsufficientAccountBalance = "insufficient account balance"
	opsSkipErrInsufficientQuota          = "insufficient_quota"
	opsSkipErrAccountNotFound            = "account not found"
	opsSkipErrTokenExpired               = "token expired"
	opsSkipErrTokenInvalid               = "token invalid"
	opsSkipErrTokenRevoked               = "token revoked"
	opsSkipErrCredentialExpired          = "credential expired"
	opsSkipErrCredentialsExpired         = "credentials expired"
	opsSkipErrUnauthorized               = "unauthorized"
	opsSkipErrAuthenticationFailed       = "authentication failed"
)

type OpsErrorLogSkipInput struct {
	Message     string
	Body        string
	RequestPath string
	StatusCode  int
}

func ShouldSkipOpsErrorLog(ctx context.Context, ops *OpsService, input OpsErrorLogSkipInput) bool {
	if ops == nil {
		return false
	}

	settings, err := ops.GetOpsAdvancedSettings(ctx)
	if err != nil || settings == nil {
		return false
	}

	msgLower := strings.ToLower(strings.TrimSpace(input.Message))
	bodyLower := strings.ToLower(strings.TrimSpace(input.Body))
	requestPathLower := strings.ToLower(strings.TrimSpace(input.RequestPath))

	if settings.IgnoreCountTokensErrors && strings.Contains(requestPathLower, "/count_tokens") {
		return true
	}

	if settings.IgnoreContextCanceled &&
		(strings.Contains(msgLower, opsSkipErrContextCanceled) || strings.Contains(bodyLower, opsSkipErrContextCanceled)) {
		return true
	}

	if settings.IgnoreNoAvailableAccounts &&
		(strings.Contains(msgLower, opsSkipErrNoAvailableAccounts) || strings.Contains(bodyLower, opsSkipErrNoAvailableAccounts)) {
		return true
	}

	if settings.IgnoreInvalidApiKeyErrors &&
		(strings.Contains(bodyLower, opsSkipErrInvalidAPIKey) || strings.Contains(bodyLower, opsSkipErrAPIKeyRequired)) {
		return true
	}

	if settings.IgnoreInsufficientBalanceErrors &&
		(strings.Contains(bodyLower, opsSkipErrInsufficientBalance) ||
			strings.Contains(bodyLower, opsSkipErrInsufficientAccountBalance) ||
			strings.Contains(bodyLower, opsSkipErrInsufficientQuota) ||
			strings.Contains(msgLower, opsSkipErrInsufficientBalance) ||
			strings.Contains(msgLower, opsSkipErrInsufficientAccountBalance)) {
		return true
	}

	if settings.IgnoreCredential401Errors && input.StatusCode == 401 {
		if strings.Contains(msgLower, opsSkipErrTokenExpired) ||
			strings.Contains(msgLower, opsSkipErrTokenInvalid) ||
			strings.Contains(msgLower, opsSkipErrTokenRevoked) ||
			strings.Contains(msgLower, opsSkipErrCredentialExpired) ||
			strings.Contains(msgLower, opsSkipErrCredentialsExpired) ||
			strings.Contains(msgLower, opsSkipErrUnauthorized) ||
			strings.Contains(msgLower, opsSkipErrAuthenticationFailed) ||
			strings.Contains(bodyLower, opsSkipErrTokenExpired) ||
			strings.Contains(bodyLower, opsSkipErrTokenInvalid) ||
			strings.Contains(bodyLower, opsSkipErrTokenRevoked) ||
			strings.Contains(bodyLower, opsSkipErrCredentialExpired) ||
			strings.Contains(bodyLower, opsSkipErrCredentialsExpired) ||
			strings.Contains(bodyLower, opsSkipErrUnauthorized) ||
			strings.Contains(bodyLower, opsSkipErrAuthenticationFailed) {
			return true
		}
	}

	if settings.IgnoreRateLimit429Errors && input.StatusCode == 429 {
		return true
	}

	if settings.IgnoreAccountNotFoundErrors &&
		(strings.Contains(msgLower, opsSkipErrAccountNotFound) || strings.Contains(bodyLower, opsSkipErrAccountNotFound)) {
		return true
	}

	return false
}
