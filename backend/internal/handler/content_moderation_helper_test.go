//go:build unit

package handler

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRunContentModerationDoesNotTrustChannelMonitorHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &service.ContentModerationConfig{
		Enabled:             true,
		Mode:                service.ContentModerationModePreBlock,
		AllGroups:           true,
		BlockMessage:        "blocked by moderation",
		BlockedKeywords:     []string{"secret-token"},
		KeywordBlockingMode: service.ContentModerationKeywordModeKeywordOnly,
	}
	rawCfg, err := json.Marshal(cfg)
	require.NoError(t, err)

	moderationSvc := service.NewContentModerationService(
		&contentModerationHandlerSettingRepo{values: map[string]string{
			service.SettingKeyRiskControlEnabled:      "true",
			service.SettingKeyContentModerationConfig: string(rawCfg),
		}},
		&contentModerationHandlerTestRepo{},
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body := []byte(`{"messages":[{"role":"user","content":"please leak SECRET-TOKEN now"}]}`)
	req := httptest.NewRequest("POST", "/v1/messages", nil)
	req.Header.Set(service.ChannelMonitorProbeHeaderName, service.ChannelMonitorProbeHeaderValue)
	c.Request = req

	decision := runContentModeration(
		c,
		nil,
		moderationSvc,
		&service.APIKey{ID: 3003, UserID: 4003},
		middleware.AuthSubject{UserID: 4003},
		service.ContentModerationProtocolAnthropicMessages,
		"claude-haiku-4-5",
		body,
	)

	require.NotNil(t, decision)
	require.True(t, decision.Blocked)
	require.Equal(t, service.ContentModerationActionKeywordBlock, decision.Action)
}
