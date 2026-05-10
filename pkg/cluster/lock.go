package cluster

import (
	"context"
	"sync"
	"time"
)

// DistributedLock provides etcd/consul-style locking using SQLite backend
type DistributedLock struct {
	mu       sync.Mutex
	backend  LockBackend
	locks    map[string]*lockEntry
}

// LockBackend abstracts the persistence layer (implemented via DB)
type LockBackend interface {
	TryAcquire(ctx context.Context, key, holder string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key, holder string) error
	Refresh(ctx context.Context, key, holder string, ttl time.Duration) error
}

// lockEntry tracks local lock state
type lockEntry struct {
	key      string
	holder   string
	ttl      time.Duration
	cancel   context.CancelFunc
	stopCh   chan struct{}
}

// NewDistributedLock creates a lock manager
func NewDistributedLock(backend LockBackend) *DistributedLock {
	return &DistributedLock{
		backend: backend,
		locks:   make(map[string]*lockEntry),
	}
}

// Acquire tries to get a distributed lock with the given TTL
func (dl *DistributedLock) Acquire(ctx context.Context, key, holder string, ttl time.Duration) (bool, error) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	// If we already hold this lock locally, check if backend still valid
	if entry, ok := dl.locks[key]; ok {
		// Try to refresh — if it fails, the lock was lost
		refreshCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := dl.backend.Refresh(refreshCtx, key, entry.holder, ttl); err != nil {
			// Lock lost, clean up local state and try to acquire fresh
			delete(dl.locks, key)
			close(entry.stopCh)
		} else {
			// We still hold it
			return false, nil
		}
	}

	acquired, err := dl.backend.TryAcquire(ctx, key, holder, ttl)
	if err != nil {
		return false, err
	}
	if !acquired {
		return false, nil
	}

	entry := &lockEntry{
		key:    key,
		holder: holder,
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}
	dl.locks[key] = entry

	// Start keep-alive goroutine
	go dl.keepAlive(entry)

	return true, nil
}

// Release gives up a held lock
func (dl *DistributedLock) Release(ctx context.Context, key, holder string) error {
	dl.mu.Lock()
	entry, ok := dl.locks[key]
	if ok {
		delete(dl.locks, key)
		close(entry.stopCh)
	}
	dl.mu.Unlock()

	return dl.backend.Release(ctx, key, holder)
}

// IsHeld checks if we currently hold a lock
func (dl *DistributedLock) IsHeld(key string) bool {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	_, ok := dl.locks[key]
	return ok
}

func (dl *DistributedLock) keepAlive(entry *lockEntry) {
	ticker := time.NewTicker(entry.ttl / 3)
	defer ticker.Stop()
	for {
		select {
		case <-entry.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := dl.backend.Refresh(ctx, entry.key, entry.holder, entry.ttl); err != nil {
				cancel()
				// Lock lost; clean up local state
				dl.mu.Lock()
				delete(dl.locks, entry.key)
				dl.mu.Unlock()
				return
			}
			cancel()
		}
	}
}
