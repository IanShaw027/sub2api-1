package handler

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildOpenAISelectionFailureMessage_StripsInternalCause(t *testing.T) {
	err := fmt.Errorf("no available OpenAI accounts supporting model: foo: no available accounts")
	got := buildOpenAISelectionFailureMessage(err, "Service temporarily unavailable")
	require.Equal(t, "No available accounts supporting model: foo", got)
}

func TestBuildOpenAISelectionFailureMessage_NoAvailableAccounts(t *testing.T) {
	got := buildOpenAISelectionFailureMessage(service.ErrNoAvailableAccounts, "Service temporarily unavailable")
	require.Equal(t, "No available accounts", got)
}

func TestBuildOpenAISelectionFailureMessage_NilUsesFallback(t *testing.T) {
	got := buildOpenAISelectionFailureMessage(nil, "Service temporarily unavailable")
	require.Equal(t, "Service temporarily unavailable", got)
}

func TestBuildOpenAISelectionFailureMessage_CompactUsesFallback(t *testing.T) {
	got := buildOpenAISelectionFailureMessage(service.ErrNoAvailableCompactAccounts, "No available accounts support /responses/compact")
	require.Equal(t, "No available accounts support /responses/compact", got)
}

func TestBuildOpenAISelectionFailureMessage_UnknownUsesFallback(t *testing.T) {
	got := buildOpenAISelectionFailureMessage(errors.New("scheduler exploded"), "Service temporarily unavailable")
	require.Equal(t, "Service temporarily unavailable", got)
}

func TestBuildOpenAISelectionExhaustedMessage_LocalExclusionsKeepFallback(t *testing.T) {
	err := fmt.Errorf("no available accounts supporting model: foo")
	got := buildOpenAISelectionExhaustedMessage(err, "Service temporarily unavailable", true)
	require.Equal(t, "Service temporarily unavailable", got)
}
