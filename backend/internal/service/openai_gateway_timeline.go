package service

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// OpenAIGatewayDebugTimelineEventInput describes one metadata-only OpenAI
// gateway timeline event. Request/response bodies are intentionally excluded.
type OpenAIGatewayDebugTimelineEventInput struct {
	Stage             string
	EndpointKind      string
	RequestStart      time.Time
	APIKey            *APIKey
	Account           *Account
	UserID            int64
	RequestedModel    string
	Stream            bool
	SwitchCount       int
	ForwardDurationMs int64
	Result            *ForwardResult
	OpenAIResult      *OpenAIForwardResult
	Err               error
	Fields            map[string]any
}

func (s *OpenAIGatewayService) EmitOpenAIGatewayDebugTimelineEvent(c *gin.Context, in OpenAIGatewayDebugTimelineEventInput) {
	if s == nil || !GatewayDebugTimelineEnabled(contextFromGin(c), s.settingService) {
		return
	}
	stage := strings.TrimSpace(in.Stage)
	if stage == "" {
		return
	}

	fields := make(map[string]any, len(in.Fields)+20)
	for k, v := range in.Fields {
		fields[k] = v
	}
	fields["component"] = "gateway_debug_timeline"
	fields["platform"] = PlatformOpenAI
	if endpointKind := strings.TrimSpace(in.EndpointKind); endpointKind != "" {
		fields["endpoint_kind"] = endpointKind
	}
	if in.UserID > 0 {
		fields["user_id"] = in.UserID
	}
	if model := strings.TrimSpace(in.RequestedModel); model != "" {
		fields["requested_model"] = model
	}
	fields["stream"] = in.Stream
	if !in.RequestStart.IsZero() {
		fields["request_start_unix_ms"] = in.RequestStart.UnixMilli()
		elapsed := time.Since(in.RequestStart).Milliseconds()
		if elapsed < 0 {
			elapsed = 0
		}
		fields["request_elapsed_ms"] = elapsed
	}
	if in.SwitchCount >= 0 {
		fields["switch_count_before_attempt"] = in.SwitchCount
	}
	if in.ForwardDurationMs > 0 {
		fields["forward_duration_ms"] = in.ForwardDurationMs
	}
	addOpenAIGatewayTimelineAPIKeyFields(fields, in.APIKey)
	addOpenAIGatewayTimelineAccountFields(fields, in.Account)
	addOpenAIGatewayTimelineResultFields(fields, in.Result)
	addOpenAIGatewayTimelineOpenAIResultFields(fields, in.OpenAIResult)
	if in.Err != nil {
		fields["error"] = truncateOpenAIWSLogValue(sanitizeUpstreamErrorMessage(in.Err.Error()), 512)
	}

	WriteGatewayDebugTimelineEvent(s.settingService, c, stage, fields)
}

func (s *OpenAIGatewayService) RecordOpenAIGatewayDebugTimelineBody(c *gin.Context, stage string, body []byte, contentType string, fields map[string]any) {
	if s == nil {
		return
	}
	RecordGatewayDebugTimelineBody(s.settingService, c, stage, body, contentType, fields)
}

func addOpenAIGatewayTimelineAPIKeyFields(fields map[string]any, apiKey *APIKey) {
	if apiKey == nil {
		return
	}
	fields["api_key_id"] = apiKey.ID
	if apiKey.GroupID != nil {
		fields["group_id"] = *apiKey.GroupID
	}
}

func addOpenAIGatewayTimelineAccountFields(fields map[string]any, account *Account) {
	if account == nil {
		return
	}
	fields["account_id"] = account.ID
	fields["account_name"] = strings.TrimSpace(account.Name)
	fields["account_type"] = strings.TrimSpace(string(account.Type))
	fields["account_platform"] = strings.TrimSpace(account.Platform)
	fields["account_concurrency"] = account.Concurrency
}

func addOpenAIGatewayTimelineResultFields(fields map[string]any, result *ForwardResult) {
	if result == nil {
		return
	}
	fields["upstream_request_id"] = strings.TrimSpace(result.RequestID)
	fields["upstream_model"] = strings.TrimSpace(result.UpstreamModel)
	fields["duration_ms"] = result.Duration.Milliseconds()
	if result.FirstTokenMs != nil {
		fields["first_forwardable_event_ms"] = *result.FirstTokenMs
	}
	fields["usage_input_tokens"] = result.Usage.InputTokens
	fields["usage_cache_read_tokens"] = result.Usage.CacheReadInputTokens
	fields["usage_output_tokens"] = result.Usage.OutputTokens
}

