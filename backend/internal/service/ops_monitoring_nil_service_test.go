//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpsServiceMonitoring_NilServiceFailsClosed(t *testing.T) {
	var svc *OpsService

	require.False(t, svc.IsMonitoringEnabled(context.Background()))
	require.False(t, svc.IsRealtimeMonitoringEnabled(context.Background()))
	require.ErrorIs(t, svc.RequireMonitoringEnabled(context.Background()), ErrOpsDisabled)
}

func TestOpsServiceGetAccountAvailabilityRejectsNilService(t *testing.T) {
	var svc *OpsService

	availability, err := svc.GetAccountAvailability(context.Background(), "", nil)

	require.Nil(t, availability)
	require.EqualError(t, err, "ops service is required")
}
