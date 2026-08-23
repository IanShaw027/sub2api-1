package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newCyberBlockTestCtx(headers map[string]string, body string) (*gin.Context, []byte) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/openai/v1/responses", strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c.Request = req
	return c, []byte(body)
}

func TestCyberSessionExplicitBlockKey(t *testing.T) {
	c1, b1 := newCyberBlockTestCtx(map[string]string{"session_id": "sess-abc"}, `{}`)
	k1 := CyberSessionExplicitBlockKey(101, c1, b1)
	require.NotEmpty(t, k1)

	// Same session, different apiKey → different key (isolation).
	c2, b2 := newCyberBlockTestCtx(map[string]string{"session_id": "sess-abc"}, `{}`)
	require.Equal(t, k1, CyberSessionExplicitBlockKey(101, c2, b2))
	require.NotEqual(t, k1, CyberSessionExplicitBlockKey(202, c2, b2))

	// Same session + same apiKey → stable key.
	c3, b3 := newCyberBlockTestCtx(map[string]string{"session_id": "sess-abc"}, `{}`)
	require.Equal(t, k1, CyberSessionExplicitBlockKey(101, c3, b3))

	// prompt_cache_key in body counts as explicit.
	c4, b4 := newCyberBlockTestCtx(nil, `{"prompt_cache_key":"pck-1"}`)
	require.NotEmpty(t, CyberSessionExplicitBlockKey(101, c4, b4))

	// No explicit signal → empty key → caller must skip blocking entirely.
	c5, b5 := newCyberBlockTestCtx(nil, `{"input":"hello world"}`)
	require.Empty(t, CyberSessionExplicitBlockKey(101, c5, b5))

	// conversation_id header counts as explicit; key is stable and non-empty.
	c6, b6 := newCyberBlockTestCtx(map[string]string{"conversation_id": "conv-xyz"}, `{}`)
	k6 := CyberSessionExplicitBlockKey(101, c6, b6)
	require.NotEmpty(t, k6)
	c6b, b6b := newCyberBlockTestCtx(map[string]string{"conversation_id": "conv-xyz"}, `{}`)
	require.Equal(t, k6, CyberSessionExplicitBlockKey(101, c6b, b6b), "conversation_id key must be stable")
}

