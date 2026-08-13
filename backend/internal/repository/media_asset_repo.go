package repository

import (
	"context"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/mediaasset"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type mediaAssetRepository struct {
	client *dbent.Client
}

func NewMediaAssetRepository(client *dbent.Client) service.MediaAssetRepository {
	return &mediaAssetRepository{client: client}
}

func (r *mediaAssetRepository) Create(ctx context.Context, asset *service.MediaAsset) error {
	client := clientFromContext(ctx, r.client)
	created, err := client.MediaAsset.Create().
		SetOwnerUserID(asset.OwnerUserID).
		SetBizType(asset.BizType).
		SetBizID(asset.BizID).
		SetStorageKey(asset.StorageKey).
		SetSha256(asset.SHA256).
		SetMime(asset.MIME).
		SetFilename(asset.Filename).
		SetSize(asset.Size).
		SetVisibility(asset.Visibility).
		SetStatus(asset.Status).
		SetStorageProfileID(asset.StorageProfileID).
		SetPublicBaseURL(asset.PublicBaseURL).
		Save(ctx)
	if err != nil {
		return err
	}
	mediaAssetEntityToService(asset, created)
	return nil
}

func (r *mediaAssetRepository) GetByID(ctx context.Context, id int64) (*service.MediaAsset, error) {
	client := clientFromContext(ctx, r.client)
	m, err := client.MediaAsset.Query().
		Where(mediaasset.IDEQ(id)).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrMediaNotFound, nil)
	}
	out := &service.MediaAsset{}
	mediaAssetEntityToService(out, m)
	return out, nil
}

func (r *mediaAssetRepository) ListByBiz(ctx context.Context, ownerUserID int64, bizType, bizID string) ([]service.MediaAsset, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.MediaAsset.Query().
		Where(
			mediaasset.OwnerUserIDEQ(ownerUserID),
			mediaasset.BizTypeEQ(bizType),
			mediaasset.BizIDEQ(bizID),
			mediaasset.StatusEQ(service.MediaStatusReady),
		).
		Order(mediaasset.ByID()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.MediaAsset, 0, len(rows))
	for _, row := range rows {
		item := service.MediaAsset{}
		mediaAssetEntityToService(&item, row)
		out = append(out, item)
	}
	return out, nil
}

func mediaAssetEntityToService(dst *service.MediaAsset, src *dbent.MediaAsset) {
	dst.ID = src.ID
	dst.OwnerUserID = src.OwnerUserID
	dst.BizType = src.BizType
	dst.BizID = src.BizID
	dst.StorageKey = src.StorageKey
	dst.SHA256 = src.Sha256
	dst.MIME = src.Mime
	dst.Filename = src.Filename
	dst.Size = src.Size
	dst.Visibility = src.Visibility
	dst.Status = src.Status
	dst.StorageProfileID = src.StorageProfileID
	dst.PublicBaseURL = src.PublicBaseURL
	dst.CreatedAt = src.CreatedAt
	if dst.Visibility == service.MediaVisibilityPublic {
		dst.AccessURL = service.MediaPublicPath(dst.ID)
	}
}
