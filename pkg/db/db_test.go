package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDB_Migrate(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()

	err = db.Migrate()
	require.NoError(t, err)

	// Verify tables exist
	tables := []string{"users", "libraries", "items", "streams", "transcode_jobs", "sessions", "collections", "collection_items", "activity_log"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		require.NoError(t, err, "table %s should exist", table)
		assert.Equal(t, table, name)
	}
}

func TestDB_UserCRUD(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Migrate())

	// Create
	err = db.CreateUser("u1", "alice", "hash123", "admin")
	require.NoError(t, err)

	// Read
	id, hash, role, err := db.GetUserByUsername("alice")
	require.NoError(t, err)
	assert.Equal(t, "u1", id)
	assert.Equal(t, "hash123", hash)
	assert.Equal(t, "admin", role)

	// Update role
	err = db.UpdateUserRole("u1", "user")
	require.NoError(t, err)
	_, _, role, err = db.GetUserByUsername("alice")
	require.NoError(t, err)
	assert.Equal(t, "user", role)

	// List
	users, err := db.ListUsers()
	require.NoError(t, err)
	assert.Len(t, users, 1)

	// Delete
	err = db.DeleteUser("u1")
	require.NoError(t, err)
	users, err = db.ListUsers()
	require.NoError(t, err)
	assert.Len(t, users, 0)
}

func TestDB_CollectionCRUD(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Migrate())

	// Need a user first
	require.NoError(t, db.CreateUser("u1", "alice", "hash", "user"))

	// Create collection
	err = db.CreateCollection("c1", "u1", "Favorites", "collection")
	require.NoError(t, err)

	// List
	cols, err := db.ListCollections("u1")
	require.NoError(t, err)
	assert.Len(t, cols, 1)

	// Get with items (empty)
	col, items, err := db.GetCollectionWithItems("c1")
	require.NoError(t, err)
	assert.Equal(t, "Favorites", col["name"])
	assert.Len(t, items, 0)
}

func TestDB_ActivityLog(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Migrate())

	err = db.LogActivity("u1", "login", "user logged in")
	require.NoError(t, err)

	acts, err := db.ListActivity(10)
	require.NoError(t, err)
	assert.Len(t, acts, 1)
	assert.Equal(t, "login", acts[0]["action"])
}

func TestDB_LibraryCRUD(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Migrate())

	err = db.CreateLibrary("l1", "Movies", "/media/movies", "movie")
	require.NoError(t, err)

	libs, err := db.ListLibraries()
	require.NoError(t, err)
	assert.Len(t, libs, 1)
	assert.Equal(t, "Movies", libs[0]["name"])
}

func TestDB_ItemCRUD(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Migrate())

	require.NoError(t, db.CreateLibrary("l1", "Movies", "/media/movies", "movie"))

	err = db.CreateItem("i1", "l1", "/media/movies/test.mp4", "test", "movie", "mp4", 1000000, 3600, 1920, 1080, "h264", "aac")
	require.NoError(t, err)

	item, err := db.GetItemByID("i1")
	require.NoError(t, err)
	assert.Equal(t, "test", item["name"])

	items, err := db.ListItemsByLibrary("l1")
	require.NoError(t, err)
	assert.Len(t, items, 1)
}