func TestCyberSessionProductionKeyDerivation(t *testing.T) {
	c1, b1 := newCyberBlockTestCtx(map[string]string{"session_id": "sess-abc"}, `{}`)
	explicitKey := CyberSessionExplicitBlockKey(101, c1, b1)
	require.NotEmpty(t, explicitKey)
	c2, b2 := newCyberBlockTestCtx(map[string]string{"session_id": "sess-abc"}, `{}`)
	require.Equal(t, explicitKey, CyberSessionExplicitBlockKey(101, c2, b2))
	require.NotEqual(t, explicitKey, CyberSessionExplicitBlockKey(202, c2, b2))

	firstTurn := []byte(`{"input":"hello world"}`)
	firstKeys := CyberSessionTranscriptBlockKeys(101, firstTurn)
	require.Len(t, firstKeys, 1, "first turn writes only the exact request key")
	require.Equal(t, firstKeys, CyberSessionTranscriptBlockKeys(101, []byte(`{"input":"hello world","previous_response_id":"resp_random"}`)))
	require.NotEqual(t, firstKeys, CyberSessionTranscriptBlockKeys(101, []byte(`{"input":"different content"}`)))
	require.Empty(t, CyberSessionTranscriptBlockKeys(101, []byte(`{}`)))

	const hitBody = `{"model":"gpt-5.5","messages":[{"role":"system","content":"policy"},{"role":"user","content":"setup"},{"role":"assistant","content":"ready"},{"role":"user","content":"triggering question"}]}`
	hitBytes := []byte(hitBody)
	hitWriteKeys := CyberSessionTranscriptBlockKeys(664, hitBytes)
	require.Len(t, hitWriteKeys, 2)
	hitKey := hitWriteKeys[1]
	require.NotEmpty(t, hitKey)

	continuationBody := []byte(`{"model":"gpt-5.5","messages":[{"role":"system","content":"policy"},{"role":"user","content":"setup"},{"role":"assistant","content":"ready"},{"role":"user","content":"triggering question"},{"role":"assistant","content":"blocked"},{"role":"user","content":"follow-up"}]}`)
	require.Contains(t, CyberSessionTranscriptLookupKeys(664, continuationBody), hitKey,
		"continuation must contain the full context before the triggering turn")

	branchBody := []byte(`{"model":"gpt-5.5","messages":[{"role":"system","content":"policy"},{"role":"user","content":"setup"},{"role":"assistant","content":"ready"},{"role":"user","content":"different new-session question"}]}`)
	require.Contains(t, CyberSessionTranscriptLookupKeys(664, branchBody), hitKey,
		"changing the triggering question still belongs to the same prior conversation")

	changedHistoryBody := []byte(`{"model":"gpt-5.5","messages":[{"role":"system","content":"policy"},{"role":"user","content":"setup"},{"role":"assistant","content":"different earlier answer"},{"role":"user","content":"triggering question"}]}`)
	require.NotContains(t, CyberSessionTranscriptLookupKeys(664, changedHistoryBody), hitKey,
		"changing prior conversation history must not match")

	changedConfigBody := []byte(`{"model":"gpt-5.6-sol","tools":[{"type":"function","name":"new_tool"}],"messages":[{"role":"system","content":"policy"},{"role":"user","content":"setup"},{"role":"assistant","content":"ready"},{"role":"user","content":"different question"}]}`)
	require.Contains(t, CyberSessionTranscriptLookupKeys(664, changedConfigBody), hitKey,
		"model and tool configuration changes must not rotate conversation identity")

	statelessBody := []byte(`{"model":"gpt-5.5","messages":[{"role":"user","content":"follow-up only"}]}`)
	require.NotContains(t, CyberSessionTranscriptLookupKeys(664, statelessBody), hitKey,
		"stateless new content must not be treated as the old conversation")
	require.Contains(t, CyberSessionTranscriptLookupKeys(664, hitBytes), hitWriteKeys[0],
		"an exact retry of the triggering request must be blocked")
}

func TestCyberTranscriptBlockLookupKeys_ToolResultTrigger(t *testing.T) {
	const hitBody = `{"model":"gpt-5.5","input":[{"role":"user","content":[{"type":"input_text","text":"setup"}]},{"role":"assistant","content":[{"type":"output_text","text":"ready"}]},{"role":"user","content":[{"type":"input_text","text":"inspect target"}]},{"type":"function_call","name":"scan","call_id":"call_1","arguments":"{\"target\":\"lab\"}"},{"type":"function_call_output","call_id":"call_1","output":"sensitive result"}]}`
	hitBytes := []byte(hitBody)
	hitKey := CyberSessionTranscriptBlockKeys(664, hitBytes)[1]
	require.NotEmpty(t, hitKey)

	continuedBody := []byte(`{"model":"gpt-5.5","input":[{"role":"user","content":[{"type":"input_text","text":"setup"}]},{"role":"assistant","content":[{"type":"output_text","text":"ready"}]},{"role":"user","content":[{"type":"input_text","text":"inspect target"}]},{"type":"function_call","name":"scan","call_id":"call_1","arguments":"{\"target\":\"lab\"}"},{"type":"function_call_output","call_id":"call_1","output":"sensitive result"},{"role":"user","content":[{"type":"input_text","text":"continue"}]}]}`)
	require.Contains(t, CyberSessionTranscriptLookupKeys(664, continuedBody), hitKey)

	changedResultBody := []byte(`{"model":"gpt-5.5","input":[{"role":"user","content":[{"type":"input_text","text":"setup"}]},{"role":"assistant","content":[{"type":"output_text","text":"ready"}]},{"role":"user","content":[{"type":"input_text","text":"inspect target differently"}]},{"type":"function_call","name":"scan","call_id":"call_1","arguments":"{\"target\":\"lab\"}"},{"type":"function_call_output","call_id":"call_1","output":"different result"},{"role":"user","content":[{"type":"input_text","text":"continue"}]}]}`)
	require.Contains(t, CyberSessionTranscriptLookupKeys(664, changedResultBody), hitKey,
		"changing the triggering turn and its tool result remains the same prior conversation")
}

