package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type accountRepoStubForAdminCreateValidation struct {
	kiroDefaultAccountRepoStub
	createCalled     bool
	bindGroupsCalled bool
}

func (s *accountRepoStubForAdminCreateValidation) Create(_ context.Context, account *Account) error {
	s.createCalled = true
	account.ID = 101
	return nil
}

func (s *accountRepoStubForAdminCreateValidation) BindGroups(_ context.Context, _ int64, _ []int64) error {
	s.bindGroupsCalled = true
	return nil
}

func TestAdminServiceCreateAccount_AllowsKiroAPIKeyType(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForAdminCreateValidation{}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "kiro-key",
		Platform:             PlatformKiro,
		Type:                 AccountTypeAPIKey,
		Credentials:          map[string]any{"api_key": "sk-test"},
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.True(t, repo.createCalled)
	require.Equal(t, AccountTypeAPIKey, account.Type)
}

func TestAdminServiceCreateAccount_AllowsGrokOAuthType(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForAdminCreateValidation{}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "grok-oauth",
		Platform:             PlatformGrok,
		Type:                 AccountTypeOAuth,
		Credentials:          map[string]any{"refresh_token": "rt-test"},
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.True(t, repo.createCalled)
	require.Equal(t, PlatformGrok, account.Platform)
	require.Equal(t, AccountTypeOAuth, account.Type)
}

func TestAdminServiceCreateAccount_RejectsUnsupportedGrokAccountTypes(t *testing.T) {
	t.Parallel()

	for _, accountType := range []string{AccountTypeSetupToken, AccountTypeServiceAccount, AccountTypeBedrock, AccountTypeUpstream} {
		accountType := accountType
		t.Run(accountType, func(t *testing.T) {
			t.Parallel()

			repo := &accountRepoStubForAdminCreateValidation{}
			svc := &adminServiceImpl{accountRepo: repo}

			account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
				Name:                 "grok-unsupported",
				Platform:             PlatformGrok,
				Type:                 accountType,
				Credentials:          map[string]any{"refresh_token": "rt-test"},
				SkipDefaultGroupBind: true,
			})

			require.Nil(t, account)
			require.Error(t, err)
			require.False(t, repo.createCalled)
			require.Contains(t, err.Error(), "grok accounts only support oauth or apikey type")
		})
	}
}

func TestAdminServiceCreateAccount_RejectsInvalidKiroCredentials(t *testing.T) {
	t.Parallel()

	svc := &adminServiceImpl{accountRepo: &kiroDefaultAccountRepoStub{}}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:        "kiro-oauth",
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"auth_method": "social"},
	})

	require.Nil(t, account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "kiro refresh_token is required")
}

func TestAdminServiceCreateAccount_NormalizesExternalIDPRawKiroShape(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForAdminCreateValidation{}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:     "kiro-external-idp",
		Platform: PlatformKiro,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"kiro_auth_token_raw": map[string]any{
				"accessToken":   "access-token",
				"refreshToken":  "refresh-token",
				"authMethod":    "external_idp",
				"clientId":      "microsoft-public-client",
				"expiresAt":     "2026-07-01T07:57:02.932Z",
				"issuerUrl":     "https://login.microsoftonline.com/tenant/v2.0",
				"provider":      "ExternalIdp",
				"scopes":        []any{"scope-a", "scope-b"},
				"tokenEndpoint": "https://login.microsoftonline.com/tenant/oauth2/v2.0/token",
			},
			"kiro_profile_raw": map[string]any{
				"arn": "arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD",
			},
		},
		SkipDefaultGroupBind: true,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.True(t, repo.createCalled)
	require.Equal(t, "external_idp", account.GetCredential("auth_method"))
	require.Equal(t, "access-token", account.GetCredential("access_token"))
	require.Equal(t, "refresh-token", account.GetCredential("refresh_token"))
	require.Equal(t, "microsoft-public-client", account.GetCredential("client_id"))
	require.Equal(t, "scope-a scope-b", account.GetCredential("scopes"))
	require.Equal(t, "https://login.microsoftonline.com/tenant/oauth2/v2.0/token", account.GetCredential("token_endpoint"))
	require.Equal(t, "arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD", account.GetCredential("profile_arn"))
	require.Equal(t, "CQRAXYDP9YVD", account.GetCredential("profile_id"))
}

func TestNormalizeKiroOAuthCredentialShapeRejectsUnsafeExternalIDPTokenEndpoints(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name        string
		credentials map[string]any
	}{
		{
			name: "explicit private http endpoint",
			credentials: map[string]any{
				"auth_method":    "external_idp",
				"token_endpoint": "http://127.0.0.1:8081/capture",
			},
		},
		{
			name: "microsoft lookalike issuer",
			credentials: map[string]any{
				"auth_method": "external_idp",
				"issuer_url":  "https://login.microsoftonline.com.evil.example/tenant/v2.0",
			},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			normalized := NormalizeKiroOAuthCredentialShape(tt.credentials)

			require.Empty(t, normalized["token_endpoint"])
		})
	}
}

