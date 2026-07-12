package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type userServiceMediaAvatarRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f userServiceMediaAvatarRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type userServiceMediaAvatarRepo struct {
	getByIDUser     *User
	upsertAvatarArg []UpsertUserAvatarInput
	deleteAvatarIDs []int64
	upsertAvatarErr error
}

func (r *userServiceMediaAvatarRepo) Create(context.Context, *User) error { return nil }
func (r *userServiceMediaAvatarRepo) GetByID(context.Context, int64) (*User, error) {
	if r.getByIDUser != nil {
		cloned := *r.getByIDUser
		return &cloned, nil
	}
	return &User{}, nil
}
func (r *userServiceMediaAvatarRepo) GetByIDIncludeDeleted(ctx context.Context, id int64) (*User, error) {
	return r.GetByID(ctx, id)
}
func (r *userServiceMediaAvatarRepo) GetByEmail(context.Context, string) (*User, error) {
	return &User{}, nil
}
func (r *userServiceMediaAvatarRepo) GetFirstAdmin(context.Context) (*User, error) {
	return &User{}, nil
}
func (r *userServiceMediaAvatarRepo) Update(context.Context, *User) error { return nil }
func (r *userServiceMediaAvatarRepo) Delete(context.Context, int64) error { return nil }
func (r *userServiceMediaAvatarRepo) GetUserAvatar(context.Context, int64) (*UserAvatar, error) {
	return nil, nil
}
func (r *userServiceMediaAvatarRepo) UpsertUserAvatar(_ context.Context, _ int64, input UpsertUserAvatarInput) (*UserAvatar, error) {
	if r.upsertAvatarErr != nil {
		return nil, r.upsertAvatarErr
	}
	r.upsertAvatarArg = append(r.upsertAvatarArg, input)
	return &UserAvatar{
		StorageProvider: input.StorageProvider,
		StorageKey:      input.StorageKey,
		URL:             input.URL,
		ContentType:     input.ContentType,
		ByteSize:        input.ByteSize,
		SHA256:          input.SHA256,
	}, nil
}
func (r *userServiceMediaAvatarRepo) DeleteUserAvatar(_ context.Context, userID int64) error {
	r.deleteAvatarIDs = append(r.deleteAvatarIDs, userID)
	return nil
}
func (r *userServiceMediaAvatarRepo) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *userServiceMediaAvatarRepo) ListWithFilters(context.Context, pagination.PaginationParams, UserListFilters) ([]User, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *userServiceMediaAvatarRepo) UpdateBalance(context.Context, int64, float64) error { return nil }
func (r *userServiceMediaAvatarRepo) AddBalanceWithoutRecharge(context.Context, int64, float64) error {
	return nil
}
func (r *userServiceMediaAvatarRepo) UpdateUserLastActiveAt(context.Context, int64, time.Time) error {
	return nil
}
func (r *userServiceMediaAvatarRepo) GetLatestUsedAtByUserIDs(context.Context, []int64) (map[int64]*time.Time, error) {
	return nil, nil
}
func (r *userServiceMediaAvatarRepo) GetLatestUsedAtByUserID(context.Context, int64) (*time.Time, error) {
	return nil, nil
}
func (r *userServiceMediaAvatarRepo) DeductBalance(context.Context, int64, float64) error { return nil }
func (r *userServiceMediaAvatarRepo) UpdateConcurrency(context.Context, int64, int) error { return nil }
func (r *userServiceMediaAvatarRepo) BatchSetConcurrency(context.Context, []int64, int) (int, error) {
	return 0, nil
}
func (r *userServiceMediaAvatarRepo) BatchAddConcurrency(context.Context, []int64, int) (int, error) {
	return 0, nil
}
func (r *userServiceMediaAvatarRepo) ExistsByEmail(context.Context, string) (bool, error) {
	return false, nil
}
func (r *userServiceMediaAvatarRepo) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	return 0, nil
}
func (r *userServiceMediaAvatarRepo) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	return nil
}
func (r *userServiceMediaAvatarRepo) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	return nil
}
func (r *userServiceMediaAvatarRepo) ListUserAuthIdentities(context.Context, int64) ([]UserAuthIdentityRecord, error) {
	return nil, nil
}
func (r *userServiceMediaAvatarRepo) UnbindUserAuthProvider(context.Context, int64, string) error {
	return nil
}
func (r *userServiceMediaAvatarRepo) UpdateTotpSecret(context.Context, int64, *string) error {
	return nil
}
func (r *userServiceMediaAvatarRepo) EnableTotp(context.Context, int64) error  { return nil }
func (r *userServiceMediaAvatarRepo) DisableTotp(context.Context, int64) error { return nil }

