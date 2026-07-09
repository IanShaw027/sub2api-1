package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var (
	ErrAffiliateProfileNotFound         = infraerrors.NotFound("AFFILIATE_PROFILE_NOT_FOUND", "affiliate profile not found")
	ErrAffiliateCodeInvalid             = infraerrors.BadRequest("AFFILIATE_CODE_INVALID", "invalid affiliate code")
	ErrAffiliateCodeTaken               = infraerrors.Conflict("AFFILIATE_CODE_TAKEN", "affiliate code already in use")
	ErrAffiliateAlreadyBound            = infraerrors.Conflict("AFFILIATE_ALREADY_BOUND", "affiliate inviter already bound")
	ErrAffiliateQuotaEmpty              = infraerrors.BadRequest("AFFILIATE_QUOTA_EMPTY", "no affiliate quota available to transfer")
	ErrAffiliateRebateRateInvalid       = infraerrors.BadRequest("AFFILIATE_REBATE_RATE_INVALID", "rebate rate must be between 0 and 100")
	ErrAffiliateCreatorQuotaUnavailable = infraerrors.ServiceUnavailable("AFFILIATE_CREATOR_QUOTA_UNAVAILABLE", "affiliate creator quota service unavailable")
)

const (
	affiliateInviteesLimit = 100
	// affiliateCodeFormatLength must stay in sync with repository.affiliateCodeLength.
	affiliateCodeFormatLength = 12
)

// affiliateCodeValidChar mirrors the repository affiliate-code charset:
// A-Z excluding I/O, plus digits 2-9.
var affiliateCodeValidChar = func() [256]bool {
	var tbl [256]bool
	for _, c := range []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789") {
		tbl[c] = true
	}
	return tbl
}()

func isValidAffiliateCodeFormat(code string) bool {
	if len(code) != affiliateCodeFormatLength {
		return false
	}
	for i := 0; i < len(code); i++ {
		if !affiliateCodeValidChar[code[i]] {
			return false
		}
	}
	return true
}

