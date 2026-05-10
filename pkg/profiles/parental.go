package profiles

import (
	"crypto/subtle"
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ParentalDB is the database interface expected by parental control.
type ParentalDB interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

// ParentalSettings stores parental control configuration for a user.
type ParentalSettings struct {
	UserID            string    `json:"userId"`
	Enabled           bool      `json:"enabled"`
	PINHash           string    `json:"-"`
	MaxAgeRestriction int       `json:"maxAgeRestriction"` // 0 = unrestricted
	MaxRating         string    `json:"maxRating"`
	BlockUnrated      bool      `json:"blockUnrated"`
	TimeWindowStart   string    `json:"timeWindowStart,omitempty"` // HH:MM
	TimeWindowEnd     string    `json:"timeWindowEnd,omitempty"`   // HH:MM
	UpdatedAt         time.Time `json:"updatedAt"`
}

// ParentalService manages parental controls.
type ParentalService struct {
	db ParentalDB
}

// NewParentalService creates a parental control service.
func NewParentalService(database ParentalDB) *ParentalService {
	return &ParentalService{db: database}
}

// SetPIN sets or updates the parental PIN for a user.
func (s *ParentalService) SetPIN(userID, pin string) error {
	if len(pin) < 4 || len(pin) > 8 {
		return fmt.Errorf("PIN must be between 4 and 8 digits")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash PIN: %w", err)
	}
	return UpsertParentalSettings(s.db, userID, string(hash), 0, "", false, "", "")
}

// VerifyPIN checks if the provided PIN matches the stored hash.
func (s *ParentalService) VerifyPIN(userID, pin string) (bool, error) {
	settings, err := GetParentalSettings(s.db, userID)
	if err != nil {
		return false, err
	}
	if settings.PINHash == "" {
		return false, fmt.Errorf("no PIN set")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(settings.PINHash), []byte(pin)); err != nil {
		return false, nil
	}
	return true, nil
}

// UpdateRestrictions updates age/rating and time window restrictions.
func (s *ParentalService) UpdateRestrictions(userID string, maxAge int, maxRating string, blockUnrated bool, windowStart, windowEnd string) error {
	return UpsertParentalSettings(s.db, userID, "", maxAge, maxRating, blockUnrated, windowStart, windowEnd)
}

// GetSettings returns parental settings for a user.
func (s *ParentalService) GetSettings(userID string) (*ParentalSettings, error) {
	return GetParentalSettings(s.db, userID)
}

// IsAllowed checks if content is allowed for a user at the current time.
func (s *ParentalService) IsAllowed(userID string, contentRating string, contentAgeRestriction int, unrated bool) (bool, string, error) {
	settings, err := GetParentalSettings(s.db, userID)
	if err != nil {
		return false, "", err
	}
	if !settings.Enabled {
		return true, "", nil
	}

	// Time window check
	if settings.TimeWindowStart != "" && settings.TimeWindowEnd != "" {
		now := time.Now()
		start, err1 := time.Parse("15:04", settings.TimeWindowStart)
		end, err2 := time.Parse("15:04", settings.TimeWindowEnd)
		if err1 == nil && err2 == nil {
			current := time.Date(0, 1, 1, now.Hour(), now.Minute(), 0, 0, time.Local)
			windowStart := time.Date(0, 1, 1, start.Hour(), start.Minute(), 0, 0, time.Local)
			windowEnd := time.Date(0, 1, 1, end.Hour(), end.Minute(), 0, 0, time.Local)
			if windowStart.Before(windowEnd) {
				if current.Before(windowStart) || current.After(windowEnd) {
					return false, "outside allowed time window", nil
				}
			} else {
				// Overnight window
				if current.Before(windowStart) && current.After(windowEnd) {
					return false, "outside allowed time window", nil
				}
			}
		}
	}

	if unrated && settings.BlockUnrated {
		return false, "unrated content blocked", nil
	}

	if contentAgeRestriction > 0 && settings.MaxAgeRestriction > 0 && contentAgeRestriction > settings.MaxAgeRestriction {
		return false, fmt.Sprintf("age restriction %d exceeds limit %d", contentAgeRestriction, settings.MaxAgeRestriction), nil
	}

	if contentRating != "" && settings.MaxRating != "" {
		if exceedsRating(contentRating, settings.MaxRating) {
			return false, fmt.Sprintf("rating %s exceeds limit %s", contentRating, settings.MaxRating), nil
		}
	}

	return true, "", nil
}

// IsEnabled returns whether parental control is enabled for a user.
func (s *ParentalService) IsEnabled(userID string) (bool, error) {
	settings, err := GetParentalSettings(s.db, userID)
	if err != nil {
		return false, err
	}
	return settings.Enabled, nil
}

// exceedsRating returns true if contentRating exceeds maxRating.
// #nosec G115 - false positive for rating comparison
func exceedsRating(contentRating, maxRating string) bool {
	order := map[string]int{"G": 1, "PG": 2, "PG-13": 3, "R": 4, "NC-17": 5}
	c, ok1 := order[contentRating]
	m, ok2 := order[maxRating]
	if !ok1 || !ok2 {
		return false
	}
	return c > m
}

// --- DB helpers ---

// UpsertParentalSettings inserts or updates parental settings. Empty PINHash means keep existing.
func UpsertParentalSettings(d ParentalDB, userID, pinHash string, maxAge int, maxRating string, blockUnrated bool, windowStart, windowEnd string) error {
	_, err := d.Exec(`
		INSERT INTO parental_settings(user_id, enabled, pin_hash, max_age_restriction, max_rating, block_unrated, time_window_start, time_window_end, updated_at)
		VALUES (?, 1, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id) DO UPDATE SET
			enabled = 1,
			max_age_restriction = excluded.max_age_restriction,
			max_rating = excluded.max_rating,
			block_unrated = excluded.block_unrated,
			time_window_start = excluded.time_window_start,
			time_window_end = excluded.time_window_end,
			updated_at = CURRENT_TIMESTAMP`,
		userID, pinHash, maxAge, maxRating, blockUnrated, windowStart, windowEnd,
	)
	return err
}

// GetParentalSettings fetches parental settings for a user.
func GetParentalSettings(d ParentalDB, userID string) (*ParentalSettings, error) {
	row := d.QueryRow(`
		SELECT user_id, enabled, pin_hash, max_age_restriction, max_rating, block_unrated, time_window_start, time_window_end, updated_at
		FROM parental_settings WHERE user_id = ?`, userID)
	var s ParentalSettings
	var maxRating sql.NullString
	var windowStart, windowEnd sql.NullString
	err := row.Scan(&s.UserID, &s.Enabled, &s.PINHash, &s.MaxAgeRestriction, &maxRating, &s.BlockUnrated, &windowStart, &windowEnd, &s.UpdatedAt)
	if err != nil {
		// No rows means disabled
		if err == sql.ErrNoRows {
			return &ParentalSettings{UserID: userID, Enabled: false}, nil
		}
		return nil, err
	}
	s.MaxRating = maxRating.String
	s.TimeWindowStart = windowStart.String
	s.TimeWindowEnd = windowEnd.String
	return &s, nil
}

// DeleteParentalSettings removes parental settings for a user.
func DeleteParentalSettings(d ParentalDB, userID string) error {
	_, err := d.Exec("DELETE FROM parental_settings WHERE user_id = ?", userID)
	return err
}

// Constant-time string comparison to avoid timing attacks on PIN checks.
func safeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
