package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	stddraw "image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/google/uuid"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	defaultMediaMaxUploadSizeBytes = 64 << 20
	defaultMediaPresignTTL         = 15 * time.Minute
)

var errMediaBizIDInvalid = infraerrors.BadRequest("MEDIA_BIZ_ID_INVALID", "media biz_id is invalid")

type MediaService struct {
	repo  MediaRepository
	store MediaObjectStore
	cfg   *config.Config
}

func NewMediaService(repo MediaRepository, store MediaObjectStore, cfg *config.Config) *MediaService {
	return &MediaService{
		repo:  repo,
		store: store,
		cfg:   cfg,
	}
}

func (s *MediaService) Upload(ctx context.Context, input UploadMediaInput) (*MediaAsset, error) {
	if !s.isEnabled() {
		return nil, ErrMediaStorageDisabled
	}

	bizType, err := normalizeMediaBizType(input.BizType)
	if err != nil {
		return nil, err
	}
	visibility, err := normalizeMediaVisibility(input.Visibility)
	if err != nil {
		return nil, err
	}
	bizID, err := normalizeMediaBizID(input.BizID)
	if err != nil {
		return nil, err
	}
	if len(input.File) == 0 {
		return nil, ErrMediaFileRequired
	}
	maxUploadSize := s.maxUploadSizeBytes()
	if int64(len(input.File)) > maxUploadSize {
		return nil, ErrMediaTooLarge
	}
	if len(input.ThumbnailFile) > 0 && int64(len(input.ThumbnailFile)) > maxUploadSize {
		return nil, ErrMediaTooLarge
	}

	fileName := strings.TrimSpace(input.FileName)
	if fileName == "" {
		fileName = "upload"
	}
	contentType := strings.TrimSpace(input.ContentType)
	if contentType == "" {
		contentType = detectMediaContentType(input.File)
	}
	sizeBytes := input.SizeBytes
	if sizeBytes <= 0 {
		sizeBytes = int64(len(input.File))
	}
	shaValue := strings.TrimSpace(input.SHA256)
	if shaValue == "" {
		sum := sha256.Sum256(input.File)
		shaValue = hex.EncodeToString(sum[:])
	}

	width, height := input.Width, input.Height
	if width == nil || height == nil {
		width, height = detectImageDimensions(input.File)
	}

	objectKey, err := s.buildObjectKey(bizType, bizID, fileName, contentType)
	if err != nil {
		return nil, err
	}
	if err := s.store.Upload(ctx, s.bucket(), objectKey, input.File, contentType); err != nil {
		return nil, fmt.Errorf("upload media object: %w", err)
	}

	thumbnailObjectKey := ""
	thumbnailMIMEType := ""
	if len(input.ThumbnailFile) > 0 {
		thumbnailContentType := detectMediaContentType(input.ThumbnailFile)
		thumbnailName := strings.TrimSpace(input.ThumbnailFileName)
		if thumbnailName == "" {
			thumbnailName = "thumbnail"
		}
		var thumbErr error
		thumbnailObjectKey, thumbErr = s.buildObjectKey(bizType+"_thumbnail", bizID, thumbnailName, thumbnailContentType)
		if thumbErr != nil {
			_ = s.store.Delete(ctx, s.bucket(), objectKey)
			return nil, thumbErr
		}
		if err := s.store.Upload(ctx, s.bucket(), thumbnailObjectKey, input.ThumbnailFile, thumbnailContentType); err != nil {
			_ = s.store.Delete(ctx, s.bucket(), objectKey)
			return nil, fmt.Errorf("upload media thumbnail object: %w", err)
		}
		thumbnailMIMEType = thumbnailContentType
	} else if thumbnailBytes, thumbnailName, thumbnailContentType, thumbErr := generateMediaThumbnail(input.File, fileName); thumbErr == nil && len(thumbnailBytes) > 0 {
		var buildErr error
		thumbnailObjectKey, buildErr = s.buildObjectKey(bizType+"_thumbnail", bizID, thumbnailName, thumbnailContentType)
		if buildErr != nil {
			_ = s.store.Delete(ctx, s.bucket(), objectKey)
			return nil, buildErr
		}
		if err := s.store.Upload(ctx, s.bucket(), thumbnailObjectKey, thumbnailBytes, thumbnailContentType); err != nil {
			_ = s.store.Delete(ctx, s.bucket(), objectKey)
			return nil, fmt.Errorf("upload generated media thumbnail object: %w", err)
		}
		thumbnailMIMEType = thumbnailContentType
	}

	asset := &MediaAsset{
		BizType:            bizType,
		BizID:              bizID,
		Bucket:             s.bucket(),
		ObjectKey:          objectKey,
		ThumbnailObjectKey: thumbnailObjectKey,
		ThumbnailMIMEType:  thumbnailMIMEType,
		Visibility:         visibility,
		MIMEType:           contentType,
		SizeBytes:          sizeBytes,
		Width:              width,
		Height:             height,
		SHA256:             shaValue,
		OwnerUserID:        input.OwnerUserID,
		Status:             MediaStatusActive,
		OriginalFileName:   fileName,
	}
	if err := s.repo.Create(ctx, asset); err != nil {
		_ = s.store.Delete(ctx, s.bucket(), objectKey)
		if thumbnailObjectKey != "" {
			_ = s.store.Delete(ctx, s.bucket(), thumbnailObjectKey)
		}
		return nil, fmt.Errorf("create media asset: %w", err)
	}
	return asset, nil
}