type AffiliateSummary struct {
	UserID               int64     `json:"user_id"`
	AffCode              string    `json:"aff_code"`
	AffCodeCustom        bool      `json:"aff_code_custom"`
	AffRebateRatePercent *float64  `json:"aff_rebate_rate_percent,omitempty"`
	InviterID            *int64    `json:"inviter_id,omitempty"`
	AffCount             int       `json:"aff_count"`
	AffQuota             float64   `json:"aff_quota"`
	AffFrozenQuota       float64   `json:"aff_frozen_quota"`
	AffHistoryQuota      float64   `json:"aff_history_quota"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type AffiliateInvitee struct {
	UserID            int64      `json:"user_id"`
	Email             string     `json:"email"`
	Username          string     `json:"username"`
	CreatedAt         *time.Time `json:"created_at,omitempty"`
	TotalRebate       float64    `json:"total_rebate"`
	TotalConsumed     float64    `json:"total_consumed"`
	RebateSlotClaimed bool       `json:"rebate_slot_claimed"`
}

type AffiliatePolicy struct {
	Enabled       bool    `json:"enabled"`
	RebateRate    float64 `json:"rebate_rate"`
	RebateCap     float64 `json:"rebate_cap"`
	InviteeLimit  int     `json:"invitee_limit"`
	FreezeHours   int     `json:"freeze_hours"`
	DurationDays  int     `json:"duration_days"`
	PerInviteeCap float64 `json:"per_invitee_cap"`
	SignupBonus   float64 `json:"signup_bonus"`
	PolicyText    string  `json:"policy_text"`
}

type AffiliateLedgerEntry struct {
	ID                 int64     `json:"id"`
	CreatedAt          time.Time `json:"created_at"`
	Action             string    `json:"action"`
	Amount             float64   `json:"amount"`
	SourceUserID       *int64    `json:"source_user_id,omitempty"`
	SourceOrderID      *int64    `json:"source_order_id,omitempty"`
	BaseAmount         float64   `json:"base_amount"`
	RebateRate         float64   `json:"rebate_rate"`
	InviteeSlotClaimed bool      `json:"invitee_slot_claimed"`
}

type AffiliateAccrualInput struct {
	InviterID     int64
	InviteeUserID int64
	Amount        float64
	BaseAmount    float64
	RebateRate    float64
	SourceOrderID int64
	RebateCap     float64
	InviteeLimit  int
	FreezeHours   int
	PerInviteeCap float64
}

// AffiliateReversalInput drives the reversal (clawback) of rebate previously
// accrued for an order that is being refunded.
//
// RefundedAmount is the *cumulative* refunded amount on the order (not the
// incremental amount of this single refund call). The repository computes the
// proportional target reversal as accrued * RefundedAmount/OrderAmount and only
// claws back the delta beyond what has already been reversed for the order. This
// makes the operation idempotent across retries and correct across repeated
// partial refunds.
type AffiliateReversalInput struct {
	SourceOrderID  int64
	RefundedAmount float64
	OrderAmount    float64
}

type AffiliateDetail struct {
	UserID                     int64              `json:"user_id"`
	AffCode                    string             `json:"aff_code"`
	AffCodeCustom              bool               `json:"aff_code_custom"`
	AffRebateRatePercent       *float64           `json:"aff_rebate_rate_percent,omitempty"`
	InviterID                  *int64             `json:"inviter_id,omitempty"`
	AffCount                   int                `json:"aff_count"`
	AffQuota                   float64            `json:"aff_quota"`
	AffFrozenQuota             float64            `json:"aff_frozen_quota"`
	AffHistoryQuota            float64            `json:"aff_history_quota"`
	InvitedCount               int                `json:"invited_count"`
	RebatedInviteeCount        int                `json:"rebated_invitee_count"`
	RemainingRebateSlots       *int               `json:"remaining_rebate_slots,omitempty"`
	EffectiveRebateRatePercent float64            `json:"effective_rebate_rate_percent"`
	Policy                     AffiliatePolicy    `json:"policy"`
	Invitees                   []AffiliateInvitee `json:"invitees"`
}

type AffiliateAdminFilter struct {
	Search   string
	Page     int
	PageSize int
}

type AffiliateAdminEntry struct {
	UserID               int64    `json:"user_id"`
	Email                string   `json:"email"`
	Username             string   `json:"username"`
	AffCode              string   `json:"aff_code"`
	AffCodeCustom        bool     `json:"aff_code_custom"`
	AffRebateRatePercent *float64 `json:"aff_rebate_rate_percent,omitempty"`
	AffCount             int      `json:"aff_count"`
}

type AffiliateRecordFilter struct {
	Search   string
	Page     int
	PageSize int
	StartAt  *time.Time
	EndAt    *time.Time
	SortBy   string
	SortDesc bool
}

type AffiliateInviteRecord struct {
	InviterID       int64     `json:"inviter_id"`
	InviterEmail    string    `json:"inviter_email"`
	InviterUsername string    `json:"inviter_username"`
	InviteeID       int64     `json:"invitee_id"`
	InviteeEmail    string    `json:"invitee_email"`
	InviteeUsername string    `json:"invitee_username"`
	AffCode         string    `json:"aff_code"`
	TotalRebate     float64   `json:"total_rebate"`
	CreatedAt       time.Time `json:"created_at"`
}

type AffiliateRebateRecord struct {
	OrderID         int64     `json:"order_id"`
	OutTradeNo      string    `json:"out_trade_no"`
	InviterID       int64     `json:"inviter_id"`
	InviterEmail    string    `json:"inviter_email"`
	InviterUsername string    `json:"inviter_username"`
	InviteeID       int64     `json:"invitee_id"`
	InviteeEmail    string    `json:"invitee_email"`
	InviteeUsername string    `json:"invitee_username"`
	OrderAmount     float64   `json:"order_amount"`
	PayAmount       float64   `json:"pay_amount"`
	Currency        string    `json:"currency,omitempty"`
	RebateAmount    float64   `json:"rebate_amount"`
	PaymentType     string    `json:"payment_type"`
	OrderStatus     string    `json:"order_status"`
	CreatedAt       time.Time `json:"created_at"`
}

type AffiliateTransferRecord struct {
	LedgerID            int64     `json:"ledger_id"`
	UserID              int64     `json:"user_id"`
	UserEmail           string    `json:"user_email"`
	Username            string    `json:"username"`
	Amount              float64   `json:"amount"`
	BalanceAfter        *float64  `json:"balance_after,omitempty"`
	AvailableQuotaAfter *float64  `json:"available_quota_after,omitempty"`
	FrozenQuotaAfter    *float64  `json:"frozen_quota_after,omitempty"`
	HistoryQuotaAfter   *float64  `json:"history_quota_after,omitempty"`
	SnapshotAvailable   bool      `json:"snapshot_available"`
	CurrentBalance      float64   `json:"-"`
	RemainingQuota      float64   `json:"-"`
	FrozenQuota         float64   `json:"-"`
	HistoryQuota        float64   `json:"-"`
	CreatedAt           time.Time `json:"created_at"`
}

type AffiliateUserOverview struct {
	UserID              int64   `json:"user_id"`
	Email               string  `json:"email"`
	Username            string  `json:"username"`
	AffCode             string  `json:"aff_code"`
	RebateRatePercent   float64 `json:"rebate_rate_percent"`
	RebateRateCustom    bool    `json:"-"`
	InvitedCount        int     `json:"invited_count"`
	RebatedInviteeCount int     `json:"rebated_invitee_count"`
	AvailableQuota      float64 `json:"available_quota"`
	HistoryQuota        float64 `json:"history_quota"`
}

type AdminAffiliateListParams struct {
	Page     int
	PageSize int
	Search   string
	StartAt  *time.Time
	EndAt    *time.Time
}

type AdminAffiliateStatsRow struct {
	UserID              int64     `json:"user_id"`
	Email               string    `json:"email"`
	Username            string    `json:"username"`
	AffCode             string    `json:"aff_code"`
	InviterID           *int64    `json:"inviter_id,omitempty"`
	AffCount            int       `json:"aff_count"`
	AffQuota            float64   `json:"aff_quota"`
	AffHistoryQuota     float64   `json:"aff_history_quota"`
	RebatedInviteeCount int       `json:"rebated_invitee_count"`
	PeriodInvitedCount  int       `json:"period_invited_count"`
	PeriodRebateAmount  float64   `json:"period_rebate_amount"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type AffiliateRepository interface {
	EnsureUserAffiliate(ctx context.Context, userID int64) (*AffiliateSummary, error)
	GetAffiliateByCode(ctx context.Context, code string) (*AffiliateSummary, error)
	BindInviter(ctx context.Context, userID, inviterID int64) (bool, error)
	AccrueQuota(ctx context.Context, input AffiliateAccrualInput) (float64, error)
	ReverseQuotaForOrder(ctx context.Context, input AffiliateReversalInput) (reversed float64, inviterID int64, err error)
	ApplySignupBonus(ctx context.Context, userID int64, amount float64) (bool, float64, error)
	GetAccruedRebateFromInvitee(ctx context.Context, inviterID, inviteeUserID int64) (float64, error)
	ThawFrozenQuota(ctx context.Context, userID int64) (float64, error)
	TransferQuotaToBalance(ctx context.Context, userID int64) (float64, float64, error)
	ListInvitees(ctx context.Context, inviterID int64, limit int) ([]AffiliateInvitee, error)
	ListInviteeLedger(ctx context.Context, inviterID, inviteeUserID int64, limit int) ([]AffiliateLedgerEntry, error)
	CountRebatedInvitees(ctx context.Context, inviterID int64) (int, error)
	ListAdminAffiliateStats(ctx context.Context, params AdminAffiliateListParams) ([]AdminAffiliateStatsRow, int64, error)
	UpdateUserAffCode(ctx context.Context, userID int64, newCode string) error
	ResetUserAffCode(ctx context.Context, userID int64) (string, error)
	SetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error
	BatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error
	ListUsersWithCustomSettings(ctx context.Context, filter AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error)
	ListAffiliateInviteRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error)
	ListAffiliateRebateRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error)
	ListAffiliateTransferRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error)
	GetAffiliateUserOverview(ctx context.Context, userID int64) (*AffiliateUserOverview, error)
}

