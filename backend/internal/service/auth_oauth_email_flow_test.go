//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type redeemCodeRepoStub struct {
	codesByCode map[string]*RedeemCode
	useErr      error
	useCalls    []struct {
		id     int64
		userID int64
	}
	updateCalls []*RedeemCode
}

type oauthEmailFlowAffiliateRepoStub struct {
	selfSummary           *AffiliateSummary
	inviterSummary        *AffiliateSummary
	lookupCode            string
	bindCalls             []struct{ userID, inviterID int64 }
	applySignupBonusCalls []struct {
		userID int64
		amount float64
	}
}

func (s *oauthEmailFlowAffiliateRepoStub) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	if s.selfSummary != nil && s.selfSummary.UserID == userID {
		return s.selfSummary, nil
	}
	if s.inviterSummary != nil && s.inviterSummary.UserID == userID {
		return s.inviterSummary, nil
	}
	return &AffiliateSummary{UserID: userID}, nil
}

func (s *oauthEmailFlowAffiliateRepoStub) GetAffiliateByCode(_ context.Context, code string) (*AffiliateSummary, error) {
	s.lookupCode = code
	if s.inviterSummary == nil {
		return nil, ErrAffiliateProfileNotFound
	}
	return s.inviterSummary, nil
}

func (s *oauthEmailFlowAffiliateRepoStub) BindInviter(_ context.Context, userID, inviterID int64) (bool, error) {
	s.bindCalls = append(s.bindCalls, struct{ userID, inviterID int64 }{userID: userID, inviterID: inviterID})
	if s.selfSummary != nil && s.selfSummary.UserID == userID {
		s.selfSummary.InviterID = &inviterID
	}
	return true, nil
}

func (s *oauthEmailFlowAffiliateRepoStub) AccrueQuota(context.Context, AffiliateAccrualInput) (float64, error) {
	panic("unexpected AccrueQuota call")
}

func (s *oauthEmailFlowAffiliateRepoStub) ApplySignupBonus(_ context.Context, userID int64, amount float64) (bool, float64, error) {
	s.applySignupBonusCalls = append(s.applySignupBonusCalls, struct {
		userID int64
		amount float64
	}{userID: userID, amount: amount})
	return true, amount, nil
}

func (s *oauthEmailFlowAffiliateRepoStub) GetAccruedRebateFromInvitee(context.Context, int64, int64) (float64, error) {
	panic("unexpected GetAccruedRebateFromInvitee call")
}

func (s *oauthEmailFlowAffiliateRepoStub) ThawFrozenQuota(context.Context, int64) (float64, error) {
	panic("unexpected ThawFrozenQuota call")
}

func (s *oauthEmailFlowAffiliateRepoStub) TransferQuotaToBalance(context.Context, int64) (float64, float64, error) {
	panic("unexpected TransferQuotaToBalance call")
}

func (s *oauthEmailFlowAffiliateRepoStub) ListInvitees(context.Context, int64, int) ([]AffiliateInvitee, error) {
	panic("unexpected ListInvitees call")
}

func (s *oauthEmailFlowAffiliateRepoStub) ListInviteeLedger(context.Context, int64, int64, int) ([]AffiliateLedgerEntry, error) {
	panic("unexpected ListInviteeLedger call")
}

func (s *oauthEmailFlowAffiliateRepoStub) CountRebatedInvitees(context.Context, int64) (int, error) {
	panic("unexpected CountRebatedInvitees call")
}

func (s *oauthEmailFlowAffiliateRepoStub) ListAdminAffiliateStats(context.Context, AdminAffiliateListParams) ([]AdminAffiliateStatsRow, int64, error) {
	panic("unexpected ListAdminAffiliateStats call")
}

func (s *oauthEmailFlowAffiliateRepoStub) UpdateUserAffCode(context.Context, int64, string) error {
	panic("unexpected UpdateUserAffCode call")
}

func (s *oauthEmailFlowAffiliateRepoStub) ResetUserAffCode(context.Context, int64) (string, error) {
	panic("unexpected ResetUserAffCode call")
}

