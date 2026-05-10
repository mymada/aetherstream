package device

import (
	"database/sql"
	"fmt"
	"net"
	"time"

	"github.com/labstack/echo/v4"
)

// TrustLevel defines device access levels
type TrustLevel string

const (
	TrustBlocked TrustLevel = "blocked"
	TrustGuest   TrustLevel = "guest"
	TrustUser    TrustLevel = "user"
	TrustAdmin   TrustLevel = "admin"
)

// Device represents a known device
type Device struct {
	ID          string     `json:"id"`
	Name        string     `json:"name,omitempty"`
	DeviceID    string     `json:"device_id"`
	IP          string     `json:"ip,omitempty"`
	TrustLevel  TrustLevel `json:"trust_level"`
	LastSeen    time.Time  `json:"last_seen,omitempty"`
	CreatedAt   time.Time  `json:"created_at,omitempty"`
}

// Store manages device access control
type Store struct {
	db *sql.DB
}

// NewStore creates a device store
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Migrate creates the devices table
func (s *Store) Migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS devices (
	id TEXT PRIMARY KEY,
	name TEXT,
	device_id TEXT UNIQUE NOT NULL,
	ip_address TEXT,
	trust_level TEXT NOT NULL DEFAULT 'user',
	last_seen DATETIME,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_devices_device_id ON devices(device_id);
CREATE INDEX IF NOT EXISTS idx_devices_ip ON devices(ip_address);
`
	_, err := s.db.Exec(schema)
	return err
}

// Register inserts or updates a device
func (s *Store) Register(id, name, deviceID, ip string, level TrustLevel) error {
	_, err := s.db.Exec(`
		INSERT INTO devices(id, name, device_id, ip_address, trust_level, last_seen)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(device_id) DO UPDATE SET
			name = excluded.name,
			ip_address = excluded.ip_address,
			trust_level = excluded.trust_level,
			last_seen = CURRENT_TIMESTAMP
	`, id, name, deviceID, ip, string(level))
	return err
}

// GetByDeviceID fetches a device by its device identifier
func (s *Store) GetByDeviceID(deviceID string) (*Device, error) {
	row := s.db.QueryRow(`
		SELECT id, name, device_id, ip_address, trust_level, last_seen, created_at
		FROM devices WHERE device_id = ?`, deviceID)
	var d Device
	var lastSeen, createdAt sql.NullTime
	err := row.Scan(&d.ID, &d.Name, &d.DeviceID, &d.IP, &d.TrustLevel, &lastSeen, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastSeen.Valid {
		d.LastSeen = lastSeen.Time
	}
	if createdAt.Valid {
		d.CreatedAt = createdAt.Time
	}
	return &d, nil
}

// GetByIP fetches a device by IP address
func (s *Store) GetByIP(ip string) (*Device, error) {
	row := s.db.QueryRow(`
		SELECT id, name, device_id, ip_address, trust_level, last_seen, created_at
		FROM devices WHERE ip_address = ?`, ip)
	var d Device
	var lastSeen, createdAt sql.NullTime
	err := row.Scan(&d.ID, &d.Name, &d.DeviceID, &d.IP, &d.TrustLevel, &lastSeen, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastSeen.Valid {
		d.LastSeen = lastSeen.Time
	}
	if createdAt.Valid {
		d.CreatedAt = createdAt.Time
	}
	return &d, nil
}

// List returns all registered devices
func (s *Store) List() ([]Device, error) {
	rows, err := s.db.Query(`
		SELECT id, name, device_id, ip_address, trust_level, last_seen, created_at
		FROM devices ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		var lastSeen, createdAt sql.NullTime
		if err := rows.Scan(&d.ID, &d.Name, &d.DeviceID, &d.IP, &d.TrustLevel, &lastSeen, &createdAt); err != nil {
			continue
		}
		if lastSeen.Valid {
			d.LastSeen = lastSeen.Time
		}
		if createdAt.Valid {
			d.CreatedAt = createdAt.Time
		}
		devices = append(devices, d)
	}
	return devices, nil
}

// UpdateTrust changes a device's trust level
func (s *Store) UpdateTrust(deviceID string, level TrustLevel) error {
	_, err := s.db.Exec(`UPDATE devices SET trust_level = ? WHERE device_id = ?`, string(level), deviceID)
	return err
}

// Delete removes a device
func (s *Store) Delete(deviceID string) error {
	_, err := s.db.Exec(`DELETE FROM devices WHERE device_id = ?`, deviceID)
	return err
}

// IsAllowed checks if a device (by deviceID or IP) is allowed access
func (s *Store) IsAllowed(deviceID, ip string) (bool, TrustLevel, error) {
	// Try deviceID first
	if deviceID != "" {
		d, err := s.GetByDeviceID(deviceID)
		if err != nil {
			return false, "", err
		}
		if d != nil {
			return d.TrustLevel != TrustBlocked, d.TrustLevel, nil
		}
	}
	// Fallback to IP
	if ip != "" {
		d, err := s.GetByIP(ip)
		if err != nil {
			return false, "", err
		}
		if d != nil {
			return d.TrustLevel != TrustBlocked, d.TrustLevel, nil
		}
	}
	// Unknown device: default allow (guest)
	return true, TrustGuest, nil
}

// Middleware returns Echo middleware that enforces device access control
func (s *Store) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			deviceID := c.Request().Header.Get("X-Device-ID")
			ip := c.RealIP()
			if ip == "" {
				host, _, _ := net.SplitHostPort(c.Request().RemoteAddr)
				ip = host
			}

			allowed, level, err := s.IsAllowed(deviceID, ip)
			if err != nil {
				return echo.NewHTTPError(500, "device check failed")
			}
			if !allowed {
				return echo.NewHTTPError(403, "device blocked")
			}

			c.Set("device_level", level)
			return next(c)
		}
	}
}

// RequireTrust returns middleware requiring minimum trust level
func RequireTrust(min TrustLevel) echo.MiddlewareFunc {
	levels := map[TrustLevel]int{
		TrustBlocked: 0,
		TrustGuest:   1,
		TrustUser:    2,
		TrustAdmin:   3,
	}
	minLevel := levels[min]
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			levelVal, ok := c.Get("device_level").(TrustLevel)
			if !ok {
				return echo.NewHTTPError(403, "device not identified")
			}
			if levels[levelVal] < minLevel {
				return echo.NewHTTPError(403, fmt.Sprintf("trust level %s required", min))
			}
			return next(c)
		}
	}
}
