package cluster

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// ClusterServer wraps cluster services and exposes HTTP endpoints
type ClusterServer struct {
	registry     *NodeRegistry
	balancer     *LoadBalancer
	replication  *DBReplication
	lockMgr      *DistributedLock
	selfRole     string
}

// NewClusterServer creates the cluster HTTP handler
func NewClusterServer(registry *NodeRegistry, balancer *LoadBalancer, replication *DBReplication, lockMgr *DistributedLock, role string) *ClusterServer {
	return &ClusterServer{
		registry:    registry,
		balancer:    balancer,
		replication: replication,
		lockMgr:     lockMgr,
		selfRole:    role,
	}
}

// RegisterRoutes adds cluster endpoints to Echo
func (cs *ClusterServer) RegisterRoutes(e *echo.Echo) {
	// Public / internal cluster endpoints
	e.GET("/api/cluster/nodes", cs.handleListNodes)
	e.POST("/api/cluster/join", cs.handleJoin)
	e.POST("/api/cluster/replicate", cs.handleReplicate)
}

func (cs *ClusterServer) handleListNodes(c echo.Context) error {
	nodes := cs.registry.Nodes()
	self := cs.registry.Self()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"self":  self,
		"nodes": nodes,
	})
}

func (cs *ClusterServer) handleJoin(c echo.Context) error {
	var req struct {
		NodeID     string `json:"node_id"`
		Address    string `json:"address"`
		GossipAddr string `json:"gossip_addr"`
		Role       string `json:"role"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if req.NodeID == "" || req.Address == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "node_id and address required")
	}

	// Add to registry directly
	node := NodeState{
		ID:         req.NodeID,
		Address:    req.Address,
		GossipAddr: req.GossipAddr,
		LastSeen:   time.Now(),
		Healthy:    true,
		Role:       req.Role,
	}
	cs.registry.mu.Lock()
	cs.registry.nodes[req.NodeID] = &node
	cs.registry.mu.Unlock()

	log.Info().Str("node_id", req.NodeID).Str("addr", req.Address).Msg("node joined cluster")

	// Return current cluster view
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "joined",
		"nodes":  cs.registry.Nodes(),
	})
}

func (cs *ClusterServer) handleReplicate(c echo.Context) error {
	if cs.replication == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "replication not configured")
	}
	data, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "bad body")
	}
	if err := cs.replication.ApplyWAL(data); err != nil {
		log.Warn().Err(err).Msg("WAL apply failed")
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ProxyToNext proxies the request to the next healthy node via round-robin
func (cs *ClusterServer) ProxyToNext(c echo.Context) error {
	node, ok := cs.balancer.Next()
	if !ok {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "no healthy nodes")
	}
	return proxyRequest(c, node.Address)
}

func proxyRequest(c echo.Context, targetAddr string) error {
	req := c.Request()
	url := "http://" + targetAddr + req.URL.Path
	if req.URL.RawQuery != "" {
		url += "?" + req.URL.RawQuery
	}

	body, _ := io.ReadAll(req.Body)
	proxyReq, err := http.NewRequest(req.Method, url, bytes.NewReader(body))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	for k, v := range req.Header {
		proxyReq.Header[k] = v
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, err.Error())
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			c.Response().Header().Add(k, v)
		}
	}
	c.Response().WriteHeader(resp.StatusCode)
	_, _ = io.Copy(c.Response().Writer, resp.Body)
	return nil
}