type userServiceMediaAvatarMediaRepo struct {
	nextID     int64
	assets     map[int64]*MediaAsset
	deletedIDs []int64
}

func (r *userServiceMediaAvatarMediaRepo) Create(_ context.Context, asset *MediaAsset) error {
	r.nextID++
	asset.ID = r.nextID
	if r.assets == nil {
		r.assets = make(map[int64]*MediaAsset)
	}
	cloned := *asset
	r.assets[asset.ID] = &cloned
	return nil
}

func (r *userServiceMediaAvatarMediaRepo) GetByID(_ context.Context, id int64) (*MediaAsset, error) {
	if r.assets == nil || r.assets[id] == nil {
		return nil, nil
	}
	cloned := *r.assets[id]
	return &cloned, nil
}

func (*userServiceMediaAvatarMediaRepo) List(context.Context, pagination.PaginationParams, MediaListFilters) ([]MediaAsset, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (*userServiceMediaAvatarMediaRepo) UpdateVisibility(context.Context, int64, string) error {
	return nil
}

func (r *userServiceMediaAvatarMediaRepo) MarkDeleted(_ context.Context, id int64, _ time.Time) error {
	if r.assets == nil || r.assets[id] == nil {
		return ErrMediaNotFound
	}
	r.deletedIDs = append(r.deletedIDs, id)
	r.assets[id].Status = MediaStatusDeleted
	return nil
}

func (r *userServiceMediaAvatarMediaRepo) GetByObjectKey(_ context.Context, bucket, objectKey string) (*MediaAsset, error) {
	bucket = strings.TrimSpace(bucket)
	for _, asset := range r.assets {
		if asset == nil || asset.ObjectKey != objectKey {
			continue
		}
		if bucket != "" && strings.TrimSpace(asset.Bucket) != bucket {
			continue
		}
		if asset != nil && asset.ObjectKey == objectKey {
			cloned := *asset
			return &cloned, nil
		}
	}
	return nil, ErrMediaNotFound
}

type userServiceMediaAvatarTestStore struct {
	uploadedObjectKeys []string
	deletedObjectKeys  []string
}

func (s *userServiceMediaAvatarTestStore) Upload(_ context.Context, _ MediaStorageRuntimeConfig, _ string, objectKey string, _ []byte, _ string) error {
	s.uploadedObjectKeys = append(s.uploadedObjectKeys, objectKey)
	return nil
}

func (*userServiceMediaAvatarTestStore) Download(context.Context, MediaStorageRuntimeConfig, string, string) (io.ReadCloser, error) {
	return nil, nil
}

func (s *userServiceMediaAvatarTestStore) Delete(_ context.Context, _ MediaStorageRuntimeConfig, _ string, objectKey string) error {
	s.deletedObjectKeys = append(s.deletedObjectKeys, objectKey)
	return nil
}
func (*userServiceMediaAvatarTestStore) Stat(context.Context, MediaStorageRuntimeConfig, string, string) (int64, error) {
	return 0, nil
}
func (*userServiceMediaAvatarTestStore) PresignGetObject(_ context.Context, _ MediaStorageRuntimeConfig, _, objectKey string, _ time.Duration) (string, error) {
	return "https://source.qazwc.com/presigned/" + objectKey, nil
}

func newUserServiceMediaAvatarTestMediaService() (*MediaService, *userServiceMediaAvatarMediaRepo, *userServiceMediaAvatarTestStore) {
	repo := &userServiceMediaAvatarMediaRepo{}
	store := &userServiceMediaAvatarTestStore{}
	cfg := &config.Config{}
	cfg.Media.Enabled = true
	cfg.Media.Endpoint = "https://storage.example.com"
	cfg.Media.Region = "auto"
	cfg.Media.Bucket = "media"
	cfg.Media.AccessKeyID = "test-ak"
	cfg.Media.SecretAccessKey = "test-sk"
	cfg.Media.PublicBaseURL = "https://source.qazwc.com"
	cfg.Media.MaxUploadSizeBytes = 64 << 20
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Security.URLAllowlist.AllowPrivateHosts = true
	return NewMediaService(repo, store, cfg), repo, store
}

func buildUserServiceMediaAvatarPNG(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(255 - x*31), G: uint8(y * 31), B: 127, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func buildUserServiceMediaAvatarOversizedPNGHeader() []byte {
	// Minimal PNG structure with huge IHDR dimensions. DecodeConfig succeeds,
	// but full decode should be rejected before any large allocation attempt.
	var buf bytes.Buffer
	_, _ = buf.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], 100000)
	binary.BigEndian.PutUint32(ihdr[4:8], 100000)
	ihdr[8] = 8
	ihdr[9] = 2
	writePNGChunk(&buf, "IHDR", ihdr)
	writePNGChunk(&buf, "IEND", nil)
	return buf.Bytes()
}

