package service

import (
	"context"
	"strings"
	"time"
)

type OpenAIStickyScheduleEventInput struct {
	CreatedAt                 time.Time
	Platform                  string
	GroupID                   *int64
	APIKeyID                  int64
	StickyAccountID           int64
	SelectedAccountID         *int64
	StickyOriginalUnavailable bool
	Reason                    string
	StickySource              string
}

type openAIStickyScheduleEventRecorder interface {
	RecordOpenAIStickyScheduleEvent(ctx context.Context, input *OpenAIStickyScheduleEventInput) error
}

func (s *OpenAIGatewayService) recordOpenAIStickyScheduleDecision(ctx context.Context, req OpenAIAccountScheduleRequest, decision OpenAIAccountScheduleDecision) {
	if s == nil || s.usageLogRepo == nil {
		return
	}
	recorder, ok := s.usageLogRepo.(openAIStickyScheduleEventRecorder)
	if !ok || recorder == nil {
		return
	}
	if decision.StickyAccountID <= 0 {
		return
	}

	selectedAccountID := (*int64)(nil)
	if decision.SelectedAccountID > 0 {
		v := decision.SelectedAccountID
		selectedAccountID = &v
	}

	reason := "sticky_hit"
	stickyOriginalUnavailable := !decision.StickySessionHit
	if stickyOriginalUnavailable {
		reason = "sticky_not_selected"
		if strings.TrimSpace(decision.Layer) != "" {
			reason = "sticky_fallback:" + strings.TrimSpace(decision.Layer)
		}
	}

	input := &OpenAIStickyScheduleEventInput{
		CreatedAt:                 time.Now(),
		Platform:                  NormalizeOpenAICompatiblePlatform(req.Platform),
		GroupID:                   req.GroupID,
		StickyAccountID:           decision.StickyAccountID,
		SelectedAccountID:         selectedAccountID,
		StickyOriginalUnavailable: stickyOriginalUnavailable,
		Reason:                    reason,
	}

	go func() {
		writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 500*time.Millisecond)
		defer cancel()
		_ = recorder.RecordOpenAIStickyScheduleEvent(writeCtx, input)
	}()
}
