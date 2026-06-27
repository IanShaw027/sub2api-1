//go:build unit

package service

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type settingUpdateRepoStub struct {
	updates map[string]string
}

func (s *settingUpdateRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *settingUpdateRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *settingUpdateRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *settingUpdateRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *settingUpdateRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	s.updates = make(map[string]string, len(settings))
	for k, v := range settings {
		s.updates[k] = v
	}
	return nil
}

func (s *settingUpdateRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *settingUpdateRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

type settingAntigravityUARepoStub struct {
	values map[string]string
}

func (s *settingAntigravityUARepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *settingAntigravityUARepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (s *settingAntigravityUARepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *settingAntigravityUARepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *settingAntigravityUARepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *settingAntigravityUARepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *settingAntigravityUARepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

type defaultSubGroupReaderStub struct {
	byID  map[int64]*Group
	errBy map[int64]error
	calls []int64
}

func (s *defaultSubGroupReaderStub) GetByID(ctx context.Context, id int64) (*Group, error) {
	s.calls = append(s.calls, id)
	if err, ok := s.errBy[id]; ok {
		return nil, err
	}
	if g, ok := s.byID[id]; ok {
		return g, nil
	}
	return nil, ErrGroupNotFound
}

func TestSettingService_UpdateSettings_DefaultSubscriptions_ValidGroup(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	groupReader := &defaultSubGroupReaderStub{
		byID: map[int64]*Group{
			11: {ID: 11, SubscriptionType: SubscriptionTypeSubscription},
		},
	}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetDefaultSubscriptionGroupReader(groupReader)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		DefaultSubscriptions: []DefaultSubscriptionSetting{
			{GroupID: 11, ValidityDays: 30},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []int64{11}, groupReader.calls)

	raw, ok := repo.updates[SettingKeyDefaultSubscriptions]
	require.True(t, ok)

	var got []DefaultSubscriptionSetting
	require.NoError(t, json.Unmarshal([]byte(raw), &got))
	require.Equal(t, []DefaultSubscriptionSetting{
		{GroupID: 11, ValidityDays: 30},
	}, got)
}

func TestSettingService_UpdateSettings_DefaultSubscriptions_RejectsNonSubscriptionGroup(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	groupReader := &defaultSubGroupReaderStub{
		byID: map[int64]*Group{
			12: {ID: 12, SubscriptionType: SubscriptionTypeStandard},
		},
	}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetDefaultSubscriptionGroupReader(groupReader)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		DefaultSubscriptions: []DefaultSubscriptionSetting{
			{GroupID: 12, ValidityDays: 7},
		},
	})
	require.Error(t, err)
	require.Equal(t, "DEFAULT_SUBSCRIPTION_GROUP_INVALID", infraerrors.Reason(err))
	require.Nil(t, repo.updates)
}

func TestSettingService_UpdateSettings_DefaultSubscriptions_RejectsNotFoundGroup(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	groupReader := &defaultSubGroupReaderStub{
		errBy: map[int64]error{
			13: ErrGroupNotFound,
		},
	}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetDefaultSubscriptionGroupReader(groupReader)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		DefaultSubscriptions: []DefaultSubscriptionSetting{
			{GroupID: 13, ValidityDays: 7},
		},
	})
	require.Error(t, err)
	require.Equal(t, "DEFAULT_SUBSCRIPTION_GROUP_INVALID", infraerrors.Reason(err))
	require.Equal(t, "13", infraerrors.FromError(err).Metadata["group_id"])
	require.Nil(t, repo.updates)
}

func TestSettingService_UpdateSettings_DefaultSubscriptions_RejectsDuplicateGroup(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	groupReader := &defaultSubGroupReaderStub{
		byID: map[int64]*Group{
			11: {ID: 11, SubscriptionType: SubscriptionTypeSubscription},
		},
	}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetDefaultSubscriptionGroupReader(groupReader)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		DefaultSubscriptions: []DefaultSubscriptionSetting{
			{GroupID: 11, ValidityDays: 30},
			{GroupID: 11, ValidityDays: 60},
		},
	})
	require.Error(t, err)
	require.Equal(t, "DEFAULT_SUBSCRIPTION_GROUP_DUPLICATE", infraerrors.Reason(err))
	require.Equal(t, "11", infraerrors.FromError(err).Metadata["group_id"])
	require.Nil(t, repo.updates)
}

