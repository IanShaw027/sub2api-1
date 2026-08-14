//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAccountIsSlotCandidate_UsesBurstNotLoadRate(t *testing.T) {
	acc := &Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 12}
	require.Equal(t, 14, acc.BurstConcurrency())

	require.True(t, accountIsSlotCandidate(acc, &AccountLoadInfo{
		AccountID: 1, CurrentConcurrency: 13, LoadRate: 108,
	}, 0), "in_flight < Burst must stay a candidate even when LoadRate > 100")

	require.False(t, accountIsSlotCandidate(acc, &AccountLoadInfo{
		AccountID: 1, CurrentConcurrency: 14, LoadRate: 116,
	}, 0), "in_flight >= Burst is not a switch target")
}

func TestAccountIsSlotCandidate_StickyFullStaysCandidate(t *testing.T) {
	acc := &Account{ID: 9, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 12}
	require.True(t, accountIsSlotCandidate(acc, &AccountLoadInfo{
		AccountID: 9, CurrentConcurrency: 12, LoadRate: 100,
	}, 9), "sticky-bound account must stay a candidate at LoadRate=100")
	require.True(t, accountIsSlotCandidate(acc, &AccountLoadInfo{
		AccountID: 9, CurrentConcurrency: 14, LoadRate: 116,
	}, 9), "sticky-bound account must stay a candidate even at Burst")
}

func TestPreferInstantFanout_NewSessionUsesFreeAccount(t *testing.T) {
	full := accountWithLoad{
		account:  &Account{ID: 1, Concurrency: 12},
		loadInfo: &AccountLoadInfo{AccountID: 1, CurrentConcurrency: 12, LoadRate: 100},
	}
	free := accountWithLoad{
		account:  &Account{ID: 2, Concurrency: 12},
		loadInfo: &AccountLoadInfo{AccountID: 2, CurrentConcurrency: 3, LoadRate: 25},
	}
	got := preferInstantFanoutAccounts([]accountWithLoad{full, free})
	require.Len(t, got, 1)
	require.Equal(t, int64(2), got[0].account.ID)
}

func TestPreferInstantFanout_AllAtNKeepsLadderCandidates(t *testing.T) {
	a := accountWithLoad{
		account:  &Account{ID: 1, Concurrency: 12},
		loadInfo: &AccountLoadInfo{AccountID: 1, CurrentConcurrency: 12, LoadRate: 100},
	}
	b := accountWithLoad{
		account:  &Account{ID: 2, Concurrency: 12},
		loadInfo: &AccountLoadInfo{AccountID: 2, CurrentConcurrency: 12, LoadRate: 100},
	}
	got := preferInstantFanoutAccounts([]accountWithLoad{a, b})
	require.Len(t, got, 2, "when every account is at N, keep them so the 30s ladder can run")
}

func TestSlotLadderMaxWaiting_AtLeastOverflowPlusTwo(t *testing.T) {
	acc := &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, Concurrency: 12}
	require.Equal(t, 2, acc.OverflowConcurrency())
	require.Equal(t, 4, slotLadderMaxWaiting(1, acc))
	require.Equal(t, 4, slotLadderMaxWaiting(3, acc))
	require.Equal(t, 10, slotLadderMaxWaiting(10, acc))
}

func TestSelectAccountWithLoadAwareness_StickyLoadRate100StillSelectable(t *testing.T) {
	ctx := context.Background()
	repo := &mockAccountRepoForPlatform{
		accounts: []Account{
			{
				ID:          1,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeOAuth,
				Priority:    1,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 12,
			},
		},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}

	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	concurrencyCache := &mockConcurrencyCache{
		acquireResults: map[int64]bool{1: false},
		waitCounts:     map[int64]int{1: 0},
		loadMap: map[int64]*AccountLoadInfo{
			1: {AccountID: 1, CurrentConcurrency: 12, LoadRate: 100},
		},
	}

	svc := &GatewayService{
		accountRepo:        repo,
		cache:              &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"sticky": 1}},
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "sticky", "claude-3-5-sonnet-20241022", nil, "", int64(0))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Account)
	require.Equal(t, int64(1), result.Account.ID, "sticky account at LoadRate=100 must remain selectable")
	require.False(t, result.Acquired)
	require.NotNil(t, result.WaitPlan, "full sticky account must enter the 30s ladder")
	require.Equal(t, 12, result.WaitPlan.MaxConcurrency)
	require.GreaterOrEqual(t, result.WaitPlan.MaxWaiting, 4)
}

