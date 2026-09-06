package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/creationimagejob"
	"github.com/Wei-Shaw/sub2api/ent/creationmessage"
	"github.com/Wei-Shaw/sub2api/ent/creationsession"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type creationSessionRepository struct {
	client *dbent.Client
}

func NewCreationSessionRepository(client *dbent.Client) service.CreationSessionRepository {
	return &creationSessionRepository{client: client}
}

func (r *creationSessionRepository) Create(ctx context.Context, input *service.CreationSession) error {
	builder := r.client.CreationSession.Create().
		SetUserID(input.UserID).
		SetGroupID(input.GroupID).
		SetTitle(input.Title).
		SetModel(input.Model).
		SetMode(input.Mode).
		SetStatus(input.Status)
	if len(input.Metadata) > 0 {
		builder.SetMetadata(input.Metadata)
	}
	row, err := builder.Save(ctx)
	if err != nil {
		return err
	}
	input.ID = row.ID
	input.CreatedAt = row.CreatedAt
	input.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *creationSessionRepository) GetByID(ctx context.Context, id int64) (*service.CreationSession, error) {
	row, err := r.client.CreationSession.Get(ctx, id)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrCreationSessionNotFound
		}
		return nil, err
	}
	return creationSessionEntityToService(row), nil
}

func (r *creationSessionRepository) GetForUser(ctx context.Context, userID, id int64) (*service.CreationSession, error) {
	row, err := r.client.CreationSession.Query().
		Where(
			creationsession.IDEQ(id),
			creationsession.UserIDEQ(userID),
		).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrCreationSessionNotFound
		}
		return nil, err
	}
	return creationSessionEntityToService(row), nil
}

func (r *creationSessionRepository) ListForUser(ctx context.Context, userID int64, filters service.CreationSessionListFilters) ([]service.CreationSession, *pagination.PaginationResult, error) {
	query := r.client.CreationSession.Query().
		Where(creationsession.UserIDEQ(userID))
	if mode := strings.TrimSpace(filters.Mode); mode != "" {
		query = query.Where(creationsession.ModeEQ(mode))
	}
	if status := strings.TrimSpace(filters.Status); status != "" {
		query = query.Where(creationsession.StatusEQ(status))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	params := pagination.PaginationParams{Page: filters.Page, PageSize: filters.PageSize}
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	rows, err := query.
		Order(dbent.Desc(creationsession.FieldUpdatedAt)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make([]service.CreationSession, 0, len(rows))
	for _, row := range rows {
		out = append(out, *creationSessionEntityToService(row))
	}
	return out, paginationResultFromTotal(int64(total), params), nil
}

func (r *creationSessionRepository) Update(ctx context.Context, id int64, input service.UpdateCreationSessionInput) (*service.CreationSession, error) {
	builder := r.client.CreationSession.UpdateOneID(id)
	if input.Title != nil {
		builder.SetTitle(strings.TrimSpace(*input.Title))
	}
	if input.Model != nil {
		builder.SetModel(strings.TrimSpace(*input.Model))
	}
	if input.Status != nil {
		builder.SetStatus(*input.Status)
	}
	if input.HasMeta {
		if len(input.Metadata) == 0 {
			builder.SetMetadata(json.RawMessage(`{}`))
		} else {
			builder.SetMetadata(input.Metadata)
		}
	}
	row, err := builder.Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrCreationSessionNotFound
		}
		return nil, err
	}
	return creationSessionEntityToService(row), nil
}

func (r *creationSessionRepository) Delete(ctx context.Context, userID, id int64) error {
	n, err := r.client.CreationSession.Delete().
		Where(
			creationsession.IDEQ(id),
			creationsession.UserIDEQ(userID),
		).
		Exec(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrCreationSessionNotFound
	}
	return nil
}

func creationSessionEntityToService(row *dbent.CreationSession) *service.CreationSession {
	if row == nil {
		return nil
	}
	out := &service.CreationSession{
		ID:        row.ID,
		UserID:    row.UserID,
		GroupID:   row.GroupID,
		Title:     row.Title,
		Model:     row.Model,
		Mode:      row.Mode,
		Status:    row.Status,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	if len(row.Metadata) > 0 {
		out.Metadata = append(json.RawMessage(nil), row.Metadata...)
	}
	return out
}

type creationMessageRepository struct {
	client *dbent.Client
}

func NewCreationMessageRepository(client *dbent.Client) service.CreationMessageRepository {
	return &creationMessageRepository{client: client}
}

func (r *creationMessageRepository) Create(ctx context.Context, msg *service.CreationMessage) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin creation message transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	builder := tx.CreationMessage.Create().
		SetSessionID(msg.SessionID).
		SetRole(msg.Role).
		SetContent(msg.Content)
	if msg.Model != nil {
		builder.SetModel(*msg.Model)
	}
	if msg.InputTokens != nil {
		builder.SetInputTokens(*msg.InputTokens)
	}
	if msg.OutputTokens != nil {
		builder.SetOutputTokens(*msg.OutputTokens)
	}
	row, err := builder.Save(ctx)
	if err != nil {
		return fmt.Errorf("save creation message: %w", err)
	}
	if _, err := tx.CreationSession.UpdateOneID(msg.SessionID).
		SetUpdatedAt(time.Now()).
		Save(ctx); err != nil {
		return fmt.Errorf("touch creation session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit creation message: %w", err)
	}
	msg.ID = row.ID
	msg.CreatedAt = row.CreatedAt
	return nil
}

