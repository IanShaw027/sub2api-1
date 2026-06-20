package service

// TEMP_DIAG(openai_ws_delta_shadow) remove_after_debug=true
//
// Shadow-only instrumentation for gateway-side strict-delta continuation. Everything in
// this file computes/measures the delta candidate; it MUST NOT mutate any upstream payload.
// See plan: only-new-input-items continuation via previous_response_id, store=false/ZDR
// compatible, active-socket most-recent-response cache.

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/tidwall/gjson"
)

// openAIWSDeltaShadowEnabled gates all shadow instrumentation. Default on; ops can disable
// at startup via OPENAI_WS_DELTA_SHADOW_DISABLED=1. Shadow is inert (no payload mutation),
// so the gate exists only as a kill-switch.
func openAIWSDeltaShadowEnabled() bool {
	return os.Getenv("OPENAI_WS_DELTA_SHADOW_DISABLED") != "1"
}

// openAIWSActiveDeltaEnabled gates payload mutation for strict-delta continuation.
// It depends on the shadow machinery because the same strict checks produce both
// the diagnostic line and the active payload.
func openAIWSActiveDeltaEnabled() bool {
	return openAIWSDeltaShadowEnabled() && os.Getenv("OPENAI_WS_ACTIVE_DELTA_DISABLED") != "1"
}

