package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func loadGroupOpenAIImageMainModels(ctx context.Context, sqlq sqlExecutor, groupIDs []int64) (map[int64]string, error) {
	models := make(map[int64]string, len(groupIDs))
	if sqlq == nil || len(groupIDs) == 0 {
		return models, nil
	}

	rows, err := sqlq.QueryContext(ctx, `
		SELECT id, openai_image_main_model
		FROM groups
		WHERE id = ANY($1) AND deleted_at IS NULL
	`, pq.Array(groupIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			groupID int64
			model   string
		)
		if err := rows.Scan(&groupID, &model); err != nil {
			return nil, err
		}
		models[groupID] = service.NormalizeOpenAIImageMainModel(model)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return models, nil
}

func saveGroupOpenAIImageMainModel(ctx context.Context, sqlq sqlExecutor, groupID int64, model string) error {
	if sqlq == nil || groupID <= 0 {
		return nil
	}
	_, err := sqlq.ExecContext(ctx, `
		UPDATE groups
		SET openai_image_main_model = $1
		WHERE id = $2
	`, service.NormalizeOpenAIImageMainModel(model), groupID)
	return err
}

func applyOpenAIImageMainModelsToGroups(groups []service.Group, models map[int64]string) {
	for i := range groups {
		if model, ok := models[groups[i].ID]; ok {
			groups[i].OpenAIImageMainModel = service.NormalizeOpenAIImageMainModel(model)
			continue
		}
		groups[i].OpenAIImageMainModel = service.NormalizeOpenAIImageMainModel(groups[i].OpenAIImageMainModel)
	}
}
