package service

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	AISkillTypePromptChat  = "prompt_chat"
	AISkillTypePromptImage = "prompt_image"
	AISkillTypeScript      = "script"
)

const (
	AISkillVersionStatusDraft     = "draft"
	AISkillVersionStatusSubmitted = "submitted"
	AISkillVersionStatusApproved  = "approved"
	AISkillVersionStatusRejected  = "rejected"
	AISkillVersionStatusDisabled  = "disabled"
)

const (
	AISkillReviewActionSubmitted = "submitted"
	AISkillReviewActionApproved  = "approved"
	AISkillReviewActionRejected  = "rejected"
	AISkillReviewActionDisabled  = "disabled"
)

const (
	AISkillBillingModeFree   = "free"
	AISkillBillingModePerRun = "per_run"
)

const (
	AISkillRunModeTest = "test"
	AISkillRunModeUse  = "use"
)

const (
	AISkillRunStatusPrepared   = "prepared"
	AISkillRunStatusDispatched = "dispatched"
	AISkillRunStatusSucceeded  = "succeeded"
	AISkillRunStatusFailed     = "failed"
)

const (
	AISkillSettlementStatusSkipped = "skipped"
	AISkillSettlementStatusPending = "pending"
	AISkillSettlementStatusSettled = "settled"
	AISkillSettlementStatusFailed  = "failed"
)

const aiSkillDefaultCurrency = "credit"

var (
	ErrAISkillServiceUnavailable = infraerrors.ServiceUnavailable("AI_SKILL_SERVICE_UNAVAILABLE", "ai skill service unavailable")
	ErrAISkillNotFound           = infraerrors.NotFound("AI_SKILL_NOT_FOUND", "ai skill not found")
	ErrAISkillVersionNotFound    = infraerrors.NotFound("AI_SKILL_VERSION_NOT_FOUND", "ai skill version not found")
	ErrAISkillRunNotFound        = infraerrors.NotFound("AI_SKILL_RUN_NOT_FOUND", "ai skill run not found")
	ErrAISkillSettlementNotFound = infraerrors.NotFound("AI_SKILL_SETTLEMENT_NOT_FOUND", "ai skill settlement not found")

	ErrAISkillTypeInvalid                = infraerrors.BadRequest("AI_SKILL_TYPE_INVALID", "ai skill type is invalid")
	ErrAISkillScriptArchivePathInvalid   = infraerrors.BadRequest("AI_SKILL_SCRIPT_ARCHIVE_PATH_INVALID", "ai skill script archive path is invalid")
	ErrAISkillScriptExecutionUnavailable = infraerrors.ServiceUnavailable("AI_SKILL_SCRIPT_EXECUTION_UNAVAILABLE", "ai skill script execution is unavailable until the runtime executor is wired")
	ErrAISkillScriptNotApproved          = infraerrors.Forbidden("AI_SKILL_SCRIPT_NOT_APPROVED", "ai skill script version has no approved artifact digest; re-approval is required before execution")
	ErrAISkillScriptArtifactMismatch     = infraerrors.Forbidden("AI_SKILL_SCRIPT_ARTIFACT_MISMATCH", "ai skill script artifact does not match the approved digest")
	ErrAISkillNameRequired               = infraerrors.BadRequest("AI_SKILL_NAME_REQUIRED", "ai skill name is required")
	ErrAISkillExecutionSpecInvalid       = infraerrors.BadRequest("AI_SKILL_EXECUTION_SPEC_INVALID", "ai skill execution spec is invalid")
	ErrAISkillBillingPolicyInvalid       = infraerrors.BadRequest("AI_SKILL_BILLING_POLICY_INVALID", "ai skill billing policy is invalid")
	ErrAISkillVersionStatusInvalid       = infraerrors.BadRequest("AI_SKILL_VERSION_STATUS_INVALID", "ai skill version status is invalid")
	ErrAISkillVersionTransitionInvalid   = infraerrors.BadRequest("AI_SKILL_VERSION_TRANSITION_INVALID", "ai skill version status transition is invalid")
	ErrAISkillVersionNotEditable         = infraerrors.BadRequest("AI_SKILL_VERSION_NOT_EDITABLE", "ai skill version is not editable")
	ErrAISkillVersionNotApproved         = infraerrors.Forbidden("AI_SKILL_VERSION_NOT_APPROVED", "ai skill version must be approved before test or use")
	ErrAISkillRunModeInvalid             = infraerrors.BadRequest("AI_SKILL_RUN_MODE_INVALID", "ai skill run mode is invalid")
	ErrAISkillSettlementUnavailable      = infraerrors.ServiceUnavailable("AI_SKILL_SETTLEMENT_UNAVAILABLE", "ai skill settlement service unavailable")
	ErrAISkillBalanceServiceUnavailable  = infraerrors.ServiceUnavailable("AI_SKILL_BALANCE_UNAVAILABLE", "ai skill balance service unavailable")
	ErrAISkillBalanceReferenceInvalid    = infraerrors.BadRequest("AI_SKILL_BALANCE_REFERENCE_INVALID", "ai skill balance reference is required")
	ErrAISkillBalanceReferenceConflict   = infraerrors.Conflict("AI_SKILL_BALANCE_REFERENCE_CONFLICT", "ai skill balance reference was already used with different attributes")
	ErrAISkillBalanceChargeRefunded      = infraerrors.Conflict("AI_SKILL_BALANCE_CHARGE_REFUNDED", "ai skill balance charge was already refunded and cannot be replayed")
	ErrAISkillCreatorEarningsUnavailable = infraerrors.ServiceUnavailable("AI_SKILL_CREATOR_EARNINGS_UNAVAILABLE", "ai skill creator earnings service unavailable")
	ErrAISkillCreatorEarningsMismatch    = infraerrors.Conflict("AI_SKILL_CREATOR_EARNINGS_MISMATCH", "creator earnings amount mismatch")
	ErrAISkillSettlementInProgress       = infraerrors.Conflict("AI_SKILL_SETTLEMENT_IN_PROGRESS", "ai skill settlement is already pending")
	ErrAISkillSettlementReplayUnsafe     = infraerrors.Conflict("AI_SKILL_SETTLEMENT_REPLAY_UNSAFE", "ai skill settlement failed after partial side effects and requires manual reconciliation before retry")
)

