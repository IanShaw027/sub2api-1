//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorUserHandlerList_NilSettingServiceReturnsEmptyList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewChannelMonitorUserHandler(nil, nil)
	r := gin.New()
	r.GET("/channel-monitors", h.List)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/channel-monitors", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"code":0,"message":"success","data":{"items":[]}}`, w.Body.String())
}

func TestChannelMonitorUserHandlerGetStatus_NilSettingServiceReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := NewChannelMonitorUserHandler(nil, nil)
	r := gin.New()
	r.GET("/channel-monitors/:id/status", h.GetStatus)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/channel-monitors/1/status", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}
