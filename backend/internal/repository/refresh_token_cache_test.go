package repository

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRefreshTokenCacheConsumeIsAtomic(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewRefreshTokenCache(rdb)
	ctx := context.Background()
	data := &service.RefreshTokenData{
		UserID:    42,
		FamilyID:  "family-42",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := cache.StoreRefreshToken(ctx, "token-hash", data, time.Hour); err != nil {
		t.Fatal(err)
	}

	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			consumed, err := cache.ConsumeRefreshToken(ctx, "token-hash", data.FamilyID, time.Hour)
			if err == nil {
				if consumed.FamilyID != data.FamilyID {
					t.Errorf("consumed family = %q, want %q", consumed.FamilyID, data.FamilyID)
				}
				successes.Add(1)
				return
			}
			if !errors.Is(err, service.ErrRefreshTokenNotFound) {
				t.Errorf("ConsumeRefreshToken error = %v", err)
			}
		}()
	}
	wg.Wait()

	if got := successes.Load(); got != 1 {
		t.Fatalf("successful consumers = %d, want exactly 1", got)
	}
	familyID, err := cache.GetUsedRefreshTokenFamily(ctx, "token-hash")
	if err != nil {
		t.Fatal(err)
	}
	if familyID != data.FamilyID {
		t.Fatalf("used marker family = %q, want %q", familyID, data.FamilyID)
	}
	markerCache, ok := cache.(service.RefreshTokenReuseMarkerCache)
	if !ok {
		t.Fatal("refresh token cache does not expose detailed reuse markers")
	}
	marker, err := markerCache.GetUsedRefreshTokenMarker(ctx, "token-hash")
	if err != nil {
		t.Fatal(err)
	}
	if marker.FamilyID != data.FamilyID {
		t.Fatalf("detailed marker family = %q, want %q", marker.FamilyID, data.FamilyID)
	}
	if marker.ConsumedAt.IsZero() {
		t.Fatal("detailed marker consumed_at is zero")
	}
	graceActive, err := markerCache.IsRefreshTokenReuseGraceActive(ctx, "token-hash")
	if err != nil {
		t.Fatal(err)
	}
	if !graceActive {
		t.Fatal("reuse grace key is not active immediately after consume")
	}
	mr.FastForward(service.RefreshTokenConcurrentReuseGrace + time.Millisecond)
	graceActive, err = markerCache.IsRefreshTokenReuseGraceActive(ctx, "token-hash")
	if err != nil {
		t.Fatal(err)
	}
	if graceActive {
		t.Fatal("reuse grace key remained active after the grace window")
	}
}
