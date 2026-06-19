package service

import (
	"context"
	"errors"
	"strings"
	"sync"
)

var errOpenAIWSSessionPreempted = errors.New("openai ws session preempted by newer request")

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

func (r *openAIWSSessionPreemptRegistry) Begin(key openAIWSSessionPreemptKey, requestID string, cancel func()) func() {
	if r == nil || strings.TrimSpace(key.sessionHash) == "" {
		return func() {}
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
	}
}

func (s *OpenAIGatewayService) beginOpenAIWSSessionPreemptContext(
	ctx context.Context,
	account *Account,
	groupID int64,
	apiKeyID int64,
	sessionHash string,
	httpIngressWSOneShot bool,
	requestID string,
) (context.Context, func(), bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || account == nil || account.Type != AccountTypeOAuth || httpIngressWSOneShot {
		return ctx, func() {}, false
	}
	key, ok := newOpenAIWSSessionPreemptKey(groupID, apiKeyID, sessionHash)
	if !ok {
		return ctx, func() {}, false
	}

	preemptCtx, cancel := context.WithCancelCause(ctx)
	cleanup := s.openaiWSSessionPreemptions.Begin(key, requestID, func() {
		cancel(errOpenAIWSSessionPreempted)
	})
	return preemptCtx, func() {
		cleanup()
		cancel(nil)
	}, true
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
