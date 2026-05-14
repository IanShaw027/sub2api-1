package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayServiceRecordUsage_AuditZeroUsageWithEndpointsStillWritesUsageLog(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	quotaSvc := &openAIRecordUsageAPIKeyQuotaStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, subRepo, nil)

	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_zero_usage_endpoint_telemetry",
			Usage:     OpenAIUsage{},
			Model:     "gpt-5.1",
			Duration:  1500 * time.Millisecond,
		},
		APIKey:           &APIKey{ID: 1100, Quota: 100, Group: &Group{RateMultiplier: 1}},
		User:             &User{ID: 2100},
		Account:          &Account{ID: 3100, Type: AccountTypeAPIKey},
		InboundEndpoint:  "/v1/responses",
		UpstreamEndpoint: "/v1/responses/compact",
		APIKeyService:    quotaSvc,
	})

	require.NoError(t, err)
	require.Equal(t, 1, billingRepo.calls)
	require.Equal(t, 1, usageRepo.calls)
	require.NotNil(t, usageRepo.lastLog)
	require.Equal(t, "resp_zero_usage_endpoint_telemetry", usageRepo.lastLog.RequestID)
	require.Equal(t, RequestTypeSync, usageRepo.lastLog.RequestType)
	require.NotNil(t, usageRepo.lastLog.InboundEndpoint)
	require.Equal(t, "/v1/responses", *usageRepo.lastLog.InboundEndpoint)
	require.NotNil(t, usageRepo.lastLog.UpstreamEndpoint)
	require.Equal(t, "/v1/responses/compact", *usageRepo.lastLog.UpstreamEndpoint)
	require.NotNil(t, usageRepo.lastLog.DurationMs)
	require.Equal(t, int((1500 * time.Millisecond).Milliseconds()), *usageRepo.lastLog.DurationMs)
	require.Zero(t, usageRepo.lastLog.TotalCost)
	require.Zero(t, usageRepo.lastLog.ActualCost)
}

func TestOpenAIGatewayServiceRecordUsage_AuditZeroUsageWithEndpointsStillPersistsInSimpleMode(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, subRepo, nil)
	svc.cfg.RunMode = config.RunModeSimple

	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_zero_usage_simple_mode_endpoint",
			Usage:     OpenAIUsage{},
			Model:     "gpt-5.1",
			Duration:  2 * time.Second,
		},
		APIKey:           &APIKey{ID: 1101, Group: &Group{RateMultiplier: 1}},
		User:             &User{ID: 2101},
		Account:          &Account{ID: 3101},
		InboundEndpoint:  "/v1/responses",
		UpstreamEndpoint: "/v1/responses/compact",
	})

	require.NoError(t, err)
	require.Equal(t, 1, usageRepo.calls)
	require.Equal(t, 0, userRepo.deductCalls)
	require.Equal(t, 0, subRepo.incrementCalls)
	require.NotNil(t, usageRepo.lastLog)
	require.Equal(t, RequestTypeSync, usageRepo.lastLog.RequestType)
	require.NotNil(t, usageRepo.lastLog.InboundEndpoint)
	require.Equal(t, "/v1/responses", *usageRepo.lastLog.InboundEndpoint)
	require.NotNil(t, usageRepo.lastLog.UpstreamEndpoint)
	require.Equal(t, "/v1/responses/compact", *usageRepo.lastLog.UpstreamEndpoint)
}

func TestOpenAIGatewayServiceRecordUsage_AuditZeroUsageImageEndpointsStillWriteUsageLog(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, subRepo, nil)

	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_zero_usage_image_endpoint",
			Usage:     OpenAIUsage{},
			Model:     "gpt-5.1",
			Duration:  time.Second,
		},
		APIKey:           &APIKey{ID: 1102, Quota: 100, Group: &Group{RateMultiplier: 1}},
		User:             &User{ID: 2102},
		Account:          &Account{ID: 3102, Type: AccountTypeAPIKey},
		InboundEndpoint:  "/v1/images/generations",
		UpstreamEndpoint: "/v1/images/generations",
	})

	require.NoError(t, err)
	require.Equal(t, 1, billingRepo.calls)
	require.Equal(t, 1, usageRepo.calls)
	require.NotNil(t, usageRepo.lastLog)
	require.Equal(t, RequestTypeImage, usageRepo.lastLog.RequestType)
	require.NotNil(t, usageRepo.lastLog.InboundEndpoint)
	require.Equal(t, "/v1/images/generations", *usageRepo.lastLog.InboundEndpoint)
	require.NotNil(t, usageRepo.lastLog.UpstreamEndpoint)
	require.Equal(t, "/v1/images/generations", *usageRepo.lastLog.UpstreamEndpoint)
	require.Zero(t, usageRepo.lastLog.TotalCost)
	require.Zero(t, usageRepo.lastLog.ActualCost)
}

func TestOpenAIGatewayServiceRecordUsage_AuditZeroUsageImages2APIEndpointsUseWebBridgeType(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
	userRepo := &openAIRecordUsageUserRepoStub{}
	subRepo := &openAIRecordUsageSubRepoStub{}
	quotaSvc := &openAIRecordUsageAPIKeyQuotaStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, subRepo, nil)

	err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID: "resp_zero_usage_image_web_bridge",
			Usage:     OpenAIUsage{},
			Model:     "gpt-5.1",
			Duration:  time.Second,
		},
		APIKey:           &APIKey{ID: 1103, Quota: 100, Group: &Group{RateMultiplier: 1}},
		User:             &User{ID: 2103},
		Account:          &Account{ID: 3103, Type: AccountTypeAPIKey},
		InboundEndpoint:  "/v1/images2api/generations",
		UpstreamEndpoint: "/v1/images2api/generations",
		APIKeyService:    quotaSvc,
	})

	require.NoError(t, err)
	require.Equal(t, 1, billingRepo.calls)
	require.Equal(t, 1, usageRepo.calls)
	require.NotNil(t, usageRepo.lastLog)
	require.Equal(t, RequestTypeImageWebBridge, usageRepo.lastLog.RequestType)
	require.NotNil(t, usageRepo.lastLog.InboundEndpoint)
	require.Equal(t, "/v1/images2api/generations", *usageRepo.lastLog.InboundEndpoint)
	require.NotNil(t, usageRepo.lastLog.UpstreamEndpoint)
	require.Equal(t, "/v1/images2api/generations", *usageRepo.lastLog.UpstreamEndpoint)
	require.Zero(t, usageRepo.lastLog.TotalCost)
	require.Zero(t, usageRepo.lastLog.ActualCost)
}
