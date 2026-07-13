//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

type rateLimit429AccountRepoStub struct {
	mockAccountRepoForGemini
	rateLimitCalls     int
	lastRateLimitID    int64
	lastRateLimitReset time.Time
}

func (r *rateLimit429AccountRepoStub) SetRateLimited(_ context.Context, id int64, resetAt time.Time) error {
	r.rateLimitCalls++
	r.lastRateLimitID = id
	r.lastRateLimitReset = resetAt
	return nil
}

func TestGetRateLimit429CooldownSettings_DefaultsWhenNotSet(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetRateLimit429CooldownSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.Equal(t, 5, settings.CooldownSeconds)
}

func TestGetRateLimit429CooldownSettings_ReadsFromDB(t *testing.T) {
	repo := newMockSettingRepo()
	data, _ := json.Marshal(RateLimit429CooldownSettings{Enabled: false, CooldownSeconds: 12})
	repo.data[SettingKeyRateLimit429CooldownSettings] = string(data)
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetRateLimit429CooldownSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Equal(t, 12, settings.CooldownSeconds)
}

func TestSetRateLimit429CooldownSettings_EnabledRejectsOutOfRange(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})

	for _, seconds := range []int{0, -1, 7201, 99999} {
		err := svc.SetRateLimit429CooldownSettings(context.Background(), &RateLimit429CooldownSettings{
			Enabled: true, CooldownSeconds: seconds,
		})
		require.Error(t, err, "should reject enabled=true + cooldown_seconds=%d", seconds)
		require.Contains(t, err.Error(), "cooldown_seconds must be between 1-7200")
	}
}

func TestHandle429_FallbackUsesDBSeconds(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	settingRepo := newMockSettingRepo()
	data, _ := json.Marshal(RateLimit429CooldownSettings{Enabled: true, CooldownSeconds: 12})
	settingRepo.data[SettingKeyRateLimit429CooldownSettings] = string(data)

	settingSvc := NewSettingService(settingRepo, &config.Config{})
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	svc.SetSettingService(settingSvc)

	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	before := time.Now()
	svc.handle429(context.Background(), account, http.Header{}, []byte(`{"error":{"type":"rate_limit_error","message":"slow down"}}`))
	after := time.Now()

	require.Equal(t, 1, accountRepo.rateLimitCalls)
	require.Equal(t, int64(42), accountRepo.lastRateLimitID)
	require.True(t, !accountRepo.lastRateLimitReset.Before(before.Add(12*time.Second)) && !accountRepo.lastRateLimitReset.After(after.Add(12*time.Second)))
}

func TestHandle429_GrokUsesResetHeaderWindow(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)

	account := &Account{ID: 77, Platform: PlatformGrok, Type: AccountTypeOAuth}
	headers := http.Header{"X-Ratelimit-Remaining-Requests": []string{"0"}, "Retry-After": []string{"45"}}
	before := time.Now()
	svc.handle429(context.Background(), account, headers, nil)

	require.Equal(t, 1, accountRepo.rateLimitCalls, "Grok 429 with reset header uses SetRateLimited")
	require.Equal(t, int64(77), accountRepo.lastRateLimitID)
	require.True(t, accountRepo.lastRateLimitReset.After(before.Add(44*time.Second)))
	require.True(t, accountRepo.lastRateLimitReset.Before(before.Add(46*time.Second)))
}

func TestHandle429_GrokCapsOversizedRetryAfter(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)

	account := &Account{ID: 78, Platform: PlatformGrok, Type: AccountTypeOAuth}
	before := time.Now()
	svc.handle429(context.Background(), account, http.Header{"Retry-After": []string{"86400"}}, nil)

	require.Equal(t, 1, accountRepo.rateLimitCalls)
	require.True(t, accountRepo.lastRateLimitReset.Before(before.Add(grokMaxUpstreamCooldown+time.Second)))
	require.True(t, accountRepo.lastRateLimitReset.After(before.Add(grokMaxUpstreamCooldown-time.Second)))
}

