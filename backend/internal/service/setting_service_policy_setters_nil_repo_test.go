//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSettingServicePolicySetters_NilRepoReturnError(t *testing.T) {
	svc := NewSettingService(nil, nil)

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "overload cooldown",
			run: func() error {
				return svc.SetOverloadCooldownSettings(context.Background(), DefaultOverloadCooldownSettings())
			},
		},
		{
			name: "429 cooldown",
			run: func() error {
				return svc.SetRateLimit429CooldownSettings(context.Background(), DefaultRateLimit429CooldownSettings())
			},
		},
		{
			name: "rectifier",
			run: func() error {
				return svc.SetRectifierSettings(context.Background(), DefaultRectifierSettings())
			},
		},
		{
			name: "beta policy",
			run: func() error {
				return svc.SetBetaPolicySettings(context.Background(), DefaultBetaPolicySettings())
			},
		},
		{
			name: "openai fast policy",
			run: func() error {
				return svc.SetOpenAIFastPolicySettings(context.Background(), DefaultOpenAIFastPolicySettings())
			},
		},
		{
			name: "stream timeout",
			run: func() error {
				return svc.SetStreamTimeoutSettings(context.Background(), DefaultStreamTimeoutSettings())
			},
		},
		{
			name: "temp unsched threshold",
			run: func() error {
				return svc.SetTempUnschedThresholdSettings(context.Background(), DefaultTempUnschedThresholdSettings())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			require.Error(t, err)
			require.Contains(t, err.Error(), "setting repository unavailable")
		})
	}
}
