package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type mediaIngestTestRepo struct{}

func (mediaIngestTestRepo) Create(context.Context, *MediaAsset) error { return nil }
func (mediaIngestTestRepo) GetByID(context.Context, int64) (*MediaAsset, error) {
	return nil, nil
}
func (mediaIngestTestRepo) List(context.Context, pagination.PaginationParams, MediaListFilters) ([]MediaAsset, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (mediaIngestTestRepo) UpdateVisibility(context.Context, int64, string) error { return nil }
func (mediaIngestTestRepo) MarkDeleted(context.Context, int64, time.Time) error   { return nil }
func (mediaIngestTestRepo) GetByObjectKey(context.Context, string, string) (*MediaAsset, error) {
	return nil, ErrMediaNotFound
}

type mediaURLTestRepo struct {
	assetsByID map[int64]*MediaAsset
}

type mediaObjectSettingsRepo struct {
	value  string
	values map[string]string
}

func (r mediaObjectSettingsRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (r mediaObjectSettingsRepo) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := r.values[key]; ok && strings.TrimSpace(value) != "" {
		return value, nil
	}
	if key != settingKeyObjectStorageConfig || strings.TrimSpace(r.value) == "" {
		return "", ErrSettingNotFound
	}
	return r.value, nil
}
func (r mediaObjectSettingsRepo) Set(context.Context, string, string) error { return nil }
func (r mediaObjectSettingsRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}
func (r mediaObjectSettingsRepo) SetMultiple(context.Context, map[string]string) error { return nil }
func (r mediaObjectSettingsRepo) GetAll(context.Context) (map[string]string, error)    { return nil, nil }
func (r mediaObjectSettingsRepo) Delete(context.Context, string) error                 { return nil }

func (r mediaURLTestRepo) Create(context.Context, *MediaAsset) error { return nil }
func (r mediaURLTestRepo) GetByID(_ context.Context, id int64) (*MediaAsset, error) {
	if asset, ok := r.assetsByID[id]; ok {
		return asset, nil
	}
	return nil, ErrMediaNotFound
}
func (r mediaURLTestRepo) List(context.Context, pagination.PaginationParams, MediaListFilters) ([]MediaAsset, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r mediaURLTestRepo) UpdateVisibility(context.Context, int64, string) error { return nil }
func (r mediaURLTestRepo) MarkDeleted(context.Context, int64, time.Time) error   { return nil }
func (r mediaURLTestRepo) GetByObjectKey(_ context.Context, bucket, objectKey string) (*MediaAsset, error) {
	bucket = strings.TrimSpace(bucket)
	objectKey = strings.TrimSpace(objectKey)
	for _, asset := range r.assetsByID {
		if asset == nil {
			continue
		}
		if strings.TrimSpace(asset.ObjectKey) != objectKey {
			continue
		}
		if bucket == "" || strings.TrimSpace(asset.Bucket) == bucket {
			return asset, nil
		}
	}
	return nil, ErrMediaNotFound
}

type mediaIngestTestStore struct {
	uploadedBucket      string
	uploadedObjectKey   string
	uploadedBody        []byte
	uploadedContentType string
}

func (s *mediaIngestTestStore) Upload(_ context.Context, _ MediaStorageRuntimeConfig, bucket, objectKey string, body []byte, contentType string) error {
	s.uploadedBucket = bucket
	s.uploadedObjectKey = objectKey
	s.uploadedBody = append([]byte(nil), body...)
	s.uploadedContentType = contentType
	return nil
}

func (*mediaIngestTestStore) Download(context.Context, MediaStorageRuntimeConfig, string, string) (io.ReadCloser, error) {
	return nil, nil
}

func (*mediaIngestTestStore) Delete(context.Context, MediaStorageRuntimeConfig, string, string) error {
	return nil
}
func (*mediaIngestTestStore) Stat(context.Context, MediaStorageRuntimeConfig, string, string) (int64, error) {
	return 0, nil
}
func (*mediaIngestTestStore) PresignGetObject(context.Context, MediaStorageRuntimeConfig, string, string, time.Duration) (string, error) {
	return "", nil
}

type mediaIngestRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f mediaIngestRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestIngestImageReferenceFromDataURL(t *testing.T) {
	raw := buildMediaIngestTestPNG(t)
	cfg := newMediaIngestTestConfig()
	store := &mediaIngestTestStore{}
	svc := NewMediaService(mediaIngestTestRepo{}, store, cfg)

	asset, err := svc.IngestImageReference(context.Background(), IngestImageReferenceInput{
		BizType:    "avatar",
		BizID:      "user-1",
		Visibility: MediaVisibilityPublic,
		Source:     "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw),
		FileName:   "avatar.png",
	})
	if err != nil {
		t.Fatalf("IngestImageReference returned error: %v", err)
	}
	if asset == nil {
		t.Fatalf("expected media asset")
	}
	if !bytes.Equal(store.uploadedBody, raw) {
		t.Fatalf("uploaded body mismatch")
	}
	if store.uploadedContentType != "image/png" {
		t.Fatalf("uploaded content type = %q, want image/png", store.uploadedContentType)
	}
	if asset.Visibility != MediaVisibilityPublic {
		t.Fatalf("asset visibility = %q, want %q", asset.Visibility, MediaVisibilityPublic)
	}
}

func TestIngestImageReferenceFromRemoteURL(t *testing.T) {
	raw := buildMediaIngestTestPNG(t)
	cfg := newMediaIngestTestConfig()
	store := &mediaIngestTestStore{}
	svc := NewMediaService(mediaIngestTestRepo{}, store, cfg)

	restore := stubMediaIngestRemoteFetchHooks(t, func(host string) error {
		if host != "cdn.example.com" {
			t.Fatalf("resolved host = %q, want cdn.example.com", host)
		}
		return nil
	}, func(_ *config.Config) (*http.Client, error) {
		return &http.Client{
			Transport: mediaIngestRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://cdn.example.com/qr.png" {
					t.Fatalf("requested url = %q", req.URL.String())
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"image/png"}},
					Body:       io.NopCloser(bytes.NewReader(raw)),
				}, nil
			}),
		}, nil
	})
	defer restore()

	asset, err := svc.IngestImageReference(context.Background(), IngestImageReferenceInput{
		BizType:    "support_qr",
		BizID:      "default",
		Visibility: MediaVisibilityPublic,
		Source:     "https://cdn.example.com/qr.png",
	})
	if err != nil {
		t.Fatalf("IngestImageReference returned error: %v", err)
	}
	if asset == nil {
		t.Fatalf("expected media asset")
	}
	if !bytes.Equal(store.uploadedBody, raw) {
		t.Fatalf("uploaded body mismatch")
	}
	if store.uploadedContentType != "image/png" {
		t.Fatalf("uploaded content type = %q, want image/png", store.uploadedContentType)
	}
}

