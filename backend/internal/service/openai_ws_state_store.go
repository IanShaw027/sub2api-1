package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	openAIWSResponseAccountCachePrefix = "openai:response:"
	openAIWSStateStoreCleanupInterval  = time.Minute
	openAIWSStateStoreCleanupMaxPerMap = 512
	openAIWSStateStoreMaxEntriesPerMap = 65536
	openAIWSStateStoreRedisTimeout     = 3 * time.Second
	openAIWSConnEvictDiagnosticTTL     = time.Hour
	openAIWSSessionContextCacheVersion = 1
	openAIWSSessionContextCachePrefix  = "wsctx:"
)

type openAIWSAccountBinding struct {
	accountID int64
	expiresAt time.Time
}

type openAIWSConnBinding struct {
	connID    string
	expiresAt time.Time
}

type openAIWSTurnStateBinding struct {
	turnState string
	expiresAt time.Time
}

type openAIWSSessionConnBinding struct {
	connID    string
	expiresAt time.Time
}

// openAIWSSessionContextValue 是 strict-delta shadow 阶段的会话上下文指纹。
// 仅保存 canonical 哈希与必要 id/count，绝不保存请求/响应原文（隐私 + 内存有界）。
type openAIWSSessionContextValue struct {
	accountID               int64
	connID                  string
	lastResponseID          string
	materializedHashes      [][32]byte // canonical hash of (input_N items ++ raw output_N items)
	materializedShapes      []string   // no raw values; JSON path/type shape for mismatch diagnostics
	materializedCount       int        // = len(input_N) + len(raw output_N)
	inputCount              int        // = len(input_N); 用于区分 break 落在 input 前缀还是 output 边界
	inputOnlyContext        bool       // raw output 未捕获时的降级上下文；后续需裁掉历史 replay 输出项
	nonInputHash            [32]byte
	nonInputFields          map[string]openAIWSNonInputFieldFingerprint // no raw values; top-level non-input field fingerprints for mismatch diagnostics
	rawVsClientVisibleEqual bool
}

type openAIWSSessionContextBinding struct {
	value     openAIWSSessionContextValue
	expiresAt time.Time
}

type openAIWSSessionContextCacheValue struct {
	Version                 int                                               `json:"version"`
	AccountID               int64                                             `json:"account_id"`
	ContextTransport        string                                            `json:"context_transport,omitempty"`
	LastResponseID          string                                            `json:"last_response_id,omitempty"`
	MaterializedHashes      []string                                          `json:"materialized_hashes,omitempty"`
	MaterializedCount       int                                               `json:"materialized_count"`
	InputCount              int                                               `json:"input_count"`
	InputOnlyContext        bool                                              `json:"input_only_context,omitempty"`
	NonInputHash            string                                            `json:"non_input_hash,omitempty"`
	NonInputFields          map[string]openAIWSSessionContextCacheFingerprint `json:"non_input_fields,omitempty"`
	RawVsClientVisibleEqual bool                                              `json:"raw_vs_client_visible_equal"`
}

type openAIWSSessionContextCacheFingerprint struct {
	Kind string `json:"kind,omitempty"`
	Size int    `json:"size"`
	Hash string `json:"hash,omitempty"`
}

type openAIWSConnLastResponseBinding struct {
	responseID string
	expiresAt  time.Time
}

type openAIWSConnEvictDiagnostic struct {
	reason    string
	evictedAt time.Time
	expiresAt time.Time
}

type openAIResponsesSessionWindowBinding struct {
	window    openAIResponsesSessionWindow
	expiresAt time.Time
}

// OpenAIWSStateStore 管理 WSv2 的粘连状态。
// - response_id -> account_id 用于续链路由
// - response_id -> conn_id 用于连接内上下文复用
//
// response_id -> account_id 优先走 GatewayCache（Redis），同时维护本地热缓存。
// response_id -> conn_id 仅在本进程内有效。
type OpenAIWSStateStore interface {
	BindResponseAccount(ctx context.Context, groupID int64, apiKeyID int64, responseID string, accountID int64, ttl time.Duration) error
	GetResponseAccount(ctx context.Context, groupID int64, apiKeyID int64, responseID string) (int64, error)
	DeleteResponseAccount(ctx context.Context, groupID int64, apiKeyID int64, responseID string) error

	BindResponseConn(groupID int64, apiKeyID int64, responseID, connID string, ttl time.Duration)
	GetResponseConn(groupID int64, apiKeyID int64, responseID string) (string, bool)
	DeleteResponseConn(groupID int64, apiKeyID int64, responseID string)

	BindSessionTurnState(groupID int64, sessionHash, turnState string, ttl time.Duration)
	GetSessionTurnState(groupID int64, sessionHash string) (string, bool)
	DeleteSessionTurnState(groupID int64, sessionHash string)

	BindSessionConn(groupID int64, apiKeyID int64, accountID int64, sessionHash, connID string, ttl time.Duration)
	GetSessionConn(groupID int64, apiKeyID int64, accountID int64, sessionHash string) (string, bool)
	DeleteSessionConn(groupID int64, apiKeyID int64, accountID int64, sessionHash string)

	// strict-delta shadow 阶段状态。
	BindSessionContext(groupID int64, apiKeyID int64, sessionHash string, value openAIWSSessionContextValue, ttl time.Duration)
	GetSessionContext(groupID int64, apiKeyID int64, sessionHash string) (openAIWSSessionContextValue, bool)
	DeleteSessionContext(groupID int64, apiKeyID int64, sessionHash string)

	BindSessionWindow(ctx context.Context, groupID int64, apiKeyID int64, sessionHash string, window openAIResponsesSessionWindow, ttl time.Duration) error
	GetSessionWindow(ctx context.Context, groupID int64, apiKeyID int64, sessionHash string) (openAIResponsesSessionWindow, bool)
	DeleteSessionWindow(ctx context.Context, groupID int64, apiKeyID int64, sessionHash string) error

	BindConnLastResponse(connID, responseID string, ttl time.Duration)
	GetConnLastResponse(connID string) (string, bool)
	DeleteConnLastResponse(connID string)
	DeleteConnScopedState(connID string, reasons ...string)
	GetConnLastEvict(connID string) (string, time.Duration, bool)

	// 原子 per-session in-flight 标记，保证 shadow inert：仅 owner 读写状态，non-owner 不等待。
	TrySessionInFlight(groupID int64, apiKeyID int64, sessionHash string) bool
	EndSessionInFlight(groupID int64, apiKeyID int64, sessionHash string)
}

