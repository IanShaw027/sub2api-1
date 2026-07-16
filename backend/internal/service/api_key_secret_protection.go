package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrAPIKeyRevealUnavailable = infraerrors.Conflict(
	"API_KEY_REVEAL_UNAVAILABLE",
	"API key reveal is unavailable because no recoverable encrypted copy is stored",
)

// APIKeySecretMaterial is the minimal stored material needed for startup
// validation and migration. LegacyKey may contain plaintext only on rows written
// by an older binary; protected rows store LookupHash in that column.
type APIKeySecretMaterial struct {
	ID            int64
	LegacyKey     string
	LookupHash    string
	KeyCiphertext string
	KeyPrefix     string
}

// APIKeySecretRepository is an optional migration/reveal extension implemented
// by the production API key repository. Keeping it separate avoids exposing
// ciphertext operations to ordinary domain consumers and test stubs.
type APIKeySecretRepository interface {
	ListAPIKeySecretMaterials(ctx context.Context) ([]APIKeySecretMaterial, error)
	ProtectAPIKeySecret(ctx context.Context, id int64, expectedLegacyKey, lookupHash, ciphertext, prefix string) error
	GetAPIKeySecretForOwner(ctx context.Context, id, userID int64) (*APIKeySecretMaterial, error)
	MigrateDeletedAPIKeyAuditHashes(ctx context.Context) error
}

// HashAPIKeyLookup returns the non-reversible database/cache fingerprint for a
// raw API key.
func HashAPIKeyLookup(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// APIKeyDisplayPrefix returns a small non-sensitive identifier for list/search
// responses without retaining the full key or its suffix.
func APIKeyDisplayPrefix(raw string) string {
	raw = strings.TrimSpace(raw)
	const prefixLength = 8
	if len(raw) <= prefixLength {
		return raw
	}
	return raw[:prefixLength]
}

func (s *APIKeyService) protectNewAPIKeySecret(key *APIKey, raw string) error {
	if key == nil {
		return ErrAPIKeyNotFound
	}
	lookupHash, ciphertext, prefix, err := s.protectedAPIKeyMaterial(raw)
	if err != nil {
		return err
	}
	key.LookupHash = lookupHash
	key.KeyPrefix = prefix
	key.KeyCiphertext = ciphertext
	return nil
}

func (s *APIKeyService) protectedAPIKeyMaterial(raw string) (string, string, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", "", ErrAPIKeyNotFound
	}
	lookupHash := HashAPIKeyLookup(raw)
	prefix := APIKeyDisplayPrefix(raw)
	if s == nil || !s.keyEncryptionStable || s.keyEncryptor == nil {
		return lookupHash, "", prefix, nil
	}
	ciphertext, err := s.keyEncryptor.Encrypt(raw)
	if err != nil {
		return "", "", "", fmt.Errorf("encrypt api key: %w", err)
	}
	return lookupHash, ciphertext, prefix, nil
}

func (s *APIKeyService) protectLegacyAPIKeySecret(ctx context.Context, id int64, raw string) error {
	repo, ok := s.apiKeyRepo.(APIKeySecretRepository)
	if !ok {
		// Lightweight adapters and unit-test stubs may not expose persistence
		// migration. Production repository implements this interface.
		return nil
	}
	lookupHash, ciphertext, prefix, err := s.protectedAPIKeyMaterial(raw)
	if err != nil {
		return err
	}
	return repo.ProtectAPIKeySecret(ctx, id, raw, lookupHash, ciphertext, prefix)
}

