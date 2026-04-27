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

func TestAdminServiceCreateAccount_DoesNotOverrideExplicitModelMapping(t *testing.T) {
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
		Credentials:          map[string]any{"api_key": "sk-test", "model_mapping": map[string]any{}},
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, map[string]any{}, account.Credentials["model_mapping"])
}