type affiliateCreatorEarningsRepository interface {
	CreditCreatorEarnings(ctx context.Context, input AISkillCreatorEarningsInput) (float64, error)
	ReverseCreatorEarnings(ctx context.Context, input AISkillCreatorEarningsReversalInput) (float64, error)
}

type AffiliateService struct {
	repo                 AffiliateRepository
	settingRepo          SettingRepository
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCacheService  *BillingCacheService
}

func NewAffiliateService(repo AffiliateRepository, settingRepo SettingRepository, authCacheInvalidator APIKeyAuthCacheInvalidator, billingCacheService *BillingCacheService) *AffiliateService {
	return &AffiliateService{
		repo:                 repo,
		settingRepo:          settingRepo,
		authCacheInvalidator: authCacheInvalidator,
		billingCacheService:  billingCacheService,
	}
}

func (s *AffiliateService) EnsureUserAffiliate(ctx context.Context, userID int64) (*AffiliateSummary, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.EnsureUserAffiliate(ctx, userID)
}

func (s *AffiliateService) GetAffiliateDetail(ctx context.Context, userID int64) (*AffiliateDetail, error) {
	if s != nil && s.repo != nil {
		_, _ = s.repo.ThawFrozenQuota(ctx, userID)
	}
	summary, err := s.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return nil, err
	}
	policy := s.LoadPolicy(ctx)
	effectiveRate := s.resolveRebateRatePercent(ctx, summary)
	rebatedCount, err := s.countRebatedInvitees(ctx, userID)
	if err != nil {
		return nil, err
	}
	var remainingSlots *int
	if policy.InviteeLimit > 0 {
		remaining := policy.InviteeLimit - rebatedCount
		if remaining < 0 {
			remaining = 0
		}
		remainingSlots = &remaining
	}
	invitees, err := s.listInvitees(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &AffiliateDetail{
		UserID:                     summary.UserID,
		AffCode:                    summary.AffCode,
		AffCodeCustom:              summary.AffCodeCustom,
		AffRebateRatePercent:       summary.AffRebateRatePercent,
		InviterID:                  summary.InviterID,
		AffCount:                   summary.AffCount,
		AffQuota:                   summary.AffQuota,
		AffFrozenQuota:             summary.AffFrozenQuota,
		AffHistoryQuota:            summary.AffHistoryQuota,
		InvitedCount:               summary.AffCount,
		RebatedInviteeCount:        rebatedCount,
		RemainingRebateSlots:       remainingSlots,
		EffectiveRebateRatePercent: effectiveRate,
		Policy:                     policy,
		Invitees:                   invitees,
	}, nil
}

