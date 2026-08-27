package admin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"net/url"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "golang.org/x/image/webp"
)

const (
	settingsLogoMaxBytes        = 300 * 1024
	settingsSupportQRMaxBytes   = 500 * 1024
	settingsPaymentHelpMaxBytes = 300 * 1024
	settingsImageMaxDimension   = 8192
	settingsImageMaxPixels      = 16_000_000
	settingsMediaCleanupTimeout = 10 * time.Second
)

func (h *SettingHandler) ingestSettingsMedia(
	ctx context.Context,
	ownerUserID int64,
	sentFields map[string]json.RawMessage,
	req *UpdateSettingsRequest,
) (systemAssetIDs, paymentAssetIDs []int64, err error) {
	if req == nil {
		return nil, nil, nil
	}
	if _, ok := sentFields["support_qr_codes"]; ok && len(req.SupportQRCodes) > 8 {
		return nil, nil, infraerrors.BadRequest("SETTINGS_SUPPORT_QR_LIMIT", "support_qr_codes must contain at most 8 entries")
	}

	if _, ok := sentFields["site_logo"]; ok {
		var assetID int64
		req.SiteLogo, assetID, err = h.ingestSettingsImage(ctx, ownerUserID, service.MediaBizSiteLogo, "default", req.SiteLogo, "site-logo", settingsLogoMaxBytes)
		if assetID > 0 {
			systemAssetIDs = append(systemAssetIDs, assetID)
		}
		if err != nil {
			return systemAssetIDs, paymentAssetIDs, err
		}
	}

	if _, ok := sentFields["support_qr_codes"]; ok {
		for index := range req.SupportQRCodes {
			assetID := int64(0)
			req.SupportQRCodes[index].ImageURL, assetID, err = h.ingestSettingsImage(
				ctx,
				ownerUserID,
				service.MediaBizSupportQR,
				fmt.Sprintf("%d", index+1),
				req.SupportQRCodes[index].ImageURL,
				fmt.Sprintf("support-qr-%d", index+1),
				settingsSupportQRMaxBytes,
			)
			if assetID > 0 {
				systemAssetIDs = append(systemAssetIDs, assetID)
			}
			if err != nil {
				return systemAssetIDs, paymentAssetIDs, err
			}
		}
	}

	if _, ok := sentFields["payment_help_image_url"]; ok && req.PaymentHelpImageURL != nil {
		var assetID int64
		*req.PaymentHelpImageURL, assetID, err = h.ingestSettingsImage(
			ctx,
			ownerUserID,
			service.MediaBizPaymentHelp,
			"default",
			*req.PaymentHelpImageURL,
			"payment-help",
			settingsPaymentHelpMaxBytes,
		)
		if assetID > 0 {
			paymentAssetIDs = append(paymentAssetIDs, assetID)
		}
		if err != nil {
			return systemAssetIDs, paymentAssetIDs, err
		}
	}

	return systemAssetIDs, paymentAssetIDs, nil
}

func (h *SettingHandler) ingestSettingsImage(
	ctx context.Context,
	ownerUserID int64,
	bizType, bizID, raw, filenameStem string,
	maxBytes int64,
) (string, int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw, 0, nil
	}
	if !strings.HasPrefix(strings.ToLower(raw), "data:image/") {
		if err := validateSettingsImageReference(raw, bizType); err != nil {
			return "", 0, err
		}
		return raw, 0, nil
	}
	if h == nil || h.mediaService == nil || ownerUserID <= 0 {
		return "", 0, service.ErrMediaStorageNotConfigured
	}

	data, extension, err := decodeSettingsImageDataURL(raw, maxBytes)
	if err != nil {
		return "", 0, err
	}
	asset, err := h.mediaService.Upload(ctx, service.UploadMediaInput{
		OwnerUserID:  ownerUserID,
		BizType:      bizType,
		BizID:        bizID,
		Filename:     filenameStem + extension,
		Visibility:   service.MediaVisibilityPublic,
		ActorIsAdmin: true,
		Data:         data,
	})
	if err != nil {
		return "", 0, err
	}
	if strings.TrimSpace(asset.AccessURL) != "" {
		return asset.AccessURL, asset.ID, nil
	}
	return service.MediaPublicPath(asset.ID), asset.ID, nil
}

