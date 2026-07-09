package xai

import "strings"

// Model describes an xAI model in OpenAI-compatible /models shape.
type Model struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	Type        string `json:"type,omitempty"`
	Created     int64  `json:"created,omitempty"`
	OwnedBy     string `json:"owned_by"`
	DisplayName string `json:"display_name,omitempty"`
}

// DefaultTextModel is the preferred general-purpose xAI text model for
// Claude Code / Codex / Grok CLI defaults and empty-model fallbacks.
// Official docs currently recommend Grok 4.5 for code and chat.
const DefaultTextModel = "grok-4.5"

// Official Imagine model IDs (https://docs.x.ai/docs/models).
const (
	DefaultImagineImageQualityModel = "grok-imagine-image-quality"
	DefaultImagineImageFastModel    = "grok-imagine-image"
	DefaultImagineVideoModel        = "grok-imagine-video"
	DefaultImagineVideo15Model      = "grok-imagine-video-1.5"
)

var defaultModels = []Model{
	// Text
	{ID: "grok-4.5", Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok 4.5"},
	{ID: "grok-4.3", Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok 4.3"},
	{ID: "grok-build-0.1", Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok Build 0.1"},
	{ID: "grok-4.20-0309-reasoning", Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok 4.20 Reasoning"},
	{ID: "grok-4.20-0309-non-reasoning", Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok 4.20 Non Reasoning"},
	{ID: "grok-4.20-multi-agent-0309", Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok 4.20 Multi Agent"},
	// Imagine (aligned to official pricing / model catalog)
	{ID: DefaultImagineImageQualityModel, Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok Imagine Image Quality"},
	{ID: DefaultImagineImageFastModel, Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok Imagine Image"},
	{ID: DefaultImagineVideoModel, Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok Imagine Video"},
	{ID: DefaultImagineVideo15Model, Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok Imagine Video 1.5"},
}

func DefaultModels() []Model {
	out := make([]Model, len(defaultModels))
	copy(out, defaultModels)
	return out
}

func DefaultModelIDs() []string {
	models := DefaultModels()
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}

// DefaultModelMapping returns identity mappings for known models plus common aliases.
// Explicit grok-* IDs map to themselves so client-chosen Grok models are not rewritten.
//
// OpenAI/Codex and Claude client model names (gpt-*, o3*, claude-*, …) map onto
// DefaultTextModel so Grok groups accept Codex CLI / Claude Code defaults without
// forcing users to rename models first. IsModelSupported uses this mapping as the
// account whitelist when credentials leave model_mapping empty.
func DefaultModelMapping() map[string]string {
	mapping := make(map[string]string, len(defaultModels)+40)
	for _, model := range defaultModels {
		mapping[model.ID] = model.ID
	}
	// Generic text aliases resolve to the current default text model.
	mapping["grok"] = DefaultTextModel
	mapping["grok-latest"] = DefaultTextModel
	mapping["grok-build"] = "grok-build-0.1"
	mapping["grok-4.20-reasoning"] = "grok-4.20-0309-reasoning"
	mapping["grok-4.20-non-reasoning"] = "grok-4.20-0309-non-reasoning"
	// Imagine aliases / legacy IDs → official catalog.
	mapping["grok-imagine"] = DefaultImagineImageQualityModel
	mapping["grok-imagine-1"] = DefaultImagineImageQualityModel
	mapping["grok-imagine-edit"] = DefaultImagineImageQualityModel
	mapping["grok-imagine-image"] = DefaultImagineImageFastModel
	mapping["grok-imagine-image-quality"] = DefaultImagineImageQualityModel
	mapping["grok-imagine-video"] = DefaultImagineVideoModel
	mapping["grok-imagine-video-1.5"] = DefaultImagineVideo15Model
	// Codex / OpenAI Responses client defaults (trailing-* wildcards only).
	// Covers gpt-5.5, gpt-5.3-codex, gpt-5.1-codex-mini, codex-auto-review, o3-mini, …
	mapping["gpt-*"] = DefaultTextModel
	mapping["codex-*"] = DefaultTextModel
	mapping["o1*"] = DefaultTextModel
	mapping["o3*"] = DefaultTextModel
	mapping["o4*"] = DefaultTextModel
	// Claude Code defaults (defense in depth; /v1/messages also remaps via group dispatch).
	mapping["claude-*"] = DefaultTextModel
	return mapping
}

// IsGrokModelID reports whether model looks like a native Grok/xAI model id
// (including aliases). Claude/OpenAI model names return false.
func IsGrokModelID(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if normalized == "" {
		return false
	}
	if strings.HasPrefix(normalized, "grok") {
		return true
	}
	// Imagine family may omit the grok- prefix in some aliases.
	if strings.HasPrefix(normalized, "imagine") {
		return true
	}
	return false
}

// IsGrokTextResponsesModelID reports whether model is a known Grok text model
// that can be sent to the xAI Responses API. Imagine image/video models and
// unknown custom grok-* IDs return false.
func IsGrokTextResponsesModelID(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	switch normalized {
	case DefaultTextModel,
		"grok",
		"grok-latest",
		"grok-4.5-latest",
		"grok-4.3",
		"grok-4.3-latest",
		"grok-build",
		"grok-build-0.1",
		"grok-code-fast",
		"grok-code-fast-1",
		"grok-code-fast-1-0825",
		"grok-4.20-0309-reasoning",
		"grok-4.20-reasoning",
		"grok-4.20-0309-non-reasoning",
		"grok-4.20-non-reasoning",
		"grok-4.20-multi-agent-0309":
		return true
	default:
		return false
	}
}

// ResolveDefaultTextModel returns DefaultTextModel when model is empty.
func ResolveDefaultTextModel(model string) string {
	if trimmed := strings.TrimSpace(model); trimmed != "" {
		return trimmed
	}
	return DefaultTextModel
}
