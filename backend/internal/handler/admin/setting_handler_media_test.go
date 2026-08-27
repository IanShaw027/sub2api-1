//go:build unit

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
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type settingsMediaRepoStub struct {
	nextID int64
	assets map[int64]*service.MediaAsset
}

func (r *settingsMediaRepoStub) Create(_ context.Context, asset *service.MediaAsset) error {
	r.nextID++
	asset.ID = r.nextID
	if r.assets == nil {
		r.assets = make(map[int64]*service.MediaAsset)
	}
	copy := *asset
	r.assets[asset.ID] = &copy
	return nil
}

func (r *settingsMediaRepoStub) GetByID(_ context.Context, id int64) (*service.MediaAsset, error) {
	asset := *r.assets[id]
	return &asset, nil
}

func (*settingsMediaRepoStub) ListByBiz(context.Context, int64, string, string) ([]service.MediaAsset, error) {
	return nil, nil
}

func (*settingsMediaRepoStub) UpdateVisibility(context.Context, int64, string, string) error {
	return nil
}

func (r *settingsMediaRepoStub) MarkDeleted(_ context.Context, id int64) error {
	delete(r.assets, id)
	return nil
}

type settingsMediaStoreStub struct {
	objects      map[string][]byte
	deleteCtxErr error
}

func (s *settingsMediaStoreStub) Put(_ context.Context, key, _ string, data []byte) error {
	if s.objects == nil {
		s.objects = make(map[string][]byte)
	}
	s.objects[key] = append([]byte(nil), data...)
	return nil
}

func (s *settingsMediaStoreStub) Get(_ context.Context, key string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.objects[key])), nil
}

func (s *settingsMediaStoreStub) Delete(ctx context.Context, key string) error {
	s.deleteCtxErr = ctx.Err()
	delete(s.objects, key)
	return nil
}

func (*settingsMediaStoreStub) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

type settingsMediaResolverStub struct {
	store service.MediaObjectStore
}

func (r settingsMediaResolverStub) Resolve(context.Context) (*service.MediaStorageBinding, service.MediaObjectStore, error) {
	return &service.MediaStorageBinding{
		ProfileID:     "backup",
		Prefix:        "media",
		PublicBaseURL: "https://media.example",
	}, r.store, nil
}

func TestSettingHandlerIngestSettingsImage(t *testing.T) {
	repo := &settingsMediaRepoStub{}
	store := &settingsMediaStoreStub{}
	handler := &SettingHandler{
		mediaService: service.NewMediaService(repo, settingsMediaResolverStub{store: store}, []byte("signing-key")),
	}

	url, assetID, err := handler.ingestSettingsImage(
		context.Background(),
		42,
		service.MediaBizSupportQR,
		"1",
		settingsTestPNGDataURL(t, 8, 8),
		"support-qr-1",
		settingsSupportQRMaxBytes,
	)
	require.NoError(t, err)
	require.Equal(t, int64(1), assetID)
	require.Contains(t, url, "https://media.example/media/support_qr/")
	require.Len(t, store.objects, 1)
	require.Equal(t, int64(42), repo.assets[assetID].OwnerUserID)
	require.Equal(t, service.MediaBizSupportQR, repo.assets[assetID].BizType)
}

func TestDecodeSettingsImageDataURLRejectsMalformedInput(t *testing.T) {
	_, _, err := decodeSettingsImageDataURL("data:image/png,not-base64", settingsLogoMaxBytes)
	require.ErrorIs(t, err, service.ErrMediaUnsupportedType)

	_, _, err = decodeSettingsImageDataURL("data:image/svg+xml;base64,PHN2Zz4=", settingsLogoMaxBytes)
	require.ErrorIs(t, err, service.ErrMediaUnsupportedType)

	_, _, err = decodeSettingsImageDataURL("data:image/png;base64,iVBORw0KGgo=", settingsLogoMaxBytes)
	require.ErrorIs(t, err, service.ErrMediaUnsupportedType)
}

