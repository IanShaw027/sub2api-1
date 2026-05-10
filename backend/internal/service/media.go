package service

import (
	"context"
	"io"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	MediaVisibilityPublic  = "public"
	MediaVisibilityPrivate = "private"
)

const (
	MediaStatusActive  = "active"
	MediaStatusDeleted = "deleted"
)

var (
	ErrMediaStorageDisabled   = infraerrors.ServiceUnavailable("MEDIA_STORAGE_DISABLED", "media storage is disabled")
	ErrMediaNotFound          = infraerrors.NotFound("MEDIA_NOT_FOUND", "media asset not found")
	ErrMediaForbidden         = infraerrors.Forbidden("MEDIA_FORBIDDEN", "media asset is not accessible")
	ErrMediaInvalidVisibility = infraerrors.BadRequest("MEDIA_VISIBILITY_INVALID", "media visibility is invalid")
	ErrMediaInvalidStatus     = infraerrors.BadRequest("MEDIA_STATUS_INVALID", "media status is invalid")
	ErrMediaBizTypeInvalid    = infraerrors.BadRequest("MEDIA_BIZ_TYPE_INVALID", "media biz_type is invalid")
	ErrMediaFileRequired      = infraerrors.BadRequest("MEDIA_FILE_REQUIRED", "media file is required")
	ErrMediaTooLarge          = infraerrors.BadRequest("MEDIA_FILE_TOO_LARGE", "media file is too large")
	ErrMediaInvalidID         = infraerrors.BadRequest("MEDIA_ID_INVALID", "media asset id is invalid")
	ErrMediaSignatureInvalid  = infraerrors.Forbidden("MEDIA_SIGNATURE_INVALID", "media download signature is invalid")
	ErrMediaSignatureExpired  = infraerrors.Forbidden("MEDIA_SIGNATURE_EXPIRED", "media download signature has expired")
)

type MediaAsset struct {
	ID                 int64      `json:"id"`
	BizType            string     `json:"biz_type"`
	BizID              string     `json:"biz_id"`
	StorageProfileID   string     `json:"storage_profile_id,omitempty"`
	Bucket             string     `json:"bucket"`
	ObjectKey          string     `json:"object_key"`
	ThumbnailObjectKey string     `json:"thumbnail_object_key"`
	ThumbnailMIMEType  string     `json:"thumbnail_mime_type"`
	Visibility         string     `json:"visibility"`
	MIMEType           string     `json:"mime_type"`
	SizeBytes          int64      `json:"size_bytes"`
	Width              *int       `json:"width,omitempty"`
	Height             *int       `json:"height,omitempty"`
	SHA256             string     `json:"sha256"`
	OwnerUserID        *int64     `json:"owner_user_id,omitempty"`
	Status             string     `json:"status"`
	OriginalFileName   string     `json:"original_file_name"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	DeletedAt          *time.Time `json:"deleted_at,omitempty"`
}

type MediaListFilters struct {
	BizType     string
	BizID       string
	Visibility  string
	Status      string
	Search      string
	OwnerUserID *int64
}

type UploadMediaInput struct {
	BizType           string
	BizID             string
	Visibility        string
	OwnerUserID       *int64
	FileName          string
	ContentType       string
	SizeBytes         int64
	SHA256            string
	Width             *int
	Height            *int
	File              []byte
	ThumbnailFileName string
	ThumbnailFile     []byte
}

type UpdateMediaVisibilityInput struct {
	Visibility string
}

type MediaDownloadURL struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

type MediaRuntimeInfo struct {
	Enabled                   bool     `json:"enabled"`
	Bucket                    string   `json:"bucket,omitempty"`
	PublicBaseURL             string   `json:"public_base_url,omitempty"`
	SourceDomain              string   `json:"source_domain,omitempty"`
	PresignExpiryMinutes      int      `json:"presign_expiry_minutes"`
	MaxUploadSizeBytes        int64    `json:"max_upload_size_bytes"`
	DefaultVisibility         string   `json:"default_visibility"`
	UploadEndpoint            string   `json:"upload_endpoint"`
	PublicEndpointTemplate    string   `json:"public_endpoint_template"`
	ThumbnailEndpointTemplate string   `json:"thumbnail_endpoint_template"`
	DownloadEndpointTemplate  string   `json:"download_endpoint_template"`
	ThumbnailDownloadTemplate string   `json:"thumbnail_download_endpoint_template"`
	SupportedBizTypes         []string `json:"supported_biz_types"`
	ThumbnailEnabled          bool     `json:"thumbnail_enabled"`
}

type MediaObjectStream struct {
	Body        io.ReadCloser
	ContentType string
	SizeBytes   int64
	FileName    string
}

type MediaRepository interface {
	Create(ctx context.Context, asset *MediaAsset) error
	GetByID(ctx context.Context, id int64) (*MediaAsset, error)
	List(ctx context.Context, params pagination.PaginationParams, filters MediaListFilters) ([]MediaAsset, *pagination.PaginationResult, error)
	UpdateVisibility(ctx context.Context, id int64, visibility string) error
	MarkDeleted(ctx context.Context, id int64, deletedAt time.Time) error
}

type MediaObjectStore interface {
	Upload(ctx context.Context, cfg MediaStorageRuntimeConfig, bucket, objectKey string, body []byte, contentType string) error
	Download(ctx context.Context, cfg MediaStorageRuntimeConfig, bucket, objectKey string) (io.ReadCloser, error)
	Delete(ctx context.Context, cfg MediaStorageRuntimeConfig, bucket, objectKey string) error
	Stat(ctx context.Context, cfg MediaStorageRuntimeConfig, bucket, objectKey string) (int64, error)
}
