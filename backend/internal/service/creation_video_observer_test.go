//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type observationTestQueue struct {
	CreationVideoObservationQueue
	finished         bool
	terminal         bool
	owner            string
	finishContextErr error
}

func (q *observationTestQueue) Finish(ctx context.Context, _, owner string, terminal bool, _ time.Duration) error {
	q.finished, q.terminal, q.owner, q.finishContextErr = true, terminal, owner, ctx.Err()
	return nil
}
func (q *observationTestQueue) Claim(ctx context.Context, _ string, _ time.Duration) (*CreationVideoObservation, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

type observationTestKeys struct {
	APIKeyRepository
	key *APIKey
}

func (r observationTestKeys) GetByID(context.Context, int64) (*APIKey, error) { return r.key, nil }

type observationTestSubscriptions struct {
	UserSubscriptionRepository
	sub *UserSubscription
}

func (r observationTestSubscriptions) GetByID(context.Context, int64) (*UserSubscription, error) {
	return r.sub, nil
}

type observationTestBilling struct {
	results  []bool
	requests []string
}

func (b *observationTestBilling) Recorded(_ context.Context, request string, _ int64) (bool, error) {
	b.requests = append(b.requests, request)
	if len(b.results) == 0 {
		return false, nil
	}
	value := b.results[0]
	b.results = b.results[1:]
	return value, nil
}

type observationTestGatewayCache struct {
	GatewayCache
	released []string
}

func (c *observationTestGatewayCache) ReleaseGrokVideoBilled(_ context.Context, key string) error {
	c.released = append(c.released, key)
	return nil
}

func observationServiceFixture() (*CreationVideoObserver, *CreationVideoObservation, *observationTestQueue, *observationTestBilling, *observationTestGatewayCache) {
	q := &observationTestQueue{}
	billing := &observationTestBilling{}
	cache := &observationTestGatewayCache{}
	key := &APIKey{ID: 9, UserID: 7, Purpose: APIKeyPurposeCreation, Group: &Group{ID: 3}, User: &User{ID: 7}}
	sub := &UserSubscription{ID: 42, UserID: 7, GroupID: 3}
	svc := NewCreationVideoObserver(q, observationTestKeys{key: key}, observationTestSubscriptions{sub: sub}, billing, &OpenAIGatewayService{cache: cache}, &config.Config{})
	job := &CreationVideoObservation{ID: "observation", UserID: 7, GroupID: 3, APIKeyID: 9, SubscriptionID: 42, TaskID: "task-1", LeaseToken: "lease-owner"}
	return svc, job, q, billing, cache
}

func TestCreationVideoObserverKeepsDoneJobUntilBillingCommits(t *testing.T) {
	svc, job, q, billing, cache := observationServiceFixture()
	billing.results = []bool{false, false}
	svc.process(context.Background(), job, func(_ context.Context, got *CreationVideoObservation, key *APIKey, sub *UserSubscription) (CreationVideoPollResult, error) {
		require.Equal(t, job.TaskID, got.TaskID)
		require.Equal(t, job.APIKeyID, key.ID)
		require.Equal(t, job.SubscriptionID, sub.ID)
		return CreationVideoPollResult{Terminal: true, Billable: true}, nil
	})
	require.True(t, q.finished)
	require.False(t, q.terminal)
	require.Equal(t, []string{"7:9:task-1"}, cache.released)
	require.Equal(t, []string{"grok-video:task-1", "grok-video:task-1"}, billing.requests)

	billing.results = []bool{false, true}
	svc.process(context.Background(), job, func(context.Context, *CreationVideoObservation, *APIKey, *UserSubscription) (CreationVideoPollResult, error) {
		return CreationVideoPollResult{Terminal: true, Billable: true}, nil
	})
	require.True(t, q.terminal)
	require.Equal(t, "lease-owner", q.owner)
	require.Len(t, cache.released, 1)
}

func TestCreationVideoObserverAlreadyBilledAndFailedJobsAreAcknowledged(t *testing.T) {
	svc, job, q, billing, _ := observationServiceFixture()
	billing.results = []bool{true}
	svc.process(context.Background(), job, func(context.Context, *CreationVideoObservation, *APIKey, *UserSubscription) (CreationVideoPollResult, error) {
		t.Fatal("committed tasks must not be polled again")
		return CreationVideoPollResult{}, nil
	})
	require.True(t, q.terminal)
	billing.results = []bool{false}
	svc.process(context.Background(), job, func(context.Context, *CreationVideoObservation, *APIKey, *UserSubscription) (CreationVideoPollResult, error) {
		return CreationVideoPollResult{Terminal: true}, nil
	})
	require.True(t, q.terminal)
}

func TestCreationVideoObserverNeverUsesAnotherOwnersRehydratedIdentity(t *testing.T) {
	svc, job, q, _, _ := observationServiceFixture()
	svc.keys = observationTestKeys{key: &APIKey{ID: 9, UserID: 8, Purpose: APIKeyPurposeCreation, Group: &Group{ID: 3}, User: &User{ID: 8}}}
	svc.process(context.Background(), job, func(context.Context, *CreationVideoObservation, *APIKey, *UserSubscription) (CreationVideoPollResult, error) {
		t.Fatal("owner mismatch must not reach the gateway")
		return CreationVideoPollResult{}, nil
	})
	require.False(t, q.terminal)
}

func TestCreationVideoObserverFailureAndPanicReleaseLeaseWithoutDroppingJob(t *testing.T) {
	for _, panics := range []bool{false, true} {
		svc, job, q, _, _ := observationServiceFixture()
		svc.process(context.Background(), job, func(context.Context, *CreationVideoObservation, *APIKey, *UserSubscription) (CreationVideoPollResult, error) {
			if panics {
				panic("upstream panic")
			}
			return CreationVideoPollResult{}, errors.New("upstream unavailable")
		})
		require.True(t, q.finished)
		require.False(t, q.terminal)
		require.NoError(t, q.finishContextErr)
	}
}

func TestCreationVideoObserverSimpleModeAndShutdown(t *testing.T) {
	svc, job, q, billing, _ := observationServiceFixture()
	svc.simple = true
	svc.process(context.Background(), job, func(context.Context, *CreationVideoObservation, *APIKey, *UserSubscription) (CreationVideoPollResult, error) {
		return CreationVideoPollResult{Terminal: true, Billable: true}, nil
	})
	require.True(t, q.terminal)
	require.Empty(t, billing.requests)
	svc.Start(func(context.Context, *CreationVideoObservation, *APIKey, *UserSubscription) (CreationVideoPollResult, error) {
		return CreationVideoPollResult{}, nil
	})
	stopped := make(chan struct{})
	go func() { svc.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("observer shutdown did not cancel outstanding claims")
	}
}