func TestCyberTranscriptBlockKeys_FixedClientPrefixNeedsModelOutput(t *testing.T) {
	const first = `{"instructions":"shared","input":[{"role":"user","content":[{"type":"input_text","text":"<environment_context>fixed</environment_context>"}]},{"role":"user","content":[{"type":"input_text","text":"question one"}]}]}`
	const second = `{"instructions":"shared","input":[{"role":"user","content":[{"type":"input_text","text":"<environment_context>fixed</environment_context>"}]},{"role":"user","content":[{"type":"input_text","text":"unrelated question"}]}]}`
	firstKeys := CyberSessionTranscriptBlockKeys(77, []byte(first))
	secondKeys := CyberSessionTranscriptBlockKeys(77, []byte(second))
	require.Len(t, firstKeys, 1, "fixed user-only prefix must not produce a broad context key")
	require.Len(t, secondKeys, 1)
	require.NotEqual(t, firstKeys[0], secondKeys[0], "exact keys must include the differing question")
}

func TestCyberTranscriptBlockKeys_WebSocketResponseCreate(t *testing.T) {
	body := []byte(`{"type":"response.create","response":{"prompt_cache_key":"ws-session","instructions":"policy","input":[{"role":"user","content":"setup"},{"role":"assistant","content":"ready"},{"role":"user","content":"trigger"}]}}`)
	c, _ := newCyberBlockTestCtx(nil, string(body))
	require.NotEmpty(t, CyberSessionExplicitBlockKey(88, c, body))
	require.Equal(t, "ws-session", explicitOpenAIRequestSessionID(c, body),
		"sticky scheduling and Cyber identity must read the same nested session ID")
	require.Len(t, CyberSessionTranscriptBlockKeys(88, body), 2)
	require.Len(t, CyberSessionTranscriptLookupKeys(88, body), 3)
}

func TestOpenAIRequestPayloadViewOnlyUnwrapsResponseEvents(t *testing.T) {
	ordinary := []byte(`{"prompt_cache_key":"root-session","response":{"prompt_cache_key":"unrelated-nested"},"input":"root input"}`)
	c, _ := newCyberBlockTestCtx(nil, string(ordinary))
	require.Equal(t, "root-session", explicitOpenAISessionID(c, ordinary))
	require.Equal(t, "root-session", explicitOpenAIRequestSessionID(c, ordinary))
	require.Len(t, CyberSessionTranscriptBlockKeys(88, ordinary), 1)

	unrelatedEvent := []byte(`{"type":"request.created","prompt_cache_key":"event-root","response":{"prompt_cache_key":"nested"}}`)
	require.Equal(t, "event-root", explicitOpenAISessionID(c, unrelatedEvent))
}

