package repository

import (
	"context"
	"errors"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/aiasset"
	"github.com/Wei-Shaw/sub2api/ent/aiauditlog"
	"github.com/Wei-Shaw/sub2api/ent/aigenerationjob"
	"github.com/Wei-Shaw/sub2api/ent/aiprompttemplate"
	"github.com/Wei-Shaw/sub2api/ent/aisession"
	"github.com/Wei-Shaw/sub2api/ent/aisessionmessage"
	dbpredicate "github.com/Wei-Shaw/sub2api/ent/predicate"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"

	entsql "entgo.io/ent/dialect/sql"
)

type aiCenterRepository struct {
	client *dbent.Client
}

func NewAICenterRepository(client *dbent.Client) service.AICenterRepository {
	return &aiCenterRepository{client: client}
}

func (r *aiCenterRepository) CreateSession(ctx context.Context, session *service.AISession) error {
	if session == nil {
		return nil
	}
	created, err := clientFromContext(ctx, r.client).AISession.Create().
		SetUserID(session.UserID).
		SetTitle(session.Title).
		SetStatus(session.Status).
		SetMetadata(cloneStringAnyMap(session.Metadata)).
		SetNillableSystemPrompt(nillableString(session.SystemPrompt)).
		SetNillableLastMessageAt(session.LastMessageAt).
		SetNillableRequestID(nillableString(session.Trace.RequestID)).
		SetNillableUsageLogID(session.Trace.UsageLogID).
		SetNillableAPIKeyID(session.Trace.APIKeyID).
		SetNillableGroupID(session.Trace.GroupID).
		Save(ctx)
	if err != nil {
		return err
	}
	applyAISessionEntity(session, created)
	return nil
}

func (r *aiCenterRepository) GetSessionByID(ctx context.Context, id int64) (*service.AISession, error) {
	m, err := r.client.AISession.Query().Where(aisession.IDEQ(id)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrAISessionNotFound, nil)
	}
	return aiSessionEntityToService(m), nil
}

func (r *aiCenterRepository) GetSessionByUserAndID(ctx context.Context, userID, id int64) (*service.AISession, error) {
	m, err := r.client.AISession.Query().
		Where(aisession.IDEQ(id), aisession.UserIDEQ(userID)).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrAISessionNotFound, nil)
	}
	return aiSessionEntityToService(m), nil
}

