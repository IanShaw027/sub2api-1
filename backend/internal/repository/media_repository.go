package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type mediaRepository struct {
	db *sql.DB
}

func NewMediaRepository(db *sql.DB) service.MediaRepository {
	return &mediaRepository{db: db}
}

func (r *mediaRepository) Create(ctx context.Context, asset *service.MediaAsset) error {
	if asset == nil {
		return fmt.Errorf("nil media asset")
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO media_assets (
			biz_type, biz_id, storage_profile_id, bucket, object_key, thumbnail_object_key, visibility,
			thumbnail_mime_type, mime_type, size_bytes, width, height, sha256, owner_user_id, status,
			original_file_name
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING id, created_at, updated_at
	`,
		asset.BizType,
		asset.BizID,
		asset.StorageProfileID,
		asset.Bucket,
		asset.ObjectKey,
		asset.ThumbnailObjectKey,
		asset.Visibility,
		asset.ThumbnailMIMEType,
		asset.MIMEType,
		asset.SizeBytes,
		asset.Width,
		asset.Height,
		asset.SHA256,
		asset.OwnerUserID,
		asset.Status,
		asset.OriginalFileName,
	)
	if err := row.Scan(&asset.ID, &asset.CreatedAt, &asset.UpdatedAt); err != nil {
		return err
	}
	return nil
}

func (r *mediaRepository) GetByID(ctx context.Context, id int64) (*service.MediaAsset, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, biz_type, biz_id, storage_profile_id, bucket, object_key, thumbnail_object_key, thumbnail_mime_type, visibility,
		       mime_type, size_bytes, width, height, sha256, owner_user_id, status,
		       original_file_name, created_at, updated_at, deleted_at
		FROM media_assets
		WHERE id = $1
	`, id)
	item, err := scanMediaAsset(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrMediaNotFound
		}
		return nil, err
	}
	return item, nil
}

func (r *mediaRepository) GetByObjectKey(ctx context.Context, bucket, objectKey string) (*service.MediaAsset, error) {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" {
		return nil, service.ErrMediaNotFound
	}
	bucket = strings.TrimSpace(bucket)
	query := `
		SELECT id, biz_type, biz_id, storage_profile_id, bucket, object_key, thumbnail_object_key, thumbnail_mime_type, visibility,
		       mime_type, size_bytes, width, height, sha256, owner_user_id, status,
		       original_file_name, created_at, updated_at, deleted_at
		FROM media_assets
		WHERE (object_key = $1 OR thumbnail_object_key = $1) AND status = $2`
	args := []any{objectKey, service.MediaStatusActive}
	if bucket != "" {
		query += " AND bucket = $3"
		args = append(args, bucket)
	}
	query += " ORDER BY CASE WHEN object_key = $1 THEN 0 ELSE 1 END, id DESC LIMIT 1"
	row := r.db.QueryRowContext(ctx, query, args...)
	item, err := scanMediaAsset(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrMediaNotFound
		}
		return nil, err
	}
	return item, nil
}

func (r *mediaRepository) List(ctx context.Context, params pagination.PaginationParams, filters service.MediaListFilters) ([]service.MediaAsset, *pagination.PaginationResult, error) {
	where := make([]string, 0, 8)
	args := make([]any, 0, 8)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}

	if filters.BizType != "" {
		where = append(where, "biz_type = "+addArg(filters.BizType))
	}
	if filters.BizID != "" {
		where = append(where, "biz_id = "+addArg(filters.BizID))
	}
	if filters.Visibility != "" {
		where = append(where, "visibility = "+addArg(filters.Visibility))
	}
	if filters.Status != "" {
		where = append(where, "status = "+addArg(filters.Status))
	}
	if filters.OwnerUserID != nil {
		where = append(where, "owner_user_id = "+addArg(*filters.OwnerUserID))
	}
	if filters.Search != "" {
		needle := "%" + strings.TrimSpace(filters.Search) + "%"
		p1 := addArg(needle)
		p2 := addArg(needle)
		p3 := addArg(needle)
		p4 := addArg(needle)
		where = append(where, fmt.Sprintf("(object_key ILIKE %s OR sha256 ILIKE %s OR original_file_name ILIKE %s OR mime_type ILIKE %s)", p1, p2, p3, p4))
	}
	whereSQL := "1=1"
	if len(where) > 0 {
		whereSQL = strings.Join(where, " AND ")
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM media_assets WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, nil, err
	}

	orderBy := mediaListOrder(params)
	listArgs := append([]any(nil), args...)
	listArgs = append(listArgs, params.Limit(), params.Offset())
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, biz_type, biz_id, storage_profile_id, bucket, object_key, thumbnail_object_key, thumbnail_mime_type, visibility,
		       mime_type, size_bytes, width, height, sha256, owner_user_id, status,
		       original_file_name, created_at, updated_at, deleted_at
		FROM media_assets
		WHERE `+whereSQL+`
		ORDER BY `+orderBy+`
		LIMIT $`+fmt.Sprint(len(listArgs)-1)+` OFFSET $`+fmt.Sprint(len(listArgs)), listArgs...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.MediaAsset, 0)
	for rows.Next() {
		item, scanErr := scanMediaAsset(rows)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	pages := int(math.Ceil(float64(total) / float64(params.Limit())))
	if pages < 1 {
		pages = 1
	}
	return items, &pagination.PaginationResult{
		Total:    total,
		Page:     max(1, params.Page),
		PageSize: params.Limit(),
		Pages:    pages,
	}, nil
}

func (r *mediaRepository) UpdateVisibility(ctx context.Context, id int64, visibility string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE media_assets
		SET visibility = $2, updated_at = NOW()
		WHERE id = $1
	`, id, visibility)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrMediaNotFound
	}
	return nil
}

func (r *mediaRepository) MarkDeleted(ctx context.Context, id int64, deletedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE media_assets
		SET status = $2, deleted_at = $3, updated_at = NOW()
		WHERE id = $1
	`, id, service.MediaStatusDeleted, deletedAt)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return service.ErrMediaNotFound
	}
	return nil
}

type mediaScanner interface {
	Scan(dest ...any) error
}

func scanMediaAsset(scanner mediaScanner) (*service.MediaAsset, error) {
	var (
		item        service.MediaAsset
		width       sql.NullInt64
		height      sql.NullInt64
		ownerUserID sql.NullInt64
		deletedAt   sql.NullTime
	)
	if err := scanner.Scan(
		&item.ID,
		&item.BizType,
		&item.BizID,
		&item.StorageProfileID,
		&item.Bucket,
		&item.ObjectKey,
		&item.ThumbnailObjectKey,
		&item.ThumbnailMIMEType,
		&item.Visibility,
		&item.MIMEType,
		&item.SizeBytes,
		&width,
		&height,
		&item.SHA256,
		&ownerUserID,
		&item.Status,
		&item.OriginalFileName,
		&item.CreatedAt,
		&item.UpdatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	if width.Valid {
		v := int(width.Int64)
		item.Width = &v
	}
	if height.Valid {
		v := int(height.Int64)
		item.Height = &v
	}
	if ownerUserID.Valid {
		v := ownerUserID.Int64
		item.OwnerUserID = &v
	}
	if deletedAt.Valid {
		item.DeletedAt = &deletedAt.Time
	}
	return &item, nil
}

func mediaListOrder(params pagination.PaginationParams) string {
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	switch sortBy {
	case "updated_at", "size_bytes", "biz_type", "owner_user_id":
	default:
		sortBy = "created_at"
	}
	order := params.NormalizedSortOrder("desc")
	return sortBy + " " + order + ", id " + order
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
