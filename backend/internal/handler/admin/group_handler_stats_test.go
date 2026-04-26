package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type groupStatsUsageRepo struct {
	service.UsageLogRepository
	requestedGroupID int64
	stats            []usagestats.GroupStat
}

func (r *groupStatsUsageRepo) GetGroupStatsWithFilters(
	ctx context.Context,
	startTime, endTime time.Time,
	userID, apiKeyID, accountID, groupID int64,
	requestType *int16,
	stream *bool,
	billingType *int8,
	billingMode string,
) ([]usagestats.GroupStat, error) {
	r.requestedGroupID = groupID
	return r.stats, nil
}

func TestGroupHandlerGetStatsReturnsRealGroupStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	groupID := int64(42)
	activeGroupID := groupID
	disabledGroupID := groupID
	otherGroupID := int64(99)
	adminSvc := newStubAdminService()
	adminSvc.apiKeys = []service.APIKey{
		{ID: 1, GroupID: &activeGroupID, Status: service.StatusActive},
		{ID: 2, GroupID: &disabledGroupID, Status: service.StatusAPIKeyDisabled},
		{ID: 3, GroupID: &otherGroupID, Status: service.StatusActive},
	}
	usageRepo := &groupStatsUsageRepo{
		stats: []usagestats.GroupStat{
			{GroupID: groupID, Requests: 17, Cost: 3.75},
		},
	}
	dashboardSvc := service.NewDashboardService(usageRepo, nil, nil, nil)
	handler := NewGroupHandler(adminSvc, dashboardSvc, nil)

	router := gin.New()
	router.GET("/api/v1/admin/groups/:id/stats", handler.GetStats)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/42/stats", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, groupID, usageRepo.requestedGroupID)

	var body struct {
		Code int `json:"code"`
		Data struct {
			TotalAPIKeys  int64   `json:"total_api_keys"`
			ActiveAPIKeys int64   `json:"active_api_keys"`
			TotalRequests int64   `json:"total_requests"`
			TotalCost     float64 `json:"total_cost"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	require.Equal(t, int64(2), body.Data.TotalAPIKeys)
	require.Equal(t, int64(1), body.Data.ActiveAPIKeys)
	require.Equal(t, int64(17), body.Data.TotalRequests)
	require.Equal(t, 3.75, body.Data.TotalCost)
}
