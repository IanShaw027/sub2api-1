package service

import (
	"context"
	"math"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

var ErrCreationMediaPricingInput = infraerrors.New(http.StatusBadRequest, "INVALID_MEDIA_PRICING_INPUT", "invalid media pricing input")

type CreationMediaPrice struct {
	Status        string   `json:"status"`
	Currency      string   `json:"currency"`
	Amount        *float64 `json:"amount"`
	BillingTarget *string  `json:"billing_target"`
	Reason        string   `json:"reason,omitempty"`
	UsageID       *int64   `json:"usage_id,omitempty"`
}

// Quotes describe one requested output. Actual size, duration and price may change.
type CreationMediaPriceInput struct {
	UserID     int64
	GroupID    int64
	Kind       string
	Model      string
	Size       string
	Duration   int
	Resolution string
}

type CreationMediaPricingService struct {
	creation *CreationService
	keys     *CreationKeyResolver
	openAI   *OpenAIGatewayService
	images   *ImageTaskService
	usage    UsageLogRepository
	receipts CreationVideoBillingLookup
	cfg      *config.Config
}

func NewCreationMediaPricingService(creation *CreationService, keys *CreationKeyResolver, openAI *OpenAIGatewayService, images *ImageTaskService, usage UsageLogRepository, receipts CreationVideoBillingLookup, cfg *config.Config) *CreationMediaPricingService {
	return &CreationMediaPricingService{creation: creation, keys: keys, openAI: openAI, images: images.ForLocalResults(), usage: usage, receipts: receipts, cfg: cfg}
}

func mediaPriceState(status, reason string) *CreationMediaPrice {
	return &CreationMediaPrice{Status: status, Currency: "USD", Reason: reason}
}

func (s *CreationMediaPricingService) Estimate(ctx context.Context, input CreationMediaPriceInput) (*CreationMediaPrice, error) {
	if input.GroupID <= 0 || (input.Kind != "image" && input.Kind != "video") || strings.TrimSpace(input.Model) == "" || len(input.Model) > 256 || input.Duration < 0 || input.Duration > 3600 {
		return nil, ErrCreationMediaPricingInput
	}
	if err := s.creation.EnsureUserCanUseGroup(ctx, input.UserID, input.GroupID); err != nil {
		return nil, err
	}
	if s.cfg != nil && s.cfg.RunMode == config.RunModeSimple {
		price := mediaPriceState("not_billed", "simple_mode")
		zero := 0.0
		price.Amount = &zero
		return price, nil
	}
	group, err := s.creation.groups.GetByID(ctx, input.GroupID)
	if err != nil {
		return nil, err
	}
	if group.Platform != PlatformOpenAI || s.openAI == nil || s.openAI.billingService == nil || s.openAI.resolver == nil {
		return mediaPriceState("unavailable", "pricing_not_deterministic"), nil
	}
	model := strings.TrimSpace(input.Model)
	modelPinned := false
	if s.openAI.channelService != nil {
		channel, err := s.openAI.channelService.GetChannelForGroup(ctx, input.GroupID)
		if err != nil {
			return mediaPriceState("unavailable", "pricing_unavailable"), nil
		}
		if channel != nil {
			mapping := s.openAI.channelService.ResolveChannelMapping(ctx, input.GroupID, model)
			switch mapping.BillingModelSource {
			case BillingModelSourceRequested:
				modelPinned = true
			case BillingModelSourceChannelMapped:
				if mapping.Mapped && mapping.MappedModel != "" && mapping.MappedModel != model {
					model = mapping.MappedModel
					modelPinned = true
				}
			default:
				return mediaPriceState("unavailable", "billing_model_unknown"), nil
			}
			if !modelPinned {
				return mediaPriceState("unavailable", "billing_model_unknown"), nil
			}
		}
	}
	if !modelPinned && (len(group.ModelPricing) > 0 || (input.Kind == "video" && len(group.VideoModelPrices) > 0)) {
		return mediaPriceState("unavailable", "billing_model_unknown"), nil
	}
	apiKey := &APIKey{UserID: input.UserID, GroupID: &group.ID, Group: group}
	resolved := s.openAI.resolveOpenAIChannelPricing(ctx, model, apiKey)
	if resolved != nil && resolved.Mode == BillingModeToken {
		return mediaPriceState("unavailable", "token_usage_unknown"), nil
	}
	multiplier := s.openAI.ResolveUserGroupRateMultiplier(ctx, input.UserID, group.ID, group.RateMultiplier)
	return s.estimateConfigured(ctx, input, model, apiKey, resolved, multiplier), nil
}

func (s *CreationMediaPricingService) estimateConfigured(ctx context.Context, input CreationMediaPriceInput, model string, key *APIKey, resolved *ResolvedPricing, multiplier float64) *CreationMediaPrice {
	if resolved != nil && resolved.Mode == BillingModeToken {
		return mediaPriceState("unavailable", "token_usage_unknown")
	}
	var cost *CostBreakdown
	var tier string
	var configured bool
	units := 1.0
	if input.Kind == "image" {
		var ok bool
		tier, ok = ClassifyImageBillingTier(input.Size)
		if !ok {
			return mediaPriceState("unavailable", "output_size_unknown")
		}
		multiplier = resolveImageRateMultiplier(key, multiplier)
		configured = apiKeyHasConfiguredImagePrice(key, tier)
	} else {
		tier = strings.ToLower(strings.TrimSpace(input.Resolution))
		if (tier != "480p" && tier != "720p" && tier != "1080p") || input.Duration <= 0 {
			return mediaPriceState("unavailable", "video_dimensions_unknown")
		}
		units = float64(input.Duration)
		multiplier = resolveVideoRateMultiplier(key, multiplier)
		configured = apiKeyHasConfiguredVideoPrice(key, model, tier)
	}
	// Mirror gateway precedence without falling through to guessed default prices.
	useResolved := resolved != nil && ((input.Kind == "image" && (resolved.Mode == BillingModeImage || resolved.Mode == BillingModePerRequest)) ||
		(input.Kind == "video" && (resolved.Mode == BillingModeVideo || (resolved.Source == PricingSourceChannel && (resolved.Mode == BillingModeImage || resolved.Mode == BillingModePerRequest)))))
	if useResolved && (resolved.Source == PricingSourceGroup || !configured) {
		if resolved.Mode != BillingModeVideo {
			units = 1
		}
		var err error
		cost, err = s.openAI.billingService.CalculateCostUnified(CostInput{Ctx: ctx, Model: model, GroupID: key.GroupID, Group: key.Group, RequestCount: 1, UsageUnits: units, SizeTier: tier, RateMultiplier: multiplier, Resolver: s.openAI.resolver, Resolved: resolved})
		if err != nil {
			return mediaPriceState("unavailable", "pricing_unavailable")
		}
		if cost.TotalCost == 0 && resolved.DefaultPerRequestPrice == 0 && (resolved.channelPricing == nil || resolved.channelPricing.PerRequestPrice == nil) {
			return mediaPriceState("unavailable", "price_not_configured")
		}
	} else if configured {
		if input.Kind == "image" {
			cost = s.openAI.billingService.CalculateImageCost(model, tier, 1, imagePriceConfigFromAPIKey(key), multiplier)
		} else {
			cost = s.openAI.billingService.CalculateVideoCost(model, tier, 1, input.Duration, videoPriceConfigFromAPIKey(key), multiplier)
		}
	}
	if cost == nil || math.IsNaN(cost.ActualCost) || math.IsInf(cost.ActualCost, 0) || cost.ActualCost < 0 {
		return mediaPriceState("unavailable", "price_not_configured")
	}
	price := mediaPriceState("estimated", "")
	amount := QuantizeUsageBillingAmount(cost.ActualCost)
	price.Amount = &amount
	target := "balance"
	if key.Group.IsSubscriptionType() {
		target = "subscription"
	}
	price.BillingTarget = &target
	return price
}

func (s *CreationMediaPricingService) Receipt(ctx context.Context, userID, groupID int64, kind, taskID string) (*CreationMediaPrice, error) {
	taskID = strings.TrimSpace(taskID)
	if groupID <= 0 || taskID == "" || len(taskID) > 256 || (kind != "image" && kind != "video") {
		return nil, ErrCreationMediaPricingInput
	}
	if err := s.creation.EnsureUserCanReadGroup(ctx, userID, groupID); err != nil {
		return nil, err
	}
	// Read the existing hidden key; a receipt request must not create or reactivate keys.
	key, err := s.keys.apiKeyRepo.GetByUserGroupAndPurpose(ctx, userID, groupID, APIKeyPurposeCreation)
	if err != nil {
		return nil, err
	}
	if key == nil || key.UserID != userID || key.GroupID == nil || *key.GroupID != groupID {
		return nil, ErrImageTaskNotFound
	}
	requestID := ""
	failed := false
	if kind == "image" {
		task, err := s.images.Get(ctx, ImageTaskOwner{UserID: userID, APIKeyID: key.ID}, taskID)
		if err != nil {
			return nil, err
		}
		requestID = task.BillingRequestID
		failed = task.Status == ImageTaskStatusFailed
	} else {
		accountID, err := s.openAI.ResolveGrokMediaVideoRequestAccount(ctx, &groupID, taskID, userID, key.ID)
		if err != nil || accountID <= 0 {
			return nil, ErrImageTaskNotFound
		}
		requestID = StableGrokVideoBillingRequestID(taskID)
	}
	if requestID == "" {
		return mediaPriceState("unavailable", "billing_correlation_missing"), nil
	}
	price, err := s.readReceipt(ctx, userID, groupID, key.ID, requestID)
	if err == nil && failed && price.Status == "pending" {
		return mediaPriceState("unavailable", "generation_failed"), nil
	}
	return price, err
}

func (s *CreationMediaPricingService) readReceipt(ctx context.Context, userID, groupID, keyID int64, requestID string) (*CreationMediaPrice, error) {
	if s.receipts == nil || s.usage == nil {
		return mediaPriceState("unavailable", "billing_receipt_unavailable"), nil
	}
	recorded, err := s.receipts.Recorded(ctx, requestID, keyID)
	if err != nil {
		return nil, err
	}
	if !recorded {
		return mediaPriceState("pending", "billing_not_recorded"), nil
	}
	rows, _, err := s.usage.ListWithFilters(ctx, pagination.PaginationParams{Page: 1, PageSize: 2}, usagestats.UsageLogFilters{UserID: userID, GroupID: groupID, APIKeyID: keyID, RequestID: requestID})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return mediaPriceState("pending", "usage_not_recorded"), nil
	}
	if len(rows) != 1 {
		return mediaPriceState("unavailable", "billing_ambiguous"), nil
	}
	row := rows[0]
	// A failed charge also writes a zero-cost usage row. A later successful retry
	// can leave that row unchanged, so zero is not proof of a free generation.
	if row.UserID != userID || row.APIKeyID != keyID || row.GroupID == nil || *row.GroupID != groupID || row.RequestID != requestID || row.ActualCost <= 0 || math.IsNaN(row.ActualCost) || math.IsInf(row.ActualCost, 0) {
		return mediaPriceState("unavailable", "billing_amount_unverified"), nil
	}
	target := "balance"
	if row.BillingType == BillingTypeSubscription {
		target = "subscription"
	} else if row.BillingType != BillingTypeBalance {
		return mediaPriceState("unavailable", "billing_target_unknown"), nil
	}
	amount := QuantizeUsageBillingAmount(row.ActualCost)
	return &CreationMediaPrice{Status: "settled", Currency: "USD", Amount: &amount, BillingTarget: &target, UsageID: &row.ID}, nil
}
