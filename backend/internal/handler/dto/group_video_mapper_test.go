package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupFromServiceClearsStaleVideoConfigForNonGrokGroups(t *testing.T) {
	price := 0.04
	out := GroupFromService(&service.Group{
		ID:                    1,
		Name:                  "openai-stale-video",
		Platform:              service.PlatformOpenAI,
		AllowVideoGeneration:  true,
		VideoGenerationRoute:  "native",
		VideoPrice480pPerSec:  &price,
		VideoPrice720pPerSec:  &price,
		VideoPrice1080pPerSec: &price,
		VideoPrice4kPerSec:    &price,
	})

	require.NotNil(t, out)
	require.False(t, out.AllowVideoGeneration)
	require.Equal(t, service.GroupVideoGenerationRouteNative, out.VideoGenerationRoute)
	require.Nil(t, out.VideoPrice480pPerSec)
	require.Nil(t, out.VideoPrice720pPerSec)
	require.Nil(t, out.VideoPrice1080pPerSec)
	require.Nil(t, out.VideoPrice4kPerSec)
}

func TestGroupFromServicePreservesGrokVideoConfig(t *testing.T) {
	price := 0.04
	out := GroupFromService(&service.Group{
		ID:                    2,
		Name:                  "grok-video",
		Platform:              service.PlatformGrok,
		AllowVideoGeneration:  true,
		VideoGenerationRoute:  "native",
		VideoPrice480pPerSec:  &price,
		VideoPrice720pPerSec:  &price,
		VideoPrice1080pPerSec: &price,
		VideoPrice4kPerSec:    &price,
	})

	require.NotNil(t, out)
	require.True(t, out.AllowVideoGeneration)
	require.Equal(t, service.GroupVideoGenerationRouteNative, out.VideoGenerationRoute)
	require.Equal(t, &price, out.VideoPrice480pPerSec)
	require.Equal(t, &price, out.VideoPrice720pPerSec)
	require.Equal(t, &price, out.VideoPrice1080pPerSec)
	require.Equal(t, &price, out.VideoPrice4kPerSec)
}

func TestGroupFromServiceMapsPeakRateFields(t *testing.T) {
	out := GroupFromService(&service.Group{
		ID:                 3,
		Name:               "peak-group",
		Platform:           service.PlatformAnthropic,
		PeakRateEnabled:    true,
		PeakStart:          "09:30",
		PeakEnd:            "18:45",
		PeakRateMultiplier: 1.75,
	})

	require.NotNil(t, out)
	require.True(t, out.PeakRateEnabled)
	require.Equal(t, "09:30", out.PeakStart)
	require.Equal(t, "18:45", out.PeakEnd)
	require.Equal(t, 1.75, out.PeakRateMultiplier)
}
