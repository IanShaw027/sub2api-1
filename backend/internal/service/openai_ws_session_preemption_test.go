//go:build unit

package service

import (
	"context"
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
	mu      sync.Mutex
	payload map[string][]byte
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
	if c.payload == nil {
		c.payload = make(map[string][]byte)
	}
	c.payload[c.key(groupID, sessionHash)] = append([]byte(nil), payload...)
	return nil
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