func (s *MediaService) GetForUser(ctx context.Context, requesterUserID, id int64) (*MediaAsset, error) {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(asset, requesterUserID, false, true); err != nil {
		return nil, err
	}
	return asset, nil
}

func (s *MediaService) GetForAdmin(ctx context.Context, id int64) (*MediaAsset, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *MediaService) ListForAdmin(ctx context.Context, params pagination.PaginationParams, filters MediaListFilters) ([]MediaAsset, *pagination.PaginationResult, error) {
	filters.BizType = strings.TrimSpace(filters.BizType)
	filters.BizID = strings.TrimSpace(filters.BizID)
	filters.Visibility = strings.TrimSpace(filters.Visibility)
	filters.Status = strings.TrimSpace(filters.Status)
	filters.Search = strings.TrimSpace(filters.Search)
	if filters.Visibility != "" {
		visibility, err := normalizeMediaVisibility(filters.Visibility)
		if err != nil {
			return nil, nil, err
		}
		filters.Visibility = visibility
	}
	if filters.BizType != "" {
		bizType, err := normalizeMediaBizType(filters.BizType)
		if err != nil {
			return nil, nil, err
		}
		filters.BizType = bizType
	}
	if filters.Status != "" {
		status := normalizeMediaStatus(filters.Status)
		if status == "" {
			return nil, nil, ErrMediaInvalidStatus
		}
		filters.Status = status
	}
	return s.repo.List(ctx, params, filters)
}

func (s *MediaService) DeleteForUser(ctx context.Context, requesterUserID, id int64) error {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.authorize(asset, requesterUserID, false, false); err != nil {
		return err
	}
	return s.deleteAsset(ctx, asset)
}

func (s *MediaService) DeleteForAdmin(ctx context.Context, id int64) error {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return s.deleteAsset(ctx, asset)
}

func (s *MediaService) UpdateVisibilityForUser(ctx context.Context, requesterUserID, id int64, input UpdateMediaVisibilityInput) (*MediaAsset, error) {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(asset, requesterUserID, false, false); err != nil {
		return nil, err
	}
	return s.updateVisibility(ctx, asset, input)
}

func (s *MediaService) UpdateVisibilityForAdmin(ctx context.Context, id int64, input UpdateMediaVisibilityInput) (*MediaAsset, error) {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.updateVisibility(ctx, asset, input)
}

func (s *MediaService) CreateDownloadURLForUser(ctx context.Context, requesterUserID, id int64) (*MediaDownloadURL, error) {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(asset, requesterUserID, false, false); err != nil {
		return nil, err
	}
	return s.buildSignedDownloadURL(asset.ID, false), nil
}

func (s *MediaService) CreateDownloadURLForAdmin(ctx context.Context, id int64) (*MediaDownloadURL, error) {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.buildSignedDownloadURL(asset.ID, false), nil
}

func (s *MediaService) CreateThumbnailDownloadURLForUser(ctx context.Context, requesterUserID, id int64) (*MediaDownloadURL, error) {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(asset, requesterUserID, false, false); err != nil {
		return nil, err
	}
	return s.buildSignedDownloadURL(asset.ID, true), nil
}

func (s *MediaService) CreateThumbnailDownloadURLForAdmin(ctx context.Context, id int64) (*MediaDownloadURL, error) {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.buildSignedDownloadURL(asset.ID, true), nil
}

func (s *MediaService) OpenPublic(ctx context.Context, id int64, thumbnail bool) (*MediaObjectStream, *MediaAsset, error) {
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if asset.Visibility != MediaVisibilityPublic || asset.Status != MediaStatusActive {
		return nil, nil, ErrMediaForbidden
	}
	stream, err := s.openObject(ctx, asset, thumbnail)
	if err != nil {
		return nil, nil, err
	}
	return stream, asset, nil
}