func (s *oauthEmailFlowAffiliateRepoStub) SetUserRebateRate(context.Context, int64, *float64) error {
	panic("unexpected SetUserRebateRate call")
}

func (s *oauthEmailFlowAffiliateRepoStub) BatchSetUserRebateRate(context.Context, []int64, *float64) error {
	panic("unexpected BatchSetUserRebateRate call")
}

func (s *oauthEmailFlowAffiliateRepoStub) ListUsersWithCustomSettings(context.Context, AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	panic("unexpected ListUsersWithCustomSettings call")
}

func (s *oauthEmailFlowAffiliateRepoStub) ListAffiliateInviteRecords(context.Context, AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	panic("unexpected ListAffiliateInviteRecords call")
}

func (s *oauthEmailFlowAffiliateRepoStub) ListAffiliateRebateRecords(context.Context, AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	panic("unexpected ListAffiliateRebateRecords call")
}

func (s *oauthEmailFlowAffiliateRepoStub) ListAffiliateTransferRecords(context.Context, AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	panic("unexpected ListAffiliateTransferRecords call")
}

func (s *oauthEmailFlowAffiliateRepoStub) GetAffiliateUserOverview(context.Context, int64) (*AffiliateUserOverview, error) {
	panic("unexpected GetAffiliateUserOverview call")
}

func (s *redeemCodeRepoStub) Create(context.Context, *RedeemCode) error {
	panic("unexpected Create call")
}

func (s *redeemCodeRepoStub) CreateBatch(context.Context, []RedeemCode) error {
	panic("unexpected CreateBatch call")
}

func (s *redeemCodeRepoStub) GetByID(context.Context, int64) (*RedeemCode, error) {
	panic("unexpected GetByID call")
}

func (s *redeemCodeRepoStub) GetByCode(_ context.Context, code string) (*RedeemCode, error) {
	if s.codesByCode == nil {
		return nil, ErrRedeemCodeNotFound
	}
	redeemCode, ok := s.codesByCode[code]
	if !ok {
		return nil, ErrRedeemCodeNotFound
	}
	cloned := *redeemCode
	return &cloned, nil
}

func (s *redeemCodeRepoStub) Update(_ context.Context, code *RedeemCode) error {
	if code == nil {
		return nil
	}
	cloned := *code
	s.updateCalls = append(s.updateCalls, &cloned)
	if s.codesByCode == nil {
		s.codesByCode = make(map[string]*RedeemCode)
	}
	s.codesByCode[cloned.Code] = &cloned
	return nil
}

func (s *redeemCodeRepoStub) BatchUpdate(context.Context, []int64, RedeemCodeBatchUpdateFields) (int64, error) {
	panic("unexpected BatchUpdate call")
}

func (s *redeemCodeRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}

func (s *redeemCodeRepoStub) Use(_ context.Context, id, userID int64) error {
	if s.useErr != nil {
		return s.useErr
	}
	for code, redeemCode := range s.codesByCode {
		if redeemCode.ID != id {
			continue
		}
		now := time.Now().UTC()
		redeemCode.Status = StatusUsed
		redeemCode.UsedBy = &userID
		redeemCode.UsedAt = &now
		s.codesByCode[code] = redeemCode
		s.useCalls = append(s.useCalls, struct {
			id     int64
			userID int64
		}{id: id, userID: userID})
		return nil
	}
	return ErrRedeemCodeNotFound
}

func (s *redeemCodeRepoStub) List(context.Context, pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *redeemCodeRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *redeemCodeRepoStub) ListByUser(context.Context, int64, int) ([]RedeemCode, error) {
	panic("unexpected ListByUser call")
}

func (s *redeemCodeRepoStub) ListByUserPaginated(context.Context, int64, pagination.PaginationParams, string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserPaginated call")
}

func (s *redeemCodeRepoStub) SumPositiveBalanceByUser(context.Context, int64) (float64, error) {
	panic("unexpected SumPositiveBalanceByUser call")
}

func (s *redeemCodeRepoStub) GetStats(context.Context) (*RedeemCodeStats, error) {
	panic("unexpected GetStats call")
}

