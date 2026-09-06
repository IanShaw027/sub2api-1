package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type creationPublicationRepository struct{ db *sql.DB }

func NewCreationPublicationRepository(db *sql.DB) service.CreationPublicationRepository {
	return &creationPublicationRepository{db: db}
}

const publicationColumns = `id, owner_user_id, request_id, title, prompt, model, kind, mime, size,
    storage_key, storage_profile_id, sha256, created_at, withdrawn_at, status`

func scanPublication(row interface{ Scan(...any) error }) (*service.CreationPublication, error) {
	p := &service.CreationPublication{}
	err := row.Scan(&p.ID, &p.OwnerUserID, &p.RequestID, &p.Title, &p.Prompt, &p.Model, &p.Kind,
		&p.MIME, &p.Size, &p.StorageKey, &p.StorageProfileID, &p.SHA256, &p.CreatedAt, &p.WithdrawnAt, &p.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCreationPublicationNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *creationPublicationRepository) GetByRequestID(ctx context.Context, ownerUserID int64, requestID string) (*service.CreationPublication, error) {
	return scanPublication(r.db.QueryRowContext(ctx, `SELECT `+publicationColumns+`
        FROM creation_publications WHERE owner_user_id = $1 AND request_id = $2`, ownerUserID, requestID))
}

func (r *creationPublicationRepository) CreateIfAbsent(ctx context.Context, p *service.CreationPublication) (*service.CreationPublication, bool, error) {
	stored, err := scanPublication(r.db.QueryRowContext(ctx, `
        INSERT INTO creation_publications
        (owner_user_id, request_id, title, prompt, model, kind, mime, size, storage_key, storage_profile_id, sha256)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
        ON CONFLICT (owner_user_id, request_id) DO NOTHING
        RETURNING `+publicationColumns,
		p.OwnerUserID, p.RequestID, p.Title, p.Prompt, p.Model, p.Kind, p.MIME, p.Size, p.StorageKey, p.StorageProfileID, p.SHA256))
	if errors.Is(err, service.ErrCreationPublicationNotFound) {
		stored, err = r.GetByRequestID(ctx, p.OwnerUserID, p.RequestID)
		return stored, false, err
	}
	return stored, err == nil, err
}

func (r *creationPublicationRepository) GetPublic(ctx context.Context, id int64) (*service.CreationPublication, error) {
	return scanPublication(r.db.QueryRowContext(ctx, `SELECT `+publicationColumns+`
        FROM creation_publications WHERE id = $1 AND withdrawn_at IS NULL AND status = 'published'`, id))
}

func (r *creationPublicationRepository) List(ctx context.Context, filter service.CreationPublicationFilter) ([]*service.CreationPublication, int64, error) {
	const where = ` FROM creation_publications WHERE withdrawn_at IS NULL AND status = 'published'
        AND ($1 = '' OR kind = $1)
        AND ($2 = '' OR title ILIKE '%' || $2 || '%' OR prompt ILIKE '%' || $2 || '%' OR model ILIKE '%' || $2 || '%')`
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*)`+where, filter.Kind, filter.Search).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+publicationColumns+where+`
        ORDER BY created_at DESC, id DESC LIMIT $3 OFFSET $4`, filter.Kind, filter.Search, filter.PageSize, (filter.Page-1)*filter.PageSize)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]*service.CreationPublication, 0)
	for rows.Next() {
		p, err := scanPublication(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	return items, total, rows.Err()
}

func (r *creationPublicationRepository) Activate(ctx context.Context, id int64) (*service.CreationPublication, error) {
	return scanPublication(r.db.QueryRowContext(ctx, `UPDATE creation_publications SET status = 'published'
        WHERE id = $1 AND withdrawn_at IS NULL RETURNING `+publicationColumns, id))
}

func (r *creationPublicationRepository) Withdraw(ctx context.Context, ownerUserID, id int64) (*service.CreationPublication, error) {
	return scanPublication(r.db.QueryRowContext(ctx, `UPDATE creation_publications
        SET withdrawn_at = COALESCE(withdrawn_at, NOW())
        WHERE id = $1 AND owner_user_id = $2 RETURNING `+publicationColumns, id, ownerUserID))
}