func (s *MediaService) OpenSignedDownload(ctx context.Context, id int64, expiresAtUnix int64, signature string, thumbnail bool) (*MediaObjectStream, *MediaAsset, error) {
	if expiresAtUnix <= 0 {
		return nil, nil, ErrMediaSignatureInvalid
	}
	expiresAt := time.Unix(expiresAtUnix, 0)
	if time.Now().After(expiresAt) {
		return nil, nil, ErrMediaSignatureExpired
	}
	if !hmac.Equal([]byte(signature), []byte(s.downloadSignature(id, expiresAtUnix, thumbnail))) {
		return nil, nil, ErrMediaSignatureInvalid
	}
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	stream, err := s.openObject(ctx, asset, thumbnail)
	if err != nil {
		return nil, nil, err
	}
	return stream, asset, nil
}

func (s *MediaService) PublicURL(id int64, visibility string) string {
	if strings.TrimSpace(visibility) != MediaVisibilityPublic {
		return ""
	}
	base := strings.TrimRight(strings.TrimSpace(s.publicBaseURL()), "/")
	if base == "" {
		return ""
	}
	return fmt.Sprintf("%s/api/v1/media/public/%d", base, id)
}

func (s *MediaService) ThumbnailPublicURL(id int64, visibility string, thumbnailObjectKey string) string {
	if strings.TrimSpace(visibility) != MediaVisibilityPublic || strings.TrimSpace(thumbnailObjectKey) == "" {
		return ""
	}
	base := strings.TrimRight(strings.TrimSpace(s.publicBaseURL()), "/")
	if base == "" {
		return ""
	}
	return fmt.Sprintf("%s/api/v1/media/public/%d/thumbnail", base, id)
}

func (s *MediaService) RuntimeInfo() MediaRuntimeInfo {
	base := strings.TrimSpace(s.publicBaseURL())
	return MediaRuntimeInfo{
		Enabled:                   s.isEnabled(),
		Bucket:                    s.bucket(),
		PublicBaseURL:             base,
		SourceDomain:              base,
		PresignExpiryMinutes:      int(s.presignTTL() / time.Minute),
		MaxUploadSizeBytes:        s.maxUploadSizeBytes(),
		DefaultVisibility:         MediaVisibilityPrivate,
		UploadEndpoint:            "/api/v1/media/upload",
		PublicEndpointTemplate:    "/api/v1/media/public/{id}",
		ThumbnailEndpointTemplate: "/api/v1/media/public/{id}/thumbnail",
		DownloadEndpointTemplate:  "/api/v1/media/download/{id}",
		ThumbnailDownloadTemplate: "/api/v1/media/download/{id}/thumbnail",
		SupportedBizTypes:         []string{"avatar", "announcement", "ticket", "ai_image", "forum"},
		ThumbnailEnabled:          s.isEnabled(),
	}
}

func (s *MediaService) buildSignedDownloadURL(id int64, thumbnail bool) *MediaDownloadURL {
	ttl := s.presignTTL()
	expiresAt := time.Now().Add(ttl)
	expiresAtUnix := expiresAt.Unix()
	base := strings.TrimRight(strings.TrimSpace(s.publicBaseURL()), "/")
	pathSuffix := ""
	if thumbnail {
		pathSuffix = "/thumbnail"
	}
	return &MediaDownloadURL{
		URL:       fmt.Sprintf("%s/api/v1/media/download/%d%s?expires=%d&sig=%s", base, id, pathSuffix, expiresAtUnix, url.QueryEscape(s.downloadSignature(id, expiresAtUnix, thumbnail))),
		ExpiresAt: expiresAt,
	}
}

func (s *MediaService) deleteAsset(ctx context.Context, asset *MediaAsset) error {
	if asset == nil {
		return ErrMediaNotFound
	}
	if asset.Status == MediaStatusDeleted {
		return nil
	}
	if err := s.store.Delete(ctx, asset.Bucket, asset.ObjectKey); err != nil {
		return fmt.Errorf("delete media object: %w", err)
	}
	if strings.TrimSpace(asset.ThumbnailObjectKey) != "" {
		if err := s.store.Delete(ctx, asset.Bucket, asset.ThumbnailObjectKey); err != nil {
			return fmt.Errorf("delete media thumbnail object: %w", err)
		}
	}
	if err := s.repo.MarkDeleted(ctx, asset.ID, time.Now()); err != nil {
		return fmt.Errorf("mark media deleted: %w", err)
	}
	return nil
}