func TestSelectAccountWithLoadAwareness_NewSessionFreeAccountDoesNotWait(t *testing.T) {
	ctx := context.Background()
	repo := &mockAccountRepoForPlatform{
		accounts: []Account{
			{
				ID:          1,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeOAuth,
				Priority:    1,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 12,
			},
			{
				ID:          2,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeOAuth,
				Priority:    1,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 12,
			},
		},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}

	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	concurrencyCache := &mockConcurrencyCache{
		acquireResults: map[int64]bool{1: false, 2: true},
		loadMap: map[int64]*AccountLoadInfo{
			1: {AccountID: 1, CurrentConcurrency: 12, LoadRate: 100},
			2: {AccountID: 2, CurrentConcurrency: 3, LoadRate: 25},
		},
	}

	svc := &GatewayService{
		accountRepo:        repo,
		cache:              &mockGatewayCacheForPlatform{},
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "", "claude-3-5-sonnet-20241022", nil, "", int64(0))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Acquired, "new session must fan out to a free account immediately")
	require.Nil(t, result.WaitPlan)
	require.Equal(t, int64(2), result.Account.ID)
}

func TestBindGatewayStickySessionDuringSelection_PreserveDoesNotOverwrite(t *testing.T) {
	cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"sess": 11}}
	svc := &GatewayService{cache: cache}
	ctx := WithPreserveStickyBinding(context.Background())

	require.NoError(t, svc.bindGatewayStickySessionDuringSelection(ctx, nil, "sess", 22))
	require.Equal(t, int64(11), cache.sessionBindings["sess"], "PreserveStickyBinding must keep the original account")

	require.NoError(t, svc.bindGatewayStickySessionDuringSelection(context.Background(), nil, "sess", 22))
	require.Equal(t, int64(22), cache.sessionBindings["sess"], "without preserve, selection may rebind")
}

func TestBindOpenAIStickySessionDuringSelection_PreserveDoesNotOverwrite(t *testing.T) {
	const sessionHash = "openai-sess"
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:" + sessionHash: 11}}
	svc := &OpenAIGatewayService{cache: cache}
	ctx := WithPreserveStickyBinding(context.Background())
	groupID := int64(1)

	require.NoError(t, svc.bindOpenAIStickySessionDuringSelection(ctx, &groupID, sessionHash, 22))
	require.Equal(t, int64(11), cache.sessionBindings["openai:"+sessionHash])

	require.NoError(t, svc.bindOpenAIStickySessionDuringSelection(context.Background(), &groupID, sessionHash, 22))
	require.Equal(t, int64(22), cache.sessionBindings["openai:"+sessionHash])
}

func TestSelectAccountWithLoadAwareness_PostSwitchImmediateNOnly(t *testing.T) {
	ctx := WithPreserveStickyBinding(context.Background())
	repo := &mockAccountRepoForPlatform{
		accounts: []Account{
			{
				ID:          2,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeOAuth,
				Priority:    1,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 12,
			},
		},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}

	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	concurrencyCache := &mockConcurrencyCache{
		acquireResults: map[int64]bool{2: false},
		loadMap: map[int64]*AccountLoadInfo{
			2: {AccountID: 2, CurrentConcurrency: 5, LoadRate: 41},
		},
	}

	gwCache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"sticky": 1}}
	svc := &GatewayService{
		accountRepo:        repo,
		cache:              gwCache,
		cfg:                cfg,
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "sticky", "claude-3-5-sonnet-20241022", map[int64]struct{}{1: {}}, "", int64(0))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(2), result.Account.ID)
	require.False(t, result.Acquired)
	require.Nil(t, result.WaitPlan, "post-switch must not start a 30s wait on the failover account")
	require.Equal(t, int64(1), gwCache.sessionBindings["sticky"], "original sticky binding must be unchanged")
}