func TestIngestImageReferenceRejectsSpoofedRemoteContentType(t *testing.T) {
	cfg := newMediaIngestTestConfig()
	store := &mediaIngestTestStore{}
	svc := NewMediaService(mediaIngestTestRepo{}, store, cfg)

	restore := stubMediaIngestRemoteFetchHooks(t, func(string) error { return nil }, func(_ *config.Config) (*http.Client, error) {
		return &http.Client{
			Transport: mediaIngestRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"image/png"}},
					Body:       io.NopCloser(bytes.NewBufferString("<html>not an image</html>")),
				}, nil
			}),
		}, nil
	})
	defer restore()

	asset, err := svc.IngestImageReference(context.Background(), IngestImageReferenceInput{
		BizType:    "support_qr",
		BizID:      "default",
		Visibility: MediaVisibilityPublic,
		Source:     "https://cdn.example.com/fake.png",
	})
	if !errors.Is(err, ErrMediaNotFound) {
		t.Fatalf("error = %v, want %v", err, ErrMediaNotFound)
	}
	if asset != nil {
		t.Fatalf("expected nil asset")
	}
	if len(store.uploadedBody) != 0 {
		t.Fatalf("expected no upload for spoofed response")
	}
}

func TestIngestImageReferenceRejectsNonImageDataURL(t *testing.T) {
	cfg := newMediaIngestTestConfig()
	store := &mediaIngestTestStore{}
	svc := NewMediaService(mediaIngestTestRepo{}, store, cfg)

	asset, err := svc.IngestImageReference(context.Background(), IngestImageReferenceInput{
		BizType:    "avatar",
		BizID:      "user-1",
		Visibility: MediaVisibilityPublic,
		Source:     "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("not-image")),
		FileName:   "avatar.png",
	})
	if !errors.Is(err, ErrMediaNotFound) {
		t.Fatalf("error = %v, want %v", err, ErrMediaNotFound)
	}
	if asset != nil {
		t.Fatalf("expected nil asset")
	}
	if len(store.uploadedBody) != 0 {
		t.Fatalf("expected no upload for non-image data url")
	}
}