func (s *MediaService) updateVisibility(ctx context.Context, asset *MediaAsset, input UpdateMediaVisibilityInput) (*MediaAsset, error) {
	visibility, err := normalizeMediaVisibility(input.Visibility)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateVisibility(ctx, asset.ID, visibility); err != nil {
		return nil, fmt.Errorf("update media visibility: %w", err)
	}
	asset.Visibility = visibility
	asset.UpdatedAt = time.Now()
	return asset, nil
}

func (s *MediaService) authorize(asset *MediaAsset, requesterUserID int64, isAdmin bool, allowPublic bool) error {
	if asset == nil || asset.Status != MediaStatusActive {
		return ErrMediaNotFound
	}
	if isAdmin {
		return nil
	}
	if asset.OwnerUserID != nil && *asset.OwnerUserID == requesterUserID {
		return nil
	}
	if allowPublic && asset.Visibility == MediaVisibilityPublic {
		return nil
	}
	return ErrMediaForbidden
}

func (s *MediaService) openObject(ctx context.Context, asset *MediaAsset, thumbnail bool) (*MediaObjectStream, error) {
	if !s.isEnabled() {
		return nil, ErrMediaStorageDisabled
	}
	objectKey := asset.ObjectKey
	fileName := asset.OriginalFileName
	if thumbnail {
		if strings.TrimSpace(asset.ThumbnailObjectKey) == "" {
			return nil, ErrMediaNotFound
		}
		objectKey = asset.ThumbnailObjectKey
		fileName = "thumbnail-" + fileName
	}
	body, err := s.store.Download(ctx, asset.Bucket, objectKey)
	if err != nil {
		return nil, fmt.Errorf("download media object: %w", err)
	}
	sizeBytes := asset.SizeBytes
	if thumbnail {
		if statSize, statErr := s.store.Stat(ctx, asset.Bucket, objectKey); statErr == nil && statSize > 0 {
			sizeBytes = statSize
		}
	}
	contentType := asset.MIMEType
	if thumbnail && strings.TrimSpace(asset.ThumbnailMIMEType) != "" {
		contentType = asset.ThumbnailMIMEType
	}
	return &MediaObjectStream{
		Body:        body,
		ContentType: contentType,
		SizeBytes:   sizeBytes,
		FileName:    fileName,
	}, nil
}

func (s *MediaService) downloadSignature(id int64, expiresAtUnix int64, thumbnail bool) string {
	payload := fmt.Sprintf("%d:%d:%t", id, expiresAtUnix, thumbnail)
	mac := hmac.New(sha256.New, []byte(s.downloadSigningSecret()))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *MediaService) buildObjectKey(bizType, bizID, fileName, contentType string) (string, error) {
	now := time.Now().UTC()
	safeName := sanitizeMediaFileName(fileName)
	ext := strings.ToLower(filepath.Ext(safeName))
	if ext == "" {
		if guessedExts, _ := mime.ExtensionsByType(contentType); len(guessedExts) > 0 {
			ext = guessedExts[0]
		}
	}
	if ext == "" {
		ext = ".bin"
	}
	baseName := strings.TrimSuffix(safeName, filepath.Ext(safeName))
	if baseName == "" {
		baseName = "file"
	}
	normalizedBizType, err := normalizeMediaBizType(bizType)
	if err != nil {
		return "", err
	}
	normalizedBizID, err := normalizeMediaBizID(bizID)
	if err != nil {
		return "", err
	}
	return path.Join(
		normalizedBizType,
		normalizedBizID,
		now.Format("2006/01/02"),
		uuid.NewString()+"-"+baseName+ext,
	), nil
}

func (s *MediaService) isEnabled() bool {
	return s != nil && s.cfg != nil && s.cfg.Media.Enabled
}

func ParseManagedMediaID(mediaService *MediaService, raw string) (int64, bool) {
	if mediaService == nil {
		return 0, false
	}
	base := strings.TrimRight(strings.TrimSpace(mediaService.publicBaseURL()), "/")
	if base == "" {
		return 0, false
	}
	for _, prefix := range []string{
		base + "/api/v1/media/public/",
		base + "/api/v1/media/download/",
	} {
		if strings.HasPrefix(raw, prefix) {
			remainder := strings.TrimPrefix(raw, prefix)
			if idx := strings.IndexAny(remainder, "?#"); idx >= 0 {
				remainder = remainder[:idx]
			}
			if idx := strings.IndexRune(remainder, '/'); idx >= 0 {
				remainder = remainder[:idx]
			}
			id, err := strconv.ParseInt(strings.TrimSpace(remainder), 10, 64)
			if err == nil && id > 0 {
				return id, true
			}
		}
	}
	return 0, false
}

