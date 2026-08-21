package service

import (
	"context"
	"database/sql"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestNormalizePublicIP(t *testing.T) {
	if got := normalizePublicIP("203.0.113.8"); got != "203.0.113.8" {
		t.Fatalf("unexpected public IP: %q", got)
	}
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "not-an-ip", "::1"} {
		if got := normalizePublicIP(raw); got != "" {
			t.Fatalf("normalizePublicIP(%q) = %q, want empty", raw, got)
		}
	}
}

func TestNormalizePublicIPSkipsCGNAT(t *testing.T) {
	if got := normalizePublicIP("100.64.0.1"); got != "" {
		t.Fatalf("expected CGNAT address to be ignored, got %q", got)
	}
	if got := normalizePublicIP("203.0.113.8"); got != "203.0.113.8" {
		t.Fatalf("expected public address to be kept, got %q", got)
	}
}

func TestNormalizeIPSecurityConfig(t *testing.T) {
	cfg := normalizeIPSecurityConfig(IPSecurityConfig{WindowMinutes: 0, AccountThreshold: 1})
	if cfg.WindowMinutes != 1 || cfg.AccountThreshold != 2 {
		t.Fatalf("unexpected normalized config: %+v", cfg)
	}
}

func TestIPSecurityObserveKnownIPUsesRedisHotPath(t *testing.T) {
	svc, repo := newTestIPSecurityService(t, true)
	repo.ips[1] = []string{"203.0.113.8"}
	activity := IPSecurityActivity{IPAddress: "203.0.113.8", UserID: 1, Source: IPSecuritySourceWeb}

	svc.Observe(context.Background(), activity)
	svc.Observe(context.Background(), activity)

	if repo.getUserIPCalls != 1 {
		t.Fatalf("GetUserIPs calls = %d, want 1 cache hydration", repo.getUserIPCalls)
	}
	if repo.addUserIPCalls != 0 {
		t.Fatalf("AddUserIPActivity calls = %d, want 0", repo.addUserIPCalls)
	}
}

func TestIPSecuritySecondWindowBansTwoAccounts(t *testing.T) {
	svc, repo := newTestIPSecurityService(t, true)
	svc.settings.(ipSecuritySettingsStub).values[SettingKeyIPMultiAccountBanWindow2Minutes] = "1440"
	svc.settings.(ipSecuritySettingsStub).values[SettingKeyIPMultiAccountBanThreshold2] = "2"
	svc.InvalidateConfig()

	svc.Observe(context.Background(), IPSecurityActivity{
		IPAddress: "203.0.113.60", UserID: 1, Source: IPSecuritySourceAPIKey,
	})
	if repo.createBanCalls != 0 {
		t.Fatalf("first account must not trip either window, got %d bans", repo.createBanCalls)
	}
	svc.Observe(context.Background(), IPSecurityActivity{
		IPAddress: "203.0.113.60", UserID: 2, Source: IPSecuritySourceAPIKey,
	})
	if repo.createBanCalls != 1 {
		t.Fatalf("second account in 24h window must create a ban, got %d", repo.createBanCalls)
	}
	if !svc.IsBlocked(context.Background(), "203.0.113.60") {
		t.Fatal("expected IP blocked by second-layer window")
	}
	if got := repo.bans["203.0.113.60"].Reason; got != "long-window multi-account activity" {
		t.Fatalf("second-window ban reason = %q", got)
	}
}

func TestIPSecurityFourthNewAccountCreatesPermanentBan(t *testing.T) {
	svc, repo := newTestIPSecurityService(t, true)
	for userID := int64(1); userID <= 4; userID++ {
		svc.Observe(context.Background(), IPSecurityActivity{
			IPAddress: "203.0.113.9", UserID: userID, Source: IPSecuritySourceAPIKey,
			APIKeyID: userID * 10, Method: "POST", Path: "/v1/responses",
		})
	}

	if repo.addUserIPCalls != 4 {
		t.Fatalf("new IP writes = %d, want 4", repo.addUserIPCalls)
	}
	if repo.createBanCalls != 1 {
		t.Fatalf("CreateBan calls = %d, want 1", repo.createBanCalls)
	}
	if !svc.IsBlocked(context.Background(), "203.0.113.9") {
		t.Fatal("expected IP to remain blocked until manual release")
	}
	if got := repo.bans["203.0.113.9"].Reason; got != "short-window multi-account activity" {
		t.Fatalf("short-window ban reason = %q", got)
	}
}

