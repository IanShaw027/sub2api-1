//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmailService_CacheBackedMethods_NilServiceOrCacheReturnUnavailable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		run  func(*EmailService) error
	}{
		{
			name: "SendVerifyCode",
			run: func(s *EmailService) error {
				return s.SendVerifyCode(context.Background(), "user@example.com", "Sub2API")
			},
		},
		{
			name: "VerifyCode",
			run: func(s *EmailService) error {
				return s.VerifyCode(context.Background(), "user@example.com", "123456")
			},
		},
		{
			name: "SendPasswordResetEmail",
			run: func(s *EmailService) error {
				return s.SendPasswordResetEmail(context.Background(), "user@example.com", "Sub2API", "https://example.com/reset")
			},
		},
		{
			name: "SendPasswordResetEmailWithCooldown",
			run: func(s *EmailService) error {
				return s.SendPasswordResetEmailWithCooldown(context.Background(), "user@example.com", "Sub2API", "https://example.com/reset")
			},
		},
		{
			name: "VerifyPasswordResetToken",
			run: func(s *EmailService) error {
				return s.VerifyPasswordResetToken(context.Background(), "user@example.com", "token")
			},
		},
		{
			name: "ConsumePasswordResetToken",
			run: func(s *EmailService) error {
				return s.ConsumePasswordResetToken(context.Background(), "user@example.com", "token")
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name+"/nil_service", func(t *testing.T) {
			t.Parallel()

			var nilService *EmailService
			require.ErrorIs(t, tc.run(nilService), ErrEmailCacheUnavailable)
		})

		t.Run(tc.name+"/nil_cache", func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, tc.run(&EmailService{}), ErrEmailCacheUnavailable)
		})
	}
}
