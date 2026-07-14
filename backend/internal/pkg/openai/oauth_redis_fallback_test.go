package openai

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSessionStoreRedisFallbackIsLimitedToFailedWrites(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	store := NewRedisSessionStore(client)
	defer store.Stop()
	session := func(state string) *OAuthSession { return &OAuthSession{State: state, CreatedAt: time.Now()} }

	if err := store.Set("remote", session("remote")); err != nil {
		t.Fatalf("Set remote session: %v", err)
	}
	if err := store.remote.Delete(context.Background(), "remote"); err != nil {
		t.Fatalf("Delete remote session: %v", err)
	}
	if _, ok := store.Get("remote"); ok {
		t.Fatal("remote miss must not fall back to a stale local copy")
	}

	mr.Close()
	if err := store.Set("local-only", session("local")); err == nil {
		t.Fatal("expected Redis write failure")
	}
	if got, ok := store.Get("local-only"); !ok || got.State != "local" {
		t.Fatal("failed Redis write must remain available on the creating instance")
	}
	if !store.TryConsumeSession("local-only") || store.TryConsumeSession("local-only") {
		t.Fatal("local-only fallback must remain single-use")
	}
}