func TestIPSecurityWhitelistPreventsAutomaticReban(t *testing.T) {
	svc, repo := newTestIPSecurityService(t, true)
	repo.bans["203.0.113.10"] = IPSecurityBan{ID: 1, IPAddress: "203.0.113.10", Status: "active"}
	if err := svc.WarmCache(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !svc.IsBlocked(context.Background(), "203.0.113.10") {
		t.Fatal("expected initial active ban")
	}
	if err := svc.WhitelistBan(context.Background(), 1, 99); err != nil {
		t.Fatal(err)
	}
	if svc.IsBlocked(context.Background(), "203.0.113.10") {
		t.Fatal("whitelisted IP must not remain blocked")
	}

	for userID := int64(1); userID <= 8; userID++ {
		svc.Observe(context.Background(), IPSecurityActivity{
			IPAddress: "203.0.113.10", UserID: userID, Source: IPSecuritySourceWeb,
		})
	}
	if repo.createBanCalls != 0 {
		t.Fatalf("CreateBan calls after whitelist = %d, want 0", repo.createBanCalls)
	}
}

func TestIPSecurityActiveSnapshotSurvivesRedisFailure(t *testing.T) {
	svc, repo := newTestIPSecurityService(t, false)
	repo.bans["203.0.113.11"] = IPSecurityBan{ID: 11, IPAddress: "203.0.113.11", Status: "active"}
	if err := svc.WarmCache(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = svc.rdb.Close()
	if !svc.IsBlocked(context.Background(), "203.0.113.11") {
		t.Fatal("active ban must remain enforced when Redis is unavailable")
	}
}

func TestIPSecurityIsBlockedReadsSharedRedisBanAcrossInstances(t *testing.T) {
	// Two service instances share Redis + repo, but each has its own process-local maps.
	mr := miniredis.RunT(t)
	rdbA := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rdbB := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdbA.Close()
		_ = rdbB.Close()
	})
	repo := &ipSecurityRepoStub{ips: map[int64][]string{}, bans: map[string]IPSecurityBan{}, saturated: map[int64]bool{}}
	settings := ipSecuritySettingsStub{values: map[string]string{
		SettingKeyIPMultiAccountBanEnabled:       "true",
		SettingKeyIPMultiAccountBanWindowMinutes: "10",
		SettingKeyIPMultiAccountBanThreshold:     "4",
	}}
	svcA := NewIPSecurityService(repo, settings, rdbA)
	svcB := NewIPSecurityService(repo, settings, rdbB)

	// Warm both so stateLoaded=true with empty local ban maps (the multi-instance bug path).
	if err := svcA.WarmCache(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := svcB.WarmCache(context.Background()); err != nil {
		t.Fatal(err)
	}

	for userID := int64(1); userID <= 4; userID++ {
		svcA.Observe(context.Background(), IPSecurityActivity{
			IPAddress: "203.0.113.50", UserID: userID, Source: IPSecuritySourceAPIKey,
		})
	}
	if !svcA.IsBlocked(context.Background(), "203.0.113.50") {
		t.Fatal("instance A should block after creating ban")
	}
	// Instance B never saw CreateBan locally; must still block via Redis.
	if !svcB.IsBlocked(context.Background(), "203.0.113.50") {
		t.Fatal("instance B must observe shared Redis ban without restart")
	}
	// Repeated calls must continue checking shared authoritative state.
	if !svcB.IsBlocked(context.Background(), "203.0.113.50") {
		t.Fatal("instance B should continue enforcing shared ban")
	}
}

func TestIPSecurityRedisFlushFallsBackToDatabaseBan(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	repo := &ipSecurityRepoStub{ips: map[int64][]string{}, bans: map[string]IPSecurityBan{}, saturated: map[int64]bool{}}
	svc := NewIPSecurityService(repo, ipSecuritySettingsStub{values: map[string]string{}}, rdb)

	if err := svc.WarmCache(context.Background()); err != nil {
		t.Fatal(err)
	}
	repo.bans["203.0.113.51"] = IPSecurityBan{ID: 51, IPAddress: "203.0.113.51", Status: "active"}
	mr.FlushAll()

	if !svc.IsBlocked(context.Background(), "203.0.113.51") {
		t.Fatal("missing Redis loaded marker must force DB fallback for active bans")
	}
}

func TestIPSecuritySharedStatusOverridesStaleLocalState(t *testing.T) {
	mr := miniredis.RunT(t)
	rdbA := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rdbB := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = rdbA.Close()
		_ = rdbB.Close()
	})
	repo := &ipSecurityRepoStub{
		ips:       map[int64][]string{},
		bans:      map[string]IPSecurityBan{"203.0.113.52": {ID: 52, IPAddress: "203.0.113.52", Status: "active"}},
		saturated: map[int64]bool{},
	}
	settings := ipSecuritySettingsStub{values: map[string]string{}}
	svcA := NewIPSecurityService(repo, settings, rdbA)
	svcB := NewIPSecurityService(repo, settings, rdbB)
	if err := svcA.WarmCache(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := svcB.WarmCache(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !svcB.IsBlocked(context.Background(), "203.0.113.52") {
		t.Fatal("expected initial active ban")
	}

	if err := svcA.WhitelistBan(context.Background(), 52, 99); err != nil {
		t.Fatal(err)
	}
	if svcB.IsBlocked(context.Background(), "203.0.113.52") {
		t.Fatal("remote whitelist change must override stale local ban")
	}

	if err := svcA.RemoveWhitelist(context.Background(), 52, 99); err != nil {
		t.Fatal(err)
	}
	if svcB.isWhitelisted(context.Background(), "203.0.113.52") {
		t.Fatal("remote whitelist removal must override stale local whitelist")
	}
}

func TestIPSecurityLearningPeriodOnlyRecordsHistory(t *testing.T) {
	svc, repo := newTestIPSecurityService(t, true)
	svc.settings.(ipSecuritySettingsStub).values[SettingKeyIPMultiAccountBanLearningUntil] = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	for userID := int64(1); userID <= 4; userID++ {
		svc.Observe(context.Background(), IPSecurityActivity{IPAddress: "203.0.113.12", UserID: userID, Source: IPSecuritySourceWeb})
	}
	if repo.createBanCalls != 0 {
		t.Fatalf("learning period must not create a ban, got %d", repo.createBanCalls)
	}
	if repo.addUserIPCalls != 4 {
		t.Fatalf("learning period must still record history, got %d writes", repo.addUserIPCalls)
	}
}

func newTestIPSecurityService(t *testing.T, enabled bool) (*IPSecurityService, *ipSecurityRepoStub) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	repo := &ipSecurityRepoStub{ips: map[int64][]string{}, bans: map[string]IPSecurityBan{}, saturated: map[int64]bool{}}
	settings := ipSecuritySettingsStub{values: map[string]string{
		SettingKeyIPMultiAccountBanEnabled:       strconv.FormatBool(enabled),
		SettingKeyIPMultiAccountBanWindowMinutes: "10",
		SettingKeyIPMultiAccountBanThreshold:     "4",
	}}
	return NewIPSecurityService(repo, settings, rdb), repo
}