func TestBindStickySessionAfterProfitAdmission_PreserveDoesNotOverwrite(t *testing.T) {
	cache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"sess": 11}}
	svc := &GatewayService{cache: cache}
	ctx := WithPreserveStickyBinding(context.Background())

	require.NoError(t, svc.BindStickySessionAfterProfitAdmission(ctx, nil, "sess", 22))
	require.Equal(t, int64(11), cache.sessionBindings["sess"], "admission-bind must keep the original account when Preserve is set")

	require.NoError(t, svc.BindStickySessionAfterProfitAdmission(context.Background(), nil, "sess", 22))
	require.Equal(t, int64(22), cache.sessionBindings["sess"], "without preserve, admission-bind may write the admitted account")
}

func TestOpenAIBindStickySessionAfterProfitAdmission_PreserveDoesNotOverwrite(t *testing.T) {
	const sessionHash = "openai-sess"
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:" + sessionHash: 11}}
	svc := &OpenAIGatewayService{cache: cache}
	ctx := WithPreserveStickyBinding(context.Background())
	groupID := int64(1)

	require.NoError(t, svc.BindStickySessionAfterProfitAdmission(ctx, &groupID, sessionHash, 22))
	require.Equal(t, int64(11), cache.sessionBindings["openai:"+sessionHash])

	require.NoError(t, svc.BindStickySessionAfterProfitAdmission(context.Background(), &groupID, sessionHash, 22))
	require.Equal(t, int64(22), cache.sessionBindings["openai:"+sessionHash])
}

func TestSelectAccountForModelWithExclusions_PreserveDoesNotOverwrite(t *testing.T) {
	sessionHash := "excluded-preserve"
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 11, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1},
			{ID: 22, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 2},
		},
	}
	cache := &stubGatewayCache{sessionBindings: map[string]int64{"openai:" + sessionHash: 11}}
	svc := &OpenAIGatewayService{accountRepo: repo, cache: cache}
	ctx := WithPreserveStickyBinding(context.Background())

	acc, err := svc.SelectAccountForModelWithExclusions(ctx, nil, sessionHash, "gpt-4", map[int64]struct{}{11: {}})
	require.NoError(t, err)
	require.NotNil(t, acc)
	require.Equal(t, int64(22), acc.ID)
	require.Equal(t, int64(11), cache.sessionBindings["openai:"+sessionHash], "Preserve must block setStickySessionAccountID on the failover account")
}

func TestOpenAISelectAccountWithLoadAwareness_PreserveDoesNotSetSticky(t *testing.T) {
	sessionHash := "load-preserve"
	groupID := int64(1)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 11, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1},
			{ID: 22, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1},
		},
	}
	cache := &stubGatewayCache{sessionBindings: map[string]int64{"openai:" + sessionHash: 11}}
	concurrencyCache := stubConcurrencyCache{
		acquireResults: map[int64]bool{11: false, 22: true},
		loadMap: map[int64]*AccountLoadInfo{
			11: {AccountID: 11, CurrentConcurrency: 1, LoadRate: 100},
			22: {AccountID: 22, CurrentConcurrency: 0, LoadRate: 0},
		},
	}
	svc := &OpenAIGatewayService{
		accountRepo:        repo,
		cache:              cache,
		concurrencyService: NewConcurrencyService(concurrencyCache),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(WithPreserveStickyBinding(context.Background()), &groupID, sessionHash, "gpt-4", map[int64]struct{}{11: {}})
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.Equal(t, int64(22), selection.Account.ID)
	require.Equal(t, int64(11), cache.sessionBindings["openai:"+sessionHash], "load-awareness setStickySessionAccountID must honor Preserve")
}

