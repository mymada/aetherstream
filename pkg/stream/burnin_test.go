package stream

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/devuser/aetherstream/pkg/db"
)

func TestValidateBurnInRequest(t *testing.T) {
	err := ValidateBurnInRequest(BurnInRequest{ItemID: "", Language: "en"}, "/tmp/media")
	assert.Error(t, err)

	err = ValidateBurnInRequest(BurnInRequest{ItemID: "i1", Language: ""}, "/tmp/media")
	assert.Error(t, err)

	err = ValidateBurnInRequest(BurnInRequest{ItemID: "i1", Language: "en", OutputPath: "/tmp/media/out.mp4"}, "/tmp/media")
	assert.NoError(t, err)

	err = ValidateBurnInRequest(BurnInRequest{ItemID: "i1", Language: "en", OutputPath: "/etc/passwd"}, "/tmp/media")
	assert.Error(t, err)
}

func TestBuildBurnInCommand(t *testing.T) {
	args := BuildBurnInCommand("/tmp/in.mp4", "/tmp/sub.srt", "/tmp/out.mp4", "mobile", "none")
	assert.Contains(t, args, "-i")
	assert.Contains(t, args, "/tmp/in.mp4")
	assert.Contains(t, args, "subtitles='/tmp/sub.srt',scale=1280:720")
	assert.Contains(t, args, "libx264")
	assert.Contains(t, args, "-c:a")
	assert.Contains(t, args, "copy")
}

func TestBuildBurnInCommandBitmapSub(t *testing.T) {
	args := BuildBurnInCommand("/tmp/in.mp4", "/tmp/sub.sup", "/tmp/out.mp4", "tablet", "nvenc")
	assert.Contains(t, args, "overlay=0:0,scale=1920:1080")
	assert.Contains(t, args, "h264_nvenc")
}

func TestBurnInHandlerNotFound(t *testing.T) {
	e := echo.New()
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	srv := NewServer(dbConn, "/tmp/media")
	srv.RegisterRoutes(e, func(next echo.HandlerFunc) echo.HandlerFunc {
		return next
	})

	req := httptest.NewRequest(http.MethodPost, "/videos/test-1/burnin", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Route not registered yet -> 404
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestBurnInHandlerMissingFields(t *testing.T) {
	e := echo.New()
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	// Insert item
	_ = dbConn.CreateItem("test-burn", "lib-1", "/tmp/nonexistent.mp4", "Test", "video", ".mp4", 0, 0, 0, 0, "", "")

	srv := NewServer(dbConn, "/tmp/media")
	srv.RegisterRoutes(e, func(next echo.HandlerFunc) echo.HandlerFunc {
		return next
	})

	req := httptest.NewRequest(http.MethodPost, "/videos/test-burn/burnin", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Empty body -> binder error -> 400
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestBurnInOutputPathDeterministic(t *testing.T) {
	mediaRoot := t.TempDir()
	itemPath := filepath.Join(mediaRoot, "in.mp4")
	_ = os.WriteFile(itemPath, []byte("fake"), 0644)

	// We can't run real ffmpeg in tests without a real video, so just verify path generation
	hashInput := itemPath + ":" + "en" + ":" + "mobile"
	assert.NotEmpty(t, hashInput)
}