func (r *aiCenterRepository) ListSessions(ctx context.Context, userID int64, params pagination.PaginationParams, status string) ([]service.AISession, *pagination.PaginationResult, error) {
	q := r.client.AISession.Query().Where(aisession.UserIDEQ(userID))
	if status = strings.TrimSpace(status); status != "" {
		q = q.Where(aisession.StatusEQ(status))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	items, err := q.
		Order(dbent.Desc(aisession.FieldUpdatedAt), dbent.Desc(aisession.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make([]service.AISession, 0, len(items))
	for i := range items {
		if s := aiSessionEntityToService(items[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out, paginationResultFromTotal(int64(total), params), nil
}

func (r *aiCenterRepository) UpdateSession(ctx context.Context, session *service.AISession) error {
	if session == nil {
		return nil
	}
	builder := clientFromContext(ctx, r.client).AISession.UpdateOneID(session.ID).
		SetTitle(session.Title).
		SetStatus(session.Status).
		SetMetadata(cloneStringAnyMap(session.Metadata)).
		SetNillableSystemPrompt(nillableString(session.SystemPrompt)).
		SetNillableLastMessageAt(session.LastMessageAt).
		SetNillableRequestID(nillableString(session.Trace.RequestID)).
		SetNillableUsageLogID(session.Trace.UsageLogID).
		SetNillableAPIKeyID(session.Trace.APIKeyID).
		SetNillableGroupID(session.Trace.GroupID)
	updated, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrAISessionNotFound, nil)
	}
	applyAISessionEntity(session, updated)
	return nil
}

func (r *aiCenterRepository) DeleteSession(ctx context.Context, id int64) error {
	return clientFromContext(ctx, r.client).AISession.DeleteOneID(id).Exec(ctx)
}

func (r *aiCenterRepository) CreateSessionMessages(ctx context.Context, messages []*service.AISessionMessage) error {
	if len(messages) == 0 {
		return nil
	}
	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}
	client := r.client
	txCtx := ctx
	if tx != nil {
		client = tx.Client()
		txCtx = dbent.NewTxContext(ctx, tx)
	}
	for i := range messages {
		msg := messages[i]
		created, createErr := client.AISessionMessage.Create().
			SetSessionID(msg.SessionID).
			SetUserID(msg.UserID).
			SetRole(msg.Role).
			SetStatus(msg.Status).
			SetContent(msg.Content).
			SetContentParts(cloneStringAnySlice(msg.ContentParts)).
			SetNillableReplyToMessageID(msg.ReplyToMessageID).
			SetNillableModel(nillableString(msg.Model)).
			SetNillableProvider(nillableString(msg.Provider)).
			SetMetadata(cloneStringAnyMap(msg.Metadata)).
			SetNillableErrorMessage(nillableString(msg.ErrorMessage)).
			SetNillableRequestID(nillableString(msg.Trace.RequestID)).
			SetNillableUsageLogID(msg.Trace.UsageLogID).
			SetNillableAPIKeyID(msg.Trace.APIKeyID).
			SetNillableGroupID(msg.Trace.GroupID).
			Save(txCtx)
		if createErr != nil {
			if tx != nil {
				_ = tx.Rollback()
			}
			return createErr
		}
		applyAISessionMessageEntity(msg, created)
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (r *aiCenterRepository) ListSessionMessages(ctx context.Context, sessionID int64, params pagination.PaginationParams) ([]service.AISessionMessage, *pagination.PaginationResult, error) {
	q := r.client.AISessionMessage.Query().Where(aisessionmessage.SessionIDEQ(sessionID))
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	items, err := q.
		Order(dbent.Asc(aisessionmessage.FieldCreatedAt), dbent.Asc(aisessionmessage.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make([]service.AISessionMessage, 0, len(items))
	for i := range items {
		if s := aiSessionMessageEntityToService(items[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out, paginationResultFromTotal(int64(total), params), nil
}

func (r *aiCenterRepository) CreatePromptTemplate(ctx context.Context, template *service.AIPromptTemplate, version *service.AIPromptTemplateVersion) error {
	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}
	client := r.client
	txCtx := ctx
	if tx != nil {
		client = tx.Client()
		txCtx = dbent.NewTxContext(ctx, tx)
	}
	createdTemplate, err := client.AIPromptTemplate.Create().
		SetUserID(template.UserID).
		SetTitle(template.Title).
		SetNillableDescription(nillableString(template.Description)).
		SetNillableCategory(nillableString(template.Category)).
		SetTags(cloneStringSlice(template.Tags)).
		SetVisibility(template.Visibility).
		SetModerationState(template.ModerationState).
		SetCurrentVersion(template.CurrentVersion).
		SetContent(template.Content).
		SetNillableModelHint(nillableString(template.ModelHint)).
		SetNillableCoverAssetID(template.CoverAssetID).
		SetMetadata(cloneStringAnyMap(template.Metadata)).
		SetNillableRequestID(nillableString(template.Trace.RequestID)).
		SetNillableUsageLogID(template.Trace.UsageLogID).
		SetNillableAPIKeyID(template.Trace.APIKeyID).
		SetNillableGroupID(template.Trace.GroupID).
		Save(txCtx)
	if err != nil {
		if tx != nil {
			_ = tx.Rollback()
		}
		return err
	}
	template.ID = createdTemplate.ID
	template.CreatedAt = createdTemplate.CreatedAt
	template.UpdatedAt = createdTemplate.UpdatedAt

	if version != nil {
		version.TemplateID = createdTemplate.ID
		createdVersion, createVersionErr := client.AIPromptTemplateVersion.Create().
			SetTemplateID(version.TemplateID).
			SetUserID(version.UserID).
			SetVersion(version.Version).
			SetTitle(version.Title).
			SetContent(version.Content).
			SetNillableModelHint(nillableString(version.ModelHint)).
			SetVariables(cloneStringAnySlice(version.Variables)).
			SetNillableChangeNote(nillableString(version.ChangeNote)).
			SetMetadata(cloneStringAnyMap(version.Metadata)).
			SetNillableRequestID(nillableString(version.Trace.RequestID)).
			SetNillableUsageLogID(version.Trace.UsageLogID).
			SetNillableAPIKeyID(version.Trace.APIKeyID).
			SetNillableGroupID(version.Trace.GroupID).
			Save(txCtx)
		if createVersionErr != nil {
			if tx != nil {
				_ = tx.Rollback()
			}
			return createVersionErr
		}
		applyAIPromptTemplateVersionEntity(version, createdVersion)
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (r *aiCenterRepository) UpdatePromptTemplate(ctx context.Context, template *service.AIPromptTemplate, version *service.AIPromptTemplateVersion) error {
	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}
	client := r.client
	txCtx := ctx
	if tx != nil {
		client = tx.Client()
		txCtx = dbent.NewTxContext(ctx, tx)
	}
	updated, err := client.AIPromptTemplate.UpdateOneID(template.ID).
		SetTitle(template.Title).
		SetNillableDescription(nillableString(template.Description)).
		SetNillableCategory(nillableString(template.Category)).
		SetTags(cloneStringSlice(template.Tags)).
		SetVisibility(template.Visibility).
		SetModerationState(template.ModerationState).
		SetCurrentVersion(template.CurrentVersion).
		SetContent(template.Content).
		SetNillableModelHint(nillableString(template.ModelHint)).
		SetNillableCoverAssetID(template.CoverAssetID).
		SetMetadata(cloneStringAnyMap(template.Metadata)).
		SetNillableRequestID(nillableString(template.Trace.RequestID)).
		SetNillableUsageLogID(template.Trace.UsageLogID).
		SetNillableAPIKeyID(template.Trace.APIKeyID).
		SetNillableGroupID(template.Trace.GroupID).
		Save(txCtx)
	if err != nil {
		if tx != nil {
			_ = tx.Rollback()
		}
		return translatePersistenceError(err, service.ErrAIPromptTemplateNotFound, nil)
	}
	applyAIPromptTemplateEntity(template, updated)

	if version != nil {
		version.TemplateID = updated.ID
		createdVersion, createVersionErr := client.AIPromptTemplateVersion.Create().
			SetTemplateID(version.TemplateID).
			SetUserID(version.UserID).
			SetVersion(version.Version).
			SetTitle(version.Title).
			SetContent(version.Content).
			SetNillableModelHint(nillableString(version.ModelHint)).
			SetVariables(cloneStringAnySlice(version.Variables)).
			SetNillableChangeNote(nillableString(version.ChangeNote)).
			SetMetadata(cloneStringAnyMap(version.Metadata)).
			SetNillableRequestID(nillableString(version.Trace.RequestID)).
			SetNillableUsageLogID(version.Trace.UsageLogID).
			SetNillableAPIKeyID(version.Trace.APIKeyID).
			SetNillableGroupID(version.Trace.GroupID).
			Save(txCtx)
		if createVersionErr != nil {
			if tx != nil {
				_ = tx.Rollback()
			}
			return createVersionErr
		}
		applyAIPromptTemplateVersionEntity(version, createdVersion)
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (r *aiCenterRepository) GetPromptTemplateByID(ctx context.Context, id int64) (*service.AIPromptTemplate, error) {
	m, err := r.client.AIPromptTemplate.Query().Where(aiprompttemplate.IDEQ(id)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrAIPromptTemplateNotFound, nil)
	}
	return aiPromptTemplateEntityToService(m), nil
}

func (r *aiCenterRepository) GetPromptTemplateByUserAndID(ctx context.Context, userID, id int64) (*service.AIPromptTemplate, error) {
	m, err := r.client.AIPromptTemplate.Query().Where(aiprompttemplate.IDEQ(id), aiprompttemplate.UserIDEQ(userID)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrAIPromptTemplateNotFound, nil)
	}
	return aiPromptTemplateEntityToService(m), nil
}

func (r *aiCenterRepository) ListPromptTemplates(ctx context.Context, viewerUserID int64, isAdmin bool, params pagination.PaginationParams, filter service.AIListPromptTemplatesFilter) ([]service.AIPromptTemplate, *pagination.PaginationResult, error) {
	q := r.client.AIPromptTemplate.Query()
	switch normalizeAIPromptScope(filter.Scope) {
	case "mine":
		if filter.OwnerUserID != nil {
			q = q.Where(aiprompttemplate.UserIDEQ(*filter.OwnerUserID))
		} else {
			q = q.Where(aiprompttemplate.UserIDEQ(viewerUserID))
		}
	case "library":
		q = q.Where(aiprompttemplate.VisibilityEQ(domainVisibilityPublic), aiprompttemplate.ModerationStateEQ(domainModerationNormal))
		if filter.Visibility != "" {
			q = q.Where(aiprompttemplate.VisibilityEQ(filter.Visibility))
		}
	case "all":
		if !isAdmin {
			q = q.Where(aiprompttemplate.UserIDEQ(viewerUserID))
		}
	}
	if s := strings.TrimSpace(filter.Visibility); s != "" {
		q = q.Where(aiprompttemplate.VisibilityEQ(s))
	}
	if s := strings.TrimSpace(filter.ModerationState); s != "" {
		q = q.Where(aiprompttemplate.ModerationStateEQ(s))
	}
	if filter.GroupID != nil {
		q = q.Where(aiprompttemplate.GroupIDEQ(*filter.GroupID))
	}
	// Status 是派生字段（metadata.status 覆盖 + moderation_state/visibility 推导），下推 SQL 保持与 promptTemplateStatus 等价
	if status := strings.TrimSpace(filter.Status); status != "" && status != "all" {
		q = q.Where(dbpredicate.AIPromptTemplate(func(s *entsql.Selector) {
			s.Where(promptTemplateStatusPredicate(s, status))
		}))
	}
	// Search 下推 SQL：按字段分别 OR（原 Go 实现拼成单一 haystack 会跨字段边界误配，此处改为按字段匹配是等价语义微调）
	// 转义 LIKE 元字符 \ % _，与原 strings.Contains 的字面子串语义严格一致（否则用户输入的 %/_ 会被当通配符）。
	if search := strings.ToLower(strings.TrimSpace(filter.Search)); search != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(search)
		like := "%" + escaped + "%"
		q = q.Where(dbpredicate.AIPromptTemplate(func(s *entsql.Selector) {
			s.Where(promptTemplateSearchPredicate(s, like))
		}))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	items, err := q.Order(dbent.Desc(aiprompttemplate.FieldUpdatedAt), dbent.Desc(aiprompttemplate.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make([]service.AIPromptTemplate, 0, len(items))
	for i := range items {
		if s := aiPromptTemplateEntityToService(items[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out, paginationResultFromTotal(int64(total), params), nil
}

// promptTemplateStatusPredicate 生成派生 status 过滤谓词，与 promptTemplateStatus 严格等价
func promptTemplateStatusPredicate(s *entsql.Selector, status string) *entsql.Predicate {
	return entsql.P(func(b *entsql.Builder) {
		b.WriteString("COALESCE(NULLIF(TRIM(").
			Ident(s.C(aiprompttemplate.FieldMetadata)).
			WriteString("->>'status'), ''), CASE WHEN ").
			Ident(s.C(aiprompttemplate.FieldModerationState)).
			WriteString(" = 'normal' AND ").
			Ident(s.C(aiprompttemplate.FieldVisibility)).
			WriteString(" = 'public' THEN 'published' WHEN ").
			Ident(s.C(aiprompttemplate.FieldModerationState)).
			WriteString(" = 'normal' THEN 'draft' WHEN ").
			Ident(s.C(aiprompttemplate.FieldModerationState)).
			WriteString(" = 'blocked' THEN 'hidden' ELSE 'archived' END) = ").
			Arg(status)
	})
}

// promptTemplateSearchPredicate 生成关键字过滤谓词，like 需已 LOWER + 前后加通配；用 Or 组合确保括号包裹
func promptTemplateSearchPredicate(s *entsql.Selector, like string) *entsql.Predicate {
	likeCol := func(expr string) *entsql.Predicate {
		return entsql.P(func(b *entsql.Builder) {
			b.WriteString(expr).WriteString(" LIKE ").Arg(like).WriteString(` ESCAPE '\'`)
		})
	}
	tagsMatch := entsql.P(func(b *entsql.Builder) {
		b.WriteString("EXISTS (SELECT 1 FROM jsonb_array_elements_text(").
			Ident(s.C(aiprompttemplate.FieldTags)).
			WriteString(") elem WHERE LOWER(elem) LIKE ").Arg(like).WriteString(` ESCAPE '\')`)
	})
	return entsql.Or(
		likeCol("LOWER("+s.C(aiprompttemplate.FieldTitle)+")"),
		likeCol("LOWER(COALESCE("+s.C(aiprompttemplate.FieldDescription)+", ''))"),
		likeCol("LOWER(COALESCE("+s.C(aiprompttemplate.FieldCategory)+", ''))"),
		likeCol("LOWER("+s.C(aiprompttemplate.FieldContent)+")"),
		tagsMatch,
	)
}

func (r *aiCenterRepository) DeletePromptTemplate(ctx context.Context, id int64) error {
	return clientFromContext(ctx, r.client).AIPromptTemplate.DeleteOneID(id).Exec(ctx)
}

func (r *aiCenterRepository) CreateGenerationJob(ctx context.Context, job *service.AIGenerationJob) error {
	if job == nil {
		return nil
	}
	created, err := clientFromContext(ctx, r.client).AIGenerationJob.Create().
		SetUserID(job.UserID).
		SetNillableSessionID(job.SessionID).
		SetNillablePromptTemplateID(job.PromptTemplateID).
		SetStatus(job.Status).
		SetModel(job.Model).
		SetPrompt(job.Prompt).
		SetNillableNegativePrompt(nillableString(job.NegativePrompt)).
		SetNillableSize(nillableString(job.Size)).
		SetImageCount(job.ImageCount).
		SetNillableSeed(job.Seed).
		SetParameters(cloneStringAnyMap(job.Parameters)).
		SetNillableRequestID(nillableString(job.Trace.RequestID)).
		SetNillableUsageLogID(job.Trace.UsageLogID).
		SetNillableAPIKeyID(job.Trace.APIKeyID).
		SetNillableGroupID(job.Trace.GroupID).
		Save(ctx)
	if err != nil {
		return err
	}
	applyAIGenerationJobEntity(job, created)
	return nil
}

func (r *aiCenterRepository) UpdateGenerationJob(ctx context.Context, job *service.AIGenerationJob) error {
	if job == nil {
		return nil
	}
	updated, err := clientFromContext(ctx, r.client).AIGenerationJob.UpdateOneID(job.ID).
		SetStatus(job.Status).
		SetModel(job.Model).
		SetPrompt(job.Prompt).
		SetNillableNegativePrompt(nillableString(job.NegativePrompt)).
		SetNillableSize(nillableString(job.Size)).
		SetImageCount(job.ImageCount).
		SetNillableSeed(job.Seed).
		SetNillableErrorMessage(nillableString(job.ErrorMessage)).
		SetParameters(cloneStringAnyMap(job.Parameters)).
		SetNillableRequestID(nillableString(job.Trace.RequestID)).
		SetNillableUsageLogID(job.Trace.UsageLogID).
		SetNillableAPIKeyID(job.Trace.APIKeyID).
		SetNillableGroupID(job.Trace.GroupID).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrAIGenerationJobNotFound, nil)
	}
	applyAIGenerationJobEntity(job, updated)
	return nil
}

func (r *aiCenterRepository) GetGenerationJobByID(ctx context.Context, id int64) (*service.AIGenerationJob, error) {
	m, err := r.client.AIGenerationJob.Query().Where(aigenerationjob.IDEQ(id)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrAIGenerationJobNotFound, nil)
	}
	return aiGenerationJobEntityToService(m), nil
}

func (r *aiCenterRepository) GetGenerationJobByUserAndID(ctx context.Context, userID, id int64) (*service.AIGenerationJob, error) {
	m, err := r.client.AIGenerationJob.Query().Where(aigenerationjob.IDEQ(id), aigenerationjob.UserIDEQ(userID)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrAIGenerationJobNotFound, nil)
	}
	return aiGenerationJobEntityToService(m), nil
}

func (r *aiCenterRepository) ListGenerationJobs(ctx context.Context, userID int64, isAdmin bool, params pagination.PaginationParams, filter service.AIListGenerationJobsFilter) ([]service.AIGenerationJob, *pagination.PaginationResult, error) {
	q := r.client.AIGenerationJob.Query()
	if !isAdmin {
		q = q.Where(aigenerationjob.UserIDEQ(userID))
	}
	if s := strings.TrimSpace(filter.Status); s != "" {
		q = q.Where(aigenerationjob.StatusEQ(s))
	}
	if filter.SessionID != nil {
		q = q.Where(aigenerationjob.SessionIDEQ(*filter.SessionID))
	}
	if filter.PromptTemplateID != nil {
		q = q.Where(aigenerationjob.PromptTemplateIDEQ(*filter.PromptTemplateID))
	}
	if filter.GroupID != nil {
		q = q.Where(aigenerationjob.GroupIDEQ(*filter.GroupID))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	items, err := q.Order(dbent.Desc(aigenerationjob.FieldUpdatedAt), dbent.Desc(aigenerationjob.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make([]service.AIGenerationJob, 0, len(items))
	for i := range items {
		if s := aiGenerationJobEntityToService(items[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out, paginationResultFromTotal(int64(total), params), nil
}

func (r *aiCenterRepository) CreateAssets(ctx context.Context, assets []*service.AIAsset) error {
	if len(assets) == 0 {
		return nil
	}
	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}
	client := r.client
	txCtx := ctx
	if tx != nil {
		client = tx.Client()
		txCtx = dbent.NewTxContext(ctx, tx)
	}
	for i := range assets {
		asset := assets[i]
		created, createErr := client.AIAsset.Create().
			SetUserID(asset.UserID).
			SetNillableGenerationJobID(asset.GenerationJobID).
			SetNillableSessionID(asset.SessionID).
			SetNillablePromptTemplateID(asset.PromptTemplateID).
			SetAssetType(asset.AssetType).
			SetStatus(asset.Status).
			SetVisibility(asset.Visibility).
			SetModerationState(asset.ModerationState).
			SetNillableStorageKind(nillableString(asset.StorageKind)).
			SetNillableStoragePath(nillableString(asset.StoragePath)).
			SetNillableSourceURL(nillableString(asset.SourceURL)).
			SetNillableMimeType(nillableString(asset.MIMEType)).
			SetNillableWidth(asset.Width).
			SetNillableHeight(asset.Height).
			SetNillableByteSize(asset.ByteSize).
			SetNillableChecksum(nillableString(asset.Checksum)).
			SetMetadata(cloneStringAnyMap(asset.Metadata)).
			SetNillableRequestID(nillableString(asset.Trace.RequestID)).
			SetNillableUsageLogID(asset.Trace.UsageLogID).
			SetNillableAPIKeyID(asset.Trace.APIKeyID).
			SetNillableGroupID(asset.Trace.GroupID).
			Save(txCtx)
		if createErr != nil {
			if tx != nil {
				_ = tx.Rollback()
			}
			return createErr
		}
		applyAIAssetEntity(asset, created)
	}
	if tx != nil {
		return tx.Commit()
	}
	return nil
}

func (r *aiCenterRepository) UpdateAsset(ctx context.Context, asset *service.AIAsset) error {
	if asset == nil {
		return nil
	}
	updated, err := clientFromContext(ctx, r.client).AIAsset.UpdateOneID(asset.ID).
		SetStatus(asset.Status).
		SetVisibility(asset.Visibility).
		SetModerationState(asset.ModerationState).
		SetNillableStorageKind(nillableString(asset.StorageKind)).
		SetNillableStoragePath(nillableString(asset.StoragePath)).
		SetNillableSourceURL(nillableString(asset.SourceURL)).
		SetNillableMimeType(nillableString(asset.MIMEType)).
		SetNillableWidth(asset.Width).
		SetNillableHeight(asset.Height).
		SetNillableByteSize(asset.ByteSize).
		SetNillableChecksum(nillableString(asset.Checksum)).
		SetMetadata(cloneStringAnyMap(asset.Metadata)).
		SetNillableRequestID(nillableString(asset.Trace.RequestID)).
		SetNillableUsageLogID(asset.Trace.UsageLogID).
		SetNillableAPIKeyID(asset.Trace.APIKeyID).
		SetNillableGroupID(asset.Trace.GroupID).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrAIAssetNotFound, nil)
	}
	applyAIAssetEntity(asset, updated)
	return nil
}

func (r *aiCenterRepository) GetAssetByID(ctx context.Context, id int64) (*service.AIAsset, error) {
	m, err := r.client.AIAsset.Query().Where(aiasset.IDEQ(id)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrAIAssetNotFound, nil)
	}
	return aiAssetEntityToService(m), nil
}

func (r *aiCenterRepository) GetAssetByUserAndID(ctx context.Context, userID, id int64) (*service.AIAsset, error) {
	m, err := r.client.AIAsset.Query().Where(aiasset.IDEQ(id), aiasset.UserIDEQ(userID)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrAIAssetNotFound, nil)
	}
	return aiAssetEntityToService(m), nil
}

func (r *aiCenterRepository) ListAssets(ctx context.Context, userID int64, isAdmin bool, params pagination.PaginationParams, filter service.AIListAssetsFilter) ([]service.AIAsset, *pagination.PaginationResult, error) {
	q := r.client.AIAsset.Query()
	if !isAdmin {
		q = q.Where(aiasset.UserIDEQ(userID))
	}
	if s := strings.TrimSpace(filter.Status); s != "" {
		q = q.Where(aiasset.StatusEQ(s))
	}
	if s := strings.TrimSpace(filter.Visibility); s != "" {
		q = q.Where(aiasset.VisibilityEQ(s))
	}
	if s := strings.TrimSpace(filter.ModerationState); s != "" {
		q = q.Where(aiasset.ModerationStateEQ(s))
	}
	if filter.GenerationJobID != nil {
		q = q.Where(aiasset.GenerationJobIDEQ(*filter.GenerationJobID))
	}
	if filter.SessionID != nil {
		q = q.Where(aiasset.SessionIDEQ(*filter.SessionID))
	}
	if filter.PromptTemplateID != nil {
		q = q.Where(aiasset.PromptTemplateIDEQ(*filter.PromptTemplateID))
	}
	if filter.GroupID != nil {
		q = q.Where(aiasset.GroupIDEQ(*filter.GroupID))
	}
	items, err := q.Order(dbent.Desc(aiasset.FieldUpdatedAt), dbent.Desc(aiasset.FieldID)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	filtered := make([]service.AIAsset, 0, len(items))
	for i := range items {
		if s := aiAssetEntityToService(items[i]); s != nil {
			if !matchesAssetFilter(s, filter) {
				continue
			}
			filtered = append(filtered, *s)
		}
	}
	return paginateAssets(filtered, params), paginationResultFromTotal(int64(len(filtered)), params), nil
}

func (r *aiCenterRepository) CreateAuditLog(ctx context.Context, audit *service.AIAuditLog) error {
	if audit == nil {
		return nil
	}
	_, err := clientFromContext(ctx, r.client).AIAuditLog.Create().
		SetNillableOperatorUserID(audit.OperatorUserID).
		SetNillableOwnerUserID(audit.OwnerUserID).
		SetEntityType(audit.EntityType).
		SetNillableEntityID(audit.EntityID).
		SetAction(audit.Action).
		SetNillableReason(nillableString(audit.Reason)).
		SetBeforeState(cloneStringAnyMap(audit.BeforeState)).
		SetAfterState(cloneStringAnyMap(audit.AfterState)).
		SetNillableRequestID(nillableString(audit.Trace.RequestID)).
		SetNillableUsageLogID(audit.Trace.UsageLogID).
		SetNillableAPIKeyID(audit.Trace.APIKeyID).
		SetNillableGroupID(audit.Trace.GroupID).
		Save(ctx)
	return err
}

func (r *aiCenterRepository) ListAuditLogs(ctx context.Context, params pagination.PaginationParams, filter service.AIListAuditLogsFilter) ([]service.AIAuditLog, *pagination.PaginationResult, error) {
	q := r.client.AIAuditLog.Query()
	if s := strings.TrimSpace(filter.EntityType); s != "" {
		q = q.Where(aiauditlog.EntityTypeEQ(s))
	}
	if filter.EntityID != nil {
		q = q.Where(aiauditlog.EntityIDEQ(*filter.EntityID))
	}
	if s := strings.TrimSpace(filter.Action); s != "" {
		q = q.Where(aiauditlog.ActionEQ(s))
	}
	if s := strings.TrimSpace(filter.RequestID); s != "" {
		q = q.Where(aiauditlog.RequestIDEQ(s))
	}
	if filter.OperatorUserID != nil {
		q = q.Where(aiauditlog.OperatorUserIDEQ(*filter.OperatorUserID))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	items, err := q.Order(dbent.Desc(aiauditlog.FieldCreatedAt), dbent.Desc(aiauditlog.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make([]service.AIAuditLog, 0, len(items))
	for i := range items {
		if s := aiAuditLogEntityToService(items[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out, paginationResultFromTotal(int64(total), params), nil
}

const (
	domainVisibilityPublic = "public"
	domainModerationNormal = "normal"
)

func normalizeAIPromptScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "library":
		return "library"
	case "all":
		return "all"
	default:
		return "mine"
	}
}

func matchesAssetFilter(item *service.AIAsset, filter service.AIListAssetsFilter) bool {
	if item == nil {
		return false
	}
	if filter.Featured != nil && metadataBool(item.Metadata, "featured") != *filter.Featured {
		return false
	}
	if search := strings.ToLower(strings.TrimSpace(filter.Search)); search != "" {
		haystack := strings.ToLower(strings.Join([]string{
			metadataString(item.Metadata, "title"),
			metadataString(item.Metadata, "prompt"),
			strings.Join(metadataStringSlice(item.Metadata, "tags"), " "),
		}, " "))
		if !strings.Contains(haystack, search) {
			return false
		}
	}
	return true
}

func nillableString(src string) *string {
	v := strings.TrimSpace(src)
	if v == "" {
		return nil
	}
	return &v
}

func trimNillableString(src *string) string {
	if src == nil {
		return ""
	}
	return strings.TrimSpace(*src)
}

func cloneStringSlice(src []string) []string {
	if len(src) == 0 {
		return []string{}
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func cloneStringAnySlice(src []map[string]any) []map[string]any {
	if len(src) == 0 {
		return []map[string]any{}
	}
	dst := make([]map[string]any, 0, len(src))
	for i := range src {
		dst = append(dst, cloneStringAnyMap(src[i]))
	}
	return dst
}

func cloneStringAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	if value, ok := metadata[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func metadataBool(metadata map[string]any, key string) bool {
	if metadata == nil {
		return false
	}
	if value, ok := metadata[key].(bool); ok {
		return value
	}
	return false
}

func metadataStringSlice(metadata map[string]any, key string) []string {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata[key]
	if !ok {
		return nil
	}
	switch value := raw.(type) {
	case []string:
		return append([]string(nil), value...)
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	default:
		return nil
	}
}

func aiMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func paginateAssets(items []service.AIAsset, params pagination.PaginationParams) []service.AIAsset {
	limit := params.Limit()
	page := aiMax(1, params.Page)
	start := (page - 1) * limit
	if start >= len(items) {
		return []service.AIAsset{}
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func aiSessionEntityToService(m *dbent.AISession) *service.AISession {
	if m == nil {
		return nil
	}
	out := &service.AISession{
		ID:            m.ID,
		UserID:        m.UserID,
		Title:         m.Title,
		Status:        m.Status,
		Metadata:      cloneStringAnyMap(m.Metadata),
		LastMessageAt: m.LastMessageAt,
		Trace: service.AITraceRef{
			RequestID:  trimNillableString(m.RequestID),
			UsageLogID: m.UsageLogID,
			APIKeyID:   m.APIKeyID,
			GroupID:    m.GroupID,
		},
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
	if m.SystemPrompt != nil {
		out.SystemPrompt = *m.SystemPrompt
	}
	return out
}

func applyAISessionEntity(dst *service.AISession, src *dbent.AISession) {
	if dst == nil || src == nil {
		return
	}
	*dst = *aiSessionEntityToService(src)
}

func aiSessionMessageEntityToService(m *dbent.AISessionMessage) *service.AISessionMessage {
	if m == nil {
		return nil
	}
	out := &service.AISessionMessage{
		ID:           m.ID,
		SessionID:    m.SessionID,
		UserID:       m.UserID,
		Role:         m.Role,
		Status:       m.Status,
		Content:      m.Content,
		ContentParts: cloneStringAnySlice(m.ContentParts),
		Metadata:     cloneStringAnyMap(m.Metadata),
		Trace:        service.AITraceRef{RequestID: trimNillableString(m.RequestID), UsageLogID: m.UsageLogID, APIKeyID: m.APIKeyID, GroupID: m.GroupID},
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
	if m.ReplyToMessageID != nil {
		out.ReplyToMessageID = m.ReplyToMessageID
	}
	if m.Model != nil {
		out.Model = *m.Model
	}
	if m.Provider != nil {
		out.Provider = *m.Provider
	}
	if m.ErrorMessage != nil {
		out.ErrorMessage = *m.ErrorMessage
	}
	return out
}

func applyAISessionMessageEntity(dst *service.AISessionMessage, src *dbent.AISessionMessage) {
	if dst == nil || src == nil {
		return
	}
	*dst = *aiSessionMessageEntityToService(src)
}

func aiPromptTemplateEntityToService(m *dbent.AIPromptTemplate) *service.AIPromptTemplate {
	if m == nil {
		return nil
	}
	out := &service.AIPromptTemplate{
		ID:              m.ID,
		UserID:          m.UserID,
		Title:           m.Title,
		Description:     "",
		Category:        "",
		Tags:            cloneStringSlice(m.Tags),
		Visibility:      m.Visibility,
		ModerationState: m.ModerationState,
		CurrentVersion:  m.CurrentVersion,
		ModelHint:       "",
		Content:         m.Content,
		Variables:       []map[string]any{},
		Metadata:        cloneStringAnyMap(m.Metadata),
		CoverAssetID:    m.CoverAssetID,
		Trace:           service.AITraceRef{RequestID: trimNillableString(m.RequestID), UsageLogID: m.UsageLogID, APIKeyID: m.APIKeyID, GroupID: m.GroupID},
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
	if m.Description != nil {
		out.Description = *m.Description
	}
	if m.Category != nil {
		out.Category = *m.Category
	}
	if m.ModelHint != nil {
		out.ModelHint = *m.ModelHint
	}
	return out
}

func applyAIPromptTemplateEntity(dst *service.AIPromptTemplate, src *dbent.AIPromptTemplate) {
	if dst == nil || src == nil {
		return
	}
	*dst = *aiPromptTemplateEntityToService(src)
}

func aiPromptTemplateVersionEntityToService(m *dbent.AIPromptTemplateVersion) *service.AIPromptTemplateVersion {
	if m == nil {
		return nil
	}
	out := &service.AIPromptTemplateVersion{
		ID:         m.ID,
		TemplateID: m.TemplateID,
		UserID:     m.UserID,
		Version:    m.Version,
		Title:      m.Title,
		Content:    m.Content,
		Variables:  cloneStringAnySlice(m.Variables),
		Metadata:   cloneStringAnyMap(m.Metadata),
		Trace:      service.AITraceRef{RequestID: trimNillableString(m.RequestID), UsageLogID: m.UsageLogID, APIKeyID: m.APIKeyID, GroupID: m.GroupID},
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
	if m.ModelHint != nil {
		out.ModelHint = *m.ModelHint
	}
	if m.ChangeNote != nil {
		out.ChangeNote = *m.ChangeNote
	}
	return out
}

func applyAIPromptTemplateVersionEntity(dst *service.AIPromptTemplateVersion, src *dbent.AIPromptTemplateVersion) {
	if dst == nil || src == nil {
		return
	}
	*dst = *aiPromptTemplateVersionEntityToService(src)
}

func aiGenerationJobEntityToService(m *dbent.AIGenerationJob) *service.AIGenerationJob {
	if m == nil {
		return nil
	}
	out := &service.AIGenerationJob{
		ID:               m.ID,
		UserID:           m.UserID,
		SessionID:        m.SessionID,
		PromptTemplateID: m.PromptTemplateID,
		Status:           m.Status,
		Model:            m.Model,
		Prompt:           m.Prompt,
		NegativePrompt:   "",
		Size:             "",
		ImageCount:       m.ImageCount,
		Seed:             m.Seed,
		ErrorMessage:     "",
		Parameters:       cloneStringAnyMap(m.Parameters),
		Trace:            service.AITraceRef{RequestID: trimNillableString(m.RequestID), UsageLogID: m.UsageLogID, APIKeyID: m.APIKeyID, GroupID: m.GroupID},
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
	if m.NegativePrompt != nil {
		out.NegativePrompt = *m.NegativePrompt
	}
	if m.Size != nil {
		out.Size = *m.Size
	}
	if m.ErrorMessage != nil {
		out.ErrorMessage = *m.ErrorMessage
	}
	return out
}

func applyAIGenerationJobEntity(dst *service.AIGenerationJob, src *dbent.AIGenerationJob) {
	if dst == nil || src == nil {
		return
	}
	*dst = *aiGenerationJobEntityToService(src)
}

func aiAssetEntityToService(m *dbent.AIAsset) *service.AIAsset {
	if m == nil {
		return nil
	}
	out := &service.AIAsset{
		ID:               m.ID,
		UserID:           m.UserID,
		GenerationJobID:  m.GenerationJobID,
		SessionID:        m.SessionID,
		PromptTemplateID: m.PromptTemplateID,
		AssetType:        m.AssetType,
		Status:           m.Status,
		Visibility:       m.Visibility,
		ModerationState:  m.ModerationState,
		Metadata:         cloneStringAnyMap(m.Metadata),
		Trace:            service.AITraceRef{RequestID: trimNillableString(m.RequestID), UsageLogID: m.UsageLogID, APIKeyID: m.APIKeyID, GroupID: m.GroupID},
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
	if m.StorageKind != nil {
		out.StorageKind = *m.StorageKind
	}
	if m.StoragePath != nil {
		out.StoragePath = *m.StoragePath
	}
	if m.SourceURL != nil {
		out.SourceURL = *m.SourceURL
	}
	if m.MimeType != nil {
		out.MIMEType = *m.MimeType
	}
	if m.Width != nil {
		out.Width = m.Width
	}
	if m.Height != nil {
		out.Height = m.Height
	}
	if m.ByteSize != nil {
		out.ByteSize = m.ByteSize
	}
	if m.Checksum != nil {
		out.Checksum = *m.Checksum
	}
	return out
}

func applyAIAssetEntity(dst *service.AIAsset, src *dbent.AIAsset) {
	if dst == nil || src == nil {
		return
	}
	*dst = *aiAssetEntityToService(src)
}

func aiAuditLogEntityToService(m *dbent.AIAuditLog) *service.AIAuditLog {
	if m == nil {
		return nil
	}
	out := &service.AIAuditLog{
		ID:             m.ID,
		OperatorUserID: m.OperatorUserID,
		OwnerUserID:    m.OwnerUserID,
		EntityType:     m.EntityType,
		EntityID:       m.EntityID,
		Action:         m.Action,
		BeforeState:    cloneStringAnyMap(m.BeforeState),
		AfterState:     cloneStringAnyMap(m.AfterState),
		Trace:          service.AITraceRef{RequestID: trimNillableString(m.RequestID), UsageLogID: m.UsageLogID, APIKeyID: m.APIKeyID, GroupID: m.GroupID},
		CreatedAt:      m.CreatedAt,
	}
	if m.Reason != nil {
		out.Reason = *m.Reason
	}
	return out
}