type defaultOpenAIWSStateStore struct {
	cache GatewayCache

	responseToAccountMu  sync.RWMutex
	responseToAccount    map[string]openAIWSAccountBinding
	responseToConnMu     sync.RWMutex
	responseToConn       map[string]openAIWSConnBinding
	sessionToTurnStateMu sync.RWMutex
	sessionToTurnState   map[string]openAIWSTurnStateBinding
	sessionToConnMu      sync.RWMutex
	sessionToConn        map[string]openAIWSSessionConnBinding

	sessionContextMu   sync.RWMutex
	sessionContext     map[string]openAIWSSessionContextBinding
	sessionWindowMu    sync.RWMutex
	sessionWindow      map[string]openAIResponsesSessionWindowBinding
	connLastResponseMu sync.RWMutex
	connLastResponse   map[string]openAIWSConnLastResponseBinding
	connEvictMu        sync.RWMutex
	connEvict          map[string]openAIWSConnEvictDiagnostic
	sessionInFlightMu  sync.Mutex
	sessionInFlight    map[string]struct{}

	lastCleanupUnixNano atomic.Int64
}

// NewOpenAIWSStateStore 创建默认 WS 状态存储。
func NewOpenAIWSStateStore(cache GatewayCache) OpenAIWSStateStore {
	store := &defaultOpenAIWSStateStore{
		cache:              cache,
		responseToAccount:  make(map[string]openAIWSAccountBinding, 256),
		responseToConn:     make(map[string]openAIWSConnBinding, 256),
		sessionToTurnState: make(map[string]openAIWSTurnStateBinding, 256),
		sessionToConn:      make(map[string]openAIWSSessionConnBinding, 256),
		sessionContext:     make(map[string]openAIWSSessionContextBinding, 256),
		sessionWindow:      make(map[string]openAIResponsesSessionWindowBinding, 256),
		connLastResponse:   make(map[string]openAIWSConnLastResponseBinding, 256),
		connEvict:          make(map[string]openAIWSConnEvictDiagnostic, 256),
		sessionInFlight:    make(map[string]struct{}, 256),
	}
	store.lastCleanupUnixNano.Store(time.Now().UnixNano())
	return store
}

func (s *defaultOpenAIWSStateStore) BindResponseAccount(ctx context.Context, groupID int64, apiKeyID int64, responseID string, accountID int64, ttl time.Duration) error {
	id := normalizeOpenAIWSResponseID(responseID)
	key := openAIWSResponseStateKey(groupID, apiKeyID, id)
	if key == "" || accountID <= 0 {
		return nil
	}
	ttl = normalizeOpenAIWSTTL(ttl)
	s.maybeCleanup()

	expiresAt := time.Now().Add(ttl)
	s.responseToAccountMu.Lock()
	ensureBindingCapacity(s.responseToAccount, key, openAIWSStateStoreMaxEntriesPerMap)
	s.responseToAccount[key] = openAIWSAccountBinding{accountID: accountID, expiresAt: expiresAt}
	s.responseToAccountMu.Unlock()

	if s.cache == nil {
		return nil
	}
	cacheKey := openAIWSResponseAccountCacheKey(apiKeyID, id)
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
	defer cancel()
	return s.cache.SetSessionAccountID(cacheCtx, groupID, cacheKey, accountID, ttl)
}

func (s *defaultOpenAIWSStateStore) getResponseAccountLocal(key string, now time.Time) (int64, bool) {
	if s == nil || key == "" {
		return 0, false
	}
	s.responseToAccountMu.RLock()
	binding, ok := s.responseToAccount[key]
	s.responseToAccountMu.RUnlock()
	if !ok || now.After(binding.expiresAt) || binding.accountID <= 0 {
		return 0, false
	}
	return binding.accountID, true
}

func (s *defaultOpenAIWSStateStore) GetResponseAccount(ctx context.Context, groupID int64, apiKeyID int64, responseID string) (int64, error) {
	id := normalizeOpenAIWSResponseID(responseID)
	key := openAIWSResponseStateKey(groupID, apiKeyID, id)
	if key == "" {
		return 0, nil
	}
	s.maybeCleanup()

	now := time.Now()
	if accountID, ok := s.getResponseAccountLocal(key, now); ok {
		return accountID, nil
	}

	if s.cache == nil {
		return 0, nil
	}

	cacheKey := openAIWSResponseAccountCacheKey(apiKeyID, id)
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
	defer cancel()
	accountID, err := s.cache.GetSessionAccountID(cacheCtx, groupID, cacheKey)
	if err == nil && accountID > 0 {
		return accountID, nil
	}
	// 缓存读取失败不阻断主流程，按未命中降级。
	return 0, nil
}

func (s *defaultOpenAIWSStateStore) DeleteResponseAccount(ctx context.Context, groupID int64, apiKeyID int64, responseID string) error {
	id := normalizeOpenAIWSResponseID(responseID)
	key := openAIWSResponseStateKey(groupID, apiKeyID, id)
	if key == "" {
		return nil
	}
	s.responseToAccountMu.Lock()
	delete(s.responseToAccount, key)
	s.responseToAccountMu.Unlock()

	if s.cache == nil {
		return nil
	}
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
	defer cancel()
	return s.cache.DeleteSessionAccountID(cacheCtx, groupID, openAIWSResponseAccountCacheKey(apiKeyID, id))
}

