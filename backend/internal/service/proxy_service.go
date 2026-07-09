package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyutil"
)

var (
	ErrProxyNotFound = infraerrors.NotFound("PROXY_NOT_FOUND", "proxy not found")
	ErrProxyInUse    = infraerrors.Conflict("PROXY_IN_USE", "proxy is in use by accounts")
)

type ProxyRepository interface {
	Create(ctx context.Context, proxy *Proxy) error
	GetByID(ctx context.Context, id int64) (*Proxy, error)
	ListByIDs(ctx context.Context, ids []int64) ([]Proxy, error)
	Update(ctx context.Context, proxy *Proxy) error
	Delete(ctx context.Context, id int64) error

	List(ctx context.Context, params pagination.PaginationParams) ([]Proxy, *pagination.PaginationResult, error)
	ListWithFilters(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]Proxy, *pagination.PaginationResult, error)
	ListWithFiltersAndAccountCount(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]ProxyWithAccountCount, *pagination.PaginationResult, error)
	ListActive(ctx context.Context) ([]Proxy, error)
	ListActiveWithAccountCount(ctx context.Context) ([]ProxyWithAccountCount, error)

	ExistsByHostPortAuth(ctx context.Context, host string, port int, username, password string) (bool, error)
	CountAccountsByProxyID(ctx context.Context, proxyID int64) (int64, error)
	ListAccountSummariesByProxyID(ctx context.Context, proxyID int64) ([]ProxyAccountSummary, error)

	SweepExpiredProxies(ctx context.Context, now time.Time) (changed int64, err error)
	ListAllForFallback(ctx context.Context) ([]Proxy, error)
	CountExpired(ctx context.Context) (int64, error)
	CountExpiringSoon(ctx context.Context, now time.Time) (int64, error)
}

// CreateProxyRequest 创建代理请求
type CreateProxyRequest struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// UpdateProxyRequest 更新代理请求
type UpdateProxyRequest struct {
	Name     *string `json:"name"`
	Protocol *string `json:"protocol"`
	Host     *string `json:"host"`
	Port     *int    `json:"port"`
	Username *string `json:"username"`
	Password *string `json:"password"`
	Status   *string `json:"status"`
}

// ProxyService 代理管理服务
type ProxyService struct {
	proxyRepo             ProxyRepository
	testConnectionURL     string
	testConnectionTimeout time.Duration
}

const (
	defaultProxyTestConnectionURL     = "http://www.gstatic.com/generate_204"
	defaultProxyTestConnectionTimeout = 5 * time.Second
)

// NewProxyService 创建代理服务实例
func NewProxyService(proxyRepo ProxyRepository) *ProxyService {
	return &ProxyService{
		proxyRepo: proxyRepo,
	}
}

func (s *ProxyService) requireProxyRepo() (ProxyRepository, error) {
	if s == nil || s.proxyRepo == nil {
		return nil, fmt.Errorf("proxy repository is unavailable")
	}
	return s.proxyRepo, nil
}

// Create 创建代理
func (s *ProxyService) Create(ctx context.Context, req CreateProxyRequest) (*Proxy, error) {
	proxyRepo, err := s.requireProxyRepo()
	if err != nil {
		return nil, err
	}
	// 创建代理
	proxy := &Proxy{
		Name:     req.Name,
		Protocol: req.Protocol,
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
		Status:   StatusActive,
	}

	if err := proxyRepo.Create(ctx, proxy); err != nil {
		return nil, fmt.Errorf("create proxy: %w", err)
	}

	return proxy, nil
}

// GetByID 根据ID获取代理
func (s *ProxyService) GetByID(ctx context.Context, id int64) (*Proxy, error) {
	proxyRepo, err := s.requireProxyRepo()
	if err != nil {
		return nil, err
	}
	proxy, err := proxyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get proxy: %w", err)
	}
	return proxy, nil
}

// List 获取代理列表
func (s *ProxyService) List(ctx context.Context, params pagination.PaginationParams) ([]Proxy, *pagination.PaginationResult, error) {
	proxyRepo, err := s.requireProxyRepo()
	if err != nil {
		return nil, nil, err
	}
	proxies, pagination, err := proxyRepo.List(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("list proxies: %w", err)
	}
	return proxies, pagination, nil
}

// ListActive 获取活跃代理列表
func (s *ProxyService) ListActive(ctx context.Context) ([]Proxy, error) {
	proxyRepo, err := s.requireProxyRepo()
	if err != nil {
		return nil, err
	}
	proxies, err := proxyRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active proxies: %w", err)
	}
	return proxies, nil
}

// Update 更新代理
func (s *ProxyService) Update(ctx context.Context, id int64, req UpdateProxyRequest) (*Proxy, error) {
	proxyRepo, err := s.requireProxyRepo()
	if err != nil {
		return nil, err
	}
	proxy, err := proxyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get proxy: %w", err)
	}

	// 更新字段
	if req.Name != nil {
		proxy.Name = *req.Name
	}

	if req.Protocol != nil {
		proxy.Protocol = *req.Protocol
	}

	if req.Host != nil {
		proxy.Host = *req.Host
	}

	if req.Port != nil {
		proxy.Port = *req.Port
	}

	if req.Username != nil {
		proxy.Username = *req.Username
	}

	if req.Password != nil {
		proxy.Password = *req.Password
	}

	if req.Status != nil {
		proxy.Status = *req.Status
	}

	if err := proxyRepo.Update(ctx, proxy); err != nil {
		return nil, fmt.Errorf("update proxy: %w", err)
	}

	return proxy, nil
}

// Delete 删除代理
func (s *ProxyService) Delete(ctx context.Context, id int64) error {
	proxyRepo, err := s.requireProxyRepo()
	if err != nil {
		return err
	}
	// 检查代理是否存在
	_, err = proxyRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get proxy: %w", err)
	}

	if err := proxyRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete proxy: %w", err)
	}

	return nil
}

// TestConnection 测试代理连接
func (s *ProxyService) TestConnection(ctx context.Context, id int64) error {
	proxyRepo, err := s.requireProxyRepo()
	if err != nil {
		return err
	}
	proxy, err := proxyRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get proxy: %w", err)
	}

	_, parsedProxy, err := proxyurl.Parse(proxy.URL())
	if err != nil {
		return fmt.Errorf("parse proxy URL: %w", err)
	}

	timeout := s.testConnectionTimeout
	if timeout <= 0 {
		timeout = defaultProxyTestConnectionTimeout
	}

	targetURL := s.testConnectionURL
	if targetURL == "" {
		targetURL = defaultProxyTestConnectionURL
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{Timeout: timeout}).DialContext,
	}
	if err := proxyutil.ConfigureTransportProxy(transport, parsedProxy); err != nil {
		return fmt.Errorf("configure proxy: %w", err)
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(testCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return fmt.Errorf("create proxy test request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("test proxy connection: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("test proxy connection: status %d", resp.StatusCode)
	}

	return nil
}

// GetURL 获取代理URL
func (s *ProxyService) GetURL(ctx context.Context, id int64) (string, error) {
	proxyRepo, err := s.requireProxyRepo()
	if err != nil {
		return "", err
	}
	proxy, err := proxyRepo.GetByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("get proxy: %w", err)
	}

	return proxy.URL(), nil
}
