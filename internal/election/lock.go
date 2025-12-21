// Package election provides leader election functionality
package election

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"mysql-ha/internal/dcs"
)

// LockManager manages the leader lock
type LockManager struct {
	dcs         dcs.DCS
	lockKey     string
	ttl         time.Duration
	renewPeriod time.Duration
	isLeader    atomic.Bool
	stopCh      chan struct{}
	mu          sync.Mutex
}

// NewLockManager creates a new LockManager
func NewLockManager(d dcs.DCS, lockKey string, ttl time.Duration) *LockManager {
	return &LockManager{
		dcs:         d,
		lockKey:     lockKey,
		ttl:         ttl,
		renewPeriod: CalculateRenewalInterval(ttl),
		stopCh:      make(chan struct{}),
	}
}

// CalculateRenewalInterval calculates the lock renewal interval as TTL/3
func CalculateRenewalInterval(ttl time.Duration) time.Duration {
	return ttl / 3
}

// TryAcquire attempts to acquire the leader lock
func (lm *LockManager) TryAcquire(ctx context.Context) (bool, error) {
	acquired, err := lm.dcs.AcquireLock(ctx, lm.lockKey, lm.ttl)
	if err != nil {
		return false, err
	}
	if acquired {
		lm.isLeader.Store(true)
	}
	return acquired, nil
}

// Renew renews the leader lock
func (lm *LockManager) Renew(ctx context.Context) error {
	if !lm.isLeader.Load() {
		return nil
	}
	return lm.dcs.RenewLock(ctx, lm.lockKey)
}

// Release releases the leader lock
func (lm *LockManager) Release(ctx context.Context) error {
	lm.isLeader.Store(false)
	return lm.dcs.ReleaseLock(ctx, lm.lockKey)
}

// IsLeader returns true if this node holds the leader lock
func (lm *LockManager) IsLeader() bool {
	return lm.isLeader.Load()
}

// StartRenewal starts the lock renewal goroutine
func (lm *LockManager) StartRenewal(ctx context.Context) {
	lm.mu.Lock()
	lm.stopCh = make(chan struct{})
	lm.mu.Unlock()

	go func() {
		ticker := time.NewTicker(lm.renewPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-lm.stopCh:
				return
			case <-ticker.C:
				if lm.isLeader.Load() {
					if err := lm.Renew(ctx); err != nil {
						// Lost leadership
						lm.isLeader.Store(false)
						return
					}
				}
			}
		}
	}()
}

// StopRenewal stops the lock renewal goroutine
func (lm *LockManager) StopRenewal() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	select {
	case <-lm.stopCh:
		// Already closed
	default:
		close(lm.stopCh)
	}
}

// GetRenewalPeriod returns the renewal period
func (lm *LockManager) GetRenewalPeriod() time.Duration {
	return lm.renewPeriod
}

// GetTTL returns the lock TTL
func (lm *LockManager) GetTTL() time.Duration {
	return lm.ttl
}
