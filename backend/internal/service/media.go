package service

import (
	"context"
	"io"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	MediaVisibilityPublic  = domain.MediaVisibilityPublic
	MediaVisibilityPrivate = domain.MediaVisibilityPrivate

	MediaBizInvoice   = domain.MediaBizInvoice
	MediaBizTicket    = domain.MediaBizTicket
	MediaBizAvatar    = domain.MediaBizAvatar
	MediaBizImageTask = domain.MediaBizImageTask

	MediaStatusReady   = domain.MediaStatusReady
	MediaStatusDeleted = domain.MediaStatusDeleted

	MaxMediaUploadBytes     = 64 << 20
	DefaultMediaTTLMinutes  = 15
	MaxMediaTTLMinutes      = 60
	MinMediaTTLMinutes      = 1
	MediaPublicCacheSeconds = 300
)

var (
	ErrMediaStorageNotConfigured = infraerrors.ServiceUnavailable("MEDIA_STORAGE_NOT_CONFIGURED", "media object storage is not configured")
	ErrMediaNotFound             = infraerrors.NotFound("MEDIA_NOT_FOUND", "media asset not found")
	ErrMediaForbidden            = infraerrors.Forbidden("MEDIA_FORBIDDEN", "not allowed to access this media asset")
	ErrMediaDownloadExpired      = infraerrors.Forbidden("MEDIA_DOWNLOAD_EXPIRED", "download signature has expired")
	ErrMediaDownloadInvalid      = infraerrors.Forbidden("MEDIA_DOWNLOAD_INVALID", "download signature is invalid")
	ErrMediaTooLarge             = infraerrors.BadRequest("MEDIA_TOO_LARGE", "upload exceeds 64MiB limit")
	ErrMediaInvalidBizType       = infraerrors.BadRequest("MEDIA_INVALID_BIZ_TYPE", "unsupported media business type")
	ErrMediaEmptyUpload          = infraerrors.BadRequest("MEDIA_EMPTY_UPLOAD", "upload file is required")
	ErrMediaUnsupportedType      = infraerrors.BadRequest("MEDIA_UNSUPPORTED_TYPE", "file type is not allowed for this media business type")
	ErrMediaAdminOnlyBiz         = infraerrors.Forbidden("MEDIA_ADMIN_ONLY", "this media business type can only be uploaded by an administrator")
)

// MediaAsset is a stored object tracked in media_assets.
type MediaAsset struct {
	ID               int64     `json:"id"`
	OwnerUserID      int64     `json:"owner_user_id"`
	BizType          string    `json:"biz_type"`
	BizID            string    `json:"biz_id"`
	StorageKey       string    `json:"storage_key"`
	SHA256           string    `json:"sha256"`
	MIME             string    `json:"mime"`
	Filename         string    `json:"filename"`
	Size             int64     `json:"size"`
	Visibility       string    `json:"visibility"`
	Status           string    `json:"status"`
	StorageProfileID string    `json:"storage_profile_id"`
	PublicBaseURL    string    `json:"public_base_url,omitempty"`
	AccessURL        string    `json:"access_url,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// UploadMediaInput is a server-side upload (invoice/ticket/avatar).
type UploadMediaInput struct {
	OwnerUserID  int64
	BizType      string
	BizID        string
	Filename     string
	Visibility   string
	ActorIsAdmin bool
	Data         []byte
}

// CreateDownloadGrantInput issues a short-lived app HMAC download URL.
type CreateDownloadGrantInput struct {
	AssetID      int64
	ActorUserID  int64
	ActorIsAdmin bool
	// VerifiedAccess skips the owner check after a caller already authorized
	// the actor for this asset (for example a ticket attachment participant).
	VerifiedAccess bool
	TTLMinutes     int
}

// MediaDownloadGrant is a time-limited download URL that never exposes the S3 endpoint.
type MediaDownloadGrant struct {
	URL       string `json:"url"`
	ExpiresAt int64  `json:"expires_at"`
	Sig       string `json:"sig"`
	TTL       int    `json:"ttl_minutes"`
}

// OpenSignedDownloadInput verifies an HMAC download.
type OpenSignedDownloadInput struct {
	AssetID int64
	Expires int64
	Sig     string
}

// MediaStorageBinding describes which configured S3 profile media uses.
type MediaStorageBinding struct {
	ProfileID     string
	Prefix        string
	PublicBaseURL string
}

// MediaObjectStore is the object-storage port. Implementations must keep the bucket private
// (no public ACL) and encrypt objects at rest with SSE-S3 AES256.
type MediaObjectStore interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// MediaObjectStoreFactory builds a store from the existing backup S3 profile.
type MediaObjectStoreFactory func(ctx context.Context, cfg *BackupS3Config) (MediaObjectStore, error)

// MediaAssetRepository persists media_assets rows.
type MediaAssetRepository interface {
	Create(ctx context.Context, asset *MediaAsset) error
	GetByID(ctx context.Context, id int64) (*MediaAsset, error)
	ListByBiz(ctx context.Context, ownerUserID int64, bizType, bizID string) ([]MediaAsset, error)
}

// MediaStorageResolver selects the active S3 profile (backup by default).
type MediaStorageResolver interface {
	Resolve(ctx context.Context) (*MediaStorageBinding, MediaObjectStore, error)
}
