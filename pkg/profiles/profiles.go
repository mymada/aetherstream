package profiles

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DB is the database interface expected by profiles package.
type DB interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Query(query string, args ...interface{}) (*sql.Rows, error)
}


// Profile represents a user profile with age restrictions and preferences.
type Profile struct {
	ID                string    `json:"id"`
	UserID            string    `json:"userId"`
	Name              string    `json:"name"`
	Avatar            string    `json:"avatar,omitempty"`
	MaxRating         string    `json:"maxRating,omitempty"`       // e.g., "G", "PG", "PG-13", "R", "NC-17"
	MaxAgeRestriction int       `json:"maxAgeRestriction,omitempty"` // 0 = unrestricted, 7+, 13+, 16+, 18+
	IsDefault         bool      `json:"isDefault"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// Service manages user profiles.
type Service struct {
	db DB
}

// NewService creates a profiles service.
func NewService(database DB) *Service {
	return &Service{db: database}
}

// CreateProfile creates a new profile for a user.
func (s *Service) CreateProfile(userID, name, avatar, maxRating string, maxAgeRestriction int, isDefault bool) (*Profile, error) {
	if name == "" {
		return nil, fmt.Errorf("profile name is required")
	}
	id := uuid.New().String()
	now := time.Now().UTC()
	p := &Profile{
		ID:                id,
		UserID:            userID,
		Name:              name,
		Avatar:            avatar,
		MaxRating:         maxRating,
		MaxAgeRestriction: maxAgeRestriction,
		IsDefault:         isDefault,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := CreateProfile(s.db, id, userID, name, avatar, maxRating, maxAgeRestriction, isDefault); err != nil {
		return nil, fmt.Errorf("create profile: %w", err)
	}
	return p, nil
}

// GetProfile returns a profile by ID.
func (s *Service) GetProfile(profileID string) (*Profile, error) {
	return GetProfileByID(s.db, profileID)
}

// ListProfiles returns all profiles for a user.
func (s *Service) ListProfiles(userID string) ([]Profile, error) {
	return ListProfilesByUser(s.db, userID)
}

// UpdateProfile updates profile fields.
func (s *Service) UpdateProfile(profileID, name, avatar, maxRating string, maxAgeRestriction int, isDefault bool) error {
	return UpdateProfileDB(s.db, profileID, name, avatar, maxRating, maxAgeRestriction, isDefault)
}

// DeleteProfile removes a profile.
func (s *Service) DeleteProfile(profileID string) error {
	return DeleteProfileDB(s.db, profileID)
}

// CanAccessContent checks if a profile is allowed to access content with the given rating/age.
func (s *Service) CanAccessContent(profileID, contentRating string, contentAgeRestriction int) (bool, error) {
	p, err := GetProfileByID(s.db, profileID)
	if err != nil {
		return false, err
	}
	if p.MaxAgeRestriction == 0 {
		return true, nil
	}
	if contentAgeRestriction > 0 && contentAgeRestriction > p.MaxAgeRestriction {
		return false, nil
	}
	// Fallback rating check if age restriction not set on content
	if contentAgeRestriction == 0 && contentRating != "" {
		if exceedsRating(contentRating, p.MaxRating) {
			return false, nil
		}
	}
	return true, nil
}



// CreateProfile inserts a profile row.
func CreateProfile(d DB, id, userID, name, avatar, maxRating string, maxAgeRestriction int, isDefault bool) error {
	_, err := d.Exec(`
		INSERT INTO profiles(id, user_id, name, avatar, max_rating, max_age_restriction, is_default, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		id, userID, name, avatar, maxRating, maxAgeRestriction, isDefault,
	)
	return err
}

// GetProfileByID fetches a profile.
func GetProfileByID(d DB, id string) (*Profile, error) {
	row := d.QueryRow(`
		SELECT id, user_id, name, avatar, max_rating, max_age_restriction, is_default, created_at, updated_at
		FROM profiles WHERE id = ?`, id)
	var p Profile
	var maxRating sql.NullString
	var avatar sql.NullString
	err := row.Scan(&p.ID, &p.UserID, &p.Name, &avatar, &maxRating, &p.MaxAgeRestriction, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.Avatar = avatar.String
	p.MaxRating = maxRating.String
	return &p, nil
}

// ListProfilesByUser returns profiles for a user.
func ListProfilesByUser(d DB, userID string) ([]Profile, error) {
	rows, err := d.Query(`
		SELECT id, user_id, name, avatar, max_rating, max_age_restriction, is_default, created_at, updated_at
		FROM profiles WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var profiles []Profile
	for rows.Next() {
		var p Profile
		var maxRating sql.NullString
		var avatar sql.NullString
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &avatar, &maxRating, &p.MaxAgeRestriction, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		p.Avatar = avatar.String
		p.MaxRating = maxRating.String
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// UpdateProfileDB updates a profile.
func UpdateProfileDB(d DB, id, name, avatar, maxRating string, maxAgeRestriction int, isDefault bool) error {
	_, err := d.Exec(`
		UPDATE profiles SET name = ?, avatar = ?, max_rating = ?, max_age_restriction = ?, is_default = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		name, avatar, maxRating, maxAgeRestriction, isDefault, id,
	)
	return err
}

// DeleteProfileDB removes a profile.
func DeleteProfileDB(d DB, id string) error {
	_, err := d.Exec("DELETE FROM profiles WHERE id = ?", id)
	return err
}
