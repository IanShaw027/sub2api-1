package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

const (
	maxCaptureBody = 16 << 20
	preludeSize    = 12
)

type accountRow struct {
	ID          int64
	Credentials map[string]any
}

type captureFrame struct {
	MessageType string         `json:"message_type"`
	EventType   string         `json:"event_type"`
	Payload     map[string]any `json:"payload"`
}

type nativeToolUse struct {
	ID    string
	Name  string
	Input any
}

type captureSummary struct {
	AccountHash         string         `json:"account_hash"`
	Region              string         `json:"region"`
	StatusCode          int            `json:"status_code"`
	RequestShape        map[string]any `json:"request_shape"`
	FrameCount          int            `json:"frame_count"`
	FrameEventTypes     []string       `json:"frame_event_types"`
	Frames              []captureFrame `json:"frames"`
	Classification      string         `json:"classification"`
	Continuation        *turnCapture   `json:"continuation,omitempty"`
	ContinuationSkipped string         `json:"continuation_skipped,omitempty"`
	Error               string         `json:"error,omitempty"`
}

type turnCapture struct {
	StatusCode      int            `json:"status_code"`
	RequestShape    map[string]any `json:"request_shape"`
	FrameCount      int            `json:"frame_count"`
	FrameEventTypes []string       `json:"frame_event_types"`
	Frames          []captureFrame `json:"frames"`
	Classification  string         `json:"classification"`
	Error           string         `json:"error,omitempty"`
}

func main() {
	var (
		model          = flag.String("model", "claude-sonnet-4.6", "Kiro modelId to send upstream")
		toolName       = flag.String("tool-name", "web_search", "Kiro native tool name candidate")
		toolType       = flag.String("tool-type", "web_search_20250305", "Anthropic source tool type to record in prompt text only")
		toolSchema     = flag.String("tool-schema", "", "tool input schema to send upstream: query, url, or empty to infer from tool name/type")
		query          = flag.String("query", "What is the current official Kiro CLI built-in web_search tool name? Answer briefly.", "search prompt")
		continueResult = flag.Bool("continue-tool-result", false, "after the first toolUseEvent, send a second Kiro request with a synthetic web_search toolResult")
		accountID      = flag.Int64("account-id", 0, "specific Kiro account id; 0 selects the first active schedulable OAuth account")
		timeout        = flag.Duration("timeout", 60*time.Second, "request timeout")
		allowNet       = flag.Bool("allow-network", false, "required to call Kiro upstream")
	)
	flag.Parse()

	if !*allowNet && os.Getenv("KIRO_NATIVE_CAPTURE_ALLOW_NETWORK") != "1" {
		fatalf("refusing network capture without -allow-network or KIRO_NATIVE_CAPTURE_ALLOW_NETWORK=1")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	db, err := openDB()
	if err != nil {
		fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	account, err := loadAccount(ctx, db, *accountID)
	if err != nil {
		fatalf("load account: %v", err)
	}
	accessToken := stringCredential(account.Credentials, "access_token")
	if strings.TrimSpace(accessToken) == "" {
		fatalf("selected account has no access_token; refresh it through the normal service path first")
	}

	requestBody := buildNativeToolRequest(account, *model, *toolName, *toolType, *toolSchema, *query)
	resp, respBody, err := callKiro(ctx, account, accessToken, requestBody)
	summary := captureSummary{
		AccountHash:  shortHash(strconv.FormatInt(account.ID, 10)),
		Region:       kiroRegion(account),
		RequestShape: redactAny(requestBody).(map[string]any),
	}
	if resp != nil {
		summary.StatusCode = resp.StatusCode
	}
	if err != nil {
		summary.Error = err.Error()
		printSummary(summary)
		os.Exit(1)
	}

	frames, parseErr := parseFrames(respBody)
	summary.FrameCount = len(frames)
	summary.Frames = frames
	summary.FrameEventTypes = frameEventTypes(frames)
	summary.Classification = classifyFrames(frames, respBody)
	if parseErr != nil {
		summary.Error = parseErr.Error()
	}
	if parseErr == nil && *continueResult {
		toolUse, ok := firstToolUse(frames)
		if !ok {
			summary.ContinuationSkipped = "first turn did not contain a toolUseEvent"
		} else {
			continuationRequest := buildNativeToolContinuationRequest(account, *model, requestBody, toolUse)
			contResp, contBody, contErr := callKiro(ctx, account, accessToken, continuationRequest)
			cont := &turnCapture{RequestShape: redactAny(continuationRequest).(map[string]any)}
			if contResp != nil {
				cont.StatusCode = contResp.StatusCode
			}
			if contErr != nil {
				cont.Error = contErr.Error()
			} else {
				contFrames, contParseErr := parseFrames(contBody)
				cont.FrameCount = len(contFrames)
				cont.Frames = contFrames
				cont.FrameEventTypes = frameEventTypes(contFrames)
				cont.Classification = classifyFrames(contFrames, contBody)
				if contParseErr != nil {
					cont.Error = contParseErr.Error()
				}
			}
			summary.Continuation = cont
		}
	}
	printSummary(summary)
	if parseErr != nil {
		os.Exit(1)
	}
	if summary.Continuation != nil && summary.Continuation.Error != "" {
		os.Exit(1)
	}
}

func openDB() (*sql.DB, error) {
	if dsn := strings.TrimSpace(os.Getenv("SUB2API_DATABASE_DSN")); dsn != "" {
		return sql.Open("postgres", dsn)
	}
	env, err := readDotEnv("../deploy/.env")
	if err != nil {
		env, err = readDotEnv("deploy/.env")
	}
	if err != nil {
		return nil, err
	}
	host := firstNonEmpty(env["DATABASE_HOST"], "127.0.0.1")
	port := firstNonEmpty(env["DATABASE_PORT"], "5432")
	user := firstNonEmpty(env["DATABASE_USER"], "sub2api")
	name := firstNonEmpty(env["DATABASE_NAME"], "sub2api")
	password := env["DATABASE_PASSWORD"]
	sslmode := firstNonEmpty(env["DATABASE_SSLMODE"], "disable")
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, name, sslmode)
	return sql.Open("postgres", dsn)
}

func readDotEnv(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	out := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"`)
		out[strings.TrimSpace(key)] = value
	}
	return out, scanner.Err()
}