func addOpenAIGatewayTimelineOpenAIResultFields(fields map[string]any, result *OpenAIForwardResult) {
	if result == nil {
		return
	}
	fields["upstream_request_id"] = strings.TrimSpace(result.RequestID)
	fields["response_id"] = strings.TrimSpace(result.ResponseID)
	fields["upstream_model"] = strings.TrimSpace(result.UpstreamModel)
	fields["duration_ms"] = result.Duration.Milliseconds()
	if result.FirstTokenMs != nil {
		fields["first_forwardable_event_ms"] = *result.FirstTokenMs
	}
	fields["usage_input_tokens"] = result.Usage.InputTokens
	fields["usage_cache_read_tokens"] = result.Usage.CacheReadInputTokens
	fields["usage_cache_creation_tokens"] = result.Usage.CacheCreationInputTokens
	fields["usage_output_tokens"] = result.Usage.OutputTokens
	fields["openai_ws_mode"] = result.OpenAIWSMode
	if strings.TrimSpace(result.OpenAIWSProfile) != "" {
		fields["openai_ws_profile"] = strings.TrimSpace(result.OpenAIWSProfile)
	}
	fields["openai_ws_conn_reused"] = result.OpenAIWSConnReused
	if result.OpenAIWSMode {
		if strings.TrimSpace(result.OpenAIWSStoreMode) != "" {
			fields["store_mode"] = strings.TrimSpace(result.OpenAIWSStoreMode)
		}
		fields["delta_active"] = result.OpenAIWSDeltaActive
		fields["payload_bytes"] = result.OpenAIWSPayloadBytes
		fields["delta_items"] = result.OpenAIWSDeltaItems
		fields["delta_bytes"] = result.OpenAIWSDeltaBytes
		fields["full_items"] = result.OpenAIWSFullItems
		fields["full_bytes"] = result.OpenAIWSFullBytes
		fields["conn_pick_ms"] = result.OpenAIWSConnPickMs
		fields["queue_wait_ms"] = result.OpenAIWSQueueWaitMs
		fields["http_ingress_ws_one_shot"] = result.OpenAIWSOneShot
	}
	fields["client_disconnected"] = result.ClientDisconnected || result.ClientDisconnect
	if result.ImageCount > 0 {
		fields["image_count"] = result.ImageCount
		fields["image_size"] = strings.TrimSpace(result.ImageSize)
	}
}

