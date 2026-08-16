package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type memoryDeviceProfileRepo struct {
	mu       sync.Mutex
	profiles map[int64]*service.AccountDeviceProfile
}

func (r *memoryDeviceProfileRepo) GetByAccountID(_ context.Context, accountID int64) (*service.AccountDeviceProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.profiles[accountID]
	if p == nil {
		return nil, nil
	}
	cp := *p
	return &cp, nil
}

func (r *memoryDeviceProfileRepo) InsertBaseline(_ context.Context, p *service.AccountDeviceProfile) (*service.AccountDeviceProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.profiles == nil {
		r.profiles = map[int64]*service.AccountDeviceProfile{}
	}
	cp := *p
	if cp.ID == 0 {
		cp.ID = cp.AccountID
	}
	r.profiles[p.AccountID] = &cp
	out := cp
	return &out, nil
}

func (r *memoryDeviceProfileRepo) UpdateCAS(context.Context, int64, int64, *service.AccountDeviceProfile) (bool, error) {
	return false, nil
}

func (r *memoryDeviceProfileRepo) DeleteByAccountID(_ context.Context, accountID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.profiles, accountID)
	return nil
}

type identityRejectDeviceProfileRepo struct {
	memoryDeviceProfileRepo
}

func (r *identityRejectDeviceProfileRepo) DeleteByAccountID(context.Context, int64) error {
	return errors.New("identity_reject: remint blocked")
}

func setupResetDeviceProfileRouter(adminSvc *stubAdminService, deviceSvc *service.AccountDeviceService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	handler.SetAccountDeviceService(deviceSvc)
	router := gin.New()
	router.GET("/api/v1/admin/accounts/:id/device-profile", handler.GetDeviceProfile)
	router.POST("/api/v1/admin/accounts/:id/reset-device-profile", handler.ResetDeviceProfile)
	return router
}

func TestResetDeviceProfileInvalidID(t *testing.T) {
	router := setupResetDeviceProfileRouter(newStubAdminService(), service.NewAccountDeviceService(&memoryDeviceProfileRepo{}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/abc/reset-device-profile", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestResetDeviceProfileServiceUnavailable(t *testing.T) {
	router := setupResetDeviceProfileRouter(newStubAdminService(), nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/3/reset-device-profile", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestResetDeviceProfileRemintsCurrentPlatform(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.getAccountResult = &service.Account{
		ID:       3,
		Name:     "grok-after-platform-edit",
		Platform: service.PlatformGrok,
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
	}
	repo := &memoryDeviceProfileRepo{profiles: map[int64]*service.AccountDeviceProfile{
		3: {
			AccountID:          3,
			Revision:           1,
			SchemaVersion:      1,
			Platform:           service.PlatformAnthropic,
			ClientFamily:       service.ClientFamilyClaudeCode,
			InstallationID:     "11111111-1111-4111-8111-111111111111",
			DeviceID:           "22222222-2222-4222-8222-222222222222",
			GatewayAccountUUID: "55555555-5555-4555-8555-555555555555",
			SessionNamespace:   "0123456789abcdef0123456789abcdef",
		},
	}}
	router := setupResetDeviceProfileRouter(adminSvc, service.NewAccountDeviceService(repo))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/3/reset-device-profile", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var payload struct {
		Data struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, int64(3), payload.Data.ID)
	require.Equal(t, "grok-after-platform-edit", payload.Data.Name)

	got, err := repo.GetByAccountID(context.Background(), 3)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, service.PlatformGrok, got.Platform)
	require.Equal(t, service.ClientFamilyGrokCLI, got.ClientFamily)
	require.NotEqual(t, "11111111-1111-4111-8111-111111111111", got.InstallationID)
}

