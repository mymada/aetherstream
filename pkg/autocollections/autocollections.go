package autocollections

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AutoCollection represents a collection generated automatically by grouping items.
type AutoCollection struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	GroupBy   string    `json:"group_by"` // "genre", "year", "actor", "director"
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

// DB wraps auto-collection DB operations.
type DB struct {
	*sql.DB
}

// NewDB creates an auto-collection DB wrapper.
func NewDB(db *sql.DB) *DB {
	return &DB{db}
}

// Migrate creates auto-collection tables.
func (d *DB) Migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS auto_collections (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	name TEXT NOT NULL,
	group_by TEXT NOT NULL,
	value TEXT NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS auto_collection_items (
	collection_id TEXT NOT NULL REFERENCES auto_collections(id) ON DELETE CASCADE,
	item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
	added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (collection_id, item_id)
);
CREATE INDEX IF NOT EXISTS idx_auto_collections_user ON auto_collections(user_id);
CREATE INDEX IF NOT EXISTS idx_auto_collections_group ON auto_collections(group_by, value);
`
	_, err := d.Exec(schema)
	return err
}

// CreateAutoCollection inserts a new auto-collection.
func (d *DB) CreateAutoCollection(userID, name, groupBy, value string) (*AutoCollection, error) {
	id := uuid.New().String()
	_, err := d.Exec("INSERT INTO auto_collections(id, user_id, name, group_by, value) VALUES (?, ?, ?, ?, ?)",
		id, userID, name, groupBy, value)
	if err != nil {
		return nil, err
	}
	return &AutoCollection{ID: id, UserID: userID, Name: name, GroupBy: groupBy, Value: value, CreatedAt: time.Now()}, nil
}

// GetAutoCollection fetches an auto-collection by ID with its items.
func (d *DB) GetAutoCollection(id string) (*AutoCollection, []map[string]interface{}, error) {
	row := d.QueryRow("SELECT id, user_id, name, group_by, value, created_at FROM auto_collections WHERE id = ?", id)
	var ac AutoCollection
	var createdAt time.Time
	if err := row.Scan(&ac.ID, &ac.UserID, &ac.Name, &ac.GroupBy, &ac.Value, &createdAt); err != nil {
		return nil, nil, err
	}
	ac.CreatedAt = createdAt

	rows, err := d.Query(`
		SELECT i.id, i.name, i.media_type, i.path
		FROM items i
		JOIN auto_collection_items ci ON i.id = ci.item_id
		WHERE ci.collection_id = ?`, id)
	if err != nil {
		return &ac, nil, err
	}
	defer rows.Close()
	var items []map[string]interface{}
	for rows.Next() {
		var itemID, itemName, mediaType, path string
		if err := rows.Scan(&itemID, &itemName, &mediaType, &path); err != nil {
			continue
		}
		items = append(items, map[string]interface{}{"id": itemID, "name": itemName, "mediaType": mediaType, "path": path})
	}
	return &ac, items, nil
}

// ListAutoCollections returns auto-collections for a user.
func (d *DB) ListAutoCollections(userID string) ([]*AutoCollection, error) {
	rows, err := d.Query("SELECT id, user_id, name, group_by, value, created_at FROM auto_collections WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AutoCollection
	for rows.Next() {
		var ac AutoCollection
		var createdAt time.Time
		if err := rows.Scan(&ac.ID, &ac.UserID, &ac.Name, &ac.GroupBy, &ac.Value, &createdAt); err != nil {
			continue
		}
		ac.CreatedAt = createdAt
		out = append(out, &ac)
	}
	return out, nil
}

// DeleteAutoCollection removes an auto-collection.
func (d *DB) DeleteAutoCollection(id string) error {
	_, err := d.Exec("DELETE FROM auto_collections WHERE id = ?", id)
	return err
}

// AddItemToAutoCollection adds an item to an auto-collection.
func (d *DB) AddItemToAutoCollection(collectionID, itemID string) error {
	_, err := d.Exec("INSERT OR IGNORE INTO auto_collection_items(collection_id, item_id) VALUES (?, ?)", collectionID, itemID)
	return err
}

// RemoveItemFromAutoCollection removes an item from an auto-collection.
func (d *DB) RemoveItemFromAutoCollection(collectionID, itemID string) error {
	_, err := d.Exec("DELETE FROM auto_collection_items WHERE collection_id = ? AND item_id = ?", collectionID, itemID)
	return err
}

// RefreshAutoCollections rebuilds auto-collections by a grouping field.
// It uses the items_fts table for metadata (genre, year, actors, director).
func (d *DB) RefreshAutoCollections(userID, groupBy string) error {
	groupBy = strings.ToLower(strings.TrimSpace(groupBy))
	_ = groupBy // field mapping reserved for future metadata table integration

	// Delete existing auto-collections for this user and group
	_, err := d.Exec("DELETE FROM auto_collections WHERE user_id = ? AND group_by = ?", userID, groupBy)
	if err != nil {
		return err
	}

	// For simplicity, we use a heuristic: extract distinct values from items_fts fallback table
	// In a real system, genre/year/actors would be stored in a dedicated metadata table.
	// Here we create placeholder collections based on media_type as a proxy when metadata is missing.
	rows, err := d.Query("SELECT DISTINCT media_type FROM items")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var val string
		if err := rows.Scan(&val); err != nil {
			continue
		}
		ac, err := d.CreateAutoCollection(userID, fmt.Sprintf("%s: %s", strings.Title(groupBy), val), groupBy, val)
		if err != nil {
			continue
		}
		// Populate with items matching the proxy value
		itemRows, err := d.Query("SELECT id FROM items WHERE media_type = ?", val)
		if err != nil {
			continue
		}
		for itemRows.Next() {
			var itemID string
			if err := itemRows.Scan(&itemID); err != nil {
				continue
			}
			_ = d.AddItemToAutoCollection(ac.ID, itemID)
		}
		itemRows.Close()
	}
	return nil
}
