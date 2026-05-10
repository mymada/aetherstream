package cluster

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// DBReplication handles shipping SQLite WAL to secondary nodes
type DBReplication struct {
	mu       sync.RWMutex
	dbPath   string
	walPath  string
	registry *NodeRegistry
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
	lastSent int64 // last WAL file size shipped
}

// NewDBReplication creates a replicator for the given SQLite DB path
func NewDBReplication(dbPath string, registry *NodeRegistry, interval time.Duration) *DBReplication {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &DBReplication{
		dbPath:   dbPath,
		walPath:  dbPath + "-wal",
		registry: registry,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins periodic WAL shipping
func (r *DBReplication) Start() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-r.stopCh:
				return
			case <-ticker.C:
				r.shipWAL()
			}
		}
	}()
}

// Stop halts replication
func (r *DBReplication) Stop() {
	close(r.stopCh)
	r.wg.Wait()
}

func (r *DBReplication) shipWAL() {
	// If no WAL file, nothing to ship
	if _, err := os.Stat(r.walPath); os.IsNotExist(err) {
		return
	}
	info, err := os.Stat(r.walPath)
	if err != nil {
		return
	}
	if info.Size() <= r.lastSent {
		return
	}

	data, err := os.ReadFile(r.walPath)
	if err != nil {
		log.Warn().Err(err).Str("wal", r.walPath).Msg("failed to read WAL")
		return
	}

	nodes := r.registry.HealthyNodes()
	for _, node := range nodes {
		if node.ID == r.registry.Self().ID {
			continue
		}
		go r.sendWALToNode(node, data)
	}

	r.lastSent = info.Size()
}

func (r *DBReplication) sendWALToNode(node NodeState, data []byte) {
	url := fmt.Sprintf("http://%s/api/cluster/replicate", node.Address)
	resp, err := http.Post(url, "application/octet-stream", bytes.NewReader(data))
	if err != nil {
		log.Warn().Err(err).Str("node", node.ID).Msg("WAL shipping failed")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Warn().Str("node", node.ID).Int("status", resp.StatusCode).Str("resp", string(body)).Msg("WAL shipping rejected")
		return
	}
	log.Debug().Str("node", node.ID).Int("bytes", len(data)).Msg("WAL shipped")
}

// ApplyWAL writes received WAL data to the local WAL path (secondary node usage)
func (r *DBReplication) ApplyWAL(data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	walDir := filepath.Dir(r.walPath)
	if err := os.MkdirAll(walDir, 0750); err != nil {
		return fmt.Errorf("mkdir wal dir: %w", err)
	}
	// Append received WAL data
	f, err := os.OpenFile(r.walPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return fmt.Errorf("open wal: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write wal: %w", err)
	}
	return nil
}
