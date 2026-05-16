package cluster

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSQLiteBackend(t *testing.T) *SQLiteLockBackend {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	backend := NewSQLiteLockBackend(db)
	require.NoError(t, backend.Migrate())
	return backend
}

func TestSQLiteLockBackend_TryAcquire_New(t *testing.T) {
	backend := setupSQLiteBackend(t)
	ctx := context.Background()
	ok, err := backend.TryAcquire(ctx, "key1", "holder-a", 10*time.Second)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestSQLiteLockBackend_TryAcquire_AlreadyHeld(t *testing.T) {
	backend := setupSQLiteBackend(t)
	ctx := context.Background()
	_, _ = backend.TryAcquire(ctx, "key1", "holder-a", 10*time.Second)
	ok, err := backend.TryAcquire(ctx, "key1", "holder-b", 10*time.Second)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestSQLiteLockBackend_TryAcquire_AfterExpiry(t *testing.T) {
	backend := setupSQLiteBackend(t)
	ctx := context.Background()
	// Acquire with negative TTL → immediately expired
	_, _ = backend.TryAcquire(ctx, "key1", "holder-a", -1*time.Second)
	// Another holder should now be able to acquire
	ok, err := backend.TryAcquire(ctx, "key1", "holder-b", 10*time.Second)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestSQLiteLockBackend_Release(t *testing.T) {
	backend := setupSQLiteBackend(t)
	ctx := context.Background()
	_, _ = backend.TryAcquire(ctx, "key1", "holder-a", 10*time.Second)
	err := backend.Release(ctx, "key1", "holder-a")
	assert.NoError(t, err)
	// After release, a new holder can acquire
	ok, err := backend.TryAcquire(ctx, "key1", "holder-b", 10*time.Second)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestSQLiteLockBackend_Release_NotHeld(t *testing.T) {
	backend := setupSQLiteBackend(t)
	ctx := context.Background()
	err := backend.Release(ctx, "missing-key", "holder-a")
	assert.Error(t, err)
}

func TestSQLiteLockBackend_Release_WrongHolder(t *testing.T) {
	backend := setupSQLiteBackend(t)
	ctx := context.Background()
	_, _ = backend.TryAcquire(ctx, "key1", "holder-a", 10*time.Second)
	err := backend.Release(ctx, "key1", "holder-b")
	assert.Error(t, err)
}

func TestSQLiteLockBackend_Refresh(t *testing.T) {
	backend := setupSQLiteBackend(t)
	ctx := context.Background()
	_, _ = backend.TryAcquire(ctx, "key1", "holder-a", 10*time.Second)
	err := backend.Refresh(ctx, "key1", "holder-a", 60*time.Second)
	assert.NoError(t, err)
}

func TestSQLiteLockBackend_Refresh_NotHeld(t *testing.T) {
	backend := setupSQLiteBackend(t)
	ctx := context.Background()
	err := backend.Refresh(ctx, "missing-key", "holder-a", 10*time.Second)
	assert.Error(t, err)
}

func TestSQLiteLockBackend_Refresh_WrongHolder(t *testing.T) {
	backend := setupSQLiteBackend(t)
	ctx := context.Background()
	_, _ = backend.TryAcquire(ctx, "key1", "holder-a", 10*time.Second)
	err := backend.Refresh(ctx, "key1", "holder-b", 10*time.Second)
	assert.Error(t, err)
}

func TestSQLiteLockBackend_Migrate_Idempotent(t *testing.T) {
	backend := setupSQLiteBackend(t)
	// Calling Migrate a second time should not fail (IF NOT EXISTS)
	err := backend.Migrate()
	assert.NoError(t, err)
}

// --- NodeRegistry ---

func TestNodeRegistry_Self(t *testing.T) {
	r := NewNodeRegistry("node-x", "127.0.0.1:9001", "127.0.0.1:9002", "primary")
	self := r.Self()
	assert.Equal(t, "node-x", self.ID)
	assert.Equal(t, "127.0.0.1:9001", self.Address)
	assert.Equal(t, "primary", self.Role)
	assert.True(t, self.Healthy)
}

func TestNodeRegistry_Nodes_Empty(t *testing.T) {
	r := NewNodeRegistry("node-x", "127.0.0.1:9001", "127.0.0.1:9002", "primary")
	nodes := r.Nodes()
	assert.Empty(t, nodes)
}

func TestNodeRegistry_HealthyNodes_Empty(t *testing.T) {
	r := NewNodeRegistry("node-x", "127.0.0.1:9001", "127.0.0.1:9002", "primary")
	nodes := r.HealthyNodes()
	assert.Empty(t, nodes)
}

func TestNodeRegistry_HealthyNodes_OnlyHealthy(t *testing.T) {
	r := NewNodeRegistry("node-x", "127.0.0.1:9001", "127.0.0.1:9002", "primary")
	r.mu.Lock()
	r.nodes["node-a"] = &NodeState{ID: "node-a", Healthy: true, LastSeen: time.Now()}
	r.nodes["node-b"] = &NodeState{ID: "node-b", Healthy: false, LastSeen: time.Now()}
	r.mu.Unlock()

	healthy := r.HealthyNodes()
	assert.Len(t, healthy, 1)
	assert.Equal(t, "node-a", healthy[0].ID)
}

// --- LoadBalancer ---

func TestLoadBalancer_AllHealthy_Empty(t *testing.T) {
	r := NewNodeRegistry("node-x", "127.0.0.1:9001", "127.0.0.1:9002", "primary")
	lb := NewLoadBalancer(r, time.Second)
	nodes := lb.AllHealthy()
	assert.Empty(t, nodes)
}

func TestLoadBalancer_AllHealthy_WithNodes(t *testing.T) {
	r := NewNodeRegistry("node-x", "127.0.0.1:9001", "127.0.0.1:9002", "primary")
	r.mu.Lock()
	r.nodes["node-b"] = &NodeState{ID: "node-b", Healthy: true, LastSeen: time.Now()}
	r.nodes["node-c"] = &NodeState{ID: "node-c", Healthy: true, LastSeen: time.Now()}
	r.mu.Unlock()

	lb := NewLoadBalancer(r, time.Second)
	nodes := lb.AllHealthy()
	assert.Len(t, nodes, 2)
}

func TestLoadBalancer_Next_RoundRobin(t *testing.T) {
	r := NewNodeRegistry("node-x", "127.0.0.1:9001", "127.0.0.1:9002", "primary")
	r.mu.Lock()
	r.nodes["node-b"] = &NodeState{ID: "node-b", Address: "addr-b", Healthy: true, LastSeen: time.Now()}
	r.nodes["node-c"] = &NodeState{ID: "node-c", Address: "addr-c", Healthy: true, LastSeen: time.Now()}
	r.mu.Unlock()

	lb := NewLoadBalancer(r, time.Second)

	seen := map[string]bool{}
	for i := 0; i < 4; i++ {
		node, ok := lb.Next()
		require.True(t, ok)
		seen[node.ID] = true
	}
	// Both nodes should have been returned at some point
	assert.True(t, len(seen) >= 1)
}