// MigrateAPIKeySecrets validates protected rows and converts active legacy
// plaintext rows. Zero-key deployments remain compatible without a stable key.
func (s *APIKeyService) MigrateAPIKeySecrets(ctx context.Context) error {
	if s == nil || s.apiKeyRepo == nil {
		return nil
	}
	repo, ok := s.apiKeyRepo.(APIKeySecretRepository)
	if !ok {
		return fmt.Errorf("api key repository does not support secret migration")
	}
	if err := repo.MigrateDeletedAPIKeyAuditHashes(ctx); err != nil {
		return fmt.Errorf("migrate deleted api key audit hashes: %w", err)
	}
	materials, err := repo.ListAPIKeySecretMaterials(ctx)
	if err != nil {
		return fmt.Errorf("list api key secret materials: %w", err)
	}
	if len(materials) == 0 {
		return nil
	}
	corruptRows := 0
	reportCorruptRow := func(material APIKeySecretMaterial, reason string) {
		corruptRows++
		slog.ErrorContext(ctx, "skipping corrupt api key secret during migration",
			slog.Int64("id", material.ID),
			slog.String("reason", reason),
		)
	}
	for _, material := range materials {
		if material.LookupHash == "" {
			raw := strings.TrimSpace(material.LegacyKey)
			if raw == "" {
				reportCorruptRow(material, "no recoverable secret material")
				continue
			}
			lookupHash, ciphertext, prefix, err := s.protectedAPIKeyMaterial(raw)
			if err != nil {
				return fmt.Errorf("protect legacy api key %d: %w", material.ID, err)
			}
			if err := repo.ProtectAPIKeySecret(ctx, material.ID, material.LegacyKey, lookupHash, ciphertext, prefix); err != nil {
				return fmt.Errorf("persist protected api key %d: %w", material.ID, err)
			}
			continue
		}

		legacyContainsRaw := material.LegacyKey != material.LookupHash
		if !legacyContainsRaw && material.KeyPrefix != "" {
			continue
		}
		raw := ""
		if legacyContainsRaw {
			raw = strings.TrimSpace(material.LegacyKey)
			if HashAPIKeyLookup(raw) != material.LookupHash {
				reportCorruptRow(material, "legacy fingerprint mismatch")
				continue
			}
		}
		if material.KeyCiphertext != "" && s.keyEncryptionStable && s.keyEncryptor != nil {
			decrypted, err := s.keyEncryptor.Decrypt(material.KeyCiphertext)
			if err != nil {
				return fmt.Errorf("decrypt protected api key %d: %w", material.ID, err)
			}
			if HashAPIKeyLookup(decrypted) != material.LookupHash {
				reportCorruptRow(material, "ciphertext fingerprint mismatch")
				continue
			}
			raw = decrypted
		}
		if raw != "" {
			ciphertext := material.KeyCiphertext
			if ciphertext == "" && s.keyEncryptionStable && s.keyEncryptor != nil {
				var err error
				ciphertext, err = s.keyEncryptor.Encrypt(raw)
				if err != nil {
					return fmt.Errorf("encrypt protected api key %d: %w", material.ID, err)
				}
			}
			if err := repo.ProtectAPIKeySecret(
				ctx,
				material.ID,
				material.LegacyKey,
				material.LookupHash,
				ciphertext,
				APIKeyDisplayPrefix(raw),
			); err != nil {
				return fmt.Errorf("normalize protected api key %d: %w", material.ID, err)
			}
		}
	}
	if corruptRows > 0 {
		slog.ErrorContext(ctx, "api key secret migration completed with corrupt rows skipped",
			slog.Int("corrupt_rows", corruptRows),
		)
	}
	return nil
}

// Reveal returns the raw key only after an owner-scoped repository lookup.
// Handler-level TOTP step-up is performed before this call.
func (s *APIKeyService) Reveal(ctx context.Context, id, userID int64) (string, error) {
	if s == nil || s.apiKeyRepo == nil {
		return "", ErrAPIKeyNotFound
	}
	repo, ok := s.apiKeyRepo.(APIKeySecretRepository)
	if !ok {
		return "", ErrAPIKeyRevealUnavailable
	}
	material, err := repo.GetAPIKeySecretForOwner(ctx, id, userID)
	if err != nil {
		return "", err
	}
	if material.KeyCiphertext == "" {
		raw := strings.TrimSpace(material.LegacyKey)
		if material.LookupHash != "" && raw == material.LookupHash {
			return "", ErrAPIKeyRevealUnavailable
		}
		if err := s.protectLegacyAPIKeySecret(ctx, id, raw); err != nil {
			return "", err
		}
		if !s.keyEncryptionStable || s.keyEncryptor == nil {
			return "", ErrAPIKeyRevealUnavailable
		}
		material, err = repo.GetAPIKeySecretForOwner(ctx, id, userID)
		if err != nil {
			return "", err
		}
	}
	if s.keyEncryptor == nil || !s.keyEncryptionStable {
		return "", ErrAPIKeyRevealUnavailable
	}
	raw, err := s.keyEncryptor.Decrypt(material.KeyCiphertext)
	if err != nil {
		return "", fmt.Errorf("decrypt api key: %w", err)
	}
	if material.LookupHash == "" || HashAPIKeyLookup(raw) != material.LookupHash {
		return "", fmt.Errorf("api key fingerprint mismatch")
	}
	return raw, nil
}
