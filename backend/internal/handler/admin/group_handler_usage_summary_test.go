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

type groupUsageSummaryRepo struct {
	service.UsageLogRepository
	requestedIDs        []int64
	requestedTodayStart time.Time
}

func (r *groupUsageSummaryRepo) GetAllGroupUsageSummary(_ context.Context, _ time.Time) ([]usagestats.GroupUsageSummary, error) {
	return []usagestats.GroupUsageSummary{
		{GroupID: 1, TodayCost: 1.1, TotalCost: 11.1},
		{GroupID: 2, TodayCost: 2.2, TotalCost: 22.2},
		{GroupID: 9, TodayCost: 9.9, TotalCost: 99.9},
	}, nil
}

func (r *groupUsageSummaryRepo) GetGroupUsageSummaryByIDs(_ context.Context, ids []int64, todayStart time.Time) ([]usagestats.GroupUsageSummary, error) {
	r.requestedIDs = append([]int64(nil), ids...)
	r.requestedTodayStart = todayStart
	return []usagestats.GroupUsageSummary{
		{GroupID: 1, TodayCost: 1.1, TotalCost: 11.1},
		{GroupID: 2, TodayCost: 2.2, TotalCost: 22.2},
	}, nil
}

func TestGroupHandlerGetUsageSummaryScopesToRequestedIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	usageRepo := &groupUsageSummaryRepo{}
	dashboardSvc := service.NewDashboardService(usageRepo, nil, nil, nil)
	handler := NewGroupHandler(newStubAdminService(), dashboardSvc, nil)

	router := gin.New()
	router.GET("/api/v1/admin/groups/usage-summary", handler.GetUsageSummary)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/groups/usage-summary?timezone=Asia/Shanghai&ids=1,2", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []int64{1, 2}, usageRepo.requestedIDs)
	require.False(t, usageRepo.requestedTodayStart.IsZero())

	var body struct {
		Code int `json:"code"`
		Data []struct {
			GroupID int64 `json:"group_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	require.Equal(t, []int64{1, 2}, []int64{body.Data[0].GroupID, body.Data[1].GroupID})
}
