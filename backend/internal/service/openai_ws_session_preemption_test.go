package service

import (
	"context"
	"testing"

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
