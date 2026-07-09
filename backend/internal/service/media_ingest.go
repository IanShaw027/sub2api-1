package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

const defaultMediaIngestTimeout = 30 * time.Second
const maxMediaIngestRedirects = 3

var (
	mediaIngestValidateHTTPURL    = validateMediaIngestHTTPURL
	mediaIngestValidateResolvedIP = urlvalidator.ValidateResolvedIP
	mediaIngestHTTPClient         = newMediaIngestHTTPClient
)

type IngestImageReferenceInput struct {
	BizType     string
	BizID       string
	Visibility  string
	OwnerUserID *int64
	Source      string
	FileName    string
	MaxBytes    int64
}

func (s *MediaService) IngestImageReference(ctx context.Context, input IngestImageReferenceInput) (*MediaAsset, error) {
	if s == nil {
		return nil, ErrMediaStorageDisabled
	}
	storageCfg := s.currentStorageConfig(ctx)
	if !storageCfg.Enabled {
		return nil, ErrMediaStorageDisabled
	}
	body, contentType, fileName, err := resolveImageReference(ctx, s.cfg, input.Source, input.FileName, input.MaxBytes)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(fileName) == "" {
		fileName = "image"
	}
	bizType, err := normalizeMediaBizType(input.BizType)
	if err != nil {
		return nil, err
	}
	bizID, err := normalizeMediaBizID(input.BizID)
	if err != nil {
		return nil, err
	}
	visibility, err := normalizeMediaVisibility(input.Visibility)
	if err != nil {
		return nil, err
	}
	contentType, width, height, err := validateIngestedImage(body, contentType)
	if err != nil {
		return nil, err
	}

	objectKey, err := s.buildObjectKey(storageCfg, bizType, bizID, fileName, contentType)
	if err != nil {
		return nil, err
	}
	if err := s.store.Upload(ctx, storageCfg, storageCfg.Bucket, objectKey, body, contentType); err != nil {
		return nil, fmt.Errorf("upload media object: %w", err)
	}

	sizeBytes := int64(len(body))
	sum := sha256SumHex(body)
	asset := &MediaAsset{
		BizType:          bizType,
		BizID:            bizID,
		StorageProfileID: storageCfg.ProfileID,
		Bucket:           storageCfg.Bucket,
		ObjectKey:        objectKey,
		Visibility:       visibility,
		MIMEType:         contentType,
		SizeBytes:        sizeBytes,
		Width:            width,
		Height:           height,
		SHA256:           sum,
		OwnerUserID:      input.OwnerUserID,
		Status:           MediaStatusActive,
		OriginalFileName: fileName,
	}
	if err := s.repo.Create(ctx, asset); err != nil {
		_ = s.store.Delete(ctx, storageCfg, storageCfg.Bucket, objectKey)
		return nil, fmt.Errorf("create media asset: %w", err)
	}
	return asset, nil
}

func resolveImageReference(ctx context.Context, cfg *config.Config, rawSource, fallbackName string, maxBytes int64) ([]byte, string, string, error) {
	source := strings.TrimSpace(rawSource)
	if source == "" {
		return nil, "", "", ErrMediaFileRequired
	}
	if strings.HasPrefix(strings.ToLower(source), "data:") {
		return decodeImageDataURL(source, fallbackName, maxBytes)
	}

	normalized, err := mediaIngestValidateHTTPURL(cfg, source)
	if err != nil {
		return nil, "", "", err
	}

	parsed, err := url.Parse(normalized)
	if err != nil {
		return nil, "", "", err
	}
	if err := validateMediaIngestPort(cfg, parsed); err != nil {
		return nil, "", "", err
	}
	if err := validateMediaIngestResolvedHost(cfg, parsed.Hostname()); err != nil {
		return nil, "", "", err
	}

	client, err := mediaIngestHTTPClient(cfg)
	if err != nil {
		return nil, "", "", err
	}
	client = withMediaIngestRedirectPolicy(client, cfg)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalized, nil)
	if err != nil {
		return nil, "", "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", "", fmt.Errorf("fetch image: unexpected status %d", resp.StatusCode)
	}

	limited := io.LimitedReader{R: resp.Body, N: maxIngestBytes(maxBytes) + 1}
	body, err := io.ReadAll(&limited)
	if err != nil {
		return nil, "", "", err
	}
	if int64(len(body)) > maxIngestBytes(maxBytes) {
		return nil, "", "", ErrMediaTooLarge
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}
	fileName := fallbackName
	if fileName == "" {
		fileName = path.Base(parsed.Path)
	}
	if fileName == "" || fileName == "." || fileName == "/" {
		fileName = "image"
	}
	if ext := strings.TrimSpace(path.Ext(fileName)); ext == "" && strings.HasPrefix(strings.ToLower(contentType), "image/") {
		if exts, _ := mime.ExtensionsByType(contentType); len(exts) > 0 {
			fileName += exts[0]
		}
	}
	return body, contentType, fileName, nil
}

