package oauth

import (
	"sync"
	"testing"
	"time"
)

func TestSessionStore_Stop_Idempotent(t *testing.T) {
	store := NewSessionStore()

	store.Stop()
	store.Stop()

	select {
	case <-store.stopCh:
		// ok
	case <-time.After(time.Second):
		t.Fatal("stopCh 未关闭")
	}
}

func TestSessionStore_Stop_Concurrent(t *testing.T) {
	store := NewSessionStore()

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.Stop()
		}()
	}

	wg.Wait()

	select {
	case <-store.stopCh:
		// ok
	case <-time.After(time.Second):
		t.Fatal("stopCh 未关闭")
	}
}

func TestSessionStore_TryConsumeSessionMemoryIsSingleUse(t *testing.T) {
	store := NewSessionStore()
	defer store.Stop()
	store.Set("single-use", &OAuthSession{CreatedAt: time.Now()})

	results := make(chan bool, 32)
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- store.TryConsumeSession("single-use")
		}()
	}
	wg.Wait()
	close(results)

	winners := 0
	for consumed := range results {
		if consumed {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("successful consumers = %d, want 1", winners)
	}
}
