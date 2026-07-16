package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type grokHTTPActiveDeltaResult struct {
	body         []byte
	log          openAIWSDeltaShadowLog
	sessionHash  string
	sessionOwner bool
	applied      bool
}

func (s *OpenAIGatewayService) grokHTTPActiveDeltaEnabled() bool {
	return s != nil && s.cfg != nil && s.cfg.Gateway.Grok.HTTPActiveDeltaEnabled
}

func (s *OpenAIGatewayService) grokHTTPActiveDeltaRequireStoreOnCreate() bool {
	return s != nil && s.cfg != nil && s.cfg.Gateway.Grok.HTTPActiveDeltaRequireStoreOnCreate
}

// grokActiveDeltaIncompatibleWithPrevious lists top-level Responses fields that xAI
// rejects when previous_response_id is set (e.g. "instructions and previous_response_id
// together"). Only applied on the delta path; full replay keeps the original body.
var grokActiveDeltaIncompatibleWithPrevious = []string{
	"instructions",
}

const grokClientExplicitStoreFalseContextKey = "grok_client_explicit_store_false"

func markGrokClientStorePreference(c *gin.Context, body []byte) {
	if c == nil {
		return
	}
	store := gjson.GetBytes(body, "store")
	c.Set(grokClientExplicitStoreFalseContextKey, store.Exists() && store.Type == gjson.False)
}

func grokClientExplicitlyDisablesStore(c *gin.Context) bool {
	if c == nil {
		return false
	}
	value, exists := c.Get(grokClientExplicitStoreFalseContextKey)
	disabled, ok := value.(bool)
	return exists && ok && disabled
}

// sanitizeGrokActiveDeltaUpstreamBody strips fields that cannot coexist with
// previous_response_id on xAI. Returns the body and the list of removed keys.
func sanitizeGrokActiveDeltaUpstreamBody(body []byte) ([]byte, []string, error) {
	if len(body) == 0 {
		return body, nil, nil
	}
	out := body
	stripped := make([]string, 0, len(grokActiveDeltaIncompatibleWithPrevious))
	for _, field := range grokActiveDeltaIncompatibleWithPrevious {
		if !gjson.GetBytes(out, field).Exists() {
			continue
		}
		next, err := sjson.DeleteBytes(out, field)
		if err != nil {
			return nil, nil, fmt.Errorf("strip grok active-delta field %s: %w", field, err)
		}
		out = next
		stripped = append(stripped, field)
	}
	return out, stripped, nil
}

// isGrokActiveDeltaInstructionsConflict reports the xAI validation error that
// rejects instructions together with previous_response_id.
func isGrokActiveDeltaInstructionsConflict(upstreamMsg string) bool {
	msg := strings.ToLower(strings.TrimSpace(upstreamMsg))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "previous_response_id") &&
		strings.Contains(msg, "instructions") &&
		(strings.Contains(msg, "not supported") ||
			strings.Contains(msg, "unsupported") ||
			strings.Contains(msg, "together"))
}

// resolveGrokActiveDeltaSessionHash derives the session key used for active-delta
// context. It is homologous with Grok cache identity (P4): empty identity means
// no bind and no incremental mutation.
func resolveGrokActiveDeltaSessionHash(c *gin.Context, cacheIdentity string) string {
	identity := strings.TrimSpace(cacheIdentity)
	if identity == "" {
		return ""
	}
	apiKeyID := getAPIKeyIDFromContext(c)
	current, _ := deriveOpenAIRequestScopedSessionHashes(c, "grok-active-delta:v1:"+identity)
	if current != "" {
		return current
	}
	// Fallback when gin context lacks api key isolation seed helpers.
	if apiKeyID > 0 {
		current, _ = deriveOpenAISessionHashes(fmt.Sprintf("api_key:%d:grok-active-delta:v1:%s", apiKeyID, identity))
		return current
	}
	current, _ = deriveOpenAISessionHashes("grok-active-delta:v1:" + identity)
	return current
}

