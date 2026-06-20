package service

import (
	"container/heap"
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"golang.org/x/sync/singleflight"
)

const (
	openAIAccountScheduleLayerPreviousResponse = "previous_response_id"
	openAIAccountScheduleLayerSessionSticky    = "session_hash"
	openAIAccountScheduleLayerLoadBalance      = "load_balance"
	openAIAdvancedSchedulerSettingKey          = "openai_advanced_scheduler_enabled"
	openAIOAuthImageBridgeTransportSettingsKey = "openai_oauth_image_bridge_transport"
)

const (
	openAIAdvancedSchedulerSettingCacheTTL  = 5 * time.Second
	openAIAdvancedSchedulerSettingDBTimeout = 2 * time.Second
)

type cachedOpenAIAdvancedSchedulerSetting struct {
	enabled   bool
	expiresAt int64
}

type cachedOpenAIStickyReservePercentSetting struct {
	percent   int
	expiresAt int64
}

type cachedOpenAIStickyWaitTimeoutSetting struct {
	timeout   time.Duration
	expiresAt int64
}

type cachedOpenAIOAuthImageBridgeTransportSettings struct {
	disableKeepAlives bool
	freshClient       bool
	expiresAt         int64
}

var openAIAdvancedSchedulerSettingCache atomic.Value // *cachedOpenAIAdvancedSchedulerSetting
var openAIAdvancedSchedulerSettingSF singleflight.Group
var openAIStickyReservePercentSettingCache atomic.Value // *cachedOpenAIStickyReservePercentSetting
var openAIStickyReservePercentSettingSF singleflight.Group
var openAIStickyWaitTimeoutSettingCache atomic.Value // *cachedOpenAIStickyWaitTimeoutSetting
var openAIStickyWaitTimeoutSettingSF singleflight.Group
var openAIOAuthImageBridgeTransportSettingsCache atomic.Value // *cachedOpenAIOAuthImageBridgeTransportSettings
var openAIOAuthImageBridgeTransportSettingsSF singleflight.Group

type OpenAIAccountScheduleRequest struct {
	GroupID                 *int64
	APIKeyID                int64
	SessionHash             string
	StickyAccountID         int64
	PreserveStickyBinding   bool
	PreviousResponseID      string
	RequestedModel          string
	RequiredTransport       OpenAIUpstreamTransport
	RequiredCapability      OpenAIEndpointCapability
	RequiredImageCapability OpenAIImagesCapability
	RequiredImageRoute      string
	RequireImageEnabled     bool
	RequireOAuthAccount     bool
	RequireCompact          bool
	ExcludedIDs             map[int64]struct{}
}

func (r OpenAIAccountScheduleRequest) MaxConcurrencyFor(account *Account) int {
	if r.RequiredImageCapability != "" {
		return concurrencyForOpenAIAccountSelection(account, openAIImageRouteForAccountScheduling(r.RequiredImageRoute))
	}
	if account == nil || account.Concurrency <= 0 {
		return 1
	}
	return account.Concurrency
}

func openAIImageRouteForAccountScheduling(route string) string {
	normalized := NormalizeGroupImageGenerationRoute(route)
	if normalized == GroupImageGenerationRouteWeb2API {
		return GroupImageGenerationRouteCodex
	}
	return normalized
}

type OpenAIAccountScheduleDecision struct {
	Layer               string
	StickyPreviousHit   bool
	StickySessionHit    bool
	CandidateCount      int
	TopK                int
	LatencyMs           int64
	LoadSkew            float64
	SelectedAccountID   int64
	SelectedAccountType string
}

type OpenAIAccountSchedulerMetricsSnapshot struct {
	SelectTotal              int64
	StickyPreviousHitTotal   int64
	StickySessionHitTotal    int64
	LoadBalanceSelectTotal   int64
	AccountSwitchTotal       int64
	SchedulerLatencyMsTotal  int64
	SchedulerLatencyMsAvg    float64
	StickyHitRatio           float64
	AccountSwitchRate        float64
	LoadSkewAvg              float64
	RuntimeStatsAccountCount int
	RuntimeStats             map[int64]OpenAIAccountRuntimeSnapshot
}

type OpenAIAccountScheduler interface {
	Select(ctx context.Context, req OpenAIAccountScheduleRequest) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error)
	ReportResult(accountID int64, success bool, firstTokenMs *int)
	RecordRecoveryReason(accountID int64, reason string)
	ReportSwitch()
	SnapshotMetrics() OpenAIAccountSchedulerMetricsSnapshot
}

type openAIAccountSchedulerMetrics struct {
	selectTotal            atomic.Int64
	stickyPreviousHitTotal atomic.Int64
	stickySessionHitTotal  atomic.Int64
	loadBalanceSelectTotal atomic.Int64
	accountSwitchTotal     atomic.Int64
	latencyMsTotal         atomic.Int64
	loadSkewMilliTotal     atomic.Int64
}

func (m *openAIAccountSchedulerMetrics) recordSelect(decision OpenAIAccountScheduleDecision) {
	if m == nil {
		return
	}
	m.selectTotal.Add(1)
	m.latencyMsTotal.Add(decision.LatencyMs)
	m.loadSkewMilliTotal.Add(int64(math.Round(decision.LoadSkew * 1000)))
	if decision.StickyPreviousHit {
		m.stickyPreviousHitTotal.Add(1)
	}
	if decision.StickySessionHit {
		m.stickySessionHitTotal.Add(1)
	}
	if decision.Layer == openAIAccountScheduleLayerLoadBalance {
		m.loadBalanceSelectTotal.Add(1)
	}
}

func (m *openAIAccountSchedulerMetrics) recordSwitch() {
	if m == nil {
		return
	}
	m.accountSwitchTotal.Add(1)
}

type openAIAccountRuntimeStats struct {
	accounts     sync.Map
	accountCount atomic.Int64
}

type openAIAccountRuntimeStat struct {
	errorRateEWMABits        atomic.Uint64
	errorRateEWMAInitialized atomic.Bool
	ttftEWMABits             atomic.Uint64
	lastRecoveryReason       atomic.Value
}

func newOpenAIAccountRuntimeStats() *openAIAccountRuntimeStats {
	return &openAIAccountRuntimeStats{}
}

func (s *openAIAccountRuntimeStats) loadOrCreate(accountID int64) *openAIAccountRuntimeStat {
	if value, ok := s.accounts.Load(accountID); ok {
		stat, _ := value.(*openAIAccountRuntimeStat)
		if stat != nil {
			return stat
		}
	}

	stat := &openAIAccountRuntimeStat{}
	stat.lastRecoveryReason.Store("")
	stat.ttftEWMABits.Store(math.Float64bits(math.NaN()))
	actual, loaded := s.accounts.LoadOrStore(accountID, stat)
	if !loaded {
		s.accountCount.Add(1)
		return stat
	}
	existing, _ := actual.(*openAIAccountRuntimeStat)
	if existing != nil {
		return existing
	}
	return stat
}

func updateEWMAAtomic(target *atomic.Uint64, sample float64, alpha float64) {
	for {
		oldBits := target.Load()
		oldValue := math.Float64frombits(oldBits)
		newValue := alpha*sample + (1-alpha)*oldValue
		if target.CompareAndSwap(oldBits, math.Float64bits(newValue)) {
			return
		}
	}
}

func (s *openAIAccountRuntimeStats) report(accountID int64, success bool, firstTokenMs *int) {
	if s == nil || accountID <= 0 {
		return
	}
	const alpha = 0.2
	stat := s.loadOrCreate(accountID)

	errorSample := 1.0
	if success {
		errorSample = 0.0
	}
	if stat.errorRateEWMAInitialized.CompareAndSwap(false, true) {
		stat.errorRateEWMABits.Store(math.Float64bits(errorSample))
	} else {
		updateEWMAAtomic(&stat.errorRateEWMABits, errorSample, alpha)
	}

	if firstTokenMs != nil && *firstTokenMs > 0 {
		ttft := float64(*firstTokenMs)
		ttftBits := math.Float64bits(ttft)
		for {
			oldBits := stat.ttftEWMABits.Load()
			oldValue := math.Float64frombits(oldBits)
			if math.IsNaN(oldValue) {
				if stat.ttftEWMABits.CompareAndSwap(oldBits, ttftBits) {
					break
				}
				continue
			}
			newValue := alpha*ttft + (1-alpha)*oldValue
			if stat.ttftEWMABits.CompareAndSwap(oldBits, math.Float64bits(newValue)) {
				break
			}
		}
	}
}

func (s *openAIAccountRuntimeStats) recordRecoveryReason(accountID int64, reason string) {
	if s == nil || accountID <= 0 {
		return
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return
	}
	stat := s.loadOrCreate(accountID)
	stat.lastRecoveryReason.Store(reason)
}

func (s *openAIAccountRuntimeStats) runtimeSnapshot(accountID int64) (OpenAIAccountRuntimeSnapshot, bool) {
	if s == nil || accountID <= 0 {
		return OpenAIAccountRuntimeSnapshot{}, false
	}
	value, ok := s.accounts.Load(accountID)
	if !ok {
		return OpenAIAccountRuntimeSnapshot{}, false
	}
	stat, _ := value.(*openAIAccountRuntimeStat)
	if stat == nil {
		return OpenAIAccountRuntimeSnapshot{}, false
	}
	return newOpenAIAccountRuntimeSnapshot(accountID, stat), true
}

func (s *openAIAccountRuntimeStats) snapshotAll() map[int64]OpenAIAccountRuntimeSnapshot {
	if s == nil {
		return nil
	}
	snapshots := make(map[int64]OpenAIAccountRuntimeSnapshot, s.size())
	s.accounts.Range(func(key, value any) bool {
		accountID, ok := key.(int64)
		if !ok {
			return true
		}
		stat, _ := value.(*openAIAccountRuntimeStat)
		if stat == nil {
			return true
		}
		snapshots[accountID] = newOpenAIAccountRuntimeSnapshot(accountID, stat)
		return true
	})
	if len(snapshots) == 0 {
		return nil
	}
	return snapshots
}

