package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

// MediaService stores and serves media objects from a private S3-compatible bucket.
type MediaService struct {
	repo      MediaAssetRepository
	resolver  MediaStorageResolver
	signKey   []byte
	now       func() time.Time
	publicTTL time.Duration
}

func DeriveMediaSigningKey(jwtSecret string) []byte {
	sum := sha256.Sum256([]byte("sub2api/media-download/v1\x00" + jwtSecret))
	return sum[:]
}

func NewMediaService(repo MediaAssetRepository, resolver MediaStorageResolver, signingKey []byte) *MediaService {
	return &MediaService{
		repo:      repo,
		resolver:  resolver,
		signKey:   signingKey,
		now:       time.Now,
		publicTTL: DefaultMediaTTLMinutes * time.Minute,
	}
}

func MediaPublicPath(id int64) string {
	return fmt.Sprintf("/api/v1/media/public/%d", id)
}

// ParseManagedMediaID extracts a media_assets id from a public media path or URL.
func ParseManagedMediaID(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	pathPart := raw
	if parsed, err := url.Parse(raw); err == nil && parsed != nil && parsed.Path != "" {
		pathPart = parsed.Path
	}
	const prefix = "/api/v1/media/public/"
	if !strings.Contains(pathPart, prefix) {
		return 0, false
	}
	idStr := pathPart[strings.LastIndex(pathPart, prefix)+len(prefix):]
	idStr = strings.Trim(idStr, "/")
	if slash := strings.IndexByte(idStr, '/'); slash >= 0 {
		idStr = idStr[:slash]
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func (s *MediaService) Enabled(ctx context.Context) bool {
	if s == nil || s.resolver == nil {
		return false
	}
	_, _, err := s.resolver.Resolve(ctx)
	return err == nil
}

func MediaDownloadPath(id int64) string {
	return fmt.Sprintf("/api/v1/media/download/%d", id)
}

func mediaBizForcedPrivate(bizType string) bool {
	return bizType == MediaBizInvoice || bizType == MediaBizTicket
}

func validMediaBizType(bizType string) bool {
	switch bizType {
	case MediaBizInvoice, MediaBizTicket, MediaBizAvatar, MediaBizSupportQR, MediaBizAnnouncement, MediaBizImageTask:
		return true
	default:
		return false
	}
}

func clampMediaTTLMinutes(minutes, max int) int {
	if max <= 0 {
		max = MaxMediaTTLMinutes
	}
	if max > InvoiceEmailDownloadTTLMinutes {
		max = InvoiceEmailDownloadTTLMinutes
	}
	if minutes <= 0 {
		return DefaultMediaTTLMinutes
	}
	if minutes < MinMediaTTLMinutes {
		return MinMediaTTLMinutes
	}
	if minutes > max {
		return max
	}
	return minutes
}

func AbsoluteMediaURL(base, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" || !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func (s *MediaService) Upload(ctx context.Context, in UploadMediaInput) (*MediaAsset, error) {
	if len(in.Data) == 0 {
		return nil, ErrMediaEmptyUpload
	}
	if int64(len(in.Data)) > MaxMediaUploadBytes {
		return nil, ErrMediaTooLarge
	}
	bizType := strings.TrimSpace(in.BizType)
	if !validMediaBizType(bizType) {
		return nil, ErrMediaInvalidBizType
	}
	if (bizType == MediaBizInvoice || bizType == MediaBizSupportQR || bizType == MediaBizAnnouncement) && !in.ActorIsAdmin {
		return nil, ErrMediaAdminOnlyBiz
	}

	visibility := strings.TrimSpace(in.Visibility)
	if visibility == "" {
		visibility = MediaVisibilityPrivate
	}
	if visibility != MediaVisibilityPublic && visibility != MediaVisibilityPrivate {
		return nil, infraerrors.BadRequest("MEDIA_INVALID_VISIBILITY", "visibility must be public or private")
	}
	if mediaBizForcedPrivate(bizType) {
		visibility = MediaVisibilityPrivate
	}

	binding, store, err := s.resolver.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	if binding == nil || store == nil {
		return nil, ErrMediaStorageNotConfigured
	}

	sum := sha256.Sum256(in.Data)
	digest := hex.EncodeToString(sum[:])
	mime := sniffMediaMIME(in.Data)
	if !allowedMediaMIME(bizType, mime) {
		return nil, ErrMediaUnsupportedType
	}
	filename := sanitizeMediaFilename(in.Filename)
	key := buildMediaStorageKey(binding.Prefix, bizType, filename, s.now)

	if err := store.Put(ctx, key, mime, in.Data); err != nil {
		return nil, fmt.Errorf("put media object: %w", err)
	}

	profileID := binding.ProfileID
	if profileID == "" {
		profileID = domain.MediaStorageProfileBackup
	}
	asset := &MediaAsset{
		OwnerUserID:      in.OwnerUserID,
		BizType:          bizType,
		BizID:            strings.TrimSpace(in.BizID),
		StorageKey:       key,
		SHA256:           digest,
		MIME:             mime,
		Filename:         filename,
		Size:             int64(len(in.Data)),
		Visibility:       visibility,
		Status:           MediaStatusReady,
		StorageProfileID: profileID,
		CreatedAt:        s.now(),
	}
	if visibility == MediaVisibilityPublic {
		asset.PublicBaseURL = strings.TrimRight(binding.PublicBaseURL, "/")
	}

	if err := s.repo.Create(ctx, asset); err != nil {
		_ = store.Delete(ctx, key)
		return nil, err
	}
	applyMediaAccessURL(asset)
	return asset, nil
}

func applyMediaAccessURL(asset *MediaAsset) {
	if asset == nil {
		return
	}
	if asset.Visibility != MediaVisibilityPublic {
		asset.PublicBaseURL = ""
		asset.AccessURL = ""
		return
	}
	if base := strings.TrimRight(strings.TrimSpace(asset.PublicBaseURL), "/"); base != "" && strings.TrimSpace(asset.StorageKey) != "" {
		asset.AccessURL = base + "/" + escapeMediaObjectKeyPath(asset.StorageKey)
		return
	}
	asset.AccessURL = MediaPublicPath(asset.ID)
}

func escapeMediaObjectKeyPath(objectKey string) string {
	parts := strings.Split(objectKey, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func (s *MediaService) OpenPublic(ctx context.Context, id int64) (*MediaAsset, []byte, error) {
	asset, rc, err := s.OpenPublicStream(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rc.Close() }()
	body, err := io.ReadAll(io.LimitReader(rc, MaxMediaUploadBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if int64(len(body)) > MaxMediaUploadBytes {
		return nil, nil, ErrMediaTooLarge
	}
	return asset, body, nil
}

func (s *MediaService) OpenPublicStream(ctx context.Context, id int64) (*MediaAsset, io.ReadCloser, error) {
	asset, err := s.loadReady(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if asset.Visibility != MediaVisibilityPublic {
		return nil, nil, ErrMediaNotFound
	}
	rc, err := s.getObject(ctx, asset)
	if err != nil {
		return nil, nil, err
	}
	return asset, rc, nil
}

func (s *MediaService) CreateDownloadGrant(ctx context.Context, in CreateDownloadGrantInput) (*MediaDownloadGrant, error) {
	asset, err := s.loadReady(ctx, in.AssetID)
	if err != nil {
		return nil, err
	}
	if !in.ActorIsAdmin && !in.VerifiedAccess && asset.OwnerUserID != in.ActorUserID {
		return nil, ErrMediaForbidden
	}
	signKey, err := s.runtimeSigningKey(ctx)
	if err != nil {
		return nil, err
	}
	ttlMin := clampMediaTTLMinutes(in.TTLMinutes, in.MaxTTLMinutes)
	expiresAt := s.now().Add(time.Duration(ttlMin) * time.Minute).Unix()
	sig := signMediaDownload(signKey, in.AssetID, expiresAt)
	q := url.Values{}
	q.Set("expires", strconv.FormatInt(expiresAt, 10))
	q.Set("sig", sig)
	return &MediaDownloadGrant{
		URL:       MediaDownloadPath(in.AssetID) + "?" + q.Encode(),
		ExpiresAt: expiresAt,
		Sig:       sig,
		TTL:       ttlMin,
	}, nil
}

func (s *MediaService) OpenSignedDownload(ctx context.Context, in OpenSignedDownloadInput) (*MediaAsset, []byte, error) {
	asset, rc, err := s.OpenSignedDownloadStream(ctx, in)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rc.Close() }()
	body, err := io.ReadAll(io.LimitReader(rc, MaxMediaUploadBytes+1))
	if err != nil {
		return nil, nil, err
	}
	return asset, body, nil
}

func (s *MediaService) OpenSignedDownloadStream(ctx context.Context, in OpenSignedDownloadInput) (*MediaAsset, io.ReadCloser, error) {
	signKey, err := s.runtimeSigningKey(ctx)
	if err != nil {
		return nil, nil, err
	}
	if in.Expires <= 0 || strings.TrimSpace(in.Sig) == "" {
		return nil, nil, ErrMediaDownloadInvalid
	}
	if !validMediaDownloadSig(signKey, in.AssetID, in.Expires, in.Sig) {
		return nil, nil, ErrMediaDownloadInvalid
	}
	if s.now().Unix() > in.Expires {
		return nil, nil, ErrMediaDownloadExpired
	}
	asset, err := s.loadReady(ctx, in.AssetID)
	if err != nil {
		return nil, nil, err
	}
	rc, err := s.getObject(ctx, asset)
	if err != nil {
		return nil, nil, err
	}
	return asset, rc, nil
}

func (s *MediaService) loadReady(ctx context.Context, id int64) (*MediaAsset, error) {
	if id <= 0 {
		return nil, ErrMediaNotFound
	}
	asset, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if asset == nil || asset.Status != MediaStatusReady {
		return nil, ErrMediaNotFound
	}
	return asset, nil
}

func (s *MediaService) getObject(ctx context.Context, asset *MediaAsset) (io.ReadCloser, error) {
	_, store, err := s.resolver.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, ErrMediaStorageNotConfigured
	}
	return store.Get(ctx, asset.StorageKey)
}

func (s *MediaService) runtimeSigningKey(ctx context.Context) ([]byte, error) {
	if binding, _, err := s.resolver.Resolve(ctx); err == nil && binding != nil && strings.TrimSpace(binding.DownloadSigningSecret) != "" {
		return []byte(binding.DownloadSigningSecret), nil
	}
	if len(s.signKey) == 0 {
		return nil, infraerrors.InternalServer("MEDIA_SIGNING_KEY_MISSING", "media download signing key is not configured")
	}
	return s.signKey, nil
}

func signMediaDownload(key []byte, id, expires int64) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(mediaDownloadPayload(id, expires)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validMediaDownloadSig(key []byte, id, expires int64, sig string) bool {
	expected := signMediaDownload(key, id, expires)
	return hmac.Equal([]byte(expected), []byte(sig))
}

func (s *MediaService) OpenForActor(ctx context.Context, in OpenForActorInput) (*MediaAsset, io.ReadCloser, error) {
	asset, err := s.loadReady(ctx, in.AssetID)
	if err != nil {
		return nil, nil, err
	}
	if !in.ActorIsAdmin && !in.VerifiedAccess && asset.OwnerUserID != in.ActorUserID {
		return nil, nil, ErrMediaForbidden
	}
	rc, err := s.getObject(ctx, asset)
	if err != nil {
		return nil, nil, err
	}
	return asset, rc, nil
}

func (s *MediaService) GetForUser(ctx context.Context, requesterUserID, id int64) (*MediaAsset, error) {
	asset, err := s.loadReady(ctx, id)
	if err != nil {
		return nil, err
	}
	if asset.OwnerUserID != requesterUserID && asset.Visibility != MediaVisibilityPublic {
		return nil, ErrMediaForbidden
	}
	applyMediaAccessURL(asset)
	return asset, nil
}

func (s *MediaService) GetForAdmin(ctx context.Context, id int64) (*MediaAsset, error) {
	asset, err := s.loadReady(ctx, id)
	if err != nil {
		return nil, err
	}
	applyMediaAccessURL(asset)
	return asset, nil
}

func (s *MediaService) UpdateVisibility(ctx context.Context, id, actorUserID int64, isAdmin bool, visibility string) (*MediaAsset, error) {
	asset, err := s.loadReady(ctx, id)
	if err != nil {
		return nil, err
	}
	if !isAdmin && asset.OwnerUserID != actorUserID {
		return nil, ErrMediaForbidden
	}
	visibility = strings.TrimSpace(visibility)
	if visibility != MediaVisibilityPublic && visibility != MediaVisibilityPrivate {
		return nil, ErrMediaInvalidVisibility
	}
	if mediaBizForcedPrivate(asset.BizType) {
		visibility = MediaVisibilityPrivate
	}
	publicBase := ""
	if visibility == MediaVisibilityPublic {
		if binding, _, err := s.resolver.Resolve(ctx); err == nil && binding != nil {
			publicBase = strings.TrimRight(strings.TrimSpace(binding.PublicBaseURL), "/")
		}
	}
	if err := s.repo.UpdateVisibility(ctx, id, visibility, publicBase); err != nil {
		return nil, err
	}
	asset.Visibility = visibility
	asset.PublicBaseURL = publicBase
	applyMediaAccessURL(asset)
	return asset, nil
}

func (s *MediaService) Delete(ctx context.Context, id, actorUserID int64, isAdmin bool) error {
	asset, err := s.loadReady(ctx, id)
	if err != nil {
		return err
	}
	if !isAdmin && asset.OwnerUserID != actorUserID {
		return ErrMediaForbidden
	}
	if _, store, err := s.resolver.Resolve(ctx); err == nil && store != nil {
		_ = store.Delete(ctx, asset.StorageKey)
	}
	return s.repo.MarkDeleted(ctx, id)
}

func mediaDownloadPayload(id, expires int64) string {
	return fmt.Sprintf("%d:%d", id, expires)
}

func buildMediaStorageKey(prefix, bizType, filename string, now func() time.Time) string {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		prefix = "media"
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if len(ext) > 16 {
		ext = ""
	}
	when := time.Now().UTC()
	if now != nil {
		when = now().UTC()
	}
	return fmt.Sprintf("%s/%s/%s/%s%s", prefix, bizType, when.Format("2006/01"), uuid.NewString(), ext)
}

func sniffMediaMIME(data []byte) string {
	detected := http.DetectContentType(data)
	detected = strings.ToLower(strings.TrimSpace(strings.Split(detected, ";")[0]))
	if detected == "" {
		return "application/octet-stream"
	}
	return detected
}

func allowedMediaMIME(bizType, mime string) bool {
	mime = strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0]))
	switch mime {
	case "text/html", "application/xhtml+xml", "application/javascript", "text/javascript", "image/svg+xml", "text/xml", "application/xml":
		return false
	}
	switch bizType {
	case MediaBizInvoice:
		return mime == "application/pdf"
	case MediaBizAvatar, MediaBizSupportQR, MediaBizAnnouncement, MediaBizImageTask:
		return mime == "image/png" || mime == "image/jpeg" || mime == "image/gif" || mime == "image/webp"
	case MediaBizTicket:
		switch mime {
		case "application/pdf", "image/png", "image/jpeg", "image/gif", "image/webp", "text/plain", "text/csv", "application/zip":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func MediaContentDisposition(filename string) string {
	base := sanitizeMediaFilename(filename)
	return `attachment; filename="` + base + `"`
}

func sanitizeMediaFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, `"`, "_")
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "file"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == 0:
			b.WriteByte('_')
		case unicode.IsControl(r):
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "file"
	}
	return out
}

func MediaFilenameFromKey(storageKey string) string {
	return sanitizeMediaFilename(path.Base(storageKey))
}
