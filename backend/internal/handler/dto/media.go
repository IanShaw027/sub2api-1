package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type MediaAsset struct {
	ID                 int64      `json:"id"`
	BizType            string     `json:"biz_type"`
	BizID              string     `json:"biz_id"`
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
	PublicURL          string     `json:"public_url,omitempty"`
	ThumbnailPublicURL string     `json:"thumbnail_public_url,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	DeletedAt          *time.Time `json:"deleted_at,omitempty"`
}

type MediaAssetPublic struct {
	ID                 int64     `json:"id"`
	BizType            string    `json:"biz_type"`
	BizID              string    `json:"biz_id"`
	Visibility         string    `json:"visibility"`
	MIMEType           string    `json:"mime_type"`
	SizeBytes          int64     `json:"size_bytes"`
	Width              *int      `json:"width,omitempty"`
	Height             *int      `json:"height,omitempty"`
	Status             string    `json:"status"`
	OriginalFileName   string    `json:"original_file_name"`
	PublicURL          string    `json:"public_url,omitempty"`
	ThumbnailPublicURL string    `json:"thumbnail_public_url,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type MediaDownloadURL struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func MediaAssetFromService(item *service.MediaAsset, publicURL, thumbnailPublicURL string) *MediaAsset {
	if item == nil {
		return nil
	}
	return &MediaAsset{
		ID:                 item.ID,
		BizType:            item.BizType,
		BizID:              item.BizID,
		Bucket:             item.Bucket,
		ObjectKey:          item.ObjectKey,
		ThumbnailObjectKey: item.ThumbnailObjectKey,
		ThumbnailMIMEType:  item.ThumbnailMIMEType,
		Visibility:         item.Visibility,
		MIMEType:           item.MIMEType,
		SizeBytes:          item.SizeBytes,
		Width:              item.Width,
		Height:             item.Height,
		SHA256:             item.SHA256,
		OwnerUserID:        item.OwnerUserID,
		Status:             item.Status,
		OriginalFileName:   item.OriginalFileName,
		PublicURL:          publicURL,
		ThumbnailPublicURL: thumbnailPublicURL,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
		DeletedAt:          item.DeletedAt,
	}
}

func MediaAssetPublicFromService(item *service.MediaAsset, publicURL, thumbnailPublicURL string) *MediaAssetPublic {
	if item == nil {
		return nil
	}
	return &MediaAssetPublic{
		ID:                 item.ID,
		BizType:            item.BizType,
		BizID:              item.BizID,
		Visibility:         item.Visibility,
		MIMEType:           item.MIMEType,
		SizeBytes:          item.SizeBytes,
		Width:              item.Width,
		Height:             item.Height,
		Status:             item.Status,
		OriginalFileName:   item.OriginalFileName,
		PublicURL:          publicURL,
		ThumbnailPublicURL: thumbnailPublicURL,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

func MediaDownloadURLFromService(item *service.MediaDownloadURL) *MediaDownloadURL {
	if item == nil {
		return nil
	}
	return &MediaDownloadURL{
		URL:       item.URL,
		ExpiresAt: item.ExpiresAt,
	}
}
