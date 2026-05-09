package domain

import (
	"slices"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	AIVisibilityPrivate  = "private"
	AIVisibilityUnlisted = "unlisted"
	AIVisibilityPublic   = "public"
)

const (
	AIModerationStateNormal        = "normal"
	AIModerationStateForcedPrivate = "forced_private"
	AIModerationStateBlocked       = "blocked"
)

const (
	AISessionStatusActive   = "active"
	AISessionStatusArchived = "archived"
)

const (
	AIMessageRoleSystem    = "system"
	AIMessageRoleUser      = "user"
	AIMessageRoleAssistant = "assistant"
	AIMessageRoleTool      = "tool"
)

const (
	AIMessageStatusAccepted  = "accepted"
	AIMessageStatusQueued    = "queued"
	AIMessageStatusCompleted = "completed"
	AIMessageStatusFailed    = "failed"
)

const (
	AIGenerationJobStatusQueued    = "queued"
	AIGenerationJobStatusRunning   = "running"
	AIGenerationJobStatusSucceeded = "succeeded"
	AIGenerationJobStatusFailed    = "failed"
	AIGenerationJobStatusCanceled  = "canceled"
)

const (
	AIAssetTypeImage = "image"
)

const (
	AIAssetStatusPending = "pending"
	AIAssetStatusReady   = "ready"
	AIAssetStatusHidden  = "hidden"
	AIAssetStatusDeleted = "deleted"
)

const (
	AIAuditEntitySession        = "ai_session"
	AIAuditEntitySessionMessage = "ai_session_message"
	AIAuditEntityPromptTemplate = "ai_prompt_template"
	AIAuditEntityGenerationJob  = "ai_generation_job"
	AIAuditEntityAsset          = "ai_asset"
)

var (
	ErrAISessionNotFound        = infraerrors.NotFound("AI_SESSION_NOT_FOUND", "ai session not found")
	ErrAISessionMessageNotFound = infraerrors.NotFound("AI_SESSION_MESSAGE_NOT_FOUND", "ai session message not found")
	ErrAIPromptTemplateNotFound = infraerrors.NotFound("AI_PROMPT_TEMPLATE_NOT_FOUND", "ai prompt template not found")
	ErrAIGenerationJobNotFound  = infraerrors.NotFound("AI_GENERATION_JOB_NOT_FOUND", "ai generation job not found")
	ErrAIAssetNotFound          = infraerrors.NotFound("AI_ASSET_NOT_FOUND", "ai asset not found")
)

func NormalizeAIVisibility(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case AIVisibilityPublic:
		return AIVisibilityPublic
	case AIVisibilityUnlisted:
		return AIVisibilityUnlisted
	default:
		return AIVisibilityPrivate
	}
}

func IsValidAIVisibility(raw string) bool {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	return slices.Contains([]string{
		AIVisibilityPrivate,
		AIVisibilityUnlisted,
		AIVisibilityPublic,
	}, normalized)
}

func NormalizeAIModerationState(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case AIModerationStateForcedPrivate:
		return AIModerationStateForcedPrivate
	case AIModerationStateBlocked:
		return AIModerationStateBlocked
	default:
		return AIModerationStateNormal
	}
}

func IsValidAIModerationState(raw string) bool {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	return slices.Contains([]string{
		AIModerationStateNormal,
		AIModerationStateForcedPrivate,
		AIModerationStateBlocked,
	}, normalized)
}

func EffectivePromptTemplateVisibility(visibility, moderationState string) string {
	switch NormalizeAIModerationState(moderationState) {
	case AIModerationStateBlocked, AIModerationStateForcedPrivate:
		return AIVisibilityPrivate
	default:
		return NormalizeAIVisibility(visibility)
	}
}

func CanReadPromptTemplate(ownerUserID, viewerUserID int64, isAdmin bool, visibility, moderationState string) bool {
	if isAdmin {
		return true
	}
	if ownerUserID > 0 && ownerUserID == viewerUserID {
		return true
	}

	if NormalizeAIModerationState(moderationState) != AIModerationStateNormal {
		return false
	}

	switch NormalizeAIVisibility(visibility) {
	case AIVisibilityPublic, AIVisibilityUnlisted:
		return true
	default:
		return false
	}
}

func CanListPromptTemplateInLibrary(ownerUserID, viewerUserID int64, visibility, moderationState string) bool {
	if ownerUserID > 0 && ownerUserID == viewerUserID {
		return true
	}
	return NormalizeAIModerationState(moderationState) == AIModerationStateNormal &&
		NormalizeAIVisibility(visibility) == AIVisibilityPublic
}

type AITraceRef struct {
	RequestID  string `json:"request_id,omitempty"`
	UsageLogID *int64 `json:"usage_log_id,omitempty"`
	APIKeyID   *int64 `json:"api_key_id,omitempty"`
	GroupID    *int64 `json:"group_id,omitempty"`
}