func TestSettingService_UpdateSettings_DefaultSubscriptions_RejectsDuplicateGroupWithoutGroupReader(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		DefaultSubscriptions: []DefaultSubscriptionSetting{
			{GroupID: 11, ValidityDays: 30},
			{GroupID: 11, ValidityDays: 60},
		},
	})
	require.Error(t, err)
	require.Equal(t, "DEFAULT_SUBSCRIPTION_GROUP_DUPLICATE", infraerrors.Reason(err))
	require.Equal(t, "11", infraerrors.FromError(err).Metadata["group_id"])
	require.Nil(t, repo.updates)
}

func TestSettingService_UpdateSettings_RegistrationEmailSuffixWhitelist_Normalized(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		RegistrationEmailSuffixWhitelist: []string{"example.com", "@EXAMPLE.com", " @foo.bar ", "*.EDU.CN"},
	})
	require.NoError(t, err)
	require.Equal(t, `["@example.com","@foo.bar","*.edu.cn"]`, repo.updates[SettingKeyRegistrationEmailSuffixWhitelist])
}

func TestSettingService_UpdateSettings_RegistrationEmailSuffixWhitelist_Invalid(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		RegistrationEmailSuffixWhitelist: []string{"@invalid_domain"},
	})
	require.Error(t, err)
	require.Equal(t, "INVALID_REGISTRATION_EMAIL_SUFFIX_WHITELIST", infraerrors.Reason(err))
}

func TestParseDefaultSubscriptions_NormalizesValues(t *testing.T) {
	got := parseDefaultSubscriptions(`[{"group_id":11,"validity_days":30},{"group_id":11,"validity_days":60},{"group_id":0,"validity_days":10},{"group_id":12,"validity_days":99999}]`)
	require.Equal(t, []DefaultSubscriptionSetting{
		{GroupID: 11, ValidityDays: 30},
		{GroupID: 11, ValidityDays: 60},
		{GroupID: 12, ValidityDays: MaxValidityDays},
	}, got)
}

func TestSettingService_UpdateSettings_TablePreferences(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		TableDefaultPageSize: 50,
		TablePageSizeOptions: []int{20, 50, 100},
	})
	require.NoError(t, err)
	require.Equal(t, "50", repo.updates[SettingKeyTableDefaultPageSize])
	require.Equal(t, "[20,50,100]", repo.updates[SettingKeyTablePageSizeOptions])

	err = svc.UpdateSettings(context.Background(), &SystemSettings{
		TableDefaultPageSize: 1000,
		TablePageSizeOptions: []int{20, 100},
	})
	require.NoError(t, err)
	require.Equal(t, "1000", repo.updates[SettingKeyTableDefaultPageSize])
	require.Equal(t, "[20,100]", repo.updates[SettingKeyTablePageSizeOptions])
}

func TestSettingService_UpdateSettings_SubscriptionExpiryNotifyEnabled(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		SubscriptionExpiryNotifyEnabled: false,
	})
	require.NoError(t, err)
	require.Equal(t, "false", repo.updates[SettingKeySubscriptionExpiryNotifyEnabled])
}

func TestSettingService_UpdateSettings_PaymentVisibleMethodsAndAdvancedScheduler(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		PaymentVisibleMethodAlipaySource:  "alipay",
		PaymentVisibleMethodWxpaySource:   "easypay",
		PaymentVisibleMethodAlipayEnabled: true,
		PaymentVisibleMethodWxpayEnabled:  false,
		OpenAIAdvancedSchedulerEnabled:    true,
	})
	require.NoError(t, err)
	require.Equal(t, VisibleMethodSourceOfficialAlipay, repo.updates[SettingPaymentVisibleMethodAlipaySource])
	require.Equal(t, VisibleMethodSourceEasyPayWechat, repo.updates[SettingPaymentVisibleMethodWxpaySource])
	require.Equal(t, "true", repo.updates[SettingPaymentVisibleMethodAlipayEnabled])
	require.Equal(t, "false", repo.updates[SettingPaymentVisibleMethodWxpayEnabled])
	require.Equal(t, "true", repo.updates[openAIAdvancedSchedulerSettingKey])
}

