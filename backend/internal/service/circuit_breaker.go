package service

import (
	"log"
	"sync"
	"time"
)

// CircuitBreaker tracks account failures and temporarily blocks selection when tripped.
type CircuitBreaker struct {
	mu           sync.RWMutex
	failureCount map[int64]int
	lastFailTime map[int64]time.Time
	threshold    int
	resetTimeout time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
	stopOnce     sync.Once
}

func NewCircuitBreaker() *CircuitBreaker {
	cb := &CircuitBreaker{
		failureCount: make(map[int64]int),
		lastFailTime: make(map[int64]time.Time),
		threshold:    5,
		resetTimeout: 5 * time.Minute,
		stopCh:       make(chan struct{}),
	}
	// Start background cleanup goroutine
	cb.wg.Add(1)
	go cb.cleanupLoop()
	return cb
}

// cleanupLoop periodically removes expired entries
func (cb *CircuitBreaker) cleanupLoop() {
	defer cb.wg.Done()
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cb.Cleanup()
		case <-cb.stopCh:
			return
		}
	}
}

// Cleanup removes expired entries from the maps
func (cb *CircuitBreaker) Cleanup() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	for id, lastFail := range cb.lastFailTime {
		if now.Sub(lastFail) >= cb.resetTimeout {
			delete(cb.failureCount, id)
			delete(cb.lastFailTime, id)
		}
	}
}

// Stop gracefully shuts down the cleanup goroutine
func (cb *CircuitBreaker) Stop() {
	cb.stopOnce.Do(func() {
		close(cb.stopCh)
	})
	cb.wg.Wait()
}

func (cb *CircuitBreaker) RecordFailure(accountID int64) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	if lastFail, ok := cb.lastFailTime[accountID]; ok && now.Sub(lastFail) >= cb.resetTimeout {
		delete(cb.failureCount, accountID)
		delete(cb.lastFailTime, accountID)
	}

	cb.failureCount[accountID]++
	cb.lastFailTime[accountID] = now

	if cb.failureCount[accountID] == cb.threshold {
		log.Printf("Circuit breaker opened for account %d after %d consecutive failures", accountID, cb.failureCount[accountID])
	}
}

func (cb *CircuitBreaker) IsOpen(accountID int64) bool {
	cb.mu.RLock()
	count, ok := cb.failureCount[accountID]
	if !ok {
		cb.mu.RUnlock()
		return false
	}

	lastFail, ok := cb.lastFailTime[accountID]
	if !ok {
		cb.mu.RUnlock()
		return false
	}

	if time.Since(lastFail) >= cb.resetTimeout {
		cb.mu.RUnlock()
		// 超时需要清理，升级到写锁
		cb.mu.Lock()
		// 双重检查，防止其他 goroutine 已经清理
		if lastFail2, ok2 := cb.lastFailTime[accountID]; ok2 && time.Since(lastFail2) >= cb.resetTimeout {
			delete(cb.failureCount, accountID)
			delete(cb.lastFailTime, accountID)
		}
		cb.mu.Unlock()
		return false
	}

	isOpen := count >= cb.threshold
	cb.mu.RUnlock()
	return isOpen
}

func (cb *CircuitBreaker) RecordSuccess(accountID int64) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if _, ok := cb.failureCount[accountID]; ok {
		delete(cb.failureCount, accountID)
		delete(cb.lastFailTime, accountID)
		log.Printf("Circuit breaker cleared for account %d after success", accountID)
	}
}