func TestResolveImageReferenceRejectsPrivateRemoteURL(t *testing.T) {
	cfg := newMediaIngestTestConfig()
	cfg.Security.URLAllowlist.AllowPrivateHosts = false

	_, _, _, err := resolveImageReference(
		context.Background(),
		cfg,
		"http://127.0.0.1/private.png",
		"private.png",
		cfg.Media.MaxUploadSizeBytes,
	)
	if err == nil {
		t.Fatalf("expected private url rejection")
	}
}

func TestResolveImageReferenceValidatesResolvedIPEachRequest(t *testing.T) {
	cfg := newMediaIngestTestConfig()
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	called := false

	restore := stubMediaIngestRemoteFetchHooks(t, func(host string) error {
		called = true
		if host != "cdn.example.com" {
			t.Fatalf("resolved host = %q, want cdn.example.com", host)
		}
		return errors.New("resolved ip rejected")
	}, func(_ *config.Config) (*http.Client, error) {
		t.Fatalf("http client should not be used after resolved-ip rejection")
		return nil, nil
	})
	defer restore()

	_, _, _, err := resolveImageReference(
		context.Background(),
		cfg,
		"https://cdn.example.com/image.png",
		"image.png",
		cfg.Media.MaxUploadSizeBytes,
	)
	if !called {
		t.Fatalf("expected resolved-ip validation to run")
	}
	if err == nil || err.Error() != "resolved ip rejected" {
		t.Fatalf("error = %v, want resolved ip rejected", err)
	}
}

func TestResolveImageReferenceRejectsNonDefaultRemotePort(t *testing.T) {
	cfg := newMediaIngestTestConfig()

	_, _, _, err := resolveImageReference(
		context.Background(),
		cfg,
		"https://cdn.example.com:8443/image.png",
		"image.png",
		cfg.Media.MaxUploadSizeBytes,
	)
	if err == nil {
		t.Fatalf("expected non-default port rejection")
	}
	if !strings.Contains(err.Error(), "port") {
		t.Fatalf("error = %v, want port-related rejection", err)
	}
}

func TestResolveImageReferenceRedirectValidatesEachHopAndLimitsCount(t *testing.T) {
	cfg := newMediaIngestTestConfig()
	var validatedHosts []string

	restore := stubMediaIngestRemoteFetchHooks(t, func(host string) error {
		validatedHosts = append(validatedHosts, host)
		if host == "127.0.0.1" {
			return errors.New("redirect target rejected")
		}
		return nil
	}, func(_ *config.Config) (*http.Client, error) {
		return &http.Client{
			Transport: mediaIngestRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Host {
				case "cdn.example.com":
					return &http.Response{
						StatusCode: http.StatusFound,
						Header: http.Header{
							"Location": []string{"http://127.0.0.1/image.png"},
						},
						Body:    io.NopCloser(bytes.NewReader(nil)),
						Request: req,
					}, nil
				default:
					t.Fatalf("unexpected request host %q", req.URL.Host)
					return nil, nil
				}
			}),
		}, nil
	})
	defer restore()

	_, _, _, err := resolveImageReference(
		context.Background(),
		cfg,
		"https://cdn.example.com/image.png",
		"image.png",
		cfg.Media.MaxUploadSizeBytes,
	)
	if err == nil || !strings.Contains(err.Error(), "redirect target rejected") {
		t.Fatalf("error = %v, want redirect target rejected", err)
	}
	if len(validatedHosts) < 2 {
		t.Fatalf("expected initial and redirect hosts to be validated, got %v", validatedHosts)
	}
}