// openAIWSHashSlicesEqual reports whether two canonical-hash slices are identical.
func openAIWSHashSlicesEqual(a, b [][32]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// openAIWSVolatileEnvelopeItemTypes: item types whose top-level id/status are
// server-assigned envelope fields safe to drop from the canonical hash. Anything NOT in
// this set (incl. item_reference and unknown future types) keeps ALL fields, so a missing
// entry can only cause an (safe) extra prefix break — never a false match.
var openAIWSVolatileEnvelopeItemTypes = map[string]struct{}{
	"message":               {},
	"reasoning":             {},
	"function_call":         {},
	"function_call_output":  {},
	"custom_tool_call":      {},
	"web_search_call":       {},
	"file_search_call":      {},
	"computer_call":         {},
	"computer_call_output":  {},
	"code_interpreter_call": {},
	"image_generation_call": {},
	"local_shell_call":      {},
	"mcp_call":              {},
	"mcp_list_tools":        {},
	"mcp_approval_request":  {},
	"tool_search_call":      {},
}

// openAIWSNonInputDenylist: fields excluded from the non-input semantic fingerprint. All
// other fields (incl. unknown ones) are included by default so future Responses params
// cannot silently cause a false match.
var openAIWSNonInputDenylist = []string{
	"input",
	"previous_response_id",
	"store",
	"client_metadata",
	"include",
	"instructions",
	"parallel_tool_calls",
	"reasoning",
	"text",
	"tool_choice",
	"tools",
	"turn_metadata",
}

// openAIWSCanonicalItemHash hashes one input/output item. JSON key order is canonicalized
// (recursively), but string CONTENT is never whitespace-normalized — a whitespace change
// in text must change the hash. Only confirmed envelope-volatile id/status are dropped,
// and only for known volatile item types.
func openAIWSCanonicalItemHash(item []byte) ([32]byte, bool) {
	var v any
	if err := json.Unmarshal(item, &v); err != nil {
		return [32]byte{}, false
	}
	if obj, ok := v.(map[string]any); ok {
		openAIWSNormalizeCanonicalItemObject(obj)
	}
	canon, err := openAIWSCanonicalJSON(v)
	if err != nil {
		return [32]byte{}, false
	}
	return sha256.Sum256(canon), true
}

func openAIWSNormalizeCanonicalItemObject(obj map[string]any) {
	itemType, _ := obj["type"].(string)
	itemType = strings.TrimSpace(itemType)
	if itemType == "" && openAIWSObjectLooksLikeReplayMessage(obj) {
		itemType = "message"
		obj["type"] = itemType
	}
	if itemType != "" {
		if _, volatile := openAIWSVolatileEnvelopeItemTypes[itemType]; volatile {
			delete(obj, "id")
			delete(obj, "status")
		}
	}
	openAIWSDropTurnIDMetadata(obj)
	switch itemType {
	case "reasoning":
		openAIWSDropEmptyArrayField(obj, "content")
	case "message":
		delete(obj, "phase")
		openAIWSNormalizeMessageContentEnvelope(obj)
		openAIWSNormalizeOutputMessageRole(obj)
	}
}

func openAIWSObjectLooksLikeReplayMessage(obj map[string]any) bool {
	if obj == nil {
		return false
	}
	role, _ := obj["role"].(string)
	switch strings.TrimSpace(role) {
	case "assistant", "user", "system", "developer":
	default:
		return false
	}
	_, hasContent := obj["content"]
	return hasContent
}

func openAIWSDropTurnIDMetadata(obj map[string]any) {
	openAIWSDropTurnIDObject(obj, "metadata")
	openAIWSDropTurnIDObject(obj, "internal_chat_message_metadata_passthrough")
}

func openAIWSDropTurnIDObject(obj map[string]any, key string) {
	metaRaw, exists := obj[key]
	if !exists {
		return
	}
	if metaRaw == nil {
		delete(obj, key)
		return
	}
	meta, ok := metaRaw.(map[string]any)
	if !ok {
		return
	}
	delete(meta, "turn_id")
	if len(meta) == 0 {
		delete(obj, key)
	}
}

func openAIWSDropEmptyArrayField(obj map[string]any, key string) {
	arr, ok := obj[key].([]any)
	if ok && len(arr) == 0 {
		delete(obj, key)
	}
}

func openAIWSNormalizeMessageContentEnvelope(obj map[string]any) {
	content, ok := obj["content"].([]any)
	if !ok {
		return
	}
	role, _ := obj["role"].(string)
	role = strings.TrimSpace(role)
	for _, raw := range content {
		contentObj, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		openAIWSDropEmptyArrayField(contentObj, "annotations")
		openAIWSDropEmptyArrayField(contentObj, "logprobs")
		openAIWSNormalizeAssistantTextContentType(role, contentObj)
	}
}

func openAIWSNormalizeAssistantTextContentType(role string, contentObj map[string]any) {
	contentType, _ := contentObj["type"].(string)
	contentType = strings.TrimSpace(contentType)
	switch contentType {
	case "output_text":
		contentObj["type"] = "assistant_text"
	case "input_text":
		if role == "assistant" {
			contentObj["type"] = "assistant_text"
		}
	}
}

func openAIWSNormalizeOutputMessageRole(obj map[string]any) {
	role, _ := obj["role"].(string)
	if strings.TrimSpace(role) != "assistant" || !openAIWSMessageContentLooksLikeOutput(obj) {
		return
	}
	delete(obj, "role")
}

func openAIWSMessageContentLooksLikeOutput(obj map[string]any) bool {
	content, ok := obj["content"].([]any)
	if !ok || len(content) == 0 {
		return false
	}
	hasOutput := false
	for _, raw := range content {
		contentObj, ok := raw.(map[string]any)
		if !ok {
			return false
		}
		contentType, _ := contentObj["type"].(string)
		switch strings.TrimSpace(contentType) {
		case "input_text", "input_image", "input_file", "input_audio":
			return false
		case "output_text", "assistant_text", "refusal":
			hasOutput = true
		}
	}
	return hasOutput
}

// openAIWSCanonicalItemHashes hashes a sequence of raw items.
func openAIWSCanonicalItemHashes(items []json.RawMessage) ([][32]byte, bool) {
	out := make([][32]byte, 0, len(items))
	for _, it := range items {
		h, ok := openAIWSCanonicalItemHash(it)
		if !ok {
			return nil, false
		}
		out = append(out, h)
	}
	return out, true
}

// openAIWSItemShapes returns no-value structural signatures for diagnostics. It records
// JSON paths and scalar kinds only, never string contents or raw request/response text.
func openAIWSItemShapes(items []json.RawMessage) ([]string, bool) {
	out := make([]string, 0, len(items))
	for _, it := range items {
		shape, ok := openAIWSItemShape(it)
		if !ok {
			return nil, false
		}
		out = append(out, shape)
	}
	return out, true
}

func openAIWSItemShape(item []byte) (string, bool) {
	var v any
	if err := json.Unmarshal(item, &v); err != nil {
		return "", false
	}
	paths := make([]string, 0, 32)
	openAIWSCollectJSONShape(v, "$", &paths)
	sort.Strings(paths)
	paths = compactOpenAIWSShapePaths(paths)
	if len(paths) > 96 {
		paths = append(paths[:96], "...")
	}
	itemType := openAIWSItemType(item)
	if itemType == "" {
		itemType = "unknown"
	}
	return "type=" + openAIWSSafeShapeToken(itemType) + ";paths=" + strings.Join(paths, "|"), true
}

func openAIWSCollectJSONShape(v any, path string, out *[]string) {
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 {
			*out = append(*out, path+"{}")
			return
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			openAIWSCollectJSONShape(t[k], path+"."+openAIWSSafeShapeToken(k), out)
		}
	case []any:
		if len(t) == 0 {
			*out = append(*out, path+"[]:empty")
			return
		}
		*out = append(*out, path+"[]")
		for _, elem := range t {
			openAIWSCollectJSONShape(elem, path+"[]", out)
		}
	case string:
		*out = append(*out, path+":string")
	case float64:
		*out = append(*out, path+":number")
	case bool:
		*out = append(*out, path+":bool")
	case nil:
		*out = append(*out, path+":null")
	default:
		*out = append(*out, path+":unknown")
	}
}

