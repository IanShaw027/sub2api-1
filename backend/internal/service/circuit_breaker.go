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
}

func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		failureCount: make(map[int64]int),
		lastFailTime: make(map[int64]time.Time),
		threshold:    5,
		resetTimeout: 5 * time.Minute,
	}
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