func (s *defaultOpenAIWSStateStore) BindResponseConn(groupID int64, apiKeyID int64, responseID, connID string, ttl time.Duration) {
	id := normalizeOpenAIWSResponseID(responseID)
	key := openAIWSResponseStateKey(groupID, apiKeyID, id)
	conn := strings.TrimSpace(connID)
	if key == "" || conn == "" {
		return
	}
	ttl = normalizeOpenAIWSTTL(ttl)
	s.maybeCleanup()

	s.responseToConnMu.Lock()
	ensureBindingCapacity(s.responseToConn, key, openAIWSStateStoreMaxEntriesPerMap)
	s.responseToConn[key] = openAIWSConnBinding{
		connID:    conn,
		expiresAt: time.Now().Add(ttl),
	}
	s.responseToConnMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) GetResponseConn(groupID int64, apiKeyID int64, responseID string) (string, bool) {
	id := normalizeOpenAIWSResponseID(responseID)
	key := openAIWSResponseStateKey(groupID, apiKeyID, id)
	if key == "" {
		return "", false
	}
	s.maybeCleanup()

	now := time.Now()
	s.responseToConnMu.RLock()
	binding, ok := s.responseToConn[key]
	s.responseToConnMu.RUnlock()
	if !ok || now.After(binding.expiresAt) || strings.TrimSpace(binding.connID) == "" {
		return "", false
	}
	return binding.connID, true
}

func (s *defaultOpenAIWSStateStore) DeleteResponseConn(groupID int64, apiKeyID int64, responseID string) {
	id := normalizeOpenAIWSResponseID(responseID)
	key := openAIWSResponseStateKey(groupID, apiKeyID, id)
	if key == "" {
		return
	}
	s.responseToConnMu.Lock()
	delete(s.responseToConn, key)
	s.responseToConnMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) BindSessionTurnState(groupID int64, sessionHash, turnState string, ttl time.Duration) {
	key := openAIWSSessionTurnStateKey(groupID, sessionHash)
	state := strings.TrimSpace(turnState)
	if key == "" || state == "" {
		return
	}
	ttl = normalizeOpenAIWSTTL(ttl)
	s.maybeCleanup()

	s.sessionToTurnStateMu.Lock()
	ensureBindingCapacity(s.sessionToTurnState, key, openAIWSStateStoreMaxEntriesPerMap)
	s.sessionToTurnState[key] = openAIWSTurnStateBinding{
		turnState: state,
		expiresAt: time.Now().Add(ttl),
	}
	s.sessionToTurnStateMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) GetSessionTurnState(groupID int64, sessionHash string) (string, bool) {
	key := openAIWSSessionTurnStateKey(groupID, sessionHash)
	if key == "" {
		return "", false
	}
	s.maybeCleanup()

	now := time.Now()
	s.sessionToTurnStateMu.RLock()
	binding, ok := s.sessionToTurnState[key]
	s.sessionToTurnStateMu.RUnlock()
	if !ok || now.After(binding.expiresAt) || strings.TrimSpace(binding.turnState) == "" {
		return "", false
	}
	return binding.turnState, true
}

func (s *defaultOpenAIWSStateStore) DeleteSessionTurnState(groupID int64, sessionHash string) {
	key := openAIWSSessionTurnStateKey(groupID, sessionHash)
	if key == "" {
		return
	}
	s.sessionToTurnStateMu.Lock()
	delete(s.sessionToTurnState, key)
	s.sessionToTurnStateMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) BindSessionConn(groupID int64, apiKeyID int64, accountID int64, sessionHash, connID string, ttl time.Duration) {
	key := openAIWSSessionConnKey(groupID, apiKeyID, accountID, sessionHash)
	conn := strings.TrimSpace(connID)
	if key == "" || conn == "" {
		return
	}
	ttl = normalizeOpenAIWSTTL(ttl)
	s.maybeCleanup()

	s.sessionToConnMu.Lock()
	ensureBindingCapacity(s.sessionToConn, key, openAIWSStateStoreMaxEntriesPerMap)
	s.sessionToConn[key] = openAIWSSessionConnBinding{
		connID:    conn,
		expiresAt: time.Now().Add(ttl),
	}
	s.sessionToConnMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) GetSessionConn(groupID int64, apiKeyID int64, accountID int64, sessionHash string) (string, bool) {
	key := openAIWSSessionConnKey(groupID, apiKeyID, accountID, sessionHash)
	if key == "" {
		return "", false
	}
	s.maybeCleanup()

	now := time.Now()
	s.sessionToConnMu.RLock()
	binding, ok := s.sessionToConn[key]
	s.sessionToConnMu.RUnlock()
	if !ok || now.After(binding.expiresAt) || strings.TrimSpace(binding.connID) == "" {
		return "", false
	}
	return binding.connID, true
}

func (s *defaultOpenAIWSStateStore) DeleteSessionConn(groupID int64, apiKeyID int64, accountID int64, sessionHash string) {
	key := openAIWSSessionConnKey(groupID, apiKeyID, accountID, sessionHash)
	if key == "" {
		return
	}
	s.sessionToConnMu.Lock()
	delete(s.sessionToConn, key)
	s.sessionToConnMu.Unlock()
}

func cloneOpenAIWSSessionContextValue(value openAIWSSessionContextValue) openAIWSSessionContextValue {
	if len(value.materializedHashes) > 0 {
		value.materializedHashes = append([][32]byte(nil), value.materializedHashes...)
	}
	if len(value.materializedShapes) > 0 {
		value.materializedShapes = append([]string(nil), value.materializedShapes...)
	}
	if len(value.nonInputFields) > 0 {
		fields := make(map[string]openAIWSNonInputFieldFingerprint, len(value.nonInputFields))
		for key, field := range value.nonInputFields {
			fields[key] = field
		}
		value.nonInputFields = fields
	}
	return value
}

