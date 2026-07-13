//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBalanceCacheOutboxTriggerTracksEligibilityChanges(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	var userID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, balance, created_at, updated_at)
		VALUES ('balance-outbox-trigger@example.test', 'test', 0, NOW(), NOW())
		RETURNING id
	`).Scan(&userID))

	_, err := tx.ExecContext(ctx, "UPDATE users SET balance = 10 WHERE id = $1", userID)
	require.NoError(t, err)

	var count int
	require.NoError(t, tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM balance_cache_outbox WHERE user_id = $1", userID,
	).Scan(&count))
	require.Equal(t, 1, count)
}

func TestSchedulerOutboxJSONTriggerFiltersOperationalWrites(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	var accountID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO accounts (name, platform, type, credentials, extra, created_at, updated_at)
		VALUES ('scheduler-json-trigger-test', 'openai', 'oauth', '{}'::jsonb, '{}'::jsonb, NOW(), NOW())
		RETURNING id
	`).Scan(&accountID))
	clearAccountEvents := func() {
		_, err := tx.ExecContext(ctx, "DELETE FROM scheduler_outbox WHERE account_id = $1", accountID)
		require.NoError(t, err)
	}
	accountEventCount := func() int {
		var count int
		require.NoError(t, tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM scheduler_outbox
			WHERE account_id = $1 AND event_type = 'account_changed'
		`, accountID).Scan(&count))
		return count
	}

	clearAccountEvents()
	_, err := tx.ExecContext(ctx, `
		UPDATE accounts
		SET credentials = jsonb_set(credentials, '{access_token}', '"rotated"')
		WHERE id = $1
	`, accountID)
	require.NoError(t, err)
	require.Zero(t, accountEventCount())

	_, err = tx.ExecContext(ctx, `
		UPDATE accounts
		SET credentials = jsonb_set(credentials, '{model_mapping}', '{"gpt-x":"gpt-y"}')
		WHERE id = $1
	`, accountID)
	require.NoError(t, err)
	require.Equal(t, 1, accountEventCount())

	clearAccountEvents()
	_, err = tx.ExecContext(ctx, `
		UPDATE accounts
		SET extra = jsonb_set(extra, '{usage_updated_at}', '"now"')
		WHERE id = $1
	`, accountID)
	require.NoError(t, err)
	require.Zero(t, accountEventCount())

	_, err = tx.ExecContext(ctx, `
		UPDATE accounts
		SET extra = jsonb_set(extra, '{openai_ws_enabled}', 'true')
		WHERE id = $1
	`, accountID)
	require.NoError(t, err)
	require.Equal(t, 1, accountEventCount())
}
