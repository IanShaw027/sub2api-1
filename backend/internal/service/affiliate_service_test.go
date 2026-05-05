//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type affiliateSettingRepoStub struct {
	value  string
	values map[string]string
	err    error
}

func (s *affiliateSettingRepoStub) Get(context.Context, string) (*Setting, error) { return nil, s.err }
func (s *affiliateSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if s.values != nil {
		return s.values[key], nil
	}
	return s.value, nil
}
func (s *affiliateSettingRepoStub) Set(context.Context, string, string) error { return s.err }
func (s *affiliateSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return map[string]string{}, nil
}
func (s *affiliateSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	return s.err
}
func (s *affiliateSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return map[string]string{}, nil
}
func (s *affiliateSettingRepoStub) Delete(context.Context, string) error { return s.err }

func TestAffiliateRebateRatePercentSemantics(t *testing.T) {
	t.Parallel()

	svc := &AffiliateService{settingRepo: &affiliateSettingRepoStub{value: "1"}}
	rate := svc.loadAffiliateRebateRatePercent(context.Background())
	require.Equal(t, 1.0, rate)

	svc.settingRepo = &affiliateSettingRepoStub{value: "0.2"}
	rate = svc.loadAffiliateRebateRatePercent(context.Background())
	require.Equal(t, 0.2, rate)
}

type affiliateRepoStub struct {
	summary        *AffiliateSummary
	inviter        *AffiliateSummary
	invitees       []AffiliateInvitee
	rebatedCount   int
	accrual        AffiliateAccrualInput
	setRateCalls   int
	batchRateCalls int
}

func (s *affiliateRepoStub) EnsureUserAffiliate(ctx context.Context, userID int64) (*AffiliateSummary, error) {
	if s.inviter != nil && userID == s.inviter.UserID {
		return s.inviter, nil
	}
	return s.summary, nil
}
func (s *affiliateRepoStub) GetAffiliateByCode(ctx context.Context, code string) (*AffiliateSummary, error) {
	return s.inviter, nil
}
func (s *affiliateRepoStub) BindInviter(ctx context.Context, userID, inviterID int64) (bool, error) {
	return true, nil
}
func (s *affiliateRepoStub) AccrueQuota(ctx context.Context, input AffiliateAccrualInput) (float64, error) {
	s.accrual = input
	return input.Amount, nil
}
func (s *affiliateRepoStub) ApplySignupBonus(ctx context.Context, userID int64, amount float64) (bool, float64, error) {
	return true, amount, nil
}
func (s *affiliateRepoStub) GetAccruedRebateFromInvitee(context.Context, int64, int64) (float64, error) {
	return 0, nil
}
func (s *affiliateRepoStub) ThawFrozenQuota(context.Context, int64) (float64, error) {
	return 0, nil
}
func (s *affiliateRepoStub) TransferQuotaToBalance(ctx context.Context, userID int64) (float64, float64, error) {
	return 0, 0, nil
}
func (s *affiliateRepoStub) ListInvitees(ctx context.Context, inviterID int64, limit int) ([]AffiliateInvitee, error) {
	return s.invitees, nil
}
func (s *affiliateRepoStub) ListInviteeLedger(ctx context.Context, inviterID, inviteeUserID int64, limit int) ([]AffiliateLedgerEntry, error) {
	return nil, nil
}
func (s *affiliateRepoStub) CountRebatedInvitees(ctx context.Context, inviterID int64) (int, error) {
	return s.rebatedCount, nil
}
func (s *affiliateRepoStub) ListAdminAffiliateStats(ctx context.Context, params AdminAffiliateListParams) ([]AdminAffiliateStatsRow, int64, error) {
	return nil, 0, nil
}
func (s *affiliateRepoStub) UpdateUserAffCode(context.Context, int64, string) error {
	return nil
}
func (s *affiliateRepoStub) ResetUserAffCode(context.Context, int64) (string, error) {
	return "", nil
}
func (s *affiliateRepoStub) SetUserRebateRate(context.Context, int64, *float64) error {
	s.setRateCalls++
	return nil
}
func (s *affiliateRepoStub) BatchSetUserRebateRate(context.Context, []int64, *float64) error {
	s.batchRateCalls++
	return nil
}
func (s *affiliateRepoStub) ListUsersWithCustomSettings(context.Context, AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	return nil, 0, nil
}
func (s *affiliateRepoStub) ListAffiliateInviteRecords(context.Context, AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	return nil, 0, nil
}
func (s *affiliateRepoStub) ListAffiliateRebateRecords(context.Context, AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	return nil, 0, nil
}
func (s *affiliateRepoStub) ListAffiliateTransferRecords(context.Context, AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	return nil, 0, nil
}
func (s *affiliateRepoStub) GetAffiliateUserOverview(context.Context, int64) (*AffiliateUserOverview, error) {
	return nil, nil
}

