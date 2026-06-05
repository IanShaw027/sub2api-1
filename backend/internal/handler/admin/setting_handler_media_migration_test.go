package admin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func asString(t *testing.T, v any) string {
	t.Helper()
	s, ok := v.(string)
	require.True(t, ok, "expected string, got %T", v)
	return s
}

type settingHandlerMediaRepoStub struct {
	nextID     int64
	assets     map[int64]*service.MediaAsset
	deletedIDs []int64
}

func (r *settingHandlerMediaRepoStub) Create(_ context.Context, asset *service.MediaAsset) error {
	r.nextID++
	asset.ID = r.nextID
	if r.assets == nil {
		r.assets = map[int64]*service.MediaAsset{}
	}
	assetCopy := *asset
	r.assets[asset.ID] = &assetCopy
	return nil
}

func (r *settingHandlerMediaRepoStub) GetByID(_ context.Context, id int64) (*service.MediaAsset, error) {
	if asset, ok := r.assets[id]; ok {
		assetCopy := *asset
		return &assetCopy, nil
	}
	return &service.MediaAsset{
		ID:         id,
		ObjectKey:  fmt.Sprintf("settings/stub/%d.png", id),
		Visibility: service.MediaVisibilityPublic,
		Status:     service.MediaStatusActive,
	}, nil
}

func (r *settingHandlerMediaRepoStub) List(context.Context, pagination.PaginationParams, service.MediaListFilters) ([]service.MediaAsset, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *settingHandlerMediaRepoStub) UpdateVisibility(context.Context, int64, string) error {
	return nil
}

func (r *settingHandlerMediaRepoStub) MarkDeleted(_ context.Context, id int64, _ time.Time) error {
	if asset, ok := r.assets[id]; ok && asset != nil {
		asset.Status = service.MediaStatusDeleted
	}
	r.deletedIDs = append(r.deletedIDs, id)
	return nil
}

func (r *settingHandlerMediaRepoStub) GetByObjectKey(_ context.Context, _, objectKey string) (*service.MediaAsset, error) {
	for _, asset := range r.assets {
		if asset != nil && asset.ObjectKey == objectKey {
			assetCopy := *asset
			return &assetCopy, nil
		}
	}
	return nil, service.ErrMediaNotFound
}

type settingHandlerMediaStoreStub struct {
	deleteCount int
}

func (*settingHandlerMediaStoreStub) Upload(context.Context, service.MediaStorageRuntimeConfig, string, string, []byte, string) error {
	return nil
}
func (*settingHandlerMediaStoreStub) Download(context.Context, service.MediaStorageRuntimeConfig, string, string) (io.ReadCloser, error) {
	return nil, nil
}
func (s *settingHandlerMediaStoreStub) Delete(context.Context, service.MediaStorageRuntimeConfig, string, string) error {
	s.deleteCount++
	return nil
}
func (*settingHandlerMediaStoreStub) Stat(context.Context, service.MediaStorageRuntimeConfig, string, string) (int64, error) {
	return 0, nil
}
func (*settingHandlerMediaStoreStub) PresignGetObject(_ context.Context, _ service.MediaStorageRuntimeConfig, _, objectKey string, _ time.Duration) (string, error) {
	return "https://media.example.com/presigned/" + objectKey, nil
}

type failingSettingHandlerRepoStub struct {
	settingHandlerRepoStub
	setMultipleErr error
}

func (s *failingSettingHandlerRepoStub) SetMultiple(context.Context, map[string]string) error {
	return s.setMultipleErr
}

type paymentConfigReadbackFailRepoStub struct {
	settingHandlerRepoStub
	getMultipleErr error
}

func (s *paymentConfigReadbackFailRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	for _, key := range keys {
		switch key {
		case service.SettingPaymentEnabled,
			service.SettingMinRechargeAmount,
			service.SettingMaxRechargeAmount,
			service.SettingDailyRechargeLimit,
			service.SettingOrderTimeoutMinutes,
			service.SettingMaxPendingOrders,
			service.SettingEnabledPaymentTypes,
			service.SettingBalancePayDisabled,
			service.SettingBalanceRechargeMult,
			service.SettingRechargeFeeRate,
			service.SettingLoadBalanceStrategy,
			service.SettingProductNamePrefix,
			service.SettingProductNameSuffix,
			service.SettingHelpImageURL,
			service.SettingHelpText,
			service.SettingCancelRateLimitOn,
			service.SettingCancelRateLimitMax,
			service.SettingCancelWindowSize,
			service.SettingCancelWindowUnit,
			service.SettingCancelWindowMode,
			service.SettingAlipayForceQRCode,
			service.SettingPaymentVisibleMethodAlipayEnabled,
			service.SettingPaymentVisibleMethodAlipaySource,
			service.SettingPaymentVisibleMethodWxpayEnabled,
			service.SettingPaymentVisibleMethodWxpaySource:
			return nil, s.getMultipleErr
		}
	}
	return s.settingHandlerRepoStub.GetMultiple(ctx, keys)
}

type paymentConfigWriteFailRepoStub struct {
	settingHandlerRepoStub
	setPaymentErr error
}

func (s *paymentConfigWriteFailRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	for key := range settings {
		switch key {
		case service.SettingPaymentEnabled,
			service.SettingMinRechargeAmount,
			service.SettingMaxRechargeAmount,
			service.SettingDailyRechargeLimit,
			service.SettingOrderTimeoutMinutes,
			service.SettingMaxPendingOrders,
			service.SettingEnabledPaymentTypes,
			service.SettingBalancePayDisabled,
			service.SettingBalanceRechargeMult,
			service.SettingRechargeFeeRate,
			service.SettingLoadBalanceStrategy,
			service.SettingProductNamePrefix,
			service.SettingProductNameSuffix,
			service.SettingHelpImageURL,
			service.SettingHelpText,
			service.SettingCancelRateLimitOn,
			service.SettingCancelRateLimitMax,
			service.SettingCancelWindowSize,
			service.SettingCancelWindowUnit,
			service.SettingCancelWindowMode,
			service.SettingAlipayForceQRCode,
			service.SettingPaymentVisibleMethodAlipayEnabled,
			service.SettingPaymentVisibleMethodAlipaySource,
			service.SettingPaymentVisibleMethodWxpayEnabled,
			service.SettingPaymentVisibleMethodWxpaySource:
			return s.setPaymentErr
		}
	}
	return s.settingHandlerRepoStub.SetMultiple(ctx, settings)
}

type rollbackFailureRepoStub struct {
	settingHandlerRepoStub
	callCount   int
	paymentErr  error
	rollbackErr error
}

func (s *rollbackFailureRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	s.callCount++
	switch s.callCount {
	case 1:
		return s.settingHandlerRepoStub.SetMultiple(ctx, settings)
	case 2:
		return s.paymentErr
	case 3:
		return s.rollbackErr
	default:
		return s.settingHandlerRepoStub.SetMultiple(ctx, settings)
	}
}