func TestResolveImageReferenceRejectsTooManyRedirects(t *testing.T) {
	cfg := newMediaIngestTestConfig()

	restore := stubMediaIngestRemoteFetchHooks(t, func(string) error { return nil }, func(_ *config.Config) (*http.Client, error) {
		return &http.Client{
			Transport: mediaIngestRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
				next, _ := url.Parse("https://cdn.example.com" + req.URL.Path + "/next")
				return &http.Response{
					StatusCode: http.StatusFound,
					Header: http.Header{
						"Location": []string{next.String()},
					},
					Body:    io.NopCloser(bytes.NewReader(nil)),
					Request: req,
				}, nil
			}),
		}, nil
	})
	defer restore()

	_, _, _, err := resolveImageReference(
		context.Background(),
		cfg,
		"https://cdn.example.com/image.png",
		"image.png",
		cfg.Media.MaxUploadSizeBytes,
	)
	if err == nil {
		t.Fatalf("expected redirect limit rejection")
	}
	if !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("error = %v, want redirect-related rejection", err)
	}
}

func TestIngestImageReferenceVisibilityHandling(t *testing.T) {
	raw := buildMediaIngestTestPNG(t)
	cfg := newMediaIngestTestConfig()

	tests := []struct {
		name       string
		visibility string
		want       string
	}{
		{name: "default private", visibility: "", want: MediaVisibilityPrivate},
		{name: "explicit private", visibility: MediaVisibilityPrivate, want: MediaVisibilityPrivate},
		{name: "explicit public", visibility: MediaVisibilityPublic, want: MediaVisibilityPublic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mediaIngestTestStore{}
			svc := NewMediaService(mediaIngestTestRepo{}, store, cfg)

			asset, err := svc.IngestImageReference(context.Background(), IngestImageReferenceInput{
				BizType:    "avatar",
				BizID:      "user-1",
				Visibility: tt.visibility,
				Source:     "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw),
				FileName:   "avatar.png",
			})
			if err != nil {
				t.Fatalf("IngestImageReference returned error: %v", err)
			}
			if asset.Visibility != tt.want {
				t.Fatalf("asset visibility = %q, want %q", asset.Visibility, tt.want)
			}
		})
	}
}

func TestBuildSignedDownloadURL_UsesMediaPublicBaseURLInsteadOfFrontendURLPath(t *testing.T) {
	cfg := newMediaIngestTestConfig()
	cfg.Server.FrontendURL = "https://app.example.com/console"
	svc := NewMediaService(mediaIngestTestRepo{}, &mediaIngestTestStore{}, cfg)

	asset := &MediaAsset{
		ID:        42,
		Bucket:    cfg.Media.Bucket,
		Status:    MediaStatusActive,
		ObjectKey: "avatars/u42.png",
	}

	result, err := svc.buildSignedDownloadURL(context.Background(), asset, false)
	if err != nil {
		t.Fatalf("buildSignedDownloadURL returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected download url result")
	}
	if got, wantPrefix := result.URL, "https://media.example/api/v1/media/download/42?"; len(got) < len(wantPrefix) || got[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("download url = %q, want prefix %q", got, wantPrefix)
	}
	if strings.Contains(result.URL, "/console/api/v1/media/download/") {
		t.Fatalf("download url should not be rooted at frontend path: %q", result.URL)
	}
}

func TestBuildSignedDownloadURL_PrefersRequestBaseURLWhenPresent(t *testing.T) {
	cfg := newMediaIngestTestConfig()
	svc := NewMediaService(mediaIngestTestRepo{}, &mediaIngestTestStore{}, cfg)

	asset := &MediaAsset{
		ID:        42,
		Bucket:    cfg.Media.Bucket,
		Status:    MediaStatusActive,
		ObjectKey: "avatars/u42.png",
	}

	ctx := WithRequestBaseURL(context.Background(), "https://api.caller.example")
	result, err := svc.buildSignedDownloadURL(ctx, asset, false)
	if err != nil {
		t.Fatalf("buildSignedDownloadURL returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected download url result")
	}
	if got, wantPrefix := result.URL, "https://api.caller.example/api/v1/media/download/42?"; len(got) < len(wantPrefix) || got[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("download url = %q, want prefix %q", got, wantPrefix)
	}
	if strings.Contains(result.URL, "media.example") {
		t.Fatalf("download url should use request base, not public base: %q", result.URL)
	}
}

