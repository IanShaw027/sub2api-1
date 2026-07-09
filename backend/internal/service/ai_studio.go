package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrAIStudioDisabled = infraerrors.Forbidden(
	"AI_STUDIO_DISABLED", "ai studio feature is disabled",
)

// AIStudioRuntime is the lightweight view of the AI Studio feature switch.
type AIStudioRuntime struct {
	Enabled bool
}

// GetAIStudioRuntime reads the AI Studio feature switch directly from settings.
// Fail-closed: AI Studio is opt-in, so unknown settings mean disabled.
func (s *SettingService) GetAIStudioRuntime(ctx context.Context) AIStudioRuntime {
	if s == nil || s.settingRepo == nil {
		return AIStudioRuntime{Enabled: false}
	}
	vals, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyAIStudioEnabled})
	if err != nil {
		return AIStudioRuntime{Enabled: false}
	}
	return AIStudioRuntime{
		Enabled: vals[SettingKeyAIStudioEnabled] == "true",
	}
}
