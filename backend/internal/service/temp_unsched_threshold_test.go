//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// fakeTempUnschedCounter 是 TempUnschedCounterCache 的内存实现，用于窗口阈值测试。
type fakeTempUnschedCounter struct {
	counts     map[string]int64
	resetCalls map[string]int
	incErr     error
}

func newFakeTempUnschedCounter() *fakeTempUnschedCounter {
	return &fakeTempUnschedCounter{
		counts:     make(map[string]int64),
		resetCalls: make(map[string]int),
	}
}

func (f *fakeTempUnschedCounter) key(accountID int64, ruleFingerprint string) string {
	return string(rune(accountID)) + ":" + ruleFingerprint
}

func (f *fakeTempUnschedCounter) IncrementTempUnschedCount(_ context.Context, accountID int64, ruleFingerprint string, _ int) (int64, error) {
	if f.incErr != nil {
		return 0, f.incErr
	}
	k := f.key(accountID, ruleFingerprint)
	f.counts[k]++
	return f.counts[k], nil
}

func (f *fakeTempUnschedCounter) IncrementTempUnschedThreshold(_ context.Context, accountID int64, ruleFingerprint string, _ int, thresholdCount int) (int64, bool, error) {
	if f.incErr != nil {
		return 0, false, f.incErr
	}
	if thresholdCount < 1 {
		thresholdCount = 1
	}
	k := f.key(accountID, ruleFingerprint)
	f.counts[k]++
	count := f.counts[k]
	if count >= int64(thresholdCount) {
		f.resetCalls[k]++
		f.counts[k] = 0
		return count, true, nil
	}
	return count, false, nil
}

func (f *fakeTempUnschedCounter) ResetTempUnschedCount(_ context.Context, accountID int64, ruleFingerprint string) error {
	k := f.key(accountID, ruleFingerprint)
	f.resetCalls[k]++
	f.counts[k] = 0
	return nil
}

func tempUnschedTestAccount() *Account {
	return &Account{
		ID:          7,
		Status:      StatusActive,
		Schedulable: true,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{
			"temp_unschedulable_enabled": true,
			"temp_unschedulable_rules": []any{
				map[string]any{
					"error_code":       float64(502),
					"keywords":         []any{"upstream request failed"},
					"duration_minutes": float64(10),
				},
				map[string]any{
					// 第二条规则：keywords 留空，仅凭错误码匹配（用于 524 等空 body 码）
					"error_code":       float64(524),
					"keywords":         []any{},
					"duration_minutes": float64(10),
				},
			},
		},
	}
}

func newTempUnschedThresholdService(t *testing.T, repo AccountRepository, counter TempUnschedCounterCache, settings *TempUnschedThresholdSettings) *RateLimitService {
	t.Helper()
	settingRepo := newMockSettingRepo()
	if settings != nil {
		data, _ := json.Marshal(settings)
		settingRepo.data[SettingKeyTempUnschedThresholdSettings] = string(data)
	}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc.SetSettingService(NewSettingService(settingRepo, &config.Config{}))
	if counter != nil {
		svc.SetTempUnschedCounterCache(counter)
	}
	return svc
}

// TestTryTempUnschedulable_ThresholdWindow 验证启用窗口阈值后，需窗口内命中 N 次才触发。
func TestTryTempUnschedulable_ThresholdWindow(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := newFakeTempUnschedCounter()
	svc := newTempUnschedThresholdService(t, repo, counter, &TempUnschedThresholdSettings{
		Enabled: true, ThresholdCount: 3, ThresholdWindowMinutes: 1,
	})

	account := tempUnschedTestAccount()
	body := []byte(`{"error":{"message":"Upstream request failed"}}`)

	// 前两次命中不应触发
	require.False(t, svc.tryTempUnschedulable(context.Background(), account, 502, body))
	require.False(t, svc.tryTempUnschedulable(context.Background(), account, 502, body))
	require.Equal(t, 0, repo.tempCalls, "未达阈值不应写入临时不可调度")

	// 第三次命中触发
	require.True(t, svc.tryTempUnschedulable(context.Background(), account, 502, body))
	require.Equal(t, 1, repo.tempCalls, "达阈值应触发一次")
	require.Equal(t, 1, counter.resetCalls[counter.key(account.ID, "status=502;keywords=upstream request failed")], "触发后应清零计数")
}

