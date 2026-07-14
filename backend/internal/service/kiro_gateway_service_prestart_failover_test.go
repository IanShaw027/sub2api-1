package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type kiroPreStartTempUnschedCall struct {
	accountID int64
	until     time.Time
	reason    string
}

type kiroPreStartAccountRepoStub struct {
	kiroDefaultAccountRepoStub
	mu       sync.Mutex
	calls    []kiroPreStartTempUnschedCall
	accounts []Account
}

func (r *kiroPreStartAccountRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, kiroPreStartTempUnschedCall{
		accountID: id,
		until:     until,
		reason:    reason,
	})
	return nil
}

func (r *kiroPreStartAccountRepoStub) ListByPlatform(context.Context, string) ([]Account, error) {
	return append([]Account(nil), r.accounts...), nil
}

type kiroFirstEventCounterStub struct {
	mu     sync.Mutex
	counts map[int64]int
}

func newKiroFirstEventCounterStub() *kiroFirstEventCounterStub {
	return &kiroFirstEventCounterStub{counts: make(map[int64]int)}
}

func (c *kiroFirstEventCounterStub) IncrementTempUnschedCount(context.Context, int64, string, int) (int64, error) {
	return 0, nil
}

func (c *kiroFirstEventCounterStub) IncrementTempUnschedThreshold(_ context.Context, accountID int64, _ string, _ int, thresholdCount int) (int64, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts[accountID]++
	count := c.counts[accountID]
	if count >= thresholdCount {
		c.counts[accountID] = 0
		return int64(count), true, nil
	}
	return int64(count), false, nil
}

func (c *kiroFirstEventCounterStub) ResetTempUnschedCount(_ context.Context, accountID int64, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts[accountID] = 0
	return nil
}

func (c *kiroFirstEventCounterStub) ResetTempUnschedFingerprint(ctx context.Context, accountID int64, fingerprint string) error {
	return c.ResetTempUnschedCount(ctx, accountID, fingerprint)
}

type kiroBlockingReadCloser struct {
	closed chan struct{}
	once   sync.Once
}

func newKiroBlockingReadCloser() *kiroBlockingReadCloser {
	return &kiroBlockingReadCloser{closed: make(chan struct{})}
}

func (r *kiroBlockingReadCloser) Read(p []byte) (int, error) {
	<-r.closed
	return 0, io.EOF
}

func (r *kiroBlockingReadCloser) Close() error {
	r.once.Do(func() {
		close(r.closed)
	})
	return nil
}

type kiroFailingReadCloser struct {
	err error
}

func (r *kiroFailingReadCloser) Read(p []byte) (int, error) {
	return 0, r.err
}

func (r *kiroFailingReadCloser) Close() error {
	return nil
}

func TestKiroGatewayService_ForwardStream_PreStartReadErrorReturnsFailoverAndTempUnsched(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	repo := &kiroPreStartAccountRepoStub{}
	svc := &KiroGatewayService{
		rateLimitService: &RateLimitService{
			accountRepo: repo,
		},
	}

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 77, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: &kiroFailingReadCloser{err: io.ErrUnexpectedEOF}, Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.5"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.Error(t, err)
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Body.String())
	require.Len(t, repo.calls, 1)
	v, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := v.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "failover", events[0].Kind)
	require.Equal(t, http.StatusBadGateway, events[0].UpstreamStatusCode)
	require.Contains(t, events[0].Message, "before first forwardable event")
}

func TestKiroGatewayService_ForwardStream_PreStartClientCanceledReadErrorDoesNotFailoverOrTempUnsched(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(canceledCtx)

	repo := &kiroPreStartAccountRepoStub{}
	svc := &KiroGatewayService{
		rateLimitService: &RateLimitService{
			accountRepo: repo,
		},
	}

	result, err := svc.forwardStream(
		context.Background(),
		c,
		&Account{ID: 79, Platform: PlatformKiro, Type: AccountTypeOAuth},
		&http.Response{Body: &kiroFailingReadCloser{err: context.Canceled}, Header: http.Header{}},
		&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
		&kiropkg.ConvertResult{Model: "claude-sonnet-4.5"},
		32,
		time.Now(),
		nil,
		kiropkg.FakeCacheHitState{},
		nil,
		"",
	)

	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "client disconnect must not trigger account failover")
	require.Empty(t, rec.Body.String())
	require.Empty(t, repo.calls)
	_, ok := c.Get(OpsUpstreamErrorsKey)
	require.False(t, ok)
}