func buildSettingHandlerTestPNGDataURL(t *testing.T) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(16 * x), G: uint8(16 * y), B: 127, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestSettingHandler_UpdateSettings_MigratesMediaReferences(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled: "true",
			service.SettingKeySiteLogo:         "https://legacy.example/logo.png",
			service.SettingKeySupportQRCodes:   `[{"image_url":"https://legacy.example/qr.png","note":"help"}]`,
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	paymentCfgSvc := service.NewPaymentConfigService(nil, repo, nil)
	mediaSvc := service.NewMediaService(&settingHandlerMediaRepoStub{}, &settingHandlerMediaStoreStub{}, &config.Config{
		Media: config.MediaConfig{
			Enabled:            true,
			Endpoint:           "https://storage.example.com",
			Region:             "auto",
			Bucket:             "media",
			AccessKeyID:        "test-ak",
			SecretAccessKey:    "test-sk",
			PublicBaseURL:      "https://media.example",
			MaxUploadSizeBytes: 1024 * 1024,
		},
	})
	handler := NewSettingHandler(svc, nil, nil, nil, paymentCfgSvc, nil, mediaSvc)
	logoDataURL := buildSettingHandlerTestPNGDataURL(t)
	qrDataURL := buildSettingHandlerTestPNGDataURL(t)
	helpDataURL := buildSettingHandlerTestPNGDataURL(t)

	body := map[string]any{
		"promo_code_enabled": true,
		"site_logo":          logoDataURL,
		"support_qr_codes": []map[string]any{
			{"image_url": qrDataURL, "note": "help"},
		},
		"payment_help_image_url": helpDataURL,
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)

	require.True(t, strings.HasPrefix(asString(t, data["site_logo"]), "https://media.example/"), "expected direct URL, got %v", data["site_logo"])
	qrs, ok := data["support_qr_codes"].([]any)
	require.True(t, ok)
	require.Len(t, qrs, 1)
	qr, ok := qrs[0].(map[string]any)
	require.True(t, ok)
	require.True(t, strings.HasPrefix(asString(t, qr["image_url"]), "https://media.example/"), "expected direct URL, got %v", qr["image_url"])
	require.Equal(t, "help", qr["note"])
	require.True(t, strings.HasPrefix(asString(t, data["payment_help_image_url"]), "https://media.example/"), "expected direct URL, got %v", data["payment_help_image_url"])

	require.True(t, strings.HasPrefix(repo.values[service.SettingKeySiteLogo], "https://media.example/"), "expected direct URL, got %s", repo.values[service.SettingKeySiteLogo])
	require.True(t, strings.Contains(repo.values[service.SettingKeySupportQRCodes], "https://media.example/"), "expected direct URL in qr codes, got %s", repo.values[service.SettingKeySupportQRCodes])
	require.True(t, strings.HasPrefix(repo.values[service.SettingHelpImageURL], "https://media.example/"), "expected direct URL, got %s", repo.values[service.SettingHelpImageURL])
}

func TestSettingHandler_UpdateSettings_DoesNotUploadMediaWhenValidationFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled: "true",
			service.SettingKeySiteLogo:         "https://legacy.example/logo.png",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	paymentCfgSvc := service.NewPaymentConfigService(nil, repo, nil)
	mediaRepo := &settingHandlerMediaRepoStub{}
	mediaSvc := service.NewMediaService(mediaRepo, &settingHandlerMediaStoreStub{}, &config.Config{
		Media: config.MediaConfig{
			Enabled:            true,
			Endpoint:           "https://storage.example.com",
			Region:             "auto",
			Bucket:             "media",
			AccessKeyID:        "test-ak",
			SecretAccessKey:    "test-sk",
			PublicBaseURL:      "https://media.example",
			MaxUploadSizeBytes: 1024 * 1024,
		},
	})
	handler := NewSettingHandler(svc, nil, nil, nil, paymentCfgSvc, nil, mediaSvc)

	body := map[string]any{
		"promo_code_enabled":      true,
		"site_logo":               buildSettingHandlerTestPNGDataURL(t),
		"min_claude_code_version": "bad",
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Zero(t, mediaRepo.nextID)
	require.Equal(t, "https://legacy.example/logo.png", repo.values[service.SettingKeySiteLogo])
}

func TestSettingHandler_UpdateSettings_LeavesImageURLsUntouchedWithoutMediaService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled: "true",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil)

	body := map[string]any{
		"promo_code_enabled": true,
		"site_logo":          "data:image/png;base64,QUJD",
		"support_qr_codes": []map[string]any{
			{"image_url": "data:image/png;base64,QUJDRA==", "note": "help"},
		},
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "data:image/png;base64,QUJD", repo.values[service.SettingKeySiteLogo])
	require.JSONEq(t, `[{"image_url":"data:image/png;base64,QUJDRA==","note":"help"}]`, repo.values[service.SettingKeySupportQRCodes])
}