// grokHTTPActiveDeltaSkipResult fills a non-applied result with a classified
// reason (expected_full_* for normal full paths, raw reason for regressions).
// body is always the original canonical payload for the full path.
func grokHTTPActiveDeltaSkipResult(
	canonicalBody []byte,
	accountID, groupID, apiKeyID int64,
	sessionHash, requestID, reason string,
	cached openAIWSSessionContextValue,
	cachedFound bool,
	clientPrevious string,
	sessionOwner bool,
) grokHTTPActiveDeltaResult {
	classified := openAIWSClassifyExpectedFullDeltaReason(
		reason,
		cachedFound,
		clientPrevious,
		"grok_http_active_delta",
	)
	if classified == "" {
		classified = reason
	}
	// Grok-specific early gates not covered by the OpenAI classifier.
	switch reason {
	case "disabled", "not_oauth_or_not_grok", "no_explicit_session", "compact_path", "tool_continuation", "missing_session_hash", "state_store_unavailable":
		classified = "expected_full_" + reason
	case "same_session_in_flight":
		classified = "expected_full_same_session_in_flight"
	case "transport_context_mismatch":
		// Keep as regression signal: HTTP path saw a non-http session context.
		classified = reason
	case "previous_response_mismatch":
		if !cachedFound {
			classified = "expected_full_no_session_context"
		}
	case "account_mismatch":
		// True sticky regression — keep raw reason.
		classified = reason
	}
	return grokHTTPActiveDeltaResult{
		body:         canonicalBody,
		sessionHash:  sessionHash,
		sessionOwner: sessionOwner,
		log: openAIWSDeltaShadowLog{
			GroupID:              groupID,
			APIKeyID:             apiKeyID,
			SessionHash:          sessionHash,
			RequestID:            requestID,
			AccountID:            accountID,
			CachedFound:          cachedFound,
			CachedAccountID:      cached.accountID,
			CachedConnID:         cached.connID,
			CachedLastResponseID: cached.lastResponseID,
			Candidate:            false,
			FallbackReason:       classified,
			StoreFallbackReason:  "grok_http_active_delta",
		},
	}
}

