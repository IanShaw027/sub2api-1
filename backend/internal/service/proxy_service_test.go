package service

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type proxyServiceRepoStub struct {
	proxy *Proxy
	err   error
}

func (r *proxyServiceRepoStub) Create(ctx context.Context, proxy *Proxy) error {
	panic("not implemented")
}
func (r *proxyServiceRepoStub) GetByID(ctx context.Context, id int64) (*Proxy, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.proxy == nil || r.proxy.ID != id {
		return nil, ErrProxyNotFound
	}
	return r.proxy, nil
}
func (r *proxyServiceRepoStub) ListByIDs(ctx context.Context, ids []int64) ([]Proxy, error) {
	panic("not implemented")
}
func (r *proxyServiceRepoStub) Update(ctx context.Context, proxy *Proxy) error {
	panic("not implemented")
}
func (r *proxyServiceRepoStub) Delete(ctx context.Context, id int64) error { panic("not implemented") }
func (r *proxyServiceRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]Proxy, *pagination.PaginationResult, error) {
	panic("not implemented")
}
func (r *proxyServiceRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]Proxy, *pagination.PaginationResult, error) {
	panic("not implemented")
}
func (r *proxyServiceRepoStub) ListWithFiltersAndAccountCount(ctx context.Context, params pagination.PaginationParams, protocol, status, search string) ([]ProxyWithAccountCount, *pagination.PaginationResult, error) {
	panic("not implemented")
}
func (r *proxyServiceRepoStub) ListActive(ctx context.Context) ([]Proxy, error) {
	panic("not implemented")
}
func (r *proxyServiceRepoStub) ListActiveWithAccountCount(ctx context.Context) ([]ProxyWithAccountCount, error) {
	panic("not implemented")
}
func (r *proxyServiceRepoStub) ExistsByHostPortAuth(ctx context.Context, host string, port int, username, password string) (bool, error) {
	panic("not implemented")
}
func (r *proxyServiceRepoStub) CountAccountsByProxyID(ctx context.Context, proxyID int64) (int64, error) {
	panic("not implemented")
}
func (r *proxyServiceRepoStub) ListAccountSummariesByProxyID(ctx context.Context, proxyID int64) ([]ProxyAccountSummary, error) {
	panic("not implemented")
}
func (r *proxyServiceRepoStub) SweepExpiredProxies(ctx context.Context, now time.Time) (int64, error) {
	panic("not implemented")
}
func (r *proxyServiceRepoStub) ListAllForFallback(ctx context.Context) ([]Proxy, error) {
	panic("not implemented")
}
func (r *proxyServiceRepoStub) CountExpired(ctx context.Context) (int64, error) {
	panic("not implemented")
}
func (r *proxyServiceRepoStub) CountExpiringSoon(ctx context.Context, now time.Time) (int64, error) {
	panic("not implemented")
}

func TestProxyService_TestConnection_ProxyNotFound(t *testing.T) {
	t.Parallel()

	svc := NewProxyService(&proxyServiceRepoStub{err: ErrProxyNotFound})

	err := svc.TestConnection(context.Background(), 99)

	require.Error(t, err)
	require.ErrorContains(t, err, "get proxy")
	require.ErrorIs(t, err, ErrProxyNotFound)
}

func TestProxyService_Create_NilRepoReturnsError(t *testing.T) {
	t.Parallel()

	svc := NewProxyService(nil)

	_, err := svc.Create(context.Background(), CreateProxyRequest{
		Protocol: "http",
		Host:     "proxy.local",
		Port:     8080,
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "proxy repository is unavailable")
}

func TestProxyService_GetByID_NilRepoReturnsError(t *testing.T) {
	t.Parallel()

	svc := NewProxyService(nil)

	_, err := svc.GetByID(context.Background(), 1)

	require.Error(t, err)
	require.ErrorContains(t, err, "proxy repository is unavailable")
}

func TestProxyService_TestConnection_NilRepoReturnsError(t *testing.T) {
	t.Parallel()

	svc := NewProxyService(nil)

	err := svc.TestConnection(context.Background(), 1)

	require.Error(t, err)
	require.ErrorContains(t, err, "proxy repository is unavailable")
}

func TestProxyService_TestConnection_InvalidProxyURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		proxy *Proxy
		want  string
	}{
		{
			name:  "missing host",
			proxy: &Proxy{ID: 1, Protocol: "http", Port: 8080},
			want:  "proxy URL missing host",
		},
		{
			name:  "unsupported protocol",
			proxy: &Proxy{ID: 1, Protocol: "ftp", Host: "proxy.local", Port: 21},
			want:  "unsupported proxy scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := NewProxyService(&proxyServiceRepoStub{proxy: tt.proxy})

			err := svc.TestConnection(context.Background(), tt.proxy.ID)

			require.Error(t, err)
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestProxyService_TestConnection_Success(t *testing.T) {
	t.Parallel()

	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.True(t, strings.Contains(r.RequestURI, "probe.local/health"), "request should be sent through proxy, got %q", r.RequestURI)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(proxyServer.Close)
	proxy := proxyFromServerURL(t, proxyServer.URL)
	svc := NewProxyService(&proxyServiceRepoStub{proxy: proxy})
	svc.testConnectionURL = "http://probe.local/health"
	svc.testConnectionTimeout = 500 * time.Millisecond

	err := svc.TestConnection(context.Background(), proxy.ID)

	require.NoError(t, err)
}

func TestProxyService_TestConnection_ProxyRequestFails(t *testing.T) {
	t.Parallel()

	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(proxyServer.Close)
	proxy := proxyFromServerURL(t, proxyServer.URL)
	svc := NewProxyService(&proxyServiceRepoStub{proxy: proxy})
	svc.testConnectionURL = "http://probe.local/health"
	svc.testConnectionTimeout = 500 * time.Millisecond

	err := svc.TestConnection(context.Background(), proxy.ID)

	require.Error(t, err)
	require.ErrorContains(t, err, "status 502")
}

func proxyFromServerURL(t *testing.T, raw string) *Proxy {
	t.Helper()
	parsed, err := url.Parse(raw)
	require.NoError(t, err)
	host, portRaw, err := net.SplitHostPort(parsed.Host)
	require.NoError(t, err)
	port, err := strconv.Atoi(portRaw)
	require.NoError(t, err)
	return &Proxy{ID: 1, Protocol: "http", Host: host, Port: port, Status: StatusActive}
}
