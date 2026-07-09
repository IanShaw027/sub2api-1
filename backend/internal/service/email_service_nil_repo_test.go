//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSMTPConfig_NilServiceOrRepoReturnsNotConfigured(t *testing.T) {
	var nilService *EmailService

	cfg, err := nilService.GetSMTPConfig(context.Background())
	require.Nil(t, cfg)
	require.ErrorIs(t, err, ErrEmailNotConfigured)

	serviceWithNilRepo := &EmailService{}
	cfg, err = serviceWithNilRepo.GetSMTPConfig(context.Background())
	require.Nil(t, cfg)
	require.ErrorIs(t, err, ErrEmailNotConfigured)
}

func TestSendEmail_NilServiceOrRepoReturnsNotConfigured(t *testing.T) {
	var nilService *EmailService
	require.ErrorIs(t, nilService.SendEmail(context.Background(), "user@example.com", "subject", "<p>body</p>"), ErrEmailNotConfigured)

	serviceWithNilRepo := &EmailService{}
	require.ErrorIs(t, serviceWithNilRepo.SendEmail(context.Background(), "user@example.com", "subject", "<p>body</p>"), ErrEmailNotConfigured)
}
