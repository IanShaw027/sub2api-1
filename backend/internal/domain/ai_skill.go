package domain

import (
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	AISkillTypePromptChat  = "prompt_chat"
	AISkillTypePromptImage = "prompt_image"
	AISkillTypeScript      = "script"
)

const (
	AISkillScopeMine    = "mine"
	AISkillScopeLibrary = "library"
	AISkillScopeAll     = "all"
)

const (
	AISkillSourceVisibilityPublic = "public"
	AISkillSourceVisibilityHidden = "hidden"
)

const (
	AISkillBillingModePerRequest = "per_request"
)

const (
	AISkillPriceModeFree = "free"
	AISkillPriceModePaid = "paid"
)

const (
	AISkillVersionReviewStatusDraft    = "draft"
	AISkillVersionReviewStatusPending  = "pending"
	AISkillVersionReviewStatusApproved = "approved"
	AISkillVersionReviewStatusRejected = "rejected"
)

const (
	AISkillReviewStatusPending  = "pending"
	AISkillReviewStatusApproved = "approved"
	AISkillReviewStatusRejected = "rejected"
	AISkillReviewStatusCanceled = "canceled"
)

const (
	AISkillRunModeTest = "test"
	AISkillRunModeUse  = "use"
)

const (
	AISkillRunStatusQueued    = "queued"
	AISkillRunStatusRunning   = "running"
	AISkillRunStatusSucceeded = "succeeded"
	AISkillRunStatusFailed    = "failed"
	AISkillRunStatusCanceled  = "canceled"
)

const (
	AISkillSettlementStatusPending      = "pending"
	AISkillSettlementStatusQuotaAccrued = "quota_accrued"
	AISkillSettlementStatusTransferred  = "transferred"
	AISkillSettlementStatusCanceled     = "canceled"
)

var (
	ErrAISkillNotFound                 = infraerrors.NotFound("AI_SKILL_NOT_FOUND", "ai skill not found")
	ErrAISkillVersionNotFound          = infraerrors.NotFound("AI_SKILL_VERSION_NOT_FOUND", "ai skill version not found")
	ErrAISkillReviewNotFound           = infraerrors.NotFound("AI_SKILL_REVIEW_NOT_FOUND", "ai skill review not found")
	ErrAISkillRunNotFound              = infraerrors.NotFound("AI_SKILL_RUN_NOT_FOUND", "ai skill run not found")
	ErrAISkillSettlementNotFound       = infraerrors.NotFound("AI_SKILL_SETTLEMENT_NOT_FOUND", "ai skill settlement not found")
	ErrAISkillTypeInvalid              = infraerrors.BadRequest("AI_SKILL_TYPE_INVALID", "invalid ai skill type")
	ErrAISkillVisibilityInvalid        = infraerrors.BadRequest("AI_SKILL_VISIBILITY_INVALID", "invalid ai skill visibility")
	ErrAISkillBillingModeInvalid       = infraerrors.BadRequest("AI_SKILL_BILLING_MODE_INVALID", "invalid ai skill billing mode")
	ErrAISkillPriceModeInvalid         = infraerrors.BadRequest("AI_SKILL_PRICE_MODE_INVALID", "invalid ai skill price mode")
	ErrAISkillSourceVisibilityInvalid  = infraerrors.BadRequest("AI_SKILL_SOURCE_VISIBILITY_INVALID", "invalid ai skill source visibility")
	ErrAISkillVersionStatusInvalid     = infraerrors.BadRequest("AI_SKILL_VERSION_STATUS_INVALID", "invalid ai skill version review status")
	ErrAISkillReviewStatusInvalid      = infraerrors.BadRequest("AI_SKILL_REVIEW_STATUS_INVALID", "invalid ai skill review status")
	ErrAISkillRunModeInvalid           = infraerrors.BadRequest("AI_SKILL_RUN_MODE_INVALID", "invalid ai skill run mode")
	ErrAISkillRunStatusInvalid         = infraerrors.BadRequest("AI_SKILL_RUN_STATUS_INVALID", "invalid ai skill run status")
	ErrAISkillSettlementStatusInvalid  = infraerrors.BadRequest("AI_SKILL_SETTLEMENT_STATUS_INVALID", "invalid ai skill settlement status")
	ErrAISkillPriceInvalid             = infraerrors.BadRequest("AI_SKILL_PRICE_INVALID", "invalid ai skill price")
	ErrAISkillTitleRequired            = infraerrors.BadRequest("AI_SKILL_TITLE_REQUIRED", "ai skill title is required")
	ErrAISkillVersionNumberInvalid     = infraerrors.BadRequest("AI_SKILL_VERSION_NUMBER_INVALID", "invalid ai skill version number")
	ErrAISkillReviewAlreadyPending     = infraerrors.Conflict("AI_SKILL_REVIEW_ALREADY_PENDING", "ai skill review already pending")
	ErrAISkillVersionTransitionInvalid = infraerrors.Conflict(
		"AI_SKILL_VERSION_TRANSITION_INVALID",
		"ai skill version review transition is invalid",
	)
	ErrAISkillReviewTransitionInvalid = infraerrors.Conflict(
		"AI_SKILL_REVIEW_TRANSITION_INVALID",
		"ai skill review transition is invalid",
	)
	ErrAISkillVersionNotApproved   = infraerrors.Forbidden("AI_SKILL_VERSION_NOT_APPROVED", "ai skill version is not approved")
	ErrAISkillVersionInUse         = infraerrors.Conflict("AI_SKILL_VERSION_IN_USE", "ai skill version is in use")
	ErrAISkillVersionAlreadyExists = infraerrors.Conflict(
		"AI_SKILL_VERSION_ALREADY_EXISTS",
		"ai skill version already exists",
	)
	ErrAISkillSettlementAlreadyExists = infraerrors.Conflict(
		"AI_SKILL_SETTLEMENT_ALREADY_EXISTS",
		"ai skill settlement already exists",
	)
)