func TestSettingService_UpdateSettings_OpenAIWSPoolRuntimeSettingsTriggerReconcile(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)
	RegisterOpenAIWSPoolReconcileHook(nil)
	t.Cleanup(func() { RegisterOpenAIWSPoolReconcileHook(nil) })

	reconcileCalls := make(chan struct{}, 4)
	RegisterOpenAIWSPoolReconcileHook(func() { reconcileCalls <- struct{}{} })

	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		OpenAIWSNeutralPrewarmPercent: 25,
		OpenAIWSSessionIdleTTLSeconds: 180,
	})
	require.NoError(t, err)
	require.Equal(t, "25", repo.updates[SettingKeyOpenAIWSNeutralPrewarmPercent])
	require.Equal(t, "180", repo.updates[SettingKeyOpenAIWSSessionIdleTTLSeconds])

	// 运行时快照已写入。
	neutralPrewarmPercent, sessionIdleTTLSeconds, ok := loadOpenAIWSPoolRuntimeSettingsForCompare()
	require.True(t, ok)
	require.Equal(t, 25, neutralPrewarmPercent)
	require.Equal(t, 180, sessionIdleTTLSeconds)

	// 首次变更应异步触发 reconcile 钩子。
	select {
	case <-reconcileCalls:
	case <-time.After(time.Second):
		t.Fatal("reconcile hook not triggered on first change")
	}

	// 相同值再次保存不应触发 reconcile。
	err = svc.UpdateSettings(context.Background(), &SystemSettings{
		OpenAIWSNeutralPrewarmPercent: 25,
		OpenAIWSSessionIdleTTLSeconds: 180,
	})
	require.NoError(t, err)
	select {
	case <-reconcileCalls:
		t.Fatal("reconcile hook should not fire when pool settings unchanged")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestSettingService_UpdateSettings_OpenAIWSPoolRuntimeSettingsRestoreDefaultsTriggersReconcile(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)
	RegisterOpenAIWSPoolReconcileHook(nil)
	t.Cleanup(func() { RegisterOpenAIWSPoolReconcileHook(nil) })

	reconcileCalls := make(chan struct{}, 4)
	RegisterOpenAIWSPoolReconcileHook(func() { reconcileCalls <- struct{}{} })

	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		OpenAIWSNeutralPrewarmPercent: 50,
		OpenAIWSSessionIdleTTLSeconds: 1200,
		OpenAIWSMinIdlePerAccount:     8,
		OpenAIWSMaxIdlePerAccount:     16,
		OpenAIStickyReservePercent:    20,
	})
	require.NoError(t, err)
	require.Equal(t, "1000", repo.updates[SettingKeyOpenAIWSSessionIdleTTLSeconds])
	select {
	case <-reconcileCalls:
	case <-time.After(time.Second):
		t.Fatal("initial reconcile hook not triggered")
	}

	err = svc.UpdateSettings(context.Background(), &SystemSettings{})
	require.NoError(t, err)
	require.Equal(t, strconv.Itoa(defaultOpenAIWSNeutralPrewarmPercent), repo.updates[SettingKeyOpenAIWSNeutralPrewarmPercent])
	require.Equal(t, strconv.Itoa(defaultOpenAIWSSessionIdleTTLSeconds), repo.updates[SettingKeyOpenAIWSSessionIdleTTLSeconds])

	select {
	case <-reconcileCalls:
	case <-time.After(time.Second):
		t.Fatal("reconcile hook not triggered when restoring default pool settings")
	}
}

func TestSettingService_UpdateSettingsWithAuthSourceDefaults_OpenAIWSPoolRuntimeSettingsTriggerReconcile(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)
	RegisterOpenAIWSPoolReconcileHook(nil)
	t.Cleanup(func() { RegisterOpenAIWSPoolReconcileHook(nil) })

	reconcileCalls := make(chan struct{}, 4)
	RegisterOpenAIWSPoolReconcileHook(func() { reconcileCalls <- struct{}{} })

	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettingsWithAuthSourceDefaults(context.Background(), &SystemSettings{
		OpenAIWSNeutralPrewarmPercent: 30,
		OpenAIWSSessionIdleTTLSeconds: 240,
	}, &AuthSourceDefaultSettings{})
	require.NoError(t, err)
	require.Equal(t, "30", repo.updates[SettingKeyOpenAIWSNeutralPrewarmPercent])
	require.Equal(t, "240", repo.updates[SettingKeyOpenAIWSSessionIdleTTLSeconds])

	select {
	case <-reconcileCalls:
	case <-time.After(time.Second):
		t.Fatal("reconcile hook not triggered on admin settings update path")
	}
}