func TestHandle429_GrokSpendingLimitUsesWeeklyBillingReset(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	resetAt := time.Now().Add(5 * 24 * time.Hour).UTC().Truncate(time.Second)
	account := &Account{
		ID:       79,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			grokBillingSnapshotExtraKey: &xai.BillingSnapshot{
				Credits: &xai.CreditsBillingConfig{
					CreditUsagePercent: 100,
					CurrentPeriod: &xai.UsagePeriod{
						End: resetAt.Format(time.RFC3339),
					},
				},
			},
		},
	}

	svc.handle429(
		context.Background(),
		account,
		http.Header{"Retry-After": []string{"45"}},
		[]byte(`{"code":"personal-team-blocked:spending-limit"}`),
	)

	require.Equal(t, 1, accountRepo.rateLimitCalls)
	require.Equal(t, resetAt, accountRepo.lastRateLimitReset)
}

func TestHandle429_GrokSpendingLimitUsesMonthlyBillingReset(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	resetAt := time.Now().Add(18 * 24 * time.Hour).UTC().Truncate(time.Second)
	account := &Account{
		ID:       81,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			grokBillingSnapshotExtraKey: &xai.BillingSnapshot{
				Credits: &xai.CreditsBillingConfig{CreditUsagePercent: 40},
				Monthly: &xai.MonthlyBillingConfig{
					MonthlyLimit:     &xai.MoneyVal{Val: 15000},
					Used:             &xai.MoneyVal{Val: 15000},
					BillingPeriodEnd: resetAt.Format(time.RFC3339),
				},
			},
		},
	}

	svc.handle429(
		context.Background(),
		account,
		http.Header{"Retry-After": []string{"45"}},
		[]byte(`{"code":"personal-team-blocked:spending-limit"}`),
	)

	require.Equal(t, 1, accountRepo.rateLimitCalls)
	require.Equal(t, resetAt, accountRepo.lastRateLimitReset)
}

func TestHandle429_GrokSpendingLimitUsesLaterOfWeeklyAndMonthlyReset(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	weeklyEnd := time.Now().Add(3 * 24 * time.Hour).UTC().Truncate(time.Second)
	monthlyEnd := time.Now().Add(18 * 24 * time.Hour).UTC().Truncate(time.Second)
	account := &Account{
		ID:       82,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			grokBillingSnapshotExtraKey: &xai.BillingSnapshot{
				Credits: &xai.CreditsBillingConfig{
					CreditUsagePercent: 100,
					CurrentPeriod:      &xai.UsagePeriod{End: weeklyEnd.Format(time.RFC3339)},
				},
				Monthly: &xai.MonthlyBillingConfig{
					MonthlyLimit:     &xai.MoneyVal{Val: 15000},
					Used:             &xai.MoneyVal{Val: 16000},
					BillingPeriodEnd: monthlyEnd.Format(time.RFC3339),
				},
			},
		},
	}

	svc.handle429(
		context.Background(),
		account,
		nil,
		[]byte(`{"code":"personal-team-blocked:spending-limit"}`),
	)

	require.Equal(t, 1, accountRepo.rateLimitCalls)
	require.Equal(t, monthlyEnd, accountRepo.lastRateLimitReset)
}

func TestHandle429_GrokSpendingLimitIgnoresUnexhaustedWeeklySnapshot(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	account := &Account{
		ID:       80,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			grokBillingSnapshotExtraKey: &xai.BillingSnapshot{
				Credits: &xai.CreditsBillingConfig{
					CreditUsagePercent: 99,
					CurrentPeriod: &xai.UsagePeriod{
						End: time.Now().Add(5 * 24 * time.Hour).UTC().Format(time.RFC3339),
					},
				},
			},
		},
	}
	before := time.Now()

	svc.handle429(
		context.Background(),
		account,
		http.Header{"Retry-After": []string{"45"}},
		[]byte(`{"code":"personal-team-blocked:spending-limit"}`),
	)

	require.Equal(t, 1, accountRepo.rateLimitCalls)
	require.True(t, accountRepo.lastRateLimitReset.After(before.Add(44*time.Second)))
	require.True(t, accountRepo.lastRateLimitReset.Before(before.Add(46*time.Second)))
}

