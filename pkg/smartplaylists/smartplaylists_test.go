package smartplaylists

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	sdb := NewDB(db)
	require.NoError(t, sdb.Migrate())
	return sdb
}

func seedItems(t *testing.T, db *sql.DB) {
	schema := `
CREATE TABLE IF NOT EXISTS items (
	id TEXT PRIMARY KEY,
	library_id TEXT NOT NULL,
	path TEXT NOT NULL,
	name TEXT,
	media_type TEXT,
	container TEXT,
	size_bytes INTEGER,
	duration_seconds REAL,
	width INTEGER,
	height INTEGER,
	video_codec TEXT,
	audio_codec TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`
	_, err := db.Exec(schema)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO items(id, library_id, path, name, media_type, container, size_bytes, duration_seconds, width, height, video_codec, audio_codec)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"item-1", "lib-1", "/a/b.mp4", "Big Buck Bunny", "video", "mp4", 1000000, 120.0, 1920, 1080, "h264", "aac")
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO items(id, library_id, path, name, media_type, container, size_bytes, duration_seconds, width, height, video_codec, audio_codec)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"item-2", "lib-1", "/a/c.mkv", "Sintel", "video", "mkv", 2000000, 900.0, 1280, 720, "h264", "aac")
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO items(id, library_id, path, name, media_type, container, size_bytes, duration_seconds, width, height, video_codec, audio_codec)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"item-3", "lib-1", "/a/d.mp3", "Podcast Intro", "audio", "mp3", 5000, 30.0, 0, 0, "", "mp3")
	require.NoError(t, err)
}

func TestNewDB(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	sdb := NewDB(db)
	assert.NotNil(t, sdb)
}

func TestMigrate(t *testing.T) {
	sdb := setupTestDB(t)
	// Verify table exists by inserting a row
	_, err := sdb.Exec("INSERT INTO smart_playlists(id, user_id, name, rules, item_limit) VALUES (?, ?, ?, ?, ?)",
		"sp-1", "user-1", "Test", "[]", 50)
	require.NoError(t, err)
}

func TestCreateSmartPlaylist(t *testing.T) {
	sdb := setupTestDB(t)

	sp, err := sdb.CreateSmartPlaylist("user-1", "My Playlist", []Rule{
		{Field: "name", Operator: "contains", Value: "Bunny"},
	}, 50)
	require.NoError(t, err)
	assert.NotEmpty(t, sp.ID)
	assert.Equal(t, "user-1", sp.UserID)
	assert.Equal(t, "My Playlist", sp.Name)
	assert.Equal(t, 50, sp.Limit)
	assert.Len(t, sp.Rules, 1)

	// Default limit
	sp2, err := sdb.CreateSmartPlaylist("user-1", "Default Limit", nil, 0)
	require.NoError(t, err)
	assert.Equal(t, 100, sp2.Limit)
}

func TestGetSmartPlaylist(t *testing.T) {
	sdb := setupTestDB(t)

	sp, err := sdb.CreateSmartPlaylist("user-1", "Get Test", []Rule{{Field: "media_type", Operator: "eq", Value: "video"}}, 10)
	require.NoError(t, err)

	got, err := sdb.GetSmartPlaylist(sp.ID)
	require.NoError(t, err)
	assert.Equal(t, sp.ID, got.ID)
	assert.Equal(t, "user-1", got.UserID)
	assert.Equal(t, "Get Test", got.Name)
	assert.Equal(t, 10, got.Limit)

	_, err = sdb.GetSmartPlaylist("nonexistent")
	assert.Error(t, err)
}

func TestListSmartPlaylists(t *testing.T) {
	sdb := setupTestDB(t)

	_, err := sdb.CreateSmartPlaylist("user-a", "A1", nil, 10)
	require.NoError(t, err)
	_, err = sdb.CreateSmartPlaylist("user-a", "A2", nil, 20)
	require.NoError(t, err)
	_, err = sdb.CreateSmartPlaylist("user-b", "B1", nil, 30)
	require.NoError(t, err)

	list, err := sdb.ListSmartPlaylists("user-a")
	require.NoError(t, err)
	assert.Len(t, list, 2)

	listB, err := sdb.ListSmartPlaylists("user-b")
	require.NoError(t, err)
	assert.Len(t, listB, 1)
	assert.Equal(t, "B1", listB[0].Name)

	listEmpty, err := sdb.ListSmartPlaylists("no-user")
	require.NoError(t, err)
	assert.Len(t, listEmpty, 0)
}

func TestDeleteSmartPlaylist(t *testing.T) {
	sdb := setupTestDB(t)

	sp, err := sdb.CreateSmartPlaylist("user-1", "ToDelete", nil, 10)
	require.NoError(t, err)

	got, err := sdb.GetSmartPlaylist(sp.ID)
	require.NoError(t, err)
	assert.Equal(t, sp.ID, got.ID)

	err = sdb.DeleteSmartPlaylist(sp.ID)
	require.NoError(t, err)

	_, err = sdb.GetSmartPlaylist(sp.ID)
	assert.Error(t, err)
}