func TestValidateSettingsImageReferenceRejectsUnsafeSchemes(t *testing.T) {
	for _, raw := range []string{
		"javascript:alert(1)",
		"data:text/html;base64,PGgxPng8L2gxPg==",
		"//evil.example/image.png",
		"file:///tmp/image.png",
	} {
		err := validateSettingsImageReference(raw, service.MediaBizSupportQR)
		require.Error(t, err, raw)
	}
	require.NoError(t, validateSettingsImageReference("https://legacy.example/image.png", service.MediaBizSupportQR))
	require.NoError(t, validateSettingsImageReference("/api/v1/media/public/42", service.MediaBizPaymentHelp))
	require.NoError(t, validateSettingsImageReference("/logo.png", service.MediaBizSiteLogo))
	require.Error(t, validateSettingsImageReference("/logo.png", service.MediaBizSupportQR))
}

func TestDecodeSettingsImageDataURLValidatesMIMEBytesSizeAndDimensions(t *testing.T) {
	pngDataURL := settingsTestPNGDataURL(t, 8, 8)
	data, extension, err := decodeSettingsImageDataURL(pngDataURL, settingsLogoMaxBytes)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	require.Equal(t, ".png", extension)

	mismatched := "data:image/jpeg;base64," + strings.TrimPrefix(pngDataURL, "data:image/png;base64,")
	_, _, err = decodeSettingsImageDataURL(mismatched, settingsLogoMaxBytes)
	require.ErrorIs(t, err, service.ErrMediaUnsupportedType)

	tooLarge := "data:image/png;base64," + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("x"), settingsLogoMaxBytes+1))
	_, _, err = decodeSettingsImageDataURL(tooLarge, settingsLogoMaxBytes)
	require.Equal(t, "MEDIA_TOO_LARGE", infraerrors.Reason(err))
	require.Contains(t, err.Error(), "300 KiB")

	_, _, err = decodeSettingsImageDataURL(settingsTestPNGDataURL(t, settingsImageMaxDimension+1, 1), settingsSupportQRMaxBytes)
	require.Error(t, err)
	require.Equal(t, "MEDIA_IMAGE_DIMENSIONS_INVALID", infraerrors.Reason(err))
}

func TestIngestSettingsMediaRejectsMoreThanEightSupportQRCodes(t *testing.T) {
	handler := &SettingHandler{}
	req := &UpdateSettingsRequest{SupportQRCodes: make([]service.SupportQRCodeEntry, 9)}
	_, _, err := handler.ingestSettingsMedia(
		context.Background(),
		42,
		map[string]json.RawMessage{"support_qr_codes": json.RawMessage(`[]`)},
		req,
	)
	require.Error(t, err)
	require.Equal(t, "SETTINGS_SUPPORT_QR_LIMIT", infraerrors.Reason(err))
}

