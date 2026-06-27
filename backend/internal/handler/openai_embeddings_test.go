package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIEmbeddings_SelectionFailure_ReturnsSupportingModelMessage(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/embeddings", `{"model":"text-embedding-3-large","input":"hello"}`)
	h := newOpenAISelectionErrorTestHandler(t, nil)

	h.Embeddings(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), `"message":"No available accounts supporting model: text-embedding-3-large"`)
}

func TestOpenAIEmbeddings_PreHashBlocksBeforeAccountSelection(t *testing.T) {
	body := `{"model":"text-embedding-3-large","input":"known bad embedding"}`
	c, rec := newOpenAISelectionErrorTestContext("/v1/embeddings", body)

	cfg := &service.ContentModerationConfig{
		Enabled:             true,
		Mode:                service.ContentModerationModePreBlock,
		AllGroups:           true,
		PreHashCheckEnabled: true,
		BlockStatus:         http.StatusForbidden,
		BlockMessage:        "blocked by hash",
		APIKeys:             []string{"sk-test"},
	}
	rawCfg, err := json.Marshal(cfg)
	require.NoError(t, err)

	hashes := map[string]struct{}{}
	for _, input := range service.ExtractContentModerationInputsForLocalBlock(service.ContentModerationProtocolOpenAIEmbeddings, []byte(body)) {
		hashes[input.Hash()] = struct{}{}
	}
	require.NotEmpty(t, hashes)
	hashCache := &recordingCyberPolicyHashCache{matched: hashes}
	moderationSvc := service.NewContentModerationService(
		&contentModerationHandlerSettingRepo{values: map[string]string{
			service.SettingKeyRiskControlEnabled:      "true",
			service.SettingKeyContentModerationConfig: string(rawCfg),
		}},
		&contentModerationHandlerTestRepo{},
		hashCache,
		nil,
		nil,
		nil,
		nil,
	)
	h := newOpenAISelectionErrorTestHandler(t, nil)
	h.contentModerationService = moderationSvc

	h.Embeddings(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "blocked by hash")
	require.NotEmpty(t, hashCache.snapshotChecked())
}
