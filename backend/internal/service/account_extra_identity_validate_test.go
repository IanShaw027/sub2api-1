//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestValidateAccountExtraIdentityRejectsInvalidOpenAIDeviceID(t *testing.T) {
	err := ValidateAccountExtraIdentity(map[string]any{
		"openai_device_id": "dev-xyz",
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "openai_device_id")
}

func TestValidateAccountExtraIdentityRejectsNonRFC4122OpenAIDeviceID(t *testing.T) {
	err := ValidateAccountExtraIdentity(map[string]any{
		"openai_device_id": "00000000-0000-0000-0000-000000000000",
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "openai_device_id")
}

func TestValidateAccountExtraIdentityRejectsRandomTLSProfileID(t *testing.T) {
	err := ValidateAccountExtraIdentity(map[string]any{
		"tls_fingerprint_profile_id": int64(-1),
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "tls_fingerprint_profile_id")
}

func TestValidateAccountExtraIdentityAcceptsValidIdentityExtras(t *testing.T) {
	err := ValidateAccountExtraIdentity(map[string]any{
		"openai_device_id":           uuid.NewString(),
		"tls_fingerprint_profile_id": int64(12),
	})

	require.NoError(t, err)
}

func TestValidateAccountExtraIdentityRejectsNonIntegerTLSProfileID(t *testing.T) {
	err := ValidateAccountExtraIdentity(map[string]any{
		"tls_fingerprint_profile_id": 3.7,
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "integer")
}

func TestValidateAccountCapacityExtraAcceptsEmptyAndInRangeValues(t *testing.T) {
	require.NoError(t, ValidateAccountCapacityExtra(nil))
	require.NoError(t, ValidateAccountCapacityExtra(map[string]any{}))
	require.NoError(t, ValidateAccountCapacityExtra(map[string]any{
		"concurrency":                  1,
		"max_sessions":                 0,
		"session_idle_timeout_minutes": 1,
		"rpm_sticky_buffer":            1,
		"codex_fingerprint_mode":       "session",
		"enable_tls_fingerprint":       true,
		"tls_fingerprint_profile_id":   int64(12),
	}))
	require.NoError(t, ValidateAccountCapacityExtra(map[string]any{
		"concurrency":                  32,
		"max_sessions":                 10000,
		"session_idle_timeout_minutes": 1440,
		"rpm_sticky_buffer":            10000,
		"codex_fingerprint_mode":       "off",
		"enable_tls_fingerprint":       false,
	}))
}

func TestValidateAccountCapacityExtraRejectsIllegalConcurrency(t *testing.T) {
	for _, value := range []any{0, -1, 999, 3.7, json.Number("3.7")} {
		err := ValidateAccountCapacityExtra(map[string]any{"concurrency": value})
		require.Error(t, err, "concurrency=%v", value)
		require.ErrorContains(t, err, "concurrency")
	}
}

func TestValidateAccountCapacityExtraRejectsIllegalMaxSessionsIdleAndBuffer(t *testing.T) {
	cases := []struct {
		key   string
		value any
		want  string
	}{
		{key: "max_sessions", value: -1, want: "max_sessions"},
		{key: "max_sessions", value: 10001, want: "max_sessions"},
		{key: "max_sessions", value: 3.7, want: "integer"},
		{key: "session_idle_timeout_minutes", value: 0, want: "session_idle_timeout_minutes"},
		{key: "session_idle_timeout_minutes", value: 1441, want: "session_idle_timeout_minutes"},
		{key: "rpm_sticky_buffer", value: 0, want: "rpm_sticky_buffer"},
		{key: "rpm_sticky_buffer", value: 10001, want: "rpm_sticky_buffer"},
		{key: "tls_fingerprint_profile_id", value: int64(-1), want: "tls_fingerprint_profile_id"},
		{key: "tls_fingerprint_profile_id", value: 3.7, want: "integer"},
		{key: "enable_tls_fingerprint", value: "yes", want: "enable_tls_fingerprint"},
		{key: "codex_fingerprint_mode", value: "random", want: "codex_fingerprint_mode"},
	}
	for _, tc := range cases {
		err := ValidateAccountCapacityExtra(map[string]any{tc.key: tc.value})
		require.Error(t, err, "%s=%v", tc.key, tc.value)
		require.ErrorContains(t, err, tc.want)
	}
}

func TestAdminCreateAndBulkRejectIllegalIdentityAndCapacityExtras(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		extra map[string]any
		want  string
	}{
		{name: "import tls -1", extra: map[string]any{"tls_fingerprint_profile_id": int64(-1)}, want: "tls_fingerprint_profile_id"},
		{name: "concurrency 999", extra: map[string]any{"concurrency": 999}, want: "concurrency"},
		{name: "float 3.7", extra: map[string]any{"max_sessions": 3.7}, want: "integer"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("create/"+tc.name, func(t *testing.T) {
			t.Parallel()
			repo := &longContextBillingRepoStub{}
			svc := &adminServiceImpl{accountRepo: repo}

			account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
				Name:                 "import-account",
				Platform:             PlatformAnthropic,
				Type:                 AccountTypeAPIKey,
				Credentials:          map[string]any{"api_key": "test"},
				Extra:                tc.extra,
				SkipDefaultGroupBind: true,
			})

			require.Nil(t, account)
			require.ErrorContains(t, err, tc.want)
			require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
			require.Nil(t, repo.createdAccount)
		})

		t.Run("bulk/"+tc.name, func(t *testing.T) {
			t.Parallel()
			repo := &longContextBillingRepoStub{account: &Account{ID: 1, Platform: PlatformAnthropic}}
			svc := &adminServiceImpl{accountRepo: repo}

			result, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
				AccountIDs: []int64{1},
				Extra:      tc.extra,
			})

			require.Nil(t, result)
			require.ErrorContains(t, err, tc.want)
			require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
			require.Zero(t, repo.bulkUpdateCalls)
		})
	}
}

