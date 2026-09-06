//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

type mediaPricingUsageStub struct {
	UsageLogRepository
	rows    []UsageLog
	filters usagestats.UsageLogFilters
	calls   int
}

func (s *mediaPricingUsageStub) ListWithFilters(_ context.Context, p pagination.PaginationParams, f usagestats.UsageLogFilters) ([]UsageLog, *pagination.PaginationResult, error) {
	s.calls++
	s.filters = f
	return s.rows, nil, nil
}

type mediaPricingReceiptStub struct {
	recorded bool
	err      error
	request  string
	keyID    int64
}

type mediaPricingKeyStub struct {
	APIKeyRepository
	key *APIKey
}

func (s *mediaPricingKeyStub) GetByUserGroupAndPurpose(context.Context, int64, int64, string) (*APIKey, error) {
	return s.key, nil
}

func TestCreationMediaPricingImageReceiptChecksTaskAndKeyOwnership(t *testing.T) {
	groupID := int64(3)
	group := &Group{ID: groupID, Status: StatusActive, Platform: PlatformOpenAI}
	creation := NewCreationService(nil, nil, nil, &groupRepoStubForGroupUpdate{group: group}, &creationTestUserRepo{}, &userSubRepoStubForGroupUpdate{})
	keys := &mediaPricingKeyStub{key: &APIKey{ID: 9, UserID: 7, GroupID: &groupID}}
	store := &imageTaskMemoryStore{}
	images := NewImageTaskService(store).ForLocalResults()
	ctx := context.WithValue(context.Background(), ctxkey.ClientRequestID, "server-owned")
	task, err := images.Create(ctx, ImageTaskOwner{UserID: 7, APIKeyID: 9})
	require.NoError(t, err)
	receipts := &mediaPricingReceiptStub{}
	s := &CreationMediaPricingService{creation: creation, keys: &CreationKeyResolver{apiKeyRepo: keys}, images: images, receipts: receipts, usage: &mediaPricingUsageStub{}}
	price, err := s.Receipt(context.Background(), 7, groupID, "image", task.ID)
	require.NoError(t, err)
	require.Equal(t, "pending", price.Status)
	require.Equal(t, "client:server-owned", receipts.request)
	store.task.Status = ImageTaskStatusFailed
	price, err = s.Receipt(context.Background(), 7, groupID, "image", task.ID)
	require.NoError(t, err)
	require.Equal(t, "unavailable", price.Status)
	require.Equal(t, "generation_failed", price.Reason)
	require.Nil(t, price.Amount)
	receipts.recorded = true
	s.usage = &mediaPricingUsageStub{rows: []UsageLog{{ID: 15, UserID: 7, APIKeyID: 9, GroupID: &groupID, RequestID: "client:server-owned", ActualCost: 0.25, BillingType: BillingTypeBalance}}}
	s.cfg = &config.Config{RunMode: config.RunModeSimple}
	price, err = s.Receipt(context.Background(), 7, groupID, "image", task.ID)
	require.NoError(t, err)
	require.Equal(t, "settled", price.Status, "failed result and a later run-mode change must not hide an existing charge")
	require.Equal(t, 0.25, *price.Amount)
	receipts.recorded = false
	s.usage = &mediaPricingUsageStub{}
	store.task.Status = ImageTaskStatusProcessing
	for _, mutate := range []func(){
		func() { keys.key.ID = 10 },
		func() { keys.key.UserID = 8 },
		func() { other := int64(4); keys.key.GroupID = &other },
		func() { store.task.LocalOnly = false },
	} {
		keys.key = &APIKey{ID: 9, UserID: 7, GroupID: &groupID}
		store.task.LocalOnly = true
		receipts.request = ""
		mutate()
		_, err := s.Receipt(context.Background(), 7, groupID, "image", task.ID)
		require.ErrorIs(t, err, ErrImageTaskNotFound)
		require.Empty(t, receipts.request, "ownership must be verified before touching billing receipts")
	}
	keys.key = &APIKey{ID: 9, UserID: 7, GroupID: &groupID}
	store.task.LocalOnly = true
	store.task.BillingRequestID = ""
	price, err = s.Receipt(context.Background(), 7, groupID, "image", task.ID)
	require.NoError(t, err)
	require.Equal(t, "unavailable", price.Status)
	require.Nil(t, price.Amount)
}

