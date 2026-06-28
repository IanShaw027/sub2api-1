package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type openAIHTTPActiveDeltaResult struct {
	body         []byte
	log          openAIWSDeltaShadowLog
	sessionHash  string
	sessionOwner bool
	applied      bool
}

func (s *OpenAIGatewayService) buildOpenAIHTTPActiveDeltaPayload(ctx context.Context, c *gin.Context, account *Account, payload []byte, clientPreviousResponseID string) (result openAIHTTPActiveDeltaResult, err error) {
	result = openAIHTTPActiveDeltaResult{body: payload}
	if s == nil || c == nil || account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth {
		return result, nil
	}
	if !s.openAIHTTPIncrementalContinuationEnabled() || !openAIWSActiveDeltaEnabled() {
		return result, nil
	}
	if isOpenAIResponsesCompactPath(c) {
		return result, nil
	}
	sessionHash := s.GenerateSessionHash(c, payload)
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
	clientPreviousResponseID = strings.TrimSpace(clientPreviousResponseID)
	if clientPreviousResponseID == "" {
		clientPreviousResponseID = strings.TrimSpace(gjsonGetBytesString(payload, "previous_response_id"))
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

	shadowInput := openAIWSDeltaShadowInput{
		GroupID:               groupID,
		APIKeyID:              apiKeyID,
		SessionHash:           sessionHash,
		RequestID:             requestID,
		AccountID:             account.ID,
		CurrentPayload:        payload,
		HasFunctionCallOutput: HasToolContinuationOutputInRawPayload(payload),
		AllowConnReanchor:     true,
		StickyAccountID:       account.ID,
		StickyAccountHit:      true,
		ConnAffinityHit:       true,
		StoreFallbackReason:   "http_active_delta",
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

func (s *OpenAIGatewayService) releaseOpenAIHTTPActiveDeltaSession(c *gin.Context, sessionHash string) {
	if s == nil || c == nil || sessionHash == "" {
		return
	}
	if store := s.getOpenAIWSStateStore(); store != nil {
		store.EndSessionInFlight(getOpenAIGroupIDFromContext(c), getAPIKeyIDFromContext(c), sessionHash)
	}
}

func (s *OpenAIGatewayService) bindHTTPResponseSessionContext(ctx context.Context, c *gin.Context, account *Account, payload []byte, responseID string) {
	if s == nil || c == nil || account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth {
		return
	}
	if !s.openAIHTTPIncrementalContinuationEnabled() {
		return
	}
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return
	}
	sessionHash := s.GenerateSessionHash(c, payload)
	if sessionHash == "" {
		return
	}
	inputItems, exists, err := openAIWSExtractNormalizedInputSequence(payload)
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
	nonInputHash, _, nonInputFields := openAIWSNonInputFingerprint(payload)
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
	logOpenAIWSSessionContextBind(groupID, apiKeyID, account.ID, account.Type, sessionHash, "http", responseID, ttl, len(inputHashes), len(inputHashes), false, true)
}

func restoreOpenAIHTTPActiveDeltaFullReplayBody(original []byte) ([]byte, bool, error) {
	if len(original) == 0 {
		return nil, false, nil
	}
	var reqBody map[string]any
	if err := json.Unmarshal(original, &reqBody); err != nil {
		return nil, false, err
	}
	if HasFunctionCallOutput(reqBody) {
		return nil, false, nil
	}
	changed := false
	if _, present := reqBody["previous_response_id"]; present {
		delete(reqBody, "previous_response_id")
		changed = true
	}
	if store, ok := reqBody["store"].(bool); !ok || store {
		reqBody["store"] = false
		changed = true
	}
	if !changed {
		return append([]byte(nil), original...), true, nil
	}
	body, err := marshalOpenAIResponsesRequestBodyOrdered(reqBody)
	if err != nil {
		return nil, false, err
	}
	return body, true, nil
}

func restoreOpenAIHTTPActiveDeltaInvalidEncryptedContentBody(original []byte) ([]byte, bool, bool, error) {
	if len(original) == 0 {
		return nil, false, false, nil
	}
	var reqBody map[string]any
	if err := json.Unmarshal(original, &reqBody); err != nil {
		return nil, false, false, err
	}
	removedReasoningItems := trimOpenAIEncryptedReasoningItems(reqBody)
	droppedPreviousResponseID := false
	if previousResponseID := openAIWSPayloadString(reqBody, "previous_response_id"); previousResponseID != "" && !HasFunctionCallOutput(reqBody) {
		delete(reqBody, "previous_response_id")
		droppedPreviousResponseID = true
	}
	if store, ok := reqBody["store"].(bool); !ok || store {
		reqBody["store"] = false
	}
	if !removedReasoningItems && !droppedPreviousResponseID {
		return nil, false, false, nil
	}
	body, err := marshalOpenAIResponsesRequestBodyOrdered(reqBody)
	if err != nil {
		return nil, false, false, err
	}
	return body, removedReasoningItems, droppedPreviousResponseID, nil
}

func gjsonGetBytesString(payload []byte, path string) string {
	return strings.TrimSpace(gjson.GetBytes(payload, path).String())
}
