//go:build unit

package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestReadPricingNormalizesNestedAndLegacyModes(t *testing.T) {
	t.Run("nested paid mode", func(t *testing.T) {
		pricing := readPricing(map[string]any{
			metaKeySkillPricing: map[string]any{
				"mode":   "PAID",
				"amount": 12.5,
			},
		})
		require.Equal(t, domain.AISkillPriceModePaid, pricing.Mode)
		require.Equal(t, 12.5, pricing.Amount)
	})

	t.Run("legacy paid mode", func(t *testing.T) {
		pricing := readPricing(map[string]any{
			"price_mode":   "paid",
			"price_amount": 7.25,
		})
		require.Equal(t, domain.AISkillPriceModePaid, pricing.Mode)
		require.Equal(t, 7.25, pricing.Amount)
	})

	t.Run("fallback to paid when amount is present", func(t *testing.T) {
		pricing := readPricing(map[string]any{
			metaKeySkillPricing: map[string]any{
				"amount": 3.5,
			},
		})
		require.Equal(t, domain.AISkillPriceModePaid, pricing.Mode)
		require.Equal(t, 3.5, pricing.Amount)
	})
}
