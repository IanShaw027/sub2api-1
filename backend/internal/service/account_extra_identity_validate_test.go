//go:build unit

package service

import (
	"testing"

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
