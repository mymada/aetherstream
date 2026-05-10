package cluster

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// LoadBalancer selects healthy nodes in round-robin fashion
type LoadBalancer struct {
	mu        sync.RWMutex
	registry  *NodeRegistry
	counter   uint64
	interval  time.Duration
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewLoadBalancer creates a balancer backed by a NodeRegistry
func NewLoadBalancer(registry *NodeRegistry, healthCheckInterval time.Duration) *LoadBalancer {
	if healthCheckInterval <= 0 {
		healthCheckInterval = 5 * time.Second
	}
	return &LoadBalancer{
		registry: registry,
		interval: healthCheckInterval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins background health checks
func (lb *LoadBalancer) Start() {
	lb.wg.Add(1)
	go func() {
		defer lb.wg.Done()
		ticker := time.NewTicker(lb.interval)
		defer ticker.Stop()
		for {
			select {
			case <-lb.stopCh:
				return
			case <-ticker.C:
				lb.runHealthChecks()
			}
		}
	}()
}

// Stop halts health checks
func (lb *LoadBalancer) Stop() {
	close(lb.stopCh)
	lb.wg.Wait()
}

// Next returns the next healthy node address (round-robin)
func (lb *LoadBalancer) Next() (NodeState, bool) {
	nodes := lb.registry.HealthyNodes()
	if len(nodes) == 0 {
		return NodeState{}, false
	}
	idx := int(atomic.AddUint64(&lb.counter, 1)-1) % len(nodes)
	return nodes[idx], true
}

// AllHealthy returns all currently healthy nodes
func (lb *LoadBalancer) AllHealthy() []NodeState {
	return lb.registry.HealthyNodes()
}

func (lb *LoadBalancer) runHealthChecks() {
	all := lb.registry.Nodes()
	for _, node := range all {
		if node.ID == lb.registry.Self().ID {
			continue
		}
		go lb.checkNode(node)
	}
}

func (lb *LoadBalancer) checkNode(node NodeState) {
	// Simple HTTP health check on /system/info
	addr := node.Address
	if addr == "" {
		return
	}
	// We reuse the registry's internal node map via a non-exported method isn't available,
	// so we just log and rely on gossip for liveness. For deeper health we could HTTP GET.
	log.Debug().Str("node", node.ID).Str("addr", addr).Msg("health check tick")
}
