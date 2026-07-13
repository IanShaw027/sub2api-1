//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayServiceCalculateOpenAIRecordUsageCost_SearchUsesGroupPrice(t *testing.T) {
	pricePer1k := 7.5
	svc := &OpenAIGatewayService{billingService: NewBillingService(nil, nil)}
	groupID := int64(123)
	breakdown, err := svc.calculateOpenAIRecordUsageCost(context.Background(), &OpenAIForwardResult{SearchCount: 2, Duration: time.Second}, &APIKey{GroupID: &groupID, Group: &Group{ID: groupID, SearchPricePer1k: &pricePer1k}}, "", 1, 1, 1, 1, UsageTokens{}, "", RequestTypeUnknown)
	require.NoError(t, err)
	require.NotNil(t, breakdown)
	require.Equal(t, "search", breakdown.BillingMode)
	require.Equal(t, 0.015, breakdown.TotalCost)
}

func TestOpenAIGatewayServiceCalculateOpenAIRecordUsageCost_SearchDoesNotReplaceTokenCost(t *testing.T) {
	t.Parallel()

	pricePer1k := 5.0
	svc := &OpenAIGatewayService{billingService: NewBillingService(nil, nil)}
	groupID := int64(123)
	usage := UsageTokens{
		InputTokens:  1000,
		OutputTokens: 200,
	}

	tokenOnlyCost := expectedOpenAICost(t, svc, "grok-4.3", OpenAIUsage{
		InputTokens:  1000,
		OutputTokens: 200,
	}, 1)
	searchCost := svc.billingService.CalculateSearchCost(2, &pricePer1k, 1)

	breakdown, err := svc.calculateOpenAIRecordUsageCost(context.Background(),
		&OpenAIForwardResult{SearchCount: 2, Duration: time.Second, Model: "grok-4.3"},
		&APIKey{GroupID: &groupID, Group: &Group{ID: groupID, SearchPricePer1k: &pricePer1k}},
		"grok-4.3",
		1,
		1,
		1,
		1,
		usage,
		"",
		RequestTypeUnknown,
	)
	require.NoError(t, err)
	require.NotNil(t, breakdown)
	require.Greater(t, breakdown.InputCost, 0.0)
	require.Greater(t, breakdown.OutputCost, 0.0)
	require.InDelta(t, tokenOnlyCost.TotalCost+searchCost.TotalCost, breakdown.TotalCost, 1e-12)
	require.InDelta(t, tokenOnlyCost.ActualCost+searchCost.ActualCost, breakdown.ActualCost, 1e-12)
}
