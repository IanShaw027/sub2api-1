package service

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	// openAIWSReconcileTimeout 限制单轮 reconcile 的总时长，避免长时间占用。
	openAIWSReconcileTimeout = 60 * time.Second
	// openAIWSReconcilePrewarmConcurrency 冷账号预热的并发拨号上限，防止拨号风暴。
	openAIWSReconcilePrewarmConcurrency = 4
)

// openAIWSReconcileMu 保证同一时刻只有一轮 reconcile 在执行。
// 运行期间的新触发会被记录为 pending，并在当前轮结束后按最新快照补跑。
var (
	openAIWSReconcileMu      sync.Mutex
	openAIWSReconcileRunning bool
	openAIWSReconcilePending bool
)

// registerOpenAIWSPoolReconcileHook 在服务初始化时注册 reconcile 回调，
// 使 SettingService 在连接池相关设置变更后无需直接引用连接池即可触发 reconcile。
func (s *OpenAIGatewayService) registerOpenAIWSPoolReconcileHook() {
	RegisterOpenAIWSPoolReconcileHook(func() {
		s.ReconcileOpenAIWSPool(context.Background())
	})
}

// ReconcileOpenAIWSPool 在连接池设置变更后执行一轮 reconcile：
// 先按新的 idle TTL / neutral target 收缩多余空闲连接，
// 再为所有可调度 OAuth OpenAI 账号补足 neutral 空闲连接到当前 target。
func (s *OpenAIGatewayService) ReconcileOpenAIWSPool(ctx context.Context) {
	if s == nil {
		return
	}

	openAIWSReconcileMu.Lock()
	if openAIWSReconcileRunning {
		openAIWSReconcilePending = true
		openAIWSReconcileMu.Unlock()
		return
	}
	openAIWSReconcileRunning = true
	openAIWSReconcilePending = false
	openAIWSReconcileMu.Unlock()

	runCtx := ctx
	for {
		s.runOpenAIWSPoolReconcileRound(runCtx)

		openAIWSReconcileMu.Lock()
		if !openAIWSReconcilePending {
			openAIWSReconcileRunning = false
			openAIWSReconcileMu.Unlock()
			return
		}
		openAIWSReconcilePending = false
		openAIWSReconcileMu.Unlock()
		runCtx = context.Background()
	}
}

func (s *OpenAIGatewayService) runOpenAIWSPoolReconcileRound(ctx context.Context) {
	if s.accountRepo == nil {
		if s.openaiWSPool != nil {
			s.openaiWSPool.ReconcileShrink()
		}
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAIWSReconcileTimeout)
	defer cancel()

	accounts, err := s.accountRepo.ListSchedulableByPlatform(runCtx, PlatformOpenAI)
	pool := s.getOpenAIWSConnPool()
	if pool == nil {
		return
	}
	if err != nil {
		pool.ReconcileShrink()
		return
	}

	targets := make(map[int64]openAIWSPoolReconcileTarget, len(accounts))
	prewarmCandidates := make([]struct {
		account *Account
		target  int
	}, 0, len(accounts))
	for i := range accounts {
		account := accounts[i]
		acc := &account
		if !acc.IsSchedulable() || !acc.IsOpenAIOAuth() {
			continue
		}
		if s.getOpenAIWSProtocolResolver().Resolve(acc).Transport != OpenAIUpstreamTransportResponsesWebsocketV2 {
			continue
		}
		target := pool.neutralPrewarmTargetForAccount(acc)
		targets[acc.ID] = openAIWSPoolReconcileTarget{
			accountConcurrency: acc.Concurrency,
		}
		if target > 0 {
			prewarmCandidates = append(prewarmCandidates, struct {
				account *Account
				target  int
			}{account: acc, target: target})
		}
	}

	// 1) 收缩：按最新账号可调度/WS 模式快照回收不再 eligible 的账号空闲连接，
	// 并按当前 target 收缩多余 neutral stock。
	pool.ReconcileTargets(targets)

	// 2) 扩张：仅当 neutral target > 0 时为可调度账号预热 neutral 连接。
	if len(prewarmCandidates) == 0 {
		return
	}

	g, gctx := errgroup.WithContext(runCtx)
	g.SetLimit(openAIWSReconcilePrewarmConcurrency)
	for _, candidate := range prewarmCandidates {
		acc := candidate.account
		target := candidate.target
		g.Go(func() error {
			s.prewarmNeutralForAccount(gctx, pool, acc, target)
			return nil
		})
	}
	_ = g.Wait()
}

// prewarmNeutralForAccount 为单个可调度 OAuth 账号合成 neutral 预热请求并补足空闲连接。
func (s *OpenAIGatewayService) prewarmNeutralForAccount(ctx context.Context, pool *openAIWSConnPool, account *Account, minIdle int) {
	if account == nil || account.ID <= 0 {
		return
	}
	wsURL, err := s.buildOpenAIResponsesWSURL(account)
	if err != nil || wsURL == "" {
		return
	}
	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil || token == "" {
		return
	}
	decision := s.getOpenAIWSProtocolResolver().Resolve(account)
	tlsFPRuntime := s.resolveOpenAITLSFingerprintRuntime(ctx, nil, account, "websocket")
	headers := s.buildOpenAIWSNeutralHeaders(account, token, decision, true)
	applyOpenAIWSFingerprintRuntimeHeaders(headers, tlsFPRuntime)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	pool.PrewarmNeutral(account.ID, openAIWSAcquireRequest{
		Account:    account,
		WSURL:      wsURL,
		Headers:    headers,
		ProxyURL:   proxyURL,
		TLSProfile: tlsFPRuntime.Profile,
		Profile:    openAIWSConnProfileNeutral,
	}, minIdle)
}