func (s *MediaService) bucket() string {
	return strings.TrimSpace(s.cfg.Media.Bucket)
}

func (s *MediaService) publicBaseURL() string {
	return strings.TrimSpace(s.cfg.Media.PublicBaseURL)
}

func (s *MediaService) downloadSigningSecret() string {
	secret := strings.TrimSpace(s.cfg.Media.DownloadSigningSecret)
	if secret != "" {
		return secret
	}
	return strings.TrimSpace(s.cfg.JWT.Secret)
}

func (s *MediaService) maxUploadSizeBytes() int64 {
	if s != nil && s.cfg != nil && s.cfg.Media.MaxUploadSizeBytes > 0 {
		return s.cfg.Media.MaxUploadSizeBytes
	}
	return defaultMediaMaxUploadSizeBytes
}

func (s *MediaService) presignTTL() time.Duration {
	if s != nil && s.cfg != nil && s.cfg.Media.PresignExpiryMinutes > 0 {
		return time.Duration(s.cfg.Media.PresignExpiryMinutes) * time.Minute
	}
	return defaultMediaPresignTTL
}

func normalizeMediaBizType(raw string) (string, error) {
	normalized := normalizeMediaToken(raw)
	if normalized == "" || normalized == "." || normalized == ".." {
		return "", ErrMediaBizTypeInvalid
	}
	return normalized, nil
}

func normalizeMediaBizID(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "unassigned", nil
	}
	normalized := normalizeMediaToken(raw)
	if normalized == "" || normalized == "." || normalized == ".." {
		return "", errMediaBizIDInvalid
	}
	return normalized, nil
}

func normalizeMediaVisibility(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", MediaVisibilityPrivate:
		return MediaVisibilityPrivate, nil
	case MediaVisibilityPublic:
		return MediaVisibilityPublic, nil
	default:
		return "", ErrMediaInvalidVisibility
	}
}

func normalizeMediaStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case MediaStatusActive:
		return MediaStatusActive
	case MediaStatusDeleted:
		return MediaStatusDeleted
	default:
		return ""
	}
}

func normalizeMediaToken(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(raw))
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			_, _ = builder.WriteRune(r)
		case r >= '0' && r <= '9':
			_, _ = builder.WriteRune(r)
		case r == '-', r == '_', r == '.':
			_, _ = builder.WriteRune(r)
		}
	}
	return builder.String()
}

func sanitizeMediaFileName(fileName string) string {
	name := filepath.Base(strings.TrimSpace(fileName))
	if name == "." || name == "/" || name == "" {
		return "file"
	}
	var builder strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			_, _ = builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			_, _ = builder.WriteRune(r)
		case r >= '0' && r <= '9':
			_, _ = builder.WriteRune(r)
		case r == '-', r == '_', r == '.':
			_, _ = builder.WriteRune(r)
		default:
			_ = builder.WriteByte('-')
		}
	}
	sanitized := strings.Trim(builder.String(), ".-")
	if sanitized == "" {
		return "file"
	}
	return sanitized
}

func generateMediaThumbnail(source []byte, originalFileName string) ([]byte, string, string, error) {
	src, _, err := image.Decode(bytes.NewReader(source))
	if err != nil {
		return nil, "", "", err
	}
	bounds := src.Bounds()
	if bounds.Empty() {
		return nil, "", "", fmt.Errorf("empty image")
	}
	const maxSide = 512
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, "", "", fmt.Errorf("invalid image dimensions")
	}
	scale := minFloat64(float64(maxSide)/float64(width), float64(maxSide)/float64(height))
	if scale > 1 {
		scale = 1
	}
	newWidth := max(1, int(float64(width)*scale))
	newHeight := max(1, int(float64(height)*scale))
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	stddraw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, stddraw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, stddraw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 82}); err != nil {
		return nil, "", "", err
	}
	name := strings.TrimSuffix(sanitizeMediaFileName(originalFileName), filepath.Ext(sanitizeMediaFileName(originalFileName)))
	if name == "" || name == "file" {
		name = "thumbnail"
	}
	return buf.Bytes(), name + ".jpg", "image/jpeg", nil
}

func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func detectMediaContentType(data []byte) string {
	if len(data) == 0 {
		return "application/octet-stream"
	}
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	return http.DetectContentType(sample)
}

func detectImageDimensions(data []byte) (*int, *int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, nil
	}
	width := cfg.Width
	height := cfg.Height
	return &width, &height
}
