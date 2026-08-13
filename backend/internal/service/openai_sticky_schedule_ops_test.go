package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type captureStickyScheduleUsageRepo struct {
	UsageLogRepository
	mu    sync.Mutex
	got   *OpenAIStickyScheduleEventInput
	wrote chan struct{}
}

func (r *captureStickyScheduleUsageRepo) RecordOpenAIStickyScheduleEvent(_ context.Context, input *OpenAIStickyScheduleEventInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := *input
	r.got = &cloned
	if r.wrote != nil {
		select {
		case <-r.wrote:
		default:
			close(r.wrote)
		}
	}
	return nil
}

func TestRecordOpenAIStickyScheduleDecisionUnavailable(t *testing.T) {
	repo := &captureStickyScheduleUsageRepo{wrote: make(chan struct{})}
	svc := &OpenAIGatewayService{usageLogRepo: repo}
	groupID := int64(9)

	svc.recordOpenAIStickyScheduleDecision(context.Background(), OpenAIAccountScheduleRequest{
		GroupID:  &groupID,
		Platform: PlatformOpenAI,
	}, OpenAIAccountScheduleDecision{
		StickyAccountID:   12,
		SelectedAccountID: 34,
		StickySessionHit:  false,
		Layer:             openAIAccountScheduleLayerLoadBalance,
	})

	select {
	case <-repo.wrote:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for sticky schedule event")
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.NotNil(t, repo.got)
	require.Equal(t, int64(12), repo.got.StickyAccountID)
	require.NotNil(t, repo.got.SelectedAccountID)
	require.Equal(t, int64(34), *repo.got.SelectedAccountID)
	require.True(t, repo.got.StickyOriginalUnavailable)
	require.Equal(t, "sticky_fallback:"+openAIAccountScheduleLayerLoadBalance, repo.got.Reason)
}