func encodeOpenAIWSSessionContextForCache(value openAIWSSessionContextValue) ([]byte, bool) {
	dto := openAIWSSessionContextCacheValue{
		Version:                 openAIWSSessionContextCacheVersion,
		AccountID:               value.accountID,
		ContextTransport:        openAIWSSessionContextTransportForCache(value.connID),
		LastResponseID:          strings.TrimSpace(value.lastResponseID),
		MaterializedHashes:      make([]string, 0, len(value.materializedHashes)),
		MaterializedCount:       value.materializedCount,
		InputCount:              value.inputCount,
		InputOnlyContext:        value.inputOnlyContext,
		NonInputHash:            openAIWSSHA256Hex(value.nonInputHash),
		RawVsClientVisibleEqual: value.rawVsClientVisibleEqual,
	}
	for _, hash := range value.materializedHashes {
		dto.MaterializedHashes = append(dto.MaterializedHashes, openAIWSSHA256Hex(hash))
	}
	if len(value.nonInputFields) > 0 {
		dto.NonInputFields = make(map[string]openAIWSSessionContextCacheFingerprint, len(value.nonInputFields))
		for key, field := range value.nonInputFields {
			dto.NonInputFields[key] = openAIWSSessionContextCacheFingerprint{
				Kind: field.kind,
				Size: field.size,
				Hash: openAIWSSHA256Hex(field.hash),
			}
		}
	}
	encoded, err := json.Marshal(dto)
	if err != nil {
		return nil, false
	}
	return encoded, true
}

func decodeOpenAIWSSessionContextFromCache(payload []byte) (openAIWSSessionContextValue, bool) {
	if len(payload) == 0 {
		return openAIWSSessionContextValue{}, false
	}
	var dto openAIWSSessionContextCacheValue
	if err := json.Unmarshal(payload, &dto); err != nil {
		return openAIWSSessionContextValue{}, false
	}
	if dto.Version != openAIWSSessionContextCacheVersion {
		return openAIWSSessionContextValue{}, false
	}
	value := openAIWSSessionContextValue{
		accountID:               dto.AccountID,
		connID:                  openAIWSSessionContextConnIDFromCacheTransport(dto.ContextTransport),
		lastResponseID:          strings.TrimSpace(dto.LastResponseID),
		materializedHashes:      make([][32]byte, 0, len(dto.MaterializedHashes)),
		materializedShapes:      nil,
		materializedCount:       dto.MaterializedCount,
		inputCount:              dto.InputCount,
		inputOnlyContext:        dto.InputOnlyContext,
		rawVsClientVisibleEqual: dto.RawVsClientVisibleEqual,
	}
	for _, encodedHash := range dto.MaterializedHashes {
		hash, ok := openAIWSDecodeSHA256Hex(encodedHash)
		if !ok {
			return openAIWSSessionContextValue{}, false
		}
		value.materializedHashes = append(value.materializedHashes, hash)
	}
	if strings.TrimSpace(dto.NonInputHash) != "" {
		hash, ok := openAIWSDecodeSHA256Hex(dto.NonInputHash)
		if !ok {
			return openAIWSSessionContextValue{}, false
		}
		value.nonInputHash = hash
	}
	if len(dto.NonInputFields) > 0 {
		value.nonInputFields = make(map[string]openAIWSNonInputFieldFingerprint, len(dto.NonInputFields))
		for key, field := range dto.NonInputFields {
			hash, ok := openAIWSDecodeSHA256Hex(field.Hash)
			if !ok {
				return openAIWSSessionContextValue{}, false
			}
			value.nonInputFields[key] = openAIWSNonInputFieldFingerprint{
				kind: field.Kind,
				size: field.Size,
				hash: hash,
			}
		}
	}
	return value, true
}

func openAIWSSessionContextTransportForCache(connID string) string {
	if strings.TrimSpace(connID) == "http" {
		return "http"
	}
	return ""
}

func openAIWSSessionContextConnIDFromCacheTransport(transport string) string {
	if strings.TrimSpace(transport) == "http" {
		return "http"
	}
	return ""
}

func openAIWSSHA256Hex(value [32]byte) string {
	return hex.EncodeToString(value[:])
}

func openAIWSDecodeSHA256Hex(value string) ([32]byte, bool) {
	var out [32]byte
	raw, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil || len(raw) != len(out) {
		return out, false
	}
	copy(out[:], raw)
	return out, true
}

