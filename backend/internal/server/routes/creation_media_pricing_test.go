//go:build unit

package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type creationPricingRouteUsers struct{ service.UserRepository }

type creationPricingRouteSettings struct {
	creationRouteSettingRepoStub
}

func (s *creationPricingRouteSettings) SetMultiple(_ context.Context, updates map[string]string) error {
	for key, value := range updates {
		s.values[key] = value
	}
	return nil
}

func (creationPricingRouteUsers) GetByID(context.Context, int64) (*service.User, error) {
	return &service.User{ID: 1, Status: service.StatusActive, Balance: -5}, nil
}

type creationPricingRouteKeys struct {
	service.APIKeyRepository
	key   *service.APIKey
	reads int
}

func (s *creationPricingRouteKeys) GetByUserGroupAndPurpose(context.Context, int64, int64, string) (*service.APIKey, error) {
	s.reads++
	return s.key, nil
}

// All key writes and auth lookups remain unimplemented, so accidental gateway
// context injection or hidden-key creation fails the request in this fixture.
type creationPricingRouteTasks struct{ task *service.ImageTaskRecord }

func (s *creationPricingRouteTasks) Save(_ context.Context, task *service.ImageTaskRecord, _ time.Duration) error {
	s.task = task
	return nil
}

func (s *creationPricingRouteTasks) Get(_ context.Context, id string) (*service.ImageTaskRecord, error) {
	if s.task == nil || s.task.ID != id {
		return nil, service.ErrImageTaskNotFound
	}
	copy := *s.task
	return &copy, nil
}

type creationPricingRouteReceipts struct{}

func (creationPricingRouteReceipts) Recorded(context.Context, string, int64) (bool, error) {
	return true, nil
}

type creationPricingRouteUsage struct {
	service.UsageLogRepository
	row service.UsageLog
}

func (s creationPricingRouteUsage) ListWithFilters(_ context.Context, _ pagination.PaginationParams, f usagestats.UsageLogFilters) ([]service.UsageLog, *pagination.PaginationResult, error) {
	if f.UserID == s.row.UserID && f.APIKeyID == s.row.APIKeyID && f.GroupID == *s.row.GroupID && f.RequestID == s.row.RequestID {
		return []service.UsageLog{s.row}, nil, nil
	}
	return nil, nil, nil
}

func TestCreationMediaPricingRoutesGuardsAndReadOnlyAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name         string
		auth         bool
		feature      bool
		backendMode  bool
		subscription bool
		missingKey   bool
		endpoint     string
		code         int
		status       string
	}{
		{"pricing jwt", false, true, false, false, false, "pricing", http.StatusUnauthorized, ""},
		{"billing jwt", false, true, false, false, false, "billing", http.StatusUnauthorized, ""},
		{"pricing feature", true, false, false, false, false, "pricing", http.StatusForbidden, ""},
		{"billing feature", true, false, false, false, false, "billing", http.StatusForbidden, ""},
		{"pricing backend mode", true, true, true, false, false, "pricing", http.StatusForbidden, ""},
		{"billing backend mode", true, true, true, false, false, "billing", http.StatusForbidden, ""},
		{"pricing negative balance and no key", true, true, false, false, true, "pricing", http.StatusOK, "unavailable"},
		{"billing negative balance", true, true, false, false, false, "billing", http.StatusOK, "settled"},
		{"billing expired subscription", true, true, false, true, false, "billing", http.StatusOK, "settled"},
		{"billing missing key stays missing", true, true, false, false, true, "billing", http.StatusNotFound, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := &service.Group{ID: 3, Status: service.StatusActive, Platform: service.PlatformOpenAI}
			billingType := service.BillingTypeBalance
			if tc.subscription {
				group.SubscriptionType = "subscription"
				billingType = service.BillingTypeSubscription
			}
			// An accidental active-subscription lookup panics through the nil
			// embedded interface; accepted-task receipt reads must not perform it.
			creation := service.NewCreationService(nil, nil, nil, creationRouteGroupRepo{group: group}, creationPricingRouteUsers{}, struct {
				service.UserSubscriptionRepository
			}{})
			keys := &creationPricingRouteKeys{key: &service.APIKey{ID: 9, UserID: 1, GroupID: &group.ID}}
			if tc.missingKey {
				keys.key = nil
			}
			images := service.NewImageTaskService(&creationPricingRouteTasks{})
			ctx := context.WithValue(context.Background(), ctxkey.ClientRequestID, "route-billing")
			task, err := images.ForLocalResults().Create(ctx, service.ImageTaskOwner{UserID: 1, APIKeyID: 9})
			require.NoError(t, err)
			usage := creationPricingRouteUsage{row: service.UsageLog{ID: 15, UserID: 1, APIKeyID: 9, GroupID: &group.ID, RequestID: "client:route-billing", ActualCost: 0.25, BillingType: billingType}}
			pricing := service.NewCreationMediaPricingService(creation, service.NewCreationKeyResolver(keys, nil), nil, images, usage, creationPricingRouteReceipts{}, &config.Config{RunMode: config.RunModeStandard})
			settings := service.NewSettingService(&creationPricingRouteSettings{creationRouteSettingRepoStub: creationRouteSettingRepoStub{values: map[string]string{
				service.SettingKeyBackendModeEnabled:     map[bool]string{true: "true", false: "false"}[tc.backendMode],
				service.SettingKeyCreationCenterEnabled:  map[bool]string{true: "true", false: "false"}[tc.feature],
				service.SettingKeyPanelRateLimitSettings: `{"enabled":false}`,
			}}}, &config.Config{})
			// Backend mode is cached process-wide, not per SettingService. Use
			// the actual settings write path to invalidate a prior fixture's value.
			previousBackendMode := settings.IsBackendModeEnabled(context.Background())
			systemSettings := &service.SystemSettings{BackendModeEnabled: tc.backendMode, CreationCenterEnabled: tc.feature}
			require.NoError(t, settings.UpdateSettings(context.Background(), systemSettings))
			t.Cleanup(func() {
				systemSettings.BackendModeEnabled = previousBackendMode
				require.NoError(t, settings.UpdateSettings(context.Background(), systemSettings))
			})
			router := gin.New()
			gatewayContextInstalled := false
			router.Use(func(c *gin.Context) {
				c.Next()
				_, gatewayContextInstalled = middleware.GetAPIKeyFromContext(c)
			})
			jwt := middleware.JWTAuthMiddleware(func(c *gin.Context) {
				if !tc.auth {
					c.AbortWithStatus(http.StatusUnauthorized)
					return
				}
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
				c.Next()
			})
			RegisterCreationMediaPricingRoutes(router.Group("/api/v1"), handler.NewCreationMediaPricingHandler(pricing), jwt, settings, middleware.NewPanelRateLimiter(nil, settings))
			path := "/api/v1/creation/local/" + tc.endpoint + "?group_id=3&kind=image&model=gpt-image-1&size=1K&task_id=" + task.ID
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			require.Equal(t, tc.code, w.Code, w.Body.String())
			require.False(t, gatewayContextInstalled)
			if tc.endpoint == "pricing" || !tc.auth || !tc.feature || tc.backendMode {
				require.Zero(t, keys.reads)
			}
			if tc.status != "" {
				var body struct {
					Data service.CreationMediaPrice `json:"data"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				require.Equal(t, tc.status, body.Data.Status)
				if tc.status == "settled" {
					require.Equal(t, 0.25, *body.Data.Amount)
					if tc.subscription {
						require.Equal(t, "subscription", *body.Data.BillingTarget)
					}
				}
			}
		})
	}
}
