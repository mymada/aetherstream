package stream

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/devuser/aetherstream/pkg/db"
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
	srv.RegisterRoutes(e, nil)

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

	// Insert item
	err := dbConn.CreateItem("test-2", "lib-1", "/tmp/nonexistent.mp4", "Test", "video", ".mp4", 0, 0, 0, 0, "", "")
	assert.NoError(t, err)

	srv := NewServer(dbConn, "/tmp/media")
	srv.RegisterRoutes(e, nil)

	req := httptest.NewRequest(http.MethodGet, "/videos/test-2/hls/master.m3u8", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Returns waiting playlist (200) because transcode not started
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Waiting for transcode")
}

func TestDirectStreamNotFound(t *testing.T) {
	e := echo.New()
	
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	srv := NewServer(dbConn, "/tmp/media")
	srv.RegisterRoutes(e, nil)

	req := httptest.NewRequest(http.MethodGet, "/videos/nonexistent/stream", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