func (s *defaultOpenAIWSStateStore) BindSessionContext(groupID int64, apiKeyID int64, sessionHash string, value openAIWSSessionContextValue, ttl time.Duration) {
	key := openAIWSSessionContextKey(groupID, apiKeyID, sessionHash)
	if key == "" {
		return
	}
	ttl = normalizeOpenAIWSTTL(ttl)
	s.maybeCleanup()

	value = cloneOpenAIWSSessionContextValue(value)
	s.sessionContextMu.Lock()
	ensureBindingCapacity(s.sessionContext, key, openAIWSStateStoreMaxEntriesPerMap)
	s.sessionContext[key] = openAIWSSessionContextBinding{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	s.sessionContextMu.Unlock()

	if s.cache == nil {
		return
	}
	cacheKey := openAIWSSessionContextCacheKey(apiKeyID, sessionHash)
	encoded, ok := encodeOpenAIWSSessionContextForCache(value)
	if cacheKey == "" || !ok {
		return
	}
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(context.Background())
	defer cancel()
	_ = s.cache.SetOpenAIResponsesSessionWindow(cacheCtx, groupID, cacheKey, encoded, ttl)
}

func (s *defaultOpenAIWSStateStore) GetSessionContext(groupID int64, apiKeyID int64, sessionHash string) (openAIWSSessionContextValue, bool) {
	key := openAIWSSessionContextKey(groupID, apiKeyID, sessionHash)
	if key == "" {
		return openAIWSSessionContextValue{}, false
	}
	s.maybeCleanup()

	now := time.Now()
	s.sessionContextMu.RLock()
	binding, ok := s.sessionContext[key]
	s.sessionContextMu.RUnlock()
	if ok && !now.After(binding.expiresAt) {
		if s.cache != nil && !s.sessionContextCacheExists(groupID, apiKeyID, sessionHash) {
			s.sessionContextMu.Lock()
			if current, exists := s.sessionContext[key]; exists && current.expiresAt.Equal(binding.expiresAt) {
				delete(s.sessionContext, key)
			}
			s.sessionContextMu.Unlock()
			return openAIWSSessionContextValue{}, false
		}
		return cloneOpenAIWSSessionContextValue(binding.value), true
	}
	if ok && now.After(binding.expiresAt) {
		s.sessionContextMu.Lock()
		if current, exists := s.sessionContext[key]; exists && now.After(current.expiresAt) {
			delete(s.sessionContext, key)
		}
		s.sessionContextMu.Unlock()
	}

	if s.cache == nil {
		return openAIWSSessionContextValue{}, false
	}
	cacheKey := openAIWSSessionContextCacheKey(apiKeyID, sessionHash)
	if cacheKey == "" {
		return openAIWSSessionContextValue{}, false
	}
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(context.Background())
	defer cancel()
	payload, err := s.cache.GetOpenAIResponsesSessionWindow(cacheCtx, groupID, cacheKey)
	if err != nil || len(payload) == 0 {
		return openAIWSSessionContextValue{}, false
	}
	value, ok := decodeOpenAIWSSessionContextFromCache(payload)
	if !ok {
		return openAIWSSessionContextValue{}, false
	}
	return cloneOpenAIWSSessionContextValue(value), true
}

func (s *defaultOpenAIWSStateStore) sessionContextCacheExists(groupID int64, apiKeyID int64, sessionHash string) bool {
	if s == nil || s.cache == nil {
		return false
	}
	cacheKey := openAIWSSessionContextCacheKey(apiKeyID, sessionHash)
	if cacheKey == "" {
		return false
	}
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(context.Background())
	defer cancel()
	payload, err := s.cache.GetOpenAIResponsesSessionWindow(cacheCtx, groupID, cacheKey)
	if err != nil || len(payload) == 0 {
		return false
	}
	_, ok := decodeOpenAIWSSessionContextFromCache(payload)
	return ok
}

func (s *defaultOpenAIWSStateStore) DeleteSessionContext(groupID int64, apiKeyID int64, sessionHash string) {
	key := openAIWSSessionContextKey(groupID, apiKeyID, sessionHash)
	if key == "" {
		return
	}
	s.sessionContextMu.Lock()
	delete(s.sessionContext, key)
	s.sessionContextMu.Unlock()

	if s.cache == nil {
		return
	}
	cacheKey := openAIWSSessionContextCacheKey(apiKeyID, sessionHash)
	if cacheKey == "" {
		return
	}
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(context.Background())
	defer cancel()
	_ = s.cache.DeleteOpenAIResponsesSessionWindow(cacheCtx, groupID, cacheKey)
}

func (s *defaultOpenAIWSStateStore) BindSessionWindow(ctx context.Context, groupID int64, apiKeyID int64, sessionHash string, window openAIResponsesSessionWindow, ttl time.Duration) error {
	key := openAIWSSessionContextKey(groupID, apiKeyID, sessionHash)
	cacheKey := openAIResponsesSessionWindowCacheKey(apiKeyID, sessionHash)
	window = normalizeOpenAIResponsesSessionWindow(window)
	if key == "" || cacheKey == "" || !openAIResponsesSessionWindowHasData(window) {
		return nil
	}
	ttl = normalizeOpenAIWSTTL(ttl)
	s.maybeCleanup()

	s.sessionWindowMu.Lock()
	ensureBindingCapacity(s.sessionWindow, key, openAIWSStateStoreMaxEntriesPerMap)
	s.sessionWindow[key] = openAIResponsesSessionWindowBinding{
		window:    window.clone(),
		expiresAt: time.Now().Add(ttl),
	}
	s.sessionWindowMu.Unlock()

	if s.cache == nil {
		return nil
	}
	encoded, err := json.Marshal(window)
	if err != nil {
		return err
	}
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
	defer cancel()
	return s.cache.SetOpenAIResponsesSessionWindow(cacheCtx, groupID, cacheKey, encoded, ttl)
}

func (s *defaultOpenAIWSStateStore) GetSessionWindow(ctx context.Context, groupID int64, apiKeyID int64, sessionHash string) (openAIResponsesSessionWindow, bool) {
	key := openAIWSSessionContextKey(groupID, apiKeyID, sessionHash)
	cacheKey := openAIResponsesSessionWindowCacheKey(apiKeyID, sessionHash)
	if key == "" || cacheKey == "" {
		return openAIResponsesSessionWindow{}, false
	}
	s.maybeCleanup()

	now := time.Now()
	s.sessionWindowMu.RLock()
	binding, ok := s.sessionWindow[key]
	s.sessionWindowMu.RUnlock()
	if ok && !now.After(binding.expiresAt) && openAIResponsesSessionWindowHasData(binding.window) {
		return binding.window.clone(), true
	}

	if s.cache == nil {
		return openAIResponsesSessionWindow{}, false
	}

	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
	defer cancel()
	payload, err := s.cache.GetOpenAIResponsesSessionWindow(cacheCtx, groupID, cacheKey)
	if err != nil || len(payload) == 0 {
		return openAIResponsesSessionWindow{}, false
	}
	var window openAIResponsesSessionWindow
	if err := json.Unmarshal(payload, &window); err != nil {
		return openAIResponsesSessionWindow{}, false
	}
	window = normalizeOpenAIResponsesSessionWindow(window)
	if !openAIResponsesSessionWindowHasData(window) {
		return openAIResponsesSessionWindow{}, false
	}
	return window, true
}

func (s *defaultOpenAIWSStateStore) DeleteSessionWindow(ctx context.Context, groupID int64, apiKeyID int64, sessionHash string) error {
	key := openAIWSSessionContextKey(groupID, apiKeyID, sessionHash)
	cacheKey := openAIResponsesSessionWindowCacheKey(apiKeyID, sessionHash)
	if key == "" || cacheKey == "" {
		return nil
	}
	s.sessionWindowMu.Lock()
	delete(s.sessionWindow, key)
	s.sessionWindowMu.Unlock()

	if s.cache == nil {
		return nil
	}
	cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(ctx)
	defer cancel()
	return s.cache.DeleteOpenAIResponsesSessionWindow(cacheCtx, groupID, cacheKey)
}

func (s *defaultOpenAIWSStateStore) BindConnLastResponse(connID, responseID string, ttl time.Duration) {
	conn := strings.TrimSpace(connID)
	id := normalizeOpenAIWSResponseID(responseID)
	if conn == "" || id == "" {
		return
	}
	ttl = normalizeOpenAIWSTTL(ttl)
	s.maybeCleanup()

	s.connLastResponseMu.Lock()
	ensureBindingCapacity(s.connLastResponse, conn, openAIWSStateStoreMaxEntriesPerMap)
	s.connLastResponse[conn] = openAIWSConnLastResponseBinding{
		responseID: id,
		expiresAt:  time.Now().Add(ttl),
	}
	s.connLastResponseMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) GetConnLastResponse(connID string) (string, bool) {
	conn := strings.TrimSpace(connID)
	if conn == "" {
		return "", false
	}
	s.maybeCleanup()

	now := time.Now()
	s.connLastResponseMu.RLock()
	binding, ok := s.connLastResponse[conn]
	s.connLastResponseMu.RUnlock()
	if !ok || now.After(binding.expiresAt) || strings.TrimSpace(binding.responseID) == "" {
		return "", false
	}
	return binding.responseID, true
}

func (s *defaultOpenAIWSStateStore) DeleteConnLastResponse(connID string) {
	conn := strings.TrimSpace(connID)
	if conn == "" {
		return
	}
	s.connLastResponseMu.Lock()
	delete(s.connLastResponse, conn)
	s.connLastResponseMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) recordConnEvictDiagnostic(connID, reason string) {
	conn := strings.TrimSpace(connID)
	if s == nil || conn == "" {
		return
	}
	evictReason := strings.TrimSpace(reason)
	if evictReason == "" {
		evictReason = "unknown"
	}
	now := time.Now()
	s.connEvictMu.Lock()
	ensureBindingCapacity(s.connEvict, conn, openAIWSStateStoreMaxEntriesPerMap)
	s.connEvict[conn] = openAIWSConnEvictDiagnostic{
		reason:    evictReason,
		evictedAt: now,
		expiresAt: now.Add(openAIWSConnEvictDiagnosticTTL),
	}
	s.connEvictMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) GetConnLastEvict(connID string) (string, time.Duration, bool) {
	conn := strings.TrimSpace(connID)
	if s == nil || conn == "" {
		return "", 0, false
	}
	s.maybeCleanup()
	now := time.Now()
	s.connEvictMu.RLock()
	diagnostic, ok := s.connEvict[conn]
	s.connEvictMu.RUnlock()
	if !ok || now.After(diagnostic.expiresAt) || strings.TrimSpace(diagnostic.reason) == "" {
		return "", 0, false
	}
	age := now.Sub(diagnostic.evictedAt)
	if age < 0 {
		age = 0
	}
	return diagnostic.reason, age, true
}

func (s *defaultOpenAIWSStateStore) DeleteConnScopedState(connID string, reasons ...string) {
	conn := strings.TrimSpace(connID)
	if s == nil || conn == "" {
		return
	}
	reason := "unknown"
	if len(reasons) > 0 && strings.TrimSpace(reasons[0]) != "" {
		reason = strings.TrimSpace(reasons[0])
	}
	s.recordConnEvictDiagnostic(conn, reason)

	responseConnDeleted := 0
	s.responseToConnMu.Lock()
	for key, binding := range s.responseToConn {
		if strings.TrimSpace(binding.connID) == conn {
			delete(s.responseToConn, key)
			responseConnDeleted++
		}
	}
	s.responseToConnMu.Unlock()

	sessionConnDeleted := 0
	s.sessionToConnMu.Lock()
	for key, binding := range s.sessionToConn {
		if strings.TrimSpace(binding.connID) == conn {
			delete(s.sessionToConn, key)
			sessionConnDeleted++
		}
	}
	s.sessionToConnMu.Unlock()

	sessionContextDeleted := 0
	sessionContextCacheDeleteKeys := make([]string, 0)
	if openAIWSConnEvictReasonInvalidatesSessionContext(reason) {
		s.sessionContextMu.Lock()
		for key, binding := range s.sessionContext {
			if strings.TrimSpace(binding.value.connID) == conn {
				delete(s.sessionContext, key)
				sessionContextDeleted++
				if openAIWSConnEvictReasonDeletesDurableSessionContext(reason) {
					sessionContextCacheDeleteKeys = append(sessionContextCacheDeleteKeys, key)
				}
			}
		}
		s.sessionContextMu.Unlock()
	}
	if s.cache != nil {
		for _, key := range sessionContextCacheDeleteKeys {
			groupID, apiKeyID, sessionHash, ok := parseOpenAIWSSessionContextKey(key)
			if !ok {
				continue
			}
			cacheKey := openAIWSSessionContextCacheKey(apiKeyID, sessionHash)
			if cacheKey == "" {
				continue
			}
			cacheCtx, cancel := withOpenAIWSStateStoreRedisTimeout(context.Background())
			_ = s.cache.DeleteOpenAIResponsesSessionWindow(cacheCtx, groupID, cacheKey)
			cancel()
		}
	}

	s.DeleteConnLastResponse(conn)
	logOpenAIWSModeInfo(
		"conn_scoped_state_deleted conn_id=%s reason=%s response_conn_deleted=%d session_conn_deleted=%d session_context_deleted=%d",
		normalizeOpenAIWSLogValue(conn),
		normalizeOpenAIWSLogValue(reason),
		responseConnDeleted,
		sessionConnDeleted,
		sessionContextDeleted,
	)
}

func openAIWSConnEvictReasonInvalidatesSessionContext(reason string) bool {
	reason = strings.TrimSpace(reason)
	switch reason {
	case "write_request_fail", "prewarm_write_fail":
		return false
	case "closed", "evict", "conn_max_age", "session_idle_ttl", "neutral_idle_ttl", "neutral_acquire_stale_idle", "neutral_over_target", "idle_over_max", "nil_conn":
		return true
	default:
		return strings.HasSuffix(reason, "_fail") ||
			strings.HasSuffix(reason, "_failed") ||
			strings.HasSuffix(reason, "_event") ||
			strings.HasPrefix(reason, "read_fail") ||
			strings.HasPrefix(reason, "write_request_fail") ||
			strings.HasPrefix(reason, "error_event") ||
			strings.HasPrefix(reason, "err_event") ||
			strings.HasPrefix(reason, "response_failed") ||
			strings.HasPrefix(reason, "session_preempted") ||
			strings.HasPrefix(reason, "client_disconnected") ||
			strings.HasPrefix(reason, "unclean_exit") ||
			strings.HasPrefix(reason, "ingress_") ||
			strings.HasPrefix(reason, "prewarm_") ||
			strings.HasPrefix(reason, "soft_rate_limit")
	}
}

func openAIWSConnEvictReasonDeletesDurableSessionContext(reason string) bool {
	reason = strings.TrimSpace(reason)
	switch reason {
	case "closed", "evict", "conn_max_age", "session_idle_ttl", "neutral_idle_ttl", "neutral_acquire_stale_idle", "neutral_over_target", "idle_over_max", "nil_conn":
		return false
	case "write_request_fail", "prewarm_write_fail":
		return false
	}
	return strings.HasSuffix(reason, "_fail") ||
		strings.HasSuffix(reason, "_failed") ||
		strings.HasSuffix(reason, "_event") ||
		strings.HasPrefix(reason, "read_fail") ||
		strings.HasPrefix(reason, "write_request_fail") ||
		strings.HasPrefix(reason, "error_event") ||
		strings.HasPrefix(reason, "err_event") ||
		strings.HasPrefix(reason, "response_failed") ||
		strings.HasPrefix(reason, "session_preempted") ||
		strings.HasPrefix(reason, "client_disconnected") ||
		strings.HasPrefix(reason, "unclean_exit") ||
		strings.HasPrefix(reason, "ingress_") ||
		strings.HasPrefix(reason, "prewarm_") ||
		strings.HasPrefix(reason, "soft_rate_limit")
}

func (s *defaultOpenAIWSStateStore) TrySessionInFlight(groupID int64, apiKeyID int64, sessionHash string) bool {
	key := openAIWSSessionContextKey(groupID, apiKeyID, sessionHash)
	if key == "" {
		return false
	}
	s.sessionInFlightMu.Lock()
	defer s.sessionInFlightMu.Unlock()
	if _, exists := s.sessionInFlight[key]; exists {
		return false
	}
	s.sessionInFlight[key] = struct{}{}
	return true
}

func (s *defaultOpenAIWSStateStore) EndSessionInFlight(groupID int64, apiKeyID int64, sessionHash string) {
	key := openAIWSSessionContextKey(groupID, apiKeyID, sessionHash)
	if key == "" {
		return
	}
	s.sessionInFlightMu.Lock()
	delete(s.sessionInFlight, key)
	s.sessionInFlightMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) maybeCleanup() {
	if s == nil {
		return
	}
	now := time.Now()
	last := time.Unix(0, s.lastCleanupUnixNano.Load())
	if now.Sub(last) < openAIWSStateStoreCleanupInterval {
		return
	}
	if !s.lastCleanupUnixNano.CompareAndSwap(last.UnixNano(), now.UnixNano()) {
		return
	}

	// 增量限额清理，避免高规模下一次性全量扫描导致长时间阻塞。
	s.responseToAccountMu.Lock()
	cleanupExpiredAccountBindings(s.responseToAccount, now, openAIWSStateStoreCleanupMaxPerMap)
	s.responseToAccountMu.Unlock()

	s.responseToConnMu.Lock()
	cleanupExpiredConnBindings(s.responseToConn, now, openAIWSStateStoreCleanupMaxPerMap)
	s.responseToConnMu.Unlock()

	s.sessionToTurnStateMu.Lock()
	cleanupExpiredTurnStateBindings(s.sessionToTurnState, now, openAIWSStateStoreCleanupMaxPerMap)
	s.sessionToTurnStateMu.Unlock()

	s.sessionToConnMu.Lock()
	cleanupExpiredSessionConnBindings(s.sessionToConn, now, openAIWSStateStoreCleanupMaxPerMap)
	s.sessionToConnMu.Unlock()

	s.sessionContextMu.Lock()
	cleanupExpiredSessionContextBindings(s.sessionContext, now, openAIWSStateStoreCleanupMaxPerMap)
	s.sessionContextMu.Unlock()

	s.sessionWindowMu.Lock()
	cleanupExpiredSessionWindowBindings(s.sessionWindow, now, openAIWSStateStoreCleanupMaxPerMap)
	s.sessionWindowMu.Unlock()

	s.connLastResponseMu.Lock()
	cleanupExpiredConnLastResponseBindings(s.connLastResponse, now, openAIWSStateStoreCleanupMaxPerMap)
	s.connLastResponseMu.Unlock()

	s.connEvictMu.Lock()
	cleanupExpiredConnEvictDiagnostics(s.connEvict, now, openAIWSStateStoreCleanupMaxPerMap)
	s.connEvictMu.Unlock()
}

func cleanupExpiredSessionWindowBindings(bindings map[string]openAIResponsesSessionWindowBinding, now time.Time, maxScan int) {
	if len(bindings) == 0 || maxScan <= 0 {
		return
	}
	scanned := 0
	for key, binding := range bindings {
		if now.After(binding.expiresAt) {
			delete(bindings, key)
		}
		scanned++
		if scanned >= maxScan {
			break
		}
	}
}

func cleanupExpiredSessionContextBindings(bindings map[string]openAIWSSessionContextBinding, now time.Time, maxScan int) {
	if len(bindings) == 0 || maxScan <= 0 {
		return
	}
	scanned := 0
	for key, binding := range bindings {
		if now.After(binding.expiresAt) {
			delete(bindings, key)
		}
		scanned++
		if scanned >= maxScan {
			break
		}
	}
}

func cleanupExpiredConnLastResponseBindings(bindings map[string]openAIWSConnLastResponseBinding, now time.Time, maxScan int) {
	if len(bindings) == 0 || maxScan <= 0 {
		return
	}
	scanned := 0
	for key, binding := range bindings {
		if now.After(binding.expiresAt) {
			delete(bindings, key)
		}
		scanned++
		if scanned >= maxScan {
			break
		}
	}
}

func cleanupExpiredConnEvictDiagnostics(bindings map[string]openAIWSConnEvictDiagnostic, now time.Time, maxScan int) {
	if len(bindings) == 0 || maxScan <= 0 {
		return
	}
	scanned := 0
	for key, binding := range bindings {
		if now.After(binding.expiresAt) {
			delete(bindings, key)
		}
		scanned++
		if scanned >= maxScan {
			break
		}
	}
}

func cleanupExpiredAccountBindings(bindings map[string]openAIWSAccountBinding, now time.Time, maxScan int) {
	if len(bindings) == 0 || maxScan <= 0 {
		return
	}
	scanned := 0
	for key, binding := range bindings {
		if now.After(binding.expiresAt) {
			delete(bindings, key)
		}
		scanned++
		if scanned >= maxScan {
			break
		}
	}
}

func cleanupExpiredConnBindings(bindings map[string]openAIWSConnBinding, now time.Time, maxScan int) {
	if len(bindings) == 0 || maxScan <= 0 {
		return
	}
	scanned := 0
	for key, binding := range bindings {
		if now.After(binding.expiresAt) {
			delete(bindings, key)
		}
		scanned++
		if scanned >= maxScan {
			break
		}
	}
}

func cleanupExpiredTurnStateBindings(bindings map[string]openAIWSTurnStateBinding, now time.Time, maxScan int) {
	if len(bindings) == 0 || maxScan <= 0 {
		return
	}
	scanned := 0
	for key, binding := range bindings {
		if now.After(binding.expiresAt) {
			delete(bindings, key)
		}
		scanned++
		if scanned >= maxScan {
			break
		}
	}
}

func cleanupExpiredSessionConnBindings(bindings map[string]openAIWSSessionConnBinding, now time.Time, maxScan int) {
	if len(bindings) == 0 || maxScan <= 0 {
		return
	}
	scanned := 0
	for key, binding := range bindings {
		if now.After(binding.expiresAt) {
			delete(bindings, key)
		}
		scanned++
		if scanned >= maxScan {
			break
		}
	}
}

func ensureBindingCapacity[T any](bindings map[string]T, incomingKey string, maxEntries int) {
	if len(bindings) < maxEntries || maxEntries <= 0 {
		return
	}
	if _, exists := bindings[incomingKey]; exists {
		return
	}
	// 固定上限保护：淘汰任意一项，优先保证内存有界。
	for key := range bindings {
		delete(bindings, key)
		return
	}
}

func normalizeOpenAIWSResponseID(responseID string) string {
	return strings.TrimSpace(responseID)
}

func openAIWSResponseAccountCacheKey(apiKeyID int64, responseID string) string {
	id := normalizeOpenAIWSResponseID(responseID)
	if id == "" {
		return ""
	}
	seed := id
	if apiKeyID > 0 {
		seed = fmt.Sprintf("api_key:%d:%s", apiKeyID, id)
	}
	sum := sha256.Sum256([]byte(seed))
	return openAIWSResponseAccountCachePrefix + hex.EncodeToString(sum[:])
}

func openAIWSResponseStateKey(groupID int64, apiKeyID int64, responseID string) string {
	id := normalizeOpenAIWSResponseID(responseID)
	if id == "" {
		return ""
	}
	return fmt.Sprintf("%d:%d:%s", groupID, apiKeyID, id)
}

func normalizeOpenAIWSTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return time.Hour
	}
	return ttl
}