func TestSettingService_GetAllSettings_LoadsOpenAIWSPoolRuntimeSettings(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	repo := &kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyOpenAIWSNeutralPrewarmPercent: "30",
			SettingKeyOpenAIWSSessionIdleTTLSeconds: "180",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, 30, settings.OpenAIWSNeutralPrewarmPercent)
	require.Equal(t, 180, settings.OpenAIWSSessionIdleTTLSeconds)

	neutralPrewarmPercent, sessionIdleTTLSeconds, ok := loadOpenAIWSPoolRuntimeSettingsForCompare()
	require.True(t, ok)
	require.Equal(t, 30, neutralPrewarmPercent)
	require.Equal(t, 180, sessionIdleTTLSeconds)
}

func TestSettingService_GetAllSettings_DefaultsOpenAIWSPoolRuntimeSettings(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	repo := &kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyOpenAIWSMinIdlePerAccount: "9",
			SettingKeyOpenAIWSMaxIdlePerAccount: "12",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, defaultOpenAIWSNeutralPrewarmPercent, settings.OpenAIWSNeutralPrewarmPercent)
	require.Equal(t, defaultOpenAIWSSessionIdleTTLSeconds, settings.OpenAIWSSessionIdleTTLSeconds)
	require.Equal(t, 9, settings.OpenAIWSMinIdlePerAccount, "legacy setting is still visible for compatibility")
	require.Equal(t, 12, settings.OpenAIWSMaxIdlePerAccount, "legacy setting is still visible for compatibility")

	neutralPrewarmPercent, sessionIdleTTLSeconds, ok := loadOpenAIWSPoolRuntimeSettingsForCompare()
	require.True(t, ok)
	require.Equal(t, defaultOpenAIWSNeutralPrewarmPercent, neutralPrewarmPercent)
	require.Equal(t, defaultOpenAIWSSessionIdleTTLSeconds, sessionIdleTTLSeconds)
}

func TestSettingService_LoadOpenAIWSPoolRuntimeSettingsInitializesCache(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)

	repo := &kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyOpenAIWSNeutralPrewarmPercent: "30",
			SettingKeyOpenAIWSSessionIdleTTLSeconds: "80",
			SettingKeyOpenAIWSMinIdlePerAccount:     "1",
			SettingKeyOpenAIWSMaxIdlePerAccount:     "8",
			SettingKeyOpenAIStickyReservePercent:    "30",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	require.NoError(t, svc.LoadOpenAIWSPoolRuntimeSettings(context.Background()))

	neutralPrewarmPercent, sessionIdleTTLSeconds, ok := loadOpenAIWSPoolRuntimeSettingsForCompare()
	require.True(t, ok)
	require.Equal(t, 30, neutralPrewarmPercent)
	require.Equal(t, 80, sessionIdleTTLSeconds)
	pool := newOpenAIWSConnPool(&config.Config{})
	t.Cleanup(pool.Close)
	require.Equal(t, 80*time.Second, pool.neutralIdleTTL())
}

func TestSettingService_UpdateSettings_OpenAIWSIdleSettingsDrivePoolRuntime(t *testing.T) {
	resetOpenAIWSPoolRuntimeSettingsCacheForTest()
	t.Cleanup(resetOpenAIWSPoolRuntimeSettingsCacheForTest)
	RegisterOpenAIWSPoolReconcileHook(nil)
	t.Cleanup(func() { RegisterOpenAIWSPoolReconcileHook(nil) })

	reconcileCalls := make(chan struct{}, 1)
	RegisterOpenAIWSPoolReconcileHook(func() { reconcileCalls <- struct{}{} })

	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})
	pool := newOpenAIWSConnPool(&config.Config{})
	t.Cleanup(pool.Close)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		OpenAIStickyReservePercent:    20,
		OpenAIWSMinIdlePerAccount:     6,
		OpenAIWSMaxIdlePerAccount:     10,
		OpenAIWSNeutralPrewarmPercent: 50,
		OpenAIWSSessionIdleTTLSeconds: 1200,
	})
	require.NoError(t, err)
	require.Equal(t, "20", repo.updates[SettingKeyOpenAIStickyReservePercent])
	require.Equal(t, "6", repo.updates[SettingKeyOpenAIWSMinIdlePerAccount])
	require.Equal(t, "10", repo.updates[SettingKeyOpenAIWSMaxIdlePerAccount])
	require.Equal(t, "50", repo.updates[SettingKeyOpenAIWSNeutralPrewarmPercent])
	require.Equal(t, "1000", repo.updates[SettingKeyOpenAIWSSessionIdleTTLSeconds])

	neutralPrewarmPercent, sessionIdleTTLSeconds, ok := loadOpenAIWSPoolRuntimeSettingsForCompare()
	require.True(t, ok)
	require.Equal(t, 50, neutralPrewarmPercent)
	require.Equal(t, 1000, sessionIdleTTLSeconds)
	require.Equal(t, 6, pool.minIdlePerAccount())
	require.Equal(t, 10, pool.maxIdlePerAccount())
	require.Equal(t, 20, pool.stickyReservePercent())

	select {
	case <-reconcileCalls:
	case <-time.After(time.Second):
		t.Fatal("pool reconcile should fire when runtime WS pool settings change")
	}
}