func TestResetDeviceProfileIdentityRejectIsBadRequest(t *testing.T) {
	adminSvc := newStubAdminService()
	adminSvc.getAccountResult = &service.Account{
		ID:       3,
		Name:     "grok-account",
		Platform: service.PlatformGrok,
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
	}
	router := setupResetDeviceProfileRouter(adminSvc, service.NewAccountDeviceService(&identityRejectDeviceProfileRepo{}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/3/reset-device-profile", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var payload struct {
		Reason  string `json:"reason"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, "IDENTITY_REJECT", payload.Reason)
	require.Contains(t, payload.Message, "identity_reject")
}

func TestGetDeviceProfileReturnsInspectDTO(t *testing.T) {
	updatedAt := time.Date(2026, 8, 16, 7, 51, 0, 0, time.UTC)
	repo := &memoryDeviceProfileRepo{profiles: map[int64]*service.AccountDeviceProfile{
		3: {
			AccountID:          3,
			Revision:           4,
			Platform:           service.PlatformAnthropic,
			ClientFamily:       service.ClientFamilyClaudeCode,
			ClientVersion:      "1.2.3",
			OSFamily:           "macos",
			Arch:               "arm64",
			LearnedFrom:        service.LearnedFromOfficial,
			LearningEnabled:    true,
			TransportFamily:    service.TransportH1,
			UpdatedAt:          updatedAt,
			InstallationID:     "11111111-1111-4111-8111-111111111111",
			DeviceID:           "22222222-2222-4222-8222-222222222222",
			ClientID:           "33333333-3333-4333-8333-333333333333",
			MachineID:          "44444444-4444-4444-8444-444444444444",
			GatewayAccountUUID: "55555555-5555-4555-8555-555555555555",
			SessionNamespace:   "0123456789abcdef0123456789abcdef",
			ProfilePayload: map[string]any{
				"user_agent": "claude-cli/1.2.3",
				"originator": "secret-originator",
			},
		},
	}}
	router := setupResetDeviceProfileRouter(newStubAdminService(), service.NewAccountDeviceService(repo))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/3/device-profile", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	raw := string(envelope.Data)
	for _, key := range []string{
		"session_namespace",
		"device_id",
		"installation_id",
		"client_id",
		"machine_id",
		"gateway_account_uuid",
		"profile_payload",
		"originator",
	} {
		require.NotContains(t, raw, `"`+key+`"`)
	}

	var payload struct {
		AccountID       int64     `json:"account_id"`
		Platform        string    `json:"platform"`
		ClientFamily    string    `json:"client_family"`
		ClientVersion   string    `json:"client_version"`
		UserAgent       string    `json:"user_agent"`
		OSFamily        string    `json:"os_family"`
		Arch            string    `json:"arch"`
		Revision        int64     `json:"revision"`
		LearnedFrom     string    `json:"learned_from"`
		LearningEnabled bool      `json:"learning_enabled"`
		TransportFamily string    `json:"transport_family"`
		UpdatedAt       time.Time `json:"updated_at"`
	}
	require.NoError(t, json.Unmarshal(envelope.Data, &payload))
	require.Equal(t, int64(3), payload.AccountID)
	require.Equal(t, service.PlatformAnthropic, payload.Platform)
	require.Equal(t, service.ClientFamilyClaudeCode, payload.ClientFamily)
	require.Equal(t, "1.2.3", payload.ClientVersion)
	require.Equal(t, "claude-cli/1.2.3", payload.UserAgent)
	require.Equal(t, "macos", payload.OSFamily)
	require.Equal(t, "arm64", payload.Arch)
	require.Equal(t, int64(4), payload.Revision)
	require.Equal(t, service.LearnedFromOfficial, payload.LearnedFrom)
	require.True(t, payload.LearningEnabled)
	require.Equal(t, service.TransportH1, payload.TransportFamily)
	require.True(t, payload.UpdatedAt.Equal(updatedAt))

	var keys map[string]any
	require.NoError(t, json.Unmarshal(envelope.Data, &keys))
	require.Equal(t, map[string]struct{}{
		"account_id":       {},
		"platform":         {},
		"client_family":    {},
		"client_version":   {},
		"user_agent":       {},
		"os_family":        {},
		"arch":             {},
		"revision":         {},
		"learned_from":     {},
		"learning_enabled": {},
		"transport_family": {},
		"updated_at":       {},
	}, keySet(keys))
}

func TestGetDeviceProfileNotFound(t *testing.T) {
	repo := &memoryDeviceProfileRepo{}
	router := setupResetDeviceProfileRouter(newStubAdminService(), service.NewAccountDeviceService(repo))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/3/device-profile", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	var payload struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, "DEVICE_PROFILE_NOT_FOUND", payload.Reason)

	got, err := repo.GetByAccountID(context.Background(), 3)
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestGetDeviceProfileInvalidID(t *testing.T) {
	router := setupResetDeviceProfileRouter(newStubAdminService(), service.NewAccountDeviceService(&memoryDeviceProfileRepo{}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/abc/device-profile", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetDeviceProfileServiceUnavailable(t *testing.T) {
	router := setupResetDeviceProfileRouter(newStubAdminService(), nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/3/device-profile", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func keySet(m map[string]any) map[string]struct{} {
	out := make(map[string]struct{}, len(m))
	for k := range m {
		out[k] = struct{}{}
	}
	return out
}