func openAIWSSessionTurnStateKey(groupID int64, sessionHash string) string {
	hash := strings.TrimSpace(sessionHash)
	if hash == "" {
		return ""
	}
	return fmt.Sprintf("%d:%s", groupID, hash)
}

func openAIWSSessionConnKey(groupID int64, apiKeyID int64, accountID int64, sessionHash string) string {
	hash := strings.TrimSpace(sessionHash)
	if hash == "" || accountID <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d:%d:%s", groupID, apiKeyID, accountID, hash)
}

// openAIWSSessionContextKey 显式包含 apiKeyID（sessionHash 虽已 api-key-scoped，仍按 plan 显式区分）。
func openAIWSSessionContextKey(groupID int64, apiKeyID int64, sessionHash string) string {
	hash := strings.TrimSpace(sessionHash)
	if hash == "" {
		return ""
	}
	return fmt.Sprintf("%d:%d:%s", groupID, apiKeyID, hash)
}

func openAIWSSessionContextCacheKey(apiKeyID int64, sessionHash string) string {
	hash := strings.TrimSpace(sessionHash)
	if hash == "" {
		return ""
	}
	return fmt.Sprintf("%s%d:%s", openAIWSSessionContextCachePrefix, apiKeyID, hash)
}

func parseOpenAIWSSessionContextKey(key string) (int64, int64, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(key), ":", 3)
	if len(parts) != 3 || strings.TrimSpace(parts[2]) == "" {
		return 0, 0, "", false
	}
	groupID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, "", false
	}
	apiKeyID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, "", false
	}
	return groupID, apiKeyID, parts[2], true
}

func withOpenAIWSStateStoreRedisTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, openAIWSStateStoreRedisTimeout)
}
