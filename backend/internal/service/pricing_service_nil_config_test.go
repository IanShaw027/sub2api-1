//go:build unit

package service

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewPricingService_NilConfigDoesNotPanicOnPathHelpers(t *testing.T) {
	t.Parallel()

	svc := NewPricingService(nil, nil)

	require.NotPanics(t, func() {
		require.Equal(t, filepath.Join("", "model_pricing.json"), svc.getPricingFilePath())
		require.Equal(t, filepath.Join("", "model_pricing.sha256"), svc.getHashFilePath())
	})
}