func writePNGChunk(buf *bytes.Buffer, chunkType string, data []byte) {
	_ = binary.Write(buf, binary.BigEndian, uint32(len(data)))
	_, _ = buf.WriteString(chunkType)
	if len(data) > 0 {
		_, _ = buf.Write(data)
	}
	crc := crc32.ChecksumIEEE(append([]byte(chunkType), data...))
	_ = binary.Write(buf, binary.BigEndian, crc)
}

func TestSetAvatar_StoresDataURLInSharedMedia(t *testing.T) {
	raw := buildUserServiceMediaAvatarPNG(t)
	dataURL := "data:image/png;base64," + encodeBase64(raw)
	repo := &userServiceMediaAvatarRepo{
		getByIDUser: &User{ID: 7, Email: "avatar@example.com", Username: "avatar-user"},
	}
	mediaSvc, _, _ := newUserServiceMediaAvatarTestMediaService()
	svc := NewUserService(repo, nil, nil, nil, mediaSvc)

	avatar, err := svc.SetAvatar(context.Background(), 7, dataURL)
	require.NoError(t, err)
	require.NotNil(t, avatar)
	require.Len(t, repo.upsertAvatarArg, 1)
	require.Equal(t, "media", repo.upsertAvatarArg[0].StorageProvider)
	require.NotEmpty(t, repo.upsertAvatarArg[0].StorageKey)
	require.Equal(t, "image/png", repo.upsertAvatarArg[0].ContentType)
	require.Equal(t, len(raw), repo.upsertAvatarArg[0].ByteSize)
	require.True(t, strings.HasPrefix(avatar.URL, "https://source.qazwc.com/"), "expected direct URL, got %s", avatar.URL)
	require.Equal(t, avatar.URL, repo.upsertAvatarArg[0].URL)
}

func TestSetAvatar_RejectsOversizedImageBomb(t *testing.T) {
	raw := buildUserServiceMediaAvatarOversizedPNGHeader()
	dataURL := "data:image/png;base64," + encodeBase64(raw)
	repo := &userServiceMediaAvatarRepo{
		getByIDUser: &User{ID: 7, Email: "avatar@example.com", Username: "avatar-user"},
	}
	mediaSvc, mediaRepo, store := newUserServiceMediaAvatarTestMediaService()
	svc := NewUserService(repo, nil, nil, nil, mediaSvc)

	avatar, err := svc.SetAvatar(context.Background(), 7, dataURL)
	require.ErrorIs(t, err, ErrAvatarInvalid)
	require.Nil(t, avatar)
	require.Empty(t, repo.upsertAvatarArg)
	require.Empty(t, store.uploadedObjectKeys)
	require.Empty(t, mediaRepo.assets)
}

func TestValidateMediaThumbnailSourceRejectsOversizedPixelHeader(t *testing.T) {
	err := validateMediaThumbnailSource(buildUserServiceMediaAvatarOversizedPNGHeader())
	require.Error(t, err)
	require.Contains(t, err.Error(), "thumbnail pixel limit")
}

func TestValidateMediaThumbnailSourceAcceptsNormalImage(t *testing.T) {
	err := validateMediaThumbnailSource(buildUserServiceMediaAvatarPNG(t))
	require.NoError(t, err)
}

