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
	summary      *AffiliateSummary
	inviter      *AffiliateSummary
	invitees     []AffiliateInvitee
	rebatedCount int
	accrual      AffiliateAccrualInput
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

func TestMaskEmail(t *testing.T) {
	t.Parallel()
	require.Equal(t, "a***@g***.com", maskEmail("alice@gmail.com"))
	require.Equal(t, "x***@d***", maskEmail("x@domain"))
	require.Equal(t, "", maskEmail(""))
}
