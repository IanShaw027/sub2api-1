package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type tlsFingerprintProfileRepoStub struct {
	profiles []*model.TLSFingerprintProfile
}

func (r *tlsFingerprintProfileRepoStub) List(context.Context) ([]*model.TLSFingerprintProfile, error) {
	return r.profiles, nil
}

func (r *tlsFingerprintProfileRepoStub) GetByID(context.Context, int64) (*model.TLSFingerprintProfile, error) {
	return nil, nil
}

func (r *tlsFingerprintProfileRepoStub) Create(_ context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	return profile, nil
}

func (r *tlsFingerprintProfileRepoStub) Update(_ context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	return profile, nil
}

func (r *tlsFingerprintProfileRepoStub) Delete(context.Context, int64) error {
	return nil
}

func TestTLSFingerprintProfileHandlerListCompleteOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	description := "Claude Code macOS HTTP/1.1"
	h := NewTLSFingerprintProfileHandler(service.NewTLSFingerprintProfileService(&tlsFingerprintProfileRepoStub{
		profiles: []*model.TLSFingerprintProfile{
			{
				ID:            301,
				Name:          "pin:claude-code:macos:h1",
				Description:   &description,
				CipherSuites:  []uint16{0x1301},
				Extensions:    []uint16{0},
				ALPNProtocols: []string{"http/1.1"},
			},
			{
				ID:            302,
				Name:          "pin:grok-cli:macos:h1",
				CipherSuites:  []uint16{0x1301},
				Extensions:    []uint16{0},
				ALPNProtocols: []string{"http/1.1"},
			},
		},
	}, nil))

	router := gin.New()
	router.GET("/api/v1/admin/tls-fingerprint-profiles/complete", h.ListCompleteOptions)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tls-fingerprint-profiles/complete?platform=anthropic", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Code int `json:"code"`
		Data []struct {
			ID            int64  `json:"id"`
			Name          string `json:"name"`
			Description   string `json:"description"`
			ClientFamily  string `json:"client_family"`
			OSFamily      string `json:"os_family"`
			Transport     string `json:"transport"`
			SoftwareLabel string `json:"software_label"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data, 1)
	require.Equal(t, int64(301), resp.Data[0].ID)
	require.Equal(t, "pin:claude-code:macos:h1", resp.Data[0].Name)
	require.Equal(t, description, resp.Data[0].Description)
	require.Equal(t, service.ClientFamilyClaudeCode, resp.Data[0].ClientFamily)
	require.Equal(t, "macos", resp.Data[0].OSFamily)
	require.Equal(t, service.TransportH1, resp.Data[0].Transport)
	require.Equal(t, claude.CLICurrentVersion+" / "+claude.CLIStainlessPackageVersion, resp.Data[0].SoftwareLabel)
}