func loadAccount(ctx context.Context, db *sql.DB, accountID int64) (*accountRow, error) {
	where := "platform='kiro' AND type='oauth' AND status='active' AND schedulable=true AND deleted_at IS NULL"
	args := []any{}
	if accountID > 0 {
		where += " AND id=$1"
		args = append(args, accountID)
	}
	query := "SELECT id, credentials FROM accounts WHERE " + where + " ORDER BY priority ASC, id DESC LIMIT 1"
	var id int64
	var raw []byte
	if err := db.QueryRowContext(ctx, query, args...).Scan(&id, &raw); err != nil {
		return nil, err
	}
	var credentials map[string]any
	if err := json.Unmarshal(raw, &credentials); err != nil {
		return nil, err
	}
	return &accountRow{ID: id, Credentials: credentials}, nil
}

func buildNativeToolRequest(account *accountRow, model, toolName, toolType, toolSchema, query string) map[string]any {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		toolName = "web_search"
	}
	schemaName := inferToolSchema(toolName, toolType, toolSchema)
	schema := nativeToolSchema(schemaName)
	action := "search"
	promptValue := query
	if schemaName == "url" {
		action = "fetch"
		if extractedURL := firstURL(query); extractedURL != "" {
			promptValue = extractedURL
		}
	}
	userInput := fmt.Sprintf("Use the native Kiro %s tool for this %s request, %s: %s", toolName, toolType, action, promptValue)
	payload := map[string]any{
		"conversationState": map[string]any{
			"agentContinuationId": uuid.NewString(),
			"agentTaskType":       "vibe",
			"chatTriggerType":     "MANUAL",
			"currentMessage": map[string]any{
				"userInputMessage": map[string]any{
					"userInputMessageContext": map[string]any{
						"tools": []any{
							map[string]any{
								"toolSpecification": map[string]any{
									"name":        toolName,
									"description": fmt.Sprintf("Native Kiro %s protocol discovery.", toolName),
									"inputSchema": map[string]any{"json": schema},
								},
							},
						},
					},
					"content": userInput,
					"modelId": model,
					"origin":  "AI_EDITOR",
				},
			},
			"conversationId": uuid.NewString(),
			"history":        []any{},
		},
	}
	if profileARN := stringCredential(account.Credentials, "profile_arn"); profileARN != "" {
		payload["profileArn"] = profileARN
	}
	return payload
}

func nativeToolSchema(schemaName string) map[string]any {
	field := "query"
	if schemaName == "url" {
		field = "url"
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			field: map[string]any{"type": "string"},
		},
		"required":             []string{field},
		"additionalProperties": false,
	}
}

