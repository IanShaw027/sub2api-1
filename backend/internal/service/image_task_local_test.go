package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLocalImageTaskNeverResolvesOrWritesObjectStorage(t *testing.T) {
	store := &imageTaskMemoryStore{}
	cloud := NewImageTaskServiceWithResolver(store, func() (*ImageResultUploader, bool) {
		t.Fatal("local generation must never resolve persistent object storage")
		return nil, false
	}, 24*time.Hour, time.Minute)
	local := cloud.ForLocalResults()
	require.True(t, local.Enabled(), "local generation does not require S3 configuration")
	owner := ImageTaskOwner{UserID: 7, APIKeyID: 9}
	task, err := local.Create(context.Background(), owner)
	require.NoError(t, err)
	require.True(t, store.task.LocalOnly)
	require.Equal(t, time.Hour, store.ttl)
	_, err = cloud.Get(context.Background(), owner, task.ID)
	require.ErrorIs(t, err, ErrImageTaskNotFound)
	require.ErrorIs(t, cloud.Complete(context.Background(), task.ID, http.StatusOK, json.RawMessage(`{"data":[{"b64_json":"iVBORw0KGgo="}]}`)), ErrImageTaskNotFound)
	require.NoError(t, local.Complete(context.Background(), task.ID, http.StatusOK, json.RawMessage(`{"data":[{"b64_json":"iVBORw0KGgo="}]}`)))
	result, err := local.Get(context.Background(), owner, task.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusCompleted, result.Status)
	require.Contains(t, string(result.Result), `"b64_json":"iVBORw0KGgo="`)
	require.Empty(t, result.ImageURL)
	require.Nil(t, result.StorageObject)
	require.Equal(t, time.Hour, store.ttl)
	_, err = local.Get(context.Background(), ImageTaskOwner{UserID: 8, APIKeyID: 9}, task.ID)
	require.ErrorIs(t, err, ErrImageTaskNotFound)
	_, err = local.Get(context.Background(), ImageTaskOwner{UserID: 7, APIKeyID: 10}, task.ID)
	require.ErrorIs(t, err, ErrImageTaskNotFound)
	store.task.LocalOnly = false
	_, err = local.Get(context.Background(), owner, task.ID)
	require.ErrorIs(t, err, ErrImageTaskNotFound, "local routes cannot accidentally expose old cloud tasks")
}

func TestLocalImageMaterializesRemoteBytesWithoutPersistentURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{137, 80, 78, 71, 13, 10, 26, 10})
	}))
	defer server.Close()
	input, err := json.Marshal(map[string]any{"data": []map[string]string{{"url": server.URL, "revised_prompt": "retained"}}})
	require.NoError(t, err)
	result, err := materializeLocalImageResult(context.Background(), input, server.Client())
	require.NoError(t, err)
	require.NotContains(t, string(result), server.URL)
	require.Contains(t, string(result), `"b64_json":"iVBORw0KGgo="`)
	require.Contains(t, string(result), `"mime_type":"image/png"`)
	require.Contains(t, string(result), `"revised_prompt":"retained"`)
}

func TestLocalImageMaterializerRejectsEmptyAndPrivateNetworkResults(t *testing.T) {
	_, err := materializeLocalImageResult(context.Background(), json.RawMessage(`{"data":[]}`), nil)
	require.Error(t, err)
	_, err = materializeLocalImageResult(context.Background(), json.RawMessage(`{"data":[{"url":"http://127.0.0.1/private"}]}`), nil)
	require.ErrorContains(t, err, "SSRF")
}

type localVideoOwnerBindingKey struct {
	groupID int64
	hash    string
}
type localVideoOwnerBindingCache struct {
	GatewayCache
	bindings map[localVideoOwnerBindingKey]int64
}

func (s *localVideoOwnerBindingCache) SetSessionAccountID(_ context.Context, groupID int64, hash string, accountID int64, _ time.Duration) error {
	s.bindings[localVideoOwnerBindingKey{groupID, hash}] = accountID
	return nil
}

func (s *localVideoOwnerBindingCache) GetSessionAccountID(_ context.Context, groupID int64, hash string) (int64, error) {
	return s.bindings[localVideoOwnerBindingKey{groupID, hash}], nil
}

func TestLocalVideoUsesExistingUserKeyAndGroupOwnerBinding(t *testing.T) {
	cache := &localVideoOwnerBindingCache{bindings: map[localVideoOwnerBindingKey]int64{}}
	gateway := &OpenAIGatewayService{cache: cache}
	ctx := context.Background()
	groupID := int64(3)
	require.NoError(t, gateway.BindGrokMediaVideoRequestAccount(ctx, &groupID, "video-owned", 7, 9, 42))
	account, err := gateway.ResolveGrokMediaVideoRequestAccount(ctx, &groupID, "video-owned", 7, 9)
	require.NoError(t, err)
	require.Equal(t, int64(42), account)
	for _, owner := range []struct{ group, user, key int64 }{{3, 8, 9}, {3, 7, 10}, {4, 7, 9}} {
		account, err := gateway.ResolveGrokMediaVideoRequestAccount(ctx, &owner.group, "video-owned", owner.user, owner.key)
		require.NoError(t, err)
		require.Zero(t, account, "another owner/group must not reach the bound upstream account")
	}
}
