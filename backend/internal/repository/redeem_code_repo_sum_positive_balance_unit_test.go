package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newRedeemCodeRepoSQLite(t *testing.T) (*redeemCodeRepository, *dbent.Client) {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	return &redeemCodeRepository{client: client}, client
}

func mustCreateRedeemCodeRepoUser(t *testing.T, ctx context.Context, client *dbent.Client, email string) *dbent.User {
	t.Helper()

	user, err := client.User.Create().
		SetEmail(email).
		SetPasswordHash("test-password-hash").
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	return user
}

func TestRedeemCodeRepositorySumPositiveBalanceByUserIncludesAdminBalance(t *testing.T) {
	repo, client := newRedeemCodeRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateRedeemCodeRepoUser(t, ctx, client, "sum-positive-balance@test.com")
	now := time.Now().UTC()

	_, err := client.RedeemCode.Create().
		SetCode("SUM-BALANCE").
		SetType(service.RedeemTypeBalance).
		SetStatus(service.StatusUsed).
		SetValue(10).
		SetNotes("").
		SetValidityDays(30).
		SetUsedBy(user.ID).
		SetUsedAt(now).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.RedeemCode.Create().
		SetCode("SUM-ADMIN-BALANCE").
		SetType(service.AdjustmentTypeAdminBalance).
		SetStatus(service.StatusUsed).
		SetValue(20).
		SetNotes("").
		SetValidityDays(30).
		SetUsedBy(user.ID).
		SetUsedAt(now).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.RedeemCode.Create().
		SetCode("SUM-NEGATIVE-BALANCE").
		SetType(service.RedeemTypeBalance).
		SetStatus(service.StatusUsed).
		SetValue(-5).
		SetNotes("").
		SetValidityDays(30).
		SetUsedBy(user.ID).
		SetUsedAt(now).
		Save(ctx)
	require.NoError(t, err)

	total, err := repo.SumPositiveBalanceByUser(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, float64(30), total)
}