func (s *openAIAccountRuntimeStats) snapshot(accountID int64) (errorRate float64, ttft float64, hasTTFT bool) {
	snapshot, ok := s.runtimeSnapshot(accountID)
	if !ok {
		return 0, 0, false
	}
	if !snapshot.HasTTFTEWMA {
		return snapshot.ErrorRateEWMA, 0, false
	}
	return snapshot.ErrorRateEWMA, snapshot.TTFTEWMA, true
}

func (s *openAIAccountRuntimeStats) size() int {
	if s == nil {
		return 0
	}
	return int(s.accountCount.Load())
}

type defaultOpenAIAccountScheduler struct {
	service *OpenAIGatewayService
	metrics openAIAccountSchedulerMetrics
	stats   *openAIAccountRuntimeStats
}

type openAIStickyEscapeConfig struct {
	enabled   bool
	ttftMs    float64
	errorRate float64
}

func newDefaultOpenAIAccountScheduler(service *OpenAIGatewayService, stats *openAIAccountRuntimeStats) OpenAIAccountScheduler {
	if stats == nil {
		stats = newOpenAIAccountRuntimeStats()
	}
	return &defaultOpenAIAccountScheduler{
		service: service,
		stats:   stats,
	}
}

func (s *defaultOpenAIAccountScheduler) Select(
	ctx context.Context,
	req OpenAIAccountScheduleRequest,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	if strings.TrimSpace(req.RequiredImageRoute) != "" {
		req.RequiredImageRoute = openAIImageRouteForAccountScheduling(req.RequiredImageRoute)
	}
	decision := OpenAIAccountScheduleDecision{}
	start := time.Now()
	defer func() {
		decision.LatencyMs = time.Since(start).Milliseconds()
		s.metrics.recordSelect(decision)
	}()

	previousResponseID := strings.TrimSpace(req.PreviousResponseID)
	if previousResponseID != "" {
		selection, err := s.service.selectAccountByPreviousResponseIDForCapability(
			ctx,
			req.GroupID,
			req.APIKeyID,
			previousResponseID,
			req.RequestedModel,
			req.ExcludedIDs,
			req.RequiredCapability,
			req.RequireCompact,
		)
		if err != nil {
			return nil, decision, err
		}
		if selection != nil && selection.Account != nil {
			if !s.isAccountTransportCompatible(selection.Account, req.RequiredTransport) ||
				!s.isAccountRequestCompatible(ctx, selection.Account, req) {
				if selection.ReleaseFunc != nil {
					selection.ReleaseFunc()
				}
				selection = nil
			}
		}
		if selection != nil && selection.Account != nil {
			decision.Layer = openAIAccountScheduleLayerPreviousResponse
			decision.StickyPreviousHit = true
			decision.SelectedAccountID = selection.Account.ID
			decision.SelectedAccountType = selection.Account.Type
			if req.SessionHash != "" {
				_ = s.service.BindStickySession(ctx, req.GroupID, req.SessionHash, selection.Account.ID)
			}
			return selection, decision, nil
		}
	}

	selection, escapedSticky, err := s.selectBySessionHash(ctx, req)
	if err != nil {
		return nil, decision, err
	}
	if selection != nil && selection.Account != nil {
		decision.Layer = openAIAccountScheduleLayerSessionSticky
		decision.StickySessionHit = true
		decision.SelectedAccountID = selection.Account.ID
		decision.SelectedAccountType = selection.Account.Type
		return selection, decision, nil
	}
	if escapedSticky {
		req.PreserveStickyBinding = true
	}

	selection, candidateCount, topK, loadSkew, err := s.selectByLoadBalance(ctx, req)
	decision.Layer = openAIAccountScheduleLayerLoadBalance
	decision.CandidateCount = candidateCount
	decision.TopK = topK
	decision.LoadSkew = loadSkew
	if err != nil {
		return nil, decision, err
	}
	if selection != nil && selection.Account != nil {
		decision.SelectedAccountID = selection.Account.ID
		decision.SelectedAccountType = selection.Account.Type
	}
	return selection, decision, nil
}

func (s *defaultOpenAIAccountScheduler) selectBySessionHash(
	ctx context.Context,
	req OpenAIAccountScheduleRequest,
) (*AccountSelectionResult, bool, error) {
	sessionHash := strings.TrimSpace(req.SessionHash)
	if sessionHash == "" || s == nil || s.service == nil || s.service.cache == nil {
		return nil, false, nil
	}

	accountID := req.StickyAccountID
	if accountID <= 0 {
		var err error
		accountID, err = s.service.getStickySessionAccountID(ctx, req.GroupID, sessionHash)
		if err != nil || accountID <= 0 {
			return nil, false, nil
		}
	}
	if accountID <= 0 {
		return nil, false, nil
	}
	if req.ExcludedIDs != nil {
		if _, excluded := req.ExcludedIDs[accountID]; excluded {
			return nil, false, nil
		}
	}

	account, err := s.service.getSchedulableAccount(ctx, accountID)
	if err != nil || account == nil {
		_ = s.service.deleteStickySessionAccountID(ctx, req.GroupID, sessionHash)
		return nil, false, nil
	}
	if !s.isStickyAccountWithinSchedulingScope(ctx, account.ID, req) {
		_ = s.service.deleteStickySessionAccountID(ctx, req.GroupID, sessionHash)
		return nil, false, nil
	}
	stickyWaitTimeout := s.service.openAIStickyWaitTimeout(ctx)
	if shouldClearOpenAIStickyAccount(account, req.RequestedModel, req.RequiredImageRoute, stickyWaitTimeout) {
		_ = s.service.deleteStickySessionAccountID(ctx, req.GroupID, sessionHash)
		return nil, false, nil
	}
	if !isOpenAIStickyCandidateCompatible(ctx, s.service.settingService, account, req.RequestedModel, req.RequireCompact, req.RequiredImageRoute, req.RequireOAuthAccount, req.RequireImageEnabled) {
		_ = s.service.deleteStickySessionAccountID(ctx, req.GroupID, sessionHash)
		return nil, false, nil
	}
	if !s.isAccountTransportCompatible(account, req.RequiredTransport) {
		_ = s.service.deleteStickySessionAccountID(ctx, req.GroupID, sessionHash)
		return nil, false, nil
	}
	account = s.service.recheckSelectedStickyOpenAIAccountFromDB(ctx, account, req.RequestedModel, req.RequireCompact, req.RequiredImageRoute, req.RequireOAuthAccount, req.RequireImageEnabled)
	if account == nil ||
		!s.isStickyAccountWithinSchedulingScope(ctx, account.ID, req) ||
		!s.isAccountTransportCompatible(account, req.RequiredTransport) ||
		!s.isAccountRequestCompatible(ctx, account, req) {
		_ = s.service.deleteStickySessionAccountID(ctx, req.GroupID, sessionHash)
		return nil, false, nil
	}
	escapeCfg := s.service.openAIStickyEscapeConfig()
	if reason, errorRate, ttft, shouldEscape := s.shouldEscapeStickyAccount(accountID, escapeCfg); shouldEscape {
		slog.Info("sticky_escape_triggered",
			"account_id", accountID,
			"reason", reason,
			"error_rate", errorRate,
			"ttft", ttft,
		)
		return nil, true, nil
	}

	maxConcurrency := req.MaxConcurrencyFor(account)
	if waitPlan := buildOpenAIAccountWaitPlan(
		account,
		req.RequestedModel,
		req.RequiredImageRoute,
		maxConcurrency,
		stickyWaitTimeout,
		s.service.schedulingConfig().StickySessionMaxWaiting,
	); waitPlan != nil {
		selection, err := s.service.newSelectionResult(ctx, account, false, nil, waitPlan)
		return selection, false, err
	}
	result, acquireErr := s.service.tryAcquireAccountSlot(ctx, accountID, req.GroupID, maxConcurrency)
	if acquireErr == nil && result != nil && result.Acquired {
		_ = s.service.refreshStickySessionTTL(ctx, req.GroupID, sessionHash, s.service.openAIWSSessionStickyTTL())
		selection, err := s.service.newAcquiredSelectionResult(ctx, account, result.ReleaseFunc)
		return selection, false, err
	}

	cfg := s.service.schedulingConfig()
	// WaitPlan.MaxConcurrency uses the scheduler-normalized image route; legacy web2api groups share codex account slots.
	if s.service.concurrencyService != nil {
		selection, err := s.service.newSelectionResult(ctx, account, false, nil, &AccountWaitPlan{
			AccountID:      accountID,
			MaxConcurrency: maxConcurrency,
			Timeout:        stickyWaitTimeout,
			MaxWaiting:     cfg.StickySessionMaxWaiting,
		})
		return selection, false, err
	}
	return nil, false, nil
}

func (s *defaultOpenAIAccountScheduler) isStickyAccountWithinSchedulingScope(ctx context.Context, accountID int64, req OpenAIAccountScheduleRequest) bool {
	if s == nil || s.service == nil || accountID <= 0 {
		return false
	}
	accounts, err := s.service.listSchedulableAccounts(ctx, req.GroupID, req.RequiredImageRoute)
	if err != nil {
		return true
	}
	for i := range accounts {
		if accounts[i].ID == accountID {
			return true
		}
	}
	return false
}