func (s *OpenAIGatewayService) buildGrokHTTPActiveDeltaPayload(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	canonicalBody []byte,
	cacheIdentity string,
	clientPreviousResponseID string,
) (result grokHTTPActiveDeltaResult, err error) {
	result = grokHTTPActiveDeltaResult{body: canonicalBody}
	requestID, _ := openAIWSRequestLogIDs(c)
	groupID := getOpenAIGroupIDFromContext(c)
	apiKeyID := getAPIKeyIDFromContext(c)
	clientPreviousResponseID = strings.TrimSpace(clientPreviousResponseID)
	if clientPreviousResponseID == "" {
		clientPreviousResponseID = strings.TrimSpace(gjsonGetBytesString(canonicalBody, "previous_response_id"))
	}

	// --- Early expected-full gates: no in-flight lock, no prefix hashing. ---
	if s == nil || c == nil || account == nil || !account.IsGrok() || account.Type != AccountTypeOAuth {
		return grokHTTPActiveDeltaSkipResult(canonicalBody, 0, groupID, apiKeyID, "", requestID, "not_oauth_or_not_grok", openAIWSSessionContextValue{}, false, clientPreviousResponseID, false), nil
	}
	if !s.grokHTTPActiveDeltaEnabled() {
		return grokHTTPActiveDeltaSkipResult(canonicalBody, account.ID, groupID, apiKeyID, "", requestID, "disabled", openAIWSSessionContextValue{}, false, clientPreviousResponseID, false), nil
	}
	if !hasExplicitGrokSessionIdentity(c) {
		out := grokHTTPActiveDeltaSkipResult(canonicalBody, account.ID, groupID, apiKeyID, "", requestID, "no_explicit_session", openAIWSSessionContextValue{}, false, clientPreviousResponseID, false)
		logOpenAIWSDeltaShadow(out.log)
		return out, nil
	}
	if isOpenAIResponsesCompactPath(c) {
		out := grokHTTPActiveDeltaSkipResult(canonicalBody, account.ID, groupID, apiKeyID, "", requestID, "compact_path", openAIWSSessionContextValue{}, false, clientPreviousResponseID, false)
		logOpenAIWSDeltaShadow(out.log)
		return out, nil
	}
	if HasToolContinuationOutputInRawPayload(canonicalBody) {
		out := grokHTTPActiveDeltaSkipResult(canonicalBody, account.ID, groupID, apiKeyID, "", requestID, "tool_continuation", openAIWSSessionContextValue{}, false, clientPreviousResponseID, false)
		logOpenAIWSDeltaShadow(out.log)
		return out, nil
	}

	sessionHash := resolveGrokActiveDeltaSessionHash(c, cacheIdentity)
	if sessionHash == "" {
		out := grokHTTPActiveDeltaSkipResult(canonicalBody, account.ID, groupID, apiKeyID, "", requestID, "missing_session_hash", openAIWSSessionContextValue{}, false, clientPreviousResponseID, false)
		logOpenAIWSDeltaShadow(out.log)
		return out, nil
	}
	store := s.getOpenAIWSStateStore()
	if store == nil {
		out := grokHTTPActiveDeltaSkipResult(canonicalBody, account.ID, groupID, apiKeyID, sessionHash, requestID, "state_store_unavailable", openAIWSSessionContextValue{}, false, clientPreviousResponseID, false)
		logOpenAIWSDeltaShadow(out.log)
		return out, nil
	}

	if !store.TrySessionInFlight(groupID, apiKeyID, sessionHash) {
		out := grokHTTPActiveDeltaSkipResult(canonicalBody, account.ID, groupID, apiKeyID, sessionHash, requestID, "same_session_in_flight", openAIWSSessionContextValue{}, false, clientPreviousResponseID, false)
		logOpenAIWSDeltaShadow(out.log)
		return out, nil
	}
	result.sessionOwner = true
	result.sessionHash = sessionHash
	defer func() {
		if result.sessionOwner && !result.applied {
			store.EndSessionInFlight(groupID, apiKeyID, sessionHash)
			result.sessionOwner = false
		}
	}()

	cached, found := store.GetSessionContext(groupID, apiKeyID, sessionHash)
	if found && strings.TrimSpace(cached.connID) != "http" {
		out := grokHTTPActiveDeltaSkipResult(canonicalBody, account.ID, groupID, apiKeyID, sessionHash, requestID, "transport_context_mismatch", cached, found, clientPreviousResponseID, true)
		logOpenAIWSDeltaShadow(out.log)
		result = out
		return result, nil
	}

	if clientPreviousResponseID != "" {
		if boundAccountID, accountErr := store.GetResponseAccount(ctx, groupID, apiKeyID, clientPreviousResponseID); accountErr == nil && boundAccountID > 0 && boundAccountID != account.ID {
			out := grokHTTPActiveDeltaSkipResult(canonicalBody, account.ID, groupID, apiKeyID, sessionHash, requestID, "account_mismatch", cached, found, clientPreviousResponseID, true)
			out.log.AccountMismatchReason = "response_account_mismatch"
			logOpenAIWSDeltaShadow(out.log)
			result = out
			return result, nil
		}
	}
	// Only enforce previous==cached.last when we actually have session context.
	// Without context, fall through to evaluate → expected_full_no_session_context
	// instead of a misleading previous_response_mismatch.
	if found && clientPreviousResponseID != "" && clientPreviousResponseID != strings.TrimSpace(cached.lastResponseID) {
		out := grokHTTPActiveDeltaSkipResult(canonicalBody, account.ID, groupID, apiKeyID, sessionHash, requestID, "previous_response_mismatch", cached, found, clientPreviousResponseID, true)
		logOpenAIWSDeltaShadow(out.log)
		result = out
		return result, nil
	}

	stickyAccountHit := !found || cached.accountID == account.ID
	shadowInput := openAIWSDeltaShadowInput{
		GroupID:               groupID,
		APIKeyID:              apiKeyID,
		SessionHash:           sessionHash,
		RequestID:             requestID,
		AccountID:             account.ID,
		CurrentPayload:        canonicalBody,
		HasFunctionCallOutput: HasToolContinuationOutputInRawPayload(canonicalBody),
		AllowConnReanchor:     true,
		AllowHTTPContext:      true,
		StickyAccountID:       account.ID,
		StickyAccountHit:      stickyAccountHit,
		StickyAccountMismatch: found && cached.accountID != account.ID,
		ConnAffinityHit:       true,
		StoreFallbackReason:   "grok_http_active_delta",
		Cached:                cached,
		CachedFound:           found,
	}
	deltaPayload, deltaLog, applied, err := buildOpenAIWSActiveDeltaPayload(shadowInput)
	if !applied && err == nil {
		// Align with OpenAI WS: normal full outcomes get expected_full_* tags.
		deltaLog.FallbackReason = openAIWSClassifyExpectedFullDeltaReason(
			deltaLog.FallbackReason,
			found,
			clientPreviousResponseID,
			"grok_http_active_delta",
		)
	}
	result.log = deltaLog
	logOpenAIWSDeltaShadow(deltaLog)
	if err != nil || !applied {
		return result, err
	}
	body, err := marshalOpenAIResponsesRequestBodyOrdered(deltaPayload)
	if err != nil {
		return result, err
	}
	// xAI rejects certain top-level fields together with previous_response_id.
	// Strip them after delta construction so full-path semantics stay intact.
	body, strippedFields, sanitizeErr := sanitizeGrokActiveDeltaUpstreamBody(body)
	if sanitizeErr != nil {
		return result, sanitizeErr
	}
	if len(strippedFields) > 0 {
		slog.Info("grok_http_active_delta_sanitized",
			"account_id", account.ID,
			"session", truncateOpenAIWSLogValue(sessionHash, 12),
			"stripped_fields", strings.Join(strippedFields, ","),
		)
	}
	// Shared OpenAI builder forces store=false (ZDR). For Grok, previous_response_id
	// only works when the prior turn was stored — keep store=true on the delta body
	// when require_store_on_create is on so T2's response remains a usable T3 anchor.
	body, storeErr := applyGrokActiveDeltaStorePolicy(body, s.grokHTTPActiveDeltaRequireStoreOnCreate())
	if storeErr != nil {
		return result, storeErr
	}
	result.body = body
	result.applied = true
	return result, nil
}

