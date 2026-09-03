package service

import (
	"context"
	"encoding/json"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	APIKeyPurposeCreation   = "creation"
	creationInternalKeyName = "__creation_center__"

	CreationSessionModeChat  = "chat"
	CreationSessionModeImage = "image"

	CreationSessionStatusActive   = "active"
	CreationSessionStatusArchived = "archived"

	CreationMessageRoleUser      = "user"
	CreationMessageRoleAssistant = "assistant"
	CreationMessageRoleSystem    = "system"

	CreationImageJobStatusPending    = "pending"
	CreationImageJobStatusProcessing = "processing"
	CreationImageJobStatusCompleted  = "completed"
	CreationImageJobStatusFailed     = "failed"
)

var (
	ErrCreationCenterDisabled  = infraerrors.Forbidden("CREATION_CENTER_DISABLED", "creation center is disabled")
	ErrCreationSessionNotFound = infraerrors.NotFound("CREATION_SESSION_NOT_FOUND", "creation session not found")
	ErrCreationImageNotFound   = infraerrors.NotFound("CREATION_IMAGE_NOT_FOUND", "creation image not found")
	ErrCreationGroupRequired   = infraerrors.BadRequest("CREATION_GROUP_REQUIRED", "group_id is required")
	ErrCreationGroupNotAllowed = infraerrors.Forbidden("CREATION_GROUP_NOT_ALLOWED", "group is not allowed for this user")
	ErrCreationInvalidMode     = infraerrors.BadRequest("CREATION_INVALID_MODE", "invalid creation session mode")
)

type CreationSession struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"user_id"`
	GroupID   int64           `json:"group_id"`
	Title     string          `json:"title"`
	Model     string          `json:"model"`
	Mode      string          `json:"mode"`
	Status    string          `json:"status"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type CreationMessage struct {
	ID           int64           `json:"id"`
	SessionID    int64           `json:"session_id"`
	Role         string          `json:"role"`
	Content      json.RawMessage `json:"content"`
	Model        *string         `json:"model,omitempty"`
	InputTokens  *int            `json:"input_tokens,omitempty"`
	OutputTokens *int            `json:"output_tokens,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

type CreationImageJob struct {
	ID             int64     `json:"id"`
	SessionID      *int64    `json:"session_id,omitempty"`
	UserID         int64     `json:"user_id"`
	GroupID        int64     `json:"group_id"`
	Status         string    `json:"status"`
	Model          string    `json:"model"`
	Prompt         string    `json:"prompt"`
	MediaAssetID   *int64    `json:"media_asset_id,omitempty"`
	ProviderTaskID *string   `json:"provider_task_id,omitempty"`
	Error          *string   `json:"error,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	MediaURL       string    `json:"media_url,omitempty"`
}

type CreateCreationSessionInput struct {
	UserID   int64
	GroupID  int64
	Title    string
	Model    string
	Mode     string
	Metadata json.RawMessage
}

type UpdateCreationSessionInput struct {
	Title    *string
	Model    *string
	Status   *string
	Metadata json.RawMessage
	HasMeta  bool
}

type CreateCreationMessageInput struct {
	UserID    int64
	SessionID int64
	Role      string
	Content   json.RawMessage
	Model     *string
}

type CreationSessionListFilters struct {
	Mode     string
	Status   string
	Page     int
	PageSize int
}

type CreationImageListFilters struct {
	SessionID *int64
	Page      int
	PageSize  int
}

type CreateCreationImageJobInput struct {
	UserID         int64
	GroupID        int64
	SessionID      *int64
	Model          string
	Prompt         string
	ProviderTaskID string
	Status         string
}

type CreationSessionRepository interface {
	Create(ctx context.Context, input *CreationSession) error
	GetByID(ctx context.Context, id int64) (*CreationSession, error)
	GetForUser(ctx context.Context, userID, id int64) (*CreationSession, error)
	ListForUser(ctx context.Context, userID int64, filters CreationSessionListFilters) ([]CreationSession, *pagination.PaginationResult, error)
	Update(ctx context.Context, id int64, input UpdateCreationSessionInput) (*CreationSession, error)
	Delete(ctx context.Context, userID, id int64) error
}

type CreationMessageRepository interface {
	Create(ctx context.Context, msg *CreationMessage) error
	ListBySession(ctx context.Context, sessionID int64) ([]CreationMessage, error)
}

type CreationImageJobRepository interface {
	Create(ctx context.Context, job *CreationImageJob) error
	GetByID(ctx context.Context, id int64) (*CreationImageJob, error)
	GetForUser(ctx context.Context, userID, id int64) (*CreationImageJob, error)
	GetByProviderTaskID(ctx context.Context, userID int64, providerTaskID string) (*CreationImageJob, error)
	ListForUser(ctx context.Context, userID int64, filters CreationImageListFilters) ([]CreationImageJob, *pagination.PaginationResult, error)
	Update(ctx context.Context, id int64, job *CreationImageJob) error
}

func NormalizeCreationSessionMode(mode string) string {
	switch mode {
	case CreationSessionModeChat, CreationSessionModeImage:
		return mode
	default:
		return ""
	}
}

func NormalizeCreationSessionStatus(status string) string {
	switch status {
	case CreationSessionStatusActive, CreationSessionStatusArchived:
		return status
	default:
		return ""
	}
}

func NormalizeCreationMessageRole(role string) string {
	switch role {
	case CreationMessageRoleUser, CreationMessageRoleAssistant, CreationMessageRoleSystem:
		return role
	default:
		return ""
	}
}
