//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyRepositoryCreateStoresHashNotPlaintext(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "api-key-protection@test.com")
	raw := "sk-direct-repository-secret"
	key := &service.APIKey{
		UserID: user.ID,
		Key:    raw,
		Name:   "protected",
		Status: service.StatusActive,
	}

	require.NoError(t, repo.Create(ctx, key))
	stored, err := client.APIKey.Get(ctx, key.ID)
	require.NoError(t, err)
	require.Equal(t, service.HashAPIKeyLookup(raw), stored.Key)
	require.NotNil(t, stored.LookupHash)
	require.Equal(t, service.HashAPIKeyLookup(raw), *stored.LookupHash)
	require.Equal(t, service.APIKeyDisplayPrefix(raw), stored.KeyPrefix)
	require.Empty(t, stored.KeyCiphertext)
	require.NotContains(t, stored.String(), raw)

	authenticated, err := repo.GetByKeyForAuth(ctx, raw)
	require.NoError(t, err)
	require.Equal(t, key.ID, authenticated.ID)
	require.Equal(t, service.HashAPIKeyLookup(raw), authenticated.LookupHash)
	require.Empty(t, authenticated.Key)
}

func TestAPIKeyRepositoryLookupHashPreventsDeletedKeyReuse(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "api-key-reuse@test.com")
	raw := "sk-never-reuse-this-key"
	first := &service.APIKey{UserID: user.ID, Key: raw, Name: "first", Status: service.StatusActive}
	require.NoError(t, repo.Create(ctx, first))
	require.NoError(t, repo.Delete(ctx, first.ID))

	second := &service.APIKey{UserID: user.ID, Key: raw, Name: "second", Status: service.StatusActive}
	err := repo.Create(ctx, second)
	require.ErrorIs(t, err, service.ErrAPIKeyExists)
}

func TestListKeysByUserIDHashesUnmigratedLegacyRows(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "api-key-legacy-cache@test.com")
	raw := "sk-unmigrated-cache-key"

	_, err := client.APIKey.Create().
		SetUserID(user.ID).
		SetKey(raw).
		SetName("legacy").
		Save(ctx)
	require.NoError(t, err)

	hashes, err := repo.ListKeysByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, []string{service.HashAPIKeyLookup(raw)}, hashes)
}

func TestMigrateDeletedAPIKeyAuditHashesIsIdempotentAndLookupCompatible(t *testing.T) {
	repo, _ := newAPIKeyRepoSQLite(t)
	db := repo.sql.(*sql.DB)
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `
		DROP TABLE IF EXISTS deleted_api_key_audits;
		CREATE TABLE deleted_api_key_audits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key VARCHAR(128) NOT NULL,
			api_key_id BIGINT NOT NULL,
			user_id BIGINT NOT NULL,
			key_name VARCHAR(100) NOT NULL DEFAULT '',
			deleted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`)
	require.NoError(t, err)

	raw := "sk-historical-raw"
	oldBareRaw := "sk-old-bare-hash"
	oldBareHash := service.HashAPIKeyLookup(oldBareRaw)
	_, err = db.ExecContext(ctx, `
		INSERT INTO deleted_api_key_audits (key, api_key_id, user_id, key_name)
		VALUES (?, 1, 10, 'raw'), (?, 2, 20, 'bare')`, raw, oldBareHash)
	require.NoError(t, err)

	require.NoError(t, repo.MigrateDeletedAPIKeyAuditHashes(ctx))
	var firstRaw, firstBare string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT key FROM deleted_api_key_audits WHERE api_key_id = 1`).Scan(&firstRaw))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT key FROM deleted_api_key_audits WHERE api_key_id = 2`).Scan(&firstBare))
	require.Equal(t, hashDeletedAPIKeyAudit(raw), firstRaw)
	require.Equal(t, hashDeletedAPIKeyAudit(oldBareHash), firstBare)

	require.NoError(t, repo.MigrateDeletedAPIKeyAuditHashes(ctx))
	var secondRaw, secondBare string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT key FROM deleted_api_key_audits WHERE api_key_id = 1`).Scan(&secondRaw))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT key FROM deleted_api_key_audits WHERE api_key_id = 2`).Scan(&secondBare))
	require.Equal(t, firstRaw, secondRaw)
	require.Equal(t, firstBare, secondBare)

	opsRepo := NewOpsRepository(db).(*opsRepository)
	result, err := opsRepo.LookupDeletedKeyAudit(ctx, raw)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(10), result.UserID)
	result, err = opsRepo.LookupDeletedKeyAudit(ctx, oldBareRaw)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(20), result.UserID)
}