func openAIWSDiagnosticStartTimelineFields(groupID, apiKeyID int64, v openAIWSDiagnosticStartLog) map[string]any {
	return map[string]any{
		"group_id":                              groupID,
		"api_key_id":                            apiKeyID,
		"openai_ws_mode":                        true,
		"openai_ws_profile":                     openAIWSProfileUsageString(v.ConnProfile),
		"openai_ws_conn_id":                     strings.TrimSpace(v.ConnID),
		"openai_ws_conn_reused":                 v.ConnReused,
		"openai_ws_transport":                   strings.TrimSpace(v.Transport),
		"openai_ws_payload_event":               strings.TrimSpace(v.PayloadEventType),
		"payload_bytes":                         v.PayloadBytes,
		"payload_keys":                          strings.TrimSpace(v.PayloadKeys),
		"input_summary":                         strings.TrimSpace(v.InputSummary),
		"previous_response_id_present":          strings.TrimSpace(v.PreviousResponseID) != "",
		"previous_response_id_kind":             strings.TrimSpace(v.PreviousResponseIDKind),
		"previous_response_id_source":           strings.TrimSpace(v.PreviousResponseIDSource),
		"original_previous_response_id_present": v.OriginalPreviousIDPresent,
		"preferred_conn_id":                     strings.TrimSpace(v.PreferredConnID),
		"store_mode":                            strings.TrimSpace(v.StoreMode),
		"store_enabled":                         v.StoreEnabled,
		"store_disabled":                        v.StoreDisabled,
		"sticky_account_hit":                    v.StickyAccountHit,
		"conn_affinity_hit":                     v.ConnAffinityHit,
		"fallback_reason":                       strings.TrimSpace(v.FallbackReason),
		"dropped_previous_response_id":          v.DroppedPreviousResponseID,
		"unsafe_tool_continuation":              v.UnsafeToolContinuation,
		"delta_active":                          v.DeltaActive,
		"delta_items":                           v.DeltaItems,
		"delta_bytes":                           v.DeltaBytes,
		"full_items":                            v.FullItems,
		"full_bytes":                            v.FullBytes,
		"conn_pick_ms":                          v.ConnPickMs,
		"queue_wait_ms":                         v.QueueWaitMs,
		"session_hash_present":                  strings.TrimSpace(v.SessionHash) != "",
		"header_session_id_present":             strings.TrimSpace(v.HeaderSessionID) != "",
		"header_conversation_id_present":        strings.TrimSpace(v.HeaderConversationID) != "",
		"session_id_source":                     strings.TrimSpace(v.SessionIDSource),
		"conversation_id_source":                strings.TrimSpace(v.ConversationIDSource),
		"has_turn_state":                        v.HasTurnState,
		"turn_state_len":                        v.TurnStateLen,
		"has_prompt_cache_key":                  v.HasPromptCacheKey,
		"has_tools":                             v.HasTools,
		"http_ingress_ws_one_shot":              v.HTTPIngressWSOneShot,
		"force_new_conn":                        v.ForceNewConn,
		"affinity_only_reuse":                   v.AffinityOnlyReuse,
		"store_disabled_conn_mode":              strings.TrimSpace(v.StoreDisabledConnMode),
		"proxy_enabled":                         v.ProxyEnabled,
	}
}

func openAIWSDiagnosticCompletedTimelineFields(groupID, apiKeyID int64, v openAIWSDiagnosticCompletedLog) map[string]any {
	return map[string]any{
		"group_id":                              groupID,
		"api_key_id":                            apiKeyID,
		"openai_ws_mode":                        true,
		"openai_ws_profile":                     openAIWSProfileUsageString(v.ConnProfile),
		"openai_ws_conn_id":                     strings.TrimSpace(v.ConnID),
		"openai_ws_conn_reused":                 v.ConnReused,
		"response_id":                           strings.TrimSpace(v.ResponseID),
		"store_mode":                            strings.TrimSpace(v.StoreMode),
		"store_enabled":                         v.StoreEnabled,
		"store_disabled":                        v.StoreDisabled,
		"previous_response_id_present":          v.HasPreviousResponseID,
		"previous_response_id_kind":             strings.TrimSpace(v.PreviousResponseIDKind),
		"previous_response_id_source":           strings.TrimSpace(v.PreviousResponseIDSource),
		"original_previous_response_id_present": v.OriginalPreviousIDPresent,
		"payload_bytes":                         v.PayloadBytes,
		"duration_ms":                           v.DurationMs,
		"first_forwardable_event_ms":            v.FirstTokenMs,
		"events":                                v.Events,
		"token_events":                          v.TokenEvents,
		"terminal_events":                       v.TerminalEvents,
		"buffered_events":                       v.BufferedEvents,
		"buffered_flushed":                      v.BufferedFlushed,
		"first_event":                           strings.TrimSpace(v.FirstEvent),
		"last_event":                            strings.TrimSpace(v.LastEvent),
		"wrote_downstream":                      v.WroteDownstream,
		"client_disconnected":                   v.ClientDisconnected,
		"http_ingress_ws_one_shot":              v.HTTPIngressWSOneShot,
		"usage_input_tokens":                    v.InputTokens,
		"usage_cache_read_tokens":               v.CacheReadTokens,
		"usage_cache_creation_tokens":           v.CacheCreationTokens,
		"usage_output_tokens":                   v.OutputTokens,
		"delta_active":                          v.DeltaActive,
		"delta_items":                           v.DeltaItems,
		"delta_bytes":                           v.DeltaBytes,
		"full_items":                            v.FullItems,
		"full_bytes":                            v.FullBytes,
	}
}

func contextFromGin(c *gin.Context) context.Context {
	if c != nil && c.Request != nil && c.Request.Context() != nil {
		return c.Request.Context()
	}
	return context.Background()
}