// TestTryTempUnschedulable_ThresholdDisabled_SingleHitTriggers 验证未启用阈值时单次命中即触发（向后兼容）。
func TestTryTempUnschedulable_ThresholdDisabled_SingleHitTriggers(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := newFakeTempUnschedCounter()
	svc := newTempUnschedThresholdService(t, repo, counter, &TempUnschedThresholdSettings{
		Enabled: false, ThresholdCount: 3, ThresholdWindowMinutes: 1,
	})

	account := tempUnschedTestAccount()
	body := []byte(`{"error":{"message":"Upstream request failed"}}`)

	require.True(t, svc.tryTempUnschedulable(context.Background(), account, 502, body))
	require.Equal(t, 1, repo.tempCalls)
	require.Empty(t, counter.counts, "未启用阈值时不应使用计数器")
}

// TestTryTempUnschedulable_NoCounter_SingleHitTriggers 验证计数缓存缺失时退回单次命中即触发。
func TestTryTempUnschedulable_NoCounter_SingleHitTriggers(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	svc := newTempUnschedThresholdService(t, repo, nil, &TempUnschedThresholdSettings{
		Enabled: true, ThresholdCount: 3, ThresholdWindowMinutes: 1,
	})

	account := tempUnschedTestAccount()
	body := []byte(`{"error":{"message":"Upstream request failed"}}`)

	require.True(t, svc.tryTempUnschedulable(context.Background(), account, 502, body))
	require.Equal(t, 1, repo.tempCalls)
}

// TestTryTempUnschedulable_IndependentCountPerRule 验证不同规则独立计数。
func TestTryTempUnschedulable_IndependentCountPerRule(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := newFakeTempUnschedCounter()
	svc := newTempUnschedThresholdService(t, repo, counter, &TempUnschedThresholdSettings{
		Enabled: true, ThresholdCount: 2, ThresholdWindowMinutes: 1,
	})

	account := tempUnschedTestAccount()
	body502 := []byte(`{"error":{"message":"Upstream request failed"}}`)

	// 规则0(502) 命中两次触发；规则1(524) 计数应不受影响
	require.False(t, svc.tryTempUnschedulable(context.Background(), account, 502, body502))
	require.True(t, svc.tryTempUnschedulable(context.Background(), account, 502, body502))
	require.Equal(t, 1, repo.tempCalls)
	require.Equal(t, int64(0), counter.counts[counter.key(account.ID, "status=524;keywords=")], "规则1计数不应被规则0影响")
}

func TestTryTempUnschedulable_ThresholdUsesStableRuleFingerprintWhenRulesReorder(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := newFakeTempUnschedCounter()
	svc := newTempUnschedThresholdService(t, repo, counter, &TempUnschedThresholdSettings{
		Enabled: true, ThresholdCount: 2, ThresholdWindowMinutes: 1,
	})

	account := tempUnschedTestAccount()
	account.Credentials["temp_unschedulable_rules"] = []any{
		map[string]any{
			"error_code":       float64(500),
			"keywords":         []any{"internal"},
			"duration_minutes": float64(10),
		},
		map[string]any{
			"error_code":       float64(502),
			"keywords":         []any{"upstream request failed"},
			"duration_minutes": float64(10),
		},
	}
	body := []byte(`{"error":{"message":"Upstream request failed"}}`)
	require.False(t, svc.tryTempUnschedulable(context.Background(), account, 502, body))

	account.Credentials["temp_unschedulable_rules"] = []any{
		map[string]any{
			"error_code":       float64(502),
			"keywords":         []any{"upstream request failed"},
			"duration_minutes": float64(10),
		},
		map[string]any{
			"error_code":       float64(500),
			"keywords":         []any{"internal"},
			"duration_minutes": float64(10),
		},
	}
	require.True(t, svc.tryTempUnschedulable(context.Background(), account, 502, body), "same rule should reach threshold after reorder")
	require.Equal(t, 1, repo.tempCalls)
}

