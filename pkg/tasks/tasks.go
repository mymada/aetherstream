package tasks

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Job represents a background transcode task.
type Job struct {
	ID          string     `json:"id"`
	ItemID      string     `json:"item_id"`
	ItemTitle   string     `json:"item_title"`
	Key         string     `json:"key"`
	AudioIndex  int        `json:"audio_index"`
	Profiles    []string   `json:"profiles"`
	Status      Status     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Err         string     `json:"error,omitempty"`
	OutputDir   string     `json:"output_dir"`
	DiskBytes   int64      `json:"disk_bytes,omitempty"`

	cancel context.CancelFunc
	Ctx    context.Context `json:"-"`
}

// Manager tracks active and recent transcode jobs.
type Manager struct {
	mu      sync.RWMutex
	active  map[string]*Job // transcode key → job
	history []*Job
	maxHist int
}

// NewManager creates a Manager keeping up to maxHistory completed jobs.
func NewManager(maxHistory int) *Manager {
	if maxHistory <= 0 {
		maxHistory = 100
	}
	return &Manager{
		active:  make(map[string]*Job),
		maxHist: maxHistory,
	}
}

// Submit creates a new job for key. Returns (existing, true) if key is already active.
func (m *Manager) Submit(itemID, itemTitle, key, outputDir string, audioIndex int, profiles []string) (*Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.active[key]; ok {
		return existing, true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	job := &Job{
		ID:         uuid.New().String(),
		ItemID:     itemID,
		ItemTitle:  itemTitle,
		Key:        key,
		AudioIndex: audioIndex,
		Profiles:   profiles,
		Status:     StatusQueued,
		CreatedAt:  time.Now(),
		OutputDir:  outputDir,
		cancel:     cancel,
		Ctx:        ctx,
	}
	m.active[key] = job
	return job, false
}

// SetRunning marks the job identified by id as running.
func (m *Manager) SetRunning(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.active {
		if j.ID == id {
			j.Status = StatusRunning
			now := time.Now()
			j.StartedAt = &now
			return
		}
	}
}

// Complete marks the job for key as completed or failed and moves it to history.
func (m *Manager) Complete(key string, diskBytes int64, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.active[key]
	if !ok {
		return
	}
	delete(m.active, key)
	job.cancel()

	now := time.Now()
	job.CompletedAt = &now
	job.DiskBytes = diskBytes
	if err != nil {
		if job.Status != StatusCancelled {
			job.Status = StatusFailed
			job.Err = err.Error()
		}
	} else {
		job.Status = StatusCompleted
	}

	m.history = append([]*Job{job}, m.history...)
	if len(m.history) > m.maxHist {
		m.history = m.history[:m.maxHist]
	}
}

// Cancel cancels the job with the given id. Returns false if not found.
func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, j := range m.active {
		if j.ID == id {
			j.Status = StatusCancelled
			j.cancel()
			delete(m.active, key)
			now := time.Now()
			j.CompletedAt = &now
			m.history = append([]*Job{j}, m.history...)
			if len(m.history) > m.maxHist {
				m.history = m.history[:m.maxHist]
			}
			return true
		}
	}
	return false
}

// List returns all active jobs followed by history (newest first).
func (m *Manager) List() []*Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Job, 0, len(m.active)+len(m.history))
	for _, j := range m.active {
		cp := *j
		result = append(result, &cp)
	}
	result = append(result, m.history...)
	return result
}

// IsActive returns true if a job for key is currently queued or running.
func (m *Manager) IsActive(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.active[key]
	return ok
}

// ActiveCount returns the number of active (queued+running) jobs.
func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.active)
}
