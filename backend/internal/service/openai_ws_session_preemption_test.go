//go:build unit

package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIWSSessionPreemptRegistry_BeginCancelsPreviousSameSessionOnly(t *testing.T) {
	var registry openAIWSSessionPreemptRegistry
	key := openAIWSSessionPreemptKey{groupID: 7, apiKeyID: 11, sessionHash: "sess"}
	otherKey := openAIWSSessionPreemptKey{groupID: 7, apiKeyID: 12, sessionHash: "sess"}

	firstCtx, firstCancel := context.WithCancel(context.Background())
	firstCleanup, preemptedPrevious := registry.Begin(key, "req_1", firstCancel)
	t.Cleanup(firstCleanup)
	require.False(t, preemptedPrevious)

	otherCtx, otherCancel := context.WithCancel(context.Background())
	otherCleanup, preemptedPrevious := registry.Begin(otherKey, "req_other", otherCancel)
	t.Cleanup(otherCleanup)
	require.False(t, preemptedPrevious)

	secondCtx, secondCancel := context.WithCancel(context.Background())
	secondCleanup, preemptedPrevious := registry.Begin(key, "req_2", secondCancel)
	require.True(t, preemptedPrevious)

	require.ErrorIs(t, firstCtx.Err(), context.Canceled, "new same-session request must cancel the previous in-flight request")
	require.NoError(t, secondCtx.Err(), "current request must remain active")
	require.NoError(t, otherCtx.Err(), "different api-key/session key must not be canceled")

	firstCleanup()
	require.NoError(t, secondCtx.Err(), "stale cleanup must not remove or cancel the replacement registration")

	secondCleanup()
	require.NoError(t, secondCtx.Err(), "own cleanup unregisters without canceling the completed request")

	replacementCtx, replacementCancel := context.WithCancel(context.Background())
	replacementCleanup, preemptedPrevious := registry.Begin(key, "req_3", replacementCancel)
	defer replacementCleanup()
	require.False(t, preemptedPrevious)
	require.NoError(t, replacementCtx.Err(), "completed registration should not block later same-session requests")
}

type preemptOwnerCacheStub struct {
	mu           sync.Mutex
	payload      map[string][]byte
	claimCalls   int
	getCalls     int
	setCalls     int
	refreshCalls int
	lastTTL      time.Duration
}

func (c *preemptOwnerCacheStub) key(_ int64, sessionHash string) string {
	return sessionHash
}

func (c *preemptOwnerCacheStub) GetSessionAccountID(context.Context, int64, string) (int64, error) {
	return 0, ErrGatewayCacheMiss
}
func (c *preemptOwnerCacheStub) SetSessionAccountID(context.Context, int64, string, int64, time.Duration) error {
	return nil
}
func (c *preemptOwnerCacheStub) RefreshSessionTTL(context.Context, int64, string, time.Duration) error {
	return nil
}
func (c *preemptOwnerCacheStub) DeleteSessionAccountID(context.Context, int64, string) error {
	return nil
}
func (c *preemptOwnerCacheStub) GetOpenAIResponsesSessionWindow(_ context.Context, groupID int64, sessionHash string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.getCalls++
	if c.payload == nil {
		return nil, ErrGatewayCacheMiss
	}
	v, ok := c.payload[c.key(groupID, sessionHash)]
	if !ok {
		return nil, ErrGatewayCacheMiss
	}
	return append([]byte(nil), v...), nil
}
func (c *preemptOwnerCacheStub) SetOpenAIResponsesSessionWindow(_ context.Context, groupID int64, sessionHash string, payload []byte, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setCalls++
	if c.payload == nil {
		c.payload = make(map[string][]byte)
	}
	c.payload[c.key(groupID, sessionHash)] = append([]byte(nil), payload...)
	return nil
}

func (c *preemptOwnerCacheStub) ClaimOpenAIResponsesSessionWindow(_ context.Context, groupID int64, sessionHash string, owner []byte, ttl time.Duration) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.claimCalls++
	c.lastTTL = ttl
	if c.payload == nil {
		c.payload = make(map[string][]byte)
	}
	key := c.key(groupID, sessionHash)
	previous := append([]byte(nil), c.payload[key]...)
	c.payload[key] = append([]byte(nil), owner...)
	return previous, nil
}