func TestSettingService_UpdateSettings_OpenAIOAuthImageBridgeTransportSettings(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		OpenAIOAuthImageBridgeDisableKeepAlives:   true,
		OpenAIOAuthImageBridgeFreshUpstreamClient: true,
	})
	require.NoError(t, err)
	require.Equal(t, "true", repo.updates[SettingKeyOpenAIOAuthImageBridgeDisableKeepAlives])
	require.Equal(t, "true", repo.updates[SettingKeyOpenAIOAuthImageBridgeFreshUpstreamClient])
}

func TestSettingService_UpdateSettings_AntigravityUserAgentVersion(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		AntigravityUserAgentVersion: "1.23.2",
	})
	require.NoError(t, err)
	require.Equal(t, "1.23.2", repo.updates[SettingKeyAntigravityUserAgentVersion])
}

func TestSettingService_UpdateSettings_APIKeyACLTrustForwardedIPRefreshesConfig(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	cfg := &config.Config{}
	svc := NewSettingService(repo, cfg)

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		APIKeyACLTrustForwardedIP: true,
	})
	require.NoError(t, err)
	require.Equal(t, "true", repo.updates[SettingKeyAPIKeyACLTrustForwardedIP])
	require.True(t, cfg.Security.TrustForwardedIPForAPIKeyACL)
	require.True(t, cfg.TrustForwardedIPForAPIKeyACL())
}

func TestSettingService_ParseSettings_APIKeyACLTrustForwardedIPFallsBackToConfigWhenMissing(t *testing.T) {
	cfg := &config.Config{}
	cfg.Security.TrustForwardedIPForAPIKeyACL = true
	svc := NewSettingService(&settingUpdateRepoStub{}, cfg)

	got := svc.parseSettings(map[string]string{})

	require.True(t, got.APIKeyACLTrustForwardedIP)
}

func TestSettingService_GetAntigravityUserAgentVersion_Precedence(t *testing.T) {
	t.Run("后台设置优先", func(t *testing.T) {
		svc := NewSettingService(&settingAntigravityUARepoStub{values: map[string]string{
			SettingKeyAntigravityUserAgentVersion: "1.24.0",
		}}, &config.Config{})

		require.Equal(t, "1.24.0", svc.GetAntigravityUserAgentVersion(context.Background()))
	})

	t.Run("空值回退配置默认值", func(t *testing.T) {
		svc := NewSettingService(&settingAntigravityUARepoStub{values: map[string]string{
			SettingKeyAntigravityUserAgentVersion: "",
		}}, &config.Config{})

		require.Equal(t, antigravity.GetDefaultUserAgentVersion(), svc.GetAntigravityUserAgentVersion(context.Background()))
	})

	t.Run("缺失回退配置默认值", func(t *testing.T) {
		svc := NewSettingService(&settingAntigravityUARepoStub{values: map[string]string{}}, &config.Config{})

		require.Equal(t, antigravity.GetDefaultUserAgentVersion(), svc.GetAntigravityUserAgentVersion(context.Background()))
	})
}

func TestSettingService_UpdateSettings_RejectsInvalidPaymentVisibleMethodSource(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		PaymentVisibleMethodAlipaySource: "not-a-provider",
	})
	require.Error(t, err)
	require.Equal(t, "INVALID_PAYMENT_VISIBLE_METHOD_SOURCE", infraerrors.Reason(err))
	require.Nil(t, repo.updates)
}

