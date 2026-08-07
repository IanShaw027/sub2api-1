package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestShouldRecordGrokMediaUsage(t *testing.T) {
	imageResult := &service.OpenAIForwardResult{ImageCount: 1}
	videoResult := &service.OpenAIForwardResult{VideoCount: 1, VideoSeconds: 8}
	emptyImageResult := &service.OpenAIForwardResult{ImageCount: 0}
	tests := []struct {
		name     string
		endpoint service.GrokMediaEndpoint
		model    string
		result   *service.OpenAIForwardResult
		want     bool
	}{
		{
			name:     "image generation records usage",
			endpoint: service.GrokMediaEndpointImagesGenerations,
			model:    "grok-imagine",
			result:   imageResult,
			want:     true,
		},
		{
			name:     "image edit records usage",
			endpoint: service.GrokMediaEndpointImagesEdits,
			model:    "grok-imagine-edit",
			result:   imageResult,
			want:     true,
		},
		{
			name:     "video generation records usage",
			endpoint: service.GrokMediaEndpointVideosGenerations,
			model:    "grok-imagine-video-1.5",
			result:   videoResult,
			want:     true,
		},
		{
			name:     "video status skips empty model usage",
			endpoint: service.GrokMediaEndpointVideoStatus,
			model:    "",
			result:   videoResult,
			want:     false,
		},
		{
			name:     "video status with model still skips usage",
			endpoint: service.GrokMediaEndpointVideoStatus,
			model:    "grok-imagine-video",
			result:   videoResult,
			want:     false,
		},
		{
			name:     "generation skips usage without model",
			endpoint: service.GrokMediaEndpointImagesGenerations,
			model:    " ",
			result:   imageResult,
			want:     false,
		},
		{
			name:     "generation skips usage without billable image output",
			endpoint: service.GrokMediaEndpointImagesGenerations,
			model:    "grok-imagine",
			result:   emptyImageResult,
			want:     false,
		},
		{
			name:     "generation skips usage when result is nil",
			endpoint: service.GrokMediaEndpointImagesGenerations,
			model:    "grok-imagine",
			result:   nil,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, shouldRecordGrokMediaUsage(tt.endpoint, tt.model, tt.result))
		})
	}
}