func compactOpenAIWSShapePaths(paths []string) []string {
	if len(paths) == 0 {
		return paths
	}
	out := paths[:0]
	last := ""
	for _, p := range paths {
		if p == last {
			continue
		}
		out = append(out, p)
		last = p
	}
	return out
}

func openAIWSSafeShapeToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			_, _ = b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			_, _ = b.WriteRune(r)
		case r >= '0' && r <= '9':
			_, _ = b.WriteRune(r)
		case r == '_' || r == '-' || r == '.':
			_, _ = b.WriteRune(r)
		default:
			_ = b.WriteByte('_')
		}
	}
	return b.String()
}

func openAIWSItemTypeFromShape(shape string) string {
	shape = strings.TrimSpace(shape)
	if !strings.HasPrefix(shape, "type=") {
		return ""
	}
	rest := strings.TrimPrefix(shape, "type=")
	if idx := strings.IndexByte(rest, ';'); idx >= 0 {
		rest = rest[:idx]
	}
	return strings.TrimSpace(rest)
}

// openAIWSNonInputHash hashes the request payload minus the denylist fields, returning the
// hash and the set of denylist keys that were actually present (for audit).
func openAIWSNonInputHash(payload []byte) ([32]byte, []string) {
	var v map[string]any
	if err := json.Unmarshal(payload, &v); err != nil {
		return [32]byte{}, nil
	}
	ignored := make([]string, 0, len(openAIWSNonInputDenylist))
	for _, k := range openAIWSNonInputDenylist {
		if _, ok := v[k]; ok {
			ignored = append(ignored, k)
			delete(v, k)
		}
	}
	canon, err := openAIWSCanonicalJSON(v)
	if err != nil {
		return [32]byte{}, ignored
	}
	return sha256.Sum256(canon), ignored
}

// openAIWSCanonicalJSON marshals with recursively sorted object keys. Go's json.Marshal
// already sorts object keys, so unmarshal→marshal is sufficient; this
// wrapper exists to make the intent explicit and centralize future tweaks.
func openAIWSCanonicalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