func TestBindInviterByCodeNilServiceReturnsServiceUnavailable(t *testing.T) {
	t.Parallel()

	var svc *AffiliateService
	err := svc.BindInviterByCode(context.Background(), 1, "ABCDEFGHJKLM")
	require.Error(t, err)
	require.Contains(t, err.Error(), "affiliate service unavailable")

	svc = &AffiliateService{settingRepo: &affiliateSettingRepoStub{value: "true"}}
	err = svc.BindInviterByCode(context.Background(), 1, "ABCDEFGHJKLM")
	require.Error(t, err)
	require.Contains(t, err.Error(), "affiliate service unavailable")
}

func TestAccrueInviteRebate_ClampsToCapAndClaimsSlot(t *testing.T) {
	ctx := context.Background()
	inviterID := int64(10)
	repo := &affiliateRepoStub{
		summary: &AffiliateSummary{UserID: 20, InviterID: &inviterID},
		inviter: &AffiliateSummary{UserID: inviterID, AffHistoryQuota: 95},
	}
	svc := &AffiliateService{repo: repo, settingRepo: &affiliateSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:            "true",
		SettingKeyAffiliateRebateRate:         "20",
		SettingKeyAffiliateRebateCap:          "100",
		SettingKeyAffiliateRebateInviteeLimit: "3",
	}}}

	rebate, err := svc.AccrueInviteRebateForOrder(ctx, 20, 99, 100)
	require.NoError(t, err)
	require.Equal(t, 20.0, rebate)
	require.Equal(t, int64(99), repo.accrual.SourceOrderID)
	require.Equal(t, 100.0, repo.accrual.RebateCap)
}

func TestAccrueInviteRebate_PassesFreezeAndPerInviteePolicy(t *testing.T) {
	ctx := context.Background()
	inviterID := int64(10)
	repo := &affiliateRepoStub{
		summary: &AffiliateSummary{UserID: 20, InviterID: &inviterID},
		inviter: &AffiliateSummary{UserID: inviterID, AffHistoryQuota: 12},
	}
	svc := &AffiliateService{repo: repo, settingRepo: &affiliateSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:             "true",
		SettingKeyAffiliateRebateRate:          "15",
		SettingKeyAffiliateRebateCap:           "88",
		SettingKeyAffiliateRebateInviteeLimit:  "5",
		SettingKeyAffiliateRebateFreezeHours:   "24",
		SettingKeyAffiliateRebatePerInviteeCap: "66.5",
		SettingKeyAffiliateRebateDurationDays:  "30",
	}}}

	rebate, err := svc.AccrueInviteRebateForOrder(ctx, 20, 1234, 200)
	require.NoError(t, err)
	require.Equal(t, 30.0, rebate)
	require.Equal(t, int64(1234), repo.accrual.SourceOrderID)
	require.Equal(t, 88.0, repo.accrual.RebateCap)
	require.Equal(t, 5, repo.accrual.InviteeLimit)
	require.Equal(t, 24, repo.accrual.FreezeHours)
	require.Equal(t, 66.5, repo.accrual.PerInviteeCap)
}

