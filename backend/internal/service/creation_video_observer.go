package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	ErrCreationVideoObservationFull    = errors.New("local video observation capacity is full")
	ErrCreationVideoObservationMissing = errors.New("local video observation reservation is unavailable")
)

const (
	CreationVideoObservationCapacity = 32
	CreationVideoReservationTTL      = 2 * time.Minute
	CreationVideoObservationTTL      = 30 * time.Minute
	CreationVideoObservationLease    = time.Minute
)

// The durable record contains only identity and queue-control metadata. Never
// add prompt, media payloads, access tokens, cookies, or request contexts here.
type CreationVideoObservation struct {
	ID             string `json:"id"`
	UserID         int64  `json:"user_id"`
	GroupID        int64  `json:"group_id"`
	APIKeyID       int64  `json:"api_key_id"`
	SubscriptionID int64  `json:"subscription_id,omitempty"`
	TaskID         string `json:"task_id,omitempty"`
	ExpiresAt      int64  `json:"expires_at"`
	LeaseToken     string `json:"lease_token,omitempty"`
}

type CreationVideoObservationQueue interface {
	Reserve(context.Context, *CreationVideoObservation, int, time.Duration) error
	Bind(context.Context, string, string, time.Duration) error
	CancelReservation(context.Context, string) error
	Claim(context.Context, string, time.Duration) (*CreationVideoObservation, error)
	Finish(context.Context, string, string, bool, time.Duration) error
}

type CreationVideoBillingLookup interface {
	Recorded(context.Context, string, int64) (bool, error)
}

type CreationVideoPollResult struct{ Terminal, Billable bool }
type CreationVideoPoll func(context.Context, *CreationVideoObservation, *APIKey, *UserSubscription) (CreationVideoPollResult, error)

type CreationVideoObserver struct {
	queue         CreationVideoObservationQueue
	keys          APIKeyRepository
	subscriptions UserSubscriptionRepository
	billing       CreationVideoBillingLookup
	gateway       *OpenAIGatewayService
	simple        bool
	start         sync.Once
	mu            sync.Mutex
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

func NewCreationVideoObserver(queue CreationVideoObservationQueue, keys APIKeyRepository, subscriptions UserSubscriptionRepository, billing CreationVideoBillingLookup, gateway *OpenAIGatewayService, cfg *config.Config) *CreationVideoObserver {
	return &CreationVideoObserver{queue: queue, keys: keys, subscriptions: subscriptions, billing: billing, gateway: gateway, simple: cfg != nil && cfg.RunMode == config.RunModeSimple}
}

func (s *CreationVideoObserver) Reserve(ctx context.Context, job *CreationVideoObservation) error {
	if s == nil || s.queue == nil {
		return ErrCreationVideoObservationMissing
	}
	if job.UserID <= 0 || job.GroupID <= 0 || job.APIKeyID <= 0 {
		return ErrCreationVideoObservationMissing
	}
	job.ID = uuid.NewString()
	return s.queue.Reserve(ctx, job, CreationVideoObservationCapacity, CreationVideoReservationTTL)
}

func (s *CreationVideoObserver) Bind(ctx context.Context, id, taskID string) error {
	if s == nil || s.queue == nil || taskID == "" || len(taskID) > 512 {
		return ErrCreationVideoObservationMissing
	}
	return s.queue.Bind(ctx, id, taskID, CreationVideoObservationTTL)
}

func (s *CreationVideoObserver) CancelReservation(ctx context.Context, id string) error {
	return s.queue.CancelReservation(ctx, id)
}

func (s *CreationVideoObserver) Start(poll CreationVideoPoll) {
	if s == nil || s.queue == nil || poll == nil {
		return
	}
	s.start.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.mu.Lock()
		s.cancel = cancel
		s.mu.Unlock()
		for range 4 {
			s.wg.Add(1)
			go func() { defer s.wg.Done(); s.run(ctx, poll) }()
		}
	})
}

func (s *CreationVideoObserver) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

func (s *CreationVideoObserver) run(ctx context.Context, poll CreationVideoPoll) {
	for ctx.Err() == nil {
		claimCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		job, err := s.queue.Claim(claimCtx, uuid.NewString(), CreationVideoObservationLease)
		cancel()
		if err != nil {
			logger.L().Warn("creation.local_video.claim_failed", zap.Error(err))
		} else if job != nil {
			s.process(ctx, job, poll)
			continue
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *CreationVideoObserver) process(ctx context.Context, job *CreationVideoObservation, poll CreationVideoPoll) {
	terminal := false
	defer func() {
		if value := recover(); value != nil {
			logger.L().Error("creation.local_video.observation_panicked", zap.String("observation_id", job.ID), zap.Any("panic", value))
		}
		finishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.queue.Finish(finishCtx, job.ID, job.LeaseToken, terminal, 5*time.Second); err != nil {
			logger.L().Warn("creation.local_video.finish_failed", zap.String("observation_id", job.ID), zap.Error(err))
		}
	}()
	pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if !s.simple {
		recorded, err := s.billing.Recorded(pollCtx, StableGrokVideoBillingRequestID(job.TaskID), job.APIKeyID)
		if err != nil {
			logger.L().Warn("creation.local_video.billing_lookup_failed", zap.String("observation_id", job.ID), zap.Error(err))
			return
		}
		if recorded {
			terminal = true
			return
		}
	}
	key, err := s.keys.GetByID(pollCtx, job.APIKeyID)
	if err != nil || key == nil || key.User == nil || key.Group == nil || key.Purpose != APIKeyPurposeCreation ||
		key.ID != job.APIKeyID || key.UserID != job.UserID || key.User.ID != job.UserID || key.Group.ID != job.GroupID {
		logger.L().Warn("creation.local_video.identity_restore_failed", zap.String("observation_id", job.ID), zap.Error(err))
		return
	}
	var subscription *UserSubscription
	if job.SubscriptionID > 0 {
		subscription, err = s.subscriptions.GetByID(pollCtx, job.SubscriptionID)
		if err != nil || subscription == nil || subscription.ID != job.SubscriptionID || subscription.UserID != job.UserID || subscription.GroupID != job.GroupID {
			logger.L().Warn("creation.local_video.subscription_restore_failed", zap.String("observation_id", job.ID), zap.Error(err))
			return
		}
	}
	result, err := poll(pollCtx, job, key, subscription)
	if err != nil {
		logger.L().Warn("creation.local_video.status_failed", zap.String("observation_id", job.ID), zap.Error(err))
	}
	if err != nil || !result.Terminal {
		return
	}
	if !result.Billable || s.simple {
		terminal = true
		return
	}
	// The native status handler can return done while a browser-owned billing
	// claim is still in flight. Only a committed ledger entry closes this job.
	recorded, err := s.billing.Recorded(pollCtx, StableGrokVideoBillingRequestID(job.TaskID), job.APIKeyID)
	if err != nil {
		return
	}
	if recorded {
		terminal = true
		return
	}
	// Recover a claim left behind by an interrupted process. The existing
	// stable request ID and database deduplication still prevent double charges.
	if s.gateway != nil {
		if err := s.gateway.ReleaseGrokVideoBilling(pollCtx, job.TaskID, job.UserID, job.APIKeyID); err != nil {
			logger.L().Warn("creation.local_video.billing_claim_recovery_failed", zap.String("observation_id", job.ID), zap.Error(err))
		}
	}
}