func inferToolSchema(toolName, toolType, explicit string) string {
	explicit = strings.ToLower(strings.TrimSpace(explicit))
	if explicit == "query" || explicit == "url" {
		return explicit
	}
	lowerName := strings.ToLower(strings.TrimSpace(toolName))
	lowerType := strings.ToLower(strings.TrimSpace(toolType))
	if strings.HasPrefix(lowerName, "web_fetch") || strings.HasPrefix(lowerType, "web_fetch") {
		return "url"
	}
	return "query"
}

func firstURL(text string) string {
	for _, field := range strings.Fields(text) {
		trimmed := strings.Trim(field, ".,;:!?()[]{}<>\"'")
		if strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "http://") {
			return trimmed
		}
	}
	return ""
}

func buildNativeToolContinuationRequest(account *accountRow, model string, firstRequest map[string]any, toolUse nativeToolUse) map[string]any {
	firstState, _ := firstRequest["conversationState"].(map[string]any)
	firstCurrent, _ := firstState["currentMessage"].(map[string]any)
	firstUser, _ := firstCurrent["userInputMessage"].(map[string]any)
	firstContent := strings.TrimSpace(anyStringField(firstUser, "content"))
	firstContext, _ := firstUser["userInputMessageContext"].(map[string]any)
	firstTools, _ := firstContext["tools"].([]any)
	if firstContent == "" {
		firstContent = "Use the native Kiro web_search tool."
	}

	contentText := syntheticToolResultText(toolUse)
	payload := map[string]any{
		"conversationState": map[string]any{
			"agentContinuationId": uuid.NewString(),
			"agentTaskType":       "vibe",
			"chatTriggerType":     "MANUAL",
			"currentMessage": map[string]any{
				"userInputMessage": map[string]any{
					"userInputMessageContext": map[string]any{
						"tools": firstTools,
						"toolResults": []any{
							map[string]any{
								"toolUseId": toolUse.ID,
								"status":    "success",
								"content": []any{
									map[string]any{"text": contentText},
								},
							},
						},
					},
					"content": fmt.Sprintf("Use the %s tool result to answer in one concise sentence.", toolUse.Name),
					"modelId": model,
					"origin":  "AI_EDITOR",
				},
			},
			"conversationId": uuid.NewString(),
			"history": []any{
				map[string]any{
					"userInputMessage": map[string]any{
						"content": firstContent,
						"modelId": model,
						"origin":  "AI_EDITOR",
					},
				},
				map[string]any{
					"assistantResponseMessage": map[string]any{
						"content": "I will call the requested tools.",
						"toolUses": []any{
							map[string]any{
								"toolUseId": toolUse.ID,
								"name":      toolUse.Name,
								"input":     toolUse.Input,
							},
						},
					},
				},
			},
		},
	}
	if profileARN := stringCredential(account.Credentials, "profile_arn"); profileARN != "" {
		payload["profileArn"] = profileARN
	}
	return payload
}

func syntheticToolResultText(toolUse nativeToolUse) string {
	if strings.EqualFold(strings.TrimSpace(toolUse.Name), "web_fetch") {
		return "Synthetic protocol-capture fetch result.\nURL: https://example.invalid/kiro-native-web-fetch\nTitle: Kiro native web_fetch capture fixture\nContent: This fixed result verifies Kiro toolResults continuation replay."
	}
	return "Synthetic protocol-capture search result.\nURL: https://example.invalid/kiro-native-web-search\nTitle: Kiro native web_search capture fixture\nSnippet: This fixed result verifies Kiro toolResults continuation replay."
}

func callKiro(ctx context.Context, account *accountRow, accessToken string, payload map[string]any) (*http.Response, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	region := kiroRegion(account)
	url := fmt.Sprintf("https://q.%s.amazonaws.com/generateAssistantResponse", region)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	host := fmt.Sprintf("q.%s.amazonaws.com", region)
	machineID := kiro.GenerateMachineID(stringCredential(account.Credentials, "machine_id"), "", stringCredential(account.Credentials, "refresh_token"))
	kiroVersion := firstNonEmpty(os.Getenv("KIRO_CAPTURE_VERSION"), "0.10.0")
	systemVersion := firstNonEmpty(os.Getenv("KIRO_CAPTURE_SYSTEM_VERSION"), "darwin#24.6.0")
	nodeVersion := firstNonEmpty(os.Getenv("KIRO_CAPTURE_NODE_VERSION"), "22.21.1")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("host", host)
	req.Header.Set("x-amzn-codewhisperer-optout", "true")
	req.Header.Set("x-amzn-kiro-agent-mode", "vibe")
	req.Header.Set("x-amz-user-agent", fmt.Sprintf("aws-sdk-js/1.0.27 KiroIDE-%s-%s", kiroVersion, machineID))
	req.Header.Set("User-Agent", fmt.Sprintf("aws-sdk-js/1.0.27 ua/2.1 os/%s lang/js md/nodejs#%s api/codewhispererstreaming#1.0.27 m/E KiroIDE-%s-%s", systemVersion, nodeVersion, kiroVersion, machineID))
	req.Header.Set("amz-sdk-invocation-id", uuid.NewString())
	req.Header.Set("amz-sdk-request", "attempt=1; max=3")

	client := &http.Client{Timeout: 65 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxCaptureBody+1))
	if err != nil {
		return resp, nil, err
	}
	if len(respBody) > maxCaptureBody {
		return resp, nil, errors.New("response body exceeded capture limit")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, respBody, fmt.Errorf("kiro upstream status %d: %s", resp.StatusCode, string(respBody))
	}
	return resp, respBody, nil
}

