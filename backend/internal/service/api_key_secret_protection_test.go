//go:build unit

package service

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type apiKeyTestEncryptor struct {
	failDecrypt bool
}

func (e *apiKeyTestEncryptor) Encrypt(plaintext string) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(plaintext)), nil
}

func (e *apiKeyTestEncryptor) Decrypt(ciphertext string) (string, error) {
	if e.failDecrypt {
		return "", errors.New("wrong key")
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	return string(raw), err
}

type apiKeySecretRepoStub struct {
	quotaBaseAPIKeyRepoStub
	materials   []APIKeySecretMaterial
	protected   []APIKeySecretMaterial
	auditRuns   int
	reveal      *APIKeySecretMaterial
	revealError error
}

func (r *apiKeySecretRepoStub) ListAPIKeySecretMaterials(context.Context) ([]APIKeySecretMaterial, error) {
	return append([]APIKeySecretMaterial(nil), r.materials...), nil
}

func (r *apiKeySecretRepoStub) ProtectAPIKeySecret(_ context.Context, id int64, expectedLegacyKey, lookupHash, ciphertext, prefix string) error {
	r.protected = append(r.protected, APIKeySecretMaterial{
		ID:            id,
		LegacyKey:     expectedLegacyKey,
		LookupHash:    lookupHash,
		KeyCiphertext: ciphertext,
		KeyPrefix:     prefix,
	})
	return nil
}

func (r *apiKeySecretRepoStub) GetAPIKeySecretForOwner(context.Context, int64, int64) (*APIKeySecretMaterial, error) {
	return r.reveal, r.revealError
}

func (r *apiKeySecretRepoStub) MigrateDeletedAPIKeyAuditHashes(context.Context) error {
	r.auditRuns++
	return nil
}

func TestAPIKeySecretProtectionHashOnlyWithoutStableEncryptionKey(t *testing.T) {
	svc := (&APIKeyService{}).WithAPIKeySecretProtection(nil, false)
	key := &APIKey{}

	require.NoError(t, svc.protectNewAPIKeySecret(key, "sk-hash-only-secret"))
	require.Equal(t, HashAPIKeyLookup("sk-hash-only-secret"), key.LookupHash)
	require.Equal(t, "sk-hash-", key.KeyPrefix)
	require.Empty(t, key.KeyCiphertext)
}

func TestAPIKeySecretProtectionStoresCiphertextWithStableKey(t *testing.T) {
	encryptor := &apiKeyTestEncryptor{}
	svc := (&APIKeyService{}).WithAPIKeySecretProtection(encryptor, true)
	key := &APIKey{}

	require.NoError(t, svc.protectNewAPIKeySecret(key, "sk-encrypted-secret"))
	require.NotEmpty(t, key.KeyCiphertext)
	raw, err := encryptor.Decrypt(key.KeyCiphertext)
	require.NoError(t, err)
	require.Equal(t, "sk-encrypted-secret", raw)
}

func TestMigrateAPIKeySecretsConvertsLegacyPlaintextToHashOnly(t *testing.T) {
	repo := &apiKeySecretRepoStub{
		materials: []APIKeySecretMaterial{{ID: 7, LegacyKey: "sk-legacy-plaintext"}},
	}
	svc := (&APIKeyService{apiKeyRepo: repo}).WithAPIKeySecretProtection(nil, false)

	require.NoError(t, svc.MigrateAPIKeySecrets(context.Background()))
	require.Equal(t, 1, repo.auditRuns)
	require.Len(t, repo.protected, 1)
	require.Equal(t, HashAPIKeyLookup("sk-legacy-plaintext"), repo.protected[0].LookupHash)
	require.Empty(t, repo.protected[0].KeyCiphertext)
	require.Equal(t, "sk-legac", repo.protected[0].KeyPrefix)
}

func TestMigrateAPIKeySecretsSkipsDecryptForFullyMigratedRows(t *testing.T) {
	repo := &apiKeySecretRepoStub{
		materials: []APIKeySecretMaterial{{
			ID:            8,
			LegacyKey:     HashAPIKeyLookup("sk-protected"),
			LookupHash:    HashAPIKeyLookup("sk-protected"),
			KeyCiphertext: base64.StdEncoding.EncodeToString([]byte("sk-protected")),
			KeyPrefix:     "sk-prote",
		}},
	}
	svc := (&APIKeyService{apiKeyRepo: repo}).WithAPIKeySecretProtection(&apiKeyTestEncryptor{failDecrypt: true}, true)

	err := svc.MigrateAPIKeySecrets(context.Background())
	require.NoError(t, err)
	require.Empty(t, repo.protected)
}

func TestMigrateAPIKeySecretsRejectsWrongConfiguredEncryptionKeyForIncompleteRows(t *testing.T) {
	repo := &apiKeySecretRepoStub{
		materials: []APIKeySecretMaterial{{
			ID:            8,
			LegacyKey:     HashAPIKeyLookup("sk-protected"),
			LookupHash:    HashAPIKeyLookup("sk-protected"),
			KeyCiphertext: base64.StdEncoding.EncodeToString([]byte("sk-protected")),
		}},
	}
	svc := (&APIKeyService{apiKeyRepo: repo}).WithAPIKeySecretProtection(&apiKeyTestEncryptor{failDecrypt: true}, true)

	err := svc.MigrateAPIKeySecrets(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong key")
}

func TestMigrateAPIKeySecretsSkipsFingerprintMismatchAndContinues(t *testing.T) {
	repo := &apiKeySecretRepoStub{
		materials: []APIKeySecretMaterial{
			{
				ID:         8,
				LegacyKey:  "sk-corrupt",
				LookupHash: HashAPIKeyLookup("sk-different"),
			},
			{
				ID:        9,
				LegacyKey: "sk-valid-legacy",
			},
		},
	}
	svc := (&APIKeyService{apiKeyRepo: repo}).WithAPIKeySecretProtection(nil, false)

	require.NoError(t, svc.MigrateAPIKeySecrets(context.Background()))
	require.Len(t, repo.protected, 1)
	require.Equal(t, int64(9), repo.protected[0].ID)
	require.Equal(t, HashAPIKeyLookup("sk-valid-legacy"), repo.protected[0].LookupHash)
}

func TestAPIKeyRevealHashOnlyIsUnavailable(t *testing.T) {
	raw := "sk-hash-only"
	repo := &apiKeySecretRepoStub{reveal: &APIKeySecretMaterial{
		ID:         1,
		LegacyKey:  HashAPIKeyLookup(raw),
		LookupHash: HashAPIKeyLookup(raw),
		KeyPrefix:  APIKeyDisplayPrefix(raw),
	}}
	svc := (&APIKeyService{apiKeyRepo: repo}).WithAPIKeySecretProtection(nil, false)

	_, err := svc.Reveal(context.Background(), 1, 2)
	require.ErrorIs(t, err, ErrAPIKeyRevealUnavailable)
}

func TestAPIKeyRevealDecryptsOwnerScopedCiphertext(t *testing.T) {
	raw := "sk-owner-secret"
	encryptor := &apiKeyTestEncryptor{}
	ciphertext, err := encryptor.Encrypt(raw)
	require.NoError(t, err)
	repo := &apiKeySecretRepoStub{reveal: &APIKeySecretMaterial{
		ID:            1,
		LegacyKey:     HashAPIKeyLookup(raw),
		LookupHash:    HashAPIKeyLookup(raw),
		KeyCiphertext: ciphertext,
		KeyPrefix:     APIKeyDisplayPrefix(raw),
	}}
	svc := (&APIKeyService{apiKeyRepo: repo}).WithAPIKeySecretProtection(encryptor, true)

	got, err := svc.Reveal(context.Background(), 1, 2)
	require.NoError(t, err)
	require.Equal(t, raw, got)
}