// openAIWSExtractResponseOutputItems extracts response.output[] as raw items from a
// response object's raw JSON (the `response` field of a terminal event).
func openAIWSExtractResponseOutputItems(responseRaw []byte) ([]json.RawMessage, bool) {
	if len(responseRaw) == 0 {
		return nil, false
	}
	out := gjson.GetBytes(responseRaw, "output")
	if !out.Exists() || !out.IsArray() {
		return nil, false
	}
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(out.Raw), &items); err != nil {
		return nil, false
	}
	if len(items) == 0 {
		return nil, false
	}
	return items, true
}

// openAIWSExtractOutputItemDoneItem extracts the completed output item from a
// response.output_item.done streaming event. Live WS streams often carry output items in
// these events while response.completed only carries summary/usage.
func openAIWSExtractOutputItemDoneItem(eventRaw []byte) (int, json.RawMessage, bool) {
	if len(eventRaw) == 0 || gjson.GetBytes(eventRaw, "type").String() != "response.output_item.done" {
		return 0, nil, false
	}
	item := gjson.GetBytes(eventRaw, "item")
	if !item.Exists() || item.Type != gjson.JSON {
		return 0, nil, false
	}
	idx := int(gjson.GetBytes(eventRaw, "output_index").Int())
	raw := json.RawMessage(append([]byte(nil), item.Raw...))
	return idx, raw, true
}

