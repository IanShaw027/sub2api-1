package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/hex"
	"fmt"
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
	rawVsClientVisibleEqual bool
}

type openAIWSSessionContextBinding struct {
	value     openAIWSSessionContextValue
	expiresAt time.Time
}

type openAIWSConnLastResponseBinding struct {
	responseID string
	expiresAt  time.Time
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

	BindSessionConn(groupID int64, sessionHash, connID string, ttl time.Duration)
	GetSessionConn(groupID int64, sessionHash string) (string, bool)
	DeleteSessionConn(groupID int64, sessionHash string)

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
	DeleteConnScopedState(connID string)

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

func (s *defaultOpenAIWSStateStore) BindSessionConn(groupID int64, sessionHash, connID string, ttl time.Duration) {
	key := openAIWSSessionTurnStateKey(groupID, sessionHash)
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

func (s *defaultOpenAIWSStateStore) GetSessionConn(groupID int64, sessionHash string) (string, bool) {
	key := openAIWSSessionTurnStateKey(groupID, sessionHash)
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

func (s *defaultOpenAIWSStateStore) DeleteSessionConn(groupID int64, sessionHash string) {
	key := openAIWSSessionTurnStateKey(groupID, sessionHash)
	if key == "" {
		return
	}
	s.sessionToConnMu.Lock()
	delete(s.sessionToConn, key)
	s.sessionToConnMu.Unlock()
}

func (s *defaultOpenAIWSStateStore) BindSessionContext(groupID int64, apiKeyID int64, sessionHash string, value openAIWSSessionContextValue, ttl time.Duration) {
	key := openAIWSSessionContextKey(groupID, apiKeyID, sessionHash)
	if key == "" {
		return
	}
	ttl = normalizeOpenAIWSTTL(ttl)
	s.maybeCleanup()

	s.sessionContextMu.Lock()
	ensureBindingCapacity(s.sessionContext, key, openAIWSStateStoreMaxEntriesPerMap)
	s.sessionContext[key] = openAIWSSessionContextBinding{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	s.sessionContextMu.Unlock()
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
	if !ok || now.After(binding.expiresAt) {
		return openAIWSSessionContextValue{}, false
	}
	return binding.value, true
}

func (s *defaultOpenAIWSStateStore) DeleteSessionContext(groupID int64, apiKeyID int64, sessionHash string) {
	key := openAIWSSessionContextKey(groupID, apiKeyID, sessionHash)
	if key == "" {
		return
	}
	s.sessionContextMu.Lock()
	delete(s.sessionContext, key)
	s.sessionContextMu.Unlock()
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

func (s *defaultOpenAIWSStateStore) DeleteConnScopedState(connID string) {
	conn := strings.TrimSpace(connID)
	if s == nil || conn == "" {
		return
	}

	s.responseToConnMu.Lock()
	for key, binding := range s.responseToConn {
		if strings.TrimSpace(binding.connID) == conn {
			delete(s.responseToConn, key)
		}
	}
	s.responseToConnMu.Unlock()

	s.sessionToConnMu.Lock()
	for key, binding := range s.sessionToConn {
		if strings.TrimSpace(binding.connID) == conn {
			delete(s.sessionToConn, key)
		}
	}
	s.sessionToConnMu.Unlock()

	s.DeleteConnLastResponse(conn)
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

// openAIWSSessionContextKey 显式包含 apiKeyID（sessionHash 虽已 api-key-scoped，仍按 plan 显式区分）。
func openAIWSSessionContextKey(groupID int64, apiKeyID int64, sessionHash string) string {
	hash := strings.TrimSpace(sessionHash)
	if hash == "" {
		return ""
	}
	return fmt.Sprintf("%d:%d:%s", groupID, apiKeyID, hash)
}

func withOpenAIWSStateStoreRedisTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, openAIWSStateStoreRedisTimeout)
}
