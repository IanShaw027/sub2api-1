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

func TestOpenAIImages2API_SelectionFailureStoresImageWebBridgeRequestType(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/images2api/generations", `{"model":"gpt-image-1","prompt":"cat"}`)
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	require.True(t, ok)
	apiKey.Group.ImageGenerationRoute = service.GroupImageGenerationRouteWeb2API
	h := newOpenAISelectionErrorTestHandler(t, nil)

	h.Images(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	rt, ok := c.Get(opsRequestTypeKey)
	require.True(t, ok)
	require.Equal(t, int16(service.RequestTypeImageWebBridge), rt)
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

func TestOpenAIImages2API_RouteDisabledStoresImageWebBridgeRequestType(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/images2api/generations", `{"model":"gpt-image-1","prompt":"cat"}`)
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	require.True(t, ok)
	apiKey.Group.ImageGenerationRoute = service.GroupImageGenerationRouteCodex
	h := newOpenAISelectionErrorTestHandler(t, nil)

	h.Images(c)

	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rt, ok := c.Get(opsRequestTypeKey)
	require.True(t, ok)
	require.Equal(t, int16(service.RequestTypeImageWebBridge), rt)
}
