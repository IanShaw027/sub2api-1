package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

func TestAntigravityOAuthServiceGenerateAuthURLFailsWhenProxyMissing(t *testing.T) {
	proxyID := int64(404)
	svc := NewAntigravityOAuthService(&mockAntigravityProxyRepo{})

	_, err := svc.GenerateAuthURL(context.Background(), &proxyID)

	if err == nil || !strings.Contains(err.Error(), "proxy not found") {
		t.Fatalf("expected proxy not found error, got %v", err)
	}
}

func TestAntigravityOAuthServiceExchangeCodeRejectsNilInput(t *testing.T) {
	svc := NewAntigravityOAuthService(&mockAntigravityProxyRepo{})

	_, err := svc.ExchangeCode(context.Background(), nil)
	if err == nil {
		t.Fatal("expected nil input error")
	}
	if !strings.Contains(err.Error(), "oauth input is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAntigravityOAuthServiceExchangeCodeRejectsProxyOverride(t *testing.T) {
	proxyID := int64(1)
	overrideProxyID := int64(2)
	svc := NewAntigravityOAuthService(&mockAntigravityProxyRepo{
		getByIDFunc: func(ctx context.Context, id int64) (*Proxy, error) {
			if id == proxyID {
				return &Proxy{ID: id, Protocol: "http", Host: "session.proxy", Port: 8080}, nil
			}
			if id == overrideProxyID {
				return &Proxy{ID: id, Protocol: "http", Host: "override.proxy", Port: 8080}, nil
			}
			return nil, fmt.Errorf("proxy not found")
		},
	})

	result, err := svc.GenerateAuthURL(context.Background(), &proxyID)
	if err != nil {
		t.Fatalf("GenerateAuthURL returned error: %v", err)
	}

	_, err = svc.ExchangeCode(context.Background(), &AntigravityExchangeCodeInput{
		SessionID: result.SessionID,
		State:     result.State,
		Code:      "code-1",
		ProxyID:   &overrideProxyID,
	})
	if err == nil || strings.Contains(err.Error(), "override.proxy") {
		t.Fatalf("expected exchange to ignore override proxy, got %v", err)
	}
}

func TestAntigravityOAuthServiceRefreshAccountTokenRejectsNilAccount(t *testing.T) {
	svc := NewAntigravityOAuthService(&mockAntigravityProxyRepo{})

	_, err := svc.RefreshAccountToken(context.Background(), nil)
	if err == nil {
		t.Fatal("expected nil account error")
	}
	if !strings.Contains(err.Error(), "Antigravity OAuth") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAntigravityOAuthServiceFillProjectIDRejectsNilAccount(t *testing.T) {
	svc := NewAntigravityOAuthService(&mockAntigravityProxyRepo{})

	projectID, err := svc.FillProjectID(context.Background(), nil, "access-token")
	if err == nil {
		t.Fatal("expected nil account error")
	}
	if projectID != "" {
		t.Fatalf("expected empty projectID, got %q", projectID)
	}
	if err.Error() != "account is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

type mockAntigravityProxyRepo struct {
	getByIDFunc func(ctx context.Context, id int64) (*Proxy, error)
}

func (m *mockAntigravityProxyRepo) Create(context.Context, *Proxy) error { panic("not impl") }

func (m *mockAntigravityProxyRepo) GetByID(ctx context.Context, id int64) (*Proxy, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, fmt.Errorf("proxy not found")
}

func (m *mockAntigravityProxyRepo) ListByIDs(context.Context, []int64) ([]Proxy, error) {
	panic("not impl")
}

func (m *mockAntigravityProxyRepo) Update(context.Context, *Proxy) error { panic("not impl") }

func (m *mockAntigravityProxyRepo) Delete(context.Context, int64) error { panic("not impl") }

func (m *mockAntigravityProxyRepo) List(context.Context, pagination.PaginationParams) ([]Proxy, *pagination.PaginationResult, error) {
	panic("not impl")
}

func (m *mockAntigravityProxyRepo) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string) ([]Proxy, *pagination.PaginationResult, error) {
	panic("not impl")
}

func (m *mockAntigravityProxyRepo) ListWithFiltersAndAccountCount(context.Context, pagination.PaginationParams, string, string, string) ([]ProxyWithAccountCount, *pagination.PaginationResult, error) {
	panic("not impl")
}

func (m *mockAntigravityProxyRepo) ListActive(context.Context) ([]Proxy, error) { panic("not impl") }

func (m *mockAntigravityProxyRepo) ListActiveWithAccountCount(context.Context) ([]ProxyWithAccountCount, error) {
	panic("not impl")
}

func (m *mockAntigravityProxyRepo) ExistsByHostPortAuth(context.Context, string, int, string, string) (bool, error) {
	panic("not impl")
}

func (m *mockAntigravityProxyRepo) CountAccountsByProxyID(context.Context, int64) (int64, error) {
	panic("not impl")
}

func (m *mockAntigravityProxyRepo) SweepExpiredProxies(context.Context, time.Time) (int64, error) {
	panic("not impl")
}

func (m *mockAntigravityProxyRepo) ListAllForFallback(context.Context) ([]Proxy, error) {
	panic("not impl")
}

func (m *mockAntigravityProxyRepo) CountExpired(context.Context) (int64, error) {
	panic("not impl")
}

func (m *mockAntigravityProxyRepo) CountExpiringSoon(context.Context, time.Time) (int64, error) {
	panic("not impl")
}

func (m *mockAntigravityProxyRepo) ListAccountSummariesByProxyID(context.Context, int64) ([]ProxyAccountSummary, error) {
	panic("not impl")
}

func TestResolveDefaultTierID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		loadRaw map[string]any
		want    string
	}{
		{
			name:    "nil loadRaw",
			loadRaw: nil,
			want:    "",
		},
		{
			name: "missing allowedTiers",
			loadRaw: map[string]any{
				"paidTier": map[string]any{"id": "g1-pro-tier"},
			},
			want: "",
		},
		{
			name:    "empty allowedTiers",
			loadRaw: map[string]any{"allowedTiers": []any{}},
			want:    "",
		},
		{
			name: "tier missing id field",
			loadRaw: map[string]any{
				"allowedTiers": []any{
					map[string]any{"isDefault": true},
				},
			},
			want: "",
		},
		{
			name: "allowedTiers but no default",
			loadRaw: map[string]any{
				"allowedTiers": []any{
					map[string]any{"id": "free-tier", "isDefault": false},
					map[string]any{"id": "standard-tier", "isDefault": false},
				},
			},
			want: "",
		},
		{
			name: "default tier found",
			loadRaw: map[string]any{
				"allowedTiers": []any{
					map[string]any{"id": "free-tier", "isDefault": true},
					map[string]any{"id": "standard-tier", "isDefault": false},
				},
			},
			want: "free-tier",
		},
		{
			name: "default tier id with spaces",
			loadRaw: map[string]any{
				"allowedTiers": []any{
					map[string]any{"id": "  standard-tier  ", "isDefault": true},
				},
			},
			want: "standard-tier",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := resolveDefaultTierID(tc.loadRaw)
			if got != tc.want {
				t.Fatalf("resolveDefaultTierID() = %q, want %q", got, tc.want)
			}
		})
	}
}