func TestSettingHandler_UpdateSettings_ReusesManagedMediaURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled: "true",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	paymentCfgSvc := service.NewPaymentConfigService(nil, repo, nil)
	mediaRepo := &settingHandlerMediaRepoStub{}
	mediaSvc := service.NewMediaService(mediaRepo, &settingHandlerMediaStoreStub{}, &config.Config{
		Media: config.MediaConfig{
			Enabled:            true,
			Endpoint:           "https://storage.example.com",
			Region:             "auto",
			Bucket:             "media",
			AccessKeyID:        "test-ak",
			SecretAccessKey:    "test-sk",
			PublicBaseURL:      "https://media.example",
			MaxUploadSizeBytes: 1024 * 1024,
		},
	})
	handler := NewSettingHandler(svc, nil, nil, nil, paymentCfgSvc, nil, mediaSvc)

	body := map[string]any{
		"promo_code_enabled": true,
		"site_logo":          "https://media.example/api/v1/media/public/11",
		"support_qr_codes": []map[string]any{
			{"image_url": "https://media.example/api/v1/media/public/12", "note": "help"},
		},
		"payment_help_image_url": "https://media.example/api/v1/media/public/13",
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Zero(t, mediaRepo.nextID)
	require.True(t, strings.HasPrefix(repo.values[service.SettingKeySiteLogo], "https://media.example/"), "expected direct URL, got %s", repo.values[service.SettingKeySiteLogo])
	require.True(t, strings.Contains(repo.values[service.SettingKeySupportQRCodes], "https://media.example/"), "expected direct URL in qr codes, got %s", repo.values[service.SettingKeySupportQRCodes])
	require.True(t, strings.HasPrefix(repo.values[service.SettingHelpImageURL], "https://media.example/"), "expected direct URL, got %s", repo.values[service.SettingHelpImageURL])
}

func TestSettingHandler_UpdateSettings_CleansUpMigratedMediaWhenPersistenceFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &failingSettingHandlerRepoStub{
		settingHandlerRepoStub: settingHandlerRepoStub{
			values: map[string]string{
				service.SettingKeyPromoCodeEnabled: "true",
				service.SettingKeySiteLogo:         "https://legacy.example/logo.png",
			},
		},
		setMultipleErr: errors.New("persist failed"),
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	paymentCfgSvc := service.NewPaymentConfigService(nil, repo, nil)
	mediaRepo := &settingHandlerMediaRepoStub{}
	mediaStore := &settingHandlerMediaStoreStub{}
	mediaSvc := service.NewMediaService(mediaRepo, mediaStore, &config.Config{
		Media: config.MediaConfig{
			Enabled:            true,
			Endpoint:           "https://storage.example.com",
			Region:             "auto",
			Bucket:             "media",
			AccessKeyID:        "test-ak",
			SecretAccessKey:    "test-sk",
			PublicBaseURL:      "https://media.example",
			MaxUploadSizeBytes: 1024 * 1024,
		},
	})
	handler := NewSettingHandler(svc, nil, nil, nil, paymentCfgSvc, nil, mediaSvc)

	body := map[string]any{
		"promo_code_enabled": true,
		"site_logo":          buildSettingHandlerTestPNGDataURL(t),
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, int64(1), mediaRepo.nextID)
	require.Len(t, mediaRepo.assets, 1)
	require.Len(t, mediaRepo.deletedIDs, 1)
	require.Equal(t, int64(1), mediaRepo.deletedIDs[0])
	require.NotNil(t, mediaRepo.assets[1])
	require.Equal(t, service.MediaStatusDeleted, mediaRepo.assets[1].Status)
	require.Greater(t, mediaStore.deleteCount, 0)
}