func (s *AffiliateService) GetInviteeLedger(ctx context.Context, inviterID, inviteeUserID int64) ([]AffiliateLedgerEntry, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListInviteeLedger(ctx, inviterID, inviteeUserID, 200)
}

func (s *AffiliateService) ListAdminAffiliateStats(ctx context.Context, params AdminAffiliateListParams) ([]AdminAffiliateStatsRow, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	if params.PageSize > 200 {
		params.PageSize = 200
	}
	params.Search = strings.TrimSpace(params.Search)
	return s.repo.ListAdminAffiliateStats(ctx, params)
}

func (s *AffiliateService) ListAdminInvitees(ctx context.Context, inviterID int64) ([]AffiliateInvitee, error) {
	if inviterID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	return s.listInvitees(ctx, inviterID)
}

func (s *AffiliateService) AdminListCustomUsers(ctx context.Context, filter AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 200 {
		filter.PageSize = 200
	}
	filter.Search = strings.TrimSpace(filter.Search)
	return s.repo.ListUsersWithCustomSettings(ctx, filter)
}

func (s *AffiliateService) AdminUpdateUserAffCode(ctx context.Context, userID int64, newCode string) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if userID <= 0 {
		return infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	code := strings.ToUpper(strings.TrimSpace(newCode))
	if !isValidAffiliateCodeFormat(code) {
		return ErrAffiliateCodeInvalid
	}
	if err := s.repo.UpdateUserAffCode(ctx, userID, code); err != nil {
		return err
	}
	s.invalidateAffiliateCaches(ctx, userID)
	return nil
}