// applyGrokActiveDeltaStorePolicy overrides the shared active-delta store=false
// default when Grok requires stored responses for multi-turn previous anchors.
func applyGrokActiveDeltaStorePolicy(body []byte, requireStore bool) ([]byte, error) {
	if len(body) == 0 || !requireStore {
		return body, nil
	}
	if gjson.GetBytes(body, "store").Bool() {
		return body, nil
	}
	return sjson.SetBytes(body, "store", true)
}

func (s *OpenAIGatewayService) releaseGrokHTTPActiveDeltaSession(c *gin.Context, sessionHash string) {
	if s == nil || c == nil || sessionHash == "" {
		return
	}
	if store := s.getOpenAIWSStateStore(); store != nil {
		store.EndSessionInFlight(getOpenAIGroupIDFromContext(c), getAPIKeyIDFromContext(c), sessionHash)
	}
}

// invalidateGrokHTTPActiveDeltaSession drops a dead session context so the next
// turn falls back to full create (with store=true) instead of re-applying a
// previous_response_id that upstream already rejected.
func (s *OpenAIGatewayService) invalidateGrokHTTPActiveDeltaSession(
	c *gin.Context,
	accountID int64,
	sessionHash string,
	previousResponseID string,
	reason string,
) {
	if s == nil || sessionHash == "" {
		return
	}
	groupID := int64(0)
	apiKeyID := int64(0)
	if c != nil {
		groupID = getOpenAIGroupIDFromContext(c)
		apiKeyID = getAPIKeyIDFromContext(c)
	}
	if store := s.getOpenAIWSStateStore(); store != nil {
		store.DeleteSessionContext(groupID, apiKeyID, sessionHash)
	}
	slog.Info("grok_http_active_delta_session_invalidated",
		"account_id", accountID,
		"reason", reason,
		"session", truncateOpenAIWSLogValue(sessionHash, 12),
		"previous_response_id", truncateOpenAIWSLogValue(previousResponseID, openAIWSIDValueMaxLen),
	)
}

func (s *OpenAIGatewayService) bindGrokHTTPResponseSessionContext(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	canonicalBody []byte,
	cacheIdentity string,
	responseID string,
) {
	if s == nil || c == nil || account == nil || !account.IsGrok() || account.Type != AccountTypeOAuth {
		return
	}
	if !s.grokHTTPActiveDeltaEnabled() {
		return
	}
	if !hasExplicitGrokSessionIdentity(c) {
		return
	}
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return
	}
	sessionHash := resolveGrokActiveDeltaSessionHash(c, cacheIdentity)
	if sessionHash == "" {
		return
	}
	inputItems, exists, err := openAIWSExtractNormalizedInputSequence(canonicalBody)
	if err != nil || !exists {
		return
	}
	inputHashes, ok := openAIWSCanonicalItemHashes(inputItems)
	if !ok {
		return
	}
	inputShapes, ok := openAIWSItemShapes(inputItems)
	if !ok {
		return
	}
	nonInputHash, _, nonInputFields := openAIWSNonInputFingerprint(canonicalBody)
	store := s.getOpenAIWSStateStore()
	if store == nil {
		return
	}
	groupID := getOpenAIGroupIDFromContext(c)
	apiKeyID := getAPIKeyIDFromContext(c)
	ttl := s.openAIWSSessionStickyTTL()
	value := openAIWSSessionContextValue{
		accountID:               account.ID,
		connID:                  "http",
		lastResponseID:          responseID,
		materializedHashes:      inputHashes,
		materializedShapes:      inputShapes,
		materializedCount:       len(inputHashes),
		inputCount:              len(inputHashes),
		inputOnlyContext:        true,
		nonInputHash:            nonInputHash,
		nonInputFields:          nonInputFields,
		rawVsClientVisibleEqual: true,
	}
	store.BindSessionContext(groupID, apiKeyID, sessionHash, value, ttl)
	_ = ctx
}

// prepareGrokFullUpstreamBody applies Full-path safety: strip untrusted
// previous_response_id (P5) and apply store policy for create turns.
func prepareGrokFullUpstreamBody(body []byte, requireStoreOnCreate bool) ([]byte, error) {
	if len(body) == 0 {
		return body, nil
	}
	if hasOpenAIHTTPActiveDeltaToolContinuationOutput(body, nil) {
		// Keep previous when tool continuation requires it; still force store policy below carefully.
		out := append([]byte(nil), body...)
		if requireStoreOnCreate {
			var err error
			if !gjson.GetBytes(out, "store").Exists() || !gjson.GetBytes(out, "store").Bool() {
				out, err = sjson.SetBytes(out, "store", true)
				if err != nil {
					return nil, err
				}
			}
		}
		return out, nil
	}
	restored, ok, err := restoreOpenAIHTTPActiveDeltaFullReplayBody(body)
	if err != nil {
		return nil, err
	}
	if ok {
		body = restored
	}
	if requireStoreOnCreate {
		body, err = sjson.SetBytes(body, "store", true)
		if err != nil {
			return nil, err
		}
	}
	return body, nil
}