func TestSettingHandler_UpdateSettings_FailsBeforePersistenceWhenPaymentConfigReadbackFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &paymentConfigReadbackFailRepoStub{
		settingHandlerRepoStub: settingHandlerRepoStub{
			values: map[string]string{
				service.SettingKeyPromoCodeEnabled: "true",
				service.SettingKeySiteLogo:         "https://legacy.example/logo.png",
			},
		},
		getMultipleErr: errors.New("payment config readback failed"),
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	paymentCfgSvc := service.NewPaymentConfigService(nil, repo, nil)
	mediaRepo := &settingHandlerMediaRepoStub{}
	mediaStore := &settingHandlerMediaStoreStub{}
	mediaSvc := service.NewMediaService(mediaRepo, mediaStore, &config.Config{
		Media: config.MediaConfig{
			Enabled:            true,
			Endpoint:           "https://storage.example.com",
			Region:             "auto",
			Bucket:             "media",
			AccessKeyID:        "test-ak",
			SecretAccessKey:    "test-sk",
			PublicBaseURL:      "https://media.example",
			MaxUploadSizeBytes: 1024 * 1024,
		},
	})
	handler := NewSettingHandler(svc, nil, nil, nil, paymentCfgSvc, nil, mediaSvc)

	body := map[string]any{
		"promo_code_enabled": true,
		"site_logo":          buildSettingHandlerTestPNGDataURL(t),
		"payment_enabled":    true,
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Zero(t, mediaRepo.nextID)
	require.Empty(t, mediaRepo.assets)
	require.Empty(t, mediaRepo.deletedIDs)
	require.Zero(t, mediaStore.deleteCount)
	require.Equal(t, "https://legacy.example/logo.png", repo.values[service.SettingKeySiteLogo])
}

func TestSettingHandler_UpdateSettings_RollsBackSettingsWhenPaymentConfigValidationFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled: "true",
			service.SettingKeySiteLogo:         "https://legacy.example/logo.png",
			service.SettingPaymentEnabled:      "true",
			service.SettingBalanceRechargeMult: "1.50",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	paymentCfgSvc := service.NewPaymentConfigService(nil, repo, nil)
	mediaRepo := &settingHandlerMediaRepoStub{}
	mediaStore := &settingHandlerMediaStoreStub{}
	mediaSvc := service.NewMediaService(mediaRepo, mediaStore, &config.Config{
		Media: config.MediaConfig{
			Enabled:            true,
			Endpoint:           "https://storage.example.com",
			Region:             "auto",
			Bucket:             "media",
			AccessKeyID:        "test-ak",
			SecretAccessKey:    "test-sk",
			PublicBaseURL:      "https://media.example",
			MaxUploadSizeBytes: 1024 * 1024,
		},
	})
	handler := NewSettingHandler(svc, nil, nil, nil, paymentCfgSvc, nil, mediaSvc)

	body := map[string]any{
		"promo_code_enabled":                  false,
		"site_logo":                           buildSettingHandlerTestPNGDataURL(t),
		"payment_balance_recharge_multiplier": 0,
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "true", repo.values[service.SettingKeyPromoCodeEnabled])
	require.Equal(t, "https://legacy.example/logo.png", repo.values[service.SettingKeySiteLogo])
	require.Equal(t, "true", repo.values[service.SettingPaymentEnabled])
	require.Equal(t, "1.50", repo.values[service.SettingBalanceRechargeMult])
	require.Equal(t, int64(1), mediaRepo.nextID)
	require.Len(t, mediaRepo.deletedIDs, 1)
	require.Equal(t, int64(1), mediaRepo.deletedIDs[0])
	require.NotNil(t, mediaRepo.assets[1])
	require.Equal(t, service.MediaStatusDeleted, mediaRepo.assets[1].Status)
	require.Greater(t, mediaStore.deleteCount, 0)
}