func TestSetAvatar_StoresRemoteURLInSharedMedia(t *testing.T) {
	raw := buildUserServiceMediaAvatarPNG(t)
	origValidateHTTPURL := mediaIngestValidateHTTPURL
	origValidateResolvedIP := mediaIngestValidateResolvedIP
	origHTTPClient := mediaIngestHTTPClient
	mediaIngestValidateHTTPURL = func(_ *config.Config, raw string) (string, error) {
		return raw, nil
	}
	mediaIngestValidateResolvedIP = func(string) error { return nil }
	mediaIngestHTTPClient = func(*config.Config) (*http.Client, error) {
		return &http.Client{
			Transport: userServiceMediaAvatarRoundTripperFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"image/png"}},
					Body:       io.NopCloser(bytes.NewReader(raw)),
				}, nil
			}),
		}, nil
	}
	defer func() {
		mediaIngestValidateHTTPURL = origValidateHTTPURL
		mediaIngestValidateResolvedIP = origValidateResolvedIP
		mediaIngestHTTPClient = origHTTPClient
	}()

	repo := &userServiceMediaAvatarRepo{
		getByIDUser: &User{ID: 9, Email: "remote@example.com", Username: "remote-user"},
	}
	mediaSvc, _, _ := newUserServiceMediaAvatarTestMediaService()
	svc := NewUserService(repo, nil, nil, nil, mediaSvc)

	avatar, err := svc.SetAvatar(context.Background(), 9, "https://cdn.example.com/avatar.png")
	require.NoError(t, err)
	require.NotNil(t, avatar)
	require.Len(t, repo.upsertAvatarArg, 1)
	require.Equal(t, "media", repo.upsertAvatarArg[0].StorageProvider)
	require.True(t, strings.HasPrefix(avatar.URL, "https://source.qazwc.com/"), "expected direct URL, got %s", avatar.URL)
	require.Equal(t, avatar.URL, repo.upsertAvatarArg[0].URL)
}

func TestSetAvatar_DeletesAvatarWhenEmptyWithMediaEnabled(t *testing.T) {
	repo := &userServiceMediaAvatarRepo{
		getByIDUser: &User{ID: 11, Email: "delete@example.com", Username: "delete-user"},
	}
	mediaSvc, _, _ := newUserServiceMediaAvatarTestMediaService()
	svc := NewUserService(repo, nil, nil, nil, mediaSvc)

	avatar, err := svc.SetAvatar(context.Background(), 11, "   ")
	require.NoError(t, err)
	require.Nil(t, avatar)
	require.Equal(t, []int64{11}, repo.deleteAvatarIDs)
	require.Empty(t, repo.upsertAvatarArg)
}

func TestSetAvatar_ReusesManagedMediaURLWithoutReupload(t *testing.T) {
	repo := &userServiceMediaAvatarRepo{
		getByIDUser: &User{ID: 13, Email: "reuse@example.com", Username: "reuse-user"},
	}
	mediaSvc, mediaRepo, _ := newUserServiceMediaAvatarTestMediaService()
	mediaRepo.assets = map[int64]*MediaAsset{
		5: {
			ID:         5,
			Bucket:     "media",
			ObjectKey:  "avatar/13/2026/05/05/reused.png",
			Visibility: MediaVisibilityPublic,
			MIMEType:   "image/png",
			SizeBytes:  123,
			SHA256:     "abc123",
			Status:     MediaStatusActive,
		},
	}
	mediaRepo.nextID = 5
	svc := NewUserService(repo, nil, nil, nil, mediaSvc)

	avatar, err := svc.SetAvatar(context.Background(), 13, "https://source.qazwc.com/api/v1/media/public/5")
	require.NoError(t, err)
	require.NotNil(t, avatar)
	require.Len(t, repo.upsertAvatarArg, 1)
	require.Equal(t, "media", repo.upsertAvatarArg[0].StorageProvider)
	require.Equal(t, "avatar/13/2026/05/05/reused.png", repo.upsertAvatarArg[0].StorageKey)
	require.Equal(t, "https://source.qazwc.com/avatar/13/2026/05/05/reused.png", repo.upsertAvatarArg[0].URL)
	require.Equal(t, int64(5), mediaRepo.nextID)
	require.Len(t, mediaRepo.assets, 1)
}

