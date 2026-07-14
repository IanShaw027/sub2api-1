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

func (s *OpenAIGatewayService) buildGrokHTTPActiveDeltaPayload(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	canonicalBody []byte,
	cacheIdentity string,
	clientPreviousResponseID string,
) (result grokHTTPActiveDeltaResult, err error) {
	result = grokHTTPActiveDeltaResult{body: canonicalBody}
	if s == nil || c == nil || account == nil || !account.IsGrok() || account.Type != AccountTypeOAuth {
		return result, nil
	}
	if !s.grokHTTPActiveDeltaEnabled() {
		return result, nil
	}
	if !hasExplicitGrokSessionIdentity(c) {
		return result, nil
	}
	if isOpenAIResponsesCompactPath(c) {
		return result, nil
	}
	if HasToolContinuationOutputInRawPayload(canonicalBody) {
		return result, nil
	}

	sessionHash := resolveGrokActiveDeltaSessionHash(c, cacheIdentity)
	if sessionHash == "" {
		return result, nil
	}
	store := s.getOpenAIWSStateStore()
	if store == nil {
		return result, nil
	}

	groupID := getOpenAIGroupIDFromContext(c)
	apiKeyID := getAPIKeyIDFromContext(c)
	requestID, _ := openAIWSRequestLogIDs(c)
	result.sessionHash = sessionHash

	if !store.TrySessionInFlight(groupID, apiKeyID, sessionHash) {
		result.log = openAIWSDeltaShadowLog{
			GroupID:        groupID,
			APIKeyID:       apiKeyID,
			SessionHash:    sessionHash,
			RequestID:      requestID,
			AccountID:      account.ID,
			Candidate:      false,
			FallbackReason: "same_session_in_flight",
		}
		logOpenAIWSDeltaShadow(result.log)
		return result, nil
	}
	result.sessionOwner = true
	defer func() {
		if result.sessionOwner && !result.applied {
			store.EndSessionInFlight(groupID, apiKeyID, sessionHash)
			result.sessionOwner = false
		}
	}()

	cached, found := store.GetSessionContext(groupID, apiKeyID, sessionHash)
	if found && strings.TrimSpace(cached.connID) != "http" {
		result.log = openAIWSDeltaShadowLog{
			GroupID:              groupID,
			APIKeyID:             apiKeyID,
			SessionHash:          sessionHash,
			RequestID:            requestID,
			AccountID:            account.ID,
			CachedFound:          found,
			CachedAccountID:      cached.accountID,
			CachedConnID:         cached.connID,
			CachedLastResponseID: cached.lastResponseID,
			Candidate:            false,
			FallbackReason:       "transport_context_mismatch",
		}
		logOpenAIWSDeltaShadow(result.log)
		return result, nil
	}

	clientPreviousResponseID = strings.TrimSpace(clientPreviousResponseID)
	if clientPreviousResponseID == "" {
		clientPreviousResponseID = strings.TrimSpace(gjsonGetBytesString(canonicalBody, "previous_response_id"))
	}
	if clientPreviousResponseID != "" {
		if boundAccountID, accountErr := store.GetResponseAccount(ctx, groupID, apiKeyID, clientPreviousResponseID); accountErr == nil && boundAccountID > 0 && boundAccountID != account.ID {
			result.log = openAIWSDeltaShadowLog{
				GroupID:               groupID,
				APIKeyID:              apiKeyID,
				SessionHash:           sessionHash,
				RequestID:             requestID,
				AccountID:             account.ID,
				CachedFound:           found,
				CachedAccountID:       cached.accountID,
				CachedLastResponseID:  cached.lastResponseID,
				Candidate:             false,
				FallbackReason:        "account_mismatch",
				AccountMismatchReason: "response_account_mismatch",
			}
			logOpenAIWSDeltaShadow(result.log)
			return result, nil
		}
	}
	if clientPreviousResponseID != "" && clientPreviousResponseID != strings.TrimSpace(cached.lastResponseID) {
		result.log = openAIWSDeltaShadowLog{
			GroupID:              groupID,
			APIKeyID:             apiKeyID,
			SessionHash:          sessionHash,
			RequestID:            requestID,
			AccountID:            account.ID,
			CachedFound:          found,
			CachedAccountID:      cached.accountID,
			CachedLastResponseID: cached.lastResponseID,
			Candidate:            false,
			FallbackReason:       "previous_response_mismatch",
		}
		logOpenAIWSDeltaShadow(result.log)
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
	result.log = deltaLog
	logOpenAIWSDeltaShadow(deltaLog)
	if err != nil || !applied {
		return result, err
	}
	body, err := marshalOpenAIResponsesRequestBodyOrdered(deltaPayload)
	if err != nil {
		return result, err
	}
	result.body = body
	result.applied = true
	return result, nil
}

func (s *OpenAIGatewayService) releaseGrokHTTPActiveDeltaSession(c *gin.Context, sessionHash string) {
	if s == nil || c == nil || sessionHash == "" {
		return
	}
	if store := s.getOpenAIWSStateStore(); store != nil {
		store.EndSessionInFlight(getOpenAIGroupIDFromContext(c), getAPIKeyIDFromContext(c), sessionHash)
	}
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
	return false
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

	if account.Type == AccountTypeOAuth && s.grokHTTPActiveDeltaEnabled() {
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
			setOpsUpstreamRequestBody(c, upstreamBody)
			slog.Info("grok_http_active_delta_applied",
				"account_id", account.ID,
				"session", truncateOpenAIWSLogValue(activeDeltaSessionHash, 12),
				"previous_response_id", truncateOpenAIWSLogValue(gjson.GetBytes(upstreamBody, "previous_response_id").String(), openAIWSIDValueMaxLen),
				"delta_items", activeDeltaLog.DeltaItems,
				"delta_bytes", activeDeltaLog.DeltaBytes,
				"full_items", activeDeltaLog.FullItems,
				"full_bytes", activeDeltaLog.FullBytes,
			)
		} else {
			activeDeltaSessionHash = deltaResult.sessionHash
			activeDeltaSessionOwner = deltaResult.sessionOwner
			activeDeltaLog = deltaResult.log
			prepared, prepErr := prepareGrokFullUpstreamBody(canonicalBody, s.grokHTTPActiveDeltaRequireStoreOnCreate())
			if prepErr != nil {
				if activeDeltaSessionOwner {
					s.releaseGrokHTTPActiveDeltaSession(c, activeDeltaSessionHash)
				}
				return nil, prepErr
			}
			upstreamBody = prepared
			if activeDeltaLog.FallbackReason != "" {
				slog.Info("grok_http_active_delta_skipped",
					"account_id", account.ID,
					"reason", activeDeltaLog.FallbackReason,
					"session", truncateOpenAIWSLogValue(activeDeltaSessionHash, 12),
				)
			}
		}
	} else if account.Type == AccountTypeOAuth && s.grokHTTPActiveDeltaRequireStoreOnCreate() {
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
					restoredBody, prepErr = prepareGrokFullUpstreamBody(restoredBody, s.grokHTTPActiveDeltaRequireStoreOnCreate())
					if prepErr != nil {
						_ = resp.Body.Close()
						release()
						return nil, prepErr
					}
				} else if s.grokHTTPActiveDeltaRequireStoreOnCreate() {
					var prepErr error
					restoredBody, prepErr = sjson.SetBytes(restoredBody, "store", true)
					if prepErr != nil {
						_ = resp.Body.Close()
						release()
						return nil, prepErr
					}
				}
				_ = resp.Body.Close()
				previousFullReplayTried = true
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
		s.bindGrokHTTPResponseSessionContext(ctx, c, account, call.CanonicalBody, call.CacheIdentity, responseID)
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