func TestEvaluateSmartPlaylist(t *testing.T) {
	sdb := setupTestDB(t)
	seedItems(t, sdb.DB)

	cases := []struct {
		name     string
		rules    []Rule
		limit    int
		expected int
	}{
		{"no rules", nil, 100, 3},
		{"name eq", []Rule{{Field: "name", Operator: "eq", Value: "Sintel"}}, 100, 1},
		{"name contains", []Rule{{Field: "name", Operator: "contains", Value: "Bunny"}}, 100, 1},
		{"media_type eq video", []Rule{{Field: "media_type", Operator: "eq", Value: "video"}}, 100, 2},
		{"media_type ne audio", []Rule{{Field: "media_type", Operator: "ne", Value: "audio"}}, 100, 2},
		{"container eq mp4", []Rule{{Field: "container", Operator: "eq", Value: "mp4"}}, 100, 1},
		{"duration gt", []Rule{{Field: "duration", Operator: ">", Value: "100"}}, 100, 2},
		{"duration lt", []Rule{{Field: "duration", Operator: "<", Value: "100"}}, 100, 1},
		{"width eq", []Rule{{Field: "width", Operator: "=", Value: "1920"}}, 100, 1},
		{"height eq", []Rule{{Field: "height", Operator: "=", Value: "1080"}}, 100, 1},
		{"size gt", []Rule{{Field: "size", Operator: ">", Value: "1000000"}}, 100, 1},
		{"limit", nil, 2, 2},
		{"unknown field", []Rule{{Field: "unknown", Operator: "eq", Value: "x"}}, 100, 3},
		{"unknown string op", []Rule{{Field: "name", Operator: "unknown", Value: "x"}}, 100, 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sp := &SmartPlaylist{ID: "sp-eval", UserID: "user-1", Name: "Eval", Rules: tc.rules, Limit: tc.limit}
			items, err := sdb.EvaluateSmartPlaylist(sp)
			require.NoError(t, err)
			assert.Len(t, items, tc.expected)
		})
	}
}

func TestEvaluateSmartPlaylist_MultipleRules(t *testing.T) {
	sdb := setupTestDB(t)
	seedItems(t, sdb.DB)

	sp := &SmartPlaylist{
		ID:     "sp-multi",
		UserID: "user-1",
		Name:   "Multi",
		Rules: []Rule{
			{Field: "media_type", Operator: "eq", Value: "video"},
			{Field: "duration", Operator: ">", Value: "500"},
		},
		Limit: 100,
	}
	items, err := sdb.EvaluateSmartPlaylist(sp)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "Sintel", items[0]["name"])
}

func TestBuildRuleClause(t *testing.T) {
	cases := []struct {
		field    string
		op       string
		value    string
		expected string
	}{
		{"name", "eq", "foo", "name = ?"},
		{"name", "ne", "foo", "name != ?"},
		{"name", "contains", "bar", "name LIKE ?"},
		{"media_type", "eq", "video", "media_type = ?"},
		{"container", "eq", "mp4", "container = ?"},
		{"duration", "gt", "120", "duration_seconds GT ?"},
		{"width", "lt", "1920", "width LT ?"},
		{"height", "eq", "1080", "height EQ ?"},
		{"size", ">", "1000", "size_bytes > ?"},
		{"unknown", "eq", "x", ""},
	}

	for _, tc := range cases {
		t.Run(tc.field+"_"+tc.op, func(t *testing.T) {
			clause, vals := buildRuleClause(Rule{Field: tc.field, Operator: tc.op, Value: tc.value})
			assert.Equal(t, tc.expected, clause)
			if tc.expected != "" {
				assert.NotEmpty(t, vals)
			}
		})
	}
}

func TestBuildStringClause(t *testing.T) {
	cases := []struct {
		op       string
		expected string
		val      interface{}
	}{
		{"eq", "name = ?", "foo"},
		{"ne", "name != ?", "foo"},
		{"contains", "name LIKE ?", "%foo%"},
		{"unknown", "", nil},
	}

	for _, tc := range cases {
		t.Run(tc.op, func(t *testing.T) {
			clause, vals := buildStringClause("name", tc.op, "foo")
			assert.Equal(t, tc.expected, clause)
			if tc.val != nil {
				assert.Equal(t, []interface{}{tc.val}, vals)
			} else {
				assert.Nil(t, vals)
			}
		})
	}
}

func TestBuildNumericClause(t *testing.T) {
	clause, vals := buildNumericClause("duration_seconds", "gt", "120")
	assert.Equal(t, "duration_seconds GT ?", clause)
	assert.Equal(t, []interface{}{"120"}, vals)

	clause, vals = buildNumericClause("size_bytes", "<", "1000")
	assert.Equal(t, "size_bytes < ?", clause)
	assert.Equal(t, []interface{}{"1000"}, vals)
}

func TestJSONHelpers(t *testing.T) {
	rules := []Rule{{Field: "name", Operator: "eq", Value: "test"}}
	out, err := jsonMarshal(rules)
	assert.NoError(t, err)
	assert.Contains(t, out, `"name"`)
	assert.Contains(t, out, `"eq"`)
	assert.Contains(t, out, `"test"`)

	// Round-trip: marshal then unmarshal
	parsed := jsonUnmarshalRules(out)
	assert.Len(t, parsed, 1)
	assert.Equal(t, "name", parsed[0].Field)

	// Empty slice
	empty := jsonUnmarshalRules("[]")
	assert.Empty(t, empty)
}
