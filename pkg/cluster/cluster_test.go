package cluster

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNodeRegistryGossip(t *testing.T) {
	// Use loopback ports to avoid collisions
	r1 := NewNodeRegistry("node-1", "127.0.0.1:18081", "127.0.0.1:19991", "primary")
	err := r1.Start()
	assert.NoError(t, err)
	defer r1.Stop()

	r2 := NewNodeRegistry("node-2", "127.0.0.1:18082", "127.0.0.1:19992", "secondary")
	err = r2.Start()
	assert.NoError(t, err)
	defer r2.Stop()

	// Manually inject node-2 into node-1 registry to simulate discovery
	r1.mu.Lock()
	r1.nodes["node-2"] = &NodeState{
		ID:         "node-2",
		Address:    "127.0.0.1:18082",
		GossipAddr: "127.0.0.1:19992",
		LastSeen:   time.Now(),
		Healthy:    true,
		Role:       "secondary",
	}
	r1.mu.Unlock()

	nodes := r1.Nodes()
	assert.Len(t, nodes, 1)
	assert.Equal(t, "node-2", nodes[0].ID)

	self := r1.Self()
	assert.Equal(t, "node-1", self.ID)
}

func TestLoadBalancerNext(t *testing.T) {
	r := NewNodeRegistry("node-a", "127.0.0.1:18081", "127.0.0.1:19991", "primary")
	_ = r.Start()
	defer r.Stop()

	lb := NewLoadBalancer(r, 1*time.Second)
	lb.Start()
	defer lb.Stop()

	// No healthy nodes yet
	_, ok := lb.Next()
	assert.False(t, ok)

	// Inject a healthy node
	r.mu.Lock()
	r.nodes["node-b"] = &NodeState{
		ID:       "node-b",
		Address:  "127.0.0.1:18082",
		LastSeen: time.Now(),
		Healthy:  true,
	}
	r.mu.Unlock()

	node, ok := lb.Next()
	assert.True(t, ok)
	assert.Equal(t, "node-b", node.ID)
}

func TestDistributedLockAcquireRelease(t *testing.T) {
	// Use an in-memory backend for unit testing
	backend := NewTestLockBackend()
	lockMgr := NewDistributedLock(backend)

	ctx := context.Background()
	acquired, err := lockMgr.Acquire(ctx, "job-1", "holder-a", 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, acquired)
	assert.True(t, lockMgr.IsHeld("job-1"))

	// Another holder should fail (locally we already hold it)
	acquired2, err := lockMgr.Acquire(ctx, "job-1", "holder-b", 2*time.Second)
	assert.NoError(t, err)
	assert.False(t, acquired2)

	// Release and re-acquire
	err = lockMgr.Release(ctx, "job-1", "holder-a")
	assert.NoError(t, err)
	assert.False(t, lockMgr.IsHeld("job-1"))

	acquired3, err := lockMgr.Acquire(ctx, "job-1", "holder-b", 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, acquired3)
	assert.True(t, lockMgr.IsHeld("job-1"))
}

func TestDistributedLockExpiry(t *testing.T) {
	backend := NewTestLockBackend()
	lockMgr := NewDistributedLock(backend)

	ctx := context.Background()
	// Acquire with short TTL
	acquired, err := lockMgr.Acquire(ctx, "expiring", "h1", 100*time.Millisecond)
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Manually stop keep-alive by releasing
	_ = lockMgr.Release(ctx, "expiring", "h1")

	// Wait for backend TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Another holder can now acquire (backend expired)
	acquired2, err := lockMgr.Acquire(ctx, "expiring", "h2", 2*time.Second)
	assert.NoError(t, err)
	assert.True(t, acquired2)
	assert.True(t, lockMgr.IsHeld("expiring"))

	// Cleanup
	_ = lockMgr.Release(ctx, "expiring", "h2")
}

// --- Test backend in-memory implementation ---

type TestLockBackend struct {
	mu     sync.Mutex
	locks  map[string]testLockRow
}

type testLockRow struct {
	holder    string
	expiresAt time.Time
}

func NewTestLockBackend() *TestLockBackend {
	return &TestLockBackend{locks: make(map[string]testLockRow)}
}

func (b *TestLockBackend) TryAcquire(ctx context.Context, key, holder string, ttl time.Duration) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	row, ok := b.locks[key]
	if !ok || time.Now().After(row.expiresAt) {
		b.locks[key] = testLockRow{holder: holder, expiresAt: time.Now().Add(ttl)}
		return true, nil
	}
	return false, nil
}

func (b *TestLockBackend) Release(ctx context.Context, key, holder string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	row, ok := b.locks[key]
	if !ok || row.holder != holder {
		return fmt.Errorf("lock not held by %s", holder)
	}
	delete(b.locks, key)
	return nil
}

func (b *TestLockBackend) Refresh(ctx context.Context, key, holder string, ttl time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	row, ok := b.locks[key]
	if !ok || row.holder != holder {
		return fmt.Errorf("lock not held by %s", holder)
	}
	b.locks[key] = testLockRow{holder: holder, expiresAt: time.Now().Add(ttl)}
	return nil
}
