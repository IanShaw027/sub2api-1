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

func newMediaIngestTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Media.Enabled = true
	cfg.Media.Endpoint = "https://storage.example.com"
	cfg.Media.Region = "auto"
	cfg.Media.Bucket = "media"
	cfg.Media.AccessKeyID = "test-ak"
	cfg.Media.SecretAccessKey = "test-sk"
	cfg.Media.PublicBaseURL = "https://media.example"
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