func TestSettingService_UpdateSettings_RejectsInvalidPlatformDefaultModelMapping(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]DefaultAccountModelConfig
	}{
		{
			name: "request wildcard not at end",
			cfg: map[string]DefaultAccountModelConfig{
				"kiro": {
					ModelMapping: map[string]string{"claude-*sonnet": "claude-sonnet-4.6"},
				},
			},
		},
		{
			name: "target wildcard",
			cfg: map[string]DefaultAccountModelConfig{
				"kiro": {
					ModelMapping: map[string]string{"claude-sonnet-*": "claude-*"},
				},
			},
		},
		{
			name: "compact target wildcard",
			cfg: map[string]DefaultAccountModelConfig{
				"openai": {
					CompactModelMapping: map[string]string{"gpt-5.4": "gpt-*"},
				},
			},
		},
		{
			name: "kiro cross family mapping",
			cfg: map[string]DefaultAccountModelConfig{
				"kiro": {
					ModelMapping: map[string]string{"claude-opus-4-6": "claude-sonnet-4.6"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &settingUpdateRepoStub{}
			svc := NewSettingService(repo, &config.Config{})

			err := svc.UpdateSettings(context.Background(), &SystemSettings{
				PlatformDefaultAccountModelConfig: tt.cfg,
			})

			require.Error(t, err)
			require.Equal(t, "INVALID_PLATFORM_DEFAULT_ACCOUNT_MODEL_CONFIG", infraerrors.Reason(err))
			require.Nil(t, repo.updates)
		})
	}
}

func TestSettingService_UpdateSettings_RejectsInvalidPlatformDefaultTempUnschedAndCustomCodes(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]DefaultAccountModelConfig
	}{
		{
			name: "temp unsched error code below http status range",
			cfg: map[string]DefaultAccountModelConfig{
				"openai": {
					TempUnschedulableRules: []TempUnschedulableRule{
						{ErrorCode: 99, DurationMinutes: 10},
					},
				},
			},
		},
		{
			name: "temp unsched error code above http status range",
			cfg: map[string]DefaultAccountModelConfig{
				"openai": {
					TempUnschedulableRules: []TempUnschedulableRule{
						{ErrorCode: 600, DurationMinutes: 10},
					},
				},
			},
		},
		{
			name: "temp unsched non-positive duration",
			cfg: map[string]DefaultAccountModelConfig{
				"openai": {
					TempUnschedulableRules: []TempUnschedulableRule{
						{ErrorCode: 502, DurationMinutes: 0},
					},
				},
			},
		},
		{
			name: "custom error code below http status range",
			cfg: map[string]DefaultAccountModelConfig{
				"openai": {
					CustomErrorCodes: []int{99},
				},
			},
		},
		{
			name: "custom error code above http status range",
			cfg: map[string]DefaultAccountModelConfig{
				"openai": {
					CustomErrorCodes: []int{600},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &settingUpdateRepoStub{}
			svc := NewSettingService(repo, &config.Config{})

			err := svc.UpdateSettings(context.Background(), &SystemSettings{
				PlatformDefaultAccountModelConfig: tt.cfg,
			})

			require.Error(t, err)
			require.Equal(t, "INVALID_PLATFORM_DEFAULT_ACCOUNT_MODEL_CONFIG", infraerrors.Reason(err))
			require.Nil(t, repo.updates)
		})
	}
}

func TestSettingService_UpdateSettings_RejectsPlatformDefaultTempUnschedEnabledWithoutRules(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		PlatformDefaultAccountModelConfig: map[string]DefaultAccountModelConfig{
			"openai": {
				TempUnschedulableEnabled: true,
			},
		},
	})

	require.Error(t, err)
	require.Equal(t, "INVALID_PLATFORM_DEFAULT_ACCOUNT_MODEL_CONFIG", infraerrors.Reason(err))
	require.Nil(t, repo.updates)
}

func TestSettingService_UpdateSettings_PlatformDefaultKiroVariantsAcceptCaseInsensitivePlatformKey(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		PlatformDefaultAccountModelConfig: map[string]DefaultAccountModelConfig{
			"Kiro": {
				KiroSubscriptionTypeModelMap: map[string]DefaultAccountModelConfig{
					"pro": {
						ModelMapping: map[string]string{"claude-sonnet-*": "claude-sonnet-4.6"},
					},
				},
			},
		},
	})

	require.NoError(t, err)
	require.JSONEq(t, `{
		"kiro": {
			"kiro_subscription_type_model_config": {
				"pro": {
					"model_mapping": {"claude-sonnet-*": "claude-sonnet-4.6"}
				}
			}
		}
	}`, repo.updates[SettingKeyPlatformDefaultAccountModelConfig])
}