func isGrokPreviousResponseRecoveryError(statusCode int, upstreamCode, upstreamMsg string, upstreamBody []byte) bool {
	if isOpenAICompatPreviousResponseNotFound(statusCode, upstreamMsg, upstreamBody) {
		return true
	}
	if statusCode == http.StatusBadRequest && isOpenAIUnsupportedPreviousResponseIDError(upstreamCode, upstreamMsg) {
		return true
	}
	// xAI: "Argument not supported: instructions and previous_response_id together"
	if statusCode == http.StatusBadRequest && isGrokActiveDeltaInstructionsConflict(upstreamMsg) {
		return true
	}
	return false
}

// grokActiveDeltaReplayReason classifies why an applied delta had to full-replay.
func grokActiveDeltaReplayReason(statusCode int, upstreamCode, upstreamMsg string, upstreamBody []byte) string {
	if isOpenAICompatPreviousResponseNotFound(statusCode, upstreamMsg, upstreamBody) {
		return "previous_response_not_found"
	}
	if isGrokActiveDeltaInstructionsConflict(upstreamMsg) {
		return "instructions_previous_conflict"
	}
	if isOpenAIUnsupportedPreviousResponseIDError(upstreamCode, upstreamMsg) {
		return "previous_response_unsupported"
	}
	return "previous_response_recovery"
}

func clientResponseAlreadyWritten(c *gin.Context) bool {
	return c != nil && c.Writer != nil && c.Writer.Written()
}

// grokResponsesHTTPCall is the shared request-side egress for Grok Responses
// (active-delta + retries). Callers own response translation to the client.
type grokResponsesHTTPCall struct {
	Resp               *http.Response
	UpstreamBody       []byte
	CanonicalBody      []byte
	CacheIdentity      string
	ActiveDeltaApplied bool
	ActiveDeltaLog     openAIWSDeltaShadowLog
	releaseSession     func()
}

func (r *grokResponsesHTTPCall) Release() {
	if r != nil && r.releaseSession != nil {
		r.releaseSession()
		r.releaseSession = nil
	}
}

