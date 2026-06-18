//go:build unit

package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type tlsFingerprintRouterHandlerRepoStub struct {
	routers []*model.TLSFingerprintRouter
}

func (r *tlsFingerprintRouterHandlerRepoStub) List(context.Context) ([]*model.TLSFingerprintRouter, error) {
	return r.routers, nil
}

func (r *tlsFingerprintRouterHandlerRepoStub) GetByID(_ context.Context, id int64) (*model.TLSFingerprintRouter, error) {
	for _, router := range r.routers {
		if router.ID == id {
			return router, nil
		}
	}
	return nil, nil
}

func (r *tlsFingerprintRouterHandlerRepoStub) Create(_ context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error) {
	r.routers = append(r.routers, router)
	return router, nil
}

func (r *tlsFingerprintRouterHandlerRepoStub) Update(_ context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error) {
	for i, existing := range r.routers {
		if existing.ID == router.ID {
			r.routers[i] = router
			return router, nil
		}
	}
	r.routers = append(r.routers, router)
	return router, nil
}

func (r *tlsFingerprintRouterHandlerRepoStub) Delete(_ context.Context, id int64) error {
	next := r.routers[:0]
	for _, router := range r.routers {
		if router.ID != id {
			next = append(next, router)
		}
	}
	r.routers = next
	return nil
}

func TestTLSFingerprintRouterHandlerUpdateNullDescriptionClearsField(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &tlsFingerprintRouterHandlerRepoStub{
		routers: []*model.TLSFingerprintRouter{
			{
				ID:          1,
				Name:        "router-1",
				Description: strPtr("keep me"),
				Enabled:     true,
				Rules:       []model.TLSFingerprintRouterRule{},
			},
		},
	}
	handler := NewTLSFingerprintRouterHandler(service.NewTLSFingerprintRouterService(repo, nil))

	body := []byte(`{"description":null}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/tls-fingerprint-routers/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	handler.Update(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Nil(t, repo.routers[0].Description)

	var payload struct {
		Code    int            `json:"code"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.Equal(t, "success", payload.Message)
}

func strPtr(s string) *string {
	return &s
}