func (s *AffiliateService) AdminResetUserAffCode(ctx context.Context, userID int64) (string, error) {
	if s == nil || s.repo == nil {
		return "", infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if userID <= 0 {
		return "", infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	code, err := s.repo.ResetUserAffCode(ctx, userID)
	if err != nil {
		return "", err
	}
	s.invalidateAffiliateCaches(ctx, userID)
	return code, nil
}

func (s *AffiliateService) AdminSetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if userID <= 0 {
		return infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if err := validateAffiliateRebateRatePercent(ratePercent); err != nil {
		return err
	}
	if err := s.repo.SetUserRebateRate(ctx, userID, ratePercent); err != nil {
		return err
	}
	s.invalidateAffiliateCaches(ctx, userID)
	return nil
}

func (s *AffiliateService) AdminBatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if len(userIDs) == 0 {
		return infraerrors.BadRequest("INVALID_USER_IDS", "user_ids cannot be empty")
	}
	if err := validateAffiliateRebateRatePercent(ratePercent); err != nil {
		return err
	}
	if err := s.repo.BatchSetUserRebateRate(ctx, userIDs, ratePercent); err != nil {
		return err
	}
	for _, userID := range userIDs {
		if userID > 0 {
			s.invalidateAffiliateCaches(ctx, userID)
		}
	}
	return nil
}

func (s *AffiliateService) AdminGetUserOverview(ctx context.Context, userID int64) (*AffiliateUserOverview, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	return s.repo.GetAffiliateUserOverview(ctx, userID)
}

func (s *AffiliateService) AdminListInviteRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateInviteRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) AdminListRebateRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateRebateRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) AdminListTransferRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateTransferRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) BindInviterByCode(ctx context.Context, userID int64, rawCode string) error {
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if code == "" {
		return nil
	}
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if !s.LoadPolicy(ctx).Enabled {
		return nil
	}
	if !isValidAffiliateCodeFormat(code) {
		return ErrAffiliateCodeInvalid
	}

	selfSummary, err := s.repo.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return err
	}
	if selfSummary.InviterID != nil {
		return nil
	}

	inviterSummary, err := s.repo.GetAffiliateByCode(ctx, code)
	if err != nil {
		if errors.Is(err, ErrAffiliateProfileNotFound) {
			return ErrAffiliateCodeInvalid
		}
		return err
	}
	if inviterSummary == nil || inviterSummary.UserID <= 0 || inviterSummary.UserID == userID {
		return ErrAffiliateCodeInvalid
	}

	bound, err := s.repo.BindInviter(ctx, userID, inviterSummary.UserID)
	if err != nil {
		return err
	}
	if !bound {
		return ErrAffiliateAlreadyBound
	}
	return nil
}

func (s *AffiliateService) AccrueInviteRebate(ctx context.Context, inviteeUserID int64, baseRechargeAmount float64) (float64, error) {
	return s.AccrueInviteRebateForOrder(ctx, inviteeUserID, 0, baseRechargeAmount)
}

func (s *AffiliateService) AccrueInviteRebateForOrder(ctx context.Context, inviteeUserID, orderID int64, baseRechargeAmount float64) (float64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if inviteeUserID <= 0 || baseRechargeAmount <= 0 || math.IsNaN(baseRechargeAmount) || math.IsInf(baseRechargeAmount, 0) {
		return 0, nil
	}
	policy := s.LoadPolicy(ctx)
	if !policy.Enabled {
		return 0, nil
	}

	inviteeSummary, err := s.repo.EnsureUserAffiliate(ctx, inviteeUserID)
	if err != nil {
		return 0, err
	}
	if inviteeSummary.InviterID == nil || *inviteeSummary.InviterID <= 0 {
		return 0, nil
	}

	inviterSummary, err := s.repo.EnsureUserAffiliate(ctx, *inviteeSummary.InviterID)
	if err != nil {
		return 0, err
	}

	rebateRatePercent := s.resolveRebateRatePercent(ctx, inviterSummary)
	rebate := roundTo(baseRechargeAmount*(rebateRatePercent/100), 8)
	if rebate <= 0 {
		return 0, nil
	}

	appliedAmount, err := s.repo.AccrueQuota(ctx, AffiliateAccrualInput{
		InviterID:     *inviteeSummary.InviterID,
		InviteeUserID: inviteeUserID,
		Amount:        rebate,
		BaseAmount:    baseRechargeAmount,
		RebateRate:    rebateRatePercent,
		SourceOrderID: orderID,
		RebateCap:     policy.RebateCap,
		InviteeLimit:  policy.InviteeLimit,
		FreezeHours:   policy.FreezeHours,
		PerInviteeCap: policy.PerInviteeCap,
	})
	if err != nil {
		return 0, err
	}
	if appliedAmount <= 0 {
		return 0, nil
	}
	return appliedAmount, nil
}