func TestSettingHandler_UpdateSettings_RollsBackSettingsWhenPaymentConfigSaveFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &paymentConfigWriteFailRepoStub{
		settingHandlerRepoStub: settingHandlerRepoStub{
			values: map[string]string{
				service.SettingKeyPromoCodeEnabled: "true",
				service.SettingKeySiteLogo:         "https://legacy.example/logo.png",
				service.SettingPaymentEnabled:      "true",
			},
		},
		setPaymentErr: errors.New("payment config persist failed"),
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	paymentCfgSvc := service.NewPaymentConfigService(nil, repo, nil)
	mediaRepo := &settingHandlerMediaRepoStub{}
	mediaStore := &settingHandlerMediaStoreStub{}
	mediaSvc := service.NewMediaService(mediaRepo, mediaStore, &config.Config{
		Media: config.MediaConfig{
			Enabled:            true,
			Endpoint:           "https://storage.example.com",
			Region:             "auto",
			Bucket:             "media",
			AccessKeyID:        "test-ak",
			SecretAccessKey:    "test-sk",
			PublicBaseURL:      "https://media.example",
			MaxUploadSizeBytes: 1024 * 1024,
		},
	})
	handler := NewSettingHandler(svc, nil, nil, nil, paymentCfgSvc, nil, mediaSvc)

	body := map[string]any{
		"promo_code_enabled": false,
		"site_logo":          buildSettingHandlerTestPNGDataURL(t),
		"payment_enabled":    false,
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, "true", repo.values[service.SettingKeyPromoCodeEnabled])
	require.Equal(t, "https://legacy.example/logo.png", repo.values[service.SettingKeySiteLogo])
	require.Equal(t, "true", repo.values[service.SettingPaymentEnabled])
	require.Equal(t, int64(1), mediaRepo.nextID)
	require.Len(t, mediaRepo.deletedIDs, 1)
	require.Equal(t, int64(1), mediaRepo.deletedIDs[0])
	require.NotNil(t, mediaRepo.assets[1])
	require.Equal(t, service.MediaStatusDeleted, mediaRepo.assets[1].Status)
	require.Greater(t, mediaStore.deleteCount, 0)
}

func TestSettingHandler_UpdateSettings_CleansUpMigratedMediaWhenFastPolicySaveFailsAndRollbackSucceeds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled:         "true",
			service.SettingKeySiteLogo:                 "https://legacy.example/logo.png",
			service.SettingKeyOpenAIFastPolicySettings: `{"rules":[]}`,
		},
		setFailKey: service.SettingKeyOpenAIFastPolicySettings,
		setFailErr: errors.New("fast policy save failed"),
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	paymentCfgSvc := service.NewPaymentConfigService(nil, repo, nil)
	mediaRepo := &settingHandlerMediaRepoStub{}
	mediaStore := &settingHandlerMediaStoreStub{}
	mediaSvc := service.NewMediaService(mediaRepo, mediaStore, &config.Config{
		Media: config.MediaConfig{
			Enabled:            true,
			Endpoint:           "https://storage.example.com",
			Region:             "auto",
			Bucket:             "media",
			AccessKeyID:        "test-ak",
			SecretAccessKey:    "test-sk",
			PublicBaseURL:      "https://media.example",
			MaxUploadSizeBytes: 1024 * 1024,
		},
	})
	handler := NewSettingHandler(svc, nil, nil, nil, paymentCfgSvc, nil, mediaSvc)

	body := map[string]any{
		"promo_code_enabled": true,
		"site_logo":          buildSettingHandlerTestPNGDataURL(t),
		"openai_fast_policy_settings": map[string]any{
			"rules": []map[string]any{
				{"service_tier": "", "action": "pass", "scope": "all"},
			},
		},
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, int64(1), mediaRepo.nextID)
	require.Len(t, mediaRepo.assets, 1)
	require.Len(t, mediaRepo.deletedIDs, 1)
	require.Equal(t, int64(1), mediaRepo.deletedIDs[0])
	require.NotNil(t, mediaRepo.assets[1])
	require.Equal(t, service.MediaStatusDeleted, mediaRepo.assets[1].Status)
	require.Greater(t, mediaStore.deleteCount, 0)
	require.Equal(t, "https://legacy.example/logo.png", repo.values[service.SettingKeySiteLogo])
}