func newOAuthEmailFlowAuthService(
	userRepo UserRepository,
	redeemRepo RedeemCodeRepository,
	refreshTokenCache RefreshTokenCache,
	settings map[string]string,
	emailCache EmailCache,
	quotaRepo UserPlatformQuotaRepository, // 新增
) *AuthService {
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                   "test-secret",
			ExpireHour:               1,
			AccessTokenExpireMinutes: 60,
			RefreshTokenExpireDays:   7,
		},
		Default: config.DefaultConfig{
			UserBalance:     3.5,
			UserConcurrency: 2,
		},
	}

	settingService := NewSettingService(&settingRepoStub{values: settings}, cfg)
	emailService := NewEmailService(&settingRepoStub{values: settings}, emailCache)

	return NewAuthService(
		nil,
		userRepo,
		redeemRepo,
		refreshTokenCache,
		cfg,
		settingService,
		emailService,
		nil,
		nil,
		nil,
		nil,
		nil,
		quotaRepo, // 替换原来的 nil
	)
}

func TestRegisterOAuthEmailAccountRollsBackCreatedUserWhenTokenPairGenerationFails(t *testing.T) {
	userRepo := &userRepoStub{nextID: 42}
	redeemRepo := &redeemCodeRepoStub{
		codesByCode: map[string]*RedeemCode{
			"INVITE123": {
				ID:     7,
				Code:   "INVITE123",
				Type:   RedeemTypeInvitation,
				Status: StatusUnused,
			},
		},
	}
	emailCache := &emailCacheStub{
		data: &VerificationCodeData{
			Code:      "246810",
			Attempts:  0,
			CreatedAt: time.Now().UTC(),
			ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
		},
	}
	authService := newOAuthEmailFlowAuthService(
		userRepo,
		redeemRepo,
		nil,
		map[string]string{
			SettingKeyRegistrationEnabled:   "true",
			SettingKeyInvitationCodeEnabled: "true",
			SettingKeyEmailVerifyEnabled:    "true",
		},
		emailCache,
		nil,
	)

	tokenPair, user, err := authService.RegisterOAuthEmailAccount(
		context.Background(),
		"fresh@example.com",
		"secret-123",
		"246810",
		"INVITE123",
		"oidc",
	)

	require.Nil(t, tokenPair)
	require.Nil(t, user)
	require.Error(t, err)
	require.Contains(t, err.Error(), "generate token pair")
	require.Equal(t, []int64{42}, userRepo.deletedIDs)
	require.Len(t, userRepo.created, 1)
	require.Empty(t, redeemRepo.useCalls)
	require.Empty(t, redeemRepo.updateCalls)
}

func TestRegisterOAuthEmailAccountSetsNormalizedSignupSourceOnCreatedUser(t *testing.T) {
	userRepo := &userRepoStub{nextID: 42}
	emailCache := &emailCacheStub{
		data: &VerificationCodeData{
			Code:      "246810",
			Attempts:  0,
			CreatedAt: time.Now().UTC(),
			ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
		},
	}
	authService := newOAuthEmailFlowAuthService(
		userRepo,
		&redeemCodeRepoStub{},
		&refreshTokenCacheStub{},
		map[string]string{
			SettingKeyRegistrationEnabled: "true",
			SettingKeyEmailVerifyEnabled:  "true",
		},
		emailCache,
		nil,
	)

	tokenPair, user, err := authService.RegisterOAuthEmailAccount(
		context.Background(),
		"fresh@example.com",
		"secret-123",
		"246810",
		"",
		" OIDC ",
	)

	require.NoError(t, err)
	require.NotNil(t, tokenPair)
	require.NotNil(t, user)
	require.Len(t, userRepo.created, 1)
	require.Equal(t, "oidc", userRepo.created[0].SignupSource)
}

