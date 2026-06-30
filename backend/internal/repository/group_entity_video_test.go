package repository

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupEntityToServiceClearsStaleVideoConfigForNonGrokGroups(t *testing.T) {
	price := 0.04
	out := groupEntityToService(&dbent.Group{
		ID:                    1,
		Name:                  "openai-stale-video",
		Platform:              service.PlatformOpenAI,
		AllowVideoGeneration:  true,
		VideoGenerationRoute:  service.GroupVideoGenerationRouteNative,
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