func TestSettingHandler_UpdateSettings_CleansUpMigratedMediaWhenRollbackFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &rollbackFailureRepoStub{
		settingHandlerRepoStub: settingHandlerRepoStub{
			values: map[string]string{
				service.SettingKeyPromoCodeEnabled: "true",
				service.SettingKeySiteLogo:         "https://legacy.example/logo.png",
				service.SettingPaymentEnabled:      "true",
			},
		},
		paymentErr:  errors.New("payment config save failed"),
		rollbackErr: errors.New("rollback failed"),
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	paymentCfgSvc := service.NewPaymentConfigService(nil, repo, nil)
	mediaRepo := &settingHandlerMediaRepoStub{}
	mediaStore := &settingHandlerMediaStoreStub{}
	mediaSvc := service.NewMediaService(mediaRepo, mediaStore, &config.Config{
		Media: config.MediaConfig{
			Enabled:            true,
			Endpoint:           "https://storage.example.com",
			Region:             "auto",
			Bucket:             "media",
			AccessKeyID:        "test-ak",
			SecretAccessKey:    "test-sk",
			PublicBaseURL:      "https://media.example",
			MaxUploadSizeBytes: 1024 * 1024,
		},
	})
	handler := NewSettingHandler(svc, nil, nil, nil, paymentCfgSvc, nil, mediaSvc)

	body := map[string]any{
		"promo_code_enabled":          true,
		"site_logo":                   buildSettingHandlerTestPNGDataURL(t),
		"payment_enabled":             false,
		"payment_help_text":           "keep cleanup",
		"payment_product_name_prefix": "prefix",
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, 4, repo.callCount)
	require.Equal(t, int64(1), mediaRepo.nextID)
	require.Len(t, mediaRepo.deletedIDs, 1)
	require.Equal(t, int64(1), mediaRepo.deletedIDs[0])
	require.NotNil(t, mediaRepo.assets[1])
	require.Equal(t, service.MediaStatusDeleted, mediaRepo.assets[1].Status)
	require.Greater(t, mediaStore.deleteCount, 0)
	require.Equal(t, "https://legacy.example/logo.png", repo.values[service.SettingKeySiteLogo])
}

func TestSettingHandler_UpdateSettings_PreservesPartialFieldsWhenSavingPaymentHelpImage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled: "true",
			service.SettingKeySiteLogo:         "https://legacy.example/logo.png",
			service.SettingPaymentEnabled:      "true",
			service.SettingMinRechargeAmount:   "12.50",
			service.SettingEnabledPaymentTypes: "alipay,wxpay",
			service.SettingProductNamePrefix:   "Prefix",
			service.SettingHelpImageURL:        "https://legacy.example/help.png",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	paymentCfgSvc := service.NewPaymentConfigService(nil, repo, nil)
	mediaRepo := &settingHandlerMediaRepoStub{}
	mediaSvc := service.NewMediaService(mediaRepo, &settingHandlerMediaStoreStub{}, &config.Config{
		Media: config.MediaConfig{
			Enabled:            true,
			Endpoint:           "https://storage.example.com",
			Region:             "auto",
			Bucket:             "media",
			AccessKeyID:        "test-ak",
			SecretAccessKey:    "test-sk",
			PublicBaseURL:      "https://media.example",
			MaxUploadSizeBytes: 1024 * 1024,
		},
	})
	handler := NewSettingHandler(svc, nil, nil, nil, paymentCfgSvc, nil, mediaSvc)
	helpDataURL := buildSettingHandlerTestPNGDataURL(t)

	body := map[string]any{
		"promo_code_enabled":     true,
		"payment_help_image_url": helpDataURL,
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(1), mediaRepo.nextID)
	require.Equal(t, "https://legacy.example/logo.png", repo.values[service.SettingKeySiteLogo])
	require.Equal(t, "true", repo.values[service.SettingPaymentEnabled])
	require.Equal(t, "12.50", repo.values[service.SettingMinRechargeAmount])
	require.Equal(t, "alipay,wxpay", repo.values[service.SettingEnabledPaymentTypes])
	require.Equal(t, "Prefix", repo.values[service.SettingProductNamePrefix])
	require.True(t, strings.HasPrefix(repo.values[service.SettingHelpImageURL], "https://media.example/"), "expected direct URL, got %s", repo.values[service.SettingHelpImageURL])
}

