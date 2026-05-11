package stream

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/devuser/aetherstream/pkg/db"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleListSubtitles(t *testing.T) {
	e := echo.New()

	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	tmpDir := t.TempDir()
	mediaPath := filepath.Join(tmpDir, "movie.mp4")
	require.NoError(t, os.WriteFile(mediaPath, []byte("fake"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "movie.en.srt"), []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"), 0644))

	err := dbConn.CreateItem("test-sub", "lib-1", mediaPath, "Test", "video", ".mp4", 0, 0, 0, 0, "", "")
	require.NoError(t, err)

	srv := NewServer(dbConn, tmpDir)
	srv.RegisterRoutes(e, func(next echo.HandlerFunc) echo.HandlerFunc {
		return next
	})

	req := httptest.NewRequest(http.MethodGet, "/videos/test-sub/subtitles", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	// For a fake video file with no internal subtitle tracks, expect empty array
	assert.Equal(t, "[]\n", body)
}

func TestHandleWebVTTNotFound(t *testing.T) {
	e := echo.New()

	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	tmpDir := t.TempDir()
	mediaPath := filepath.Join(tmpDir, "movie.mp4")
	require.NoError(t, os.WriteFile(mediaPath, []byte("fake"), 0644))

	err := dbConn.CreateItem("test-vtt", "lib-1", mediaPath, "Test", "video", ".mp4", 0, 0, 0, 0, "", "")
	require.NoError(t, err)

	srv := NewServer(dbConn, tmpDir)
	srv.RegisterRoutes(e, func(next echo.HandlerFunc) echo.HandlerFunc {
		return next
	})

	req := httptest.NewRequest(http.MethodGet, "/videos/test-vtt/subtitles/ger/vtt", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Fake file has no subtitle tracks → probe fails → 404 (or 400 if index invalid)
	assert.Contains(t, []int{http.StatusNotFound, http.StatusBadRequest}, rec.Code)
}

func TestHandleWebVTTInvalidLang(t *testing.T) {
	e := echo.New()

	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	tmpDir := t.TempDir()
	mediaPath := filepath.Join(tmpDir, "movie.mp4")
	require.NoError(t, os.WriteFile(mediaPath, []byte("fake"), 0644))

	err := dbConn.CreateItem("test-vtt2", "lib-1", mediaPath, "Test", "video", ".mp4", 0, 0, 0, 0, "", "")
	require.NoError(t, err)

	srv := NewServer(dbConn, tmpDir)
	srv.RegisterRoutes(e, func(next echo.HandlerFunc) echo.HandlerFunc {
		return next
	})

	req := httptest.NewRequest(http.MethodGet, "/videos/test-vtt2/subtitles/../vtt", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
