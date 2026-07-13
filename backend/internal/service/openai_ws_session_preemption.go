package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var errOpenAIWSSessionPreempted = errors.New("openai ws session preempted by newer request")

const (
	openAIWSSessionPreemptOwnerTTL      = 2 * time.Hour
	openAIWSSessionPreemptWatchInterval = 2 * time.Second
	openAIWSSessionPreemptCachePrefix   = "wspreempt:"
)

func NewOpenAIWSSessionPreemptedError() error {
	return errOpenAIWSSessionPreempted
}

type openAIWSSessionPreemptKey struct {
	groupID     int64
	apiKeyID    int64
	sessionHash string
}

func newOpenAIWSSessionPreemptKey(groupID, apiKeyID int64, sessionHash string) (openAIWSSessionPreemptKey, bool) {
	sessionHash = strings.TrimSpace(sessionHash)
	if sessionHash == "" {
		return openAIWSSessionPreemptKey{}, false
	}
	return openAIWSSessionPreemptKey{groupID: groupID, apiKeyID: apiKeyID, sessionHash: sessionHash}, true
}

// openAIWSSessionPreemptCacheHash includes apiKeyID so different keys that share
// a session hash never cross-preempt across instances (matches local keying).
func openAIWSSessionPreemptCacheHash(apiKeyID int64, sessionHash string) string {
	return fmt.Sprintf("%s%d:%s", openAIWSSessionPreemptCachePrefix, apiKeyID, strings.TrimSpace(sessionHash))
}

// compareAndDeleteSessionWindowCache is an optional GatewayCache capability used
// for atomic WS preemption owner release. Implemented by the Redis gateway cache.
type compareAndDeleteSessionWindowCache interface {
	CompareAndDeleteOpenAIResponsesSessionWindow(ctx context.Context, groupID int64, sessionHash string, expected []byte) (bool, error)
}

type openAIWSSessionPreemptEntry struct {
	generation uint64
	requestID  string
	cancel     func()
}

type openAIWSSessionPreemptRegistry struct {
	mu     sync.Mutex
	next   uint64
	active map[openAIWSSessionPreemptKey]openAIWSSessionPreemptEntry
}

func (r *openAIWSSessionPreemptRegistry) Begin(key openAIWSSessionPreemptKey, requestID string, cancel func()) (func(), bool) {
	if r == nil || strings.TrimSpace(key.sessionHash) == "" {
		return func() {}, false
	}

	r.mu.Lock()
	if r.active == nil {
		r.active = make(map[openAIWSSessionPreemptKey]openAIWSSessionPreemptEntry)
	}
	r.next++
	generation := r.next
	previous, hadPrevious := r.active[key]
	r.active[key] = openAIWSSessionPreemptEntry{
		generation: generation,
		requestID:  strings.TrimSpace(requestID),
		cancel:     cancel,
	}
	r.mu.Unlock()

	if hadPrevious && previous.cancel != nil {
		previous.cancel()
	}

	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		current, ok := r.active[key]
		if !ok || current.generation != generation {
			return
		}
		delete(r.active, key)
	}, hadPrevious
}

func (s *OpenAIGatewayService) beginOpenAIWSSessionPreemptContext(
	ctx context.Context,
	account *Account,
	groupID int64,
	apiKeyID int64,
	sessionHash string,
	httpIngressWSOneShot bool,
	requestID string,
) (context.Context, func(), bool, bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || account == nil || account.Type != AccountTypeOAuth || httpIngressWSOneShot {
		return ctx, func() {}, false, false
	}
	key, ok := newOpenAIWSSessionPreemptKey(groupID, apiKeyID, sessionHash)
	if !ok {
		return ctx, func() {}, false, false
	}

	preemptCtx, cancel := context.WithCancelCause(ctx)
	ownerToken := strings.TrimSpace(requestID)
	if ownerToken == "" {
		ownerToken = fmt.Sprintf("gen-%d-%d", time.Now().UnixNano(), apiKeyID)
	}

	// Multi-instance: claim Redis ownership first. Previous owners (any pod)
	// watch the same key and self-cancel when ownership changes.
	previousRemoteOwner, claimedRemote := s.claimOpenAIWSSessionPreemptOwner(ctx, key, ownerToken)
	preemptedPrevious := previousRemoteOwner != "" && previousRemoteOwner != ownerToken

	cleanupLocal, hadLocalPrevious := s.openaiWSSessionPreemptions.Begin(key, ownerToken, func() {
		cancel(errOpenAIWSSessionPreempted)
	})
	if hadLocalPrevious {
		preemptedPrevious = true
	}

	stopWatch := s.watchOpenAIWSSessionPreemptOwner(preemptCtx, key, ownerToken, func() {
		cancel(errOpenAIWSSessionPreempted)
	})

	_ = claimedRemote
	return preemptCtx, func() {
		stopWatch()
		cleanupLocal()
		s.releaseOpenAIWSSessionPreemptOwner(context.Background(), key, ownerToken)
		cancel(nil)
	}, true, preemptedPrevious
}