func TestSettingHandler_UpdateSettings_RoundTripsPaymentAlipayForceQRCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled: "true",
			service.SettingPaymentEnabled:      "true",
			service.SettingMinRechargeAmount:   "12.50",
			service.SettingHelpText:            "keep me",
			service.SettingAlipayForceQRCode:   "false",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	paymentCfgSvc := service.NewPaymentConfigService(nil, repo, nil)
	handler := NewSettingHandler(svc, nil, nil, nil, paymentCfgSvc, nil, nil)

	body := map[string]any{
		"promo_code_enabled":          true,
		"payment_alipay_force_qrcode": true,
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "true", repo.values[service.SettingAlipayForceQRCode])
	require.Equal(t, "true", repo.values[service.SettingPaymentEnabled])
	require.Equal(t, "12.50", repo.values[service.SettingMinRechargeAmount])
	require.Equal(t, "keep me", repo.values[service.SettingHelpText])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, data["payment_alipay_force_qrcode"])
	require.Equal(t, true, data["payment_enabled"])
	require.Equal(t, "keep me", data["payment_help_text"])
}

func TestSettingHandler_UpdateSettings_RoundTripsApiKeyAclTrustForwardedIPAndCodexUserAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled:          "true",
			service.SettingKeyAPIKeyACLTrustForwardedIP: "false",
			service.SettingKeyOpenAICodexUserAgent:      "OpenAI-Codex/legacy",
			service.SettingAlipayForceQRCode:            "false",
			service.SettingPaymentEnabled:               "true",
			service.SettingMinRechargeAmount:            "12.50",
			service.SettingHelpText:                     "keep me",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	paymentCfgSvc := service.NewPaymentConfigService(nil, repo, nil)
	handler := NewSettingHandler(svc, nil, nil, nil, paymentCfgSvc, nil, nil)

	body := map[string]any{
		"promo_code_enabled":             true,
		"api_key_acl_trust_forwarded_ip": true,
		"openai_codex_user_agent":        "OpenAI-Codex/2025.05",
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "true", repo.values[service.SettingKeyAPIKeyACLTrustForwardedIP])
	require.Equal(t, "OpenAI-Codex/2025.05", repo.values[service.SettingKeyOpenAICodexUserAgent])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, data["api_key_acl_trust_forwarded_ip"])
	require.Equal(t, "OpenAI-Codex/2025.05", data["openai_codex_user_agent"])
}

func TestSettingHandler_UpdateSettings_RoundTripsDingTalkAuthSourceDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyRegistrationEnabled: "true",
			service.SettingKeyPromoCodeEnabled:    "true",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil)

	body := map[string]any{
		"registration_enabled":                     true,
		"promo_code_enabled":                       true,
		"auth_source_default_dingtalk_balance":     4.5,
		"auth_source_default_dingtalk_concurrency": 8,
		"auth_source_default_dingtalk_subscriptions": []map[string]any{
			{"group_id": 77, "validity_days": 15},
		},
		"auth_source_default_dingtalk_grant_on_signup":     true,
		"auth_source_default_dingtalk_grant_on_first_bind": false,
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "4.50000000", repo.values[service.SettingKeyAuthSourceDefaultDingTalkBalance])
	require.Equal(t, "8", repo.values[service.SettingKeyAuthSourceDefaultDingTalkConcurrency])
	require.JSONEq(t, `[{"group_id":77,"validity_days":15}]`, repo.values[service.SettingKeyAuthSourceDefaultDingTalkSubscriptions])
	require.Equal(t, "true", repo.values[service.SettingKeyAuthSourceDefaultDingTalkGrantOnSignup])
	require.Equal(t, "false", repo.values[service.SettingKeyAuthSourceDefaultDingTalkGrantOnFirstBind])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, 4.5, data["auth_source_default_dingtalk_balance"])
	require.Equal(t, float64(8), data["auth_source_default_dingtalk_concurrency"])
	require.Equal(t, true, data["auth_source_default_dingtalk_grant_on_signup"])
	require.Equal(t, false, data["auth_source_default_dingtalk_grant_on_first_bind"])
	subs, ok := data["auth_source_default_dingtalk_subscriptions"].([]any)
	require.True(t, ok)
	require.Len(t, subs, 1)
	sub, ok := subs[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(77), sub["group_id"])
	require.Equal(t, float64(15), sub["validity_days"])
}