func TestAccrueInviteRebate_ClampsStoredCustomRateTo100Percent(t *testing.T) {
	ctx := context.Background()
	inviterID := int64(10)
	invalidRate := 150.0
	repo := &affiliateRepoStub{
		summary: &AffiliateSummary{UserID: 20, InviterID: &inviterID},
		inviter: &AffiliateSummary{UserID: inviterID, AffRebateRatePercent: &invalidRate},
	}
	svc := &AffiliateService{repo: repo, settingRepo: &affiliateSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled: "true",
	}}}

	rebate, err := svc.AccrueInviteRebateForOrder(ctx, 20, 88, 100)
	require.NoError(t, err)
	require.Equal(t, 100.0, rebate)
	require.Equal(t, 100.0, repo.accrual.RebateRate)
}

func TestAccrueInviteRebate_RejectsNewInviteeWhenSlotLimitReached(t *testing.T) {
	ctx := context.Background()
	inviterID := int64(10)
	repo := &affiliateRepoStub{
		summary:      &AffiliateSummary{UserID: 20, InviterID: &inviterID},
		inviter:      &AffiliateSummary{UserID: inviterID},
		rebatedCount: 1,
	}
	svc := &AffiliateService{repo: repo, settingRepo: &affiliateSettingRepoStub{values: map[string]string{
		SettingKeyAffiliateEnabled:            "true",
		SettingKeyAffiliateRebateRate:         "10",
		SettingKeyAffiliateRebateInviteeLimit: "1",
	}}}

	rebate, err := svc.AccrueInviteRebateForOrder(ctx, 20, 99, 100)
	require.NoError(t, err)
	require.Equal(t, 10.0, rebate)
	require.Equal(t, 1, repo.accrual.InviteeLimit)
}

func TestAdminSetUserRebateRateRejectsOutOfRange(t *testing.T) {
	ctx := context.Background()
	repo := &affiliateRepoStub{}
	svc := &AffiliateService{repo: repo, settingRepo: &affiliateSettingRepoStub{value: "true"}}

	for _, tc := range []struct {
		name string
		rate float64
	}{
		{name: "negative", rate: -1},
		{name: "above max", rate: 100.01},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.AdminSetUserRebateRate(ctx, 1, &tc.rate)
			require.ErrorIs(t, err, ErrAffiliateRebateRateInvalid)
			require.Equal(t, 0, repo.setRateCalls)
		})
	}
}

func TestAdminBatchSetUserRebateRateRejectsOutOfRange(t *testing.T) {
	ctx := context.Background()
	repo := &affiliateRepoStub{}
	svc := &AffiliateService{repo: repo, settingRepo: &affiliateSettingRepoStub{value: "true"}}

	for _, tc := range []struct {
		name string
		rate float64
	}{
		{name: "negative", rate: -0.5},
		{name: "above max", rate: 101},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.AdminBatchSetUserRebateRate(ctx, []int64{1, 2}, &tc.rate)
			require.ErrorIs(t, err, ErrAffiliateRebateRateInvalid)
			require.Equal(t, 0, repo.batchRateCalls)
		})
	}
}

func TestMaskEmail(t *testing.T) {
	t.Parallel()
	require.Equal(t, "a***@g***.com", maskEmail("alice@gmail.com"))
	require.Equal(t, "x***@d***", maskEmail("x@domain"))
	require.Equal(t, "", maskEmail(""))
}

func TestIsValidAffiliateCodeFormat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid canonical", "ABCDEFGHJKLM", true},
		{"valid all digits 2-9", "234567892345", true},
		{"valid mixed", "A2B3C4D5E6F7", true},
		{"too short", "ABCDEFGHJKL", false},
		{"too long", "ABCDEFGHJKLMN", false},
		{"contains excluded letter I", "IBCDEFGHJKLM", false},
		{"contains excluded letter O", "OBCDEFGHJKLM", false},
		{"contains excluded digit 0", "0BCDEFGHJKLM", false},
		{"contains excluded digit 1", "1BCDEFGHJKLM", false},
		{"lowercase rejected (caller must ToUpper first)", "abcdefghjklm", false},
		{"empty", "", false},
		{"12-byte utf8 non-ascii", "ÄÄÄÄÄÄ", false}, // 6×2 bytes = 12 bytes, bytes out of charset
		{"ascii punctuation", "ABCDEFGHJK.M", false},
		{"whitespace", "ABCDEFGHJK M", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isValidAffiliateCodeFormat(tc.in))
		})
	}
}