func TestCyberTranscriptToolResultDoesNotStartUserTurn(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"setup"},{"role":"assistant","content":[{"type":"tool_use","name":"scan","input":{}}]},{"role":"user","content":[{"type":"tool_result","content":"result"}]},{"role":"user","content":"follow up"}]}`)
	derived := deriveOpenAICyberTranscriptBlockKeys(99, body)
	require.Len(t, derived.lookupKeys, 4)
	require.Equal(t, derived.lookupKeys[2], derived.preLatestUserKey,
		"Anthropic tool_result must remain part of prior context rather than starting a user turn")
}

func TestCyberTranscriptBlockKeysRequireModelGeneratedHistory(t *testing.T) {
	first := []byte(`{"instructions":"shared","input":[{"role":"user","content":"fixed environment"},{"role":"user","content":"question one"}]}`)
	second := []byte(`{"instructions":"shared","input":[{"role":"user","content":"fixed environment"},{"role":"user","content":"question two"}]}`)
	firstKeys := CyberSessionTranscriptBlockKeys(77, first)
	secondKeys := CyberSessionTranscriptBlockKeys(77, second)
	require.Len(t, firstKeys, 1)
	require.Len(t, secondKeys, 1)
	require.NotEqual(t, firstKeys[0], secondKeys[0])

	hit := []byte(`{"messages":[{"role":"user","content":"setup"},{"role":"assistant","content":"ready"},{"role":"user","content":"trigger"}]}`)
	continuation := []byte(`{"messages":[{"role":"user","content":"setup"},{"role":"assistant","content":"ready"},{"role":"user","content":"different trigger"},{"role":"assistant","content":"blocked"},{"role":"user","content":"continue"}]}`)
	hitKeys := CyberSessionTranscriptBlockKeys(77, hit)
	require.Len(t, hitKeys, 2)
	require.Contains(t, CyberSessionTranscriptLookupKeys(77, continuation), hitKeys[1])
}

func TestCyberTranscriptLookupKeysAreBoundedAndKeepNewestOrder(t *testing.T) {
	messages := make([]map[string]string, maxOpenAICyberTranscriptLookupKeys+44)
	for i := range messages {
		messages[i] = map[string]string{"role": "user", "content": "message-" + strconv.Itoa(i)}
	}
	body, err := json.Marshal(map[string]any{"messages": messages})
	require.NoError(t, err)

	keys := CyberSessionTranscriptLookupKeys(77, body)
	require.Len(t, keys, maxOpenAICyberTranscriptLookupKeys)

	firstRetainedBody, err := json.Marshal(map[string]any{"messages": messages[:45]})
	require.NoError(t, err)
	firstRetainedPrefix := CyberSessionTranscriptLookupKeys(77, firstRetainedBody)
	require.Equal(t, firstRetainedPrefix[len(firstRetainedPrefix)-1], keys[0])

	fullKey := CyberSessionTranscriptBlockKeys(77, body)[0]
	require.Equal(t, fullKey, keys[len(keys)-1])
}

// --- fakes ---

type fakeCyberBlockStore struct {
	blocked    map[string]bool
	scopes     map[string]bool
	scopeReads int
	findReads  int
	findCalls  int
}

var _ CyberSessionBlockStore = (*fakeCyberBlockStore)(nil)

func (f *fakeCyberBlockStore) SetCyberSessionBlocked(_ context.Context, scopeKey string, keys []string, _ time.Duration) error {
	if f.blocked == nil {
		f.blocked = map[string]bool{}
	}
	for _, key := range keys {
		f.blocked[key] = true
	}
	if scopeKey != "" {
		if f.scopes == nil {
			f.scopes = map[string]bool{}
		}
		f.scopes[scopeKey] = true
	}
	return nil
}

func (f *fakeCyberBlockStore) IsCyberSessionScopeActive(_ context.Context, scopeKey string) (bool, error) {
	f.scopeReads++
	return f.scopes[scopeKey], nil
}

func (f *fakeCyberBlockStore) FindCyberSessionBlocked(_ context.Context, keys []string) (string, error) {
	f.findReads++
	f.findCalls++
	for _, key := range keys {
		if f.blocked[key] {
			return key, nil
		}
	}
	return "", nil
}

func TestCyberSessionScopeKeyNormalizesUAAndIsolatesSource(t *testing.T) {
	base := CyberSessionScopeKey(7, "203.0.113.10", "Codex CLI 1.2.3")
	require.NotEmpty(t, base)
	require.Equal(t, base, CyberSessionScopeKey(7, "203.0.113.10", "Codex CLI 1.2.4"))
	require.NotEqual(t, base, CyberSessionScopeKey(8, "203.0.113.10", "Codex CLI 1.2.3"))
	require.NotEqual(t, base, CyberSessionScopeKey(7, "203.0.113.11", "Codex CLI 1.2.3"))
	require.NotEqual(t, base, CyberSessionScopeKey(7, "203.0.113.10", "OtherClient/1.2.3"))
}

// fakeSettingRepo is a minimal SettingRepository stub for unit tests.
// Only GetValue is exercised by GetCyberSessionBlockRuntime; all other methods
// panic so accidental calls are caught immediately.
type fakeSettingRepo struct {
	vals map[string]string
}

func (r *fakeSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	v, ok := r.vals[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return v, nil
}
func (r *fakeSettingRepo) Get(_ context.Context, _ string) (*Setting, error) {
	panic("fakeSettingRepo.Get not implemented")
}
func (r *fakeSettingRepo) Set(_ context.Context, _, _ string) error {
	panic("fakeSettingRepo.Set not implemented")
}
func (r *fakeSettingRepo) GetMultiple(_ context.Context, _ []string) (map[string]string, error) {
	panic("fakeSettingRepo.GetMultiple not implemented")
}
func (r *fakeSettingRepo) SetMultiple(_ context.Context, _ map[string]string) error {
	panic("fakeSettingRepo.SetMultiple not implemented")
}
func (r *fakeSettingRepo) GetAll(_ context.Context) (map[string]string, error) {
	panic("fakeSettingRepo.GetAll not implemented")
}
func (r *fakeSettingRepo) Delete(_ context.Context, _ string) error {
	panic("fakeSettingRepo.Delete not implemented")
}

var _ SettingRepository = (*fakeSettingRepo)(nil)

// comboCacheAndStore implements both GatewayCache (no-op stubs) and
// CyberSessionBlockStore (delegates to fakeCyberBlockStore) so it can be
// injected as s.cache and successfully type-asserted to CyberSessionBlockStore.
type comboCacheAndStore struct {
	store fakeCyberBlockStore
}

var _ GatewayCache = (*comboCacheAndStore)(nil)
var _ CyberSessionBlockStore = (*comboCacheAndStore)(nil)

func (c *comboCacheAndStore) GetSessionAccountID(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, errors.New("stub")
}
func (c *comboCacheAndStore) SetSessionAccountID(_ context.Context, _ int64, _ string, _ int64, _ time.Duration) error {
	return nil
}
func (c *comboCacheAndStore) RefreshSessionTTL(_ context.Context, _ int64, _ string, _ time.Duration) error {
	return nil
}
func (c *comboCacheAndStore) DeleteSessionAccountID(_ context.Context, _ int64, _ string) error {
	return nil
}

func (c *comboCacheAndStore) SetGrokVideoPendingBilling(_ context.Context, _ string, _ []byte, _ time.Duration) error {
	return nil
}
func (c *comboCacheAndStore) GetGrokVideoPendingBilling(_ context.Context, _ string) ([]byte, error) {
	return nil, nil
}
func (c *comboCacheAndStore) ClaimGrokVideoBilled(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}

func (c *comboCacheAndStore) ReleaseGrokVideoBilled(_ context.Context, _ string) error {
	return nil
}

func (c *comboCacheAndStore) SetReasoningContent(_ context.Context, _ string, _ string, _ time.Duration) error {
	return nil
}
func (c *comboCacheAndStore) GetReasoningContent(_ context.Context, _ string) (string, error) {
	return "", ErrReasoningContentNotFound
}

func (c *comboCacheAndStore) SetCyberSessionBlocked(ctx context.Context, scopeKey string, keys []string, ttl time.Duration) error {
	return c.store.SetCyberSessionBlocked(ctx, scopeKey, keys, ttl)
}
func (c *comboCacheAndStore) IsCyberSessionScopeActive(ctx context.Context, scopeKey string) (bool, error) {
	return c.store.IsCyberSessionScopeActive(ctx, scopeKey)
}
func (c *comboCacheAndStore) FindCyberSessionBlocked(ctx context.Context, keys []string) (string, error) {
	return c.store.FindCyberSessionBlocked(ctx, keys)
}

// --- tests ---

func TestMarkCyberSessionBlocked_NilServiceDoesNotPanic(t *testing.T) {
	var nilSvc *OpenAIGatewayService
	require.NotPanics(t, func() { nilSvc.MarkCyberSessionBlocked(context.Background(), "scope", []string{"k"}) })
	require.Empty(t, nilSvc.FindCyberSessionBlockedForRequest(context.Background(), 1, nil, nil, "", ""))
}

func TestFindCyberSessionBlocked_EmptyAndNilService(t *testing.T) {
	var nilSvc *OpenAIGatewayService
	require.Empty(t, nilSvc.FindCyberSessionBlockedForRequest(context.Background(), 1, nil, nil, "", ""))
	require.NotPanics(t, func() { nilSvc.MarkCyberSessionBlocked(context.Background(), "", []string{"k"}) })

	svc := &OpenAIGatewayService{}
	require.Empty(t, svc.FindCyberSessionBlockedForRequest(context.Background(), 1, nil, nil, "", ""))
}

func TestMarkCyberSessionBlocked_WritesScopeAndAllKeys(t *testing.T) {
	settingSvc := &SettingService{
		settingRepo: &fakeSettingRepo{
			vals: map[string]string{
				SettingKeyCyberSessionBlockEnabled:    "true",
				SettingKeyCyberSessionBlockTTLSeconds: "60",
			},
		},
	}

	combo := &comboCacheAndStore{}
	svc := &OpenAIGatewayService{
		cache:          combo,
		settingService: settingSvc,
	}

	ctx := context.Background()
	const scopeKey = "scope-deadbeef"
	svc.MarkCyberSessionBlocked(ctx, scopeKey, []string{"exact-key", "context-key"})
	require.True(t, combo.store.scopes[scopeKey])
	require.True(t, combo.store.blocked["exact-key"])
	require.True(t, combo.store.blocked["context-key"])

	const testKey = "deadbeef1234"
	c, body := newCyberBlockTestCtx(map[string]string{"session_id": "sess-roundtrip"}, `{}`)
	explicitKey := CyberSessionExplicitBlockKey(1, c, body)
	require.Empty(t, svc.FindCyberSessionBlockedForRequest(ctx, 1, c, body, "203.0.113.1", "client/1.0"))
	svc.MarkCyberSessionBlocked(ctx, "", []string{explicitKey, testKey})
	require.Equal(t, explicitKey, svc.FindCyberSessionBlockedForRequest(ctx, 1, c, body, "203.0.113.1", "client/1.0"))
}

func TestFindCyberSessionBlockedForRequestUsesScopeBeforeTranscriptLookup(t *testing.T) {
	settingSvc := &SettingService{settingRepo: &fakeSettingRepo{vals: map[string]string{
		SettingKeyCyberSessionBlockEnabled:    "true",
		SettingKeyCyberSessionBlockTTLSeconds: "60",
	}}}
	combo := &comboCacheAndStore{}
	svc := &OpenAIGatewayService{cache: combo, settingService: settingSvc}
	ctx := context.Background()

	const hitBody = `{"messages":[{"role":"user","content":"setup"},{"role":"assistant","content":"ready"},{"role":"user","content":"trigger"}]}`
	const nextBody = `{"messages":[{"role":"user","content":"setup"},{"role":"assistant","content":"ready"},{"role":"user","content":"different trigger"},{"role":"assistant","content":"blocked"},{"role":"user","content":"continue"}]}`
	_, hitBytes := newCyberBlockTestCtx(nil, hitBody)
	nextCtx, nextBytes := newCyberBlockTestCtx(nil, nextBody)
	scopeKey := CyberSessionScopeKey(9, "203.0.113.20", "client/1.0.0")
	blockKey := CyberSessionTranscriptBlockKeys(9, hitBytes)[1]
	require.NotEmpty(t, blockKey)

	// No source scope means no transcript MGET.
	require.Empty(t, svc.FindCyberSessionBlockedForRequest(ctx, 9, nextCtx, nextBytes, "203.0.113.20", "client/1.0.0"))
	require.Equal(t, 1, combo.store.scopeReads)
	require.Zero(t, combo.store.findReads)

	svc.MarkCyberSessionBlocked(ctx, scopeKey, []string{blockKey})
	require.Equal(t, blockKey, svc.FindCyberSessionBlockedForRequest(ctx, 9, nextCtx, nextBytes, "203.0.113.20", "client/1.0.1"))
	require.Equal(t, 2, combo.store.scopeReads)
	require.Equal(t, 1, combo.store.findReads)

	// Explicit sessions bypass the coarse source scope entirely.
	explicitCtx, explicitBody := newCyberBlockTestCtx(map[string]string{"session_id": "sess-direct"}, `{}`)
	explicitKey := CyberSessionExplicitBlockKey(9, explicitCtx, explicitBody)
	svc.MarkCyberSessionBlocked(ctx, "", []string{explicitKey})
	beforeScopeReads := combo.store.scopeReads
	require.Equal(t, explicitKey, svc.FindCyberSessionBlockedForRequest(ctx, 9, explicitCtx, explicitBody, "198.51.100.2", "other/9.0"))
	require.Equal(t, beforeScopeReads, combo.store.scopeReads)

	// A rotating explicit signal misses first, then falls back through the
	// active source scope to the transcript block.
	randomCtx, randomBody := newCyberBlockTestCtx(map[string]string{"session_id": "random-each-request"}, nextBody)
	randomCtx.Request.Header.Set("User-Agent", "client/1.0.2")
	randomCtx.Request.RemoteAddr = "203.0.113.20:1234"
	beforeFindReads := combo.store.findReads
	require.Equal(t, blockKey, svc.FindCyberSessionBlockedForRequest(ctx, 9, randomCtx, randomBody, "203.0.113.20", "client/1.0.2"))
	require.Equal(t, beforeFindReads+2, combo.store.findReads, "explicit miss and transcript MGET are separate exact lookups")
}

func TestFindCyberSessionBlockedForRequestFailsClosedOnScopedTranscriptOverflow(t *testing.T) {
	settingSvc := &SettingService{settingRepo: &fakeSettingRepo{vals: map[string]string{
		SettingKeyCyberSessionBlockEnabled:    "true",
		SettingKeyCyberSessionBlockTTLSeconds: "60",
	}}}
	combo := &comboCacheAndStore{}
	svc := &OpenAIGatewayService{cache: combo, settingService: settingSvc}
	ctx := context.Background()
	const apiKeyID = int64(9)
	const clientIP = "203.0.113.20"
	const userAgent = "Codex CLI 1.2.3"

	messages := make([]map[string]string, maxOpenAICyberTranscriptLookupKeys+1)
	for i := range messages {
		messages[i] = map[string]string{"role": "user", "content": "message-" + strconv.Itoa(i)}
	}
	body, err := json.Marshal(map[string]any{"messages": messages})
	require.NoError(t, err)
	c, _ := newCyberBlockTestCtx(nil, string(body))
	require.Empty(t, svc.FindCyberSessionBlockedForRequest(ctx, apiKeyID, c, body, clientIP, userAgent),
		"overflow alone must not bypass the scope gate")
	combo.store.scopes = map[string]bool{CyberSessionScopeKey(apiKeyID, clientIP, userAgent): true}

	require.Equal(t, cyberSessionTranscriptLookupOverflowBlockKey,
		svc.FindCyberSessionBlockedForRequest(ctx, apiKeyID, c, body, clientIP, userAgent))
	require.Zero(t, combo.store.findCalls, "overflow must not issue an unbounded Redis lookup")
}