func (r *creationMessageRepository) CreateExchange(ctx context.Context, input service.CreateCreationExchangeInput) (*service.CreationExchange, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin creation exchange transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Updating the parent takes a write lock before the idempotency lookup,
	// serializing retries and concurrent deletion on PostgreSQL and SQLite.
	n, err := tx.CreationSession.Update().Where(creationsession.IDEQ(input.SessionID), creationsession.UserIDEQ(input.UserID)).SetUpdatedAt(time.Now()).Save(ctx)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, service.ErrCreationSessionNotFound
	}
	rows, err := tx.CreationMessage.Query().Where(creationmessage.SessionIDEQ(input.SessionID), creationmessage.ExchangeRequestIDEQ(input.RequestID)).All(ctx)
	if err != nil {
		return nil, err
	}
	userContent, _ := json.Marshal(input.UserContent)
	assistantContent, _ := json.Marshal(input.AssistantContent)
	if len(rows) != 0 {
		exchange := &service.CreationExchange{}
		for _, row := range rows {
			switch row.Role {
			case service.CreationMessageRoleUser:
				exchange.User = *creationMessageEntityToService(row)
			case service.CreationMessageRoleAssistant:
				exchange.Assistant = *creationMessageEntityToService(row)
			}
		}
		if len(rows) != 2 || !creationContentMatches(exchange.User.Content, input.UserContent) || !creationContentMatches(exchange.Assistant.Content, input.AssistantContent) ||
			exchange.Assistant.Model == nil || *exchange.Assistant.Model != input.Model ||
			!sameCreationTokenCount(exchange.Assistant.InputTokens, input.InputTokens) || !sameCreationTokenCount(exchange.Assistant.OutputTokens, input.OutputTokens) {
			return nil, service.ErrCreationExchangeConflict
		}
		// A replay must not change the session's last-message timestamp.
		if err := tx.Rollback(); err != nil {
			return nil, err
		}
		return exchange, nil
	}
	user, err := tx.CreationMessage.Create().SetSessionID(input.SessionID).SetExchangeRequestID(input.RequestID).
		SetRole(service.CreationMessageRoleUser).SetContent(userContent).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("save creation exchange user: %w", err)
	}
	assistant, err := tx.CreationMessage.Create().SetSessionID(input.SessionID).SetExchangeRequestID(input.RequestID).
		SetRole(service.CreationMessageRoleAssistant).SetContent(assistantContent).SetModel(input.Model).
		SetNillableInputTokens(input.InputTokens).SetNillableOutputTokens(input.OutputTokens).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("save creation exchange assistant: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit creation exchange: %w", err)
	}
	return &service.CreationExchange{User: *creationMessageEntityToService(user), Assistant: *creationMessageEntityToService(assistant)}, nil
}

func sameCreationTokenCount(a, b *int) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func creationContentMatches(raw json.RawMessage, content string) bool {
	var decoded string
	return json.Unmarshal(raw, &decoded) == nil && decoded == content
}

func creationMessageEntityToService(row *dbent.CreationMessage) *service.CreationMessage {
	return &service.CreationMessage{
		ID: row.ID, SessionID: row.SessionID, Role: row.Role,
		ExchangeRequestID: row.ExchangeRequestID,
		Content:           append(json.RawMessage(nil), row.Content...), Model: row.Model,
		InputTokens: row.InputTokens, OutputTokens: row.OutputTokens, CreatedAt: row.CreatedAt,
	}
}

func (r *creationMessageRepository) ListBySession(ctx context.Context, sessionID int64) ([]service.CreationMessage, error) {
	rows, err := r.client.CreationMessage.Query().
		Where(creationmessage.SessionIDEQ(sessionID)).
		Order(dbent.Asc(creationmessage.FieldCreatedAt), dbent.Asc(creationmessage.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.CreationMessage, 0, len(rows))
	for _, row := range rows {
		out = append(out, *creationMessageEntityToService(row))
	}
	return out, nil
}

type creationImageJobRepository struct {
	client *dbent.Client
}

func NewCreationImageJobRepository(client *dbent.Client) service.CreationImageJobRepository {
	return &creationImageJobRepository{client: client}
}

func (r *creationImageJobRepository) Create(ctx context.Context, job *service.CreationImageJob) error {
	builder := r.client.CreationImageJob.Create().
		SetUserID(job.UserID).
		SetGroupID(job.GroupID).
		SetStatus(job.Status).
		SetModel(job.Model).
		SetPrompt(job.Prompt).
		SetMediaURL(job.MediaURL).
		SetStorageID(job.StorageID).
		SetStorageKey(job.StorageKey)
	if job.SessionID != nil {
		builder.SetSessionID(*job.SessionID)
	}
	if job.MediaAssetID != nil {
		builder.SetMediaAssetID(*job.MediaAssetID)
	}
	if job.ProviderTaskID != nil {
		builder.SetProviderTaskID(*job.ProviderTaskID)
	}
	if job.Error != nil {
		builder.SetError(*job.Error)
	}
	row, err := builder.Save(ctx)
	if err != nil {
		return err
	}
	job.ID = row.ID
	job.CreatedAt = row.CreatedAt
	job.UpdatedAt = row.UpdatedAt
	return nil
}

func (r *creationImageJobRepository) GetByID(ctx context.Context, id int64) (*service.CreationImageJob, error) {
	row, err := r.client.CreationImageJob.Get(ctx, id)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrCreationImageNotFound
		}
		return nil, err
	}
	return creationImageJobEntityToService(row), nil
}

