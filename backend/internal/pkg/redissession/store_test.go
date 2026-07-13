//go:build unit

package redissession

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type sampleSession struct {
	State       string    `json:"state"`
	CodeVerifier string   `json:"code_verifier"`
	CreatedAt   time.Time `json:"created_at"`
}

func newTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return New(rdb, "oauth:test", time.Minute), mr
}

func TestStore_SetGetDelete(t *testing.T) {
	t.Parallel()
	store, _ := newTestStore(t)
	ctx := context.Background()
	sess := sampleSession{State: "st", CodeVerifier: "cv", CreatedAt: time.Now().UTC()}
	require.NoError(t, store.Set(ctx, "sid-1", sess))

	var got sampleSession
	ok, err := store.Get(ctx, "sid-1", &got)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "st", got.State)

	require.NoError(t, store.Delete(ctx, "sid-1"))
	ok, err = store.Get(ctx, "sid-1", &got)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestStore_TryConsumeOnce(t *testing.T) {
	t.Parallel()
	store, _ := newTestStore(t)
	ctx := context.Background()
	require.NoError(t, store.Set(ctx, "sid-2", sampleSession{State: "a"}))

	ok, err := store.TryConsume(ctx, "sid-2")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = store.TryConsume(ctx, "sid-2")
	require.NoError(t, err)
	require.False(t, ok, "second consume must fail across instances")
}