// ReverseInviteRebateForOrder claws back rebate previously accrued for an order
// that is being refunded. refundedAmount is the *cumulative* refunded amount on
// the order; orderAmount is the full order amount. The reversal is proportional
// (accrued * refundedAmount/orderAmount) and idempotent: the repository tracks
// what has already been reversed for the order and only claws back the delta.
//
// Deduction order inside the repository: aff_frozen_quota first, then aff_quota,
// and finally the inviter's balance if the rebate was already transferred out.
// Returns the amount actually reversed in this call (0 if nothing to do).
func (s *AffiliateService) ReverseInviteRebateForOrder(ctx context.Context, orderID int64, refundedAmount, orderAmount float64) (float64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if orderID <= 0 || refundedAmount <= 0 || orderAmount <= 0 {
		return 0, nil
	}
	if math.IsNaN(refundedAmount) || math.IsInf(refundedAmount, 0) ||
		math.IsNaN(orderAmount) || math.IsInf(orderAmount, 0) {
		return 0, nil
	}

	reversed, inviterID, err := s.repo.ReverseQuotaForOrder(ctx, AffiliateReversalInput{
		SourceOrderID:  orderID,
		RefundedAmount: refundedAmount,
		OrderAmount:    orderAmount,
	})
	if err != nil {
		return 0, err
	}
	if reversed <= 0 {
		return 0, nil
	}
	// The reversal may have clawed back from the inviter's spendable balance and
	// always mutates affiliate quota; invalidate the inviter's auth/billing caches
	// so the gateway does not keep billing against a stale (higher) balance until
	// TTL expiry. Mirrors the transfer path's invalidation.
	if inviterID > 0 {
		s.invalidateAffiliateCaches(ctx, inviterID)
	}
	return reversed, nil
}

func (s *AffiliateService) ApplySignupBonus(ctx context.Context, userID int64) (float64, float64, error) {
	policy := s.LoadPolicy(ctx)
	if s == nil || s.repo == nil || userID <= 0 || !policy.Enabled || policy.SignupBonus <= 0 {
		return 0, 0, nil
	}
	applied, balance, err := s.repo.ApplySignupBonus(ctx, userID, policy.SignupBonus)
	if err != nil || !applied {
		return 0, balance, err
	}
	s.invalidateAffiliateCaches(ctx, userID)
	return policy.SignupBonus, balance, nil
}

