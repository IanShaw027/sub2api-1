package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	MaxCreationPublicationBytes  = MaxMediaUploadBytes
	CreationPublicationPending   = "pending"
	CreationPublicationPublished = "published"
)

var (
	ErrCreationPublicationNotFound = infraerrors.NotFound("CREATION_PUBLICATION_NOT_FOUND", "published creation not found")
	ErrCreationPublicationInvalid  = infraerrors.BadRequest("CREATION_PUBLICATION_INVALID", "invalid publication metadata or public visibility confirmation")
	ErrCreationPublicationConflict = infraerrors.Conflict("CREATION_PUBLICATION_CONFLICT", "request_id has already been used for another or withdrawn publication")
)

// Storage references are intentionally never serialized or exposed as public URLs.
type CreationPublication struct {
	ID               int64      `json:"id"`
	OwnerUserID      int64      `json:"owner_user_id"`
	RequestID        string     `json:"request_id"`
	Title            string     `json:"title"`
	Prompt           string     `json:"prompt"`
	Model            string     `json:"model"`
	Kind             string     `json:"kind"`
	Status           string     `json:"status"`
	MIME             string     `json:"mime"`
	Size             int64      `json:"size"`
	MediaURL         string     `json:"media_url"`
	CreatedAt        time.Time  `json:"created_at"`
	WithdrawnAt      *time.Time `json:"withdrawn_at,omitempty"`
	StorageKey       string     `json:"-"`
	StorageProfileID string     `json:"-"`
	SHA256           string     `json:"-"`
}

type PublishCreationInput struct {
	OwnerUserID int64
	RequestID   string
	Title       string
	Prompt      string
	Model       string
	Kind        string
	Visibility  string
	Data        []byte
}

type CreationPublicationFilter struct {
	Page     int
	PageSize int
	Kind     string
	Search   string
}

type CreationPublicationPage struct {
	Items    []*CreationPublication `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
	Pages    int64                  `json:"pages"`
}

type CreationPublicationRepository interface {
	GetByRequestID(ctx context.Context, ownerUserID int64, requestID string) (*CreationPublication, error)
	CreateIfAbsent(ctx context.Context, publication *CreationPublication) (*CreationPublication, bool, error)
	Activate(ctx context.Context, id int64) (*CreationPublication, error)
	GetPublic(ctx context.Context, id int64) (*CreationPublication, error)
	List(ctx context.Context, filter CreationPublicationFilter) ([]*CreationPublication, int64, error)
	Withdraw(ctx context.Context, ownerUserID, id int64) (*CreationPublication, error)
}
