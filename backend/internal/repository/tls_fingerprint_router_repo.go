package repository

import (
	"context"
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/tlsfingerprintrouter"
	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type tlsFingerprintRouterRepository struct {
	client *ent.Client
}

// NewTLSFingerprintRouterRepository 创建 TLS 指纹路由仓库。
func NewTLSFingerprintRouterRepository(client *ent.Client) service.TLSFingerprintRouterRepository {
	return &tlsFingerprintRouterRepository{client: client}
}

func (r *tlsFingerprintRouterRepository) List(ctx context.Context) ([]*model.TLSFingerprintRouter, error) {
	routers, err := r.client.TLSFingerprintRouter.Query().
		Order(ent.Asc(tlsfingerprintrouter.FieldName)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*model.TLSFingerprintRouter, len(routers))
	for i, router := range routers {
		result[i] = r.toModel(router)
	}
	return result, nil
}

func (r *tlsFingerprintRouterRepository) GetByID(ctx context.Context, id int64) (*model.TLSFingerprintRouter, error) {
	router, err := r.client.TLSFingerprintRouter.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(router), nil
}

func (r *tlsFingerprintRouterRepository) Create(ctx context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error) {
	builder := r.client.TLSFingerprintRouter.Create().
		SetName(router.Name).
		SetEnabled(router.Enabled).
		SetRules(routerRulesToRaw(router.Rules))
	if router.Description != nil {
		builder.SetDescription(*router.Description)
	}
	created, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toModel(created), nil
}

func (r *tlsFingerprintRouterRepository) Update(ctx context.Context, router *model.TLSFingerprintRouter) (*model.TLSFingerprintRouter, error) {
	builder := r.client.TLSFingerprintRouter.UpdateOneID(router.ID).
		SetName(router.Name).
		SetEnabled(router.Enabled).
		SetRules(routerRulesToRaw(router.Rules))
	if router.Description != nil {
		builder.SetDescription(*router.Description)
	} else {
		builder.ClearDescription()
	}
	updated, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toModel(updated), nil
}

func (r *tlsFingerprintRouterRepository) Delete(ctx context.Context, id int64) error {
	return r.client.TLSFingerprintRouter.DeleteOneID(id).Exec(ctx)
}

func (r *tlsFingerprintRouterRepository) toModel(e *ent.TLSFingerprintRouter) *model.TLSFingerprintRouter {
	router := &model.TLSFingerprintRouter{
		ID:          e.ID,
		Name:        e.Name,
		Description: e.Description,
		Enabled:     e.Enabled,
		Rules:       rawRouterRulesToModel(e.Rules),
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
	if router.Rules == nil {
		router.Rules = []model.TLSFingerprintRouterRule{}
	}
	return router
}

func routerRulesToRaw(rules []model.TLSFingerprintRouterRule) []map[string]any {
	if rules == nil {
		return []map[string]any{}
	}
	data, err := json.Marshal(rules)
	if err != nil {
		return []map[string]any{}
	}
	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return []map[string]any{}
	}
	if raw == nil {
		return []map[string]any{}
	}
	return raw
}

func rawRouterRulesToModel(raw []map[string]any) []model.TLSFingerprintRouterRule {
	if raw == nil {
		return []model.TLSFingerprintRouterRule{}
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return []model.TLSFingerprintRouterRule{}
	}
	var rules []model.TLSFingerprintRouterRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return []model.TLSFingerprintRouterRule{}
	}
	if rules == nil {
		return []model.TLSFingerprintRouterRule{}
	}
	return rules
}