// callGrokResponsesHTTP performs the Grok Responses HTTP round-trip with
// safe active-delta mutation and recovery retries (P2 request egress).
func (s *OpenAIGatewayService) callGrokResponsesHTTP(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	canonicalBody []byte,
	cacheIdentity string,
) (*grokResponsesHTTPCall, error) {
	if s == nil || account == nil {
		return nil, fmt.Errorf("grok upstream: missing service or account")
	}

	clientPreviousResponseID := strings.TrimSpace(gjson.GetBytes(canonicalBody, "previous_response_id").String())
	activeDeltaOriginalBody := append([]byte(nil), canonicalBody...)
	upstreamBody := append([]byte(nil), canonicalBody...)
	activeDeltaApplied := false
	activeDeltaSessionHash := ""
	activeDeltaSessionOwner := false
	activeDeltaLog := openAIWSDeltaShadowLog{}
	activeDeltaPreviousResponseID := ""
	clientStoreDisabled := grokClientExplicitlyDisablesStore(c)
	requireStoreOnCreate := s.grokHTTPActiveDeltaRequireStoreOnCreate() && !clientStoreDisabled
	activeDeltaEnabled := s.grokHTTPActiveDeltaEnabled() && !clientStoreDisabled

	if account.Type == AccountTypeOAuth && activeDeltaEnabled {
		deltaResult, buildErr := s.buildGrokHTTPActiveDeltaPayload(ctx, c, account, canonicalBody, cacheIdentity, clientPreviousResponseID)
		if buildErr != nil {
			logger.LegacyPrintf(
				"service.openai_gateway",
				"[Grok] Skip HTTP active delta after build error (account: %s, error: %v)",
				account.Name,
				buildErr,
			)
		} else if deltaResult.applied {
			upstreamBody = deltaResult.body
			activeDeltaApplied = true
			activeDeltaSessionHash = deltaResult.sessionHash
			activeDeltaSessionOwner = deltaResult.sessionOwner
			activeDeltaLog = deltaResult.log
			activeDeltaPreviousResponseID = strings.TrimSpace(gjson.GetBytes(upstreamBody, "previous_response_id").String())
			setOpsUpstreamRequestBody(c, upstreamBody)
			slog.Info("grok_http_active_delta_applied",
				"account_id", account.ID,
				"session", truncateOpenAIWSLogValue(activeDeltaSessionHash, 12),
				"previous_response_id", truncateOpenAIWSLogValue(activeDeltaPreviousResponseID, openAIWSIDValueMaxLen),
				"delta_items", activeDeltaLog.DeltaItems,
				"delta_bytes", activeDeltaLog.DeltaBytes,
				"full_items", activeDeltaLog.FullItems,
				"full_bytes", activeDeltaLog.FullBytes,
			)
		} else {
			activeDeltaSessionHash = deltaResult.sessionHash
			activeDeltaSessionOwner = deltaResult.sessionOwner
			activeDeltaLog = deltaResult.log
			prepared, prepErr := prepareGrokFullUpstreamBody(canonicalBody, requireStoreOnCreate)
			if prepErr != nil {
				if activeDeltaSessionOwner {
					s.releaseGrokHTTPActiveDeltaSession(c, activeDeltaSessionHash)
				}
				return nil, prepErr
			}
			upstreamBody = prepared
			if activeDeltaLog.FallbackReason != "" {
				// expected_full_* = normal full create; other reasons = regressions to watch.
				level := "info"
				if !strings.HasPrefix(activeDeltaLog.FallbackReason, "expected_full_") {
					level = "warn"
				}
				attrs := []any{
					"account_id", account.ID,
					"reason", activeDeltaLog.FallbackReason,
					"session", truncateOpenAIWSLogValue(activeDeltaSessionHash, 12),
					"expected_full", strings.HasPrefix(activeDeltaLog.FallbackReason, "expected_full_"),
				}
				if level == "warn" {
					slog.Warn("grok_http_active_delta_skipped", attrs...)
				} else {
					slog.Info("grok_http_active_delta_skipped", attrs...)
				}
			}
		}
	} else if account.Type == AccountTypeOAuth && requireStoreOnCreate {
		prepared, prepErr := prepareGrokFullUpstreamBody(canonicalBody, true)
		if prepErr != nil {
			return nil, prepErr
		}
		upstreamBody = prepared
	}

	release := func() {
		if activeDeltaSessionOwner {
			s.releaseGrokHTTPActiveDeltaSession(c, activeDeltaSessionHash)
			activeDeltaSessionOwner = false
		}
	}

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		release()
		return nil, err
	}

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()

	buildReq := func(body []byte) (*http.Request, error) {
		req, buildErr := buildGrokResponsesRequest(upstreamCtx, c, account, body, token, s.settingService)
		if buildErr != nil {
			return nil, buildErr
		}
		identity := strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String())
		if identity == "" {
			identity = strings.TrimSpace(cacheIdentity)
		}
		if identity != "" {
			req.Header.Set("session_id", identity)
			req.Header.Set("x-grok-conv-id", identity)
		}
		return req, nil
	}

	upstreamReq, err := buildReq(upstreamBody)
	if err != nil {
		release()
		return nil, err
	}

	tlsRuntime := s.resolveGrokTLSFingerprintRuntime(ctx, c, account, "http")
	applyGrokRuntimeHeaders(upstreamReq, tlsRuntime)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	compactionRetryTried := false
	previousFullReplayTried := false
	var resp *http.Response
	for {
		upstreamStart := time.Now()
		resp, err = s.httpUpstream.DoWithTLS(upstreamReq, proxyURL, account.ID, account.Concurrency, tlsRuntime.Profile)
		SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
		if err != nil {
			release()
			return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
		}

		if resp.StatusCode < 400 {
			if activeDeltaApplied {
				slog.Info("grok_http_active_delta_success",
					"account_id", account.ID,
					"session", truncateOpenAIWSLogValue(activeDeltaSessionHash, 12),
					"previous_response_id", truncateOpenAIWSLogValue(activeDeltaPreviousResponseID, openAIWSIDValueMaxLen),
					"delta_items", activeDeltaLog.DeltaItems,
					"delta_bytes", activeDeltaLog.DeltaBytes,
					"full_items", activeDeltaLog.FullItems,
					"full_bytes", activeDeltaLog.FullBytes,
					"upstream_status", resp.StatusCode,
				)
			}
			break
		}

		respBody := s.readUpstreamErrorBody(resp)
		resp.Body = io.NopCloser(bytes.NewReader(respBody))

		if !compactionRetryTried && isGrokCompactionBlobDecodeError(resp.StatusCode, respBody) {
			retryBody, retryable, retryErr := sanitizeGrokCompactionReplayBody(upstreamBody)
			if retryErr != nil {
				_ = resp.Body.Close()
				release()
				return nil, retryErr
			}
			if retryable {
				_ = resp.Body.Close()
				compactionRetryTried = true
				upstreamBody = retryBody
				cacheIdentity = strings.TrimSpace(gjson.GetBytes(upstreamBody, "prompt_cache_key").String())
				setOpsUpstreamRequestBody(c, upstreamBody)
				upstreamReq, err = buildReq(upstreamBody)
				if err != nil {
					release()
					return nil, err
				}
				applyGrokRuntimeHeaders(upstreamReq, tlsRuntime)
				continue
			}
		}

		upstreamMsg := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(respBody))
		upstreamCode := extractUpstreamErrorCode(respBody)
		if !previousFullReplayTried &&
			!clientResponseAlreadyWritten(c) &&
			isGrokPreviousResponseRecoveryError(resp.StatusCode, upstreamCode, upstreamMsg, respBody) {
			replaySource := activeDeltaOriginalBody
			if len(replaySource) == 0 {
				replaySource = canonicalBody
			}
			restoredBody, restored, restoreErr := restoreOpenAIHTTPActiveDeltaFullReplayBody(replaySource)
			if restoreErr != nil {
				_ = resp.Body.Close()
				release()
				return nil, fmt.Errorf("restore grok active delta full replay body: %w", restoreErr)
			}
			if !restored && gjson.GetBytes(upstreamBody, "previous_response_id").Exists() &&
				!hasOpenAIHTTPActiveDeltaToolContinuationOutput(upstreamBody, nil) {
				restoredBody, restored, restoreErr = restoreOpenAIHTTPActiveDeltaFullReplayBody(upstreamBody)
				if restoreErr != nil {
					_ = resp.Body.Close()
					release()
					return nil, restoreErr
				}
			}
			if restored || activeDeltaApplied {
				if !restored {
					restoredBody = append([]byte(nil), replaySource...)
					var prepErr error
					restoredBody, prepErr = prepareGrokFullUpstreamBody(restoredBody, requireStoreOnCreate)
					if prepErr != nil {
						_ = resp.Body.Close()
						release()
						return nil, prepErr
					}
				} else if requireStoreOnCreate {
					var prepErr error
					restoredBody, prepErr = sjson.SetBytes(restoredBody, "store", true)
					if prepErr != nil {
						_ = resp.Body.Close()
						release()
						return nil, prepErr
					}
				}
				// Any previous_response recovery full-replay drops the session context so a
				// dead anchor (applied delta OR stale session/client previous) is not re-used.
				replayReason := grokActiveDeltaReplayReason(resp.StatusCode, upstreamCode, upstreamMsg, respBody)
				failedPreviousID := activeDeltaPreviousResponseID
				if failedPreviousID == "" {
					failedPreviousID = strings.TrimSpace(gjson.GetBytes(upstreamBody, "previous_response_id").String())
				}
				if activeDeltaSessionHash != "" {
					s.invalidateGrokHTTPActiveDeltaSession(
						c,
						account.ID,
						activeDeltaSessionHash,
						failedPreviousID,
						replayReason,
					)
				}
				_ = resp.Body.Close()
				previousFullReplayTried = true
				wasDeltaApplied := activeDeltaApplied
				activeDeltaApplied = false
				upstreamBody = restoredBody
				setOpsUpstreamRequestBody(c, upstreamBody)
				upstreamReq, err = buildReq(upstreamBody)
				if err != nil {
					release()
					return nil, err
				}
				applyGrokRuntimeHeaders(upstreamReq, tlsRuntime)
				slog.Info("grok_http_active_delta_full_replay_retry",
					"account_id", account.ID,
					"status", resp.StatusCode,
					"was_delta_applied", wasDeltaApplied,
					"reason", replayReason,
					"session", truncateOpenAIWSLogValue(activeDeltaSessionHash, 12),
					"previous_response_id", truncateOpenAIWSLogValue(failedPreviousID, openAIWSIDValueMaxLen),
					"upstream_msg", truncateOpenAIWSLogValue(upstreamMsg, 120),
				)
				continue
			}
			if clientResponseAlreadyWritten(c) {
				slog.Info("grok_http_active_delta_full_replay_blocked",
					"account_id", account.ID,
					"reason", "already_written",
				)
			}
		}

		// Non-recoverable upstream error: return response to caller for path-specific handling.
		out := &grokResponsesHTTPCall{
			Resp:               resp,
			UpstreamBody:       upstreamBody,
			CanonicalBody:      activeDeltaOriginalBody,
			CacheIdentity:      cacheIdentity,
			ActiveDeltaApplied: activeDeltaApplied,
			ActiveDeltaLog:     activeDeltaLog,
			releaseSession:     release,
		}
		return out, nil
	}

	return &grokResponsesHTTPCall{
		Resp:               resp,
		UpstreamBody:       upstreamBody,
		CanonicalBody:      activeDeltaOriginalBody,
		CacheIdentity:      cacheIdentity,
		ActiveDeltaApplied: activeDeltaApplied,
		ActiveDeltaLog:     activeDeltaLog,
		releaseSession:     release,
	}, nil
}

