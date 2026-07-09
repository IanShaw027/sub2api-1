package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// rateLimitAccountRepoStub is shared by Gemini and rate-limit related tests.
// Keep it independent from build-tagged helpers so default test builds can use it.
type rateLimitAccountRepoStub struct {
	setErrorCalls          int
	tempCalls              int
	rateLimitedCalls       int
	updateCredentialsCalls int
	lastCredentials        map[string]any
	lastErrorMsg           string
	lastTempReason         string
	lastTempID             int64
	lastRateLimitedAt      time.Time
	accountsByID           map[int64]*Account
	tempErr                error
}

func (r *rateLimitAccountRepoStub) Create(context.Context, *Account) error { return nil }
func (r *rateLimitAccountRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	if r.accountsByID == nil {
		return nil, nil
	}
	return r.accountsByID[id], nil
}
func (r *rateLimitAccountRepoStub) GetByIDs(context.Context, []int64) ([]*Account, error) {
	return nil, nil
}
func (r *rateLimitAccountRepoStub) ExistsByID(context.Context, int64) (bool, error) {
	return false, nil
}
func (r *rateLimitAccountRepoStub) GetByCRSAccountID(context.Context, string) (*Account, error) {
	return nil, nil
}
func (r *rateLimitAccountRepoStub) FindByExtraField(context.Context, string, any) ([]Account, error) {
	return nil, nil
}
func (r *rateLimitAccountRepoStub) ListCRSAccountIDs(context.Context) (map[string]int64, error) {
	return nil, nil
}
func (r *rateLimitAccountRepoStub) Update(context.Context, *Account) error { return nil }
func (r *rateLimitAccountRepoStub) Delete(context.Context, int64) error    { return nil }
func (r *rateLimitAccountRepoStub) List(context.Context, pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *rateLimitAccountRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, string, int64, string) ([]Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *rateLimitAccountRepoStub) ListByGroup(context.Context, int64) ([]Account, error) {
	return nil, nil
}
func (r *rateLimitAccountRepoStub) ListActive(context.Context) ([]Account, error) { return nil, nil }
func (r *rateLimitAccountRepoStub) ListByPlatform(context.Context, string) ([]Account, error) {
	return nil, nil
}
func (r *rateLimitAccountRepoStub) UpdateLastUsed(context.Context, int64) error { return nil }
func (r *rateLimitAccountRepoStub) BatchUpdateLastUsed(context.Context, map[int64]time.Time) error {
	return nil
}
func (r *rateLimitAccountRepoStub) SetError(ctx context.Context, id int64, errorMsg string) error {
	r.setErrorCalls++
	r.lastErrorMsg = errorMsg
	return nil
}
func (r *rateLimitAccountRepoStub) ClearError(context.Context, int64) error           { return nil }
func (r *rateLimitAccountRepoStub) SetSchedulable(context.Context, int64, bool) error { return nil }
func (r *rateLimitAccountRepoStub) AutoPauseExpiredAccounts(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (r *rateLimitAccountRepoStub) BindGroups(context.Context, int64, []int64) error { return nil }
func (r *rateLimitAccountRepoStub) ListSchedulable(context.Context) ([]Account, error) {
	return nil, nil
}
func (r *rateLimitAccountRepoStub) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	return nil, nil
}
func (r *rateLimitAccountRepoStub) ListSchedulableByPlatform(context.Context, string) ([]Account, error) {
	return nil, nil
}
func (r *rateLimitAccountRepoStub) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]Account, error) {
	return nil, nil
}
func (r *rateLimitAccountRepoStub) ListSchedulableByPlatforms(context.Context, []string) ([]Account, error) {
	return nil, nil
}
func (r *rateLimitAccountRepoStub) ListSchedulableByGroupIDAndPlatforms(context.Context, int64, []string) ([]Account, error) {
	return nil, nil
}
func (r *rateLimitAccountRepoStub) ListSchedulableUngroupedByPlatform(context.Context, string) ([]Account, error) {
	return nil, nil
}
func (r *rateLimitAccountRepoStub) ListSchedulableUngroupedByPlatforms(context.Context, []string) ([]Account, error) {
	return nil, nil
}
func (r *rateLimitAccountRepoStub) SetRateLimited(ctx context.Context, id int64, resetAt time.Time) error {
	r.rateLimitedCalls++
	r.lastRateLimitedAt = resetAt
	return nil
}
func (r *rateLimitAccountRepoStub) SetModelRateLimit(context.Context, int64, string, time.Time, ...string) error {
	return nil
}
func (r *rateLimitAccountRepoStub) SetOverloaded(context.Context, int64, time.Time) error { return nil }
func (r *rateLimitAccountRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	r.tempCalls++
	r.lastTempID = id
	r.lastTempReason = reason
	return r.tempErr
}
func (r *rateLimitAccountRepoStub) ClearTempUnschedulable(context.Context, int64) error { return nil }
func (r *rateLimitAccountRepoStub) ClearRateLimit(context.Context, int64) error         { return nil }
func (r *rateLimitAccountRepoStub) ClearAntigravityQuotaScopes(context.Context, int64) error {
	return nil
}
func (r *rateLimitAccountRepoStub) ClearModelRateLimits(context.Context, int64) error { return nil }
func (r *rateLimitAccountRepoStub) UpdateSessionWindow(context.Context, int64, *time.Time, *time.Time, string) error {
	return nil
}

func (r *rateLimitAccountRepoStub) UpdateSessionWindowEnd(context.Context, int64, time.Time) error {
	return nil
}

func (r *rateLimitAccountRepoStub) UpdateExtra(context.Context, int64, map[string]any) error {
	return nil
}
func (r *rateLimitAccountRepoStub) BulkUpdate(context.Context, []int64, AccountBulkUpdate) (int64, error) {
	return 0, nil
}
func (r *rateLimitAccountRepoStub) IncrementQuotaUsed(context.Context, int64, float64) error {
	return nil
}
func (r *rateLimitAccountRepoStub) ResetQuotaUsed(context.Context, int64) error { return nil }

func (r *rateLimitAccountRepoStub) RevertProxyFallback(context.Context, int64) error {
	return nil
}

func (r *rateLimitAccountRepoStub) UpdateCredentials(ctx context.Context, id int64, credentials map[string]any) error {
	r.updateCredentialsCalls++
	r.lastCredentials = cloneCredentials(credentials)
	return nil
}

func (r *rateLimitAccountRepoStub) ListOAuthRefreshCandidates(context.Context) ([]Account, error) {
	return nil, nil
}
func (r *rateLimitAccountRepoStub) ListShadowsByParent(context.Context, int64) ([]*Account, error) {
	return nil, nil
}

var _ AccountRepository = (*rateLimitAccountRepoStub)(nil)

func (s *rateLimitAccountRepoStub) ListAllWithFilters(ctx context.Context, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, error) {
	return nil, nil
}