type ipSecuritySettingsStub struct{ values map[string]string }

func (s ipSecuritySettingsStub) GetValue(_ context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", sql.ErrNoRows
	}
	return value, nil
}
func (s ipSecuritySettingsStub) Set(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

type ipSecurityRepoStub struct {
	mu             sync.Mutex
	ips            map[int64][]string
	bans           map[string]IPSecurityBan
	getUserIPCalls int
	addUserIPCalls int
	createBanCalls int
	saturated      map[int64]bool
}

func (r *ipSecurityRepoStub) GetUserIPs(_ context.Context, userID int64) ([]string, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.getUserIPCalls++
	return append([]string(nil), r.ips[userID]...), r.saturated[userID], nil
}

func (r *ipSecurityRepoStub) AddUserIPActivity(_ context.Context, activity IPSecurityActivity, _ time.Time) (bool, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.addUserIPCalls++
	if containsIP(r.ips[activity.UserID], activity.IPAddress) {
		return false, r.saturated[activity.UserID], nil
	}
	if r.saturated[activity.UserID] {
		return false, true, nil
	}
	r.ips[activity.UserID] = append(r.ips[activity.UserID], activity.IPAddress)
	if len(r.ips[activity.UserID]) >= maxUserIPHistory {
		if r.saturated == nil {
			r.saturated = make(map[int64]bool)
		}
		r.saturated[activity.UserID] = true
	}
	return true, r.saturated[activity.UserID], nil
}

func (r *ipSecurityRepoStub) CreateBan(_ context.Context, ban *IPSecurityBan) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.bans[ban.IPAddress]; ok && existing.Status == "whitelisted" {
		return false, nil
	}
	r.createBanCalls++
	ban.ID = int64(len(r.bans) + 1)
	r.bans[ban.IPAddress] = *ban
	return true, nil
}

func (r *ipSecurityRepoStub) IsIPStatus(_ context.Context, ip, status string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ban, ok := r.bans[ip]
	return ok && ban.Status == status, nil
}

func (r *ipSecurityRepoStub) LoadIPStatuses(_ context.Context) (active, whitelisted []string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for ip, ban := range r.bans {
		switch ban.Status {
		case "active":
			active = append(active, ip)
		case "whitelisted":
			whitelisted = append(whitelisted, ip)
		}
	}
	return active, whitelisted, nil
}

func (r *ipSecurityRepoStub) ListBans(context.Context, string, int, int) ([]IPSecurityBan, int64, error) {
	return nil, 0, nil
}
func (r *ipSecurityRepoStub) GetBan(context.Context, int64) (*IPSecurityBan, error) {
	return nil, sql.ErrNoRows
}
func (r *ipSecurityRepoStub) ListActivity(context.Context, string, time.Time, time.Time) ([]IPSecurityActivityDetail, error) {
	return nil, nil
}

func (r *ipSecurityRepoStub) WhitelistBan(_ context.Context, id, _ int64, now time.Time) (*IPSecurityBan, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for ip, ban := range r.bans {
		if ban.ID == id && ban.Status == "active" {
			ban.Status = "whitelisted"
			ban.ReleasedAt = &now
			r.bans[ip] = ban
			return &ban, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *ipSecurityRepoStub) RemoveWhitelist(_ context.Context, id, _ int64, now time.Time) (*IPSecurityBan, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for ip, ban := range r.bans {
		if ban.ID == id && ban.Status == "whitelisted" {
			ban.Status = "released"
			ban.ReleasedAt = &now
			r.bans[ip] = ban
			return &ban, nil
		}
	}
	return nil, sql.ErrNoRows
}