func openAIStickyAccountMatchesGroup(account *Account, groupID *int64) bool {
	if account == nil {
		return false
	}
	if groupID == nil {
		return len(account.AccountGroups) == 0 && len(account.GroupIDs) == 0
	}
	for _, accountGroupID := range account.GroupIDs {
		if accountGroupID == *groupID {
			return true
		}
	}
	for _, accountGroup := range account.AccountGroups {
		if accountGroup.GroupID == *groupID {
			return true
		}
	}
	return false
}

func (s *defaultOpenAIAccountScheduler) shouldEscapeStickyAccount(accountID int64, cfg openAIStickyEscapeConfig) (reason string, errorRate float64, ttft float64, shouldEscape bool) {
	if !cfg.enabled || s == nil || s.stats == nil || accountID <= 0 {
		return "", 0, 0, false
	}
	errorRate, ttft, hasTTFT := s.stats.snapshot(accountID)
	if hasTTFT && ttft > cfg.ttftMs {
		return "ttft", errorRate, ttft, true
	}
	if errorRate > cfg.errorRate {
		return "error_rate", errorRate, ttft, true
	}
	return "", errorRate, ttft, false
}

type openAIAccountCandidateScore struct {
	account   *Account
	loadInfo  *AccountLoadInfo
	score     float64
	errorRate float64
	ttft      float64
	hasTTFT   bool
}

type openAIAccountCandidateHeap []openAIAccountCandidateScore

func (h openAIAccountCandidateHeap) Len() int {
	return len(h)
}

func (h openAIAccountCandidateHeap) Less(i, j int) bool {
	// 最小堆根节点保存“最差”候选，便于 O(log k) 维护 topK。
	return isOpenAIAccountCandidateBetter(h[j], h[i])
}

func (h openAIAccountCandidateHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *openAIAccountCandidateHeap) Push(x any) {
	candidate, ok := x.(openAIAccountCandidateScore)
	if !ok {
		panic("openAIAccountCandidateHeap: invalid element type")
	}
	*h = append(*h, candidate)
}

func (h *openAIAccountCandidateHeap) Pop() any {
	old := *h
	n := len(old)
	last := old[n-1]
	*h = old[:n-1]
	return last
}

func isOpenAIAccountCandidateBetter(left openAIAccountCandidateScore, right openAIAccountCandidateScore) bool {
	if left.score != right.score {
		return left.score > right.score
	}
	if left.account.Priority != right.account.Priority {
		return left.account.Priority < right.account.Priority
	}
	if left.loadInfo.LoadRate != right.loadInfo.LoadRate {
		return left.loadInfo.LoadRate < right.loadInfo.LoadRate
	}
	if left.loadInfo.WaitingCount != right.loadInfo.WaitingCount {
		return left.loadInfo.WaitingCount < right.loadInfo.WaitingCount
	}
	return left.account.ID < right.account.ID
}

func selectTopKOpenAICandidates(candidates []openAIAccountCandidateScore, topK int) []openAIAccountCandidateScore {
	if len(candidates) == 0 {
		return nil
	}
	if topK <= 0 {
		topK = 1
	}
	if topK >= len(candidates) {
		ranked := append([]openAIAccountCandidateScore(nil), candidates...)
		sort.Slice(ranked, func(i, j int) bool {
			return isOpenAIAccountCandidateBetter(ranked[i], ranked[j])
		})
		return ranked
	}

	best := make(openAIAccountCandidateHeap, 0, topK)
	for _, candidate := range candidates {
		if len(best) < topK {
			heap.Push(&best, candidate)
			continue
		}
		if isOpenAIAccountCandidateBetter(candidate, best[0]) {
			best[0] = candidate
			heap.Fix(&best, 0)
		}
	}

	ranked := make([]openAIAccountCandidateScore, len(best))
	copy(ranked, best)
	sort.Slice(ranked, func(i, j int) bool {
		return isOpenAIAccountCandidateBetter(ranked[i], ranked[j])
	})
	return ranked
}

type openAISelectionRNG struct {
	state uint64
}

func newOpenAISelectionRNG(seed uint64) openAISelectionRNG {
	if seed == 0 {
		seed = 0x9e3779b97f4a7c15
	}
	return openAISelectionRNG{state: seed}
}

func (r *openAISelectionRNG) nextUint64() uint64 {
	// xorshift64*
	x := r.state
	x ^= x >> 12
	x ^= x << 25
	x ^= x >> 27
	r.state = x
	return x * 2685821657736338717
}

func (r *openAISelectionRNG) nextFloat64() float64 {
	// [0,1)
	return float64(r.nextUint64()>>11) / (1 << 53)
}

func deriveOpenAISelectionSeed(req OpenAIAccountScheduleRequest) uint64 {
	hasher := fnv.New64a()
	writeValue := func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		_, _ = hasher.Write([]byte(trimmed))
		_, _ = hasher.Write([]byte{0})
	}

	writeValue(req.SessionHash)
	writeValue(req.PreviousResponseID)
	writeValue(req.RequestedModel)
	if req.GroupID != nil {
		_, _ = hasher.Write([]byte(strconv.FormatInt(*req.GroupID, 10)))
	}

	seed := hasher.Sum64()
	// 对“无会话锚点”的纯负载均衡请求引入时间熵，避免固定命中同一账号。
	if strings.TrimSpace(req.SessionHash) == "" && strings.TrimSpace(req.PreviousResponseID) == "" {
		seed ^= uint64(time.Now().UnixNano())
	}
	if seed == 0 {
		seed = uint64(time.Now().UnixNano()) ^ 0x9e3779b97f4a7c15
	}
	return seed
}

func buildOpenAIWeightedSelectionOrder(
	candidates []openAIAccountCandidateScore,
	req OpenAIAccountScheduleRequest,
) []openAIAccountCandidateScore {
	if len(candidates) <= 1 {
		return append([]openAIAccountCandidateScore(nil), candidates...)
	}

	pool := append([]openAIAccountCandidateScore(nil), candidates...)
	weights := make([]float64, len(pool))
	minScore := pool[0].score
	for i := 1; i < len(pool); i++ {
		if pool[i].score < minScore {
			minScore = pool[i].score
		}
	}
	for i := range pool {
		// 将 top-K 分值平移到正区间，避免“单一最高分账号”长期垄断。
		weight := (pool[i].score - minScore) + 1.0
		if math.IsNaN(weight) || math.IsInf(weight, 0) || weight <= 0 {
			weight = 1.0
		}
		weights[i] = weight
	}

	order := make([]openAIAccountCandidateScore, 0, len(pool))
	rng := newOpenAISelectionRNG(deriveOpenAISelectionSeed(req))
	for len(pool) > 0 {
		total := 0.0
		for _, w := range weights {
			total += w
		}

		selectedIdx := 0
		if total > 0 {
			r := rng.nextFloat64() * total
			acc := 0.0
			for i, w := range weights {
				acc += w
				if r <= acc {
					selectedIdx = i
					break
				}
			}
		} else {
			selectedIdx = int(rng.nextUint64() % uint64(len(pool)))
		}

		order = append(order, pool[selectedIdx])
		pool = append(pool[:selectedIdx], pool[selectedIdx+1:]...)
		weights = append(weights[:selectedIdx], weights[selectedIdx+1:]...)
	}
	return order
}

