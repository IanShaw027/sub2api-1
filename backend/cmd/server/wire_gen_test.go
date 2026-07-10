package main

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestProvideServiceBuildInfo(t *testing.T) {
	in := handler.BuildInfo{
		Version:   "v-test",
		BuildType: "release",
	}
	out := provideServiceBuildInfo(in)
	require.Equal(t, in.Version, out.Version)
	require.Equal(t, in.BuildType, out.BuildType)
}

func TestProvideCleanup_WithMinimalDependencies_NoPanic(t *testing.T) {
	cfg := &config.Config{}

	oauthSvc := service.NewOAuthService(nil, nil)
	openAIOAuthSvc := service.NewOpenAIOAuthService(nil, nil)
	geminiOAuthSvc := service.NewGeminiOAuthService(nil, nil, nil, cfg)
	antigravityOAuthSvc := service.NewAntigravityOAuthService(nil)
	kiroOAuthSvc := service.NewKiroOAuthService(nil, nil, nil, nil)

	tokenRefreshSvc := service.NewTokenRefreshService(
		nil,
		oauthSvc,
		openAIOAuthSvc,
		geminiOAuthSvc,
		antigravityOAuthSvc,
		nil,
		nil,
		cfg,
		nil,
	)
	accountExpirySvc := service.NewAccountExpiryService(nil, time.Second)
	proxyExpirySvc := service.NewProxyExpiryService(nil, time.Second)
	subscriptionExpirySvc := service.NewSubscriptionExpiryService(nil, time.Second)
	pricingSvc := service.NewPricingService(cfg, nil)
	emailQueueSvc := service.NewEmailQueueService(nil, 1)
	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	idempotencyCleanupSvc := service.NewIdempotencyCleanupService(nil, cfg)
	schedulerSnapshotSvc := service.NewSchedulerSnapshotService(nil, nil, nil, nil, cfg)
	opsSystemLogSinkSvc := service.NewOpsSystemLogSink(nil)
	batchImageWorker := service.NewBatchImageWorkerRuntime(
		service.NewBatchImageWorker(&blockingBatchImageQueue{}, batchImageProcessor{}, service.BatchImageWorkerOptions{
			DelayedPollInterval: time.Hour,
			RecoveryInterval:    time.Hour,
		}),
		&config.Config{BatchImage: config.BatchImageConfig{QueueEnabled: true}},
	)
	batchImageWorker.Start()
	require.True(t, batchImageWorker.Running())

	cleanup := provideCleanup(
		nil, // entClient
		nil, // redis
		&service.OpsMetricsCollector{},
		&service.OpsAggregationService{},
		&service.OpsAlertEvaluatorService{},
		&service.OpsCleanupService{},
		&service.OpsScheduledReportService{},
		opsSystemLogSinkSvc,
		schedulerSnapshotSvc,
		tokenRefreshSvc,
		accountExpirySvc,
		proxyExpirySvc,
		subscriptionExpirySvc,
		&service.UsageCleanupService{},
		idempotencyCleanupSvc,
		nil, // auditRetention
		nil, // usageUserDailyCostAggregator
		nil, // batchImageCleanup
		batchImageWorker,
		pricingSvc,
		emailQueueSvc,
		billingCacheSvc,
		&service.UsageRecordWorkerPool{},
		&service.SubscriptionService{},
		oauthSvc,
		openAIOAuthSvc,
		geminiOAuthSvc,
		antigravityOAuthSvc,
		kiroOAuthSvc,
		nil, // grokOAuth
		nil, // openAIGateway
		nil, // scheduledTestRunner
		nil, // backupSvc
		nil, // paymentOrderExpiry
		nil, // channelMonitorRunner
		nil, // quotaFlusher
		nil, // tlsFingerprintCaptureListener
	)

	require.NotPanics(t, func() {
		cleanup()
	})
	require.False(t, batchImageWorker.Running())
}

type batchImageProcessor struct{}

func (batchImageProcessor) Process(context.Context, string) (service.BatchImageProcessResult, error) {
	return service.BatchImageProcessResult{Terminal: true}, nil
}

type blockingBatchImageQueue struct{}

func (*blockingBatchImageQueue) Enqueue(context.Context, string) error { return nil }

func (*blockingBatchImageQueue) Reserve(ctx context.Context, _ time.Duration) (service.ReservedBatchImageJob, error) {
	<-ctx.Done()
	return service.ReservedBatchImageJob{}, ctx.Err()
}

func (*blockingBatchImageQueue) RequeueAfter(context.Context, string, time.Duration) error {
	return nil
}

func (*blockingBatchImageQueue) Ack(context.Context, string) error       { return nil }
func (*blockingBatchImageQueue) Heartbeat(context.Context, string) error { return nil }
func (*blockingBatchImageQueue) MoveDueDelayedToReady(context.Context, int) (int, error) {
	return 0, nil
}
func (*blockingBatchImageQueue) RecoverStaleActive(context.Context, time.Duration, int) (int, error) {
	return 0, nil
}
func (*blockingBatchImageQueue) TryAcquireJobLock(context.Context, string, time.Duration) (service.BatchImageJobLock, bool, error) {
	return nil, false, nil
}