func (c *preemptOwnerCacheStub) CompareAndRefreshOpenAIResponsesSessionWindow(_ context.Context, groupID int64, sessionHash string, expected []byte, ttl time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	current := c.payload[c.key(groupID, sessionHash)]
	if string(current) != string(expected) {
		return false, nil
	}
	c.refreshCalls++
	c.lastTTL = ttl
	return true, nil
}

func (c *preemptOwnerCacheStub) CompareAndDeleteOpenAIResponsesSessionWindow(_ context.Context, groupID int64, sessionHash string, expected []byte) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := c.key(groupID, sessionHash)
	if string(c.payload[key]) != string(expected) {
		return false, nil
	}
	delete(c.payload, key)
	return true, nil
}
func (c *preemptOwnerCacheStub) DeleteOpenAIResponsesSessionWindow(_ context.Context, groupID int64, sessionHash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.payload, c.key(groupID, sessionHash))
	return nil
}

func TestOpenAIWSSessionPreempt_RemoteOwnerWatchCancels(t *testing.T) {
	t.Parallel()
	cache := &preemptOwnerCacheStub{}
	svc := &OpenAIGatewayService{cache: cache}
	account := &Account{ID: 1, Type: AccountTypeOAuth}

	ctx, cleanup, armed, _ := svc.beginOpenAIWSSessionPreemptContext(
		context.Background(), account, 7, 11, "sess-a", false, "owner-1",
	)
	require.True(t, armed)
	defer cleanup()

	_, ok := svc.claimOpenAIWSSessionPreemptOwner(context.Background(), openAIWSSessionPreemptKey{
		groupID: 7, apiKeyID: 11, sessionHash: "sess-a",
	}, "owner-2")
	require.True(t, ok)

	require.Eventually(t, func() bool {
		return isOpenAIWSSessionPreempted(ctx)
	}, 3*time.Second, 50*time.Millisecond)
}

