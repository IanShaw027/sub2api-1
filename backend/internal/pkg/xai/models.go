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
	DefaultImagineImageQualityModel  = "grok-imagine-image-quality"
	DefaultImagineImageFastModel     = "grok-imagine-image"
	DefaultImagineVideoModel         = "grok-imagine-video"
	DefaultImagineVideo15LegacyModel = "grok-imagine-video-1.5"
	DefaultImagineVideo15Model       = "grok-imagine-video-1.5-preview"
)

var defaultModels = []Model{
	// Text
	{ID: "grok-4.5", Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok 4.5"},
	{ID: "grok-4.3", Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok 4.3"},
	{ID: "grok-3-mini", Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok 3 Mini"},
	{ID: "grok-3-mini-fast", Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok 3 Mini Fast"},
	{ID: "grok-build-0.1", Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok Build 0.1"},
	{ID: "grok-4.20-0309-reasoning", Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok 4.20 Reasoning"},
	{ID: "grok-4.20-0309-non-reasoning", Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok 4.20 Non Reasoning"},
	{ID: "grok-4.20-multi-agent-0309", Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok 4.20 Multi Agent"},
	// Imagine (aligned to official pricing / model catalog)
	{ID: DefaultImagineImageQualityModel, Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok Imagine Image Quality"},
	{ID: DefaultImagineImageFastModel, Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok Imagine Image"},
	{ID: DefaultImagineVideoModel, Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok Imagine Video"},
	{ID: DefaultImagineVideo15Model, Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok Imagine Video 1.5 Preview"},
	{ID: DefaultImagineVideo15LegacyModel, Object: "model", Type: "model", OwnedBy: "xai", DisplayName: "Grok Imagine Video 1.5 Legacy"},
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
// grokTextResponsesModelAliases is the SINGLE source of truth for Grok text
// models that may be sent to the xAI Responses API: every client-facing /
// un-dated alias mapped to its canonical upstream ID. It backs BOTH the
// Responses model whitelist (DefaultModelMapping, which grok accounts fall back
// to when model_mapping is empty) AND IsGrokTextResponsesModelID, so adding a
// model here makes it simultaneously accepted (not rejected as
// group-model-unsupported) and forwarded to the correct upstream ID.
//
// Un-dated aliases matter because official clients send them: Grok CLI uses
// "grok-build" (main) and "grok-4.20-multi-agent" (its built-in web_search
// sub-call); Codex/opencode may request "grok-code-fast". See grok-build
// crates/codegen/xai-grok-models/default_models.json.
var grokTextResponsesModelAliases = map[string]string{
	"grok":                         DefaultTextModel,
	"grok-latest":                  DefaultTextModel,
	"grok-4.5":                     DefaultTextModel,
	"grok-4.5-latest":              DefaultTextModel,
	"grok-4.3":                     "grok-4.3",
	"grok-4.3-latest":              "grok-4.3",
	"grok-3-mini":                  "grok-3-mini",
	"grok-3-mini-fast":             "grok-3-mini-fast",
	"grok-build":                   "grok-build-0.1",
	"grok-build-latest":            "grok-build-0.1",
	"grok-build-0.1":               "grok-build-0.1",
	"grok-composer-2.5-fast":       "grok-composer-2.5-fast",
	"grok-composer":                "grok-composer-2.5-fast",
	"composer-2.5":                 "grok-composer-2.5-fast",
	"grok-code-fast":               "grok-code-fast-1-0825",
	"grok-code-fast-1":             "grok-code-fast-1-0825",
	"grok-code-fast-1-0825":        "grok-code-fast-1-0825",
	"grok-4.20-reasoning":          "grok-4.20-0309-reasoning",
	"grok-4.20-0309-reasoning":     "grok-4.20-0309-reasoning",
	"grok-4.20-non-reasoning":      "grok-4.20-0309-non-reasoning",
	"grok-4.20-0309-non-reasoning": "grok-4.20-0309-non-reasoning",
	"grok-4.20-multi-agent":        "grok-4.20-multi-agent-0309",
	"grok-4.20-multi-agent-latest": "grok-4.20-multi-agent-0309",
	"grok-4.20-multi-agent-0309":   "grok-4.20-multi-agent-0309",
}

func DefaultModelMapping() map[string]string {
	mapping := make(map[string]string, len(defaultModels)+len(grokTextResponsesModelAliases)+40)
	for _, model := range defaultModels {
		mapping[model.ID] = model.ID
	}
	// Grok text models + un-dated client aliases (shared source of truth).
	for alias, canonical := range grokTextResponsesModelAliases {
		mapping[alias] = canonical
	}
	// Imagine aliases / legacy IDs → official catalog.
	mapping["grok-imagine"] = DefaultImagineImageQualityModel
	mapping["grok-imagine-1"] = DefaultImagineImageQualityModel
	mapping["grok-imagine-edit"] = DefaultImagineImageQualityModel
	mapping["grok-imagine-image"] = DefaultImagineImageFastModel
	mapping["grok-imagine-image-quality"] = DefaultImagineImageQualityModel
	mapping["grok-imagine-video"] = DefaultImagineVideoModel
	mapping["grok-imagine-video-1.5"] = DefaultImagineVideo15Model
	mapping["grok-imagine-video-1.5-preview"] = DefaultImagineVideo15Model
	mapping["grok-video-1.5"] = DefaultImagineVideo15Model
	// Codex / OpenAI Responses client defaults (trailing-* wildcards only).
	// Covers gpt-5.5, gpt-5.3-codex, gpt-5.1-codex-mini, codex-auto-review, o3-mini, …
	mapping["gpt-*"] = DefaultTextModel
	mapping["codex-*"] = DefaultTextModel
	mapping["o1*"] = DefaultTextModel
	mapping["o3*"] = DefaultTextModel
	mapping["o4*"] = DefaultTextModel
	// Claude Code defaults (defense in depth; /v1/messages also remaps via group dispatch).
	mapping["claude-*"] = DefaultTextModel
	addGrokProviderPrefixedMappings(mapping)
	return mapping
}

func addGrokProviderPrefixedMappings(mapping map[string]string) {
	snapshot := make(map[string]string, len(mapping))
	for key, value := range mapping {
		snapshot[key] = value
	}
	for key, value := range snapshot {
		if !isGrokNativeOrAlias(key) {
			continue
		}
		for _, prefix := range []string{"xai/", "x-ai/", "grok/"} {
			mapping[prefix+key] = value
		}
	}
}

func isGrokNativeOrAlias(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(model, "grok") ||
		strings.HasPrefix(model, "imagine") ||
		strings.HasPrefix(model, "composer")
}

// StripGrokProviderPrefix removes common provider prefixes accepted by CPA for
// xAI/Grok models, returning the native model ID.
func StripGrokProviderPrefix(model string) string {
	trimmed := strings.TrimSpace(model)
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"xai/", "x-ai/", "grok/"} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(trimmed[len(prefix):])
		}
	}
	return trimmed
}

// IsGrokModelID reports whether model looks like a native Grok/xAI model id
// (including aliases). Claude/OpenAI model names return false.
func IsGrokModelID(model string) bool {
	normalized := strings.ToLower(StripGrokProviderPrefix(model))
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
// unknown custom grok-* IDs return false. Backed by the same
// grokTextResponsesModelAliases table as DefaultModelMapping so the whitelist
// and this helper never drift.
func IsGrokTextResponsesModelID(model string) bool {
	normalized := strings.ToLower(StripGrokProviderPrefix(model))
	_, ok := grokTextResponsesModelAliases[normalized]
	return ok
}

// ResolveDefaultTextModel returns DefaultTextModel when model is empty.
func ResolveDefaultTextModel(model string) string {
	if trimmed := strings.TrimSpace(model); trimmed != "" {
		return trimmed
	}
	return DefaultTextModel
}
