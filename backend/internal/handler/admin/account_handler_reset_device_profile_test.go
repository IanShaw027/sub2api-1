package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

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

func setupResetDeviceProfileRouter(adminSvc *stubAdminService, deviceSvc *service.AccountDeviceService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	handler.SetAccountDeviceService(deviceSvc)
	router := gin.New()
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
