package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// NodeState represents the known state of a peer node
type NodeState struct {
	ID        string    `json:"id"`
	Address   string    `json:"address"`   // HTTP API address (host:port)
	GossipAddr string   `json:"gossip_addr"` // UDP gossip address
	LastSeen  time.Time `json:"last_seen"`
	Healthy   bool      `json:"healthy"`
	Role      string    `json:"role"` // "primary" or "secondary"
}

// NodeRegistry manages cluster membership via gossip protocol
type NodeRegistry struct {
	mu sync.RWMutex

	selfID      string
	selfAddr    string
	selfGossip  string
	role        string

	nodes       map[string]*NodeState
	listeners   []func(NodeState)

	gossipConn  *net.UDPConn
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// GossipMessage is the UDP broadcast payload
type GossipMessage struct {
	Type      string    `json:"type"` // "ping" or "pong"
	NodeID    string    `json:"node_id"`
	Address   string    `json:"address"`
	GossipAddr string   `json:"gossip_addr"`
	Timestamp int64     `json:"timestamp"`
}

// NewNodeRegistry creates a registry for this node
func NewNodeRegistry(nodeID, selfAddr, gossipAddr, role string) *NodeRegistry {
	return &NodeRegistry{
		selfID:     nodeID,
		selfAddr:   selfAddr,
		selfGossip: gossipAddr,
		role:       role,
		nodes:      make(map[string]*NodeState),
		stopCh:     make(chan struct{}),
	}
}

// Start begins UDP gossip listener and periodic broadcasts
func (r *NodeRegistry) Start() error {
	addr, err := net.ResolveUDPAddr("udp", r.selfGossip)
	if err != nil {
		return fmt.Errorf("resolve gossip addr: %w", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen gossip: %w", err)
	}
	r.gossipConn = conn

	// Listener goroutine
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		buf := make([]byte, 4096)
		for {
			select {
			case <-r.stopCh:
				return
			default:
			}
			_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, remote, err := conn.ReadFromUDP(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}
			var msg GossipMessage
			if err := json.Unmarshal(buf[:n], &msg); err != nil {
				continue
			}
			r.handleGossip(msg, remote)
		}
	}()

	// Periodic broadcast
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.stopCh:
				return
			case <-ticker.C:
				r.broadcastPing()
				r.pruneStaleNodes()
			}
		}
	}()

	// Immediate first broadcast
	go r.broadcastPing()

	log.Info().Str("node_id", r.selfID).Str("gossip", r.selfGossip).Msg("NodeRegistry started")
	return nil
}

// Stop shuts down gossip
func (r *NodeRegistry) Stop() {
	close(r.stopCh)
	if r.gossipConn != nil {
		_ = r.gossipConn.Close()
	}
	r.wg.Wait()
}

func (r *NodeRegistry) handleGossip(msg GossipMessage, remote *net.UDPAddr) {
	if msg.NodeID == r.selfID {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	node, exists := r.nodes[msg.NodeID]
	if !exists {
		node = &NodeState{
			ID:         msg.NodeID,
			Address:    msg.Address,
			GossipAddr: msg.GossipAddr,
			Healthy:    true,
			Role:       "secondary",
		}
		r.nodes[msg.NodeID] = node
		log.Info().Str("node_id", msg.NodeID).Str("addr", msg.Address).Msg("new node discovered")
		for _, fn := range r.listeners {
			go fn(*node)
		}
	}

	node.LastSeen = time.Now()
	node.Address = msg.Address
	node.GossipAddr = msg.GossipAddr
	node.Healthy = true

	// If we got a ping, send a unicast pong back
	if msg.Type == "ping" {
		go r.sendPong(remote)
	}
}

func (r *NodeRegistry) broadcastPing() {
	msg := GossipMessage{
		Type:       "ping",
		NodeID:     r.selfID,
		Address:    r.selfAddr,
		GossipAddr: r.selfGossip,
		Timestamp:  time.Now().Unix(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	// Broadcast to subnet on gossip port
	port := r.selfGossip
	if host, p, err := net.SplitHostPort(r.selfGossip); err == nil {
		if host == "0.0.0.0" || host == "" {
			host = "255.255.255.255"
		}
		port = net.JoinHostPort(host, p)
	}

	addr, err := net.ResolveUDPAddr("udp", port)
	if err != nil {
		return
	}
	if r.gossipConn != nil {
		_, _ = r.gossipConn.WriteToUDP(data, addr)
	}
}

func (r *NodeRegistry) sendPong(remote *net.UDPAddr) {
	msg := GossipMessage{
		Type:       "pong",
		NodeID:     r.selfID,
		Address:    r.selfAddr,
		GossipAddr: r.selfGossip,
		Timestamp:  time.Now().Unix(),
	}
	data, _ := json.Marshal(msg)
	if r.gossipConn != nil {
		_, _ = r.gossipConn.WriteToUDP(data, remote)
	}
}

func (r *NodeRegistry) pruneStaleNodes() {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().Add(-15 * time.Second)
	for id, node := range r.nodes {
		if node.LastSeen.Before(cutoff) {
			node.Healthy = false
			log.Warn().Str("node_id", id).Msg("node marked unhealthy (stale)")
			// Keep in map but mark unhealthy; remove after longer timeout
			if node.LastSeen.Before(time.Now().Add(-2 * time.Minute)) {
				delete(r.nodes, id)
				log.Info().Str("node_id", id).Msg("node removed from registry")
			}
		}
	}
}

// Nodes returns a snapshot of known nodes (excluding self)
func (r *NodeRegistry) Nodes() []NodeState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []NodeState
	for _, n := range r.nodes {
		out = append(out, *n)
	}
	return out
}

// HealthyNodes returns only healthy nodes
func (r *NodeRegistry) HealthyNodes() []NodeState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []NodeState
	for _, n := range r.nodes {
		if n.Healthy {
			out = append(out, *n)
		}
	}
	return out
}

// Self returns this node's info
func (r *NodeRegistry) Self() NodeState {
	return NodeState{
		ID:         r.selfID,
		Address:    r.selfAddr,
		GossipAddr: r.selfGossip,
		LastSeen:   time.Now(),
		Healthy:    true,
		Role:       r.role,
	}
}

// OnNodeChange registers a callback for new/updated nodes
func (r *NodeRegistry) OnNodeChange(fn func(NodeState)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listeners = append(r.listeners, fn)
}

// JoinCluster sends HTTP POST /api/cluster/join to a known seed node
func (r *NodeRegistry) JoinCluster(seedAddr string) error {
	payload := map[string]string{
		"node_id":     r.selfID,
		"address":     r.selfAddr,
		"gossip_addr": r.selfGossip,
		"role":        r.role,
	}
	data, _ := json.Marshal(payload)
	url := fmt.Sprintf("http://%s/api/cluster/join", seedAddr)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data)) // #nosec G107 - URL constructed from known seed address
	if err != nil {
		return fmt.Errorf("join request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("join rejected: %s", resp.Status)
	}
	return nil
}
