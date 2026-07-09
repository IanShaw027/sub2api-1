package service

import (
	"context"
	"strings"
	"time"
)

func (s *OpenAIGatewayService) recordOpenAIStickyScheduleDecision(ctx context.Context, req OpenAIAccountScheduleRequest, decision OpenAIAccountScheduleDecision) {
	if s == nil || s.usageLogRepo == nil {
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
		if decision.StickyEscapeTriggered {
			reason = "sticky_escape"
			if r := strings.TrimSpace(decision.StickyEscapeReason); r != "" {
				reason = reason + ":" + r
			}
		} else if strings.TrimSpace(decision.Layer) != "" {
			reason = "sticky_fallback:" + strings.TrimSpace(decision.Layer)
		}
	}

	input := &OpenAIStickyScheduleEventInput{
		CreatedAt:                 time.Now(),
		Platform:                  normalizeOpenAICompatiblePlatform(req.Platform),
		GroupID:                   req.GroupID,
		APIKeyID:                  req.APIKeyID,
		StickyAccountID:           decision.StickyAccountID,
		SelectedAccountID:         selectedAccountID,
		StickyOriginalUnavailable: stickyOriginalUnavailable,
		Reason:                    reason,
		StickySource:              req.StickySource,
	}

	go func() {
		writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 500*time.Millisecond)
		defer cancel()
		_ = s.usageLogRepo.RecordOpenAIStickyScheduleEvent(writeCtx, input)
	}()
}
