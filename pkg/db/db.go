package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps sql.DB with AetherStream schema helpers
type DB struct {
	*sql.DB
}

// New opens SQLite with WAL mode
func New(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite single writer
	db.SetMaxIdleConns(1)
	return &DB{db}, nil
}

// Migrate creates tables if not exist
func (d *DB) Migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	username TEXT UNIQUE NOT NULL,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'user',
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS libraries (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	path TEXT NOT NULL,
	media_type TEXT NOT NULL DEFAULT 'mixed',
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS items (
	id TEXT PRIMARY KEY,
	library_id TEXT NOT NULL REFERENCES libraries(id),
	path TEXT NOT NULL,
	name TEXT NOT NULL,
	media_type TEXT NOT NULL,
	container TEXT,
	size_bytes INTEGER,
	duration_seconds REAL,
	width INTEGER,
	height INTEGER,
	video_codec TEXT,
	audio_codec TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS streams (
	id TEXT PRIMARY KEY,
	item_id TEXT NOT NULL REFERENCES items(id),
	user_id TEXT NOT NULL REFERENCES users(id),
	profile TEXT NOT NULL DEFAULT 'auto',
	bandwidth_kbps INTEGER,
	started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	ended_at DATETIME
);

CREATE TABLE IF NOT EXISTS transcode_jobs (
	id TEXT PRIMARY KEY,
	item_id TEXT NOT NULL REFERENCES items(id),
	profile TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending',
	progress REAL DEFAULT 0,
	output_path TEXT,
	started_at DATETIME,
	completed_at DATETIME,
	error TEXT
);

CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id),
	device_id TEXT,
	ip_address TEXT,
	client TEXT,
	bandwidth_kbps INTEGER,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	last_seen DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS collections (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id),
	name TEXT NOT NULL,
	collection_type TEXT NOT NULL DEFAULT 'collection',
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS collection_items (
	collection_id TEXT NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
	item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
	added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (collection_id, item_id)
);

CREATE TABLE IF NOT EXISTS activity_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id TEXT,
	action TEXT NOT NULL,
	details TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_items_library ON items(library_id);
CREATE INDEX IF NOT EXISTS idx_streams_user ON streams(user_id);
CREATE INDEX IF NOT EXISTS idx_transcode_status ON transcode_jobs(status);
CREATE INDEX IF NOT EXISTS idx_collection_user ON collections(user_id);
CREATE INDEX IF NOT EXISTS idx_activity_user ON activity_log(user_id);
`
	if _, err := d.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	// FTS5 virtual table for full-text search (manually managed index)
	ftsSchema := `
CREATE VIRTUAL TABLE IF NOT EXISTS items_fts USING fts5(
	item_id UNINDEXED,
	title,
	description,
	actors,
	director
);
`
	if _, err := d.Exec(ftsSchema); err != nil {
		// FTS5 may not be available; log but don't fail
		// Create a plain fallback table so search works when FTS5 is unavailable
		fallbackSchema := `
CREATE TABLE IF NOT EXISTS items_fts (
	item_id TEXT PRIMARY KEY,
	title TEXT,
	description TEXT,
	actors TEXT,
	director TEXT
);
`
		if _, err2 := d.Exec(fallbackSchema); err2 != nil {
			return nil
		}
		return nil
	}

	// Also create a plain fallback table so search works when FTS5 is unavailable
	fallbackSchema := `
CREATE TABLE IF NOT EXISTS items_fts (
	item_id TEXT PRIMARY KEY,
	title TEXT,
	description TEXT,
	actors TEXT,
	director TEXT
);
`
	if _, err := d.Exec(fallbackSchema); err != nil {
		return nil
	}

	// Trickplay images
	trickplaySchema := `
CREATE TABLE IF NOT EXISTS trickplay_images (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
	image_path TEXT NOT NULL,
	position_seconds REAL NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_trickplay_item ON trickplay_images(item_id);
`
	if _, err := d.Exec(trickplaySchema); err != nil {
		return fmt.Errorf("migrate trickplay: %w", err)
	}

	// Tags
	tagsSchema := `
CREATE TABLE IF NOT EXISTS tags (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT UNIQUE NOT NULL
);
CREATE TABLE IF NOT EXISTS item_tags (
	item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
	tag_id INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (item_id, tag_id)
);
CREATE INDEX IF NOT EXISTS idx_item_tags_tag ON item_tags(tag_id);
`
	if _, err := d.Exec(tagsSchema); err != nil {
		return fmt.Errorf("migrate tags: %w", err)
	}

	// Smart playlists
	smartSchema := `
CREATE TABLE IF NOT EXISTS smart_playlists (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	rules TEXT NOT NULL DEFAULT '[]',
	item_limit INTEGER DEFAULT 100,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_smart_playlists_user ON smart_playlists(user_id);
`
	if _, err := d.Exec(smartSchema); err != nil {
		return fmt.Errorf("migrate smart_playlists: %w", err)
	}

	// Auto collections
	autoColSchema := `
CREATE TABLE IF NOT EXISTS auto_collections (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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
	if _, err := d.Exec(autoColSchema); err != nil {
		return fmt.Errorf("migrate auto_collections: %w", err)
	}

	return nil
}

// --- Users ---

// CreateUser inserts a new user
func (d *DB) CreateUser(id, username, passwordHash, role string) error {
	_, err := d.Exec(
		"INSERT INTO users(id, username, password_hash, role) VALUES (?, ?, ?, ?)",
		id, username, passwordHash, role,
	)
	return err
}

// GetUserByUsername fetches user for auth
func (d *DB) GetUserByUsername(username string) (id, passwordHash, role string, err error) {
	row := d.QueryRow("SELECT id, password_hash, role FROM users WHERE username = ?", username)
	err = row.Scan(&id, &passwordHash, &role)
	return
}

// ListUsers returns all users
func (d *DB) ListUsers() ([]map[string]interface{}, error) {
	rows, err := d.Query("SELECT id, username, role, created_at FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id, username, role string
		var createdAt time.Time
		if err := rows.Scan(&id, &username, &role, &createdAt); err != nil {
			continue
		}
		users = append(users, map[string]interface{}{
			"id":        id,
			"username":  username,
			"role":      role,
			"createdAt": createdAt,
		})
	}
	return users, nil
}

// --- Libraries ---

// CreateLibrary inserts library
func (d *DB) CreateLibrary(id, name, path, mediaType string) error {
	_, err := d.Exec(
		"INSERT INTO libraries(id, name, path, media_type) VALUES (?, ?, ?, ?)",
		id, name, path, mediaType,
	)
	return err
}

// ListLibraries returns all libraries
func (d *DB) ListLibraries() ([]map[string]interface{}, error) {
	rows, err := d.Query("SELECT id, name, path, media_type, created_at FROM libraries")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var libs []map[string]interface{}
	for rows.Next() {
		var id, name, path, mediaType string
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &path, &mediaType, &createdAt); err != nil {
			continue
		}
		libs = append(libs, map[string]interface{}{
			"id":         id,
			"name":       name,
			"path":       path,
			"mediaType":  mediaType,
			"createdAt":  createdAt,
		})
	}
	return libs, nil
}

// --- Items ---

// CreateItem inserts media item
func (d *DB) CreateItem(id, libraryID, path, name, mediaType, container string, sizeBytes int64, duration float64, width, height int, videoCodec, audioCodec string) error {
	_, err := d.Exec(
		`INSERT INTO items(id, library_id, path, name, media_type, container, size_bytes, duration_seconds, width, height, video_codec, audio_codec)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, libraryID, path, name, mediaType, container, sizeBytes, duration, width, height, videoCodec, audioCodec,
	)
	return err
}

// GetItemByID fetches single item
func (d *DB) GetItemByID(id string) (map[string]interface{}, error) {
	row := d.QueryRow(
		`SELECT id, library_id, path, name, media_type, container, size_bytes, duration_seconds, width, height, video_codec, audio_codec, created_at
		 FROM items WHERE id = ?`, id)
	
	var itemID, libID, path, name, mediaType, container, videoCodec, audioCodec string
	var sizeBytes int64
	var duration float64
	var width, height int
	var createdAt time.Time
	
	err := row.Scan(&itemID, &libID, &path, &name, &mediaType, &container, &sizeBytes, &duration, &width, &height, &videoCodec, &audioCodec, &createdAt)
	if err != nil {
		return nil, err
	}
	
	return map[string]interface{}{
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
	}, nil
}

// ListItemsByLibrary returns items in a library
func (d *DB) ListItemsByLibrary(libraryID string) ([]map[string]interface{}, error) {
	rows, err := d.Query(
		`SELECT id, library_id, path, name, media_type, container, size_bytes, duration_seconds, width, height, video_codec, audio_codec, created_at
		 FROM items WHERE library_id = ?`, libraryID)
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
		
		if err := rows.Scan(&itemID, &libID, &path, &name, &mediaType, &container, &sizeBytes, &duration, &width, &height, &videoCodec, &audioCodec, &createdAt); err != nil {
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

// --- Sessions ---

// CreateSession records a new streaming session
func (d *DB) CreateSession(id, userID, deviceID, ip, client string, bandwidthKbps int) error {
	_, err := d.Exec(
		"INSERT INTO sessions(id, user_id, device_id, ip_address, client, bandwidth_kbps) VALUES (?, ?, ?, ?, ?, ?)",
		id, userID, deviceID, ip, client, bandwidthKbps,
	)
	return err
}

// UpdateSessionBandwidth updates QoS bandwidth for session
func (d *DB) UpdateSessionBandwidth(sessionID string, bandwidthKbps int) error {
	_, err := d.Exec(
		"UPDATE sessions SET bandwidth_kbps = ?, last_seen = CURRENT_TIMESTAMP WHERE id = ?",
		bandwidthKbps, sessionID,
	)
	return err
}

// --- Users (extended) ---

// UpdateUserRole changes user role
func (d *DB) UpdateUserRole(id, role string) error {
	_, err := d.Exec("UPDATE users SET role = ? WHERE id = ?", role, id)
	return err
}

// DeleteUser removes a user
func (d *DB) DeleteUser(id string) error {
	_, err := d.Exec("DELETE FROM users WHERE id = ?", id)
	return err
}

// --- Collections ---

// CreateCollection inserts a new collection/playlist
func (d *DB) CreateCollection(id, userID, name, colType string) error {
	_, err := d.Exec(
		"INSERT INTO collections(id, user_id, name, collection_type) VALUES (?, ?, ?, ?)",
		id, userID, name, colType,
	)
	return err
}

// ListCollections returns collections for a user
func (d *DB) ListCollections(userID string) ([]map[string]interface{}, error) {
	rows, err := d.Query("SELECT id, name, collection_type, created_at FROM collections WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []map[string]interface{}
	for rows.Next() {
		var id, name, colType string
		var createdAt time.Time
		if err := rows.Scan(&id, &name, &colType, &createdAt); err != nil {
			continue
		}
		cols = append(cols, map[string]interface{}{
			"id":   id,
			"name": name,
			"type": colType,
			"createdAt": createdAt,
		})
	}
	return cols, nil
}

// GetCollectionWithItems returns collection + items
func (d *DB) GetCollectionWithItems(colID string) (map[string]interface{}, []map[string]interface{}, error) {
	row := d.QueryRow("SELECT id, user_id, name, collection_type, created_at FROM collections WHERE id = ?", colID)
	var id, userID, name, colType string
	var createdAt time.Time
	if err := row.Scan(&id, &userID, &name, &colType, &createdAt); err != nil {
		return nil, nil, err
	}
	col := map[string]interface{}{
		"id":   id,
		"userId": userID,
		"name": name,
		"type": colType,
		"createdAt": createdAt,
	}

	rows, err := d.Query(`
		SELECT i.id, i.name, i.media_type, i.path
		FROM items i
		JOIN collection_items ci ON i.id = ci.item_id
		WHERE ci.collection_id = ?`, colID)
	if err != nil {
		return col, nil, err
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var itemID, itemName, mediaType, path string
		if err := rows.Scan(&itemID, &itemName, &mediaType, &path); err != nil {
			continue
		}
		items = append(items, map[string]interface{}{
			"id":       itemID,
			"name":     itemName,
			"mediaType": mediaType,
			"path":     path,
		})
	}
	return col, items, nil
}

// AddItemToCollection adds item to collection
func (d *DB) AddItemToCollection(colID, itemID string) error {
	_, err := d.Exec(
		"INSERT OR IGNORE INTO collection_items(collection_id, item_id) VALUES (?, ?)",
		colID, itemID,
	)
	return err
}

// RemoveItemFromCollection removes item from collection
func (d *DB) RemoveItemFromCollection(colID, itemID string) error {
	_, err := d.Exec(
		"DELETE FROM collection_items WHERE collection_id = ? AND item_id = ?",
		colID, itemID,
	)
	return err
}

// --- Activity Log ---

// LogActivity records an action
func (d *DB) LogActivity(userID, action, details string) error {
	_, err := d.Exec(
		"INSERT INTO activity_log(user_id, action, details) VALUES (?, ?, ?)",
		userID, action, details,
	)
	return err
}

// ListActivity returns recent activity
func (d *DB) ListActivity(limit int) ([]map[string]interface{}, error) {
	rows, err := d.Query(
		"SELECT id, user_id, action, details, created_at FROM activity_log ORDER BY created_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var acts []map[string]interface{}
	for rows.Next() {
		var id int
		var userID, action, details string
		var createdAt time.Time
		if err := rows.Scan(&id, &userID, &action, &details, &createdAt); err != nil {
			continue
		}
		acts = append(acts, map[string]interface{}{
			"id":        id,
			"userId":    userID,
			"action":    action,
			"details":   details,
			"createdAt": createdAt,
		})
	}
	return acts, nil
}

// --- FTS5 helpers ---

// UpdateFTSIndex updates the FTS5 index for an item with richer metadata fields.
// This should be called after metadata enrichment (e.g. TMDb fetch).
// Uses INSERT OR REPLACE to handle both insert and update.
func (d *DB) UpdateFTSIndex(itemID, title, description, actors, director string) error {
	// First delete any existing entry for this item_id
	_, _ = d.Exec("DELETE FROM items_fts WHERE item_id = ?", itemID)
	// Then insert new entry
	_, err := d.Exec(
		"INSERT INTO items_fts(item_id, title, description, actors, director) VALUES (?, ?, ?, ?, ?)",
		itemID, title, description, actors, director,
	)
	return err
}

// DeleteFTSIndex removes an item from the FTS index.
func (d *DB) DeleteFTSIndex(itemID string) error {
	_, err := d.Exec("DELETE FROM items_fts WHERE item_id = ?", itemID)
	return err
}

// SearchItemsFTS performs full-text search across title, description, actors, director.
// mediaType filters by items.media_type if non-empty. limit caps results (default 20).
func (d *DB) SearchItemsFTS(query, mediaType string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 20
	}
	if query == "" {
		return []map[string]interface{}{}, nil
	}

	var rows *sql.Rows
	var err error
	if mediaType != "" {
		rows, err = d.Query(`
			SELECT i.id, i.library_id, i.path, i.name, i.media_type, i.container,
				i.size_bytes, i.duration_seconds, i.width, i.height,
				i.video_codec, i.audio_codec, i.created_at
			FROM items_fts
			JOIN items i ON i.id = items_fts.item_id
			WHERE items_fts MATCH ? AND i.media_type = ?
			ORDER BY rank
			LIMIT ?`, query, mediaType, limit)
		if err != nil {
			// Fallback: if FTS5 is unavailable, use LIKE search on items_fts fallback table
			rows, err = d.Query(`
				SELECT i.id, i.library_id, i.path, i.name, i.media_type, i.container,
					i.size_bytes, i.duration_seconds, i.width, i.height,
					i.video_codec, i.audio_codec, i.created_at
				FROM items_fts
				JOIN items i ON i.id = items_fts.item_id
				WHERE (items_fts.title LIKE ? OR items_fts.description LIKE ? OR items_fts.actors LIKE ? OR items_fts.director LIKE ?)
					AND i.media_type = ?
				LIMIT ?`, "%"+query+"%", "%"+query+"%", "%"+query+"%", "%"+query+"%", mediaType, limit)
		}
	} else {
		rows, err = d.Query(`
			SELECT i.id, i.library_id, i.path, i.name, i.media_type, i.container,
				i.size_bytes, i.duration_seconds, i.width, i.height,
				i.video_codec, i.audio_codec, i.created_at
			FROM items_fts
			JOIN items i ON i.id = items_fts.item_id
			WHERE items_fts MATCH ?
			ORDER BY rank
			LIMIT ?`, query, limit)
		if err != nil {
			// Fallback: if FTS5 is unavailable, use LIKE search on items_fts fallback table
			rows, err = d.Query(`
				SELECT i.id, i.library_id, i.path, i.name, i.media_type, i.container,
					i.size_bytes, i.duration_seconds, i.width, i.height,
					i.video_codec, i.audio_codec, i.created_at
				FROM items_fts
				JOIN items i ON i.id = items_fts.item_id
				WHERE items_fts.title LIKE ? OR items_fts.description LIKE ? OR items_fts.actors LIKE ? OR items_fts.director LIKE ?
				LIMIT ?`, "%"+query+"%", "%"+query+"%", "%"+query+"%", "%"+query+"%", limit)
		}
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []map[string]interface{}
	for rows.Next() {
		var itemID, libID, path, name, mediaTypeResult, container, videoCodec, audioCodec string
		var sizeBytes int64
		var duration float64
		var width, height int
		var createdAt time.Time
		if err := rows.Scan(&itemID, &libID, &path, &name, &mediaTypeResult, &container,
			&sizeBytes, &duration, &width, &height, &videoCodec, &audioCodec, &createdAt); err != nil {
			continue
		}
		items = append(items, map[string]interface{}{
			"id":             itemID,
			"libraryId":      libID,
			"path":           path,
			"name":           name,
			"mediaType":      mediaTypeResult,
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
