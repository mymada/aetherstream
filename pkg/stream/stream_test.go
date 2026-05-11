package stream

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/devuser/aetherstream/pkg/db"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestHandleProbe(t *testing.T) {
	e := echo.New()
	
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	// Insert a test item
	err := dbConn.CreateItem("test-1", "lib-1", "/tmp/nonexistent.mp4", "Test", "video", ".mp4", 0, 0, 0, 0, "", "")
	assert.NoError(t, err)

	srv := NewServer(dbConn, "/tmp/media")
	srv.RegisterRoutes(e, func(next echo.HandlerFunc) echo.HandlerFunc {
		return next
	})

	// Probe endpoint
	req := httptest.NewRequest(http.MethodGet, "/videos/test-1/probe", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Should fail because file doesn't exist, but endpoint is wired
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHLSMasterNotFound(t *testing.T) {
	e := echo.New()

	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	// Use a unique item ID to avoid collision with leftover transcode dirs
	itemID := fmt.Sprintf("hls-test-%d", 999999)

	err := dbConn.CreateItem(itemID, "lib-1", "/tmp/nonexistent.mp4", "Test", "video", ".mp4", 0, 0, 0, 0, "", "")
	assert.NoError(t, err)

	// Use a temp dir as media root to ensure no pre-existing transcode dirs
	tmpRoot := t.TempDir()
	srv := NewServer(dbConn, tmpRoot)
	srv.RegisterRoutes(e, func(next echo.HandlerFunc) echo.HandlerFunc {
		return next
	})

	req := httptest.NewRequest(http.MethodGet, "/videos/"+itemID+"/hls/master.m3u8", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Returns 503 with Retry-After header while transcode is in progress
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestDirectStreamNotFound(t *testing.T) {
	e := echo.New()
	
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	srv := NewServer(dbConn, "/tmp/media")
	srv.RegisterRoutes(e, func(next echo.HandlerFunc) echo.HandlerFunc {
		return next
	})

	req := httptest.NewRequest(http.MethodGet, "/videos/nonexistent/stream", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTranscoderConcurrency(t *testing.T) {
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	tr := NewTranscoder(dbConn, "/tmp/media")

	// Simulate concurrent calls to Transcode for the same item
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = tr.Transcode("item-1", []string{"mobile"}, 0)
		}()
	}
	wg.Wait()

	// Only one job should have been marked running; after completion jobs should be empty
	assert.Equal(t, 0, tr.jobMgr.ActiveCount())
}

func TestTranscoderMultipleItems(t *testing.T) {
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	tr := NewTranscoder(dbConn, "/tmp/media")

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			itemID := fmt.Sprintf("item-%d", idx)
			_ = tr.Transcode(itemID, []string{"mobile"}, 0)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 0, tr.jobMgr.ActiveCount())
}