func TestCleanupSettingsMediaIgnoresRequestCancellation(t *testing.T) {
	repo := &settingsMediaRepoStub{}
	store := &settingsMediaStoreStub{}
	handler := &SettingHandler{
		mediaService: service.NewMediaService(repo, settingsMediaResolverStub{store: store}, []byte("signing-key")),
	}
	_, assetID, err := handler.ingestSettingsImage(
		context.Background(),
		42,
		service.MediaBizSiteLogo,
		"default",
		settingsTestPNGDataURL(t, 8, 8),
		"site-logo",
		settingsLogoMaxBytes,
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	handler.cleanupSettingsMedia(ctx, []int64{assetID})

	require.NoError(t, store.deleteCtxErr)
	require.NotContains(t, repo.assets, assetID)
	require.Empty(t, store.objects)
}

type failingSettingsMediaSettingRepo struct {
	settingHandlerRepoStub
	failAll     bool
	failPayment bool
}

func (r *failingSettingsMediaSettingRepo) SetMultiple(ctx context.Context, values map[string]string) error {
	if r.failAll {
		return errors.New("settings persistence failed")
	}
	if r.failPayment {
		if _, ok := values[service.SettingHelpImageURL]; ok {
			return errors.New("payment persistence failed")
		}
	}
	return r.settingHandlerRepoStub.SetMultiple(ctx, values)
}

func TestUpdateSettingsPersistsAllImagesAndRetainsAssets(t *testing.T) {
	settingsRepo := &failingSettingsMediaSettingRepo{
		settingHandlerRepoStub: settingHandlerRepoStub{values: map[string]string{}},
	}
	mediaRepo := &settingsMediaRepoStub{}
	mediaStore := &settingsMediaStoreStub{}
	handler := newSettingsMediaUpdateHandler(settingsRepo, mediaRepo, mediaStore)
	dataURL := settingsTestPNGDataURL(t, 8, 8)

	rec := doUpdateSettings(t, handler, map[string]any{
		"site_logo": dataURL,
		"support_qr_codes": []map[string]string{
			{"image_url": dataURL, "note": "support"},
		},
		"payment_help_image_url": dataURL,
	}, setSettingsMediaAuthSubject)

	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Len(t, mediaRepo.assets, 3)
	require.Len(t, mediaStore.objects, 3)
	require.Contains(t, settingsRepo.values[service.SettingKeySiteLogo], "https://media.example/media/site_logo/")
	require.Contains(t, settingsRepo.values[service.SettingKeySupportQRCodes], "https://media.example/media/support_qr/")
	require.Contains(t, settingsRepo.values[service.SettingHelpImageURL], "https://media.example/media/payment_help/")
}

func TestUpdateSettingsCleansUploadedImageWhenSystemPersistenceFails(t *testing.T) {
	settingsRepo := &failingSettingsMediaSettingRepo{
		settingHandlerRepoStub: settingHandlerRepoStub{values: map[string]string{}},
		failAll:                true,
	}
	mediaRepo := &settingsMediaRepoStub{}
	mediaStore := &settingsMediaStoreStub{}
	handler := newSettingsMediaUpdateHandler(settingsRepo, mediaRepo, mediaStore)

	rec := doUpdateSettings(t, handler, map[string]any{
		"site_logo": settingsTestPNGDataURL(t, 8, 8),
	}, setSettingsMediaAuthSubject)

	require.NotEqual(t, 200, rec.Code)
	require.Empty(t, mediaRepo.assets)
	require.Empty(t, mediaStore.objects)
}

func TestUpdateSettingsPaymentFailureOnlyCleansUnpersistedPaymentImage(t *testing.T) {
	settingsRepo := &failingSettingsMediaSettingRepo{
		settingHandlerRepoStub: settingHandlerRepoStub{values: map[string]string{}},
		failPayment:            true,
	}
	mediaRepo := &settingsMediaRepoStub{}
	mediaStore := &settingsMediaStoreStub{}
	handler := newSettingsMediaUpdateHandler(settingsRepo, mediaRepo, mediaStore)
	dataURL := settingsTestPNGDataURL(t, 8, 8)

	rec := doUpdateSettings(t, handler, map[string]any{
		"site_logo":              dataURL,
		"payment_help_image_url": dataURL,
	}, setSettingsMediaAuthSubject)

	require.NotEqual(t, 200, rec.Code)
	require.Len(t, mediaRepo.assets, 1)
	for _, asset := range mediaRepo.assets {
		require.Equal(t, service.MediaBizSiteLogo, asset.BizType)
	}
	require.Len(t, mediaStore.objects, 1)
	require.Contains(t, settingsRepo.values[service.SettingKeySiteLogo], "https://media.example/media/site_logo/")
	require.Empty(t, settingsRepo.values[service.SettingHelpImageURL])
}

func newSettingsMediaUpdateHandler(
	settingsRepo service.SettingRepository,
	mediaRepo service.MediaAssetRepository,
	mediaStore service.MediaObjectStore,
) *SettingHandler {
	settingSvc := service.NewSettingService(settingsRepo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	paymentConfigSvc := service.NewPaymentConfigService(nil, settingsRepo, nil)
	handler := NewSettingHandler(settingSvc, nil, nil, nil, paymentConfigSvc, nil, nil)
	handler.SetMediaService(service.NewMediaService(mediaRepo, settingsMediaResolverStub{store: mediaStore}, []byte("signing-key")))
	return handler
}

func setSettingsMediaAuthSubject(c *gin.Context) {
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
}

func settingsTestPNGDataURL(t *testing.T, width, height int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 0x22, G: 0x66, B: 0xaa, A: 0xff})
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, img))
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())
}
