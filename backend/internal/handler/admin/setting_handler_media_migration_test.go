package admin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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
	return nil, nil
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
			service.SettingPaymentVisibleMethodAlipayEnabled,
			service.SettingPaymentVisibleMethodAlipaySource,
			service.SettingPaymentVisibleMethodWxpayEnabled,
			service.SettingPaymentVisibleMethodWxpaySource:
			return nil, s.getMultipleErr
		}
	}
	return s.settingHandlerRepoStub.GetMultiple(ctx, keys)
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

	require.Equal(t, "https://media.example/api/v1/media/public/1", data["site_logo"])
	qrs, ok := data["support_qr_codes"].([]any)
	require.True(t, ok)
	require.Len(t, qrs, 1)
	qr, ok := qrs[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "https://media.example/api/v1/media/public/2", qr["image_url"])
	require.Equal(t, "help", qr["note"])
	require.Equal(t, "https://media.example/api/v1/media/public/3", data["payment_help_image_url"])

	require.Equal(t, "https://media.example/api/v1/media/public/1", repo.values[service.SettingKeySiteLogo])
	require.JSONEq(t, `[{"image_url":"https://media.example/api/v1/media/public/2","note":"help"}]`, repo.values[service.SettingKeySupportQRCodes])
	require.Equal(t, "https://media.example/api/v1/media/public/3", repo.values[service.SettingHelpImageURL])
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
	require.Equal(t, "https://media.example/api/v1/media/public/11", repo.values[service.SettingKeySiteLogo])
	require.JSONEq(t, `[{"image_url":"https://media.example/api/v1/media/public/12","note":"help"}]`, repo.values[service.SettingKeySupportQRCodes])
	require.Equal(t, "https://media.example/api/v1/media/public/13", repo.values[service.SettingHelpImageURL])
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
	require.Equal(t, "https://legacy.example/logo.png", repo.values[service.SettingKeySiteLogo])
}

func TestSettingHandler_UpdateSettings_KeepsMigratedMediaWhenPaymentConfigReadbackFailsAfterPersistence(t *testing.T) {
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
	require.Equal(t, int64(1), mediaRepo.nextID)
	require.Len(t, mediaRepo.assets, 1)
	require.Empty(t, mediaRepo.deletedIDs)
	require.NotNil(t, mediaRepo.assets[1])
	require.NotEqual(t, service.MediaStatusDeleted, mediaRepo.assets[1].Status)
	require.Zero(t, mediaStore.deleteCount)
	require.Equal(t, "https://media.example/api/v1/media/public/1", repo.values[service.SettingKeySiteLogo])
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
	require.Equal(t, "https://media.example/api/v1/media/public/1", repo.values[service.SettingHelpImageURL])
}

func TestSettingHandler_UpdateSettings_CanonicalizesManagedMediaURLsAcrossBaseURLChanges(t *testing.T) {
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
			Bucket:             "media",
			PublicBaseURL:      "https://media.example",
			MaxUploadSizeBytes: 1024 * 1024,
		},
	})
	handler := NewSettingHandler(svc, nil, nil, nil, paymentCfgSvc, nil, mediaSvc)

	body := map[string]any{
		"promo_code_enabled": true,
		"site_logo":          "https://old-media.example/api/v1/media/public/11?cache=1",
		"support_qr_codes": []map[string]any{
			{"image_url": "https://old-media.example/api/v1/media/public/12#preview", "note": "help"},
		},
		"payment_help_image_url": "https://old-media.example/api/v1/media/download/13/thumbnail.png",
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
	require.Equal(t, "https://media.example/api/v1/media/public/11", repo.values[service.SettingKeySiteLogo])
	require.JSONEq(t, `[{"image_url":"https://media.example/api/v1/media/public/12","note":"help"}]`, repo.values[service.SettingKeySupportQRCodes])
	require.Equal(t, "https://media.example/api/v1/media/public/13", repo.values[service.SettingHelpImageURL])
}