func NormalizeAISkillType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case AISkillTypePromptChat:
		return AISkillTypePromptChat
	case AISkillTypePromptImage:
		return AISkillTypePromptImage
	case AISkillTypeScript:
		return AISkillTypeScript
	default:
		return ""
	}
}

func IsValidAISkillType(raw string) bool {
	return NormalizeAISkillType(raw) != ""
}

func NormalizeAISkillScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case AISkillScopeLibrary:
		return AISkillScopeLibrary
	case AISkillScopeAll:
		return AISkillScopeAll
	default:
		return AISkillScopeMine
	}
}

func NormalizeAISkillBillingMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", AISkillBillingModePerRequest:
		return AISkillBillingModePerRequest
	default:
		return ""
	}
}

func IsValidAISkillBillingMode(raw string) bool {
	return NormalizeAISkillBillingMode(raw) == AISkillBillingModePerRequest
}

func NormalizeAISkillPriceMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case AISkillPriceModeFree:
		return AISkillPriceModeFree
	case AISkillPriceModePaid:
		return AISkillPriceModePaid
	default:
		return ""
	}
}

func IsValidAISkillPriceMode(raw string) bool {
	return NormalizeAISkillPriceMode(raw) != ""
}

func NormalizeAISkillSourceVisibility(raw, visibility string, price float64) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case AISkillSourceVisibilityPublic:
		return AISkillSourceVisibilityPublic
	case AISkillSourceVisibilityHidden:
		return AISkillSourceVisibilityHidden
	case "":
		if NormalizeAIVisibility(visibility) == AIVisibilityPublic && price > 0 {
			return AISkillSourceVisibilityHidden
		}
		return AISkillSourceVisibilityPublic
	}
	return ""
}

func IsValidAISkillSourceVisibility(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case AISkillSourceVisibilityPublic, AISkillSourceVisibilityHidden:
		return true
	default:
		return false
	}
}

func EffectiveAISkillSourceVisibility(visibility, sourceVisibility string, price float64) string {
	normalized := NormalizeAISkillSourceVisibility(sourceVisibility, visibility, price)
	if normalized != "" {
		return normalized
	}
	return AISkillSourceVisibilityHidden
}

func NormalizeAISkillVersionReviewStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return AISkillVersionReviewStatusDraft
	case AISkillVersionReviewStatusDraft:
		return AISkillVersionReviewStatusDraft
	case AISkillVersionReviewStatusPending:
		return AISkillVersionReviewStatusPending
	case AISkillVersionReviewStatusApproved:
		return AISkillVersionReviewStatusApproved
	case AISkillVersionReviewStatusRejected:
		return AISkillVersionReviewStatusRejected
	default:
		return ""
	}
}

func IsValidAISkillVersionReviewStatus(raw string) bool {
	return NormalizeAISkillVersionReviewStatus(raw) != ""
}

func CanSubmitAISkillVersionForReview(reviewStatus string) bool {
	switch NormalizeAISkillVersionReviewStatus(reviewStatus) {
	case AISkillVersionReviewStatusDraft, AISkillVersionReviewStatusRejected:
		return true
	default:
		return false
	}
}

func CanReviewAISkillVersion(reviewStatus string) bool {
	return NormalizeAISkillVersionReviewStatus(reviewStatus) == AISkillVersionReviewStatusPending
}

func NextAISkillVersionReviewStatus(current string, approved bool) (string, error) {
	if !CanReviewAISkillVersion(current) {
		return "", ErrAISkillVersionTransitionInvalid
	}
	if approved {
		return AISkillVersionReviewStatusApproved, nil
	}
	return AISkillVersionReviewStatusRejected, nil
}

func CanUseAISkillVersion(reviewStatus string) bool {
	return NormalizeAISkillVersionReviewStatus(reviewStatus) == AISkillVersionReviewStatusApproved
}

func CanPublishAISkillVersion(reviewStatus string) bool {
	return CanUseAISkillVersion(reviewStatus)
}

func NormalizeAISkillReviewStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return AISkillReviewStatusPending
	case AISkillReviewStatusPending:
		return AISkillReviewStatusPending
	case AISkillReviewStatusApproved:
		return AISkillReviewStatusApproved
	case AISkillReviewStatusRejected:
		return AISkillReviewStatusRejected
	case AISkillReviewStatusCanceled:
		return AISkillReviewStatusCanceled
	default:
		return ""
	}
}

func IsValidAISkillReviewStatus(raw string) bool {
	return NormalizeAISkillReviewStatus(raw) != ""
}

func CanApplyAISkillReview(reviewStatus string) bool {
	return NormalizeAISkillReviewStatus(reviewStatus) == AISkillReviewStatusPending
}

func NextAISkillReviewStatus(current string, approved bool) (string, error) {
	if !CanApplyAISkillReview(current) {
		return "", ErrAISkillReviewTransitionInvalid
	}
	if approved {
		return AISkillReviewStatusApproved, nil
	}
	return AISkillReviewStatusRejected, nil
}

func NormalizeAISkillRunMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return AISkillRunModeUse
	case AISkillRunModeTest:
		return AISkillRunModeTest
	case AISkillRunModeUse:
		return AISkillRunModeUse
	default:
		return ""
	}
}

func IsValidAISkillRunMode(raw string) bool {
	return NormalizeAISkillRunMode(raw) != ""
}

func NormalizeAISkillRunStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return AISkillRunStatusQueued
	case AISkillRunStatusQueued:
		return AISkillRunStatusQueued
	case AISkillRunStatusRunning:
		return AISkillRunStatusRunning
	case AISkillRunStatusSucceeded:
		return AISkillRunStatusSucceeded
	case AISkillRunStatusFailed:
		return AISkillRunStatusFailed
	case AISkillRunStatusCanceled:
		return AISkillRunStatusCanceled
	default:
		return ""
	}
}

func IsValidAISkillRunStatus(raw string) bool {
	return NormalizeAISkillRunStatus(raw) != ""
}

func NormalizeAISkillSettlementStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return AISkillSettlementStatusPending
	case AISkillSettlementStatusPending:
		return AISkillSettlementStatusPending
	case AISkillSettlementStatusQuotaAccrued:
		return AISkillSettlementStatusQuotaAccrued
	case AISkillSettlementStatusTransferred:
		return AISkillSettlementStatusTransferred
	case AISkillSettlementStatusCanceled:
		return AISkillSettlementStatusCanceled
	default:
		return ""
	}
}

func IsValidAISkillSettlementStatus(raw string) bool {
	return NormalizeAISkillSettlementStatus(raw) != ""
}

func CanReadAISkill(ownerUserID, viewerUserID int64, isAdmin bool, visibility string, publishedVersionID *int64) bool {
	if isAdmin {
		return true
	}
	if ownerUserID > 0 && ownerUserID == viewerUserID {
		return true
	}
	if publishedVersionID == nil {
		return false
	}
	switch NormalizeAIVisibility(visibility) {
	case AIVisibilityPublic, AIVisibilityUnlisted:
		return true
	default:
		return false
	}
}

func CanListAISkillInLibrary(ownerUserID, viewerUserID int64, visibility string, publishedVersionID *int64) bool {
	if ownerUserID > 0 && ownerUserID == viewerUserID {
		return true
	}
	return publishedVersionID != nil && NormalizeAIVisibility(visibility) == AIVisibilityPublic
}

func CanReadAISkillSource(ownerUserID, viewerUserID int64, isAdmin bool, visibility, sourceVisibility string, price float64, publishedVersionID *int64) bool {
	if isAdmin {
		return true
	}
	if ownerUserID > 0 && ownerUserID == viewerUserID {
		return true
	}
	if !CanReadAISkill(ownerUserID, viewerUserID, false, visibility, publishedVersionID) {
		return false
	}
	return EffectiveAISkillSourceVisibility(visibility, sourceVisibility, price) == AISkillSourceVisibilityPublic
}

type AISkillListFilter struct {
	Scope         string
	OwnerUserID   *int64
	SkillType     string
	Category      string
	Status        string
	Visibility    string
	BillingMode   string
	PriceMode     string
	Search        string
	PublishedOnly bool
	Installed     *bool
	GroupID       *int64
}

type AISkill struct {
	ID                      int64            `json:"id"`
	UserID                  int64            `json:"user_id"`
	Type                    string           `json:"type"`
	Title                   string           `json:"title"`
	Summary                 string           `json:"summary,omitempty"`
	Description             string           `json:"description,omitempty"`
	Category                string           `json:"category,omitempty"`
	Tags                    []string         `json:"tags,omitempty"`
	Visibility              string           `json:"visibility"`
	SourceVisibility        string           `json:"source_visibility"`
	BillingMode             string           `json:"billing_mode"`
	Price                   float64          `json:"price"`
	CurrentVersionID        *int64           `json:"current_version_id,omitempty"`
	PublishedVersionID      *int64           `json:"published_version_id,omitempty"`
	LatestApprovedVersionID *int64           `json:"latest_approved_version_id,omitempty"`
	LatestVersion           int              `json:"latest_version"`
	LikeCount               int              `json:"like_count"`
	RunCount                int              `json:"run_count"`
	TotalIncome             float64          `json:"total_income"`
	CoverAssetID            *int64           `json:"cover_asset_id,omitempty"`
	Metadata                map[string]any   `json:"metadata,omitempty"`
	Trace                   AITraceRef       `json:"trace"`
	CreatedAt               time.Time        `json:"created_at"`
	UpdatedAt               time.Time        `json:"updated_at"`
	DeletedAt               *time.Time       `json:"deleted_at,omitempty"`
	Versions                []AISkillVersion `json:"versions,omitempty"`
}