func TestSelectAccountWithLoadAwareness_QueueFullEntersLadder(t *testing.T) {
	ctx := context.Background()
	repo := &mockAccountRepoForPlatform{
		accounts: []Account{
			{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Priority: 1, Status: StatusActive, Schedulable: true, Concurrency: 12},
			{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Priority: 2, Status: StatusActive, Schedulable: true, Concurrency: 12},
		},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}
	cfg := testConfig()
	cfg.Gateway.Scheduling.LoadBatchEnabled = true
	cfg.Gateway.Scheduling.StickySessionMaxWaiting = 3
	gwCache := &mockGatewayCacheForPlatform{sessionBindings: map[string]int64{"sticky": 1}}
	svc := &GatewayService{
		accountRepo: repo,
		cache:       gwCache,
		cfg:         cfg,
		concurrencyService: NewConcurrencyService(&mockConcurrencyCache{
			acquireResults: map[int64]bool{1: false, 2: true},
			waitCounts:     map[int64]int{1: 999},
			loadMap: map[int64]*AccountLoadInfo{
				1: {AccountID: 1, CurrentConcurrency: 12, LoadRate: 100},
				2: {AccountID: 2, CurrentConcurrency: 0, LoadRate: 0},
			},
		}),
	}

	result, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "sticky", "claude-3-5-sonnet-20241022", nil, "", int64(0))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(1), result.Account.ID, "queue-full must still return the sticky account so the ladder can switch")
	require.False(t, result.Acquired)
	require.NotNil(t, result.WaitPlan)
	require.Equal(t, int64(1), gwCache.sessionBindings["sticky"], "queue-full must not bind a different account")
}

func TestOpenAISelectAccountWithLoadAwareness_QueueFullEntersLadder(t *testing.T) {
	sessionHash := "sticky-queue-full"
	groupID := int64(1)
	repo := stubOpenAIAccountRepo{
		accounts: []Account{
			{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 12, Priority: 1, GroupIDs: []int64{groupID}},
			{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 12, Priority: 1, GroupIDs: []int64{groupID}},
		},
	}
	cache := &stubGatewayCache{sessionBindings: map[string]int64{"openai:" + sessionHash: 1}}
	svc := &OpenAIGatewayService{
		accountRepo: repo,
		cache:       cache,
		concurrencyService: NewConcurrencyService(stubConcurrencyCache{
			acquireResults: map[int64]bool{1: false, 2: true},
			waitCounts:     map[int64]int{1: 999},
			loadMap: map[int64]*AccountLoadInfo{
				1: {AccountID: 1, CurrentConcurrency: 12, LoadRate: 100},
				2: {AccountID: 2, CurrentConcurrency: 0, LoadRate: 0},
			},
		}),
	}

	selection, err := svc.SelectAccountWithLoadAwareness(context.Background(), &groupID, sessionHash, "gpt-4", nil)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(1), selection.Account.ID, "queue-full must enter the ladder on the sticky account")
	require.False(t, selection.Acquired)
	require.NotNil(t, selection.WaitPlan)
	require.Equal(t, int64(1), cache.sessionBindings["openai:"+sessionHash])
}

func TestSelectAccountByPreviousResponseID_WaitPlanUsesSlotLadderMaxWaiting(t *testing.T) {
	ctx := context.Background()
	groupID := int64(9)
	account := Account{
		ID:          1001,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 12,
		Extra: map[string]any{
			"openai_apikey_responses_websockets_v2_enabled": true,
		},
	}
	cfg := testConfig()
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StickySessionTTLSeconds = 1800
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600
	cfg.Gateway.Scheduling.StickySessionMaxWaiting = 3
	cfg.Gateway.Scheduling.StickySessionWaitTimeout = 45 * time.Second

	svc := &OpenAIGatewayService{
		accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:       &schedulerTestGatewayCache{},
		cfg:         cfg,
		concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{
			acquireResults: map[int64]bool{1001: false},
		}),
	}
	require.NoError(t, svc.getOpenAIWSStateStore().BindResponseAccount(ctx, groupID, "resp_ladder_001", account.ID, time.Hour))

	selection, err := svc.SelectAccountByPreviousResponseID(ctx, &groupID, "resp_ladder_001", "gpt-5.1", nil, false)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.NotNil(t, selection.Account)
	require.False(t, selection.Acquired)
	require.NotNil(t, selection.WaitPlan)
	require.GreaterOrEqual(t, selection.WaitPlan.MaxWaiting, 4, "N=12 must floor MaxWaiting at overflow+2")
}