// doGrokResponsesUpstream handles OpenAI Responses client protocol end-to-end
// via the shared callGrokResponsesHTTP egress.
func (s *OpenAIGatewayService) doGrokResponsesUpstream(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	canonicalBody []byte,
	originalModel string,
	upstreamModel string,
	cacheIdentity string,
	reqStream bool,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	call, err := s.callGrokResponsesHTTP(ctx, c, account, canonicalBody, cacheIdentity)
	if err != nil {
		return nil, err
	}
	defer call.Release()
	resp := call.Resp
	defer func() {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	if resp.StatusCode >= 400 {
		respBody := s.readUpstreamErrorBody(resp)
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		upstreamMsg := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(respBody))
		if upstreamMsg == "" {
			upstreamMsg = fmt.Sprintf("xAI upstream returned status %d", resp.StatusCode)
		}
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id")),
			Kind:               "failover",
			Message:            upstreamMsg,
		})
		s.handleGrokAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
		if s.shouldFailoverUpstreamError(resp.StatusCode) {
			return nil, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
			}
		}
		return s.handleErrorResponse(ctx, resp, c, account, call.UpstreamBody, upstreamModel)
	}

	s.updateGrokUsageSnapshot(ctx, account, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))

	var usage *OpenAIUsage
	var firstTokenMs *int
	responseID := ""
	searchCount := 0
	if reqStream {
		streamResult, err := s.handleStreamingResponse(ctx, resp, c, account, startTime, originalModel, upstreamModel)
		if err != nil {
			if streamResult != nil && openaiStreamingErrorBillsPartial(err) {
				partial := buildOpenAIStreamingPartialForwardResult(resp, call.UpstreamBody, originalModel, upstreamModel, streamResult, time.Since(startTime))
				partial.OpenAIWSDeltaActive = call.ActiveDeltaApplied
				partial.OpenAIWSPayloadBytes = len(call.UpstreamBody)
				partial.OpenAIWSDeltaItems = call.ActiveDeltaLog.DeltaItems
				partial.OpenAIWSDeltaBytes = call.ActiveDeltaLog.DeltaBytes
				partial.OpenAIWSFullItems = call.ActiveDeltaLog.FullItems
				partial.OpenAIWSFullBytes = call.ActiveDeltaLog.FullBytes
				return partial, err
			}
			return nil, err
		}
		usage = streamResult.usage
		firstTokenMs = streamResult.firstTokenMs
		responseID = strings.TrimSpace(streamResult.responseID)
		searchCount = streamResult.searchCount
	} else {
		nonStreamResult, err := s.handleNonStreamingResponse(ctx, resp, c, account, originalModel, upstreamModel)
		if err != nil {
			if nonStreamResult != nil {
				partial := buildOpenAIPartialForwardResult(resp, call.UpstreamBody, originalModel, upstreamModel, nonStreamResult.usage, nonStreamResult.responseID, nonStreamResult.imageCount, nonStreamResult.searchCount, time.Since(startTime))
				partial.OpenAIWSDeltaActive = call.ActiveDeltaApplied
				partial.OpenAIWSPayloadBytes = len(call.UpstreamBody)
				partial.OpenAIWSDeltaItems = call.ActiveDeltaLog.DeltaItems
				partial.OpenAIWSDeltaBytes = call.ActiveDeltaLog.DeltaBytes
				partial.OpenAIWSFullItems = call.ActiveDeltaLog.FullItems
				partial.OpenAIWSFullBytes = call.ActiveDeltaLog.FullBytes
				return partial, err
			}
			return nil, err
		}
		usage = nonStreamResult.usage
		responseID = strings.TrimSpace(nonStreamResult.responseID)
		searchCount = nonStreamResult.searchCount
	}

	if usage == nil {
		usage = &OpenAIUsage{}
	}
	if responseID != "" {
		s.bindHTTPResponseAccount(ctx, c, account, responseID)
		if !grokClientExplicitlyDisablesStore(c) {
			s.bindGrokHTTPResponseSessionContext(ctx, c, account, call.CanonicalBody, call.CacheIdentity, responseID)
		}
	}

	return &OpenAIForwardResult{
		RequestID:            firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id")),
		ResponseID:           responseID,
		Usage:                *usage,
		Model:                originalModel,
		BillingModel:         upstreamModel,
		UpstreamModel:        upstreamModel,
		ReasoningEffort:      extractOpenAIReasoningEffortFromBody(call.UpstreamBody, originalModel, upstreamModel),
		Stream:               reqStream,
		OpenAIWSMode:         false,
		OpenAIWSDeltaActive:  call.ActiveDeltaApplied,
		OpenAIWSPayloadBytes: len(call.UpstreamBody),
		OpenAIWSDeltaItems:   call.ActiveDeltaLog.DeltaItems,
		OpenAIWSDeltaBytes:   call.ActiveDeltaLog.DeltaBytes,
		OpenAIWSFullItems:    call.ActiveDeltaLog.FullItems,
		OpenAIWSFullBytes:    call.ActiveDeltaLog.FullBytes,
		ResponseHeaders:      resp.Header.Clone(),
		Duration:             time.Since(startTime),
		FirstTokenMs:         firstTokenMs,
		SearchCount:          searchCount,
	}, nil
}
