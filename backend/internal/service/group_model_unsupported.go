package service

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

const groupModelUnsupportedAvailableModelsLimit = 20

// GroupModelUnsupportedError indicates that otherwise schedulable accounts in
// the current group do not support the requested model.
type GroupModelUnsupportedError struct {
	Platform        string
	RequestedModel  string
	AvailableModels []string
}

func (e *GroupModelUnsupportedError) Error() string {
	if e == nil {
		return ""
	}
	return buildGroupModelUnsupportedMessage(e.RequestedModel, truncateModelListForError(e.AvailableModels))
}

func buildGroupModelUnsupportedMessage(requestedModel string, availableModels []string) string {
	requestedModel = strings.TrimSpace(requestedModel)
	message := fmt.Sprintf("The current group does not support the requested model %q", requestedModel)
	if len(availableModels) == 0 {
		return message
	}
	return fmt.Sprintf("%s. Available models: %s", message, strings.Join(availableModels, ", "))
}

func truncateModelListForError(models []string) []string {
	if len(models) <= groupModelUnsupportedAvailableModelsLimit {
		return cloneStringSlice(models)
	}
	out := cloneStringSlice(models[:groupModelUnsupportedAvailableModelsLimit])
	out = append(out, fmt.Sprintf("and %d more", len(models)-groupModelUnsupportedAvailableModelsLimit))
	return out
}

func defaultRequestModelIDsForPlatform(platform string) []string {
	switch platform {
	case PlatformOpenAI:
		return openai.DefaultModelIDs()
	case PlatformGemini:
		ids := make([]string, 0, len(geminicli.DefaultModels))
		for _, model := range geminicli.DefaultModels {
			ids = append(ids, model.ID)
		}
		return ids
	case PlatformAntigravity:
		models := antigravity.DefaultModels()
		ids := make([]string, 0, len(models))
		for _, model := range models {
			ids = append(ids, model.ID)
		}
		return ids
	default:
		return claude.DefaultModelIDs()
	}
}

func availableRequestModelsFromAccounts(accounts []Account, platform string) []string {
	modelSet := make(map[string]struct{})
	for i := range accounts {
		acc := &accounts[i]
		if !acc.IsSchedulable() || !accountMatchesModelListPlatform(acc, platform) {
			continue
		}
		mapping := acc.GetModelMapping()
		if len(mapping) == 0 {
			for _, model := range defaultRequestModelIDsForPlatform(platform) {
				if model = strings.TrimSpace(model); model != "" {
					modelSet[model] = struct{}{}
				}
			}
			continue
		}
		for model := range mapping {
			if model = strings.TrimSpace(model); model != "" {
				modelSet[model] = struct{}{}
			}
		}
	}
	if len(modelSet) == 0 {
		return nil
	}
	models := make([]string, 0, len(modelSet))
	for model := range modelSet {
		models = append(models, model)
	}
	sort.Strings(models)
	return models
}

func accountMatchesModelListPlatform(account *Account, platform string) bool {
	if account == nil {
		return false
	}
	switch platform {
	case PlatformAnthropic, PlatformGemini:
		return account.Platform == platform || (account.Platform == PlatformAntigravity && account.IsMixedSchedulingEnabled())
	default:
		return account.Platform == platform
	}
}

func newGroupModelUnsupportedError(platform string, requestedModel string, accounts []Account) error {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" || len(accounts) == 0 {
		return nil
	}
	return &GroupModelUnsupportedError{
		Platform:        platform,
		RequestedModel:  requestedModel,
		AvailableModels: availableRequestModelsFromAccounts(accounts, platform),
	}
}
