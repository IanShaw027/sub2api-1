package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// AvailableGroupRef 渠道视图中关联分组的简要信息。
//
// 用户侧「可用渠道」页面据此展示：专属分组 vs 公开分组（IsExclusive）、
// 订阅 vs 标准（SubscriptionType）、默认倍率（RateMultiplier）与高峰倍率规则。
// 用户专属倍率不在这里暴露，前端自己通过 /groups/rates 拉取，和 API 密钥页面保持一致。
type AvailableGroupRef struct {
	ID               int64
	Name             string
	DisplayName      string
	Platform         string
	SubscriptionType string
	RateMultiplier   float64
	IsExclusive      bool
	UserSelectable   bool
}

// AvailableChannel 可用渠道视图：用于「可用渠道」页面展示渠道基础信息 +
// 关联的分组 + 推导出的支持模型列表（无通配符）。
type AvailableChannel struct {
	ID                 int64
	Name               string
	Description        string
	Status             string
	BillingModelSource string
	RestrictModels     bool
	Groups             []AvailableGroupRef
	SupportedModels    []SupportedModel
}

// ListAvailable 返回所有渠道的可用视图：每个渠道附带关联分组信息与支持模型列表。
//
// 支持模型通过 (*Channel).SupportedModels() 计算（mapping ∪ pricing 并联）。
// 对于渠道未配置定价的模型，进一步用 PricingService 的全局 LiteLLM 数据合成
// 一份展示用定价，让用户看到默认价格而非"未配置"。
//
// 关联分组信息通过 groupRepo.ListActive 查询后按 ID 映射；渠道 GroupIDs 中未在活跃列表中
// 的分组（已停用或删除）会被忽略。
//
// 前置条件：s.groupRepo 必须非 nil（由 wire DI 保证）。直接 nil-deref 用于 fail-fast，
// 避免静默掩盖注入缺失。
func (s *ChannelService) ListAvailable(ctx context.Context) ([]AvailableChannel, error) {
	channels, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}

	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups: %w", err)
	}
	groupByID := make(map[int64]AvailableGroupRef, len(groups))
	for i := range groups {
		g := groups[i]
		groupByID[g.ID] = AvailableGroupRef{
			ID:               g.ID,
			Name:             g.Name,
			DisplayName:      g.DisplayLabel(),
			Platform:         g.Platform,
			SubscriptionType: g.SubscriptionType,
			RateMultiplier:   g.RateMultiplier,
			IsExclusive:      g.IsExclusive,
			UserSelectable:   g.UserSelectable,
		}
	}

	out := make([]AvailableChannel, 0, len(channels))
	for i := range channels {
		ch := &channels[i]
		groups := make([]AvailableGroupRef, 0, len(ch.GroupIDs))
		for _, gid := range ch.GroupIDs {
			if ref, ok := groupByID[gid]; ok {
				groups = append(groups, ref)
			}
		}
		sort.SliceStable(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })

		ch.normalizeBillingModelSource()

		supported := filterCapabilityModelsForAvailable(ch.SupportedModels())
		supported, err = s.filterAvailableModelsBySchedulableAccounts(ctx, groups, supported)
		if err != nil {
			return nil, err
		}
		s.fillGlobalPricingFallback(supported)

		out = append(out, AvailableChannel{
			ID:                 ch.ID,
			Name:               ch.Name,
			Description:        ch.Description,
			Status:             ch.Status,
			BillingModelSource: ch.BillingModelSource,
			RestrictModels:     ch.RestrictModels,
			Groups:             groups,
			SupportedModels:    supported,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// FilterAvailableModelsForGroups narrows available-channel supported models to the
// schedulable capabilities of the provided visible groups. It is used by the
// user-facing available-channels handler after group visibility filtering so
// same-platform hidden groups cannot leak their models into the response.
func (s *ChannelService) FilterAvailableModelsForGroups(ctx context.Context, groups []AvailableGroupRef, models []SupportedModel) ([]SupportedModel, error) {
	return s.filterAvailableModelsBySchedulableAccounts(ctx, groups, models)
}

type availableGroupPlatformKey struct {
	groupID  int64
	platform string
}

// filterAvailableModelsBySchedulableAccounts intersects channel-advertised capabilities
// with currently schedulable account capabilities for the active groups attached to
// the channel. Accounts with no model_mapping are treated as unrestricted for that
// platform, so they preserve the channel capability list instead of trying to
// synthesize an infinite model catalog.
func (s *ChannelService) filterAvailableModelsBySchedulableAccounts(ctx context.Context, groups []AvailableGroupRef, models []SupportedModel) ([]SupportedModel, error) {
	if s.accountRepo == nil || len(groups) == 0 || len(models) == 0 {
		return models, nil
	}

	accountsByKey := make(map[availableGroupPlatformKey][]Account, len(groups))
	for _, group := range groups {
		key := availableGroupPlatformKey{groupID: group.ID, platform: group.Platform}
		if _, ok := accountsByKey[key]; ok {
			continue
		}
		accounts, err := s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, group.ID, group.Platform)
		if err != nil {
			return nil, fmt.Errorf("list schedulable accounts for available channel group %d platform %s: %w", group.ID, group.Platform, err)
		}
		accountsByKey[key] = accounts
	}

	out := make([]SupportedModel, 0, len(models))
	for _, model := range models {
		if schedulableAccountsSupportModel(groups, accountsByKey, model) {
			out = append(out, model)
		}
	}
	return out, nil
}

func schedulableAccountsSupportModel(groups []AvailableGroupRef, accountsByKey map[availableGroupPlatformKey][]Account, model SupportedModel) bool {
	for _, group := range groups {
		if group.Platform != model.Platform {
			continue
		}
		accounts := accountsByKey[availableGroupPlatformKey{groupID: group.ID, platform: group.Platform}]
		for i := range accounts {
			if accountSupportsAvailableModel(&accounts[i], model.Name) {
				return true
			}
		}
	}
	return false
}

func accountSupportsAvailableModel(account *Account, modelName string) bool {
	if account == nil || !account.IsSchedulable() {
		return false
	}
	mapping := account.GetModelMapping()
	if len(mapping) == 0 {
		return true
	}
	return account.IsModelSupported(modelName)
}

// fillGlobalPricingFallback 对未命中渠道定价的支持模型，从全局 LiteLLM 数据合成一份
// 展示用定价（按 token 计费）。仅用于「可用渠道」展示，不影响真实计费链路。
//
// 当 s.pricingService 为 nil（测试场景），跳过回落。
func (s *ChannelService) fillGlobalPricingFallback(models []SupportedModel) {
	if s.pricingService == nil {
		return
	}
	for i := range models {
		if models[i].Pricing != nil {
			continue
		}
		lookupName := models[i].PricingLookupName
		if lookupName == "" {
			lookupName = models[i].Name
		}
		lp := s.pricingService.GetModelPricing(lookupName)
		if lp == nil {
			continue
		}
		models[i].Pricing = synthesizePricingFromLiteLLM(lp)
	}
}

// filterCapabilityModelsForAvailable keeps models that are either backed by
// channel mapping/capability OR have explicit channel pricing configured.
// This ensures that models with pricing entries are visible in the available
// channels view even without a corresponding mapping entry.
func filterCapabilityModelsForAvailable(models []SupportedModel) []SupportedModel {
	out := make([]SupportedModel, 0, len(models))
	for i := range models {
		if models[i].IsCapability || models[i].Pricing != nil {
			out = append(out, models[i])
		}
	}
	return out
}

// synthesizePricingFromLiteLLM 把 LiteLLM 的定价数据转成 ChannelModelPricing 形态，
// 仅用于展示。BillingMode 固定为 token；图片场景的 OutputCostPerImageToken 也归到
// ImageOutputPrice 字段（与渠道侧"图片输出按 token 计价"语义一致）。
//
// LiteLLM 中字段 0 视为未配置，不带入展示。
func synthesizePricingFromLiteLLM(lp *LiteLLMModelPricing) *ChannelModelPricing {
	if lp == nil {
		return nil
	}
	return &ChannelModelPricing{
		BillingMode:      BillingModeToken,
		InputPrice:       nonZeroPtr(lp.InputCostPerToken),
		OutputPrice:      nonZeroPtr(lp.OutputCostPerToken),
		CacheWritePrice:  nonZeroPtr(lp.CacheCreationInputTokenCost),
		CacheReadPrice:   nonZeroPtr(lp.CacheReadInputTokenCost),
		ImageOutputPrice: nonZeroPtr(lp.OutputCostPerImageToken),
	}
}

func nonZeroPtr(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}