func TestKiroGatewayService_HandleTransportError_ClientCanceledDoesNotRecordOpsOrFailover(t *testing.T) {
	setGinTestMode()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(canceledCtx)

	repo := &kiroPreStartAccountRepoStub{}
	svc := &KiroGatewayService{
		rateLimitService: &RateLimitService{
			accountRepo: repo,
		},
	}

	err := svc.handleKiroTransportError(
		context.Background(),
		c,
		&Account{ID: 80, Platform: PlatformKiro, Type: AccountTypeOAuth},
		"https://q.us-east-1.amazonaws.com/generateAssistantResponse",
		context.Canceled,
	)

	require.ErrorIs(t, err, context.Canceled)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "client disconnect must not trigger account failover")
	require.Empty(t, rec.Body.String())
	require.Empty(t, repo.calls)
	_, ok := c.Get(OpsUpstreamErrorsKey)
	require.False(t, ok)
}

func TestKiroGatewayService_ForwardStream_PreFirstForwardableTimeoutUsesThresholdAndProfileExclusions(t *testing.T) {
	setGinTestMode()

	prevTimeout := kiroFirstForwardableEventTimeout
	kiroFirstForwardableEventTimeout = 20 * time.Millisecond
	t.Cleanup(func() {
		kiroFirstForwardableEventTimeout = prevTimeout
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	profileARN := "arn:aws:codewhisperer:eu-central-1:123:profile/shared"
	repo := &kiroPreStartAccountRepoStub{accounts: []Account{
		{ID: 78, Platform: PlatformKiro, Credentials: map[string]any{"profile_arn": profileARN}},
		{ID: 79, Platform: PlatformKiro, Credentials: map[string]any{"profile_arn": profileARN}},
		{ID: 80, Platform: PlatformKiro, Credentials: map[string]any{"profile_arn": "arn:other"}},
	}}
	counter := newKiroFirstEventCounterStub()
	svc := &KiroGatewayService{
		rateLimitService: &RateLimitService{
			accountRepo:        repo,
			tempUnschedCounter: counter,
		},
	}
	account := &Account{ID: 78, Platform: PlatformKiro, Type: AccountTypeOAuth, Credentials: map[string]any{"profile_arn": profileARN}}

	for attempt := 1; attempt <= kiroFirstEventTimeoutThresholdCount; attempt++ {
		rec = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(rec)
		result, err := svc.forwardStream(
			context.Background(), c, account,
			&http.Response{Body: newKiroBlockingReadCloser(), Header: http.Header{}},
			&ParsedRequest{Model: "claude-sonnet-4", Stream: true},
			&kiropkg.ConvertResult{Model: "claude-sonnet-4.5"},
			32, time.Now(), nil, kiropkg.FakeCacheHitState{}, nil, "",
		)

		require.Error(t, err)
		require.Nil(t, result)
		var failoverErr *UpstreamFailoverError
		require.ErrorAs(t, err, &failoverErr)
		require.Equal(t, http.StatusGatewayTimeout, failoverErr.StatusCode)
		require.Equal(t, []int64{78, 79}, failoverErr.ExcludedAccountIDs)
		require.Len(t, repo.calls, attempt/kiroFirstEventTimeoutThresholdCount)
	}

	require.Len(t, repo.calls, 1)
	require.WithinDuration(t, time.Now().Add(kiroFirstEventTimeoutCooldown), repo.calls[0].until, 2*time.Second)
	require.Contains(t, repo.calls[0].reason, kiroFirstEventTimeoutReasonKeyword)
	v, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := v.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "failover", events[0].Kind)
	require.Equal(t, http.StatusGatewayTimeout, events[0].UpstreamStatusCode)
	require.Contains(t, events[0].Message, "did not emit a forwardable event")
}

func TestKiroGatewayService_FirstForwardableEventResetsConsecutiveTimeouts(t *testing.T) {
	repo := &kiroPreStartAccountRepoStub{}
	counter := newKiroFirstEventCounterStub()
	svc := &KiroGatewayService{rateLimitService: &RateLimitService{
		accountRepo:        repo,
		tempUnschedCounter: counter,
	}}
	account := &Account{ID: 81, Platform: PlatformKiro}

	svc.maybeMarkKiroFirstEventTimeout(context.Background(), account, "timeout 1")
	svc.maybeMarkKiroFirstEventTimeout(context.Background(), account, "timeout 2")
	svc.resetKiroFirstEventTimeoutCount(context.Background(), account)
	svc.maybeMarkKiroFirstEventTimeout(context.Background(), account, "timeout after success")

	require.Empty(t, repo.calls)
	counter.mu.Lock()
	count := counter.counts[account.ID]
	counter.mu.Unlock()
	require.Equal(t, 1, count)
}