func (s *AffiliateService) TransferAffiliateQuota(ctx context.Context, userID int64) (float64, float64, error) {
	if s == nil || s.repo == nil {
		return 0, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}

	transferred, balance, err := s.repo.TransferQuotaToBalance(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	if transferred > 0 {
		s.invalidateAffiliateCaches(ctx, userID)
	}
	return transferred, balance, nil
}

func (s *AffiliateService) CreditCreatorEarnings(ctx context.Context, input AISkillCreatorEarningsInput) (float64, error) {
	if s == nil || s.repo == nil {
		return 0, ErrAffiliateCreatorQuotaUnavailable
	}
	if input.CreatorUserID <= 0 {
		return 0, infraerrors.BadRequest("AFFILIATE_CREATOR_USER_INVALID", "creator user is invalid")
	}
	if input.Amount <= 0 || math.IsNaN(input.Amount) || math.IsInf(input.Amount, 0) {
		return 0, nil
	}
	repo, ok := s.repo.(affiliateCreatorEarningsRepository)
	if !ok {
		return 0, ErrAffiliateCreatorQuotaUnavailable
	}
	applied, err := repo.CreditCreatorEarnings(ctx, input)
	if err != nil {
		return 0, err
	}
	if applied > 0 {
		s.invalidateAffiliateCaches(ctx, input.CreatorUserID)
	}
	return applied, nil
}

func (s *AffiliateService) ReverseCreatorEarnings(ctx context.Context, input AISkillCreatorEarningsReversalInput) (float64, error) {
	if s == nil || s.repo == nil {
		return 0, ErrAffiliateCreatorQuotaUnavailable
	}
	if input.CreatorUserID <= 0 {
		return 0, infraerrors.BadRequest("AFFILIATE_CREATOR_USER_INVALID", "creator user is invalid")
	}
	if input.Amount <= 0 || math.IsNaN(input.Amount) || math.IsInf(input.Amount, 0) {
		return 0, nil
	}
	repo, ok := s.repo.(affiliateCreatorEarningsRepository)
	if !ok {
		return 0, ErrAffiliateCreatorQuotaUnavailable
	}
	reversed, err := repo.ReverseCreatorEarnings(ctx, input)
	if err != nil {
		return 0, err
	}
	if reversed > 0 {
		s.invalidateAffiliateCaches(ctx, input.CreatorUserID)
	}
	return reversed, nil
}

func (s *AffiliateService) listInvitees(ctx context.Context, inviterID int64) ([]AffiliateInvitee, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	invitees, err := s.repo.ListInvitees(ctx, inviterID, affiliateInviteesLimit)
	if err != nil {
		return nil, err
	}
	for i := range invitees {
		invitees[i].Email = maskEmail(invitees[i].Email)
	}
	return invitees, nil
}

func (s *AffiliateService) countRebatedInvitees(ctx context.Context, inviterID int64) (int, error) {
	if s == nil || s.repo == nil {
		return 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.CountRebatedInvitees(ctx, inviterID)
}

func (s *AffiliateService) LoadPolicy(ctx context.Context) AffiliatePolicy {
	policy := AffiliatePolicy{
		Enabled:       s.loadAffiliateEnabled(ctx),
		RebateRate:    s.loadAffiliateRebateRatePercent(ctx),
		RebateCap:     s.loadNonNegativeFloat(ctx, SettingKeyAffiliateRebateCap, AffiliateRebateCapDefault),
		InviteeLimit:  s.loadNonNegativeInt(ctx, SettingKeyAffiliateRebateInviteeLimit, AffiliateRebateInviteeLimitDefault),
		FreezeHours:   s.loadNonNegativeInt(ctx, SettingKeyAffiliateRebateFreezeHours, AffiliateRebateFreezeHoursDefault),
		DurationDays:  s.loadNonNegativeInt(ctx, SettingKeyAffiliateRebateDurationDays, AffiliateRebateDurationDaysDefault),
		PerInviteeCap: s.loadNonNegativeFloat(ctx, SettingKeyAffiliateRebatePerInviteeCap, AffiliateRebatePerInviteeCapDefault),
		SignupBonus:   s.loadNonNegativeFloat(ctx, SettingKeyAffiliateSignupBonus, AffiliateSignupBonusDefault),
	}
	policy.PolicyText = buildAffiliatePolicyText(policy)
	return policy
}

func (s *AffiliateService) resolveRebateRatePercent(ctx context.Context, summary *AffiliateSummary) float64 {
	if summary != nil && summary.AffRebateRatePercent != nil {
		return clampAffiliateRebateRatePercent(*summary.AffRebateRatePercent)
	}
	return s.loadAffiliateRebateRatePercent(ctx)
}

func normalizeAffiliateRecordFilter(filter AffiliateRecordFilter) AffiliateRecordFilter {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 200 {
		filter.PageSize = 200
	}
	filter.Search = strings.TrimSpace(filter.Search)
	filter.SortBy = strings.TrimSpace(filter.SortBy)
	return filter
}

func (s *AffiliateService) loadAffiliateEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyAffiliateEnabled)
	return err == nil && strings.TrimSpace(raw) == "true"
}