type AISkill struct {
	ID            int64          `json:"id"`
	CreatorUserID int64          `json:"creator_user_id"`
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	Type          string         `json:"type"`
	LatestVersion int            `json:"latest_version"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	Trace         AITraceRef     `json:"trace"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type AISkillBillingPolicy struct {
	Mode                   string  `json:"mode"`
	PricePerRun            float64 `json:"price_per_run"`
	PlatformCommissionRate float64 `json:"platform_commission_rate"`
	Currency               string  `json:"currency,omitempty"`
}

func (p AISkillBillingPolicy) CreatorShareRate() float64 {
	rate := roundTo(1-p.PlatformCommissionRate, 8)
	if rate < 0 {
		return 0
	}
	return rate
}

type AISkillPromptChatSpec struct {
	Model              string           `json:"model,omitempty"`
	SystemPrompt       string           `json:"system_prompt,omitempty"`
	UserPromptTemplate string           `json:"user_prompt_template"`
	Variables          []map[string]any `json:"variables,omitempty"`
	ResponseFormat     map[string]any   `json:"response_format,omitempty"`
}

type AISkillPromptImageSpec struct {
	Model                  string           `json:"model,omitempty"`
	PromptTemplate         string           `json:"prompt_template"`
	NegativePromptTemplate string           `json:"negative_prompt_template,omitempty"`
	Size                   string           `json:"size,omitempty"`
	ImageCount             int              `json:"image_count,omitempty"`
	Variables              []map[string]any `json:"variables,omitempty"`
}

type AISkillScriptSpec struct {
	Runtime        string            `json:"runtime,omitempty"`
	ScriptName     string            `json:"script_name,omitempty"`
	EntryPoint     string            `json:"entry_point,omitempty"`
	Protocol       string            `json:"protocol,omitempty"`
	ArchivePath    string            `json:"archive_path,omitempty"`
	ArchiveBase64  string            `json:"archive_base64,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	Environment    map[string]string `json:"environment,omitempty"`
	Arguments      []map[string]any  `json:"arguments,omitempty"`
}

type AISkillExecutionSpec struct {
	Type        string                  `json:"type"`
	PromptChat  *AISkillPromptChatSpec  `json:"prompt_chat,omitempty"`
	PromptImage *AISkillPromptImageSpec `json:"prompt_image,omitempty"`
	Script      *AISkillScriptSpec      `json:"script,omitempty"`
}

type AISkillVersion struct {
	ID                     int64                `json:"id"`
	SkillID                int64                `json:"skill_id"`
	CreatorUserID          int64                `json:"creator_user_id"`
	Version                int                  `json:"version"`
	Type                   string               `json:"type"`
	Status                 string               `json:"status"`
	ApprovedArtifactDigest string               `json:"approved_artifact_digest,omitempty"`
	ExecutionSpec          AISkillExecutionSpec `json:"execution_spec"`
	BillingPolicy          AISkillBillingPolicy `json:"billing_policy"`
	ChangeNote             string               `json:"change_note,omitempty"`
	ReviewComment          string               `json:"review_comment,omitempty"`
	Metadata               map[string]any       `json:"metadata,omitempty"`
	SubmittedAt            *time.Time           `json:"submitted_at,omitempty"`
	ReviewedAt             *time.Time           `json:"reviewed_at,omitempty"`
	ApprovedAt             *time.Time           `json:"approved_at,omitempty"`
	RejectedAt             *time.Time           `json:"rejected_at,omitempty"`
	DisabledAt             *time.Time           `json:"disabled_at,omitempty"`
	ReviewerUserID         *int64               `json:"reviewer_user_id,omitempty"`
	Trace                  AITraceRef           `json:"trace"`
	CreatedAt              time.Time            `json:"created_at"`
	UpdatedAt              time.Time            `json:"updated_at"`
}

type AISkillReview struct {
	ID             int64          `json:"id"`
	SkillID        int64          `json:"skill_id"`
	VersionID      int64          `json:"version_id"`
	OperatorUserID int64          `json:"operator_user_id"`
	Action         string         `json:"action"`
	StatusFrom     string         `json:"status_from,omitempty"`
	StatusTo       string         `json:"status_to,omitempty"`
	Comment        string         `json:"comment,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Trace          AITraceRef     `json:"trace"`
	CreatedAt      time.Time      `json:"created_at"`
}

type AISkillRunAttachment struct {
	AssetID  *int64 `json:"asset_id,omitempty"`
	MediaID  *int64 `json:"media_id,omitempty"`
	URL      string `json:"url,omitempty"`
	Purpose  string `json:"purpose,omitempty"`
	FileName string `json:"file_name,omitempty"`
}

type AISkillRun struct {
	ID            int64                  `json:"id"`
	SkillID       int64                  `json:"skill_id"`
	VersionID     int64                  `json:"version_id"`
	UserID        int64                  `json:"user_id"`
	Mode          string                 `json:"mode"`
	Type          string                 `json:"type"`
	Status        string                 `json:"status"`
	BillingMode   string                 `json:"billing_mode,omitempty"`
	ChargeAmount  float64                `json:"charge_amount"`
	Currency      string                 `json:"currency,omitempty"`
	SettlementID  *int64                 `json:"settlement_id,omitempty"`
	Provider      string                 `json:"provider,omitempty"`
	ExternalJobID string                 `json:"external_job_id,omitempty"`
	Parameters    map[string]any         `json:"parameters,omitempty"`
	Attachments   []AISkillRunAttachment `json:"attachments,omitempty"`
	Metadata      map[string]any         `json:"metadata,omitempty"`
	Output        map[string]any         `json:"output,omitempty"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	Trace         AITraceRef             `json:"trace"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

type AISkillSettlement struct {
	ID                     int64          `json:"id"`
	RunID                  int64          `json:"run_id"`
	SkillID                int64          `json:"skill_id"`
	VersionID              int64          `json:"version_id"`
	BuyerUserID            int64          `json:"buyer_user_id"`
	CreatorUserID          int64          `json:"creator_user_id"`
	BillingMode            string         `json:"billing_mode"`
	Currency               string         `json:"currency,omitempty"`
	TotalAmount            float64        `json:"total_amount"`
	PlatformAmount         float64        `json:"platform_amount"`
	CreatorAmount          float64        `json:"creator_amount"`
	CreatorCreditedAmount  float64        `json:"creator_credited_amount"`
	PlatformCommissionRate float64        `json:"platform_commission_rate"`
	Status                 string         `json:"status"`
	BalanceAfter           *float64       `json:"balance_after,omitempty"`
	FailureReason          string         `json:"failure_reason,omitempty"`
	Metadata               map[string]any `json:"metadata,omitempty"`
	Trace                  AITraceRef     `json:"trace"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
}

type AISkillPromptChatExecution struct {
	Model              string                 `json:"model,omitempty"`
	SystemPrompt       string                 `json:"system_prompt,omitempty"`
	UserPromptTemplate string                 `json:"user_prompt_template"`
	Variables          []map[string]any       `json:"variables,omitempty"`
	ResponseFormat     map[string]any         `json:"response_format,omitempty"`
	Parameters         map[string]any         `json:"parameters,omitempty"`
	Attachments        []AISkillRunAttachment `json:"attachments,omitempty"`
}

type AISkillPromptImageExecution struct {
	Model                  string                 `json:"model,omitempty"`
	PromptTemplate         string                 `json:"prompt_template"`
	NegativePromptTemplate string                 `json:"negative_prompt_template,omitempty"`
	Size                   string                 `json:"size,omitempty"`
	ImageCount             int                    `json:"image_count,omitempty"`
	Variables              []map[string]any       `json:"variables,omitempty"`
	Parameters             map[string]any         `json:"parameters,omitempty"`
	Attachments            []AISkillRunAttachment `json:"attachments,omitempty"`
}

type AISkillScriptExecution struct {
	Runtime                string            `json:"runtime,omitempty"`
	ScriptName             string            `json:"script_name,omitempty"`
	EntryPoint             string            `json:"entry_point,omitempty"`
	Protocol               string            `json:"protocol,omitempty"`
	ArchivePath            string            `json:"archive_path,omitempty"`
	ArchiveBase64          string            `json:"archive_base64,omitempty"`
	ApprovedArtifactDigest string            `json:"approved_artifact_digest,omitempty"`
	VersionStatus          string            `json:"version_status,omitempty"`
	ReviewerUserID         *int64            `json:"reviewer_user_id,omitempty"`
	TimeoutSeconds         int               `json:"timeout_seconds,omitempty"`
	Environment            map[string]string `json:"environment,omitempty"`
	Arguments              []map[string]any  `json:"arguments,omitempty"`
	Parameters             map[string]any    `json:"parameters,omitempty"`
}

type AISkillExecutionRequest struct {
	RunID       int64                        `json:"run_id"`
	SkillID     int64                        `json:"skill_id"`
	VersionID   int64                        `json:"version_id"`
	UserID      int64                        `json:"user_id"`
	Mode        string                       `json:"mode"`
	Type        string                       `json:"type"`
	Skill       *AISkill                     `json:"skill,omitempty"`
	Version     *AISkillVersion              `json:"version,omitempty"`
	Settlement  *AISkillSettlement           `json:"settlement,omitempty"`
	PromptChat  *AISkillPromptChatExecution  `json:"prompt_chat,omitempty"`
	PromptImage *AISkillPromptImageExecution `json:"prompt_image,omitempty"`
	Script      *AISkillScriptExecution      `json:"script,omitempty"`
	Trace       AITraceRef                   `json:"trace"`
	// BillingAPIKey is server-resolved from Trace.APIKeyID (never client-forged).
	BillingAPIKey *APIKey `json:"-"`
}

type AISkillPreparedRun struct {
	Skill      *AISkill                 `json:"skill,omitempty"`
	Version    *AISkillVersion          `json:"version,omitempty"`
	Run        *AISkillRun              `json:"run,omitempty"`
	Settlement *AISkillSettlement       `json:"settlement,omitempty"`
	Execution  *AISkillExecutionRequest `json:"execution,omitempty"`
}

type AISkillDispatchResult struct {
	Status        string         `json:"status,omitempty"`
	Provider      string         `json:"provider,omitempty"`
	ExternalJobID string         `json:"external_job_id,omitempty"`
	Output        map[string]any `json:"output,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type AISkillRunResult struct {
	Prepared           *AISkillPreparedRun    `json:"prepared,omitempty"`
	Dispatch           *AISkillDispatchResult `json:"dispatch,omitempty"`
	SettlementDeferred bool                   `json:"settlement_deferred,omitempty"`
	SettlementStatus   string                 `json:"settlement_status,omitempty"`
}

type AISkillSettlementQuote struct {
	BillingMode            string  `json:"billing_mode"`
	Currency               string  `json:"currency,omitempty"`
	TotalAmount            float64 `json:"total_amount"`
	PlatformAmount         float64 `json:"platform_amount"`
	CreatorAmount          float64 `json:"creator_amount"`
	PlatformCommissionRate float64 `json:"platform_commission_rate"`
}

type AICreateSkillInput struct {
	Name        string
	Description *string
	Type        string
	Metadata    map[string]any
	Trace       AIWriteTrace
}

type AIUpdateSkillInput struct {
	Name        *string
	Description **string
	Metadata    *map[string]any
	Trace       AIWriteTrace
}

type AICreateSkillVersionInput struct {
	ExecutionSpec AISkillExecutionSpec
	BillingPolicy AISkillBillingPolicy
	ChangeNote    *string
	Metadata      map[string]any
	Trace         AIWriteTrace
}

type AIUpdateSkillVersionInput struct {
	ExecutionSpec *AISkillExecutionSpec
	BillingPolicy *AISkillBillingPolicy
	ChangeNote    **string
	Metadata      *map[string]any
	Trace         AIWriteTrace
}

type AISubmitSkillVersionInput struct {
	Comment  *string
	Metadata map[string]any
	Trace    AIWriteTrace
}

type AIReviewSkillVersionInput struct {
	Comment  *string
	Metadata map[string]any
	Trace    AIWriteTrace
}

type AISkillRunInput struct {
	SkillID        int64
	VersionID      *int64
	Mode           string
	Parameters     map[string]any
	Attachments    []AISkillRunAttachment
	Metadata       map[string]any
	IdempotencyKey string
	Trace          AIWriteTrace
}

type AISkillSettleInput struct {
	RunID         int64
	SkillID       int64
	VersionID     int64
	BuyerUserID   int64
	CreatorUserID int64
	BillingPolicy AISkillBillingPolicy
	Metadata      map[string]any
	Trace         AIWriteTrace
}

type AISkillBalanceChargeInput struct {
	UserID    int64
	Amount    float64
	Reference string
	Metadata  map[string]any
	Trace     AIWriteTrace
}

type AISkillBalanceRefundInput struct {
	UserID    int64
	Amount    float64
	Reference string
	Metadata  map[string]any
	Trace     AIWriteTrace
}

type AISkillBalanceChargeResult struct {
	ChargedAmount float64 `json:"charged_amount"`
	BalanceAfter  float64 `json:"balance_after"`
}

type AISkillBalanceRefundResult struct {
	RefundedAmount float64 `json:"refunded_amount"`
	BalanceAfter   float64 `json:"balance_after"`
}

type AISkillBalanceLedgerResult struct {
	Amount        float64
	BalanceBefore float64
	BalanceAfter  float64
	Duplicate     bool
	Refunded      bool
}

type AISkillCreatorEarningsInput struct {
	CreatorUserID int64
	BuyerUserID   int64
	SkillID       int64
	VersionID     int64
	RunID         int64
	Amount        float64
	Currency      string
	Metadata      map[string]any
	Trace         AIWriteTrace
}

type AISkillCreatorEarningsReversalInput struct {
	CreatorUserID int64
	BuyerUserID   int64
	SkillID       int64
	VersionID     int64
	RunID         int64
	Amount        float64
	Currency      string
	Metadata      map[string]any
	Trace         AIWriteTrace
}

type AISkillRepository interface {
	CreateSkill(ctx context.Context, skill *AISkill) error
	GetSkillByID(ctx context.Context, id int64) (*AISkill, error)
	GetSkillByCreatorAndID(ctx context.Context, creatorUserID, id int64) (*AISkill, error)
	UpdateSkill(ctx context.Context, skill *AISkill) error
}

// AISkillDomainRepository exposes the domain-shaped operations still needed by
// the HTTP presentation layer. Its implementation belongs to the repository
// layer; defining the port here keeps handlers independent from infrastructure.
type AISkillDomainRepository interface {
	GetSkillByID(ctx context.Context, id int64) (*domain.AISkill, error)
	GetSkillByUserAndID(ctx context.Context, userID, id int64) (*domain.AISkill, error)
	ListSkills(ctx context.Context, viewerUserID int64, isAdmin bool, params pagination.PaginationParams, filter domain.AISkillListFilter) ([]domain.AISkill, *pagination.PaginationResult, error)
	SetSkillInstall(ctx context.Context, skillID, userID int64, installed bool) error
	HasSkillInstall(ctx context.Context, skillID, userID int64) (bool, error)
	GetSkillInstallStates(ctx context.Context, userID int64, skillIDs []int64) (map[int64]bool, error)
	GetSkillInstallCounts(ctx context.Context, skillIDs []int64) (map[int64]int, error)
	UpdateSkill(ctx context.Context, skill *domain.AISkill) error
	GetSkillVersionByID(ctx context.Context, id int64) (*domain.AISkillVersion, error)
	ListSkillVersions(ctx context.Context, skillID int64) ([]domain.AISkillVersion, error)
	GetSkillReviewByID(ctx context.Context, id int64) (*domain.AISkillReview, error)
	GetSkillSettlementByID(ctx context.Context, id int64) (*domain.AISkillSettlement, error)
}

// AISkillDomainOperations is the handler-facing contract implemented by
// AISkillDomainService. Keeping it separate from the repository port makes the
// production dependency explicit while allowing focused handler test doubles.
type AISkillDomainOperations interface {
	AISkillDomainRepository
}

type AISkillVersionRepository interface {
	CreateVersion(ctx context.Context, version *AISkillVersion) error
	GetVersionByID(ctx context.Context, id int64) (*AISkillVersion, error)
	GetVersionByCreatorAndID(ctx context.Context, creatorUserID, id int64) (*AISkillVersion, error)
	GetLatestApprovedVersionBySkillID(ctx context.Context, skillID int64) (*AISkillVersion, error)
	UpdateVersion(ctx context.Context, version *AISkillVersion) error
}

type AISkillReviewRepository interface {
	CreateReview(ctx context.Context, review *AISkillReview) error
}

type AISkillRunRepository interface {
	CreateRun(ctx context.Context, run *AISkillRun) error
	GetRunByID(ctx context.Context, id int64) (*AISkillRun, error)
	UpdateRun(ctx context.Context, run *AISkillRun) error
}

type AISkillSettlementRepository interface {
	CreateSettlement(ctx context.Context, settlement *AISkillSettlement) error
	GetSettlementByRunID(ctx context.Context, runID int64) (*AISkillSettlement, error)
	UpdateSettlement(ctx context.Context, settlement *AISkillSettlement) error
}

type AISkillBalanceCharger interface {
	ChargeUserBalance(ctx context.Context, input AISkillBalanceChargeInput) (*AISkillBalanceChargeResult, error)
	RefundUserBalance(ctx context.Context, input AISkillBalanceRefundInput) (*AISkillBalanceRefundResult, error)
}

// AISkillBalanceLedgerRepository atomically claims a reference and applies the
// matching wallet mutation. Implementations must return the original result for
// duplicate calls with the same reference and attributes.
type AISkillBalanceLedgerRepository interface {
	ApplyAISkillBalanceCharge(ctx context.Context, input AISkillBalanceChargeInput) (*AISkillBalanceLedgerResult, error)
	ApplyAISkillBalanceRefund(ctx context.Context, input AISkillBalanceRefundInput) (*AISkillBalanceLedgerResult, error)
}

type AISkillCreatorEarningsCreditor interface {
	CreditCreatorEarnings(ctx context.Context, input AISkillCreatorEarningsInput) (float64, error)
	ReverseCreatorEarnings(ctx context.Context, input AISkillCreatorEarningsReversalInput) (float64, error)
}

type AISkillRuntimeGateway interface {
	Execute(ctx context.Context, req AISkillExecutionRequest) (*AISkillDispatchResult, error)
}

type AISkillOpenAIChatRuntimeInput struct {
	GroupID        *int64
	SessionHash    string
	PromptCacheKey string
	Model          string
	Body           []byte
	// BillingAPIKey, when set, attributes gateway token usage to the buyer key.
	// Marketplace settlement remains separate (per-run skill fee).
	BillingAPIKey *APIKey
}

type AISkillOpenAIChatRuntimeResult struct {
	Forward      *OpenAIForwardResult
	ResponseBody []byte
}

type AISkillOpenAIChatRuntime interface {
	ExecuteChatCompat(ctx context.Context, input AISkillOpenAIChatRuntimeInput) (*AISkillOpenAIChatRuntimeResult, error)
}

type AISkillOpenAIImageRuntimeInput struct {
	GroupID       *int64
	SessionHash   string
	Model         string
	Path          string
	ContentType   string
	Body          []byte
	BillingAPIKey *APIKey
}

type AISkillOpenAIImageRuntimeResult struct {
	Forward      *OpenAIForwardResult
	ResponseBody []byte
}

type AISkillOpenAIImageRuntime interface {
	ExecuteImages(ctx context.Context, input AISkillOpenAIImageRuntimeInput) (*AISkillOpenAIImageRuntimeResult, error)
}

type AISkillScriptRuntimeInput struct {
	RunID                  int64
	SkillID                int64
	VersionID              int64
	UserID                 int64
	Mode                   string
	Runtime                string
	ScriptName             string
	EntryPoint             string
	Protocol               string
	ArchivePath            string
	ArchiveBase64          string
	ApprovedArtifactDigest string
	VersionStatus          string
	ReviewerUserID         *int64
	TimeoutSeconds         int
	Environment            map[string]string
	Arguments              []map[string]any
	Parameters             map[string]any
	Trace                  AITraceRef
}

type AISkillScriptRuntimeResult struct {
	Status        string         `json:"status,omitempty"`
	ExternalJobID string         `json:"external_job_id,omitempty"`
	Output        map[string]any `json:"output,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type AISkillScriptRuntime interface {
	ExecuteScript(ctx context.Context, input AISkillScriptRuntimeInput) (*AISkillScriptRuntimeResult, error)
}

func normalizeAISkillType(raw string) string {
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

func normalizeAISkillVersionStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case AISkillVersionStatusDraft:
		return AISkillVersionStatusDraft
	case AISkillVersionStatusSubmitted:
		return AISkillVersionStatusSubmitted
	case AISkillVersionStatusApproved:
		return AISkillVersionStatusApproved
	case AISkillVersionStatusRejected:
		return AISkillVersionStatusRejected
	case AISkillVersionStatusDisabled:
		return AISkillVersionStatusDisabled
	default:
		return ""
	}
}

func normalizeAISkillBillingMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", AISkillBillingModeFree:
		return AISkillBillingModeFree
	case AISkillBillingModePerRun:
		return AISkillBillingModePerRun
	default:
		return ""
	}
}

func normalizeAISkillRunMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", AISkillRunModeUse:
		return AISkillRunModeUse
	case AISkillRunModeTest:
		return AISkillRunModeTest
	default:
		return ""
	}
}

func normalizeAISkillDispatchStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case AISkillRunStatusPrepared:
		return AISkillRunStatusPrepared
	case AISkillRunStatusSucceeded:
		return AISkillRunStatusSucceeded
	case AISkillRunStatusFailed:
		return AISkillRunStatusFailed
	default:
		return AISkillRunStatusDispatched
	}
}

func canAISkillVersionEdit(status string) bool {
	switch normalizeAISkillVersionStatus(status) {
	case AISkillVersionStatusDraft, AISkillVersionStatusRejected:
		return true
	default:
		return false
	}
}

func canAISkillVersionTestUseOrPublish(status string) bool {
	return normalizeAISkillVersionStatus(status) == AISkillVersionStatusApproved
}

func canAISkillVersionTransition(from, to string) bool {
	from = normalizeAISkillVersionStatus(from)
	to = normalizeAISkillVersionStatus(to)
	switch from {
	case AISkillVersionStatusDraft:
		return to == AISkillVersionStatusSubmitted || to == AISkillVersionStatusDisabled
	case AISkillVersionStatusSubmitted:
		return to == AISkillVersionStatusApproved || to == AISkillVersionStatusRejected || to == AISkillVersionStatusDisabled
	case AISkillVersionStatusApproved:
		return to == AISkillVersionStatusDisabled
	case AISkillVersionStatusRejected:
		return to == AISkillVersionStatusDraft || to == AISkillVersionStatusSubmitted || to == AISkillVersionStatusDisabled
	case AISkillVersionStatusDisabled:
		return to == AISkillVersionStatusDraft || to == AISkillVersionStatusSubmitted
	default:
		return false
	}
}

func transitionAISkillVersionStatus(from, to string) (string, error) {
	from = normalizeAISkillVersionStatus(from)
	to = normalizeAISkillVersionStatus(to)
	if from == "" || to == "" {
		return "", ErrAISkillVersionStatusInvalid
	}
	if !canAISkillVersionTransition(from, to) {
		return "", ErrAISkillVersionTransitionInvalid
	}
	return to, nil
}

func normalizeAISkillBillingPolicy(policy AISkillBillingPolicy) (AISkillBillingPolicy, error) {
	policy.Mode = normalizeAISkillBillingMode(policy.Mode)
	if policy.Mode == "" {
		return AISkillBillingPolicy{}, ErrAISkillBillingPolicyInvalid
	}
	policy.Currency = strings.TrimSpace(policy.Currency)
	if policy.Currency == "" {
		policy.Currency = aiSkillDefaultCurrency
	}
	if math.IsNaN(policy.PricePerRun) || math.IsInf(policy.PricePerRun, 0) || policy.PricePerRun < 0 {
		return AISkillBillingPolicy{}, ErrAISkillBillingPolicyInvalid
	}
	if math.IsNaN(policy.PlatformCommissionRate) || math.IsInf(policy.PlatformCommissionRate, 0) {
		return AISkillBillingPolicy{}, ErrAISkillBillingPolicyInvalid
	}
	if policy.PlatformCommissionRate < 0 || policy.PlatformCommissionRate > 1 {
		return AISkillBillingPolicy{}, ErrAISkillBillingPolicyInvalid
	}
	if policy.Mode == AISkillBillingModeFree {
		policy.PricePerRun = 0
		policy.PlatformCommissionRate = 0
		return policy, nil
	}
	if policy.PricePerRun <= 0 {
		return AISkillBillingPolicy{}, ErrAISkillBillingPolicyInvalid
	}
	policy.PricePerRun = roundTo(policy.PricePerRun, 8)
	policy.PlatformCommissionRate = roundTo(policy.PlatformCommissionRate, 8)
	return policy, nil
}

func buildAISkillSettlementQuote(policy AISkillBillingPolicy) (*AISkillSettlementQuote, error) {
	policy, err := normalizeAISkillBillingPolicy(policy)
	if err != nil {
		return nil, err
	}
	if policy.Mode == AISkillBillingModeFree {
		return &AISkillSettlementQuote{
			BillingMode: policy.Mode,
			Currency:    policy.Currency,
		}, nil
	}
	total := roundTo(policy.PricePerRun, 8)
	platform := roundTo(total*policy.PlatformCommissionRate, 8)
	creator := roundTo(total-platform, 8)
	if creator < 0 {
		creator = 0
	}
	return &AISkillSettlementQuote{
		BillingMode:            policy.Mode,
		Currency:               policy.Currency,
		TotalAmount:            total,
		PlatformAmount:         platform,
		CreatorAmount:          creator,
		PlatformCommissionRate: policy.PlatformCommissionRate,
	}, nil
}

func normalizeAISkillExecutionSpec(skillType string, spec AISkillExecutionSpec) (AISkillExecutionSpec, error) {
	skillType = normalizeAISkillType(skillType)
	if skillType == "" {
		return AISkillExecutionSpec{}, ErrAISkillTypeInvalid
	}
	spec.Type = skillType
	switch skillType {
	case AISkillTypePromptChat:
		if spec.PromptChat == nil || spec.PromptImage != nil || spec.Script != nil {
			return AISkillExecutionSpec{}, ErrAISkillExecutionSpecInvalid
		}
		spec.PromptChat = &AISkillPromptChatSpec{
			Model:              strings.TrimSpace(spec.PromptChat.Model),
			SystemPrompt:       strings.TrimSpace(spec.PromptChat.SystemPrompt),
			UserPromptTemplate: strings.TrimSpace(spec.PromptChat.UserPromptTemplate),
			Variables:          cloneAIMapSlice(spec.PromptChat.Variables),
			ResponseFormat:     cloneAIMap(spec.PromptChat.ResponseFormat),
		}
		if spec.PromptChat.UserPromptTemplate == "" {
			return AISkillExecutionSpec{}, ErrAISkillExecutionSpecInvalid
		}
	case AISkillTypePromptImage:
		if spec.PromptImage == nil || spec.PromptChat != nil || spec.Script != nil {
			return AISkillExecutionSpec{}, ErrAISkillExecutionSpecInvalid
		}
		spec.PromptImage = &AISkillPromptImageSpec{
			Model:                  strings.TrimSpace(spec.PromptImage.Model),
			PromptTemplate:         strings.TrimSpace(spec.PromptImage.PromptTemplate),
			NegativePromptTemplate: strings.TrimSpace(spec.PromptImage.NegativePromptTemplate),
			Size:                   strings.TrimSpace(spec.PromptImage.Size),
			ImageCount:             spec.PromptImage.ImageCount,
			Variables:              cloneAIMapSlice(spec.PromptImage.Variables),
		}
		if spec.PromptImage.PromptTemplate == "" {
			return AISkillExecutionSpec{}, ErrAISkillExecutionSpecInvalid
		}
		if spec.PromptImage.ImageCount <= 0 {
			spec.PromptImage.ImageCount = 1
		}
	case AISkillTypeScript:
		if spec.Script == nil || spec.PromptChat != nil || spec.PromptImage != nil {
			return AISkillExecutionSpec{}, ErrAISkillExecutionSpecInvalid
		}
		spec.Script = &AISkillScriptSpec{
			Runtime:        strings.TrimSpace(spec.Script.Runtime),
			ScriptName:     strings.TrimSpace(spec.Script.ScriptName),
			EntryPoint:     strings.TrimSpace(spec.Script.EntryPoint),
			Protocol:       strings.TrimSpace(spec.Script.Protocol),
			ArchivePath:    strings.TrimSpace(spec.Script.ArchivePath),
			ArchiveBase64:  strings.TrimSpace(spec.Script.ArchiveBase64),
			TimeoutSeconds: spec.Script.TimeoutSeconds,
			Environment:    cloneAISkillStringMap(spec.Script.Environment),
			Arguments:      cloneAIMapSlice(spec.Script.Arguments),
		}
		if spec.Script.ScriptName == "" && spec.Script.EntryPoint == "" {
			return AISkillExecutionSpec{}, ErrAISkillExecutionSpecInvalid
		}
		if spec.Script.TimeoutSeconds <= 0 {
			spec.Script.TimeoutSeconds = 30
		}
	default:
		return AISkillExecutionSpec{}, ErrAISkillExecutionSpecInvalid
	}
	return spec, nil
}

func cloneAISkillExecutionSpec(spec AISkillExecutionSpec) AISkillExecutionSpec {
	cloned := AISkillExecutionSpec{Type: spec.Type}
	if spec.PromptChat != nil {
		cloned.PromptChat = &AISkillPromptChatSpec{
			Model:              spec.PromptChat.Model,
			SystemPrompt:       spec.PromptChat.SystemPrompt,
			UserPromptTemplate: spec.PromptChat.UserPromptTemplate,
			Variables:          cloneAIMapSlice(spec.PromptChat.Variables),
			ResponseFormat:     cloneAIMap(spec.PromptChat.ResponseFormat),
		}
	}
	if spec.PromptImage != nil {
		cloned.PromptImage = &AISkillPromptImageSpec{
			Model:                  spec.PromptImage.Model,
			PromptTemplate:         spec.PromptImage.PromptTemplate,
			NegativePromptTemplate: spec.PromptImage.NegativePromptTemplate,
			Size:                   spec.PromptImage.Size,
			ImageCount:             spec.PromptImage.ImageCount,
			Variables:              cloneAIMapSlice(spec.PromptImage.Variables),
		}
	}
	if spec.Script != nil {
		cloned.Script = &AISkillScriptSpec{
			Runtime:        spec.Script.Runtime,
			ScriptName:     spec.Script.ScriptName,
			EntryPoint:     spec.Script.EntryPoint,
			Protocol:       spec.Script.Protocol,
			ArchivePath:    spec.Script.ArchivePath,
			ArchiveBase64:  spec.Script.ArchiveBase64,
			TimeoutSeconds: spec.Script.TimeoutSeconds,
			Environment:    cloneAISkillStringMap(spec.Script.Environment),
			Arguments:      cloneAIMapSlice(spec.Script.Arguments),
		}
	}
	return cloned
}

func cloneAISkillRunAttachments(src []AISkillRunAttachment) []AISkillRunAttachment {
	if len(src) == 0 {
		return []AISkillRunAttachment{}
	}
	dst := make([]AISkillRunAttachment, 0, len(src))
	dst = append(dst, src...)
	return dst
}

func cloneAISkillStringMap(src map[string]string) map[string]string {
	if src == nil {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func resetAISkillReviewState(version *AISkillVersion) {
	if version == nil {
		return
	}
	version.ReviewComment = ""
	version.ReviewerUserID = nil
	version.ReviewedAt = nil
	version.ApprovedAt = nil
	version.RejectedAt = nil
	version.DisabledAt = nil
}
