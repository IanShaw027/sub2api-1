package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type accountModelDefaultsSettingRepoStub struct {
	values map[string]string
}

func (s *accountModelDefaultsSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *accountModelDefaultsSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", ErrSettingNotFound
}

func (s *accountModelDefaultsSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *accountModelDefaultsSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *accountModelDefaultsSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *accountModelDefaultsSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *accountModelDefaultsSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestAdminServiceCreateAccount_InjectsPlatformDefaultModelConfig(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	repo := &kiroDefaultAccountRepoStub{}
	settingRepo := &accountModelDefaultsSettingRepoStub{values: map[string]string{
		SettingKeyPlatformDefaultAccountModelConfig: `{
			"anthropic": {
				"model_whitelist": ["claude-sonnet-4-5-20250929", "claude-*"],
				"model_mapping": {"claude-opus-4-6": "claude-sonnet-4-5-20250929"},
				"compact_model_mapping": {"claude-sonnet-4-6": "claude-sonnet-4-5-20250929"}
			}
		}`,
	}}
	svc := &adminServiceImpl{
		accountRepo:    repo,
		settingService: NewSettingService(settingRepo, &config.Config{}),
	}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "anthropic-key",
		Platform:             PlatformAnthropic,
		Type:                 AccountTypeAPIKey,
		Credentials:          map[string]any{"api_key": "sk-test"},
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.ElementsMatch(t, []string{"claude-sonnet-4-5-20250929", "claude-*"}, account.Credentials["model_whitelist"])
	require.Equal(t, map[string]any{
		"claude-opus-4-6": "claude-sonnet-4-5-20250929",
	}, account.Credentials["model_mapping"])
	require.Equal(t, map[string]string{
		"claude-opus-4-6": "claude-sonnet-4-5-20250929",
	}, account.GetModelMapping())
	require.Equal(t, map[string]string{
		"claude-sonnet-4-6": "claude-sonnet-4-5-20250929",
	}, account.Credentials["compact_model_mapping"])
}

func TestAdminServiceCreateAccount_RejectsNilInput(t *testing.T) {
	repo := &kiroDefaultAccountRepoStub{}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.CreateAccount(context.Background(), nil)

	require.Nil(t, account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "account input is required")
	require.Empty(t, repo.createdAccounts)
}

func TestAdminServiceCreateAccount_InjectsPlatformDefaultWhenModelMappingIsEmpty(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	settingRepo := &accountModelDefaultsSettingRepoStub{values: map[string]string{
		SettingKeyPlatformDefaultAccountModelConfig: `{
			"kiro": {"model_mapping": {"anthropic-opus-4-8": "claude-opus-4.8"}}
		}`,
	}}
	svc := &adminServiceImpl{
		accountRepo:    &kiroDefaultAccountRepoStub{},
		settingService: NewSettingService(settingRepo, &config.Config{}),
	}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "kiro-pro",
		Platform:             PlatformKiro,
		Type:                 AccountTypeOAuth,
		Credentials:          map[string]any{"refresh_token": "rt-test-valid-refresh-token-1234567890", "model_mapping": map[string]any{}},
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, map[string]any{
		"anthropic-opus-4-8": "claude-opus-4.8",
	}, account.Credentials["model_mapping"])
}

func TestAdminServiceCreateAccount_DoesNotOverrideNonEmptyExplicitModelMapping(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	settingRepo := &accountModelDefaultsSettingRepoStub{values: map[string]string{
		SettingKeyPlatformDefaultAccountModelConfig: `{
			"openai": {"model_mapping": {"gpt-5.2": "gpt-5.4"}}
		}`,
	}}
	svc := &adminServiceImpl{
		accountRepo:    &kiroDefaultAccountRepoStub{},
		settingService: NewSettingService(settingRepo, &config.Config{}),
	}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "openai-key",
		Platform:             PlatformOpenAI,
		Type:                 AccountTypeAPIKey,
		Credentials:          map[string]any{"api_key": "sk-test", "model_mapping": map[string]any{"gpt-5.2": "gpt-account"}},
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, map[string]any{"gpt-5.2": "gpt-account"}, account.Credentials["model_mapping"])
}

func TestAdminServiceCreateAccount_InjectsKiroSubscriptionTypeDefaultModelConfig(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	repo := &kiroDefaultAccountRepoStub{}
	settingRepo := &accountModelDefaultsSettingRepoStub{values: map[string]string{
		SettingKeyPlatformDefaultAccountModelConfig: `{
			"kiro": {
				"model_mapping": {"claude-sonnet-*": "claude-sonnet-4.6"},
				"kiro_subscription_type_model_config": {
					"pro": {
						"model_mapping": {"claude-opus-*": "claude-opus-4.7"},
						"compact_model_mapping": {"claude-opus-4.6": "claude-opus-4.7"}
					}
				}
			}
		}`,
	}}
	svc := &adminServiceImpl{
		accountRepo:    repo,
		settingService: NewSettingService(settingRepo, &config.Config{}),
	}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "kiro-pro",
		Platform:             PlatformKiro,
		Type:                 AccountTypeOAuth,
		Credentials:          map[string]any{"refresh_token": "rt-test-valid-refresh-token-1234567890", "subscription_type": "Kiro Pro"},
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, map[string]any{
		"claude-sonnet-*": "claude-sonnet-4.6",
		"claude-opus-*":   "claude-opus-4.7",
	}, account.Credentials["model_mapping"])
	require.Equal(t, map[string]string{
		"claude-opus-4.6": "claude-opus-4.7",
	}, account.Credentials["compact_model_mapping"])
}