func (s *OpenAIGatewayService) claimOpenAIWSSessionPreemptOwner(ctx context.Context, key openAIWSSessionPreemptKey, ownerToken string) (previous string, ok bool) {
	if s == nil || s.cache == nil || strings.TrimSpace(ownerToken) == "" {
		return "", false
	}
	cacheHash := openAIWSSessionPreemptCacheHash(key.apiKeyID, key.sessionHash)
	cacheCtx, cancel := context.WithTimeout(ctx, openAIWSStateStoreRedisTimeout)
	defer cancel()

	if payload, err := s.cache.GetOpenAIResponsesSessionWindow(cacheCtx, key.groupID, cacheHash); err == nil {
		previous = strings.TrimSpace(string(payload))
	}
	if err := s.cache.SetOpenAIResponsesSessionWindow(cacheCtx, key.groupID, cacheHash, []byte(ownerToken), openAIWSSessionPreemptOwnerTTL); err != nil {
		return previous, false
	}
	return previous, true
}

func (s *OpenAIGatewayService) releaseOpenAIWSSessionPreemptOwner(ctx context.Context, key openAIWSSessionPreemptKey, ownerToken string) {
	if s == nil || s.cache == nil || strings.TrimSpace(ownerToken) == "" {
		return
	}
	cacheHash := openAIWSSessionPreemptCacheHash(key.apiKeyID, key.sessionHash)
	cacheCtx, cancel := context.WithTimeout(ctx, openAIWSStateStoreRedisTimeout)
	defer cancel()
	expected := []byte(strings.TrimSpace(ownerToken))
	// Prefer atomic compare-and-delete so a newer claim cannot be wiped by a stale cleanup.
	if cad, ok := s.cache.(compareAndDeleteSessionWindowCache); ok && cad != nil {
		_, _ = cad.CompareAndDeleteOpenAIResponsesSessionWindow(cacheCtx, key.groupID, cacheHash, expected)
		return
	}
	// Fallback for test stubs without atomic CAD.
	if payload, err := s.cache.GetOpenAIResponsesSessionWindow(cacheCtx, key.groupID, cacheHash); err == nil {
		if strings.TrimSpace(string(payload)) != strings.TrimSpace(ownerToken) {
			return
		}
	}
	_ = s.cache.DeleteOpenAIResponsesSessionWindow(cacheCtx, key.groupID, cacheHash)
}

// watchOpenAIWSSessionPreemptOwner polls Redis ownership and invokes onLost when
// another instance (or local turn) claims the same session.
func (s *OpenAIGatewayService) watchOpenAIWSSessionPreemptOwner(ctx context.Context, key openAIWSSessionPreemptKey, ownerToken string, onLost func()) (stop func()) {
	if s == nil || s.cache == nil || onLost == nil || strings.TrimSpace(ownerToken) == "" {
		return func() {}
	}
	stopCh := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(openAIWSSessionPreemptWatchInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				cacheCtx, cancel := context.WithTimeout(context.Background(), openAIWSStateStoreRedisTimeout)
				payload, err := s.cache.GetOpenAIResponsesSessionWindow(cacheCtx, key.groupID, openAIWSSessionPreemptCacheHash(key.apiKeyID, key.sessionHash))
				cancel()
				if err != nil {
					continue
				}
				current := strings.TrimSpace(string(payload))
				if current == "" {
					// Key expired — treat as lost so we do not hold a zombie turn.
					onLost()
					return
				}
				if current != strings.TrimSpace(ownerToken) {
					onLost()
					return
				}
			}
		}
	}()
	return func() {
		once.Do(func() { close(stopCh) })
	}
}

func isOpenAIWSSessionPreempted(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	return errors.Is(context.Cause(ctx), errOpenAIWSSessionPreempted)
}

func IsOpenAIWSSessionPreemptedError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errOpenAIWSSessionPreempted) {
		return true
	}
	var fallbackErr *openAIWSFallbackError
	if !errors.As(err, &fallbackErr) || fallbackErr == nil {
		return false
	}
	reason := strings.TrimPrefix(strings.TrimSpace(fallbackErr.Reason), "prewarm_")
	return reason == "session_preempted"
}