func TestRegisterOAuthEmailAccountKeepsGitHubAndGoogleSignupSource(t *testing.T) {
	tests := []struct {
		name         string
		email        string
		signupSource string
		want         string
	}{
		{
			name:         "github",
			email:        "github@example.com",
			signupSource: " GitHub ",
			want:         "github",
		},
		{
			name:         "google",
			email:        "google@example.com",
			signupSource: " Google ",
			want:         "google",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &userRepoStub{nextID: 43}
			emailCache := &emailCacheStub{
				data: &VerificationCodeData{
					Code:      "246810",
					Attempts:  0,
					CreatedAt: time.Now().UTC(),
					ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
				},
			}
			authService := newOAuthEmailFlowAuthService(
				userRepo,
				&redeemCodeRepoStub{},
				&refreshTokenCacheStub{},
				map[string]string{
					SettingKeyRegistrationEnabled: "true",
					SettingKeyEmailVerifyEnabled:  "true",
				},
				emailCache,
				nil,
			)

			tokenPair, user, err := authService.RegisterOAuthEmailAccount(
				context.Background(),
				tt.email,
				"secret-123",
				"246810",
				"",
				tt.signupSource,
			)

			require.NoError(t, err)
			require.NotNil(t, tokenPair)
			require.NotNil(t, user)
			require.Len(t, userRepo.created, 1)
			require.Equal(t, tt.want, userRepo.created[0].SignupSource)
		})
	}
}

func TestRegisterOAuthEmailAccountFallsBackUnknownSignupSourceToEmail(t *testing.T) {
	userRepo := &userRepoStub{nextID: 43}
	emailCache := &emailCacheStub{
		data: &VerificationCodeData{
			Code:      "246810",
			Attempts:  0,
			CreatedAt: time.Now().UTC(),
			ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
		},
	}
	authService := newOAuthEmailFlowAuthService(
		userRepo,
		&redeemCodeRepoStub{},
		&refreshTokenCacheStub{},
		map[string]string{
			SettingKeyRegistrationEnabled: "true",
			SettingKeyEmailVerifyEnabled:  "true",
		},
		emailCache,
		nil,
	)

	tokenPair, user, err := authService.RegisterOAuthEmailAccount(
		context.Background(),
		"fallback@example.com",
		"secret-123",
		"246810",
		"",
		"unknown-provider",
	)

	require.NoError(t, err)
	require.NotNil(t, tokenPair)
	require.NotNil(t, user)
	require.Len(t, userRepo.created, 1)
	require.Equal(t, "email", userRepo.created[0].SignupSource)
}

func TestFinalizeOAuthEmailAccountBindsTrimmedAffiliateCodeAndSignupBonus(t *testing.T) {
	affiliateRepo := &oauthEmailFlowAffiliateRepoStub{
		selfSummary:    &AffiliateSummary{UserID: 42},
		inviterSummary: &AffiliateSummary{UserID: 7, AffCode: "ABCDEFGH2345"},
	}
	authService := newOAuthEmailFlowAuthService(
		&userRepoStub{},
		&redeemCodeRepoStub{},
		&refreshTokenCacheStub{},
		map[string]string{},
		&emailCacheStub{},
		nil,
	)
	authService.affiliateService = NewAffiliateService(affiliateRepo, &settingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:     "true",
		SettingKeyAffiliateSignupBonus: "1.5",
	}}, nil, nil)

	user := &User{ID: 42, Balance: 10}
	err := authService.FinalizeOAuthEmailAccount(context.Background(), user, "", " abcdefgh2345 ", " OIDC ")

	require.NoError(t, err)
	require.Equal(t, "ABCDEFGH2345", affiliateRepo.lookupCode)
	require.Len(t, affiliateRepo.bindCalls, 1)
	require.Equal(t, int64(42), affiliateRepo.bindCalls[0].userID)
	require.Equal(t, int64(7), affiliateRepo.bindCalls[0].inviterID)
	require.Len(t, affiliateRepo.applySignupBonusCalls, 1)
	require.Equal(t, 1.5, affiliateRepo.applySignupBonusCalls[0].amount)
	require.Equal(t, 11.5, user.Balance)
}