// TestTryTempUnschedulable_EmptyKeywordsMatchesByCodeOnly 验证 keywords 留空时仅凭错误码匹配（含空 body）。
func TestTryTempUnschedulable_EmptyKeywordsMatchesByCodeOnly(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	svc := newTempUnschedThresholdService(t, repo, nil, &TempUnschedThresholdSettings{Enabled: false})

	account := tempUnschedTestAccount()

	// 524 规则 keywords 留空，body 为空也应命中
	require.True(t, svc.tryTempUnschedulable(context.Background(), account, 524, nil))
	require.Equal(t, 1, repo.tempCalls)

	// 未配置的错误码不应命中
	repo.tempCalls = 0
	require.False(t, svc.tryTempUnschedulable(context.Background(), account, 500, nil))
	require.Equal(t, 0, repo.tempCalls)
}

// ===========================================================================
// SettingService: TempUnschedThresholdSettings
// ===========================================================================

func TestGetTempUnschedThresholdSettings_DefaultsWhenNotSet(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})
	settings, err := svc.GetTempUnschedThresholdSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Equal(t, 3, settings.ThresholdCount)
	require.Equal(t, 1, settings.ThresholdWindowMinutes)
}

func TestGetTempUnschedThresholdSettings_ClampsValues(t *testing.T) {
	repo := newMockSettingRepo()
	data, _ := json.Marshal(TempUnschedThresholdSettings{Enabled: true, ThresholdCount: 9999, ThresholdWindowMinutes: 999})
	repo.data[SettingKeyTempUnschedThresholdSettings] = string(data)
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetTempUnschedThresholdSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1000, settings.ThresholdCount)
	require.Equal(t, 60, settings.ThresholdWindowMinutes)
}

func TestSetTempUnschedThresholdSettings_RoundTrip(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewSettingService(repo, &config.Config{})

	err := svc.SetTempUnschedThresholdSettings(context.Background(), &TempUnschedThresholdSettings{
		Enabled: true, ThresholdCount: 5, ThresholdWindowMinutes: 2,
	})
	require.NoError(t, err)

	settings, err := svc.GetTempUnschedThresholdSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.Equal(t, 5, settings.ThresholdCount)
	require.Equal(t, 2, settings.ThresholdWindowMinutes)
}

func TestSetTempUnschedThresholdSettings_RejectsOutOfRange(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})

	err := svc.SetTempUnschedThresholdSettings(context.Background(), &TempUnschedThresholdSettings{
		Enabled: true, ThresholdCount: 0, ThresholdWindowMinutes: 1,
	})
	require.Error(t, err)

	err = svc.SetTempUnschedThresholdSettings(context.Background(), &TempUnschedThresholdSettings{
		Enabled: true, ThresholdCount: 3, ThresholdWindowMinutes: 999,
	})
	require.Error(t, err)

	// 1001 超过上限应拒绝；1000 为上限应接受
	require.Error(t, svc.SetTempUnschedThresholdSettings(context.Background(), &TempUnschedThresholdSettings{
		Enabled: true, ThresholdCount: 1001, ThresholdWindowMinutes: 1,
	}))
	require.NoError(t, svc.SetTempUnschedThresholdSettings(context.Background(), &TempUnschedThresholdSettings{
		Enabled: true, ThresholdCount: 1000, ThresholdWindowMinutes: 1,
	}))
}

func TestSetTempUnschedThresholdSettings_RejectsNil(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})
	require.Error(t, svc.SetTempUnschedThresholdSettings(context.Background(), nil))
}

// poolModeAccountWithRulesAndCustomCodes 模拟生产中的池模式 openai apikey 账号：
// 开启 pool_mode + custom_error_codes(含502) + temp_unschedulable_rules(502)。
func poolModeAccountWithRulesAndCustomCodes() *Account {
	return &Account{
		ID:          11,
		Status:      StatusActive,
		Schedulable: true,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{
			"pool_mode":                  true,
			"custom_error_codes_enabled": true,
			"custom_error_codes":         []any{float64(500), float64(502), float64(503)},
			"temp_unschedulable_enabled": true,
			"temp_unschedulable_rules": []any{
				map[string]any{
					"error_code":       float64(502),
					"keywords":         []any{"upstream request failed"},
					"duration_minutes": float64(10),
				},
			},
		},
	}
}