func openAIWSOrderedOutputDoneItems(items map[int]json.RawMessage) []json.RawMessage {
	if len(items) == 0 {
		return nil
	}
	indexes := make([]int, 0, len(items))
	for idx := range items {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	out := make([]json.RawMessage, 0, len(indexes))
	for _, idx := range indexes {
		out = append(out, items[idx])
	}
	return out
}

// openAIWSItemType returns the item's "type" for break-by-type metrics.
func openAIWSItemType(item []byte) string {
	return strings.TrimSpace(gjson.GetBytes(item, "type").String())
}

// openAIWSHashPrefixBreak returns (matched, breakIndex). matched==true means prefix is a
// strict prefix of full. Otherwise breakIndex is the first differing/missing index.
func openAIWSHashPrefixBreak(full [][32]byte, prefix [][32]byte) (bool, int) {
	if len(prefix) > len(full) {
		// missing item(s); break at the first missing position
		if len(full) < len(prefix) {
			return false, len(full)
		}
	}
	n := len(prefix)
	if len(full) < n {
		n = len(full)
	}
	for i := 0; i < n; i++ {
		if full[i] != prefix[i] {
			return false, i
		}
	}
	if len(prefix) > len(full) {
		return false, len(full)
	}
	return true, -1
}

// openAIWSDeltaShadowLog is the removable shadow diagnostic line.
type openAIWSDeltaShadowLog struct {
	RequestID           string
	AccountID           int64
	ConnID              string
	Active              bool
	Candidate           bool
	FallbackReason      string
	PrefixMatch         bool
	BreakBoundary       string // "input" | "output" | ""
	BreakItemType       string
	BreakCachedItemType string
	BreakCachedShape    string
	BreakCurrentShape   string
	ConnMatch           bool
	MostRecentMatch     bool
	NonInputMatch       bool
	RawClientEquiv      bool
	MaterializedCount   int
	CurrentInputCount   int
	DeltaItems          int
	DeltaBytes          int
	FullItems           int
	FullBytes           int
}

func logOpenAIWSDeltaShadow(v openAIWSDeltaShadowLog) {
	logger.LegacyPrintf("service.openai_gateway",
		"openai_ws_delta_shadow temporary_diag=openai_ws_delta_shadow remove_after_debug=true "+
			"request_id=%s account_id=%d conn_id=%s active=%v candidate=%v fallback_reason=%s prefix_match=%v "+
			"break_boundary=%s break_item_type=%s break_cached_item_type=%s break_cached_shape=%s "+
			"break_current_shape=%s conn_match=%v most_recent_match=%v non_input_match=%v "+
			"raw_client_equiv=%v materialized_count=%d current_input_count=%d delta_items=%d delta_bytes=%d "+
			"full_items=%d full_bytes=%d",
		normalizeOpenAIWSLogValue(v.RequestID),
		v.AccountID,
		normalizeOpenAIWSLogValue(v.ConnID),
		v.Active,
		v.Candidate,
		normalizeOpenAIWSLogValue(v.FallbackReason),
		v.PrefixMatch,
		normalizeOpenAIWSLogValue(v.BreakBoundary),
		normalizeOpenAIWSLogValue(v.BreakItemType),
		normalizeOpenAIWSLogValue(v.BreakCachedItemType),
		truncateOpenAIWSLogValue(v.BreakCachedShape, openAIWSLogValueMaxLen),
		truncateOpenAIWSLogValue(v.BreakCurrentShape, openAIWSLogValueMaxLen),
		v.ConnMatch,
		v.MostRecentMatch,
		v.NonInputMatch,
		v.RawClientEquiv,
		v.MaterializedCount,
		v.CurrentInputCount,
		v.DeltaItems,
		v.DeltaBytes,
		v.FullItems,
		v.FullBytes,
	)
}

// openAIWSDeltaShadowInput carries the runtime inputs for one follow-up turn's candidate
// evaluation. All fields are read-only snapshots; evaluation never mutates anything.
type openAIWSDeltaShadowInput struct {
	RequestID                string
	AccountID                int64
	LeaseConnID              string
	ConnMostRecentResponseID string // GetConnLastResponse(leaseConnID)
	CurrentPayload           []byte
	HasFunctionCallOutput    bool
	AllowConnReanchor        bool
	Cached                   openAIWSSessionContextValue
	CachedFound              bool
}

// evaluateOpenAIWSDeltaShadowCandidate computes (read-only) whether the current follow-up
// turn could have been sent as a strict delta, and what it would have saved. It returns a
// fully populated log record; it NEVER mutates the payload.
func evaluateOpenAIWSDeltaShadowCandidate(in openAIWSDeltaShadowInput) openAIWSDeltaShadowLog {
	log := openAIWSDeltaShadowLog{
		RequestID: in.RequestID,
		AccountID: in.AccountID,
		ConnID:    in.LeaseConnID,
	}

	fullItems, _, err := openAIWSExtractNormalizedInputSequence(in.CurrentPayload)
	if err != nil {
		log.FallbackReason = "input_parse_error"
		return log
	}
	log.FullItems = len(fullItems)
	log.FullBytes = openAIWSRawItemsByteLen(fullItems)
	log.CurrentInputCount = len(fullItems)

	if !in.CachedFound {
		log.FallbackReason = "no_session_context"
		return log
	}

	log.MaterializedCount = in.Cached.materializedCount
	accountMatch := in.Cached.accountID == in.AccountID
	log.ConnMatch = strings.TrimSpace(in.Cached.connID) != "" && in.Cached.connID == strings.TrimSpace(in.LeaseConnID)
	log.MostRecentMatch = strings.TrimSpace(in.ConnMostRecentResponseID) != "" &&
		in.ConnMostRecentResponseID == strings.TrimSpace(in.Cached.lastResponseID)
	connAnchorMatch := log.ConnMatch && log.MostRecentMatch
	connReanchorMatch := in.AllowConnReanchor && strings.TrimSpace(in.Cached.lastResponseID) != ""
	log.RawClientEquiv = in.Cached.rawVsClientVisibleEqual

	currentNonInput, _ := openAIWSNonInputHash(in.CurrentPayload)
	log.NonInputMatch = currentNonInput == in.Cached.nonInputHash

	currentHashes, hok := openAIWSCanonicalItemHashes(fullItems)
	if !hok {
		log.FallbackReason = "input_hash_error"
		return log
	}
	matched, breakIdx := openAIWSHashPrefixBreak(currentHashes, in.Cached.materializedHashes)
	log.PrefixMatch = matched
	if !matched {
		if breakIdx < in.Cached.inputCount {
			log.BreakBoundary = "input"
		} else {
			log.BreakBoundary = "output"
		}
		if breakIdx >= 0 && breakIdx < len(fullItems) {
			log.BreakItemType = openAIWSItemType(fullItems[breakIdx])
			if currentShape, ok := openAIWSItemShape(fullItems[breakIdx]); ok {
				log.BreakCurrentShape = currentShape
			}
		}
		if breakIdx >= 0 && breakIdx < len(in.Cached.materializedShapes) {
			log.BreakCachedShape = in.Cached.materializedShapes[breakIdx]
			log.BreakCachedItemType = openAIWSItemTypeFromShape(log.BreakCachedShape)
		}
	}

	deltaToolFallbackReason := ""
	if matched {
		if in.Cached.materializedCount < 0 || in.Cached.materializedCount > len(fullItems) {
			log.FallbackReason = "materialized_count_invalid"
			return log
		}
		deltaStart, ok := openAIWSActiveDeltaStartIndex(fullItems, in.Cached)
		if !ok {
			log.FallbackReason = "materialized_count_invalid"
			return log
		}
		delta := fullItems[deltaStart:]
		log.DeltaItems = len(delta)
		log.DeltaBytes = openAIWSRawItemsByteLen(delta)
		deltaToolFallbackReason = openAIWSActiveDeltaToolContinuationFallbackReason(delta)
	}

	log.Candidate = accountMatch && (connAnchorMatch || connReanchorMatch) && log.NonInputMatch &&
		log.RawClientEquiv && matched && deltaToolFallbackReason == ""

	if log.Candidate {
		return log
	}

	// 选择最具体的 fallback 原因（按依赖顺序）。
	switch {
	case !accountMatch:
		log.FallbackReason = "account_mismatch"
	case !log.ConnMatch && !connReanchorMatch:
		log.FallbackReason = "conn_mismatch"
	case !log.MostRecentMatch && !connReanchorMatch:
		log.FallbackReason = "not_most_recent"
	case !log.NonInputMatch:
		log.FallbackReason = "non_input_mismatch"
	case !log.RawClientEquiv:
		log.FallbackReason = "raw_client_divergent"
	case deltaToolFallbackReason != "":
		log.FallbackReason = deltaToolFallbackReason
	case !matched && log.BreakBoundary == "input":
		log.FallbackReason = "prefix_break_input"
	case !matched:
		log.FallbackReason = "prefix_break_output"
	default:
		log.FallbackReason = "unknown"
	}
	return log
}

func buildOpenAIWSActiveDeltaPayload(in openAIWSDeltaShadowInput) (map[string]any, openAIWSDeltaShadowLog, bool, error) {
	log := evaluateOpenAIWSDeltaShadowCandidate(in)
	if !log.Candidate {
		return nil, log, false, nil
	}
	if strings.TrimSpace(in.Cached.lastResponseID) == "" {
		log.Candidate = false
		log.FallbackReason = "missing_last_response_id"
		return nil, log, false, nil
	}

	fullItems, _, err := openAIWSExtractNormalizedInputSequence(in.CurrentPayload)
	if err != nil {
		log.Candidate = false
		log.FallbackReason = "input_parse_error"
		return nil, log, false, err
	}
	deltaStart, ok := openAIWSActiveDeltaStartIndex(fullItems, in.Cached)
	if !ok {
		log.Candidate = false
		log.FallbackReason = "materialized_count_invalid"
		return nil, log, false, nil
	}
	deltaItems := fullItems[deltaStart:]
	if len(deltaItems) == 0 {
		log.Candidate = false
		log.FallbackReason = "empty_delta"
		return nil, log, false, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(in.CurrentPayload, &payload); err != nil {
		log.Candidate = false
		log.FallbackReason = "payload_parse_error"
		return nil, log, false, err
	}

	deltaRaw, err := json.Marshal(deltaItems)
	if err != nil {
		log.Candidate = false
		log.FallbackReason = "delta_marshal_error"
		return nil, log, false, err
	}
	var deltaInput []any
	if err := json.Unmarshal(deltaRaw, &deltaInput); err != nil {
		log.Candidate = false
		log.FallbackReason = "delta_parse_error"
		return nil, log, false, err
	}

	payload["input"] = deltaInput
	payload["previous_response_id"] = strings.TrimSpace(in.Cached.lastResponseID)
	payload["store"] = false
	log.Active = true
	log.DeltaItems = len(deltaItems)
	log.DeltaBytes = openAIWSRawItemsByteLen(deltaItems)
	return payload, log, true, nil
}

func openAIWSActiveDeltaStartIndex(fullItems []json.RawMessage, cached openAIWSSessionContextValue) (int, bool) {
	if cached.materializedCount < 0 || cached.materializedCount > len(fullItems) {
		return 0, false
	}
	start := cached.materializedCount
	if cached.inputOnlyContext && cached.inputCount >= 0 && cached.materializedCount == cached.inputCount {
		start = openAIWSSkipReplayedOutputItems(fullItems, start)
	}
	return start, true
}

func openAIWSSkipReplayedOutputItems(items []json.RawMessage, start int) int {
	for start < len(items) && openAIWSItemLooksLikeReplayedResponseOutput(items[start]) {
		start++
	}
	return start
}

func openAIWSItemLooksLikeReplayedResponseOutput(item json.RawMessage) bool {
	parsed := gjson.ParseBytes(item)
	itemType := strings.TrimSpace(parsed.Get("type").String())
	role := strings.TrimSpace(parsed.Get("role").String())
	switch itemType {
	case "reasoning",
		"web_search_call",
		"file_search_call",
		"computer_call",
		"code_interpreter_call",
		"image_generation_call",
		"mcp_list_tools",
		"mcp_approval_request":
		return true
	case "message":
		return role == "" || role == "assistant"
	}
	if itemType == "" && role == "assistant" {
		return true
	}
	return isCodexToolCallContextItemType(itemType)
}

func openAIWSActiveDeltaToolContinuationFallbackReason(delta []json.RawMessage) string {
	if len(delta) == 0 {
		return ""
	}
	hasOutput := false
	hasContext := false
	missingOutputCallID := false
	for _, item := range delta {
		parsed := gjson.ParseBytes(item)
		itemType := parsed.Get("type").String()
		if rawInputItemHasToolContinuationOutput(parsed) {
			hasOutput = true
			callID := firstNonEmptyString(
				parsed.Get("call_id").String(),
				parsed.Get("tool_call_id").String(),
				parsed.Get("id").String(),
			)
			if strings.TrimSpace(callID) == "" {
				missingOutputCallID = true
			}
		}
		if isCodexToolCallContextItemType(itemType) {
			hasContext = true
		}
	}
	if !hasOutput {
		return ""
	}
	if missingOutputCallID {
		return "delta_tool_continuation_missing_call_id"
	}
	if !hasContext {
		return ""
	}
	if openAIWSRawItemsHaveToolCallContextForOutputs(delta) {
		return "delta_tool_continuation_self_contained"
	}
	// Mixed deltas can contain a function_call_output that belongs to the
	// previous response plus unrelated new tool-call context. The injected
	// previous_response_id is the required anchor for that output.
	return ""
}

// openAIWSRawItemsByteLen sums the raw byte length of a sequence of items (approx transmission size).
func openAIWSRawItemsByteLen(items []json.RawMessage) int {
	total := 0
	for _, it := range items {
		total += len(it)
	}
	return total
}
