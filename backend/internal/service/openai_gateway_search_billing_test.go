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
	breakdown, err := svc.calculateOpenAIRecordUsageCost(context.Background(), &OpenAIForwardResult{SearchCount: 2, Duration: time.Second}, &APIKey{GroupID: &groupID, Group: &Group{ID: groupID, SearchPricePer1k: &pricePer1k}}, "", 1, 1, UsageTokens{}, "", RequestTypeUnknown)
	require.NoError(t, err)
	require.NotNil(t, breakdown)
	require.Equal(t, "search", breakdown.BillingMode)
	require.Equal(t, 0.015, breakdown.TotalCost)
}