func (s *defaultOpenAIAccountScheduler) selectByLoadBalance(
	ctx context.Context,
	req OpenAIAccountScheduleRequest,
) (*AccountSelectionResult, int, int, float64, error) {
	accounts, err := s.service.listSchedulableAccounts(ctx, req.GroupID, req.RequiredImageRoute)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	if len(accounts) == 0 {
		return nil, 0, 0, 0, noAvailableOpenAISelectionErrorWithRouting(ctx, s.service.settingService, req.RequestedModel, false, req.RequireCompact, accounts)
	}

	// require_privacy_set: 获取分组信息
	var schedGroup *Group
	if req.GroupID != nil && s.service.schedulerSnapshot != nil {
		schedGroup, _ = s.service.schedulerSnapshot.GetGroupByID(ctx, *req.GroupID)
	}
	errorAccounts := s.selectionErrorAccounts(ctx, req, accounts, schedGroup)

	filtered := make([]*Account, 0, len(accounts))
	loadReq := make([]AccountWithConcurrency, 0, len(accounts))
	for i := range accounts {
		account := &accounts[i]
		if req.ExcludedIDs != nil {
			if _, excluded := req.ExcludedIDs[account.ID]; excluded {
				continue
			}
		}
		if !s.isLoadBalanceAccountSchedulableForRequest(account, req) {
			continue
		}
		if s.service.isOpenAIAccountRuntimeBlocked(account) {
			continue
		}
		// require_privacy_set: 跳过 privacy 未设置的账号并标记异常
		if schedGroup != nil && schedGroup.RequirePrivacySet && !account.IsPrivacySet() {
			s.service.BlockAccountScheduling(account, time.Time{}, "privacy_not_set")
			_ = s.service.accountRepo.SetError(ctx, account.ID,
				fmt.Sprintf("Privacy not set, required by group [%s]", schedGroup.Name))
			continue
		}
		if !s.isAccountRequestCompatible(ctx, account, req) {
			continue
		}
		if !s.isAccountTransportCompatible(account, req.RequiredTransport) {
			continue
		}
		filtered = append(filtered, account)
		loadReq = append(loadReq, AccountWithConcurrency{
			ID:             account.ID,
			MaxConcurrency: account.EffectiveLoadFactor(),
		})
	}
	if len(filtered) == 0 {
		return nil, 0, 0, 0, noAvailableOpenAISelectionErrorWithRouting(ctx, s.service.settingService, req.RequestedModel, false, req.RequireCompact, errorAccounts)
	}

	loadMap := map[int64]*AccountLoadInfo{}
	if s.service.concurrencyService != nil {
		if batchLoad, loadErr := s.service.concurrencyService.GetAccountsLoadBatch(ctx, loadReq); loadErr == nil {
			loadMap = batchLoad
		}
	}

	allCandidates := make([]openAIAccountCandidateScore, 0, len(filtered))
	eligibleNewSessionCandidates := make([]openAIAccountCandidateScore, 0, len(filtered))
	for _, account := range filtered {
		loadInfo := loadMap[account.ID]
		if loadInfo == nil {
			loadInfo = &AccountLoadInfo{AccountID: account.ID}
		}
		errorRate, ttft, hasTTFT := s.stats.snapshot(account.ID)
		candidate := openAIAccountCandidateScore{
			account:   account,
			loadInfo:  loadInfo,
			errorRate: errorRate,
			ttft:      ttft,
			hasTTFT:   hasTTFT,
		}
		allCandidates = append(allCandidates, candidate)
		if loadInfo.CurrentConcurrency < s.service.freshSessionAdmissionLimit(ctx, account, req.RequiredImageRoute) {
			eligibleNewSessionCandidates = append(eligibleNewSessionCandidates, candidate)
		}
	}

	// Compact 模式下把明确不支持 compact 的账号拆出，仅在 schedulerSnapshot 启用
	// 时作为最后兜底（snapshot 可能已陈旧）。
	candidates := eligibleNewSessionCandidates
	waitCandidates := allCandidates
	staleSnapshotCompactRetry := make([]openAIAccountCandidateScore, 0, len(eligibleNewSessionCandidates))
	waitStaleSnapshotCompactRetry := make([]openAIAccountCandidateScore, 0, len(allCandidates))
	if req.RequireCompact {
		candidates = make([]openAIAccountCandidateScore, 0, len(eligibleNewSessionCandidates))
		for _, candidate := range eligibleNewSessionCandidates {
			if openAICompactSupportTier(candidate.account) == 0 {
				staleSnapshotCompactRetry = append(staleSnapshotCompactRetry, candidate)
				continue
			}
			candidates = append(candidates, candidate)
		}
		waitCandidates = make([]openAIAccountCandidateScore, 0, len(allCandidates))
		for _, candidate := range allCandidates {
			if openAICompactSupportTier(candidate.account) == 0 {
				waitStaleSnapshotCompactRetry = append(waitStaleSnapshotCompactRetry, candidate)
				continue
			}
			waitCandidates = append(waitCandidates, candidate)
		}
		if len(waitCandidates) == 0 && len(waitStaleSnapshotCompactRetry) == 0 {
			return nil, 0, 0, 0, ErrNoAvailableCompactAccounts
		}
	}

	candidateCount := len(candidates)
	loadSkew := 0.0
	if len(candidates) > 0 {
		minPriority, maxPriority := candidates[0].account.Priority, candidates[0].account.Priority
		maxWaiting := 1
		loadRateSum := 0.0
		loadRateSumSquares := 0.0
		minTTFT, maxTTFT := 0.0, 0.0
		hasTTFTSample := false
		for _, candidate := range candidates {
			if candidate.account.Priority < minPriority {
				minPriority = candidate.account.Priority
			}
			if candidate.account.Priority > maxPriority {
				maxPriority = candidate.account.Priority
			}
			if candidate.loadInfo.WaitingCount > maxWaiting {
				maxWaiting = candidate.loadInfo.WaitingCount
			}
			if candidate.hasTTFT && candidate.ttft > 0 {
				if !hasTTFTSample {
					minTTFT, maxTTFT = candidate.ttft, candidate.ttft
					hasTTFTSample = true
				} else {
					if candidate.ttft < minTTFT {
						minTTFT = candidate.ttft
					}
					if candidate.ttft > maxTTFT {
						maxTTFT = candidate.ttft
					}
				}
			}
			loadRate := float64(candidate.loadInfo.LoadRate)
			loadRateSum += loadRate
			loadRateSumSquares += loadRate * loadRate
		}
		loadSkew = calcLoadSkewByMoments(loadRateSum, loadRateSumSquares, len(candidates))

		weights := s.service.openAIWSSchedulerWeights()
		for i := range candidates {
			item := &candidates[i]
			priorityFactor := 1.0
			if maxPriority > minPriority {
				priorityFactor = 1 - float64(item.account.Priority-minPriority)/float64(maxPriority-minPriority)
			}
			loadFactor := 1 - clamp01(float64(item.loadInfo.LoadRate)/100.0)
			queueFactor := 1 - clamp01(float64(item.loadInfo.WaitingCount)/float64(maxWaiting))
			errorFactor := 1 - clamp01(item.errorRate)
			ttftFactor := 0.5
			if item.hasTTFT && hasTTFTSample && maxTTFT > minTTFT {
				ttftFactor = 1 - clamp01((item.ttft-minTTFT)/(maxTTFT-minTTFT))
			}

			item.score = weights.Priority*priorityFactor +
				weights.Load*loadFactor +
				weights.Queue*queueFactor +
				weights.ErrorRate*errorFactor +
				weights.TTFT*ttftFactor
		}
	}

	topK := 0
	if len(candidates) > 0 {
		topK = s.service.openAIWSLBTopK()
		if topK > len(candidates) {
			topK = len(candidates)
		}
		if topK <= 0 {
			topK = 1
		}
	}

	buildSelectionOrder := func(pool []openAIAccountCandidateScore) []openAIAccountCandidateScore {
		if len(pool) == 0 {
			return nil
		}
		poolTopK := s.service.openAIWSLBTopK()
		if poolTopK > len(pool) {
			poolTopK = len(pool)
		}
		if poolTopK <= 0 {
			poolTopK = 1
		}
		return reorderOpenAIImageRouteRateLimitedCandidates(
			buildOpenAIWeightedSelectionOrder(selectTopKOpenAICandidates(pool, poolTopK), req),
			req.RequiredImageRoute,
		)
	}
	sortCompactRetryCandidates := func(pool []openAIAccountCandidateScore) []openAIAccountCandidateScore {
		if len(pool) == 0 {
			return nil
		}
		ordered := append([]openAIAccountCandidateScore(nil), pool...)
		sort.SliceStable(ordered, func(i, j int) bool {
			return lessOpenAINewSessionCandidate(
				ordered[i],
				ordered[j],
				s.service.freshSessionAdmissionLimit(ctx, ordered[i].account, req.RequiredImageRoute),
				s.service.freshSessionAdmissionLimit(ctx, ordered[j].account, req.RequiredImageRoute),
			)
		})
		return reorderOpenAIImageRouteRateLimitedCandidates(ordered, req.RequiredImageRoute)
	}

	selectionOrder := make([]openAIAccountCandidateScore, 0, len(allCandidates))
	waitSelectionOrder := make([]openAIAccountCandidateScore, 0, len(allCandidates))
	if req.RequireCompact {
		supported := make([]openAIAccountCandidateScore, 0, len(candidates))
		unknown := make([]openAIAccountCandidateScore, 0, len(candidates))
		for _, candidate := range candidates {
			switch openAICompactSupportTier(candidate.account) {
			case 2:
				supported = append(supported, candidate)
			case 1:
				unknown = append(unknown, candidate)
			}
		}
		if len(supported) == 0 && len(unknown) == 0 && s.service.schedulerSnapshot == nil {
			return nil, candidateCount, topK, loadSkew, ErrNoAvailableCompactAccounts
		}
		selectionOrder = append(selectionOrder, buildSelectionOrder(supported)...)
		selectionOrder = append(selectionOrder, buildSelectionOrder(unknown)...)
		if len(staleSnapshotCompactRetry) > 0 && s.service.schedulerSnapshot != nil {
			selectionOrder = append(selectionOrder, sortCompactRetryCandidates(staleSnapshotCompactRetry)...)
		}

		waitSupported := make([]openAIAccountCandidateScore, 0, len(waitCandidates))
		waitUnknown := make([]openAIAccountCandidateScore, 0, len(waitCandidates))
		for _, candidate := range waitCandidates {
			switch openAICompactSupportTier(candidate.account) {
			case 2:
				waitSupported = append(waitSupported, candidate)
			case 1:
				waitUnknown = append(waitUnknown, candidate)
			}
		}
		waitSelectionOrder = append(waitSelectionOrder, buildSelectionOrder(waitSupported)...)
		waitSelectionOrder = append(waitSelectionOrder, buildSelectionOrder(waitUnknown)...)
		if len(waitStaleSnapshotCompactRetry) > 0 && s.service.schedulerSnapshot != nil {
			waitSelectionOrder = append(waitSelectionOrder, sortCompactRetryCandidates(waitStaleSnapshotCompactRetry)...)
		}
	} else {
		selectionOrder = buildSelectionOrder(candidates)
		waitSelectionOrder = buildSelectionOrder(waitCandidates)
	}
	if len(selectionOrder) == 0 && len(waitSelectionOrder) == 0 {
		return nil, candidateCount, topK, loadSkew, noAvailableOpenAISelectionErrorWithRouting(ctx, s.service.settingService, req.RequestedModel, req.RequireCompact && len(allCandidates) > 0, req.RequireCompact, errorAccounts)
	}

	compactBlocked := false
	for i := 0; i < len(selectionOrder); i++ {
		candidate := selectionOrder[i]
		fresh := s.service.resolveFreshSchedulableOpenAIAccount(ctx, candidate.account, req.RequestedModel, false, req.RequiredImageRoute)
		if fresh == nil || !s.isAccountTransportCompatible(fresh, req.RequiredTransport) || !s.isAccountRequestCompatible(ctx, fresh, req) {
			continue
		}
		fresh = s.service.recheckSelectedOpenAIAccountFromDB(ctx, fresh, req.RequestedModel, false, req.RequiredImageRoute)
		if fresh == nil || !s.isAccountTransportCompatible(fresh, req.RequiredTransport) || !s.isAccountRequestCompatible(ctx, fresh, req) {
			continue
		}
		if req.RequireCompact && openAICompactSupportTier(fresh) == 0 {
			compactBlocked = true
			continue
		}
		maxConcurrency := s.service.freshSessionAdmissionLimit(ctx, fresh, req.RequiredImageRoute)
		if waitPlan := buildOpenAIAccountWaitPlan(
			fresh,
			req.RequestedModel,
			req.RequiredImageRoute,
			maxConcurrency,
			s.service.schedulingConfig().FallbackWaitTimeout,
			s.service.schedulingConfig().FallbackMaxWaiting,
		); waitPlan != nil {
			if req.SessionHash != "" {
				_ = s.service.BindStickySession(ctx, req.GroupID, req.SessionHash, fresh.ID)
			}
			selection, err := s.service.newSelectionResult(ctx, fresh, false, nil, waitPlan)
			return selection, candidateCount, topK, loadSkew, err
		}
		if hasOpenAIAccountTemporaryRecoveryPending(fresh, req.RequestedModel, req.RequiredImageRoute) {
			continue
		}
		result, acquireErr := s.service.tryAcquireAccountSlot(ctx, fresh.ID, req.GroupID, maxConcurrency)
		if acquireErr != nil {
			return nil, candidateCount, topK, loadSkew, acquireErr
		}
		if result != nil && result.Acquired {
			if req.SessionHash != "" {
				_ = s.service.BindStickySession(ctx, req.GroupID, req.SessionHash, fresh.ID)
			}
			selection, err := s.service.newAcquiredSelectionResult(ctx, fresh, result.ReleaseFunc)
			return selection, candidateCount, topK, loadSkew, err
		}
	}

	cfg := s.service.schedulingConfig()
	// WaitPlan.MaxConcurrency uses the scheduler-normalized image route; legacy web2api groups share codex account slots.
	for _, candidate := range waitSelectionOrder {
		fresh := s.service.resolveFreshSchedulableOpenAIAccount(ctx, candidate.account, req.RequestedModel, false, req.RequiredImageRoute)
		if fresh == nil || !s.isAccountTransportCompatible(fresh, req.RequiredTransport) || !s.isAccountRequestCompatible(ctx, fresh, req) {
			continue
		}
		fresh = s.service.recheckSelectedOpenAIAccountFromDB(ctx, fresh, req.RequestedModel, false, req.RequiredImageRoute)
		if fresh == nil || !s.isAccountTransportCompatible(fresh, req.RequiredTransport) || !s.isAccountRequestCompatible(ctx, fresh, req) {
			continue
		}
		if req.RequireCompact && openAICompactSupportTier(fresh) == 0 {
			compactBlocked = true
			continue
		}
		if waitPlan := buildOpenAIAccountWaitPlan(
			fresh,
			req.RequestedModel,
			req.RequiredImageRoute,
			s.service.freshSessionAdmissionLimit(ctx, fresh, req.RequiredImageRoute),
			cfg.FallbackWaitTimeout,
			cfg.FallbackMaxWaiting,
		); waitPlan != nil {
			selection, err := s.service.newSelectionResult(ctx, fresh, false, nil, waitPlan)
			return selection, candidateCount, topK, loadSkew, err
		}
		if hasOpenAIAccountTemporaryRecoveryPending(fresh, req.RequestedModel, req.RequiredImageRoute) {
			continue
		}
		selection, err := s.service.newSelectionResult(ctx, fresh, false, nil, &AccountWaitPlan{
			AccountID:      fresh.ID,
			MaxConcurrency: s.service.freshSessionAdmissionLimit(ctx, fresh, req.RequiredImageRoute),
			Timeout:        cfg.FallbackWaitTimeout,
			MaxWaiting:     cfg.FallbackMaxWaiting,
		})
		return selection, candidateCount, topK, loadSkew, err
	}

	return nil, candidateCount, topK, loadSkew, noAvailableOpenAISelectionErrorWithRouting(ctx, s.service.settingService, req.RequestedModel, compactBlocked, req.RequireCompact, errorAccounts)
}