func (s *mediaPricingReceiptStub) Recorded(_ context.Context, request string, keyID int64) (bool, error) {
	s.request, s.keyID = request, keyID
	return s.recorded, s.err
}

func TestCreationMediaPricingReceiptRequiresCommittedOwnedAmount(t *testing.T) {
	groupID := int64(3)
	base := UsageLog{ID: 1, UserID: 7, APIKeyID: 9, GroupID: &groupID, RequestID: "client:trusted", ActualCost: 0.125, BillingType: BillingTypeBalance}
	for _, tt := range []struct {
		name     string
		recorded bool
		rows     []UsageLog
		status   string
	}{
		{"no receipt", false, []UsageLog{base}, "pending"},
		{"receipt before usage", true, nil, "pending"},
		{"settled", true, []UsageLog{base}, "settled"},
		{"ambiguous", true, []UsageLog{base, base}, "unavailable"},
		{"zero is not free", true, []UsageLog{{UserID: 7, APIKeyID: 9, GroupID: &groupID, RequestID: base.RequestID}}, "unavailable"},
		{"wrong owner", true, []UsageLog{{UserID: 8, APIKeyID: 9, GroupID: &groupID, RequestID: base.RequestID, ActualCost: 1}}, "unavailable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			receipts := &mediaPricingReceiptStub{recorded: tt.recorded}
			usage := &mediaPricingUsageStub{rows: tt.rows}
			s := &CreationMediaPricingService{receipts: receipts, usage: usage}
			price, err := s.readReceipt(context.Background(), 7, groupID, 9, base.RequestID)
			require.NoError(t, err)
			require.Equal(t, tt.status, price.Status)
			require.Equal(t, base.RequestID, receipts.request)
			require.Equal(t, int64(9), receipts.keyID)
			if tt.recorded {
				require.Equal(t, usagestats.UsageLogFilters{UserID: 7, APIKeyID: 9, GroupID: 3, RequestID: base.RequestID}, usage.filters)
			} else {
				require.Zero(t, usage.calls)
			}
			if tt.status == "settled" {
				require.Equal(t, base.ActualCost, *price.Amount)
				require.Equal(t, "balance", *price.BillingTarget)
			} else {
				require.Nil(t, price.Amount)
				require.Nil(t, price.BillingTarget)
			}
		})
	}
	base.BillingType = BillingTypeSubscription
	s := &CreationMediaPricingService{receipts: &mediaPricingReceiptStub{recorded: true}, usage: &mediaPricingUsageStub{rows: []UsageLog{base}}}
	price, err := s.readReceipt(context.Background(), 7, groupID, 9, base.RequestID)
	require.NoError(t, err)
	require.Equal(t, "subscription", *price.BillingTarget)
	s.receipts = &mediaPricingReceiptStub{err: errors.New("database unavailable")}
	_, err = s.readReceipt(context.Background(), 7, groupID, 9, base.RequestID)
	require.Error(t, err)
}