func TestBuildSignedDownloadURL_UsesAPIBaseURLOriginWithoutDuplicatingAPIPrefix(t *testing.T) {
	cfg := newMediaIngestTestConfig()
	svc := NewMediaService(mediaIngestTestRepo{}, &mediaIngestTestStore{}, cfg)
	svc.SetStorageConfigProvider(NewMediaStorageConfigProvider(mediaObjectSettingsRepo{
		values: map[string]string{
			SettingKeyAPIBaseURL: "https://api.example.com/api/v1",
		},
	}, nil, cfg))

	asset := &MediaAsset{
		ID:        42,
		Bucket:    cfg.Media.Bucket,
		Status:    MediaStatusActive,
		ObjectKey: "avatars/u42.png",
	}

	result, err := svc.buildSignedDownloadURL(context.Background(), asset, false)
	if err != nil {
		t.Fatalf("buildSignedDownloadURL returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected download url result")
	}
	if got, wantPrefix := result.URL, "https://api.example.com/api/v1/media/download/42?"; len(got) < len(wantPrefix) || got[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("download url = %q, want prefix %q", got, wantPrefix)
	}
	if strings.Contains(result.URL, "/api/v1/api/v1/") {
		t.Fatalf("download url should not duplicate api prefix: %q", result.URL)
	}
}

func TestOpenSignedDownloadRejectsDeletedAsset(t *testing.T) {
	cfg := newMediaIngestTestConfig()
	asset := &MediaAsset{
		ID:         44,
		Bucket:     cfg.Media.Bucket,
		ObjectKey:  "media/private/deleted.png",
		Visibility: MediaVisibilityPrivate,
		Status:     MediaStatusDeleted,
	}
	svc := NewMediaService(mediaURLTestRepo{
		assetsByID: map[int64]*MediaAsset{44: asset},
	}, &mediaIngestTestStore{}, cfg)
	expires := time.Now().Add(time.Hour).Unix()
	signature := svc.downloadSignature(asset.ID, expires, false)

	stream, gotAsset, err := svc.OpenSignedDownload(context.Background(), asset.ID, expires, signature, false)

	if err == nil {
		t.Fatalf("OpenSignedDownload deleted asset error = nil, stream=%v asset=%v", stream, gotAsset)
	}
	if !errors.Is(err, ErrMediaNotFound) {
		t.Fatalf("OpenSignedDownload deleted asset error = %v, want ErrMediaNotFound", err)
	}
}

func TestMediaDownloadSigningDoesNotFallbackToJWTSecret(t *testing.T) {
	cfg := newMediaIngestTestConfig()
	cfg.Media.DownloadSigningSecret = ""
	cfg.JWT.Secret = strings.Repeat("j", 32)
	svc := NewMediaService(nil, nil, cfg)

	if signature := svc.downloadSignature(42, time.Now().Add(time.Hour).Unix(), false); signature != "" {
		t.Fatalf("download signature must fail closed without an independent media key, got %q", signature)
	}
}

func TestManagedPublicMediaURLs_UseDirectObjectKeyAndRoundTripManagedID(t *testing.T) {
	cfg := newMediaIngestTestConfig()
	asset := &MediaAsset{
		ID:                 42,
		StorageProfileID:   "old",
		Bucket:             cfg.Media.Bucket,
		ObjectKey:          "media/avatar/user-42/original.png",
		ThumbnailObjectKey: "media/avatar_thumbnail/user-42/thumb.png",
		Visibility:         MediaVisibilityPublic,
		Status:             MediaStatusActive,
	}
	svc := NewMediaService(mediaURLTestRepo{
		assetsByID: map[int64]*MediaAsset{
			42: asset,
		},
	}, &mediaIngestTestStore{}, cfg)

	publicURL := svc.PublicURL(asset)
	if publicURL != "https://media.example/media/avatar/user-42/original.png" {
		t.Fatalf("public url = %q, want direct object url", publicURL)
	}
	thumbnailURL := svc.ThumbnailPublicURL(asset)
	if thumbnailURL != "https://media.example/media/avatar_thumbnail/user-42/thumb.png" {
		t.Fatalf("thumbnail public url = %q, want direct object url", thumbnailURL)
	}

	if id, ok := ParseManagedMediaID(svc, publicURL); !ok || id != asset.ID {
		t.Fatalf("ParseManagedMediaID(public) = (%d, %v), want (%d, true)", id, ok, asset.ID)
	}

	legacyPublic := "https://media.example/api/v1/media/public/42"
	if id, ok := ParseManagedMediaID(svc, legacyPublic); !ok || id != asset.ID {
		t.Fatalf("ParseManagedMediaID(legacyPublic) = (%d, %v), want (%d, true)", id, ok, asset.ID)
	}
	legacyThumbnail := "https://media.example/api/v1/media/public/42/thumbnail"
	if id, ok := ParseManagedMediaID(svc, legacyThumbnail); !ok || id != asset.ID {
		t.Fatalf("ParseManagedMediaID(legacyThumbnail) = (%d, %v), want (%d, true)", id, ok, asset.ID)
	}
}