func parseFrames(raw []byte) ([]captureFrame, error) {
	frames := []captureFrame{}
	for len(raw) > 0 {
		frame, consumed, ok, err := parseFrame(raw)
		if err != nil {
			return frames, err
		}
		if !ok {
			return frames, io.ErrUnexpectedEOF
		}
		frames = append(frames, frame)
		raw = raw[consumed:]
	}
	return frames, nil
}

func firstToolUse(frames []captureFrame) (nativeToolUse, bool) {
	byID := map[string]*nativeToolUse{}
	order := []string{}
	for _, frame := range frames {
		if frame.EventType != "toolUseEvent" {
			continue
		}
		id := strings.TrimSpace(anyStringField(frame.Payload, "toolUseId"))
		name := strings.TrimSpace(anyStringField(frame.Payload, "name"))
		if id == "" || name == "" {
			continue
		}
		state := byID[id]
		if state == nil {
			state = &nativeToolUse{ID: id, Name: name, Input: ""}
			byID[id] = state
			order = append(order, id)
		}
		if state.Name == "" {
			state.Name = name
		}
		rawInput := strings.TrimSpace(anyStringField(frame.Payload, "input"))
		if rawInput != "" {
			state.Input = anyString(state.Input) + rawInput
		}
		if stop, _ := frame.Payload["stop"].(bool); stop {
			return finalizeNativeToolUse(*state), true
		}
	}
	if len(order) == 0 {
		return nativeToolUse{}, false
	}
	return finalizeNativeToolUse(*byID[order[0]]), true
}

func finalizeNativeToolUse(toolUse nativeToolUse) nativeToolUse {
	rawInput := strings.TrimSpace(anyString(toolUse.Input))
	if rawInput == "" {
		toolUse.Input = map[string]any{}
		return toolUse
	}
	var parsed any
	if err := json.Unmarshal([]byte(rawInput), &parsed); err == nil && parsed != nil {
		toolUse.Input = parsed
	} else {
		toolUse.Input = rawInput
	}
	return toolUse
}

func anyString(value any) string {
	text, _ := value.(string)
	return text
}

func parseFrame(buffer []byte) (captureFrame, int, bool, error) {
	if len(buffer) < preludeSize {
		return captureFrame{}, 0, false, nil
	}
	totalLength := int(binary.BigEndian.Uint32(buffer[0:4]))
	headerLength := int(binary.BigEndian.Uint32(buffer[4:8]))
	preludeCRC := binary.BigEndian.Uint32(buffer[8:12])
	if totalLength < preludeSize+4 || totalLength > maxCaptureBody {
		return captureFrame{}, 0, false, fmt.Errorf("invalid frame length %d", totalLength)
	}
	if preludeSize+headerLength > totalLength-4 {
		return captureFrame{}, 0, false, fmt.Errorf("invalid header length %d", headerLength)
	}
	if len(buffer) < totalLength {
		return captureFrame{}, 0, false, nil
	}
	if crc32.ChecksumIEEE(buffer[:8]) != preludeCRC {
		return captureFrame{}, 0, false, errors.New("prelude crc mismatch")
	}
	messageCRC := binary.BigEndian.Uint32(buffer[totalLength-4 : totalLength])
	if crc32.ChecksumIEEE(buffer[:totalLength-4]) != messageCRC {
		return captureFrame{}, 0, false, errors.New("message crc mismatch")
	}
	headers, err := parseHeaders(buffer[preludeSize : preludeSize+headerLength])
	if err != nil {
		return captureFrame{}, 0, false, err
	}
	payload := map[string]any{}
	payloadBytes := buffer[preludeSize+headerLength : totalLength-4]
	if len(payloadBytes) > 0 {
		if err := json.Unmarshal(payloadBytes, &payload); err != nil {
			return captureFrame{}, 0, false, err
		}
	}
	frame := captureFrame{
		MessageType: firstNonEmpty(headers[":message-type"], "event"),
		EventType:   headers[":event-type"],
		Payload:     redactAny(payload).(map[string]any),
	}
	return frame, totalLength, true, nil
}