func TestSetAvatar_ReusesManagedDirectObjectURLFromHistoricalBucketWithoutReupload(t *testing.T) {
	repo := &userServiceMediaAvatarRepo{
		getByIDUser: &User{ID: 16, Email: "history@example.com", Username: "history-user"},
	}
	mediaSvc, mediaRepo, store := newUserServiceMediaAvatarTestMediaService()
	mediaRepo.assets = map[int64]*MediaAsset{
		6: {
			ID:               6,
			StorageProfileID: "archive",
			Bucket:           "archive-bucket",
			ObjectKey:        "avatar/16/2026/05/05/reused.png",
			Visibility:       MediaVisibilityPublic,
			MIMEType:         "image/png",
			SizeBytes:        321,
			SHA256:           "def456",
			Status:           MediaStatusActive,
		},
	}
	mediaRepo.nextID = 6
	svc := NewUserService(repo, nil, nil, nil, mediaSvc)

	avatar, err := svc.SetAvatar(context.Background(), 16, "https://source.qazwc.com/avatar/16/2026/05/05/reused.png")
	require.NoError(t, err)
	require.NotNil(t, avatar)
	require.Len(t, repo.upsertAvatarArg, 1)
	require.Equal(t, "media", repo.upsertAvatarArg[0].StorageProvider)
	require.Equal(t, "avatar/16/2026/05/05/reused.png", repo.upsertAvatarArg[0].StorageKey)
	require.Equal(t, "https://source.qazwc.com/avatar/16/2026/05/05/reused.png", repo.upsertAvatarArg[0].URL)
	require.Equal(t, int64(6), mediaRepo.nextID)
	require.Len(t, mediaRepo.assets, 1)
	require.Empty(t, store.uploadedObjectKeys)
}

func TestSetAvatar_FallsBackToRemoteURLWhenManagedAssetMissing(t *testing.T) {
	repo := &userServiceMediaAvatarRepo{
		getByIDUser: &User{ID: 14, Email: "fallback@example.com", Username: "fallback-user"},
	}
	mediaSvc, mediaRepo, _ := newUserServiceMediaAvatarTestMediaService()
	mediaRepo.nextID = 5
	svc := NewUserService(repo, nil, nil, nil, mediaSvc)

	raw := "https://source.qazwc.com/api/v1/media/public/5"
	avatar, err := svc.SetAvatar(context.Background(), 14, raw)
	require.NoError(t, err)
	require.NotNil(t, avatar)
	require.Len(t, repo.upsertAvatarArg, 1)
	require.Equal(t, "remote_url", repo.upsertAvatarArg[0].StorageProvider)
	require.Equal(t, raw, repo.upsertAvatarArg[0].URL)
	require.Equal(t, int64(5), mediaRepo.nextID)
	require.Empty(t, mediaRepo.assets)
}

func TestSetAvatar_CleansUpUploadedMediaWhenAvatarUpsertFails(t *testing.T) {
	raw := buildUserServiceMediaAvatarPNG(t)
	origValidateHTTPURL := mediaIngestValidateHTTPURL
	origValidateResolvedIP := mediaIngestValidateResolvedIP
	origHTTPClient := mediaIngestHTTPClient
	mediaIngestValidateHTTPURL = func(_ *config.Config, raw string) (string, error) {
		return raw, nil
	}
	mediaIngestValidateResolvedIP = func(string) error { return nil }
	mediaIngestHTTPClient = func(*config.Config) (*http.Client, error) {
		return &http.Client{
			Transport: userServiceMediaAvatarRoundTripperFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"image/png"}},
					Body:       io.NopCloser(bytes.NewReader(raw)),
				}, nil
			}),
		}, nil
	}
	defer func() {
		mediaIngestValidateHTTPURL = origValidateHTTPURL
		mediaIngestValidateResolvedIP = origValidateResolvedIP
		mediaIngestHTTPClient = origHTTPClient
	}()

	repo := &userServiceMediaAvatarRepo{
		getByIDUser:     &User{ID: 15, Email: "cleanup@example.com", Username: "cleanup-user"},
		upsertAvatarErr: errors.New("avatar write failed"),
	}
	mediaSvc, mediaRepo, store := newUserServiceMediaAvatarTestMediaService()
	svc := NewUserService(repo, nil, nil, nil, mediaSvc)

	avatar, err := svc.SetAvatar(context.Background(), 15, "https://cdn.example.com/avatar.png")
	require.Nil(t, avatar)
	require.EqualError(t, err, "upsert avatar: avatar write failed")
	require.Len(t, mediaRepo.deletedIDs, 1)
	require.Equal(t, int64(1), mediaRepo.deletedIDs[0])
	require.NotEmpty(t, store.uploadedObjectKeys)
	require.NotEmpty(t, store.deletedObjectKeys)
}

func encodeBase64(raw []byte) string {
	return base64.StdEncoding.EncodeToString(raw)
}