func TestParseManagedMediaID_DirectObjectURLMatchesHistoricalBucket(t *testing.T) {
	cfg := newMediaIngestTestConfig()
	asset := &MediaAsset{
		ID:               43,
		StorageProfileID: "archive",
		Bucket:           "archive-bucket",
		ObjectKey:        "media/avatar/user-43/original.png",
		Status:           MediaStatusActive,
	}
	svc := NewMediaService(mediaURLTestRepo{
		assetsByID: map[int64]*MediaAsset{
			43: asset,
		},
	}, &mediaIngestTestStore{}, cfg)
	svc.SetStorageConfigProvider(NewMediaStorageConfigProvider(mediaObjectSettingsRepo{
		value: `{
			"profiles":[
				{"id":"current","name":"Current","provider":"s3","endpoint":"https://storage.example.com","region":"auto","bucket":"media","access_key_id":"test-ak","secret_access_key":"test-sk"},
				{"id":"archive","name":"Archive","provider":"s3","endpoint":"https://archive-storage.example.com","region":"auto","bucket":"archive-bucket","access_key_id":"archive-ak","secret_access_key":"archive-sk"}
			],
			"media_enabled":true,
			"media_profile_id":"current",
			"media_public_base_url":"https://media.example",
			"media_prefix":""
		}`,
	}, nil, cfg))

	raw := "https://media.example/media/avatar/user-43/original.png"
	if id, ok := ParseManagedMediaID(svc, raw); !ok || id != asset.ID {
		t.Fatalf("ParseManagedMediaID(%q) = (%d, %v), want (%d, true)", raw, id, ok, asset.ID)
	}
}

func newMediaIngestTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Media.Enabled = true
	cfg.Media.Endpoint = "https://storage.example.com"
	cfg.Media.Region = "auto"
	cfg.Media.Bucket = "media"
	cfg.Media.AccessKeyID = "test-ak"
	cfg.Media.SecretAccessKey = "test-sk"
	cfg.Media.PublicBaseURL = "https://media.example"
	cfg.Media.DownloadSigningSecret = strings.Repeat("m", 32)
	cfg.Media.MaxUploadSizeBytes = 1024 * 1024
	cfg.Security.URLAllowlist.Enabled = true
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	return cfg
}

func buildMediaIngestTestPNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(240 - x*10), G: uint8(32 + y*12), B: 180, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func stubMediaIngestRemoteFetchHooks(
	t *testing.T,
	validateResolvedIP func(string) error,
	getClient func(*config.Config) (*http.Client, error),
) func() {
	t.Helper()

	originalValidateURL := mediaIngestValidateHTTPURL
	originalValidateResolvedIP := mediaIngestValidateResolvedIP
	originalGetClient := mediaIngestHTTPClient

	mediaIngestValidateHTTPURL = func(_ *config.Config, raw string) (string, error) {
		return raw, nil
	}
	mediaIngestValidateResolvedIP = validateResolvedIP
	mediaIngestHTTPClient = getClient

	return func() {
		mediaIngestValidateHTTPURL = originalValidateURL
		mediaIngestValidateResolvedIP = originalValidateResolvedIP
		mediaIngestHTTPClient = originalGetClient
	}
}
