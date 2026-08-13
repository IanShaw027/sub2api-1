//go:build unit

package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type liveListUsersStub struct {
	service.AdminService
	users []service.User
}

func (s *liveListUsersStub) ListUsers(_ context.Context, page, pageSize int, _ service.UserListFilters, _, _ string) ([]service.User, int64, error) {
	start := (page - 1) * pageSize
	if start > len(s.users) {
		start = len(s.users)
	}
	end := start + pageSize
	if end > len(s.users) {
		end = len(s.users)
	}
	return append([]service.User(nil), s.users[start:end]...), int64(len(s.users)), nil
}

type usageStatsReaderStub struct {
	stats map[int64]*usagestats.BatchUserUsageStats
}

func (s *usageStatsReaderStub) GetBatchUserUsageStats(context.Context, []int64, time.Time, time.Time) (map[int64]*usagestats.BatchUserUsageStats, error) {
	return s.stats, nil
}

func TestAdminUserList_ParsesIncludeUsageStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &listUsersFilterStub{AdminService: newStubAdminService()}
	h := NewUserHandler(stub, nil, nil, nil, nil, nil, nil)
	r := gin.New()
	r.GET("/admin/users", h.List)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/users?include_usage_stats=true", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, stub.captured.IncludeUsageStats)
	require.True(t, *stub.captured.IncludeUsageStats)
}

func TestAdminUserList_LiveConcurrencySortAndUsageStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminStub := &liveListUsersStub{
		AdminService: newStubAdminService(),
		users: []service.User{
			{ID: 1, Email: "a@example.com", Concurrency: 5},
			{ID: 2, Email: "b@example.com", Concurrency: 5},
			{ID: 3, Email: "c@example.com", Concurrency: 5},
		},
	}
	cache := &liveConcurrencyCacheStub{load: map[int64]*service.UserLoadInfo{
		1: {UserID: 1, CurrentConcurrency: 1},
		2: {UserID: 2, CurrentConcurrency: 4},
		3: {UserID: 3, CurrentConcurrency: 2},
	}}
	conc := service.NewConcurrencyService(cache)
	conc.SetSlotHeartbeatInterval(0)
	h := NewUserHandler(adminStub, conc, nil, nil, nil, nil, nil)
	h.SetUsageStatsReader(&usageStatsReaderStub{stats: map[int64]*usagestats.BatchUserUsageStats{
		2: {UserID: 2, TodayActualCost: 1.5, TotalActualCost: 9, TodayBalanceActualCost: 1.1, TodaySubscriptionActualCost: 0.4},
	}})

	r := gin.New()
	r.GET("/admin/users", h.List)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/users?sort_by=current_concurrency&sort_order=desc&include_usage_stats=true&page_size=2", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data struct {
			Items []struct {
				ID                          int64    `json:"id"`
				CurrentConcurrency          int      `json:"current_concurrency"`
				TodayBalanceActualCost      *float64 `json:"today_balance_actual_cost"`
				TodaySubscriptionActualCost *float64 `json:"today_subscription_actual_cost"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, int64(3), body.Data.Total)
	require.Len(t, body.Data.Items, 2)
	require.Equal(t, int64(2), body.Data.Items[0].ID)
	require.Equal(t, 4, body.Data.Items[0].CurrentConcurrency)
	require.NotNil(t, body.Data.Items[0].TodayBalanceActualCost)
	require.InDelta(t, 1.1, *body.Data.Items[0].TodayBalanceActualCost, 0.0001)
	require.Equal(t, int64(3), body.Data.Items[1].ID)
}

type liveConcurrencyCacheStub struct {
	service.ConcurrencyCache
	load map[int64]*service.UserLoadInfo
	err  error
}

func (c *liveConcurrencyCacheStub) GetUsersLoadBatch(_ context.Context, _ []service.UserWithConcurrency) (map[int64]*service.UserLoadInfo, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.load, nil
}

func TestAdminUserList_LiveConcurrencySortFailsOpenOnRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminStub := &liveListUsersStub{
		AdminService: newStubAdminService(),
		users: []service.User{
			{ID: 1, Email: "a@example.com", Concurrency: 5},
			{ID: 2, Email: "b@example.com", Concurrency: 5},
		},
	}
	cache := &liveConcurrencyCacheStub{err: errors.New("redis down")}
	conc := service.NewConcurrencyService(cache)
	conc.SetSlotHeartbeatInterval(0)
	h := NewUserHandler(adminStub, conc, nil, nil, nil, nil, nil)

	r := gin.New()
	r.GET("/admin/users", h.List)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/users?sort_by=current_concurrency&sort_order=desc", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}
