package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type AITraceRef = domain.AITraceRef
type AISession = domain.AISession
type AISessionMessage = domain.AISessionMessage
type AIPromptTemplate = domain.AIPromptTemplate
type AIPromptTemplateVersion = domain.AIPromptTemplateVersion
type AIGenerationJob = domain.AIGenerationJob
type AIAsset = domain.AIAsset
type AIAuditLog = domain.AIAuditLog

const (
	AIVisibilityPrivate  = domain.AIVisibilityPrivate
	AIVisibilityUnlisted = domain.AIVisibilityUnlisted
	AIVisibilityPublic   = domain.AIVisibilityPublic

	AIModerationStateNormal        = domain.AIModerationStateNormal
	AIModerationStateForcedPrivate = domain.AIModerationStateForcedPrivate
	AIModerationStateBlocked       = domain.AIModerationStateBlocked

	AISessionStatusActive   = domain.AISessionStatusActive
	AISessionStatusArchived = domain.AISessionStatusArchived

	AIMessageRoleSystem    = domain.AIMessageRoleSystem
	AIMessageRoleUser      = domain.AIMessageRoleUser
	AIMessageRoleAssistant = domain.AIMessageRoleAssistant
	AIMessageRoleTool      = domain.AIMessageRoleTool

	AIMessageStatusAccepted  = domain.AIMessageStatusAccepted
	AIMessageStatusQueued    = domain.AIMessageStatusQueued
	AIMessageStatusCompleted = domain.AIMessageStatusCompleted
	AIMessageStatusFailed    = domain.AIMessageStatusFailed

	AIGenerationJobStatusQueued    = domain.AIGenerationJobStatusQueued
	AIGenerationJobStatusRunning   = domain.AIGenerationJobStatusRunning
	AIGenerationJobStatusSucceeded = domain.AIGenerationJobStatusSucceeded
	AIGenerationJobStatusFailed    = domain.AIGenerationJobStatusFailed
	AIGenerationJobStatusCanceled  = domain.AIGenerationJobStatusCanceled

	AIAssetTypeImage = domain.AIAssetTypeImage

	AIAssetStatusPending = domain.AIAssetStatusPending
	AIAssetStatusReady   = domain.AIAssetStatusReady
	AIAssetStatusHidden  = domain.AIAssetStatusHidden
	AIAssetStatusDeleted = domain.AIAssetStatusDeleted
)

var (
	ErrAISessionNotFound        = domain.ErrAISessionNotFound
	ErrAISessionMessageNotFound = domain.ErrAISessionMessageNotFound
	ErrAIPromptTemplateNotFound = domain.ErrAIPromptTemplateNotFound
	ErrAIGenerationJobNotFound  = domain.ErrAIGenerationJobNotFound
	ErrAIAssetNotFound          = domain.ErrAIAssetNotFound
)

type AIWriteTrace = domain.AITraceRef

type AIListPromptTemplatesFilter struct {
	Scope           string
	Visibility      string
	Status          string
	ModerationState string
	Search          string
	OwnerUserID     *int64
	GroupID         *int64
}

type AIListGenerationJobsFilter struct {
	Status           string
	SessionID        *int64
	PromptTemplateID *int64
	GroupID          *int64
}

type AIListAssetsFilter struct {
	Status           string
	Visibility       string
	ModerationState  string
	GenerationJobID  *int64
	SessionID        *int64
	PromptTemplateID *int64
	GroupID          *int64
	Featured         *bool
	Search           string
}

type AIListAuditLogsFilter struct {
	EntityType     string
	EntityID       *int64
	Action         string
	RequestID      string
	OperatorUserID *int64
}

type AICreateSessionInput struct {
	Title        string
	SystemPrompt *string
	Metadata     map[string]any
	Trace        AIWriteTrace
}

type AIUpdateSessionInput struct {
	Title        *string
	Status       *string
	SystemPrompt **string
	Metadata     *map[string]any
	Trace        AIWriteTrace
}

type AISendMessageInput struct {
	Role                   string
	Content                string
	ContentParts           []map[string]any
	Model                  string
	Provider               string
	Status                 string
	ReplyToMessageID       *int64
	CreateAssistantReply   bool
	AssistantReplyModel    string
	AssistantReplyProvider string
	AssistantReplyStatus   string
	AssistantReplyContent  string
	AssistantReplyMetadata map[string]any
	Trace                  AIWriteTrace
}

type AICreatePromptTemplateInput struct {
	Title           string
	Description     *string
	Category        *string
	Tags            []string
	Visibility      string
	ModerationState string
	ModelHint       *string
	Content         string
	Variables       []map[string]any
	Metadata        map[string]any
	CoverAssetID    *int64
	ChangeNote      *string
	Trace           AIWriteTrace
}

type AIUpdatePromptTemplateInput struct {
	Title           *string
	Description     **string
	Category        **string
	Tags            *[]string
	Visibility      *string
	ModerationState *string
	ModelHint       **string
	Content         *string
	Variables       *[]map[string]any
	Metadata        *map[string]any
	CoverAssetID    **int64
	ChangeNote      *string
	Trace           AIWriteTrace
}

type AICreateGenerationJobInput struct {
	SessionID        *int64
	PromptTemplateID *int64
	Model            string
	Prompt           string
	NegativePrompt   *string
	Size             *string
	ImageCount       *int
	Seed             *int64
	Parameters       map[string]any
	Status           string
	Trace            AIWriteTrace
}

type AIUpdateGenerationJobInput struct {
	Status         *string
	ErrorMessage   *string
	Parameters     *map[string]any
	Model          *string
	Prompt         *string
	NegativePrompt *string
	Size           *string
	ImageCount     *int
	Seed           *int64
	Trace          AIWriteTrace
}