func TestHandle429_FallbackDisabledSkipsLocalMark(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	settingRepo := newMockSettingRepo()
	data, _ := json.Marshal(RateLimit429CooldownSettings{Enabled: false, CooldownSeconds: 12})
	settingRepo.data[SettingKeyRateLimit429CooldownSettings] = string(data)

	settingSvc := NewSettingService(settingRepo, &config.Config{})
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	svc.SetSettingService(settingSvc)

	account := &Account{ID: 43, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	svc.handle429(context.Background(), account, http.Header{}, []byte(`{"error":{"type":"rate_limit_error","message":"slow down"}}`))

	require.Zero(t, accountRepo.rateLimitCalls)
}

// Anthropic 无 reset 头的 429（如 Extra usage required）也应走兜底冷却，
// 否则账号永不冷却，调度器会让每个请求反复撞同一批 429 账号（旋转木马）。
func TestHandle429_AnthropicNoResetTimeUsesFallbackCooldown(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	settingRepo := newMockSettingRepo()
	data, _ := json.Marshal(RateLimit429CooldownSettings{Enabled: true, CooldownSeconds: 12})
	settingRepo.data[SettingKeyRateLimit429CooldownSettings] = string(data)

	settingSvc := NewSettingService(settingRepo, &config.Config{})
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	svc.SetSettingService(settingSvc)

	account := &Account{ID: 45, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	before := time.Now()
	svc.handle429(context.Background(), account, http.Header{}, []byte(`{"error":{"type":"rate_limit_error","message":"Extra usage required"}}`))
	after := time.Now()

	require.Equal(t, 1, accountRepo.rateLimitCalls)
	require.Equal(t, int64(45), accountRepo.lastRateLimitID)
	require.True(t, !accountRepo.lastRateLimitReset.Before(before.Add(12*time.Second)) && !accountRepo.lastRateLimitReset.After(after.Add(12*time.Second)))
}

// 管理端关闭兜底冷却时，Anthropic 无 reset 头的 429 保持旧行为：不标记账号。
func TestHandle429_AnthropicNoResetTimeFallbackDisabledSkipsMark(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	settingRepo := newMockSettingRepo()
	data, _ := json.Marshal(RateLimit429CooldownSettings{Enabled: false, CooldownSeconds: 12})
	settingRepo.data[SettingKeyRateLimit429CooldownSettings] = string(data)

	settingSvc := NewSettingService(settingRepo, &config.Config{})
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	svc.SetSettingService(settingSvc)

	account := &Account{ID: 46, Platform: PlatformAnthropic, Type: AccountTypeOAuth}
	svc.handle429(context.Background(), account, http.Header{}, []byte(`{"error":{"type":"rate_limit_error","message":"Extra usage required"}}`))

	require.Zero(t, accountRepo.rateLimitCalls)
}

func TestHandle429_FallbackUsesDefaultSecondsWhenSettingServiceMissing(t *testing.T) {
	accountRepo := &rateLimit429AccountRepoStub{}
	cfg := &config.Config{}
	svc := NewRateLimitService(accountRepo, nil, cfg, nil, nil)

	account := &Account{ID: 44, Platform: PlatformGemini, Type: AccountTypeAPIKey}
	before := time.Now()
	svc.handle429(context.Background(), account, http.Header{}, []byte(`{"error":{"message":"slow down"}}`))
	after := time.Now()

	require.Equal(t, 1, accountRepo.rateLimitCalls)
	require.Equal(t, int64(44), accountRepo.lastRateLimitID)
	require.True(t, !accountRepo.lastRateLimitReset.Before(before.Add(5*time.Second)) && !accountRepo.lastRateLimitReset.After(after.Add(5*time.Second)))
}