func (s *defaultOpenAIAccountScheduler) selectionErrorAccounts(ctx context.Context, req OpenAIAccountScheduleRequest, accounts []Account, schedGroup *Group) []Account {
	if s == nil || s.service == nil {
		return nil
	}
	return s.service.openAISelectionErrorAccounts(ctx, accounts, req.RequestedModel, req.RequireCompact, req.ExcludedIDs, func(account *Account) bool {
		if !s.isLoadBalanceAccountSchedulableForRequest(account, req) {
			return false
		}
		if paused, _ := shouldAutoPauseOpenAIAccountByQuota(ctx, account); paused {
			return false
		}
		if schedGroup != nil && schedGroup.RequirePrivacySet && !account.IsPrivacySet() {
			return false
		}
		if !s.isAccountTransportCompatible(account, req.RequiredTransport) {
			return false
		}
		if !accountSupportsOpenAICapabilities(account, req.RequiredCapability, req.RequiredImageCapability) {
			return false
		}
		if req.GroupID != nil && s.service.needsUpstreamChannelRestrictionCheck(ctx, req.GroupID) &&
			s.service.isUpstreamModelRestrictedByChannel(ctx, *req.GroupID, account, req.RequestedModel, req.RequireCompact) {
			return false
		}
		return true
	})
}

func (s *defaultOpenAIAccountScheduler) isAccountTransportCompatible(account *Account, requiredTransport OpenAIUpstreamTransport) bool {
	if requiredTransport == OpenAIUpstreamTransportAny || requiredTransport == OpenAIUpstreamTransportHTTPSSE {
		return true
	}
	if s == nil || s.service == nil {
		return false
	}
	return s.service.isOpenAIAccountTransportCompatible(account, requiredTransport)
}

func (s *defaultOpenAIAccountScheduler) isAccountRequestCompatible(ctx context.Context, account *Account, req OpenAIAccountScheduleRequest) bool {
	if account == nil {
		return false
	}
	if req.RequireImageEnabled && !account.OpenAIImageGenerationAllowed() {
		return false
	}
	if req.RequireOAuthAccount && !account.IsOpenAIOAuth() {
		return false
	}
	if req.RequiredImageRoute != "" && !account.IsSelectableForOpenAIImageRoute(req.RequiredImageRoute, allowRateLimitedOpenAIImageRouteScheduling(req.RequiredImageRoute)) {
		return false
	}
	if s != nil && s.service != nil && s.service.isOpenAIAccountRuntimeBlocked(account) {
		return false
	}
	if paused, reason := shouldAutoPauseOpenAIAccountByQuota(ctx, account); paused {
		slog.Debug("account_auto_paused_by_quota",
			"account_id", account.ID,
			"window", reason.window,
			"threshold", reason.threshold,
			"utilization", reason.utilization,
		)
		return false
	}
	if NormalizeGroupImageGenerationRoute(req.RequiredImageRoute) == GroupImageGenerationRouteWeb2API && !account.HasOpenAIImageWeb2APIProfile() {
		return false
	}
	if req.RequestedModel != "" && !ResolveEffectiveModelRouting(ctx, s.service.settingService, account, req.RequestedModel, req.RequireCompact).Supported {
		return false
	}
	if req.GroupID != nil && s != nil && s.service != nil &&
		s.service.needsUpstreamChannelRestrictionCheck(ctx, req.GroupID) &&
		s.service.isUpstreamModelRestrictedByChannel(ctx, *req.GroupID, account, req.RequestedModel, req.RequireCompact) {
		return false
	}
	return accountSupportsOpenAICapabilities(account, req.RequiredCapability, req.RequiredImageCapability)
}

func accountSupportsOpenAICapabilities(account *Account, endpointCapability OpenAIEndpointCapability, imageCapability OpenAIImagesCapability) bool {
	if account == nil {
		return false
	}
	if !account.SupportsOpenAIEndpointCapability(endpointCapability) {
		return false
	}
	if imageCapability != "" && !account.SupportsOpenAIImageCapability(imageCapability) {
		return false
	}
	return true
}

func (s *defaultOpenAIAccountScheduler) isLoadBalanceAccountSchedulableForRequest(account *Account, req OpenAIAccountScheduleRequest) bool {
	if account == nil || !account.IsOpenAI() {
		return false
	}
	if req.RequireImageEnabled && !account.OpenAIImageGenerationAllowed() {
		return false
	}
	if req.RequireOAuthAccount && !account.IsOpenAIOAuth() {
		return false
	}
	if req.RequiredImageRoute != "" {
		if !account.IsSelectableForOpenAIImageRoute(req.RequiredImageRoute, allowRateLimitedOpenAIImageRouteScheduling(req.RequiredImageRoute)) {
			return false
		}
		if NormalizeGroupImageGenerationRoute(req.RequiredImageRoute) == GroupImageGenerationRouteWeb2API && !account.HasOpenAIImageWeb2APIProfile() {
			return false
		}
		return true
	}
	return account.IsSchedulable()
}

func (s *defaultOpenAIAccountScheduler) ReportResult(accountID int64, success bool, firstTokenMs *int) {
	if s == nil || s.stats == nil {
		return
	}
	s.stats.report(accountID, success, firstTokenMs)
}

func (s *defaultOpenAIAccountScheduler) RecordRecoveryReason(accountID int64, reason string) {
	if s == nil || s.stats == nil {
		return
	}
	s.stats.recordRecoveryReason(accountID, reason)
}

func (s *defaultOpenAIAccountScheduler) ReportSwitch() {
	if s == nil {
		return
	}
	s.metrics.recordSwitch()
}

