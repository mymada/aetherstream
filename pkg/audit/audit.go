package audit

import (
	"database/sql"
	"fmt"
	"time"
)

// Logger records security-relevant events to SQLite.
type Logger struct {
	db *sql.DB
}

// NewLogger creates an audit logger backed by db.
func NewLogger(db *sql.DB) *Logger {
	return &Logger{db: db}
}

// Migrate creates the audit_logs table if it does not exist.
func (l *Logger) Migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS audit_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
	user_id TEXT,
	username TEXT,
	action TEXT NOT NULL,
	resource TEXT,
	resource_id TEXT,
	ip_address TEXT,
	user_agent TEXT,
	details TEXT
);
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_logs(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action);
`
	if _, err := l.db.Exec(schema); err != nil {
		return fmt.Errorf("audit migrate: %w", err)
	}
	return nil
}

// Event represents a single auditable action.
type Event struct {
	Timestamp time.Time
	UserID    string
	Username  string
	Action    string
	Resource  string
	ResourceID string
	IP        string
	UserAgent string
	Details   string
}

// Log writes an audit event to the database asynchronously (best-effort).
func (l *Logger) Log(e Event) {
	// Best-effort async logging; ignore errors to avoid disrupting requests.
	go func() {
		_, _ = l.db.Exec(
			`INSERT INTO audit_logs(timestamp, user_id, username, action, resource, resource_id, ip_address, user_agent, details)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.Timestamp, e.UserID, e.Username, e.Action, e.Resource, e.ResourceID, e.IP, e.UserAgent, e.Details,
		)
	}()
}

// Query returns recent audit events ordered by timestamp descending.
func (l *Logger) Query(limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := l.db.Query(
		`SELECT timestamp, user_id, username, action, resource, resource_id, ip_address, user_agent, details
		 FROM audit_logs ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var ts string
		if err := rows.Scan(&ts, &e.UserID, &e.Username, &e.Action, &e.Resource, &e.ResourceID, &e.IP, &e.UserAgent, &e.Details); err != nil {
			continue
		}
		e.Timestamp, _ = time.Parse(time.RFC3339, ts)
		if e.Timestamp.IsZero() {
			e.Timestamp, _ = time.Parse("2006-01-02 15:04:05", ts)
		}
		events = append(events, e)
	}
	return events, nil
}