type AIAssetInput struct {
	GenerationJobID  *int64
	SessionID        *int64
	PromptTemplateID *int64
	AssetType        string
	Status           string
	Visibility       string
	ModerationState  string
	StorageKind      *string
	StoragePath      *string
	SourceURL        *string
	MIMEType         *string
	Width            *int
	Height           *int
	ByteSize         *int64
	Checksum         *string
	Metadata         map[string]any
	Trace            AIWriteTrace
}

type AIUpdateAssetInput struct {
	Status          *string
	Visibility      *string
	ModerationState *string
	StorageKind     **string
	StoragePath     **string
	SourceURL       **string
	MIMEType        **string
	Width           **int
	Height          **int
	ByteSize        **int64
	Checksum        **string
	Metadata        *map[string]any
	Trace           AIWriteTrace
}

type AIAuditLogInput struct {
	OperatorUserID *int64
	OwnerUserID    *int64
	EntityType     string
	EntityID       *int64
	Action         string
	Reason         *string
	BeforeState    map[string]any
	AfterState     map[string]any
	Trace          AIWriteTrace
}

type AICenterRepository interface {
	CreateSession(ctx context.Context, session *AISession) error
	GetSessionByID(ctx context.Context, id int64) (*AISession, error)
	GetSessionByUserAndID(ctx context.Context, userID, id int64) (*AISession, error)
	ListSessions(ctx context.Context, userID int64, params pagination.PaginationParams, status string) ([]AISession, *pagination.PaginationResult, error)
	UpdateSession(ctx context.Context, session *AISession) error
	DeleteSession(ctx context.Context, id int64) error

	CreateSessionMessages(ctx context.Context, messages []*AISessionMessage) error
	ListSessionMessages(ctx context.Context, sessionID int64, params pagination.PaginationParams) ([]AISessionMessage, *pagination.PaginationResult, error)

	CreatePromptTemplate(ctx context.Context, template *AIPromptTemplate, version *AIPromptTemplateVersion) error
	UpdatePromptTemplate(ctx context.Context, template *AIPromptTemplate, version *AIPromptTemplateVersion) error
	GetPromptTemplateByID(ctx context.Context, id int64) (*AIPromptTemplate, error)
	GetPromptTemplateByUserAndID(ctx context.Context, userID, id int64) (*AIPromptTemplate, error)
	ListPromptTemplates(ctx context.Context, viewerUserID int64, isAdmin bool, params pagination.PaginationParams, filter AIListPromptTemplatesFilter) ([]AIPromptTemplate, *pagination.PaginationResult, error)
	DeletePromptTemplate(ctx context.Context, id int64) error

	CreateGenerationJob(ctx context.Context, job *AIGenerationJob) error
	UpdateGenerationJob(ctx context.Context, job *AIGenerationJob) error
	GetGenerationJobByID(ctx context.Context, id int64) (*AIGenerationJob, error)
	GetGenerationJobByUserAndID(ctx context.Context, userID, id int64) (*AIGenerationJob, error)
	ListGenerationJobs(ctx context.Context, userID int64, isAdmin bool, params pagination.PaginationParams, filter AIListGenerationJobsFilter) ([]AIGenerationJob, *pagination.PaginationResult, error)

	CreateAssets(ctx context.Context, assets []*AIAsset) error
	UpdateAsset(ctx context.Context, asset *AIAsset) error
	GetAssetByID(ctx context.Context, id int64) (*AIAsset, error)
	GetAssetByUserAndID(ctx context.Context, userID, id int64) (*AIAsset, error)
	ListAssets(ctx context.Context, userID int64, isAdmin bool, params pagination.PaginationParams, filter AIListAssetsFilter) ([]AIAsset, *pagination.PaginationResult, error)

	CreateAuditLog(ctx context.Context, audit *AIAuditLog) error
	ListAuditLogs(ctx context.Context, params pagination.PaginationParams, filter AIListAuditLogsFilter) ([]AIAuditLog, *pagination.PaginationResult, error)
}

type AICenterService struct {
	repo         AICenterRepository
	mediaService *MediaService
}

func NewAICenterService(repo AICenterRepository, mediaService *MediaService) *AICenterService {
	return &AICenterService{repo: repo, mediaService: mediaService}
}

func (s *AICenterService) requireRepo() (AICenterRepository, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("AI_CENTER_REPO_UNAVAILABLE: ai center repository not available")
	}
	return s.repo, nil
}

func normalizeAIWriteTrace(ctx context.Context, trace AIWriteTrace) AIWriteTrace {
	if strings.TrimSpace(trace.RequestID) == "" && ctx != nil {
		if requestID, _ := ctx.Value(ctxkey.RequestID).(string); strings.TrimSpace(requestID) != "" {
			trace.RequestID = strings.TrimSpace(requestID)
		}
	}
	return trace
}

func normalizeAIStringPtr(v **string) *string {
	if v == nil || *v == nil {
		return nil
	}
	return *v
}

func normalizeAIInt64Ptr(v **int64) *int64 {
	if v == nil || *v == nil {
		return nil
	}
	return *v
}

func normalizeAIIntPtr(v **int) *int {
	if v == nil || *v == nil {
		return nil
	}
	return *v
}

func cloneAIMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneAIMapSlice(src []map[string]any) []map[string]any {
	if len(src) == 0 {
		return []map[string]any{}
	}
	dst := make([]map[string]any, 0, len(src))
	for i := range src {
		dst = append(dst, cloneAIMap(src[i]))
	}
	return dst
}