func (s *defaultOpenAIAccountScheduler) SnapshotMetrics() OpenAIAccountSchedulerMetricsSnapshot {
	if s == nil {
		return OpenAIAccountSchedulerMetricsSnapshot{}
	}
	stats := s.stats

	selectTotal := s.metrics.selectTotal.Load()
	prevHit := s.metrics.stickyPreviousHitTotal.Load()
	sessionHit := s.metrics.stickySessionHitTotal.Load()
	switchTotal := s.metrics.accountSwitchTotal.Load()
	latencyTotal := s.metrics.latencyMsTotal.Load()
	loadSkewTotal := s.metrics.loadSkewMilliTotal.Load()

	runtimeStatsCount := 0
	var runtimeStats map[int64]OpenAIAccountRuntimeSnapshot
	if stats != nil {
		runtimeStatsCount = stats.size()
		runtimeStats = stats.snapshotAll()
	}

	snapshot := OpenAIAccountSchedulerMetricsSnapshot{
		SelectTotal:              selectTotal,
		StickyPreviousHitTotal:   prevHit,
		StickySessionHitTotal:    sessionHit,
		LoadBalanceSelectTotal:   s.metrics.loadBalanceSelectTotal.Load(),
		AccountSwitchTotal:       switchTotal,
		SchedulerLatencyMsTotal:  latencyTotal,
		RuntimeStatsAccountCount: runtimeStatsCount,
		RuntimeStats:             runtimeStats,
	}
	if selectTotal > 0 {
		snapshot.SchedulerLatencyMsAvg = float64(latencyTotal) / float64(selectTotal)
		snapshot.StickyHitRatio = float64(prevHit+sessionHit) / float64(selectTotal)
		snapshot.AccountSwitchRate = float64(switchTotal) / float64(selectTotal)
		snapshot.LoadSkewAvg = float64(loadSkewTotal) / 1000 / float64(selectTotal)
	}
	return snapshot
}

func (s *OpenAIGatewayService) openAISettingsRepo() SettingRepository {
	if s == nil {
		return nil
	}
	if s.settingService != nil && s.settingService.settingRepo != nil {
		return s.settingService.settingRepo
	}
	if s.rateLimitService != nil && s.rateLimitService.settingService != nil {
		return s.rateLimitService.settingService.settingRepo
	}
	return nil
}

func (s *OpenAIGatewayService) isOpenAIAdvancedSchedulerEnabled(ctx context.Context) bool {
	if cached, ok := openAIAdvancedSchedulerSettingCache.Load().(*cachedOpenAIAdvancedSchedulerSetting); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.enabled
		}
	}

	result, _, _ := openAIAdvancedSchedulerSettingSF.Do(openAIAdvancedSchedulerSettingKey, func() (any, error) {
		if cached, ok := openAIAdvancedSchedulerSettingCache.Load().(*cachedOpenAIAdvancedSchedulerSetting); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.enabled, nil
			}
		}

		enabled := false
		if repo := s.openAISettingsRepo(); repo != nil {
			dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAIAdvancedSchedulerSettingDBTimeout)
			defer cancel()

			value, err := repo.GetValue(dbCtx, openAIAdvancedSchedulerSettingKey)
			if err == nil {
				enabled = strings.EqualFold(strings.TrimSpace(value), "true")
			}
		}

		openAIAdvancedSchedulerSettingCache.Store(&cachedOpenAIAdvancedSchedulerSetting{
			enabled:   enabled,
			expiresAt: time.Now().Add(openAIAdvancedSchedulerSettingCacheTTL).UnixNano(),
		})
		return enabled, nil
	})

	enabled, _ := result.(bool)
	return enabled
}

func (s *OpenAIGatewayService) openAIStickyReservePercent(ctx context.Context) int {
	if cached, ok := openAIStickyReservePercentSettingCache.Load().(*cachedOpenAIStickyReservePercentSetting); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.percent
		}
	}

	result, _, _ := openAIStickyReservePercentSettingSF.Do(SettingKeyOpenAIStickyReservePercent, func() (any, error) {
		if cached, ok := openAIStickyReservePercentSettingCache.Load().(*cachedOpenAIStickyReservePercentSetting); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.percent, nil
			}
		}

		percent := defaultOpenAIWSStickyReservePercent
		if repo := s.openAISettingsRepo(); repo != nil {
			dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAIAdvancedSchedulerSettingDBTimeout)
			defer cancel()

			value, err := repo.GetValue(dbCtx, SettingKeyOpenAIStickyReservePercent)
			if err == nil {
				if parsed, parseErr := strconv.Atoi(strings.TrimSpace(value)); parseErr == nil {
					if parsed < 0 {
						parsed = 0
					}
					if parsed > 100 {
						parsed = 100
					}
					percent = parsed
				}
			}
		}

		openAIStickyReservePercentSettingCache.Store(&cachedOpenAIStickyReservePercentSetting{
			percent:   percent,
			expiresAt: time.Now().Add(openAIAdvancedSchedulerSettingCacheTTL).UnixNano(),
		})
		return percent, nil
	})

	percent, _ := result.(int)
	return percent
}

func (s *OpenAIGatewayService) openAIStickyWaitTimeout(ctx context.Context) time.Duration {
	if cached, ok := openAIStickyWaitTimeoutSettingCache.Load().(*cachedOpenAIStickyWaitTimeoutSetting); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.timeout
		}
	}

	result, _, _ := openAIStickyWaitTimeoutSettingSF.Do(SettingKeyOpenAIStickyWaitTimeoutSeconds, func() (any, error) {
		if cached, ok := openAIStickyWaitTimeoutSettingCache.Load().(*cachedOpenAIStickyWaitTimeoutSetting); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.timeout, nil
			}
		}

		timeout := 30 * time.Second
		if repo := s.openAISettingsRepo(); repo != nil {
			dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAIAdvancedSchedulerSettingDBTimeout)
			defer cancel()

			value, err := repo.GetValue(dbCtx, SettingKeyOpenAIStickyWaitTimeoutSeconds)
			if err == nil {
				if parsed, parseErr := strconv.Atoi(strings.TrimSpace(value)); parseErr == nil {
					timeout = time.Duration(boundedIntOrDefault(parsed, 1, 300, 30)) * time.Second
				}
			}
		}

		openAIStickyWaitTimeoutSettingCache.Store(&cachedOpenAIStickyWaitTimeoutSetting{
			timeout:   timeout,
			expiresAt: time.Now().Add(openAIAdvancedSchedulerSettingCacheTTL).UnixNano(),
		})
		return timeout, nil
	})

	timeout, _ := result.(time.Duration)
	if timeout <= 0 {
		return 30 * time.Second
	}
	return timeout
}

func (s *OpenAIGatewayService) openAIOAuthImageBridgeTransportSettings(ctx context.Context) cachedOpenAIOAuthImageBridgeTransportSettings {
	if cached, ok := openAIOAuthImageBridgeTransportSettingsCache.Load().(*cachedOpenAIOAuthImageBridgeTransportSettings); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return *cached
		}
	}

	result, _, _ := openAIOAuthImageBridgeTransportSettingsSF.Do(openAIOAuthImageBridgeTransportSettingsKey, func() (any, error) {
		if cached, ok := openAIOAuthImageBridgeTransportSettingsCache.Load().(*cachedOpenAIOAuthImageBridgeTransportSettings); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return *cached, nil
			}
		}

		settings := cachedOpenAIOAuthImageBridgeTransportSettings{}
		if s != nil && s.cfg != nil {
			settings.disableKeepAlives = s.cfg.Gateway.OpenAIOAuthImageBridgeDisableKeepAlives
			settings.freshClient = s.cfg.Gateway.OpenAIOAuthImageBridgeFreshUpstreamClient
		}
		if repo := s.openAISettingsRepo(); repo != nil {
			dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAIAdvancedSchedulerSettingDBTimeout)
			defer cancel()

			values, err := repo.GetMultiple(dbCtx, []string{
				SettingKeyOpenAIOAuthImageBridgeDisableKeepAlives,
				SettingKeyOpenAIOAuthImageBridgeFreshUpstreamClient,
			})
			if err == nil {
				if raw, ok := values[SettingKeyOpenAIOAuthImageBridgeDisableKeepAlives]; ok && strings.TrimSpace(raw) != "" {
					settings.disableKeepAlives = strings.EqualFold(strings.TrimSpace(raw), "true")
				}
				if raw, ok := values[SettingKeyOpenAIOAuthImageBridgeFreshUpstreamClient]; ok && strings.TrimSpace(raw) != "" {
					settings.freshClient = strings.EqualFold(strings.TrimSpace(raw), "true")
				}
			}
		}

		settings.expiresAt = time.Now().Add(openAIAdvancedSchedulerSettingCacheTTL).UnixNano()
		openAIOAuthImageBridgeTransportSettingsCache.Store(&settings)
		return settings, nil
	})

	if settings, ok := result.(cachedOpenAIOAuthImageBridgeTransportSettings); ok {
		return settings
	}
	return cachedOpenAIOAuthImageBridgeTransportSettings{}
}

func (s *OpenAIGatewayService) getOpenAIAccountScheduler(ctx context.Context) OpenAIAccountScheduler {
	if s == nil {
		return nil
	}
	if !s.isOpenAIAdvancedSchedulerEnabled(ctx) {
		return nil
	}
	s.openaiSchedulerOnce.Do(func() {
		if s.openaiAccountStats == nil {
			s.openaiAccountStats = newOpenAIAccountRuntimeStats()
		}
		if s.openaiScheduler == nil {
			s.openaiScheduler = newDefaultOpenAIAccountScheduler(s, s.openaiAccountStats)
		}
	})
	return s.openaiScheduler
}

func resetOpenAIAdvancedSchedulerSettingCacheForTest() {
	openAIAdvancedSchedulerSettingCache = atomic.Value{}
	openAIAdvancedSchedulerSettingSF = singleflight.Group{}
}