func TestRollbackOAuthEmailAccountCreationRestoresInvitationUsage(t *testing.T) {
	userRepo := &userRepoStub{}
	redeemRepo := &redeemCodeRepoStub{
		codesByCode: map[string]*RedeemCode{
			"INVITE123": {
				ID:     7,
				Code:   "INVITE123",
				Type:   RedeemTypeInvitation,
				Status: StatusUsed,
				UsedBy: func() *int64 {
					v := int64(42)
					return &v
				}(),
				UsedAt: func() *time.Time {
					v := time.Now().UTC()
					return &v
				}(),
			},
		},
	}
	authService := newOAuthEmailFlowAuthService(
		userRepo,
		redeemRepo,
		&refreshTokenCacheStub{},
		map[string]string{
			SettingKeyRegistrationEnabled:   "true",
			SettingKeyInvitationCodeEnabled: "true",
		},
		&emailCacheStub{},
		nil,
	)

	err := authService.RollbackOAuthEmailAccountCreation(context.Background(), 42, "INVITE123")

	require.NoError(t, err)
	require.Equal(t, []int64{42}, userRepo.deletedIDs)
	require.Len(t, redeemRepo.updateCalls, 1)
	require.Equal(t, StatusUnused, redeemRepo.updateCalls[0].Status)
	require.Nil(t, redeemRepo.updateCalls[0].UsedBy)
	require.Nil(t, redeemRepo.updateCalls[0].UsedAt)
}

func TestRollbackOAuthEmailAccountCreationPropagatesDeleteError(t *testing.T) {
	userRepo := &userRepoStub{deleteErr: errors.New("delete failed")}
	authService := newOAuthEmailFlowAuthService(
		userRepo,
		&redeemCodeRepoStub{},
		&refreshTokenCacheStub{},
		map[string]string{
			SettingKeyRegistrationEnabled: "true",
		},
		&emailCacheStub{},
		nil,
	)

	err := authService.RollbackOAuthEmailAccountCreation(context.Background(), 42, "")

	require.Error(t, err)
	require.Contains(t, err.Error(), "delete created oauth user")
}

func TestFinalizeOAuthEmailAccount_SnapshotsPlatformQuotaDefaults(t *testing.T) {
	userRepo := &userRepoStub{nextID: 99}
	quotaRepo := &userPlatformQuotaRepoStub{}

	authService := newOAuthEmailFlowAuthService(
		userRepo,
		nil,
		&refreshTokenCacheStub{},
		map[string]string{
			SettingKeyRegistrationEnabled:                "true",
			SettingKeyEmailVerifyEnabled:                 "true",
			SettingKeyDefaultPlatformQuotas:              `{"anthropic": {"daily": 5.5}}`,
			SettingKeyAuthSourceDefaultOIDCGrantOnSignup: "true",
			SettingKeyAuthSourcePlatformQuotas("oidc"):   `{"anthropic": {"monthly": 17.25}}`,
		},
		&emailCacheStub{},
		quotaRepo,
	)

	user := &User{
		ID:           99,
		Email:        "newuser@example.com",
		Role:         RoleUser,
		Status:       StatusActive,
		SignupSource: "oidc",
	}

	err := authService.FinalizeOAuthEmailAccount(
		context.Background(),
		user,
		"",
		"",
		"oidc",
	)

	require.NoError(t, err)

	require.Len(t, quotaRepo.bulkInsertCalls, 1, "snapshotPlatformQuotaDefaults must call BulkInsertInitial once on successful OAuth signup")

	records := quotaRepo.bulkInsertCalls[0]
	var anthropicRecord *UserPlatformQuotaRecord
	for i := range records {
		if records[i].Platform == "anthropic" {
			anthropicRecord = &records[i]
			break
		}
	}
	require.NotNil(t, anthropicRecord, "expected anthropic platform record")
	require.Equal(t, int64(99), anthropicRecord.UserID)
	require.NotNil(t, anthropicRecord.DailyLimitUSD)
	require.InDelta(t, 5.5, *anthropicRecord.DailyLimitUSD, 0.0001)
	require.NotNil(t, anthropicRecord.MonthlyLimitUSD, "expected auth-source quota override to be merged")
	require.InDelta(t, 17.25, *anthropicRecord.MonthlyLimitUSD, 0.0001)
}
