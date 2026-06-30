//go:build unit

package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupModelUnsupportedError_ErrorTruncatesAvailableModels(t *testing.T) {
	models := make([]string, 0, 22)
	for i := 1; i <= 22; i++ {
		models = append(models, fmt.Sprintf("model-%02d", i))
	}

	err := &GroupModelUnsupportedError{
		RequestedModel:  "missing-model",
		AvailableModels: models,
	}

	message := err.Error()
	require.Contains(t, message, `requested model "missing-model"`)
	require.Contains(t, message, "model-01")
	require.Contains(t, message, "model-20")
	require.Contains(t, message, "and 2 more")
	require.NotContains(t, message, "model-21")
	require.NotContains(t, message, "model-22")
}

func TestAvailableRequestModelsFromAccounts_UsesMappingKeysAndDefaults(t *testing.T) {
	accounts := []Account{
		{
			ID:          1,
			Platform:    PlatformOpenAI,
			Status:      StatusActive,
			Schedulable: true,
		},
		{
			ID:          2,
			Platform:    PlatformOpenAI,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"custom-openai": "gpt-5.4",
				},
			},
		},
		{
			ID:          3,
			Platform:    PlatformOpenAI,
			Status:      StatusActive,
			Schedulable: false,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"disabled-model": "gpt-5.4",
				},
			},
		},
	}

	models := availableRequestModelsFromAccounts(accounts, PlatformOpenAI)

	require.Contains(t, models, "gpt-5.5")
	require.Contains(t, models, "custom-openai")
	require.NotContains(t, models, "disabled-model")
	require.True(t, sortStringsAreAscending(models), "models should be sorted: %v", models)
}

func TestAvailableRequestModelsFromAccounts_IncludesMixedAntigravityForAnthropic(t *testing.T) {
	accounts := []Account{
		{
			ID:          1,
			Platform:    PlatformAnthropic,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-fable-5": "claude-fable-5",
				},
			},
		},
		{
			ID:          2,
			Platform:    PlatformAntigravity,
			Status:      StatusActive,
			Schedulable: true,
			Extra:       map[string]any{"mixed_scheduling": true},
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"claude-sonnet-4-5": "claude-sonnet-4-5",
				},
			},
		},
		{
			ID:          3,
			Platform:    PlatformAntigravity,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"mixed-disabled": "claude-sonnet-4-5",
				},
			},
		},
		{
			ID:          4,
			Platform:    PlatformGemini,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"gemini-2.5-pro": "gemini-2.5-pro",
				},
			},
		},
	}

	models := availableRequestModelsFromAccounts(accounts, PlatformAnthropic)

	require.Contains(t, models, "claude-fable-5")
	require.Contains(t, models, "claude-sonnet-4-5")
	require.NotContains(t, models, "mixed-disabled")
	require.NotContains(t, models, "gemini-2.5-pro")
}

func TestNewGroupModelUnsupportedError_BuildsTypedError(t *testing.T) {
	err := newGroupModelUnsupportedError(PlatformGemini, "gemini-missing", []Account{
		{
			ID:          1,
			Platform:    PlatformGemini,
			Status:      StatusActive,
			Schedulable: true,
		},
	})

	var modelErr *GroupModelUnsupportedError
	require.ErrorAs(t, err, &modelErr)
	require.Equal(t, PlatformGemini, modelErr.Platform)
	require.Equal(t, "gemini-missing", modelErr.RequestedModel)
	require.Contains(t, modelErr.AvailableModels, "gemini-2.5-pro")
}

func TestNewGroupModelUnsupportedError_UsesGrokDefaults(t *testing.T) {
	err := newGroupModelUnsupportedError(PlatformGrok, "grok-missing", []Account{
		{
			ID:          1,
			Platform:    PlatformGrok,
			Status:      StatusActive,
			Schedulable: true,
		},
	})

	var modelErr *GroupModelUnsupportedError
	require.ErrorAs(t, err, &modelErr)
	require.Equal(t, PlatformGrok, modelErr.Platform)
	require.Equal(t, "grok-missing", modelErr.RequestedModel)
	require.Contains(t, modelErr.AvailableModels, "grok-4.3")
	require.NotContains(t, modelErr.AvailableModels, "claude-sonnet-4-6")
}

func TestNoAvailableOpenAICompatibleSelectionErrorWithRouting_UsesGrokPlatform(t *testing.T) {
	err := noAvailableOpenAICompatibleSelectionErrorWithRouting(context.Background(), nil, PlatformGrok, "grok-missing", false, false, []Account{
		{
			ID:          1,
			Platform:    PlatformGrok,
			Status:      StatusActive,
			Schedulable: true,
		},
	})

	var modelErr *GroupModelUnsupportedError
	require.ErrorAs(t, err, &modelErr)
	require.Equal(t, PlatformGrok, modelErr.Platform)
	require.Contains(t, modelErr.AvailableModels, "grok-4.3")
	require.NotContains(t, err.Error(), "OpenAI")

	err = noAvailableOpenAICompatibleSelectionErrorWithRouting(context.Background(), nil, PlatformGrok, "grok-missing", false, false)
	require.ErrorContains(t, err, "no available Grok accounts supporting model: grok-missing")
	require.NotContains(t, err.Error(), "OpenAI")
}

func sortStringsAreAscending(values []string) bool {
	return strings.Join(values, "\x00") == strings.Join(sortedStringCopy(values), "\x00")
}

func sortedStringCopy(values []string) []string {
	out := append([]string(nil), values...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
