package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type proxyBatchListCall struct {
	page      int
	pageSize  int
	protocol  string
	status    string
	search    string
	sortBy    string
	sortOrder string
}

type proxyBatchAdminService struct {
	*stubAdminService

	mu                sync.Mutex
	listCalls         []proxyBatchListCall
	testedIDs         []int64
	qualityCheckedIDs []int64
	testResults       map[int64]*service.ProxyTestResult
	testErrors        map[int64]error
	qualityResults    map[int64]*service.ProxyQualityCheckResult
	qualityErrors     map[int64]error
}

func newProxyBatchAdminService(proxies []service.Proxy) *proxyBatchAdminService {
	base := newStubAdminService()
	base.proxies = proxies
	return &proxyBatchAdminService{
		stubAdminService: base,
		testResults:      map[int64]*service.ProxyTestResult{},
		testErrors:       map[int64]error{},
		qualityResults:   map[int64]*service.ProxyQualityCheckResult{},
		qualityErrors:    map[int64]error{},
	}
}

func (s *proxyBatchAdminService) ListProxies(_ context.Context, page, pageSize int, protocol, status, search string, sortBy, sortOrder string) ([]service.Proxy, int64, error) {
	s.mu.Lock()
	s.listCalls = append(s.listCalls, proxyBatchListCall{
		page:      page,
		pageSize:  pageSize,
		protocol:  protocol,
		status:    status,
		search:    search,
		sortBy:    sortBy,
		sortOrder: sortOrder,
	})
	s.mu.Unlock()

	filtered := make([]service.Proxy, 0, len(s.proxies))
	search = strings.TrimSpace(strings.ToLower(search))
	for _, proxy := range s.proxies {
		if protocol != "" && proxy.Protocol != protocol {
			continue
		}
		if status != "" && proxy.Status != status {
			continue
		}
		if search != "" {
			name := strings.ToLower(proxy.Name)
			host := strings.ToLower(proxy.Host)
			if !strings.Contains(name, search) && !strings.Contains(host, search) {
				continue
			}
		}
		filtered = append(filtered, proxy)
	}

	start := (page - 1) * pageSize
	if start >= len(filtered) {
		return []service.Proxy{}, int64(len(filtered)), nil
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return append([]service.Proxy(nil), filtered[start:end]...), int64(len(filtered)), nil
}

func (s *proxyBatchAdminService) TestProxy(_ context.Context, id int64) (*service.ProxyTestResult, error) {
	s.mu.Lock()
	s.testedIDs = append(s.testedIDs, id)
	s.mu.Unlock()

	if err := s.testErrors[id]; err != nil {
		return nil, err
	}
	if result, ok := s.testResults[id]; ok {
		return result, nil
	}
	return &service.ProxyTestResult{Success: true, Message: "ok"}, nil
}

func (s *proxyBatchAdminService) CheckProxyQuality(_ context.Context, id int64) (*service.ProxyQualityCheckResult, error) {
	s.mu.Lock()
	s.qualityCheckedIDs = append(s.qualityCheckedIDs, id)
	s.mu.Unlock()

	if err := s.qualityErrors[id]; err != nil {
		return nil, err
	}
	if result, ok := s.qualityResults[id]; ok {
		return result, nil
	}
	return &service.ProxyQualityCheckResult{
		ProxyID:        id,
		Score:          100,
		Grade:          "A",
		Summary:        "ok",
		PassedCount:    1,
		WarnCount:      0,
		FailedCount:    0,
		ChallengeCount: 0,
		CheckedAt:      1710000000,
		Items: []service.ProxyQualityCheckItem{
			{Target: "base_connectivity", Status: "pass"},
		},
	}, nil
}

func setupProxyBatchRouter(adminSvc service.AdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewProxyHandler(adminSvc)
	router.POST("/api/v1/admin/proxies/batch-test", handler.BatchTest)
	router.POST("/api/v1/admin/proxies/batch-quality-check", handler.BatchQualityCheck)
	return router
}

func TestProxyHandlerBatchTestUsesFilteredPaginationAndAggregatesResults(t *testing.T) {
	proxies := make([]service.Proxy, 0, 206)
	for i := 1; i <= 205; i++ {
		proxies = append(proxies, service.Proxy{
			ID:       int64(i),
			Name:     "edge-http-" + strings.Repeat("a", 0),
			Protocol: "http",
			Host:     "edge-host",
			Port:     8000 + i,
			Status:   service.StatusActive,
		})
	}
	proxies = append(proxies, service.Proxy{
		ID:       999,
		Name:     "skip-me",
		Protocol: "socks5",
		Host:     "other-host",
		Port:     9999,
		Status:   service.StatusDisabled,
	})

	adminSvc := newProxyBatchAdminService(proxies)
	adminSvc.testResults[2] = &service.ProxyTestResult{Success: false, Message: "timeout"}

	router := setupProxyBatchRouter(adminSvc)

	body, err := json.Marshal(map[string]any{
		"protocol":   "http",
		"status":     service.StatusActive,
		"search":     "edge-http",
		"sort_by":    "name",
		"sort_order": "asc",
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/proxies/batch-test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Total   int `json:"total"`
			Success int `json:"success"`
			Failed  int `json:"failed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.Equal(t, "success", payload.Message)
	require.Equal(t, 205, payload.Data.Total)
	require.Equal(t, 204, payload.Data.Success)
	require.Equal(t, 1, payload.Data.Failed)

	require.Len(t, adminSvc.listCalls, 2)
	require.Equal(t, proxyBatchListCall{page: 1, pageSize: 200, protocol: "http", status: service.StatusActive, search: "edge-http", sortBy: "name", sortOrder: "asc"}, adminSvc.listCalls[0])
	require.Equal(t, proxyBatchListCall{page: 2, pageSize: 200, protocol: "http", status: service.StatusActive, search: "edge-http", sortBy: "name", sortOrder: "asc"}, adminSvc.listCalls[1])

	tested := append([]int64(nil), adminSvc.testedIDs...)
	sort.Slice(tested, func(i, j int) bool { return tested[i] < tested[j] })
	require.Len(t, tested, 205)
	require.Equal(t, int64(1), tested[0])
	require.Equal(t, int64(205), tested[len(tested)-1])
	require.NotContains(t, tested, int64(999))
}

func TestProxyHandlerBatchQualityCheckUsesFilteredPaginationAndSummaries(t *testing.T) {
	adminSvc := newProxyBatchAdminService([]service.Proxy{
		{ID: 1, Name: "edge-healthy", Protocol: "http", Host: "host-1", Port: 8001, Status: service.StatusActive},
		{ID: 2, Name: "edge-warn", Protocol: "http", Host: "host-2", Port: 8002, Status: service.StatusActive},
		{ID: 3, Name: "edge-challenge", Protocol: "http", Host: "host-3", Port: 8003, Status: service.StatusActive},
		{ID: 4, Name: "edge-failed", Protocol: "http", Host: "host-4", Port: 8004, Status: service.StatusActive},
		{ID: 5, Name: "other", Protocol: "socks5", Host: "host-5", Port: 8005, Status: service.StatusDisabled},
	})
	adminSvc.qualityResults[1] = &service.ProxyQualityCheckResult{ProxyID: 1, PassedCount: 1, CheckedAt: 1710000001, Items: []service.ProxyQualityCheckItem{{Target: "base_connectivity", Status: "pass"}}}
	adminSvc.qualityResults[2] = &service.ProxyQualityCheckResult{ProxyID: 2, WarnCount: 1, CheckedAt: 1710000002, Items: []service.ProxyQualityCheckItem{{Target: "base_connectivity", Status: "pass"}}}
	adminSvc.qualityResults[3] = &service.ProxyQualityCheckResult{ProxyID: 3, ChallengeCount: 1, CheckedAt: 1710000003, Items: []service.ProxyQualityCheckItem{{Target: "base_connectivity", Status: "pass"}}}
	adminSvc.qualityErrors[4] = context.DeadlineExceeded

	router := setupProxyBatchRouter(adminSvc)

	body, err := json.Marshal(map[string]any{
		"protocol":   "http",
		"status":     service.StatusActive,
		"search":     "edge-",
		"sort_by":    "id",
		"sort_order": "desc",
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/proxies/batch-quality-check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Total     int `json:"total"`
			Healthy   int `json:"healthy"`
			Warn      int `json:"warn"`
			Challenge int `json:"challenge"`
			Failed    int `json:"failed"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, 0, payload.Code)
	require.Equal(t, "success", payload.Message)
	require.Equal(t, 4, payload.Data.Total)
	require.Equal(t, 1, payload.Data.Healthy)
	require.Equal(t, 1, payload.Data.Warn)
	require.Equal(t, 1, payload.Data.Challenge)
	require.Equal(t, 1, payload.Data.Failed)

	require.Len(t, adminSvc.listCalls, 1)
	require.Equal(t, proxyBatchListCall{page: 1, pageSize: 200, protocol: "http", status: service.StatusActive, search: "edge-", sortBy: "id", sortOrder: "desc"}, adminSvc.listCalls[0])

	checked := append([]int64(nil), adminSvc.qualityCheckedIDs...)
	sort.Slice(checked, func(i, j int) bool { return checked[i] < checked[j] })
	require.Equal(t, []int64{1, 2, 3, 4}, checked)
}