func TestCreationMediaPricingConfiguredQuotes(t *testing.T) {
	imagePrice, videoPrice := 0.2, 0.1
	group := &Group{ID: 3, RateMultiplier: 2, ImagePrice1K: &imagePrice, VideoPrice720P: &videoPrice}
	key := &APIKey{Group: group, GroupID: &group.ID}
	billing := &BillingService{}
	s := &CreationMediaPricingService{openAI: &OpenAIGatewayService{billingService: billing, resolver: NewModelPricingResolver(nil, billing)}}
	image := CreationMediaPriceInput{Kind: "image", Size: "1024x1024"}
	price := s.estimateConfigured(context.Background(), image, "gpt-image-1", key, nil, 2)
	require.Equal(t, "estimated", price.Status)
	require.InDelta(t, 0.4, *price.Amount, 1e-8)
	group.ImageRateIndependent, group.ImageRateMultiplier = true, 0.5
	price = s.estimateConfigured(context.Background(), image, "gpt-image-1", key, nil, 2)
	require.InDelta(t, 0.1, *price.Amount, 1e-8)
	image.Size = "auto"
	require.Equal(t, "unavailable", s.estimateConfigured(context.Background(), image, "gpt-image-1", key, nil, 2).Status)
	image.Size = "4K"
	require.Nil(t, s.estimateConfigured(context.Background(), image, "gpt-image-1", key, nil, 2).Amount)
	image.Size = "1K"
	require.Equal(t, "unavailable", s.estimateConfigured(context.Background(), image, "gpt-image-1", key, &ResolvedPricing{Mode: BillingModeToken}, 2).Status)
	video := CreationMediaPriceInput{Kind: "video", Resolution: "720p", Duration: 6}
	price = s.estimateConfigured(context.Background(), video, "grok-imagine-video", key, nil, 2)
	require.InDelta(t, 1.2, *price.Amount, 1e-8)
	video.Duration = 0
	require.Nil(t, s.estimateConfigured(context.Background(), video, "grok-imagine-video", key, nil, 2).Amount)
	video.Duration, video.Resolution = 6, ""
	require.Nil(t, s.estimateConfigured(context.Background(), video, "grok-imagine-video", key, nil, 2).Amount)
	image.Size = "1K"
	resolved := &ResolvedPricing{Mode: BillingModeImage, Source: PricingSourceGroup, DefaultPerRequestPrice: 0.8}
	price = s.estimateConfigured(context.Background(), image, "gpt-image-1", key, resolved, 2)
	require.InDelta(t, 0.4, *price.Amount, 1e-8, "group model overrides flat group price, using independent multiplier")
	resolved.Source = PricingSourceChannel
	price = s.estimateConfigured(context.Background(), image, "gpt-image-1", key, resolved, 2)
	require.InDelta(t, 0.1, *price.Amount, 1e-8, "flat group price overrides channel")
}

func TestCreationMediaPricingSimpleAndUnknownPlatform(t *testing.T) {
	group := &Group{ID: 3, Status: StatusActive, Platform: PlatformGemini}
	creation := NewCreationService(nil, nil, nil, &groupRepoStubForGroupUpdate{group: group}, &creationTestUserRepo{}, &userSubRepoStubForGroupUpdate{})
	s := &CreationMediaPricingService{creation: creation, cfg: &config.Config{RunMode: config.RunModeSimple}}
	input := CreationMediaPriceInput{UserID: 7, GroupID: 3, Kind: "image", Model: "gemini-image", Size: "1K"}
	price, err := s.Estimate(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "not_billed", price.Status)
	require.Zero(t, *price.Amount)
	s.cfg.RunMode = config.RunModeStandard
	price, err = s.Estimate(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "unavailable", price.Status)
	require.Nil(t, price.Amount)
	input.Kind = "other"
	_, err = s.Estimate(context.Background(), input)
	require.ErrorIs(t, err, ErrCreationMediaPricingInput)
}

func TestImageTaskBillingAssociationIsPrivateAndUsesTrustedContext(t *testing.T) {
	store := &imageTaskMemoryStore{}
	s := NewImageTaskService(store).ForLocalResults()
	ctx := context.WithValue(context.Background(), ctxkey.ClientRequestID, "trusted-server-id")
	task, err := s.Create(ctx, ImageTaskOwner{UserID: 7, APIKeyID: 9})
	require.NoError(t, err)
	require.Equal(t, "client:trusted-server-id", store.task.BillingRequestID)
	loaded, err := s.Get(context.Background(), ImageTaskOwner{UserID: 7, APIKeyID: 9}, task.ID)
	require.NoError(t, err)
	require.Equal(t, store.task.BillingRequestID, loaded.BillingRequestID)
	data, err := json.Marshal(loaded)
	require.NoError(t, err)
	require.NotContains(t, string(data), "trusted-server-id")
	_, err = s.Get(context.Background(), ImageTaskOwner{UserID: 8, APIKeyID: 9}, task.ID)
	require.ErrorIs(t, err, ErrImageTaskNotFound)
	task, err = s.Create(context.Background(), ImageTaskOwner{UserID: 7, APIKeyID: 9})
	require.NoError(t, err)
	require.Empty(t, task.BillingRequestID, "no invented billing ID when middleware context is missing")
}