func TestOpenAIWSSessionPreempt_ConcurrentRemoteClaimsAreAtomic(t *testing.T) {
	t.Parallel()
	cache := &preemptOwnerCacheStub{}
	svc := &OpenAIGatewayService{cache: cache}
	key := openAIWSSessionPreemptKey{groupID: 7, apiKeyID: 11, sessionHash: "atomic-claim"}
	const contenders = 32
	start := make(chan struct{})
	type claimResult struct {
		previous string
		ok       bool
	}
	results := make(chan claimResult, contenders)
	var wg sync.WaitGroup
	for i := 0; i < contenders; i++ {
		owner := fmt.Sprintf("owner-%02d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			prior, ok := svc.claimOpenAIWSSessionPreemptOwner(context.Background(), key, owner)
			results <- claimResult{previous: prior, ok: ok}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	emptyCount := 0
	seen := make(map[string]struct{}, contenders)
	for result := range results {
		require.True(t, result.ok)
		prior := result.previous
		if prior == "" {
			emptyCount++
			continue
		}
		_, duplicate := seen[prior]
		require.False(t, duplicate)
		seen[prior] = struct{}{}
	}
	require.Equal(t, 1, emptyCount)
	require.Len(t, seen, contenders-1)
	cache.mu.Lock()
	require.Equal(t, contenders, cache.claimCalls)
	require.Equal(t, 0, cache.getCalls, "atomic-capable cache must not use GET+SET fallback")
	require.Equal(t, 0, cache.setCalls, "atomic-capable cache must not use GET+SET fallback")
	require.Equal(t, openAIWSSessionPreemptOwnerTTL, cache.lastTTL)
	cache.mu.Unlock()
}

func TestOpenAIWSSessionPreempt_StaleOwnerCannotReleaseReplacement(t *testing.T) {
	t.Parallel()
	cache := &preemptOwnerCacheStub{}
	svc := &OpenAIGatewayService{cache: cache}
	key := openAIWSSessionPreemptKey{groupID: 7, apiKeyID: 11, sessionHash: "owned-release"}

	previous, ok := svc.claimOpenAIWSSessionPreemptOwner(context.Background(), key, "owner-a")
	require.True(t, ok)
	require.Empty(t, previous)
	previous, ok = svc.claimOpenAIWSSessionPreemptOwner(context.Background(), key, "owner-b")
	require.True(t, ok)
	require.Equal(t, "owner-a", previous)

	svc.releaseOpenAIWSSessionPreemptOwner(context.Background(), key, "owner-a")
	payload, err := cache.GetOpenAIResponsesSessionWindow(context.Background(), key.groupID, openAIWSSessionPreemptCacheHash(key.apiKeyID, key.sessionHash))
	require.NoError(t, err)
	require.Equal(t, []byte("owner-b"), payload)

	svc.releaseOpenAIWSSessionPreemptOwner(context.Background(), key, "owner-b")
	_, err = cache.GetOpenAIResponsesSessionWindow(context.Background(), key.groupID, openAIWSSessionPreemptCacheHash(key.apiKeyID, key.sessionHash))
	require.ErrorIs(t, err, ErrGatewayCacheMiss)
}

func TestOpenAIWSSessionPreempt_WatchRefreshesCurrentOwnerLease(t *testing.T) {
	t.Parallel()
	cache := &preemptOwnerCacheStub{}
	svc := &OpenAIGatewayService{cache: cache}
	key := openAIWSSessionPreemptKey{groupID: 7, apiKeyID: 11, sessionHash: "owner-heartbeat"}
	_, ok := svc.claimOpenAIWSSessionPreemptOwner(context.Background(), key, "owner-a")
	require.True(t, ok)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var lost atomic.Bool
	stop := svc.watchOpenAIWSSessionPreemptOwner(ctx, key, "owner-a", func() { lost.Store(true) })
	defer stop()

	require.Eventually(t, func() bool {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		return cache.refreshCalls > 0 && cache.lastTTL == openAIWSSessionPreemptOwnerTTL
	}, 3*time.Second, 25*time.Millisecond)
	require.False(t, lost.Load())
}

func TestOpenAIWSSessionPreempt_SameRequestIDStillGetsUniqueRemoteOwner(t *testing.T) {
	t.Parallel()
	cache := &preemptOwnerCacheStub{}
	firstService := &OpenAIGatewayService{cache: cache}
	secondService := &OpenAIGatewayService{cache: cache}
	account := &Account{ID: 1, Type: AccountTypeOAuth}
	const groupID = int64(7)
	const apiKeyID = int64(11)
	const sessionHash = "duplicate-request-id"

	_, cleanupFirst, armed, preempted := firstService.beginOpenAIWSSessionPreemptContext(
		context.Background(), account, groupID, apiKeyID, sessionHash, false, "same-request-id",
	)
	require.True(t, armed)
	require.False(t, preempted)
	cacheHash := openAIWSSessionPreemptCacheHash(apiKeyID, sessionHash)
	firstOwner, err := cache.GetOpenAIResponsesSessionWindow(context.Background(), groupID, cacheHash)
	require.NoError(t, err)

	_, cleanupSecond, armed, preempted := secondService.beginOpenAIWSSessionPreemptContext(
		context.Background(), account, groupID, apiKeyID, sessionHash, false, "same-request-id",
	)
	require.True(t, armed)
	require.True(t, preempted)
	secondOwner, err := cache.GetOpenAIResponsesSessionWindow(context.Background(), groupID, cacheHash)
	require.NoError(t, err)
	require.NotEqual(t, firstOwner, secondOwner)
	require.Contains(t, string(firstOwner), "same-request-id:")
	require.Contains(t, string(secondOwner), "same-request-id:")

	cleanupFirst()
	current, err := cache.GetOpenAIResponsesSessionWindow(context.Background(), groupID, cacheHash)
	require.NoError(t, err)
	require.Equal(t, secondOwner, current, "stale cleanup must preserve the unique replacement owner")
	cleanupSecond()
	_, err = cache.GetOpenAIResponsesSessionWindow(context.Background(), groupID, cacheHash)
	require.ErrorIs(t, err, ErrGatewayCacheMiss)
}

func TestOpenAIWSSessionPreempt_LocalBeginStillCancelsPrevious(t *testing.T) {
	t.Parallel()
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 1, Type: AccountTypeOAuth}

	var firstCanceled atomic.Bool
	ctx1, cleanup1, armed1, _ := svc.beginOpenAIWSSessionPreemptContext(
		context.Background(), account, 1, 2, "sess-local", false, "req-1",
	)
	require.True(t, armed1)
	go func() {
		<-ctx1.Done()
		firstCanceled.Store(true)
	}()

	_, cleanup2, armed2, preempted := svc.beginOpenAIWSSessionPreemptContext(
		context.Background(), account, 1, 2, "sess-local", false, "req-2",
	)
	require.True(t, armed2)
	require.True(t, preempted)
	defer cleanup2()
	defer cleanup1()

	require.Eventually(t, firstCanceled.Load, time.Second, 10*time.Millisecond)
}
