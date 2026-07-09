package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type captureStickyScheduleUsageRepo struct {
	UsageLogRepository
	ch chan *OpenAIStickyScheduleEventInput
}

func (r *captureStickyScheduleUsageRepo) RecordOpenAIStickyScheduleEvent(_ context.Context, input *OpenAIStickyScheduleEventInput) error {
	r.ch <- input
	return nil
}

func TestRecordOpenAIStickyScheduleDecisionCountsStickyEscapeAsUnavailable(t *testing.T) {
	repo := &captureStickyScheduleUsageRepo{ch: make(chan *OpenAIStickyScheduleEventInput, 1)}
	svc := &OpenAIGatewayService{usageLogRepo: repo}
	groupID := int64(12)

	svc.recordOpenAIStickyScheduleDecision(context.Background(), OpenAIAccountScheduleRequest{
		GroupID:      &groupID,
		APIKeyID:     34,
		Platform:     PlatformOpenAI,
		StickySource: "session",
	}, OpenAIAccountScheduleDecision{
		StickyAccountID:       56,
		SelectedAccountID:     78,
		StickyEscapeTriggered: true,
		StickyEscapeReason:    "ttft",
		StickySessionHit:      false,
	})

	select {
	case input := <-repo.ch:
		require.Equal(t, PlatformOpenAI, input.Platform)
		require.NotNil(t, input.GroupID)
		require.Equal(t, groupID, *input.GroupID)
		require.Equal(t, int64(34), input.APIKeyID)
		require.Equal(t, int64(56), input.StickyAccountID)
		require.NotNil(t, input.SelectedAccountID)
		require.Equal(t, int64(78), *input.SelectedAccountID)
		require.True(t, input.StickyOriginalUnavailable)
		require.Equal(t, "sticky_escape:ttft", input.Reason)
		require.Equal(t, "session", input.StickySource)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sticky schedule event")
	}
}
