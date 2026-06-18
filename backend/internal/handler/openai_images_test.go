package handler

import (
	"net/http"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIImages_SelectionFailure_PreservesCompatibleAccountsMessage(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/images/generations", `{"model":"gpt-image-1","prompt":"cat"}`)
	h := newOpenAISelectionErrorTestHandler(t, nil)

	h.Images(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"message":"No available compatible accounts"`)
}

func TestOpenAIImages_SelectionFailureStoresImageRequestType(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/images/generations", `{"model":"gpt-image-1","prompt":"cat"}`)
	h := newOpenAISelectionErrorTestHandler(t, nil)

	h.Images(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	rt, ok := c.Get(opsRequestTypeKey)
	require.True(t, ok)
	require.Equal(t, int16(service.RequestTypeImage), rt)
}

func TestOpenAIImages_GroupModelUnsupported_ReturnsPermissionError(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/images/generations", `{"model":"gpt-image-1","prompt":"cat"}`)
	h := newOpenAISelectionErrorTestHandler(t, []service.Account{unsupportedOpenAIImagesTestAccount()})

	h.Images(c)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), `"type":"permission_error"`)
	require.Contains(t, rec.Body.String(), `requested model \"gpt-image-1\"`)
	require.Contains(t, rec.Body.String(), "gpt-5.4-mini")
}

func unsupportedOpenAIImagesTestAccount() service.Account {
	account := unsupportedOpenAITestAccount()
	account.Type = service.AccountTypeAPIKey
	return account
}

func TestOpenAIImages_PermissionFailureStoresImageRequestType(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/images/generations", `{"model":"gpt-image-1","prompt":"cat"}`)
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	require.True(t, ok)
	apiKey.Group.AllowImageGeneration = false
	h := newOpenAISelectionErrorTestHandler(t, nil)

	h.Images(c)

	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rt, ok := c.Get(opsRequestTypeKey)
	require.True(t, ok)
	require.Equal(t, int16(service.RequestTypeImage), rt)
}