func resetOpenAIStickyReservePercentSettingCacheForTest() {
	openAIStickyReservePercentSettingCache = atomic.Value{}
	openAIStickyReservePercentSettingSF = singleflight.Group{}
}

func resetOpenAIStickyWaitTimeoutSettingCacheForTest() {
	openAIStickyWaitTimeoutSettingCache = atomic.Value{}
	openAIStickyWaitTimeoutSettingSF = singleflight.Group{}
}

func (s *OpenAIGatewayService) SelectAccountWithScheduler(
	ctx context.Context,
	groupID *int64,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requireCompact bool,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	return s.selectAccountWithScheduler(ctx, groupID, 0, previousResponseID, sessionHash, requestedModel, excludedIDs, requiredTransport, OpenAIEndpointCapabilityChatCompletions, "", "", false, false, requireCompact)
}

func (s *OpenAIGatewayService) SelectAccountWithSchedulerForCapability(
	ctx context.Context,
	groupID *int64,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requiredCapability OpenAIEndpointCapability,
	requireCompact bool,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	return s.selectAccountWithScheduler(ctx, groupID, 0, previousResponseID, sessionHash, requestedModel, excludedIDs, requiredTransport, requiredCapability, "", "", false, false, requireCompact)
}

func (s *OpenAIGatewayService) SelectAccountWithSchedulerForResponses(
	ctx context.Context,
	groupID *int64,
	apiKeyID int64,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requireImageEnabled bool,
	requireCompact bool,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	requiredImageRoute := ""
	if requireImageEnabled {
		requiredImageRoute = GroupImageGenerationRouteCodex
		if groupID != nil && *groupID > 0 {
			if group := s.loadGroupForImageRoute(ctx, *groupID); group != nil {
				requiredImageRoute = group.EffectiveImageGenerationRoute()
			}
		}
	}
	return s.selectAccountWithScheduler(ctx, groupID, apiKeyID, previousResponseID, sessionHash, requestedModel, excludedIDs, requiredTransport, OpenAIEndpointCapabilityResponsesIngress, "", requiredImageRoute, requireImageEnabled, false, requireCompact)
}

func (s *OpenAIGatewayService) SelectAccountWithSchedulerForImages(
	ctx context.Context,
	groupID *int64,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredCapability OpenAIImagesCapability,
	requiredRoute string,
	requireOAuthAccount bool,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	if strings.TrimSpace(requiredRoute) == "" && groupID != nil && *groupID > 0 {
		if group := s.loadGroupForImageRoute(ctx, *groupID); group != nil {
			requiredRoute = group.EffectiveImageGenerationRoute()
		}
	}
	if strings.TrimSpace(requiredRoute) != "" {
		requiredRoute = openAIImageRouteForAccountScheduling(requiredRoute)
	}
	selection, decision, err := s.selectAccountWithScheduler(ctx, groupID, 0, "", sessionHash, requestedModel, excludedIDs, OpenAIUpstreamTransportHTTPSSE, "", requiredCapability, requiredRoute, true, requireOAuthAccount, false)
	if err == nil && selection != nil && selection.Account != nil {
		if !selection.Account.SupportsOpenAIImageRoute(requiredRoute) {
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			nextExcluded := cloneExcludedAccountIDs(excludedIDs)
			if nextExcluded == nil {
				nextExcluded = make(map[int64]struct{})
			}
			nextExcluded[selection.Account.ID] = struct{}{}
			return s.SelectAccountWithSchedulerForImages(ctx, groupID, sessionHash, requestedModel, nextExcluded, requiredCapability, requiredRoute, requireOAuthAccount)
		}
		return s.normalizeOpenAIImageSelectionConcurrency(ctx, groupID, selection, requiredCapability, requiredRoute), decision, nil
	}
	// 如果要求 native 能力（如指定了模型）但没有可用的 APIKey 账号，回退到 basic（OAuth 账号）
	if requiredCapability == OpenAIImagesCapabilityNative {
		selection, decision, err = s.selectAccountWithScheduler(ctx, groupID, 0, "", sessionHash, requestedModel, excludedIDs, OpenAIUpstreamTransportHTTPSSE, "", OpenAIImagesCapabilityBasic, requiredRoute, true, requireOAuthAccount, false)
		if err == nil && selection != nil && selection.Account != nil && !selection.Account.SupportsOpenAIImageRoute(requiredRoute) {
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			nextExcluded := cloneExcludedAccountIDs(excludedIDs)
			if nextExcluded == nil {
				nextExcluded = make(map[int64]struct{})
			}
			nextExcluded[selection.Account.ID] = struct{}{}
			return s.SelectAccountWithSchedulerForImages(ctx, groupID, sessionHash, requestedModel, nextExcluded, OpenAIImagesCapabilityBasic, requiredRoute, requireOAuthAccount)
		}
		return s.normalizeOpenAIImageSelectionConcurrency(ctx, groupID, selection, OpenAIImagesCapabilityBasic, requiredRoute), decision, err
	}
	return selection, decision, err
}

func (s *OpenAIGatewayService) normalizeOpenAIImageSelectionConcurrency(ctx context.Context, groupID *int64, selection *AccountSelectionResult, requiredCapability OpenAIImagesCapability, requiredRoute string) *AccountSelectionResult {
	if selection == nil || selection.Account == nil || requiredCapability == "" {
		return selection
	}
	maxConcurrency := concurrencyForOpenAIAccountSelection(selection.Account, requiredRoute)
	if selection.WaitPlan != nil {
		selection.WaitPlan.MaxConcurrency = maxConcurrency
	}
	if !selection.Acquired || maxConcurrency <= 1 || s == nil || s.concurrencyService == nil {
		return selection
	}
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
	result, err := s.tryAcquireAccountSlot(ctx, selection.Account.ID, groupID, maxConcurrency)
	if err == nil && result != nil && result.Acquired {
		selection.ReleaseFunc = result.ReleaseFunc
		selection.Acquired = true
		return selection
	}
	selection.Acquired = false
	selection.ReleaseFunc = nil
	selection.WaitPlan = &AccountWaitPlan{AccountID: selection.Account.ID, MaxConcurrency: maxConcurrency, Timeout: s.schedulingConfig().FallbackWaitTimeout, MaxWaiting: s.schedulingConfig().FallbackMaxWaiting}
	return selection
}

func (s *OpenAIGatewayService) selectAccountWithScheduler(
	ctx context.Context,
	groupID *int64,
	apiKeyID int64,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requiredCapability OpenAIEndpointCapability,
	requiredImageCapability OpenAIImagesCapability,
	requiredImageRoute string,
	requireImageEnabled bool,
	requireOAuthAccount bool,
	requireCompact bool,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	ctx = s.withOpenAIQuotaAutoPauseContext(ctx)
	decision := OpenAIAccountScheduleDecision{}
	if strings.TrimSpace(requiredImageRoute) != "" {
		requiredImageRoute = openAIImageRouteForAccountScheduling(requiredImageRoute)
	}
	if requiredImageCapability != "" && requiredImageRoute == "" {
		requiredImageRoute = GroupImageGenerationRouteCodex
	}
	req := OpenAIAccountScheduleRequest{
		GroupID:                 groupID,
		APIKeyID:                apiKeyID,
		SessionHash:             sessionHash,
		PreviousResponseID:      previousResponseID,
		RequestedModel:          requestedModel,
		RequiredTransport:       requiredTransport,
		RequiredCapability:      requiredCapability,
		RequiredImageCapability: requiredImageCapability,
		RequiredImageRoute:      requiredImageRoute,
		RequireImageEnabled:     requireImageEnabled,
		RequireOAuthAccount:     requireOAuthAccount,
		RequireCompact:          requireCompact,
		ExcludedIDs:             excludedIDs,
	}
	scheduler := s.getOpenAIAccountScheduler(ctx)
	if scheduler == nil {
		selection, previousHit, err := s.selectByPreviousResponseForScheduleRequest(ctx, req)
		if err != nil {
			return nil, decision, err
		}
		if previousHit {
			decision.Layer = openAIAccountScheduleLayerPreviousResponse
			decision.StickyPreviousHit = true
			decision.SelectedAccountID = selection.Account.ID
			decision.SelectedAccountType = selection.Account.Type
			return selection, decision, nil
		}

		decision.Layer = openAIAccountScheduleLayerLoadBalance
		useImageRouteFallback := (requiredTransport == OpenAIUpstreamTransportAny || requiredTransport == OpenAIUpstreamTransportHTTPSSE) &&
			(requiredImageCapability != "" || strings.TrimSpace(requiredImageRoute) != "" || requireImageEnabled)
		if useImageRouteFallback {
			effectiveExcludedIDs := cloneExcludedAccountIDs(excludedIDs)
			for {
				selection, err := s.selectAccountWithLoadAwarenessForImageRoute(ctx, groupID, sessionHash, requestedModel, effectiveExcludedIDs, requireCompact, requiredImageRoute, requireOAuthAccount)
				if err != nil {
					return nil, decision, err
				}
				if selection == nil || selection.Account == nil {
					return selection, decision, nil
				}
				if requireImageEnabled && !selection.Account.OpenAIImageGenerationAllowed() {
					if selection.ReleaseFunc != nil {
						selection.ReleaseFunc()
					}
				} else if accountSupportsOpenAICapabilities(selection.Account, requiredCapability, requiredImageCapability) {
					return selection, decision, nil
				} else if selection.ReleaseFunc != nil {
					selection.ReleaseFunc()
				}
				if effectiveExcludedIDs == nil {
					effectiveExcludedIDs = make(map[int64]struct{})
				}
				if _, exists := effectiveExcludedIDs[selection.Account.ID]; exists {
					return nil, decision, ErrNoAvailableAccounts
				}
				effectiveExcludedIDs[selection.Account.ID] = struct{}{}
			}
		}

		effectiveExcludedIDs := cloneExcludedAccountIDs(excludedIDs)
		for {
			selection, err := s.selectAccountWithLoadAwareness(ctx, groupID, sessionHash, requestedModel, effectiveExcludedIDs, requireCompact, "", false)
			if err != nil {
				return nil, decision, err
			}
			if selection == nil || selection.Account == nil {
				return selection, decision, nil
			}
			if (!requireImageEnabled || selection.Account.OpenAIImageGenerationAllowed()) &&
				s.isOpenAIAccountTransportCompatible(selection.Account, requiredTransport) &&
				accountSupportsOpenAICapabilities(selection.Account, requiredCapability, requiredImageCapability) {
				return selection, decision, nil
			}
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			if effectiveExcludedIDs == nil {
				effectiveExcludedIDs = make(map[int64]struct{})
			}
			if _, exists := effectiveExcludedIDs[selection.Account.ID]; exists {
				return nil, decision, ErrNoAvailableAccounts
			}
			effectiveExcludedIDs[selection.Account.ID] = struct{}{}
		}
	}

	if s.checkChannelPricingRestriction(ctx, groupID, requestedModel) {
		slog.Warn("channel pricing restriction blocked request",
			"group_id", derefGroupID(groupID),
			"model", requestedModel)
		return nil, decision, fmt.Errorf("%w supporting model: %s (channel pricing restriction)", ErrNoAvailableAccounts, requestedModel)
	}

	var stickyAccountID int64
	if sessionHash != "" && s.cache != nil {
		if accountID, err := s.getStickySessionAccountID(ctx, groupID, sessionHash); err == nil && accountID > 0 {
			stickyAccountID = accountID
		}
	}

	return scheduler.Select(ctx, OpenAIAccountScheduleRequest{
		GroupID:                 req.GroupID,
		APIKeyID:                req.APIKeyID,
		SessionHash:             req.SessionHash,
		StickyAccountID:         stickyAccountID,
		PreviousResponseID:      previousResponseID,
		RequestedModel:          requestedModel,
		RequiredTransport:       requiredTransport,
		RequiredCapability:      requiredCapability,
		RequiredImageCapability: requiredImageCapability,
		RequiredImageRoute:      requiredImageRoute,
		RequireImageEnabled:     requireImageEnabled,
		RequireOAuthAccount:     requireOAuthAccount,
		RequireCompact:          requireCompact,
		ExcludedIDs:             excludedIDs,
	})
}