func TestAccountWritePathsRejectIllegalCapacityExtras(t *testing.T) {
	t.Parallel()

	illegal := map[string]any{"concurrency": 999}

	t.Run("account service create", func(t *testing.T) {
		t.Parallel()
		repo := &accountRepoStubForOAuthOnlyGroup{}
		svc := NewAccountService(repo, &groupRepoStubForOAuthOnlyGroup{})

		account, err := svc.Create(context.Background(), CreateAccountRequest{
			Name:        "svc-create",
			Platform:    PlatformAnthropic,
			Type:        AccountTypeAPIKey,
			Credentials: map[string]any{"api_key": "test"},
			Extra:       illegal,
		})

		require.Nil(t, account)
		require.ErrorContains(t, err, "concurrency")
		require.Nil(t, repo.createdAccount)
	})

	t.Run("account service update", func(t *testing.T) {
		t.Parallel()
		repo := &accountRepoStubForOAuthOnlyGroup{
			getByIDAccount: &Account{ID: 7, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Extra: map[string]any{}},
		}
		svc := NewAccountService(repo, &groupRepoStubForOAuthOnlyGroup{})
		extra := map[string]any{"concurrency": 999}

		account, err := svc.Update(context.Background(), 7, UpdateAccountRequest{Extra: &extra})

		require.Nil(t, account)
		require.ErrorContains(t, err, "concurrency")
		require.Nil(t, repo.updatedAccount)
	})

	t.Run("admin update", func(t *testing.T) {
		t.Parallel()
		repo := &longContextBillingRepoStub{account: &Account{ID: 1, Platform: PlatformAnthropic, Extra: map[string]any{}}}
		svc := &adminServiceImpl{accountRepo: repo}

		account, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{Extra: illegal})

		require.Nil(t, account)
		require.ErrorContains(t, err, "concurrency")
		require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	})

	t.Run("admin update extra", func(t *testing.T) {
		t.Parallel()
		repo := &longContextBillingRepoStub{account: &Account{ID: 1, Platform: PlatformAnthropic}}
		svc := &adminServiceImpl{accountRepo: repo}

		err := svc.UpdateAccountExtra(context.Background(), 1, map[string]any{"concurrency": 999})

		require.ErrorContains(t, err, "concurrency")
		require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
		require.Zero(t, repo.updateExtraCalls)
	})
}