func TestNormalizeKiroOAuthCredentialShapeAcceptsMicrosoftExternalIDPEndpointAndClientSecret(t *testing.T) {
	t.Parallel()

	normalized := NormalizeKiroOAuthCredentialShape(map[string]any{
		"authMethod":    "idc",
		"clientId":      "client-id",
		"clientSecret":  "client-secret",
		"issuerUrl":     "https://login.microsoftonline.com/tenant/v2.0",
		"tokenEndpoint": "https://login.microsoftonline.com/tenant/oauth2/v2.0/token",
	})

	require.Equal(t, "client-id", normalized["client_id"])
	require.Equal(t, "client-secret", normalized["client_secret"])
	require.Equal(t, "https://login.microsoftonline.com/tenant/oauth2/v2.0/token", normalized["token_endpoint"])
}

func TestAdminServiceUpdateAccount_AllowsSwitchToKiroAPIKeyType(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			55: {
				ID:       55,
				Name:     "kiro-oauth",
				Platform: PlatformKiro,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"refresh_token": "rt-placeholder",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.UpdateAccount(context.Background(), 55, &UpdateAccountInput{
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test"},
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, AccountTypeAPIKey, account.Type)
	require.Equal(t, "sk-test", account.GetCredential("api_key"))
}

func TestAdminServiceUpdateAccount_IgnoresInvalidKiroIDCAuthPatches(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			55: {
				ID:       55,
				Name:     "kiro-oauth",
				Platform: PlatformKiro,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"refresh_token": "rt-placeholder",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.UpdateAccount(context.Background(), 55, &UpdateAccountInput{
		Credentials: map[string]any{
			"refresh_token": "rt-placeholder",
			"auth_method":   "idc",
			"client_id":     "client-only",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, "rt-placeholder", account.GetCredential("refresh_token"))
	require.Empty(t, account.GetCredential("auth_method"))
	require.Empty(t, account.GetCredential("client_id"))
}

func TestAdminServiceUpdateAccount_IgnoresKiroOAuthAuthCredentialPatches(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			55: {
				ID:       55,
				Name:     "kiro-oauth",
				Platform: PlatformKiro,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"refresh_token":                  "old-refresh",
					"access_token":                   "old-access",
					"expires_at":                     "1735689600",
					"client_id":                      "old-client-id",
					"client_secret":                  "old-client-secret",
					"auth_method":                    "idc",
					"profile_arn":                    "arn:aws:kiro:old",
					"auth_region":                    "us-west-2",
					"api_region":                     "us-east-2",
					"machine_id":                     "machine-1",
					"model_mapping":                  map[string]any{"claude-sonnet-4": "claude-sonnet-4"},
					"temp_unschedulable_enabled":     true,
					"temp_unschedulable_rules":       []any{map[string]any{"error_code": float64(429), "duration_minutes": float64(10)}},
					"intercept_warmup_requests":      true,
					"custom_unrelated_runtime_field": "keep-me",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.UpdateAccount(context.Background(), 55, &UpdateAccountInput{
		Credentials: map[string]any{
			"refresh_token": "new-refresh",
			"access_token":  "new-access",
			"expires_at":    nil,
			"client_id":     "new-client-id",
			"client_secret": "new-client-secret",
			"auth_method":   "social",
			"profile_arn":   "arn:aws:kiro:new",
			"auth_region":   "eu-west-1",
			"api_region":    "eu-central-1",
			"machine_id":    "machine-2",
			"model_mapping": map[string]any{"claude-sonnet-4.6": "claude-sonnet-4.6"},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, "old-refresh", account.GetCredential("refresh_token"))
	require.Equal(t, "old-access", account.GetCredential("access_token"))
	require.Equal(t, "1735689600", account.GetCredential("expires_at"))
	require.Equal(t, "old-client-id", account.GetCredential("client_id"))
	require.Equal(t, "old-client-secret", account.GetCredential("client_secret"))
	require.Equal(t, "idc", account.GetCredential("auth_method"))
	require.Equal(t, "arn:aws:kiro:new", account.GetCredential("profile_arn"))
	require.Equal(t, "eu-west-1", account.GetCredential("auth_region"))
	require.Equal(t, "eu-central-1", account.GetCredential("api_region"))
	require.Equal(t, "machine-2", account.GetCredential("machine_id"))
	require.Equal(t, map[string]any{"claude-sonnet-4.6": "claude-sonnet-4.6"}, account.Credentials["model_mapping"])
	require.Equal(t, true, account.Credentials["temp_unschedulable_enabled"])
	require.Equal(t, []any{map[string]any{"error_code": float64(429), "duration_minutes": float64(10)}}, account.Credentials["temp_unschedulable_rules"])
	require.Equal(t, true, account.Credentials["intercept_warmup_requests"])
	require.Equal(t, "keep-me", account.GetCredential("custom_unrelated_runtime_field"))
}

func TestAdminServiceUpdateAccount_PreservesOmittedKiroAPIKeyCredentials(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			56: {
				ID:       56,
				Name:     "kiro-apikey",
				Platform: PlatformKiro,
				Type:     AccountTypeAPIKey,
				Status:   StatusActive,
				Credentials: map[string]any{
					"api_key":       "old-api-key",
					"region":        "us-east-1",
					"model":         "legacy-model",
					"profile_arn":   "arn:aws:kiro:old",
					"auth_region":   "us-west-2",
					"api_region":    "us-east-2",
					"machine_id":    "machine-1",
					"model_mapping": map[string]any{"claude-sonnet-4": "claude-sonnet-4"},
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.UpdateAccount(context.Background(), 56, &UpdateAccountInput{
		Credentials: map[string]any{
			"region": "eu-west-1",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, "old-api-key", account.GetCredential("api_key"))
	require.Equal(t, "eu-west-1", account.GetCredential("region"))
	require.Equal(t, "legacy-model", account.GetCredential("model"))
	require.Equal(t, "arn:aws:kiro:old", account.GetCredential("profile_arn"))
	require.Equal(t, "us-west-2", account.GetCredential("auth_region"))
	require.Equal(t, "us-east-2", account.GetCredential("api_region"))
	require.Equal(t, "machine-1", account.GetCredential("machine_id"))
	require.Equal(t, map[string]any{"claude-sonnet-4": "claude-sonnet-4"}, account.Credentials["model_mapping"])
}

func TestAdminServiceUpdateAccount_IgnoresKiroOAuthExpiresAtEmptyStringPatch(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			57: {
				ID:       57,
				Name:     "kiro-oauth",
				Platform: PlatformKiro,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"refresh_token": "rt-placeholder",
					"expires_at":    "1735689600",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.UpdateAccount(context.Background(), 57, &UpdateAccountInput{
		Credentials: map[string]any{
			"expires_at": "",
		},
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, "rt-placeholder", account.GetCredential("refresh_token"))
	require.Equal(t, "1735689600", account.GetCredential("expires_at"))
}

func TestAdminServiceUpdateAccount_IgnoresKiroOAuthExpiresAtNullPatch(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			58: {
				ID:       58,
				Name:     "kiro-oauth",
				Platform: PlatformKiro,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"refresh_token": "rt-placeholder",
					"expires_at":    "1735689600",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.UpdateAccount(context.Background(), 58, &UpdateAccountInput{
		Credentials: map[string]any{
			"expires_at": nil,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, "rt-placeholder", account.GetCredential("refresh_token"))
	require.Equal(t, "1735689600", account.GetCredential("expires_at"))
}

func TestAdminServiceUpdateAccount_ClearsKiroModelMappingWithEmptyObject(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		accountsByID: map[int64]*Account{
			59: {
				ID:       59,
				Name:     "kiro-oauth",
				Platform: PlatformKiro,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"refresh_token": "rt-placeholder",
					"model_mapping": map[string]any{"claude-sonnet-4": "claude-sonnet-4"},
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.UpdateAccount(context.Background(), 59, &UpdateAccountInput{
		Credentials: map[string]any{
			"model_mapping": map[string]any{},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, "rt-placeholder", account.GetCredential("refresh_token"))
	require.Equal(t, map[string]any{}, account.Credentials["model_mapping"])
}

func TestAdminServiceBulkUpdateAccounts_RejectsInvalidMergedKiroCredentials(t *testing.T) {
	t.Parallel()

	repo := &kiroDefaultAccountRepoStub{
		getByIDsAccounts: []*Account{
			{
				ID:       77,
				Platform: PlatformKiro,
				Type:     AccountTypeAPIKey,
				Credentials: map[string]any{
					"api_key": "kiro-token",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	result, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs: []int64{77},
		Credentials: map[string]any{
			"api_key": "",
		},
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "kiro api_key is required")
	require.Empty(t, repo.bulkUpdateIDs)
}

func TestAdminServiceBulkUpdateAccounts_DoesNotMutateSharedKiroCredentialPayload(t *testing.T) {
	t.Parallel()

	inputCredentials := map[string]any{
		"region": "eu-west-1",
	}
	repo := &kiroDefaultAccountRepoStub{
		getByIDsAccounts: []*Account{
			{
				ID:       77,
				Platform: PlatformKiro,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"refresh_token": "first-refresh",
					"region":        "us-east-1",
				},
			},
			{
				ID:       78,
				Platform: PlatformKiro,
				Type:     AccountTypeOAuth,
				Credentials: map[string]any{
					"refresh_token": "second-refresh",
					"region":        "us-east-1",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	result, err := svc.BulkUpdateAccounts(context.Background(), &BulkUpdateAccountsInput{
		AccountIDs:  []int64{77, 78},
		Credentials: inputCredentials,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, repo.bulkUpdateIDs)
	require.Equal(t, map[string]any{"region": "eu-west-1"}, inputCredentials)
	require.Empty(t, repo.bulkUpdateCreds)
	require.Len(t, repo.updatedAccounts, 2)
	require.Equal(t, "first-refresh", repo.updatedAccounts[0].GetCredential("refresh_token"))
	require.Equal(t, "second-refresh", repo.updatedAccounts[1].GetCredential("refresh_token"))
	require.Equal(t, "eu-west-1", repo.updatedAccounts[0].GetCredential("region"))
	require.Equal(t, "eu-west-1", repo.updatedAccounts[1].GetCredential("region"))
}

func TestAdminServiceCreateAccount_ValidatesGroupsBeforePersist(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForAdminCreateValidation{}
	svc := &adminServiceImpl{
		accountRepo: repo,
		groupRepo:   &kiroDefaultGroupRepoStub{getErr: ErrGroupNotFound},
	}

	account, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:        "kiro-oauth",
		Platform:    PlatformKiro,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"refresh_token": "rt-placeholder"},
		GroupIDs:    []int64{404},
	})

	require.Nil(t, account)
	require.Error(t, err)
	require.False(t, repo.createCalled)
	require.False(t, repo.bindGroupsCalled)
}

func TestValidateAccountPlatform_RejectsUnknownPlatform(t *testing.T) {
	t.Parallel()

	err := validateAccountPlatform("not-a-real-platform")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported platform")
}

func TestValidateKiroCredentials_AcceptsUnixSecondsExpiresAt(t *testing.T) {
	t.Parallel()

	err := validateKiroCredentials(map[string]any{
		"refresh_token": "rt-placeholder",
		"expires_at":    time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)
}

func TestValidateKiroCredentials_RequiresIDCSecretsForAllIDCRefreshMethods(t *testing.T) {
	t.Parallel()

	for _, authMethod := range []string{"idc", "builder-id", "iam"} {
		err := validateKiroCredentials(map[string]any{
			"refresh_token": "rt-placeholder",
			"auth_method":   authMethod,
			"client_id":     "client-only",
		})
		require.Error(t, err, authMethod)
		require.Contains(t, err.Error(), "kiro idc client_id and client_secret are required", authMethod)
	}
}

func TestValidateKiroCredentials_AllowsExternalIDPWithoutClientSecret(t *testing.T) {
	t.Parallel()

	err := validateKiroCredentials(map[string]any{
		"refresh_token":  "rt-placeholder",
		"auth_method":    "external_idp",
		"client_id":      "microsoft-public-client",
		"token_endpoint": "https://login.microsoftonline.com/tenant/oauth2/v2.0/token",
		"profile_arn":    "arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD",
	})

	require.NoError(t, err)
}

func TestValidateKiroCredentials_RequiresExternalIDPClientID(t *testing.T) {
	t.Parallel()

	err := validateKiroCredentials(map[string]any{
		"refresh_token":  "rt-placeholder",
		"auth_method":    "ExternalIdp",
		"token_endpoint": "https://login.microsoftonline.com/tenant/oauth2/v2.0/token",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "kiro external_idp client_id is required")
}

func TestValidateKiroCredentials_ExternalIDPIssuerDiscoveryIsMicrosoftOnly(t *testing.T) {
	t.Parallel()

	err := validateKiroCredentials(map[string]any{
		"refresh_token": "rt-placeholder",
		"auth_method":   "ExternalIdp",
		"client_id":     "public-client",
		"issuer_url":    "https://issuer.example.com/tenant",
		"profile_arn":   "arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "kiro external_idp token_endpoint or Microsoft issuer_url is required")
}

func TestNormalizeKiroAuthMethod_InfersIDCFromClientCredentials(t *testing.T) {
	t.Parallel()

	got := NormalizeKiroAuthMethod(map[string]any{
		"refresh_token": "rt-placeholder",
		"client_id":     "client-id",
		"client_secret": "client-secret",
	})
	require.Equal(t, "idc", got)
}

func TestNormalizeKiroAuthMethod_ExplicitSocialOverridesClientCredentials(t *testing.T) {
	t.Parallel()

	got := NormalizeKiroAuthMethod(map[string]any{
		"refresh_token": "rt-placeholder",
		"auth_method":   "social",
		"client_id":     "client-id",
		"client_secret": "client-secret",
	})
	require.Equal(t, "social", got)
}
