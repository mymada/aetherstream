package tags

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// DB wraps tag-related DB operations.
type DB struct {
	*sql.DB
}

// NewTagDB creates a tag DB wrapper.
func NewTagDB(db *sql.DB) *DB {
	return &DB{db}
}

// Migrate creates tag tables.
func (d *DB) Migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS tags (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT UNIQUE NOT NULL
);
CREATE TABLE IF NOT EXISTS item_tags (
	item_id TEXT NOT NULL,
	tag_id INTEGER NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (item_id, tag_id)
);
CREATE INDEX IF NOT EXISTS idx_item_tags_tag ON item_tags(tag_id);
`
	_, err := d.Exec(schema)
	return err
}

// AddTag creates a tag if it doesn't exist and returns its ID.
func (d *DB) AddTag(name string) (int64, error) {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return 0, fmt.Errorf("tag name empty")
	}
	var id int64
	err := d.QueryRow("SELECT id FROM tags WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	res, err := d.Exec("INSERT INTO tags(name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// TagItem attaches a tag to an item.
func (d *DB) TagItem(itemID string, tagID int64) error {
	_, err := d.Exec("INSERT OR IGNORE INTO item_tags(item_id, tag_id) VALUES (?, ?)", itemID, tagID)
	return err
}

// UntagItem removes a tag from an item.
func (d *DB) UntagItem(itemID string, tagID int64) error {
	_, err := d.Exec("DELETE FROM item_tags WHERE item_id = ? AND tag_id = ?", itemID, tagID)
	return err
}

// GetItemTags returns tags for an item.
func (d *DB) GetItemTags(itemID string) ([]map[string]interface{}, error) {
	rows, err := d.Query("SELECT t.id, t.name FROM tags t JOIN item_tags it ON t.id = it.tag_id WHERE it.item_id = ?", itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []map[string]interface{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		tags = append(tags, map[string]interface{}{"id": id, "name": name})
	}
	return tags, nil
}

// SearchItemsByTag returns items matching a tag name.
func (d *DB) SearchItemsByTag(tagName string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.Query(`
		SELECT i.id, i.library_id, i.path, i.name, i.media_type, i.container,
			i.size_bytes, i.duration_seconds, i.width, i.height,
			i.video_codec, i.audio_codec, i.created_at
		FROM items i
		JOIN item_tags it ON i.id = it.item_id
		JOIN tags t ON t.id = it.tag_id
		WHERE t.name = ?
		LIMIT ?`, strings.ToLower(strings.TrimSpace(tagName)), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []map[string]interface{}
	for rows.Next() {
		var itemID, libID, path, name, mediaType, container, videoCodec, audioCodec string
		var sizeBytes int64
		var duration float64
		var width, height int
		var createdAt time.Time
		if err := rows.Scan(&itemID, &libID, &path, &name, &mediaType, &container,
			&sizeBytes, &duration, &width, &height, &videoCodec, &audioCodec, &createdAt); err != nil {
			continue
		}
		items = append(items, map[string]interface{}{
			"id":             itemID,
			"libraryId":      libID,
			"path":           path,
			"name":           name,
			"mediaType":      mediaType,
			"container":      container,
			"sizeBytes":      sizeBytes,
			"durationSeconds": duration,
			"width":          width,
			"height":         height,
			"videoCodec":     videoCodec,
			"audioCodec":     audioCodec,
			"createdAt":      createdAt,
		})
	}
	return items, nil
}

// ListAllTags returns all tags.
func (d *DB) ListAllTags() ([]map[string]interface{}, error) {
	rows, err := d.Query("SELECT id, name FROM tags ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []map[string]interface{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		tags = append(tags, map[string]interface{}{"id": id, "name": name})
	}
	return tags, nil
}