func (s *OpenAIGatewayService) selectByPreviousResponseForScheduleRequest(
	ctx context.Context,
	req OpenAIAccountScheduleRequest,
) (*AccountSelectionResult, bool, error) {
	previousResponseID := strings.TrimSpace(req.PreviousResponseID)
	if previousResponseID == "" {
		return nil, false, nil
	}

	selection, err := s.SelectAccountByPreviousResponseID(
		ctx,
		req.GroupID,
		req.APIKeyID,
		previousResponseID,
		req.RequestedModel,
		req.ExcludedIDs,
		req.RequireCompact,
	)
	if err != nil || selection == nil || selection.Account == nil {
		return selection, false, err
	}

	compat := &defaultOpenAIAccountScheduler{service: s}
	if !compat.isAccountTransportCompatible(selection.Account, req.RequiredTransport) ||
		!compat.isAccountRequestCompatible(ctx, selection.Account, req) {
		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
		return nil, false, nil
	}

	if req.SessionHash != "" {
		_ = s.BindStickySession(ctx, req.GroupID, req.SessionHash, selection.Account.ID)
	}
	return selection, true, nil
}

func (s *OpenAIGatewayService) loadGroupForImageRoute(ctx context.Context, groupID int64) *Group {
	if s == nil || groupID <= 0 {
		return nil
	}
	if group, ok := ctx.Value(ctxkey.Group).(*Group); ok && IsGroupContextValid(group) && group.ID == groupID {
		return group
	}
	if s.schedulerSnapshot != nil {
		if group, err := s.schedulerSnapshot.GetGroupByID(ctx, groupID); err == nil && group != nil {
			return group
		}
	}
	return nil
}

func cloneExcludedAccountIDs(excludedIDs map[int64]struct{}) map[int64]struct{} {
	if len(excludedIDs) == 0 {
		return nil
	}
	cloned := make(map[int64]struct{}, len(excludedIDs))
	for id := range excludedIDs {
		cloned[id] = struct{}{}
	}
	return cloned
}

func (s *OpenAIGatewayService) isOpenAIAccountTransportCompatible(account *Account, requiredTransport OpenAIUpstreamTransport) bool {
	if requiredTransport == OpenAIUpstreamTransportAny || requiredTransport == OpenAIUpstreamTransportHTTPSSE {
		return true
	}
	if s == nil || account == nil {
		return false
	}
	return s.getOpenAIWSProtocolResolver().Resolve(account).Transport == requiredTransport
}

func (s *OpenAIGatewayService) ReportOpenAIAccountScheduleResult(accountID int64, success bool, firstTokenMs *int) {
	scheduler := s.getOpenAIAccountScheduler(context.Background())
	if scheduler == nil {
		return
	}
	scheduler.ReportResult(accountID, success, firstTokenMs)
}

func (s *OpenAIGatewayService) RefreshOpenAIResponsesTurnAccount(ctx context.Context, account *Account, requestedModel string) *Account {
	if account == nil {
		return nil
	}

	refreshed := account
	if s != nil && s.accountRepo != nil {
		latest, err := s.accountRepo.GetByID(ctx, account.ID)
		if err != nil || latest == nil {
			return nil
		}
		refreshed = latest
	}

	if s == nil {
		return refreshed
	}

	rechecked := s.recheckSelectedOpenAIAccountFromDB(ctx, refreshed, requestedModel, false, "")
	if rechecked != nil {
		return rechecked
	}
	return nil
}

func (s *OpenAIGatewayService) RecordOpenAIAccountRecoveryReason(accountID int64, reason string) {
	scheduler := s.getOpenAIAccountScheduler(context.Background())
	if scheduler == nil {
		return
	}
	scheduler.RecordRecoveryReason(accountID, reason)
}

func (s *OpenAIGatewayService) RecordOpenAIAccountSwitch() {
	scheduler := s.getOpenAIAccountScheduler(context.Background())
	if scheduler == nil {
		return
	}
	scheduler.ReportSwitch()
}

func (s *OpenAIGatewayService) SnapshotOpenAIAccountSchedulerMetrics() OpenAIAccountSchedulerMetricsSnapshot {
	scheduler := s.getOpenAIAccountScheduler(context.Background())
	if scheduler == nil {
		return OpenAIAccountSchedulerMetricsSnapshot{}
	}
	return scheduler.SnapshotMetrics()
}

func (s *OpenAIGatewayService) openAIWSSessionStickyTTL() time.Duration {
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds > 0 {
		return time.Duration(s.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds) * time.Second
	}
	return openaiStickySessionTTL
}

func (s *OpenAIGatewayService) openAIWSLBTopK() int {
	if s != nil && s.cfg != nil && s.cfg.Gateway.OpenAIWS.LBTopK > 0 {
		return s.cfg.Gateway.OpenAIWS.LBTopK
	}
	return 7
}

func (s *OpenAIGatewayService) openAIStickyEscapeConfig() openAIStickyEscapeConfig {
	if s != nil && s.cfg != nil {
		cfg := s.cfg.Gateway.OpenAIScheduler
		enabled := cfg.StickyEscapeEnabled
		if !enabled && cfg.StickyEscapeTTFTMs == 0 && cfg.StickyEscapeErrorRate == 0 {
			enabled = true
		}
		ttftMs := float64(cfg.StickyEscapeTTFTMs)
		if ttftMs <= 0 {
			ttftMs = 15000
		}
		errorRate := cfg.StickyEscapeErrorRate
		if errorRate < 0 || errorRate > 1 {
			errorRate = 0.5
		}
		if errorRate == 0 && cfg.StickyEscapeTTFTMs == 0 && cfg.StickyEscapeErrorRate == 0 {
			errorRate = 0.5
		}
		return openAIStickyEscapeConfig{
			enabled:   enabled,
			ttftMs:    ttftMs,
			errorRate: errorRate,
		}
	}
	return openAIStickyEscapeConfig{
		enabled:   true,
		ttftMs:    15000,
		errorRate: 0.5,
	}
}

func (s *OpenAIGatewayService) openAIWSSchedulerWeights() GatewayOpenAIWSSchedulerScoreWeightsView {
	if s != nil && s.cfg != nil {
		return GatewayOpenAIWSSchedulerScoreWeightsView{
			Priority:  s.cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Priority,
			Load:      s.cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load,
			Queue:     s.cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue,
			ErrorRate: s.cfg.Gateway.OpenAIWS.SchedulerScoreWeights.ErrorRate,
			TTFT:      s.cfg.Gateway.OpenAIWS.SchedulerScoreWeights.TTFT,
		}
	}
	return GatewayOpenAIWSSchedulerScoreWeightsView{
		Priority:  1.0,
		Load:      1.0,
		Queue:     0.7,
		ErrorRate: 0.8,
		TTFT:      0.5,
	}
}

type GatewayOpenAIWSSchedulerScoreWeightsView struct {
	Priority  float64
	Load      float64
	Queue     float64
	ErrorRate float64
	TTFT      float64
}

func clamp01(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}

func calcLoadSkewByMoments(sum float64, sumSquares float64, count int) float64 {
	if count <= 1 {
		return 0
	}
	mean := sum / float64(count)
	variance := sumSquares/float64(count) - mean*mean
	if variance < 0 {
		variance = 0
	}
	return math.Sqrt(variance)
}