func parseHeaders(data []byte) (map[string]string, error) {
	headers := map[string]string{}
	for offset := 0; offset < len(data); {
		nameLen := int(data[offset])
		offset++
		if offset+nameLen > len(data) {
			return nil, errors.New("header name out of bounds")
		}
		name := string(data[offset : offset+nameLen])
		offset += nameLen
		if offset >= len(data) {
			return nil, errors.New("header missing type")
		}
		valueType := data[offset]
		offset++
		switch valueType {
		case 7:
			if offset+2 > len(data) {
				return nil, errors.New("header string length missing")
			}
			valueLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
			offset += 2
			if offset+valueLen > len(data) {
				return nil, errors.New("header string out of bounds")
			}
			headers[name] = string(data[offset : offset+valueLen])
			offset += valueLen
		case 0, 1:
			headers[name] = strconv.FormatBool(valueType == 0)
		case 2:
			offset++
		case 3:
			offset += 2
		case 4:
			offset += 4
		case 5, 8:
			offset += 8
		case 6:
			if offset+2 > len(data) {
				return nil, errors.New("header bytes length missing")
			}
			valueLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
			offset += 2 + valueLen
		case 9:
			offset += 16
		default:
			return nil, fmt.Errorf("unknown header type %d", valueType)
		}
		if offset > len(data) {
			return nil, errors.New("header parse overflow")
		}
	}
	return headers, nil
}

func classifyFrames(frames []captureFrame, raw []byte) string {
	if len(frames) == 0 && len(raw) > 0 {
		return "unparsed"
	}
	hasToolUse := false
	hasAssistant := false
	for _, frame := range frames {
		if frame.EventType == "toolUseEvent" {
			if strings.Contains(strings.ToLower(toJSON(frame.Payload)), "web") || strings.Contains(strings.ToLower(toJSON(frame.Payload)), "search") {
				hasToolUse = true
			}
		}
		if frame.EventType == "assistantResponseEvent" {
			hasAssistant = true
		}
	}
	switch {
	case hasToolUse:
		return "native_client_tool_turn"
	case hasAssistant:
		return "unstructured_or_no_tool_use"
	default:
		return "unsupported_or_error"
	}
}

func frameEventTypes(frames []captureFrame) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, frame := range frames {
		key := frame.MessageType + ":" + frame.EventType
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}

func redactAny(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			lower := strings.ToLower(key)
			switch {
			case strings.Contains(lower, "token"), strings.Contains(lower, "secret"), strings.Contains(lower, "authorization"):
				out[key] = "[redacted]"
			case strings.Contains(lower, "arn"):
				out[key] = "[redacted-arn]"
			case strings.Contains(lower, "conversationid"), strings.Contains(lower, "continuationid"), strings.Contains(lower, "requestid"), strings.EqualFold(key, "id"):
				out[key] = "[redacted-id]"
			default:
				out[key] = redactAny(v[key])
			}
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, redactAny(item))
		}
		return out
	case string:
		if len(v) > 240 {
			return v[:240] + "...[truncated]"
		}
		return v
	default:
		return v
	}
}

func printSummary(summary captureSummary) {
	encoded, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		fatalf("marshal summary: %v", err)
	}
	fmt.Println(string(encoded))
}

func kiroRegion(account *accountRow) string {
	for _, key := range []string{"api_region", "auth_region", "region"} {
		if value := stringCredential(account.Credentials, key); value != "" {
			return value
		}
	}
	return "us-east-1"
}

func stringCredential(credentials map[string]any, key string) string {
	value, _ := credentials[key]
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return strings.TrimSpace(v.String())
	case float64:
		return strconv.FormatInt(int64(v), 10)
	default:
		return ""
	}
}

func anyStringField(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key]
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return strings.TrimSpace(v.String())
	case float64:
		return strconv.FormatInt(int64(v), 10)
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func shortHash(value string) string {
	sum := crc32.ChecksumIEEE([]byte(value))
	return fmt.Sprintf("%08x", sum)
}

func toJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
