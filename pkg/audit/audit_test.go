package audit

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditMigrateAndLog(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	logger := NewLogger(db)
	require.NoError(t, logger.Migrate())

	logger.Log(Event{
		Timestamp:  time.Now(),
		UserID:     "user-1",
		Username:   "alice",
		Action:     "login",
		Resource:   "auth",
		ResourceID: "",
		IP:         "127.0.0.1",
		UserAgent:  "test-agent",
		Details:    "successful login",
	})

	// Give async insert a moment
	time.Sleep(100 * time.Millisecond)

	events, err := logger.Query(10)
	require.NoError(t, err)
	assert.True(t, len(events) >= 1)
	assert.Equal(t, "login", events[0].Action)
	assert.Equal(t, "alice", events[0].Username)
}