// TestHandleUpstreamError_CustomCodeWithTempUnschedRule_NotPermanentlyDisabled
// 验证关键修复：池模式账号开启自定义错误码后，被临时不可调度规则覆盖的错误码
// 在窗口未达阈值/关键词不匹配时不应被 default 分支永久禁用。
func TestHandleUpstreamError_CustomCodeWithTempUnschedRule_NotPermanentlyDisabled(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := newFakeTempUnschedCounter()
	svc := newTempUnschedThresholdService(t, repo, counter, &TempUnschedThresholdSettings{
		Enabled: true, ThresholdCount: 3, ThresholdWindowMinutes: 1,
	})

	account := poolModeAccountWithRulesAndCustomCodes()
	body := []byte(`{"error":{"message":"Upstream request failed"}}`)

	// 前两次：命中规则但未达阈值 → 既不临时不可调度，也不能永久禁用
	for i := 0; i < 2; i++ {
		disable := svc.HandleUpstreamError(context.Background(), account, 502, nil, body)
		require.False(t, disable, "未达阈值不应禁用")
	}
	require.Equal(t, 0, repo.setErrorCalls, "绝不能永久 SetError")
	require.Equal(t, 0, repo.tempCalls, "未达阈值不应写临时不可调度")

	// 第三次：达阈值 → 临时不可调度（非永久禁用）
	disable := svc.HandleUpstreamError(context.Background(), account, 502, nil, body)
	require.True(t, disable)
	require.Equal(t, 0, repo.setErrorCalls, "应走临时不可调度而非永久禁用")
	require.Equal(t, 1, repo.tempCalls)
}

func TestHandleUpstreamError_CustomCodeWithTempUnschedRule_KeywordMismatchStillDisables(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := newFakeTempUnschedCounter()
	svc := newTempUnschedThresholdService(t, repo, counter, &TempUnschedThresholdSettings{
		Enabled: true, ThresholdCount: 3, ThresholdWindowMinutes: 1,
	})

	account := poolModeAccountWithRulesAndCustomCodes()
	body := []byte(`{"error":{"message":"different upstream failure"}}`)

	disable := svc.HandleUpstreamError(context.Background(), account, 502, nil, body)
	require.True(t, disable)
	require.Equal(t, 1, repo.setErrorCalls, "custom error code should still apply when the temp-unsched rule does not match")
	require.Equal(t, 0, repo.tempCalls)
	require.Empty(t, counter.counts, "keyword mismatch should not increment a temp-unsched threshold counter")
}

// TestHandleUpstreamError_CustomCodeWithoutTempUnschedRule_StillDisables
// 验证未被临时不可调度规则覆盖的自定义错误码仍按原逻辑永久禁用（不回归）。
func TestHandleUpstreamError_CustomCodeWithoutTempUnschedRule_StillDisables(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	svc := newTempUnschedThresholdService(t, repo, nil, &TempUnschedThresholdSettings{Enabled: true, ThresholdCount: 3, ThresholdWindowMinutes: 1})

	account := poolModeAccountWithRulesAndCustomCodes()
	// 503 在自定义错误码列表里，但没有对应的 temp-unsched 规则 → 应永久禁用
	disable := svc.HandleUpstreamError(context.Background(), account, 503, nil, []byte(`{"error":{"message":"some other error"}}`))
	require.True(t, disable)
	require.Equal(t, 1, repo.setErrorCalls, "无 temp-unsched 规则覆盖的自定义错误码应永久禁用")
}

func TestCheckErrorPolicy_CustomCodeWithTempUnschedRule_DefersUntilThreshold(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	counter := newFakeTempUnschedCounter()
	svc := newTempUnschedThresholdService(t, repo, counter, &TempUnschedThresholdSettings{
		Enabled: true, ThresholdCount: 2, ThresholdWindowMinutes: 1,
	})

	account := poolModeAccountWithRulesAndCustomCodes()
	body := []byte(`{"error":{"message":"Upstream request failed"}}`)

	result := svc.CheckErrorPolicy(context.Background(), account, 502, body)
	require.Equal(t, ErrorPolicyNone, result, "custom code must not terminate while temp-unsched threshold is deferred")
	require.Equal(t, 0, repo.tempCalls)

	result = svc.CheckErrorPolicy(context.Background(), account, 502, body)
	require.Equal(t, ErrorPolicyTempUnscheduled, result)
	require.Equal(t, 1, repo.tempCalls)
}