func TestSettingService_UpdateSettings_RejectsAccountDefaultFieldsInKiroSubscriptionVariants(t *testing.T) {
	tests := []struct {
		name string
		cfg  DefaultAccountModelConfig
	}{
		{
			name: "temp unsched enabled",
			cfg: DefaultAccountModelConfig{
				ModelMapping:             map[string]string{"claude-sonnet-*": "claude-sonnet-4.6"},
				TempUnschedulableEnabled: true,
			},
		},
		{
			name: "temp unsched rules",
			cfg: DefaultAccountModelConfig{
				ModelMapping:           map[string]string{"claude-sonnet-*": "claude-sonnet-4.6"},
				TempUnschedulableRules: []TempUnschedulableRule{{ErrorCode: 502, DurationMinutes: 10}},
			},
		},
		{
			name: "custom error codes enabled",
			cfg: DefaultAccountModelConfig{
				ModelMapping:            map[string]string{"claude-sonnet-*": "claude-sonnet-4.6"},
				CustomErrorCodesEnabled: true,
			},
		},
		{
			name: "custom error codes",
			cfg: DefaultAccountModelConfig{
				ModelMapping:     map[string]string{"claude-sonnet-*": "claude-sonnet-4.6"},
				CustomErrorCodes: []int{502},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &settingUpdateRepoStub{}
			svc := NewSettingService(repo, &config.Config{})

			err := svc.UpdateSettings(context.Background(), &SystemSettings{
				PlatformDefaultAccountModelConfig: map[string]DefaultAccountModelConfig{
					"kiro": {
						KiroSubscriptionTypeModelMap: map[string]DefaultAccountModelConfig{
							"pro": tt.cfg,
						},
					},
				},
			})

			require.Error(t, err)
			require.Equal(t, "INVALID_PLATFORM_DEFAULT_ACCOUNT_MODEL_CONFIG", infraerrors.Reason(err))
			require.Nil(t, repo.updates)
		})
	}
}

func TestClonePlatformModelConfigMap_PreservesAccountDefaultPolicyFields(t *testing.T) {
	src := map[string]DefaultAccountModelConfig{
		"openai": {
			ModelWhitelist:           []string{"gpt-5.4"},
			TempUnschedulableEnabled: true,
			TempUnschedulableRules:   []TempUnschedulableRule{{ErrorCode: 502, Keywords: []string{"overloaded"}, DurationMinutes: 10}},
			CustomErrorCodesEnabled:  true,
			CustomErrorCodes:         []int{500, 502},
			KiroSubscriptionTypeModelMap: map[string]DefaultAccountModelConfig{
				"free": {
					TempUnschedulableEnabled: true,
					TempUnschedulableRules:   []TempUnschedulableRule{{ErrorCode: 429, DurationMinutes: 5}},
					CustomErrorCodesEnabled:  true,
					CustomErrorCodes:         []int{429},
				},
			},
		},
	}

	cloned := clonePlatformModelConfigMap(src)

	require.True(t, cloned["openai"].TempUnschedulableEnabled)
	require.Equal(t, []TempUnschedulableRule{{ErrorCode: 502, Keywords: []string{"overloaded"}, DurationMinutes: 10}}, cloned["openai"].TempUnschedulableRules)
	require.True(t, cloned["openai"].CustomErrorCodesEnabled)
	require.Equal(t, []int{500, 502}, cloned["openai"].CustomErrorCodes)
	require.True(t, cloned["openai"].KiroSubscriptionTypeModelMap["free"].TempUnschedulableEnabled)
	require.Equal(t, []TempUnschedulableRule{{ErrorCode: 429, DurationMinutes: 5}}, cloned["openai"].KiroSubscriptionTypeModelMap["free"].TempUnschedulableRules)
	require.True(t, cloned["openai"].KiroSubscriptionTypeModelMap["free"].CustomErrorCodesEnabled)
	require.Equal(t, []int{429}, cloned["openai"].KiroSubscriptionTypeModelMap["free"].CustomErrorCodes)

	src["openai"].TempUnschedulableRules[0].ErrorCode = 503
	src["openai"].CustomErrorCodes[0] = 599
	require.Equal(t, 502, cloned["openai"].TempUnschedulableRules[0].ErrorCode)
	require.Equal(t, 500, cloned["openai"].CustomErrorCodes[0])
}
