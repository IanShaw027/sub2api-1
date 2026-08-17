package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type tlsFingerprintProfileRouteRepoStub struct {
	profiles []*model.TLSFingerprintProfile
}

func (r *tlsFingerprintProfileRouteRepoStub) List(context.Context) ([]*model.TLSFingerprintProfile, error) {
	return r.profiles, nil
}

func (r *tlsFingerprintProfileRouteRepoStub) GetByID(context.Context, int64) (*model.TLSFingerprintProfile, error) {
	return nil, nil
}

func (r *tlsFingerprintProfileRouteRepoStub) Create(_ context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	return profile, nil
}

func (r *tlsFingerprintProfileRouteRepoStub) Update(_ context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	return profile, nil
}

func (r *tlsFingerprintProfileRouteRepoStub) Delete(context.Context, int64) error {
	return nil
}

func TestTLSFingerprintProfileRoutesCompleteBeforeID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tlsSvc := service.NewTLSFingerprintProfileService(&tlsFingerprintProfileRouteRepoStub{
		profiles: []*model.TLSFingerprintProfile{
			{
				ID:            401,
				Name:          "pin:claude-code:macos:h1",
				CipherSuites:  []uint16{0x1301},
				Extensions:    []uint16{0},
				ALPNProtocols: []string{"http/1.1"},
			},
		},
	}, nil)
	handlers := &handler.Handlers{
		Admin: &handler.AdminHandlers{
			TLSFingerprintProfile: adminhandler.NewTLSFingerprintProfileHandler(tlsSvc),
		},
	}
	router := gin.New()
	registerTLSFingerprintProfileRoutes(router.Group("/api/v1/admin"), handlers)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tls-fingerprint-profiles/complete?platform=anthropic", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Code int `json:"code"`
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data, 1)
	require.Equal(t, int64(401), resp.Data[0].ID)
}