func (s *AffiliateService) IsEnabled(ctx context.Context) bool {
	return s.loadAffiliateEnabled(ctx)
}

func (s *AffiliateService) loadAffiliateRebateRatePercent(ctx context.Context) float64 {
	if s == nil || s.settingRepo == nil {
		return AffiliateRebateRateDefault
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingKeyAffiliateRebateRate)
	if err != nil {
		return AffiliateRebateRateDefault
	}

	rate, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return AffiliateRebateRateDefault
	}
	return clampAffiliateRebateRatePercent(rate)
}

func (s *AffiliateService) loadNonNegativeFloat(ctx context.Context, key string, fallback float64) float64 {
	if s == nil || s.settingRepo == nil {
		return fallback
	}
	raw, err := s.settingRepo.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return fallback
	}
	return value
}

func (s *AffiliateService) loadNonNegativeInt(ctx context.Context, key string, fallback int) int {
	if s == nil || s.settingRepo == nil {
		return fallback
	}
	raw, err := s.settingRepo.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func validateAffiliateRebateRatePercent(ratePercent *float64) error {
	if ratePercent == nil {
		return nil
	}
	if math.IsNaN(*ratePercent) || math.IsInf(*ratePercent, 0) {
		return ErrAffiliateRebateRateInvalid
	}
	if *ratePercent < AffiliateRebateRateMin || *ratePercent > AffiliateRebateRateMax {
		return ErrAffiliateRebateRateInvalid
	}
	return nil
}

func clampAffiliateRebateRatePercent(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return AffiliateRebateRateDefault
	}
	if value < AffiliateRebateRateMin {
		return AffiliateRebateRateMin
	}
	if value > AffiliateRebateRateMax {
		return AffiliateRebateRateMax
	}
	return value
}

func buildAffiliatePolicyText(policy AffiliatePolicy) string {
	if !policy.Enabled {
		return "邀请返利当前未启用。"
	}
	parts := []string{fmt.Sprintf("被邀请用户充值余额后，邀请人按 %.2f%% 获得返利额度。", policy.RebateRate)}
	if policy.RebateCap > 0 {
		parts = append(parts, fmt.Sprintf("每位邀请人累计最多可获得 %.2f 返利额度。", policy.RebateCap))
	} else {
		parts = append(parts, "每位邀请人的累计返利额度暂无上限。")
	}
	if policy.InviteeLimit > 0 {
		parts = append(parts, fmt.Sprintf("每位邀请人最多可纳入 %d 位已返利消费用户；被邀请用户首次产生有效返利消费即占用 1 个名额。", policy.InviteeLimit))
	} else {
		parts = append(parts, "已返利消费用户人数暂无上限。")
	}
	if policy.SignupBonus > 0 {
		parts = append(parts, fmt.Sprintf("新用户通过邀请链接注册成功可获得 %.2f 余额奖励。", policy.SignupBonus))
	}
	return strings.Join(parts, " ")
}

func roundTo(v float64, scale int) float64 {
	factor := math.Pow10(scale)
	return math.Round(v*factor) / factor
}

func maskEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	at := strings.Index(email, "@")
	if at <= 0 || at >= len(email)-1 {
		return "***"
	}

	local := email[:at]
	domain := email[at+1:]
	dot := strings.LastIndex(domain, ".")

	maskedLocal := maskSegment(local)
	if dot <= 0 || dot >= len(domain)-1 {
		return maskedLocal + "@" + maskSegment(domain)
	}

	domainName := domain[:dot]
	tld := domain[dot:]
	return maskedLocal + "@" + maskSegment(domainName) + tld
}

func maskSegment(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return "***"
	}
	if len(r) == 1 {
		return string(r[0]) + "***"
	}
	return string(r[0]) + "***"
}

func (s *AffiliateService) invalidateAffiliateCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		if err := s.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
			logger.LegacyPrintf("service.affiliate", "[Affiliate] Failed to invalidate billing cache for user %d: %v", userID, err)
		}
	}
}