type AISession struct {
	ID            int64              `json:"id"`
	UserID        int64              `json:"user_id"`
	Title         string             `json:"title"`
	Status        string             `json:"status"`
	SystemPrompt  string             `json:"system_prompt,omitempty"`
	Metadata      map[string]any     `json:"metadata,omitempty"`
	LastMessageAt *time.Time         `json:"last_message_at,omitempty"`
	Trace         AITraceRef         `json:"trace"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	Messages      []AISessionMessage `json:"messages,omitempty"`
}

type AISessionMessage struct {
	ID               int64            `json:"id"`
	SessionID        int64            `json:"session_id"`
	UserID           int64            `json:"user_id"`
	ReplyToMessageID *int64           `json:"reply_to_message_id,omitempty"`
	Role             string           `json:"role"`
	Status           string           `json:"status"`
	Content          string           `json:"content"`
	ContentParts     []map[string]any `json:"content_parts,omitempty"`
	Model            string           `json:"model,omitempty"`
	Provider         string           `json:"provider,omitempty"`
	ErrorMessage     string           `json:"error_message,omitempty"`
	Metadata         map[string]any   `json:"metadata,omitempty"`
	Trace            AITraceRef       `json:"trace"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

type AIPromptTemplate struct {
	ID              int64                     `json:"id"`
	UserID          int64                     `json:"user_id"`
	Title           string                    `json:"title"`
	Description     string                    `json:"description,omitempty"`
	Category        string                    `json:"category,omitempty"`
	Tags            []string                  `json:"tags,omitempty"`
	Visibility      string                    `json:"visibility"`
	ModerationState string                    `json:"moderation_state"`
	CurrentVersion  int                       `json:"current_version"`
	ModelHint       string                    `json:"model_hint,omitempty"`
	Content         string                    `json:"content,omitempty"`
	Variables       []map[string]any          `json:"variables,omitempty"`
	Metadata        map[string]any            `json:"metadata,omitempty"`
	CoverAssetID    *int64                    `json:"cover_asset_id,omitempty"`
	Trace           AITraceRef                `json:"trace"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
	Versions        []AIPromptTemplateVersion `json:"versions,omitempty"`
}

type AIPromptTemplateVersion struct {
	ID         int64            `json:"id"`
	TemplateID int64            `json:"template_id"`
	UserID     int64            `json:"user_id"`
	Version    int              `json:"version"`
	Title      string           `json:"title"`
	Content    string           `json:"content"`
	ModelHint  string           `json:"model_hint,omitempty"`
	Variables  []map[string]any `json:"variables,omitempty"`
	ChangeNote string           `json:"change_note,omitempty"`
	Metadata   map[string]any   `json:"metadata,omitempty"`
	Trace      AITraceRef       `json:"trace"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

type AIGenerationJob struct {
	ID               int64          `json:"id"`
	UserID           int64          `json:"user_id"`
	SessionID        *int64         `json:"session_id,omitempty"`
	PromptTemplateID *int64         `json:"prompt_template_id,omitempty"`
	Status           string         `json:"status"`
	Model            string         `json:"model"`
	Prompt           string         `json:"prompt"`
	NegativePrompt   string         `json:"negative_prompt,omitempty"`
	Size             string         `json:"size,omitempty"`
	ImageCount       int            `json:"image_count"`
	Seed             *int64         `json:"seed,omitempty"`
	ErrorMessage     string         `json:"error_message,omitempty"`
	Parameters       map[string]any `json:"parameters,omitempty"`
	Trace            AITraceRef     `json:"trace"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	Assets           []AIAsset      `json:"assets,omitempty"`
}

type AIAsset struct {
	ID               int64          `json:"id"`
	UserID           int64          `json:"user_id"`
	GenerationJobID  *int64         `json:"generation_job_id,omitempty"`
	SessionID        *int64         `json:"session_id,omitempty"`
	PromptTemplateID *int64         `json:"prompt_template_id,omitempty"`
	AssetType        string         `json:"asset_type"`
	Status           string         `json:"status"`
	Visibility       string         `json:"visibility"`
	ModerationState  string         `json:"moderation_state"`
	StorageKind      string         `json:"storage_kind,omitempty"`
	StoragePath      string         `json:"storage_path,omitempty"`
	SourceURL        string         `json:"source_url,omitempty"`
	MIMEType         string         `json:"mime_type,omitempty"`
	Width            *int           `json:"width,omitempty"`
	Height           *int           `json:"height,omitempty"`
	ByteSize         *int64         `json:"byte_size,omitempty"`
	Checksum         string         `json:"checksum,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	Trace            AITraceRef     `json:"trace"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type AIAuditLog struct {
	ID             int64          `json:"id"`
	OperatorUserID *int64         `json:"operator_user_id,omitempty"`
	OwnerUserID    *int64         `json:"owner_user_id,omitempty"`
	EntityType     string         `json:"entity_type"`
	EntityID       *int64         `json:"entity_id,omitempty"`
	Action         string         `json:"action"`
	Reason         string         `json:"reason,omitempty"`
	BeforeState    map[string]any `json:"before_state,omitempty"`
	AfterState     map[string]any `json:"after_state,omitempty"`
	Trace          AITraceRef     `json:"trace"`
	CreatedAt      time.Time      `json:"created_at"`
}