func (r *creationImageJobRepository) GetForUser(ctx context.Context, userID, id int64) (*service.CreationImageJob, error) {
	row, err := r.client.CreationImageJob.Query().
		Where(
			creationimagejob.IDEQ(id),
			creationimagejob.UserIDEQ(userID),
		).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrCreationImageNotFound
		}
		return nil, err
	}
	return creationImageJobEntityToService(row), nil
}

func (r *creationImageJobRepository) GetByProviderTaskID(ctx context.Context, userID int64, providerTaskID string) (*service.CreationImageJob, error) {
	providerTaskID = strings.TrimSpace(providerTaskID)
	if providerTaskID == "" {
		return nil, service.ErrCreationImageNotFound
	}
	row, err := r.client.CreationImageJob.Query().
		Where(
			creationimagejob.UserIDEQ(userID),
			creationimagejob.ProviderTaskIDEQ(providerTaskID),
		).
		Order(dbent.Desc(creationimagejob.FieldCreatedAt)).
		First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrCreationImageNotFound
		}
		return nil, err
	}
	return creationImageJobEntityToService(row), nil
}

func (r *creationImageJobRepository) ListForUser(ctx context.Context, userID int64, filters service.CreationImageListFilters) ([]service.CreationImageJob, *pagination.PaginationResult, error) {
	query := r.client.CreationImageJob.Query().
		Where(creationimagejob.UserIDEQ(userID))
	if filters.SessionID != nil {
		query = query.Where(creationimagejob.SessionIDEQ(*filters.SessionID))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	params := pagination.PaginationParams{Page: filters.Page, PageSize: filters.PageSize}
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	rows, err := query.
		Order(dbent.Desc(creationimagejob.FieldCreatedAt)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make([]service.CreationImageJob, 0, len(rows))
	for _, row := range rows {
		out = append(out, *creationImageJobEntityToService(row))
	}
	return out, paginationResultFromTotal(int64(total), params), nil
}

func (r *creationImageJobRepository) Update(ctx context.Context, id int64, job *service.CreationImageJob) error {
	builder := r.client.CreationImageJob.Update().Where(creationimagejob.IDEQ(id)).
		SetStatus(job.Status).
		SetMediaURL(job.MediaURL).
		SetStorageID(job.StorageID).
		SetStorageKey(job.StorageKey).
		SetUpdatedAt(time.Now())
	if job.Status == service.CreationImageJobStatusPending || job.Status == service.CreationImageJobStatusProcessing {
		builder.Where(creationimagejob.StatusIn(service.CreationImageJobStatusPending, service.CreationImageJobStatusProcessing))
	} else {
		builder.Where(creationimagejob.StatusIn(service.CreationImageJobStatusPending, service.CreationImageJobStatusProcessing, job.Status))
	}
	if job.MediaAssetID != nil {
		builder.SetMediaAssetID(*job.MediaAssetID)
	}
	if job.ProviderTaskID != nil {
		builder.SetProviderTaskID(*job.ProviderTaskID)
	}
	if job.Error != nil {
		builder.SetError(*job.Error)
	}
	_, err := builder.Save(ctx)
	return err
}

func creationImageJobEntityToService(row *dbent.CreationImageJob) *service.CreationImageJob {
	if row == nil {
		return nil
	}
	out := &service.CreationImageJob{
		MediaURL:   row.MediaURL,
		StorageID:  row.StorageID,
		StorageKey: row.StorageKey,
		ID:         row.ID,
		UserID:     row.UserID,
		GroupID:    row.GroupID,
		Status:     row.Status,
		Model:      row.Model,
		Prompt:     row.Prompt,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
	if row.SessionID != nil {
		v := *row.SessionID
		out.SessionID = &v
	}
	if row.MediaAssetID != nil {
		v := *row.MediaAssetID
		out.MediaAssetID = &v
	}
	if row.ProviderTaskID != nil {
		v := *row.ProviderTaskID
		out.ProviderTaskID = &v
	}
	if row.Error != nil {
		v := *row.Error
		out.Error = &v
	}
	return out
}