func validateSettingsImageReference(raw, bizType string) error {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if strings.HasPrefix(lower, "data:") {
		return service.ErrMediaUnsupportedType
	}
	if id, ok := service.ParseManagedMediaID(raw); ok && raw == service.MediaPublicPath(id) {
		return nil
	}
	if bizType == service.MediaBizSiteLogo && strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil || parsed.Host == "" ||
		(!strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https")) {
		return infraerrors.BadRequest("SETTINGS_IMAGE_URL_INVALID", "settings image URL must use http or https")
	}
	return nil
}

func decodeSettingsImageDataURL(raw string, maxBytes int64) ([]byte, string, error) {
	header, payload, ok := strings.Cut(strings.TrimSpace(raw), ",")
	if !ok {
		return nil, "", service.ErrMediaUnsupportedType
	}
	parts := strings.Split(header, ";")
	if len(parts) != 2 || !strings.EqualFold(parts[1], "base64") {
		return nil, "", service.ErrMediaUnsupportedType
	}
	mediaHeader := strings.ToLower(strings.TrimSpace(parts[0]))
	mime := strings.TrimPrefix(mediaHeader, "data:")
	extension := map[string]string{
		"image/png":  ".png",
		"image/jpeg": ".jpg",
		"image/gif":  ".gif",
		"image/webp": ".webp",
	}[mime]
	if extension == "" {
		return nil, "", service.ErrMediaUnsupportedType
	}
	payload = strings.TrimSpace(payload)
	decodedLen := base64.StdEncoding.DecodedLen(len(payload))
	if strings.HasSuffix(payload, "==") {
		decodedLen -= 2
	} else if strings.HasSuffix(payload, "=") {
		decodedLen--
	}
	if maxBytes > 0 && int64(decodedLen) > maxBytes {
		return nil, "", settingsImageTooLarge(maxBytes)
	}
	data, err := base64.StdEncoding.Strict().DecodeString(payload)
	if err != nil || len(data) == 0 {
		return nil, "", service.ErrMediaUnsupportedType
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, "", settingsImageTooLarge(maxBytes)
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return nil, "", service.ErrMediaUnsupportedType
	}
	actualMIME := map[string]string{
		"png":  "image/png",
		"jpeg": "image/jpeg",
		"gif":  "image/gif",
		"webp": "image/webp",
	}[strings.ToLower(format)]
	if actualMIME == "" || actualMIME != mime {
		return nil, "", service.ErrMediaUnsupportedType
	}
	if config.Width > settingsImageMaxDimension || config.Height > settingsImageMaxDimension ||
		int64(config.Width)*int64(config.Height) > settingsImageMaxPixels {
		return nil, "", infraerrors.BadRequest("MEDIA_IMAGE_DIMENSIONS_INVALID", "image dimensions exceed the allowed limit")
	}
	if _, decodedFormat, err := image.Decode(bytes.NewReader(data)); err != nil || !strings.EqualFold(decodedFormat, format) {
		return nil, "", service.ErrMediaUnsupportedType
	}
	return data, extension, nil
}

func settingsImageTooLarge(maxBytes int64) error {
	return infraerrors.BadRequest("MEDIA_TOO_LARGE", fmt.Sprintf("settings image exceeds %d KiB limit", maxBytes/1024))
}

func (h *SettingHandler) cleanupSettingsMedia(ctx context.Context, assetIDs []int64) {
	if h == nil || h.mediaService == nil {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), settingsMediaCleanupTimeout)
	defer cancel()
	for index := len(assetIDs) - 1; index >= 0; index-- {
		assetID := assetIDs[index]
		if assetID <= 0 {
			continue
		}
		if err := h.mediaService.Delete(cleanupCtx, assetID, 0, true); err != nil {
			slog.Warn("failed to clean up unreferenced settings media", "asset_id", assetID, "error", err)
		}
	}
}