func TestAdminServiceCreateAccount_InjectsKiroFreeDefaultsWhenSubscriptionMissing(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	repo := &kiroDefaultAccountRepoStub{}
	settingRepo := &accountModelDefaultsSettingRepoStub{values: map[string]string{
		SettingKeyPlatformDefaultAccountModelConfig: `{
			"kiro": {
				"model_mapping": {"claude-sonnet-*": "claude-sonnet-4.6"},
				"kiro_subscription_type_model_config": {
					"free": {
						"model_mapping": {"claude-sonnet-4-5": "claude-sonnet-4.5"}
					}
				}
			}
		}`,
	}}
	svc := &adminServiceImpl{
		accountRepo:    repo,
		settingService: NewSettingService(settingRepo, &config.Config{}),
	}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "kiro-free",
		Platform:             PlatformKiro,
		Type:                 AccountTypeOAuth,
		Credentials:          map[string]any{"refresh_token": "rt-test-valid-refresh-token-1234567890"},
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, map[string]any{
		"claude-sonnet-*":   "claude-sonnet-4.6",
		"claude-sonnet-4-5": "claude-sonnet-4.5",
	}, account.Credentials["model_mapping"])
}

func TestAdminServiceCreateAccount_InjectsTempUnschedAndCustomErrorCodeDefaults(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	repo := &kiroDefaultAccountRepoStub{}
	settingRepo := &accountModelDefaultsSettingRepoStub{values: map[string]string{
		SettingKeyPlatformDefaultAccountModelConfig: `{
			"openai": {
				"temp_unschedulable_enabled": true,
				"temp_unschedulable_rules": [
					{"error_code": 99, "keywords": ["bad"], "duration_minutes": 10},
					{"error_code": 502, "keywords": ["Upstream request failed"], "duration_minutes": 10},
					{"error_code": 524, "duration_minutes": 10},
					{"error_code": 600, "keywords": ["bad"], "duration_minutes": 10}
				],
				"custom_error_codes_enabled": true,
				"custom_error_codes": [99, 500, 502, 503, 600]
			}
		}`,
	}}
	svc := &adminServiceImpl{
		accountRepo:    repo,
		settingService: NewSettingService(settingRepo, &config.Config{}),
	}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "openai-key",
		Platform:             PlatformOpenAI,
		Type:                 AccountTypeAPIKey,
		Credentials:          map[string]any{"api_key": "sk-test"},
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.True(t, account.IsTempUnschedulableEnabled())
	require.True(t, account.IsCustomErrorCodesEnabled())
	require.ElementsMatch(t, []int{500, 502, 503}, account.GetCustomErrorCodes())

	rules := account.GetTempUnschedulableRules()
	require.Len(t, rules, 2)
	require.Equal(t, 502, rules[0].ErrorCode)
	require.Equal(t, []string{"Upstream request failed"}, rules[0].Keywords)
	require.Equal(t, 10, rules[0].DurationMinutes)
	// 第二条 keywords 留空（纯错误码匹配）
	require.Equal(t, 524, rules[1].ErrorCode)
	require.Empty(t, rules[1].Keywords)
}

func TestAdminServiceCreateAccount_DoesNotOverrideExplicitTempUnschedRules(t *testing.T) {
	resetPlatformModelRoutingConfigCacheForTest()
	settingRepo := &accountModelDefaultsSettingRepoStub{values: map[string]string{
		SettingKeyPlatformDefaultAccountModelConfig: `{
			"openai": {
				"temp_unschedulable_enabled": true,
				"temp_unschedulable_rules": [{"error_code": 502, "keywords": ["x"], "duration_minutes": 10}]
			}
		}`,
	}}
	svc := &adminServiceImpl{
		accountRepo:    &kiroDefaultAccountRepoStub{},
		settingService: NewSettingService(settingRepo, &config.Config{}),
	}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "openai-key",
		Platform:             PlatformOpenAI,
		Type:                 AccountTypeAPIKey,
		Credentials:          map[string]any{"api_key": "sk-test", "temp_unschedulable_rules": []any{}},
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	// 用户显式传了空数组，不应被默认值覆盖
	require.Equal(t, []any{}, account.Credentials["temp_unschedulable_rules"])
}
