//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNotificationEmailGetTemplate_NilServiceOrRepoReturnsOfficialTemplate(t *testing.T) {
	ctx := context.Background()

	var nilService *NotificationEmailService
	tmpl, err := nilService.GetTemplate(ctx, NotificationEmailEventBalanceLow, "en")
	require.NoError(t, err)
	require.Equal(t, NotificationEmailEventBalanceLow, tmpl.Event)
	require.Equal(t, "en", tmpl.Locale)
	require.False(t, tmpl.IsCustom)
	require.NotEmpty(t, tmpl.Subject)
	require.NotEmpty(t, tmpl.HTML)

	serviceWithNilRepo := &NotificationEmailService{}
	tmpl, err = serviceWithNilRepo.GetTemplate(ctx, NotificationEmailEventBalanceLow, "zh")
	require.NoError(t, err)
	require.Equal(t, NotificationEmailEventBalanceLow, tmpl.Event)
	require.Equal(t, "zh", tmpl.Locale)
	require.False(t, tmpl.IsCustom)
	require.NotEmpty(t, tmpl.Subject)
	require.NotEmpty(t, tmpl.HTML)
}

func TestNotificationEmailUpdateTemplate_NilServiceOrRepoReturnsRepoNotInitialized(t *testing.T) {
	ctx := context.Background()

	var nilService *NotificationEmailService
	_, err := nilService.UpdateTemplate(ctx, NotificationEmailEventBalanceLow, "en", "subject", "<p>{{recipient_name}}</p>")
	require.EqualError(t, err, "setting repository not initialized")

	serviceWithNilRepo := &NotificationEmailService{}
	_, err = serviceWithNilRepo.UpdateTemplate(ctx, NotificationEmailEventBalanceLow, "en", "subject", "<p>{{recipient_name}}</p>")
	require.EqualError(t, err, "setting repository not initialized")
}

func TestNotificationEmailRestoreOfficialTemplate_NilServiceOrRepoReturnsRepoNotInitialized(t *testing.T) {
	ctx := context.Background()

	var nilService *NotificationEmailService
	_, err := nilService.RestoreOfficialTemplate(ctx, NotificationEmailEventBalanceLow, "en")
	require.EqualError(t, err, "setting repository not initialized")

	serviceWithNilRepo := &NotificationEmailService{}
	_, err = serviceWithNilRepo.RestoreOfficialTemplate(ctx, NotificationEmailEventBalanceLow, "en")
	require.EqualError(t, err, "setting repository not initialized")
}

func TestNotificationEmailIsUnsubscribed_NilServiceOrRepoReturnsFalse(t *testing.T) {
	ctx := context.Background()

	var nilService *NotificationEmailService
	unsubscribed, err := nilService.IsUnsubscribed(ctx, "user@example.com", NotificationEmailEventBalanceLow)
	require.NoError(t, err)
	require.False(t, unsubscribed)

	serviceWithNilRepo := &NotificationEmailService{}
	unsubscribed, err = serviceWithNilRepo.IsUnsubscribed(ctx, "user@example.com", NotificationEmailEventBalanceLow)
	require.NoError(t, err)
	require.False(t, unsubscribed)
}

func TestNotificationEmailUnsubscribe_NilServiceOrRepoReturnsRepoNotInitialized(t *testing.T) {
	ctx := context.Background()
	working := NewNotificationEmailService(newNotificationEmailMemorySettingRepo(), nil)
	token, err := working.createUnsubscribeToken(ctx, "user@example.com", NotificationEmailEventBalanceLow)
	require.NoError(t, err)

	var nilService *NotificationEmailService
	_, err = nilService.Unsubscribe(ctx, token)
	require.EqualError(t, err, "setting repository not initialized")

	serviceWithNilRepo := &NotificationEmailService{}
	_, err = serviceWithNilRepo.Unsubscribe(ctx, token)
	require.EqualError(t, err, "setting repository not initialized")
}

func TestNotificationEmailSend_NilServiceOrRepoReturnsConfigError(t *testing.T) {
	ctx := context.Background()
	input := NotificationEmailSendInput{
		Event:          NotificationEmailEventBalanceLow,
		RecipientEmail: "user@example.com",
		RecipientName:  "User",
	}

	var nilService *NotificationEmailService
	err := nilService.Send(ctx, input)
	require.Error(t, err)
	require.EqualError(t, err, "setting repository not initialized")
	require.True(t, shouldFallbackNotificationEmail(err))
	var configErr notificationEmailConfigError
	require.True(t, errors.As(err, &configErr))

	serviceWithNilRepo := &NotificationEmailService{}
	err = serviceWithNilRepo.Send(ctx, input)
	require.Error(t, err)
	require.EqualError(t, err, "setting repository not initialized")
	require.True(t, shouldFallbackNotificationEmail(err))
	require.True(t, errors.As(err, &configErr))
}