func decodeImageDataURL(raw string, fallbackName string, maxBytes int64) ([]byte, string, string, error) {
	payload := strings.TrimPrefix(strings.TrimSpace(raw), "data:")
	meta, encoded, ok := strings.Cut(payload, ",")
	if !ok {
		return nil, "", "", ErrMediaFileRequired
	}
	meta = strings.TrimSpace(meta)
	encoded = strings.TrimSpace(encoded)
	if !strings.HasSuffix(strings.ToLower(meta), ";base64") {
		return nil, "", "", ErrMediaFileRequired
	}
	contentType := strings.TrimSpace(strings.TrimSuffix(meta, ";base64"))
	if contentType == "" {
		contentType = "image/png"
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, "", "", err
	}
	if int64(len(decoded)) > maxIngestBytes(maxBytes) {
		return nil, "", "", ErrMediaTooLarge
	}
	fileName := fallbackName
	if fileName == "" {
		if exts, _ := mime.ExtensionsByType(contentType); len(exts) > 0 {
			fileName = "image" + exts[0]
		} else {
			fileName = "image"
		}
	}
	return decoded, contentType, fileName, nil
}

func validateMediaIngestHTTPURL(cfg *config.Config, raw string) (string, error) {
	return urlvalidator.ValidateHTTPURL(raw, cfg != nil && cfg.Security.URLAllowlist.AllowInsecureHTTP, urlvalidator.ValidationOptions{
		AllowPrivate: cfg != nil && cfg.Security.URLAllowlist.AllowPrivateHosts,
	})
}

func newMediaIngestHTTPClient(cfg *config.Config) (*http.Client, error) {
	return httpclient.GetClient(mediaIngestHTTPClientOptions(cfg))
}

func withMediaIngestRedirectPolicy(client *http.Client, cfg *config.Config) *http.Client {
	if client == nil {
		return nil
	}
	cloned := *client
	previousCheckRedirect := client.CheckRedirect
	cloned.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxMediaIngestRedirects {
			return fmt.Errorf("redirect limit exceeded")
		}
		if req == nil || req.URL == nil {
			return fmt.Errorf("redirect target is invalid")
		}
		normalized, err := mediaIngestValidateHTTPURL(cfg, req.URL.String())
		if err != nil {
			return err
		}
		parsed, err := url.Parse(normalized)
		if err != nil {
			return err
		}
		if err := validateMediaIngestPort(cfg, parsed); err != nil {
			return err
		}
		if err := validateMediaIngestResolvedHost(cfg, parsed.Hostname()); err != nil {
			return err
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(req, via)
		}
		return nil
	}
	return &cloned
}

func mediaIngestHTTPClientOptions(cfg *config.Config) httpclient.Options {
	// Keep the transport aligned with the configured literal-host policy. DNS
	// rebinding protection is enforced explicitly per request above.
	return httpclient.Options{
		Timeout:            defaultMediaIngestTimeout,
		ValidateResolvedIP: true,
		AllowPrivateHosts:  cfg != nil && cfg.Security.URLAllowlist.AllowPrivateHosts,
	}
}

func validateIngestedImage(body []byte, _ string) (string, *int, *int, error) {
	contentType := detectMediaContentType(body)
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "image/") {
		return "", nil, nil, ErrMediaNotFound
	}
	width, height := detectImageDimensions(body)
	if width == nil || height == nil {
		return "", nil, nil, ErrMediaNotFound
	}
	return contentType, width, height, nil
}

func validateMediaIngestResolvedHost(cfg *config.Config, host string) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return fmt.Errorf("invalid host")
	}
	if cfg != nil && cfg.Security.URLAllowlist.AllowPrivateHosts && isPrivateLiteralHost(host) {
		return nil
	}
	return mediaIngestValidateResolvedIP(host)
}

func validateMediaIngestPort(cfg *config.Config, parsed *url.URL) error {
	if parsed == nil {
		return fmt.Errorf("invalid url")
	}
	port := strings.TrimSpace(parsed.Port())
	if port == "" {
		return nil
	}
	if cfg != nil && cfg.Security.URLAllowlist.AllowPrivateHosts && isPrivateLiteralHost(parsed.Hostname()) {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(parsed.Scheme)) {
	case "http":
		if port == "80" {
			return nil
		}
	case "https":
		if port == "443" {
			return nil
		}
	}
	return fmt.Errorf("remote image port %s is not allowed", port)
}

func isPrivateLiteralHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

func maxIngestBytes(maxBytes int64) int64 {
	if maxBytes > 0 {
		return maxBytes
	}
	return defaultMediaMaxUploadSizeBytes
}

func sha256SumHex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
