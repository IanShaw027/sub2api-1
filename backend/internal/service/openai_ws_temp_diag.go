package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	defaultOpenAIWSTempDiagLogsRPM = 60
	maxOpenAIWSTempDiagLogsRPM     = 600
	openAIWSTempDiagSlowTTFTMs     = 2000
)

type openAIWSTempDiagRateLimiter struct {
	mu         sync.Mutex
	window     int64
	emitted    int
	suppressed int64
}

var globalOpenAIWSTempDiagRateLimiter openAIWSTempDiagRateLimiter

func normalizeOpenAIWSTempDiagLogsRPM(rpm int) int {
	return boundedIntOrDefault(rpm, 1, maxOpenAIWSTempDiagLogsRPM, defaultOpenAIWSTempDiagLogsRPM)
}

func (l *openAIWSTempDiagRateLimiter) allow(now time.Time, rpm int) (bool, int64) {
	if l == nil {
		return false, 0
	}
	window := now.Unix() / 60
	l.mu.Lock()
	defer l.mu.Unlock()

	previousSuppressed := int64(0)
	if l.window != window {
		previousSuppressed = l.suppressed
		l.window = window
		l.emitted = 0
		l.suppressed = 0
	}
	if l.emitted >= normalizeOpenAIWSTempDiagLogsRPM(rpm) {
		l.suppressed++
		return false, 0
	}
	l.emitted++
	return true, previousSuppressed
}

func logOpenAIWSTemporaryAnomaly(event, format string, args ...any) {
	if !openAIWSTemporaryDiagnosticLogsEnabled() {
		return
	}
	rpm := openAIWSTemporaryDiagnosticLogsRPM()
	allowed, previousSuppressed := globalOpenAIWSTempDiagRateLimiter.allow(time.Now(), rpm)
	if !allowed {
		return
	}
	detail := fmt.Sprintf(format, args...)
	logger.LegacyPrintf(
		"service.openai_gateway",
		"[OpenAI WS Anomaly][temporary_diag=openai_ws_anomaly] event=%s rpm_limit=%d suppressed_previous_window=%d %s",
		event,
		rpm,
		previousSuppressed,
		detail,
	)
}

func logOpenAIWSSlowCompletion(v openAIWSDiagnosticCompletedLog) {
	if v.FirstTokenMs <= openAIWSTempDiagSlowTTFTMs {
		return
	}
	logOpenAIWSTemporaryAnomaly(
		"slow_first_token",
		"request_id=%s client_request_id=%s account_id=%d model=%s upstream_model=%s conn_profile=%s conn_id=%s conn_reused=%v conn_pick_ms=%d queue_wait_ms=%d conn_age_ms=%d conn_idle_ms=%d conn_lease_count=%d payload_bytes=%d write_sent_ms=%d first_event_ms=%d first_token_ms=%d first_token_after_first_event_ms=%d duration_ms=%d active_delta=%v delta_candidate=%v delta_fallback_reason=%s delta_prefix_match=%v delta_conn_match=%v delta_most_recent_match=%v delta_non_input_match=%v delta_raw_client_equiv=%v delta_break_boundary=%s delta_break_item_type=%s delta_store_fallback_reason=%s delta_conn_reanchor_blockers=%s delta_items=%d delta_bytes=%d full_items=%d full_bytes=%d pool_acquire_state=%s pool_total=%d pool_neutral=%d pool_session_bound=%d pool_idle_neutral=%d pool_idle_session_bound=%d pool_neutral_stock=%d pool_matching=%d pool_matching_idle=%d pool_leased=%d pool_waiters=%d pool_creating=%d pool_creating_neutral=%d pool_creating_session_bound=%d pool_pinned=%d pool_neutral_variants=%d pool_neutral_target=%d pool_effective_max=%d pool_prewarm_active=%v pool_prewarm_failures=%d input_tokens=%d cache_read_tokens=%d cache_creation_tokens=%d output_tokens=%d first_event=%s first_token_event=%s last_event=%s",
		normalizeOpenAIWSLogValue(v.RequestID),
		normalizeOpenAIWSLogValue(v.ClientRequestID),
		v.AccountID,
		normalizeOpenAIWSLogValue(v.Model),
		normalizeOpenAIWSLogValue(v.UpstreamModel),
		normalizeOpenAIWSLogValue(openAIWSProfileUsageString(v.ConnProfile)),
		truncateOpenAIWSLogValue(v.ConnID, openAIWSIDValueMaxLen),
		v.ConnReused,
		v.ConnPickMs,
		v.QueueWaitMs,
		v.ConnAgeMs,
		v.ConnIdleMs,
		v.ConnLeaseCount,
		v.PayloadBytes,
		v.WriteSentMs,
		v.FirstEventMs,
		v.FirstTokenMs,
		v.FirstTokenAfterFirstEventMs,
		v.DurationMs,
		v.DeltaActive,
		v.DeltaCandidate,
		normalizeOpenAIWSLogValueNoReplace(v.DeltaFallbackReason),
		v.DeltaPrefixMatch,
		v.DeltaConnMatch,
		v.DeltaMostRecentMatch,
		v.DeltaNonInputMatch,
		v.DeltaRawClientEquiv,
		normalizeOpenAIWSLogValueNoReplace(v.DeltaBreakBoundary),
		normalizeOpenAIWSLogValueNoReplace(v.DeltaBreakItemType),
		normalizeOpenAIWSLogValueNoReplace(v.DeltaStoreFallbackReason),
		normalizeOpenAIWSLogValueNoReplace(v.DeltaConnReanchorBlockers),
		v.DeltaItems,
		v.DeltaBytes,
		v.FullItems,
		v.FullBytes,
		normalizeOpenAIWSLogValueNoReplace(v.PoolAcquireState),
		v.PoolSnapshot.TotalConns,
		v.PoolSnapshot.NeutralConns,
		v.PoolSnapshot.SessionBoundConns,
		v.PoolSnapshot.IdleNeutralConns,
		v.PoolSnapshot.IdleSessionBoundConns,
		v.PoolSnapshot.NeutralStockConns,
		v.PoolSnapshot.MatchingConns,
		v.PoolSnapshot.MatchingIdleConns,
		v.PoolSnapshot.LeasedConns,
		v.PoolSnapshot.Waiters,
		v.PoolSnapshot.Creating,
		v.PoolSnapshot.CreatingNeutral,
		v.PoolSnapshot.CreatingSessionBound,
		v.PoolSnapshot.PinnedConns,
		v.PoolSnapshot.NeutralVariants,
		v.PoolSnapshot.NeutralPrewarmTarget,
		v.PoolSnapshot.EffectiveMaxConns,
		v.PoolSnapshot.PrewarmActive,
		v.PoolSnapshot.PrewarmFailures,
		v.InputTokens,
		v.CacheReadTokens,
		v.CacheCreationTokens,
		v.OutputTokens,
		normalizeOpenAIWSLogValue(v.FirstEvent),
		normalizeOpenAIWSLogValue(v.FirstTokenEvent),
		normalizeOpenAIWSLogValue(v.LastEvent),
	)
}

func resetOpenAIWSTempDiagRateLimiterForTest() {
	globalOpenAIWSTempDiagRateLimiter.mu.Lock()
	globalOpenAIWSTempDiagRateLimiter.window = 0
	globalOpenAIWSTempDiagRateLimiter.emitted = 0
	globalOpenAIWSTempDiagRateLimiter.suppressed = 0
	globalOpenAIWSTempDiagRateLimiter.mu.Unlock()
}
