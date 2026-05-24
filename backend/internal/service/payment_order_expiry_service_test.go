//go:build unit

package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type paymentOrderExpiryWorkerStub struct {
	reconcileCalls int32
	expireCalls    int32
}

func (s *paymentOrderExpiryWorkerStub) ReconcilePendingWxpayOrders(context.Context) (int, error) {
	atomic.AddInt32(&s.reconcileCalls, 1)
	return 0, nil
}

func (s *paymentOrderExpiryWorkerStub) ExpireTimedOutOrders(context.Context) (int, error) {
	atomic.AddInt32(&s.expireCalls, 1)
	return 0, nil
}

func TestPaymentOrderExpiryServiceStartIsIdempotent(t *testing.T) {
	worker := &paymentOrderExpiryWorkerStub{}
	svc := NewPaymentOrderExpiryService(worker, 500*time.Millisecond)

	svc.Start()
	svc.Start()

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&worker.reconcileCalls) == 1 && atomic.LoadInt32(&worker.expireCalls) == 1
	}, time.Second, 10*time.Millisecond)

	time.Sleep(150 * time.Millisecond)
	svc.Stop()

	require.Equal(t, int32(1), atomic.LoadInt32(&worker.reconcileCalls))
	require.Equal(t, int32(1), atomic.LoadInt32(&worker.expireCalls))
}