type AISkillVersion struct {
	ID                     int64          `json:"id"`
	SkillID                int64          `json:"skill_id"`
	UserID                 int64          `json:"user_id"`
	Version                int            `json:"version"`
	ReviewStatus           string         `json:"review_status"`
	ApprovedArtifactDigest string         `json:"approved_artifact_digest,omitempty"`
	ContentFormat          string         `json:"content_format,omitempty"`
	Runtime                string         `json:"runtime,omitempty"`
	SourceContent          string         `json:"source_content,omitempty"`
	Config                 map[string]any `json:"config,omitempty"`
	InputSchema            map[string]any `json:"input_schema,omitempty"`
	OutputSchema           map[string]any `json:"output_schema,omitempty"`
	ChangeNote             string         `json:"change_note,omitempty"`
	SubmittedAt            *time.Time     `json:"submitted_at,omitempty"`
	ReviewedAt             *time.Time     `json:"reviewed_at,omitempty"`
	ReviewerUserID         *int64         `json:"reviewer_user_id,omitempty"`
	ReviewNote             string         `json:"review_note,omitempty"`
	Metadata               map[string]any `json:"metadata,omitempty"`
	Trace                  AITraceRef     `json:"trace"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
	DeletedAt              *time.Time     `json:"deleted_at,omitempty"`
}

type AISkillReview struct {
	ID              int64          `json:"id"`
	SkillID         int64          `json:"skill_id"`
	VersionID       int64          `json:"version_id"`
	SubmitterUserID int64          `json:"submitter_user_id"`
	ReviewerUserID  *int64         `json:"reviewer_user_id,omitempty"`
	Status          string         `json:"status"`
	SubmitNote      string         `json:"submit_note,omitempty"`
	ReviewNote      string         `json:"review_note,omitempty"`
	Snapshot        map[string]any `json:"snapshot,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	ReviewedAt      *time.Time     `json:"reviewed_at,omitempty"`
	Trace           AITraceRef     `json:"trace"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type AISkillReviewSubmitInput struct {
	SkillID         int64          `json:"skill_id"`
	VersionID       int64          `json:"version_id"`
	SubmitterUserID int64          `json:"submitter_user_id"`
	SubmitNote      string         `json:"submit_note,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	Trace           AITraceRef     `json:"trace"`
}

type AISkillReviewDecisionInput struct {
	ReviewID         int64          `json:"review_id"`
	ReviewerUserID   int64          `json:"reviewer_user_id"`
	Approved         bool           `json:"approved"`
	ReviewNote       string         `json:"review_note,omitempty"`
	PublishOnApprove bool           `json:"publish_on_approve,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	Trace            AITraceRef     `json:"trace"`
}

type AISkillRun struct {
	ID            int64          `json:"id"`
	SkillID       int64          `json:"skill_id"`
	VersionID     int64          `json:"version_id"`
	UserID        int64          `json:"user_id"`
	RunMode       string         `json:"run_mode"`
	Status        string         `json:"status"`
	BillingMode   string         `json:"billing_mode"`
	Price         float64        `json:"price"`
	InputPayload  map[string]any `json:"input_payload,omitempty"`
	OutputPayload map[string]any `json:"output_payload,omitempty"`
	ErrorMessage  string         `json:"error_message,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	Trace         AITraceRef     `json:"trace"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type AISkillLikeInput struct {
	SkillID int64      `json:"skill_id"`
	UserID  int64      `json:"user_id"`
	Liked   bool       `json:"liked"`
	Trace   AITraceRef `json:"trace"`
}

type AISkillSettlement struct {
	ID                   int64          `json:"id"`
	SkillID              int64          `json:"skill_id"`
	VersionID            int64          `json:"version_id"`
	RunID                int64          `json:"run_id"`
	OwnerUserID          int64          `json:"owner_user_id"`
	BuyerUserID          int64          `json:"buyer_user_id"`
	Status               string         `json:"status"`
	BillingMode          string         `json:"billing_mode"`
	Amount               float64        `json:"amount"`
	QuotaAmount          float64        `json:"quota_amount"`
	QuotaAppliedAt       *time.Time     `json:"quota_applied_at,omitempty"`
	BalanceTransferredAt *time.Time     `json:"balance_transferred_at,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
	Trace                AITraceRef     `json:"trace"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}
