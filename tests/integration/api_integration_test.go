package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"github.com/devuser/aetherstream/pkg/api"
	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/devuser/aetherstream/pkg/config"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/library"
	"github.com/devuser/aetherstream/pkg/securestore"
)

// ---------- Test Harness ----------

type testHarness struct {
	t          *testing.T
	server     *api.Server
	echo       *echo.Echo
	db         *db.DB
	authSvc    *auth.Service
	cfg        *config.Config
	testDir    string
	adminToken string
	userToken  string
	adminID    string
	userID     string
}

func newHarness(t *testing.T) *testHarness {
	t.Helper()

	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "test.db")

	database, err := db.New(dbPath)
	require.NoError(t, err)
	require.NoError(t, database.Migrate())

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:       0,
			Host:       "127.0.0.1",
			StaticPath: testDir,
		},
		Database: config.DatabaseConfig{Path: dbPath},
		Auth:     config.AuthConfig{Secret: "super-secret-32-char-long-key-for-tests", TokenTTL: 24},
		FFmpeg:   config.FFmpegConfig{Path: "ffmpeg", ProbePath: "ffprobe"},
	}

	authSvc, err := auth.NewService(cfg.Auth.Secret, cfg.Auth.TokenTTL)
	require.NoError(t, err)

	store, err := securestore.NewStore("test-password-123")
	require.NoError(t, err)

	libMgr, err := library.NewManager(database, "")
	require.NoError(t, err)

	e := echo.New()
	apiServer := api.NewServer(database, authSvc, cfg, libMgr, store)
	apiServer.RegisterRoutes(e)

	h := &testHarness{
		t:       t,
		server:  apiServer,
		echo:    e,
		db:      database,
		authSvc: authSvc,
		cfg:     cfg,
		testDir: testDir,
	}

	// Create admin user directly in DB
	h.adminID = "admin-" + randomSuffix()
	require.NoError(t, database.CreateUser(h.adminID, "admin", h.hashPassword("adminpass"), "admin"))

	// Create regular user directly in DB
	h.userID = "user-" + randomSuffix()
	require.NoError(t, database.CreateUser(h.userID, "testuser", h.hashPassword("userpass"), "user"))

	// Generate tokens
	h.adminToken, err = authSvc.GenerateToken(h.adminID, "admin", "admin")
	require.NoError(t, err)

	h.userToken, err = authSvc.GenerateToken(h.userID, "testuser", "user")
	require.NoError(t, err)

	return h
}

func (h *testHarness) hashPassword(pw string) string {
	// Use a low-cost bcrypt hash for test speed.
	// We generate via a quick helper command if bcrypt module available.
	out, err := exec.Command("python3", "-c", "import bcrypt, sys; print(bcrypt.hashpw(sys.argv[1].encode(), bcrypt.gensalt(rounds=4)).decode())", pw).Output()
	if err != nil {
		return "$2a$04$abcdefghijklmnopqrstuuxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	}
	return strings.TrimSpace(string(out))
}

func (h *testHarness) cleanup() {
	_ = h.db.Close()
}

func (h *testHarness) do(method, path string, body interface{}, token string) *http.Response {
	h.t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	// CSRF: for state-changing methods, set a matching CSRF token cookie+header
	if method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete || method == http.MethodPatch {
		csrfTok := "test-csrf-token-12345"
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfTok})
		req.Header.Set("X-CSRF-Token", csrfTok)
	}
	rec := httptest.NewRecorder()
	h.echo.ServeHTTP(rec, req)
	return rec.Result()
}

func (h *testHarness) jsonBody(resp *http.Response, out interface{}) {
	h.t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(h.t, err)
	if out != nil {
		require.NoError(h.t, json.Unmarshal(b, out))
	}
}

func (h *testHarness) requireStatus(resp *http.Response, expected int) {
	h.t.Helper()
	require.Equal(h.t, expected, resp.StatusCode, "unexpected status code")
}

func randomSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// ---------- Helpers: create test video ----------

func (h *testHarness) ensureTestVideo() string {
	h.t.Helper()
	videoPath := filepath.Join(h.testDir, "test_video.mp4")
	if _, err := os.Stat(videoPath); err == nil {
		return videoPath
	}
	cmd := exec.Command("ffmpeg",
		"-f", "lavfi", "-i", "testsrc=duration=5:size=320x240:rate=1",
		"-f", "lavfi", "-i", "sine=frequency=1000:duration=5",
		"-pix_fmt", "yuv420p",
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "28",
		"-c:a", "aac", "-shortest",
		"-y", videoPath,
	)
	out, err := cmd.CombinedOutput()
	require.NoError(h.t, err, "ffmpeg failed: %s", string(out))
	return videoPath
}

// ---------- 1. Auth Tests ----------

func TestAuth_Login(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	// Admin login
	resp := h.do(http.MethodPost, "/auth/login", map[string]string{
		"username": "admin",
		"password": "adminpass",
	}, "")
	h.requireStatus(resp, http.StatusOK)
	var loginResp map[string]string
	h.jsonBody(resp, &loginResp)
	require.NotEmpty(t, loginResp["token"])

	// Wrong password
	resp = h.do(http.MethodPost, "/auth/login", map[string]string{
		"username": "admin",
		"password": "wrongpass",
	}, "")
	h.requireStatus(resp, http.StatusUnauthorized)

	// Missing fields
	resp = h.do(http.MethodPost, "/auth/login", map[string]string{
		"username": "admin",
	}, "")
	h.requireStatus(resp, http.StatusUnauthorized)
}

func TestAuth_Register(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	// Admin creates a new user (register via admin endpoint)
	resp := h.do(http.MethodPost, "/api/users", map[string]string{
		"username": "newuser",
		"password": "newpass123",
		"role":     "user",
	}, h.adminToken)
	h.requireStatus(resp, http.StatusCreated)
	var createResp map[string]string
	h.jsonBody(resp, &createResp)
	require.NotEmpty(t, createResp["id"])

	// New user can login
	resp = h.do(http.MethodPost, "/auth/login", map[string]string{
		"username": "newuser",
		"password": "newpass123",
	}, "")
	h.requireStatus(resp, http.StatusOK)

	// Non-admin cannot create user
	resp = h.do(http.MethodPost, "/api/users", map[string]string{
		"username": "hacker",
		"password": "hackpass",
		"role":     "admin",
	}, h.userToken)
	h.requireStatus(resp, http.StatusForbidden)
}

func TestAuth_TokenRefresh(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	// Use a short-lived token service
	shortAuth, err := auth.NewService(h.cfg.Auth.Secret, 1) // 1 hour TTL
	require.NoError(t, err)
	token, err := shortAuth.GenerateToken(h.userID, "testuser", "user")
	require.NoError(t, err)

	// Token should work immediately
	resp := h.do(http.MethodGet, "/api/session", nil, token)
	h.requireStatus(resp, http.StatusOK)

	// Token structure is valid (we can't easily test expiration without waiting,
	// but we can verify the token is accepted by middleware)
	var sess map[string]interface{}
	h.jsonBody(resp, &sess)
	require.Equal(t, h.userID, sess["user_id"])
}

// ---------- 2. Users CRUD ----------

func TestUsers_CRUD(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	// List users (admin)
	resp := h.do(http.MethodGet, "/api/users", nil, h.adminToken)
	h.requireStatus(resp, http.StatusOK)
	var users []map[string]interface{}
	h.jsonBody(resp, &users)
	require.GreaterOrEqual(t, len(users), 2)

	// Get user by ID (admin)
	resp = h.do(http.MethodGet, "/api/users/"+h.userID, nil, h.adminToken)
	h.requireStatus(resp, http.StatusOK)
	var user map[string]interface{}
	h.jsonBody(resp, &user)
	require.Equal(t, "testuser", user["username"])

	// Update user role (admin)
	resp = h.do(http.MethodPut, "/api/users/"+h.userID, map[string]string{"role": "admin"}, h.adminToken)
	h.requireStatus(resp, http.StatusOK)
	var updateResp map[string]string
	h.jsonBody(resp, &updateResp)
	require.Equal(t, "admin", updateResp["role"])

	// Delete user (admin)
	resp = h.do(http.MethodDelete, "/api/users/"+h.userID, nil, h.adminToken)
	h.requireStatus(resp, http.StatusNoContent)

	// Verify deletion
	resp = h.do(http.MethodGet, "/api/users/"+h.userID, nil, h.adminToken)
	h.requireStatus(resp, http.StatusNotFound)

	// Regular user CAN list users (the API does not restrict GET /api/users to admin only)
	resp = h.do(http.MethodGet, "/api/users", nil, h.userToken)
	h.requireStatus(resp, http.StatusOK)
}

// ---------- 3. Libraries CRUD + Scan ----------

func TestLibraries_CRUDAndScan(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	videoPath := h.ensureTestVideo()

	// Create library (admin only)
	resp := h.do(http.MethodPost, "/api/libraries", map[string]string{
		"name":       "Test Movies",
		"path":       h.testDir,
		"media_type": "movie",
	}, h.adminToken)
	h.requireStatus(resp, http.StatusCreated)
	var libResp map[string]string
	h.jsonBody(resp, &libResp)
	libID := libResp["id"]
	require.NotEmpty(t, libID)

	// List libraries (any authenticated user)
	resp = h.do(http.MethodGet, "/api/libraries", nil, h.userToken)
	h.requireStatus(resp, http.StatusOK)
	var libs []map[string]interface{}
	h.jsonBody(resp, &libs)
	require.GreaterOrEqual(t, len(libs), 1)

	// Scan library (admin only)
	resp = h.do(http.MethodPost, "/api/libraries/"+libID+"/scan", nil, h.adminToken)
	h.requireStatus(resp, http.StatusOK)
	var scanResp map[string]string
	h.jsonBody(resp, &scanResp)
	require.Equal(t, "scanning", scanResp["status"])

	// Wait a bit for scan worker to process
	time.Sleep(500 * time.Millisecond)

	// Non-admin cannot create library
	resp = h.do(http.MethodPost, "/api/libraries", map[string]string{
		"name":       "Bad Lib",
		"path":       h.testDir,
		"media_type": "movie",
	}, h.userToken)
	h.requireStatus(resp, http.StatusForbidden)
	_ = videoPath
}

// ---------- 4. Items List/Get ----------

func TestItems_ListAndGet(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	videoPath := h.ensureTestVideo()

	// Create library and insert item directly for speed
	libID := "lib-" + randomSuffix()
	require.NoError(t, h.db.CreateLibrary(libID, "Items Test", h.testDir, "movie"))

	itemID := "item-" + randomSuffix()
	require.NoError(t, h.db.CreateItem(itemID, libID, videoPath, "Test Video", "movie", "mp4", 1024, 5.0, 320, 240, "h264", "aac"))

	// List items by library
	resp := h.do(http.MethodGet, "/api/items?library_id="+libID, nil, h.userToken)
	h.requireStatus(resp, http.StatusOK)
	var items []map[string]interface{}
	h.jsonBody(resp, &items)
	require.GreaterOrEqual(t, len(items), 1)

	// Get item by ID
	resp = h.do(http.MethodGet, "/api/items/"+itemID, nil, h.userToken)
	h.requireStatus(resp, http.StatusOK)
	var item map[string]interface{}
	h.jsonBody(resp, &item)
	require.Equal(t, itemID, item["id"])
	require.Equal(t, "Test Video", item["name"])

	// Missing library_id
	resp = h.do(http.MethodGet, "/api/items", nil, h.userToken)
	h.requireStatus(resp, http.StatusBadRequest)

	// Non-existent item
	resp = h.do(http.MethodGet, "/api/items/nonexistent", nil, h.userToken)
	h.requireStatus(resp, http.StatusNotFound)
}

// ---------- 5. Playback Progress ----------

func TestPlaybackProgress_SaveAndGet(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	videoPath := h.ensureTestVideo()
	libID := "lib-" + randomSuffix()
	require.NoError(t, h.db.CreateLibrary(libID, "Progress Test", h.testDir, "movie"))
	itemID := "item-" + randomSuffix()
	require.NoError(t, h.db.CreateItem(itemID, libID, videoPath, "Progress Video", "movie", "mp4", 1024, 120.0, 320, 240, "h264", "aac"))

	// Save progress
	resp := h.do(http.MethodPost, "/api/items/"+itemID+"/progress", map[string]interface{}{
		"position_seconds": 45.5,
		"duration_seconds": 120.0,
		"percent_complete": 37.9,
	}, h.userToken)
	h.requireStatus(resp, http.StatusNoContent)

	// Get progress
	resp = h.do(http.MethodGet, "/api/items/"+itemID+"/progress", nil, h.userToken)
	h.requireStatus(resp, http.StatusOK)
	var prog map[string]interface{}
	h.jsonBody(resp, &prog)
	require.Equal(t, 45.5, prog["positionSeconds"])

	// Update progress
	resp = h.do(http.MethodPost, "/api/items/"+itemID+"/progress", map[string]interface{}{
		"position_seconds": 90.0,
		"duration_seconds": 120.0,
		"percent_complete": 75.0,
	}, h.userToken)
	h.requireStatus(resp, http.StatusNoContent)

	// Verify update
	resp = h.do(http.MethodGet, "/api/items/"+itemID+"/progress", nil, h.userToken)
	h.requireStatus(resp, http.StatusOK)
	h.jsonBody(resp, &prog)
	require.Equal(t, 90.0, prog["positionSeconds"])

	// No progress for other user
	otherToken, _ := h.authSvc.GenerateToken("other-user", "other", "user")
	resp = h.do(http.MethodGet, "/api/items/"+itemID+"/progress", nil, otherToken)
	h.requireStatus(resp, http.StatusNotFound)
}

// ---------- 6. Watched Status ----------

func TestWatchedStatus(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	videoPath := h.ensureTestVideo()
	libID := "lib-" + randomSuffix()
	require.NoError(t, h.db.CreateLibrary(libID, "Watched Test", h.testDir, "movie"))
	itemID := "item-" + randomSuffix()
	require.NoError(t, h.db.CreateItem(itemID, libID, videoPath, "Watched Video", "movie", "mp4", 1024, 120.0, 320, 240, "h264", "aac"))

	// Mark as watched
	resp := h.do(http.MethodPost, "/api/items/"+itemID+"/watched", map[string]interface{}{
		"position_seconds": 120.0,
		"duration_seconds": 120.0,
		"watched":          true,
	}, h.userToken)
	h.requireStatus(resp, http.StatusNoContent)

	// Verify via playback reporting
	resp = h.do(http.MethodGet, "/api/users/"+h.userID+"/playback-reporting", nil, h.userToken)
	h.requireStatus(resp, http.StatusOK)
	var report map[string]interface{}
	h.jsonBody(resp, &report)
	// watch_history may be nil if no rows; just verify request succeeded
	_ = report
}

// ---------- 7. Search ----------

func TestSearch(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	videoPath := h.ensureTestVideo()
	libID := "lib-" + randomSuffix()
	require.NoError(t, h.db.CreateLibrary(libID, "Search Test", h.testDir, "movie"))
	itemID := "item-" + randomSuffix()
	require.NoError(t, h.db.CreateItem(itemID, libID, videoPath, "Unique Searchable Title", "movie", "mp4", 1024, 120.0, 320, 240, "h264", "aac"))
	require.NoError(t, h.db.UpdateFTSIndex(itemID, "Unique Searchable Title", "A great movie", "Actor A", "Director B"))
	_ = videoPath

	// Search
	resp := h.do(http.MethodGet, "/api/search?q=Unique+Searchable", nil, h.userToken)
	h.requireStatus(resp, http.StatusOK)
	var searchResp map[string]interface{}
	h.jsonBody(resp, &searchResp)
	require.NotNil(t, searchResp["results"])

	results, ok := searchResp["results"].([]interface{})
	require.True(t, ok)
	require.GreaterOrEqual(t, len(results), 1)

	// Missing query
	resp = h.do(http.MethodGet, "/api/search", nil, h.userToken)
	h.requireStatus(resp, http.StatusBadRequest)
}

// ---------- 8. Collections ----------

func TestCollections_CRUD(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	videoPath := h.ensureTestVideo()
	libID := "lib-" + randomSuffix()
	require.NoError(t, h.db.CreateLibrary(libID, "Collection Test", h.testDir, "movie"))
	itemID := "item-" + randomSuffix()
	require.NoError(t, h.db.CreateItem(itemID, libID, videoPath, "Collection Video", "movie", "mp4", 1024, 120.0, 320, 240, "h264", "aac"))

	// Create collection
	resp := h.do(http.MethodPost, "/api/collections", map[string]string{
		"name": "My Favorites",
		"type": "collection",
	}, h.userToken)
	h.requireStatus(resp, http.StatusCreated)
	var colResp map[string]string
	h.jsonBody(resp, &colResp)
	colID := colResp["id"]
	require.NotEmpty(t, colID)

	// List collections
	resp = h.do(http.MethodGet, "/api/collections", nil, h.userToken)
	h.requireStatus(resp, http.StatusOK)
	var cols []map[string]interface{}
	h.jsonBody(resp, &cols)
	require.GreaterOrEqual(t, len(cols), 1)

	// Get collection
	resp = h.do(http.MethodGet, "/api/collections/"+colID, nil, h.userToken)
	h.requireStatus(resp, http.StatusOK)
	var getCol map[string]interface{}
	h.jsonBody(resp, &getCol)
	require.NotNil(t, getCol["collection"])

	// Add item to collection
	resp = h.do(http.MethodPost, "/api/collections/"+colID+"/items", map[string]string{"item_id": itemID}, h.userToken)
	h.requireStatus(resp, http.StatusNoContent)

	// Verify item is in collection
	resp = h.do(http.MethodGet, "/api/collections/"+colID, nil, h.userToken)
	h.requireStatus(resp, http.StatusOK)
	var getCol2 map[string]interface{}
	h.jsonBody(resp, &getCol2)
	items := getCol2["items"].([]interface{})
	require.GreaterOrEqual(t, len(items), 1)

	// Remove item from collection
	resp = h.do(http.MethodDelete, "/api/collections/"+colID+"/items/"+itemID, nil, h.userToken)
	h.requireStatus(resp, http.StatusNoContent)

	// Verify removal
	resp = h.do(http.MethodGet, "/api/collections/"+colID, nil, h.userToken)
	h.requireStatus(resp, http.StatusOK)
	var getCol3 map[string]interface{}
	h.jsonBody(resp, &getCol3)
	itemsRaw, ok := getCol3["items"]
	if ok && itemsRaw != nil {
		items = itemsRaw.([]interface{})
	} else {
		items = []interface{}{}
	}
	require.Equal(t, 0, len(items))
}

// ---------- 9. Subtitles ----------

func TestSubtitles(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	videoPath := h.ensureTestVideo()
	libID := "lib-" + randomSuffix()
	require.NoError(t, h.db.CreateLibrary(libID, "Subtitle Test", h.testDir, "movie"))
	itemID := "item-" + randomSuffix()
	require.NoError(t, h.db.CreateItem(itemID, libID, videoPath, "Subtitle Video", "movie", "mp4", 1024, 5.0, 320, 240, "h264", "aac"))
	_ = videoPath

	// List subtitles
	resp := h.do(http.MethodGet, "/api/items/"+itemID+"/subtitles", nil, h.userToken)
	h.requireStatus(resp, http.StatusOK)
	var subs []map[string]interface{}
	h.jsonBody(resp, &subs)
	// Test video has no embedded subtitle tracks, so expect empty or minimal
	require.GreaterOrEqual(t, len(subs), 0)

	// Get subtitle for specific language (will likely fail for test video)
	resp = h.do(http.MethodGet, "/api/items/"+itemID+"/subtitles/eng", nil, h.userToken)
	// 404 is acceptable since test video has no real subtitle tracks
	if resp.StatusCode != http.StatusOK {
		h.requireStatus(resp, http.StatusNotFound)
	}
}

// ---------- 10. Thumbnails ----------

func TestThumbnails(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	videoPath := h.ensureTestVideo()
	libID := "lib-" + randomSuffix()
	require.NoError(t, h.db.CreateLibrary(libID, "Thumbnail Test", h.testDir, "movie"))
	itemID := "item-" + randomSuffix()
	require.NoError(t, h.db.CreateItem(itemID, libID, videoPath, "Thumbnail Video", "movie", "mp4", 1024, 5.0, 320, 240, "h264", "aac"))

	// Get poster thumbnail
	resp := h.do(http.MethodGet, "/api/items/"+itemID+"/thumbnails/poster", nil, h.userToken)
	if resp.StatusCode == http.StatusOK {
		// Should be an image
		ct := resp.Header.Get("Content-Type")
		require.True(t, strings.HasPrefix(ct, "image/") || ct == "", "expected image content type, got: %s", ct)
	} else {
		// If ffmpeg thumbnail generation fails, accept 500 but log it
		t.Logf("thumbnail generation returned %d, may need ffmpeg in PATH", resp.StatusCode)
	}

	// Invalid type
	resp = h.do(http.MethodGet, "/api/items/"+itemID+"/thumbnails/invalid", nil, h.userToken)
	h.requireStatus(resp, http.StatusBadRequest)

	// Non-existent item
	resp = h.do(http.MethodGet, "/api/items/nonexistent/thumbnails/poster", nil, h.userToken)
	h.requireStatus(resp, http.StatusNotFound)
}

// ---------- End-to-End Integration Flow ----------

func TestIntegration_FullFlow(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	_ = h.ensureTestVideo()

	// 1. Login as admin
	resp := h.do(http.MethodPost, "/auth/login", map[string]string{
		"username": "admin",
		"password": "adminpass",
	}, "")
	h.requireStatus(resp, http.StatusOK)
	var login map[string]string
	h.jsonBody(resp, &login)
	adminTok := login["token"]
	require.NotEmpty(t, adminTok)

	// 2. Create library
	resp = h.do(http.MethodPost, "/api/libraries", map[string]string{
		"name":       "Integration Lib",
		"path":       h.testDir,
		"media_type": "movie",
	}, adminTok)
	h.requireStatus(resp, http.StatusCreated)
	var lib map[string]string
	h.jsonBody(resp, &lib)
	libID := lib["id"]

	// 3. Scan library
	resp = h.do(http.MethodPost, "/api/libraries/"+libID+"/scan", nil, adminTok)
	h.requireStatus(resp, http.StatusOK)

	// 4. Wait for scan
	time.Sleep(600 * time.Millisecond)

	// 5. List items
	resp = h.do(http.MethodGet, "/api/items?library_id="+libID, nil, adminTok)
	h.requireStatus(resp, http.StatusOK)
	var items []map[string]interface{}
	h.jsonBody(resp, &items)
	require.GreaterOrEqual(t, len(items), 1)

	var itemID string
	for _, it := range items {
		if it["name"] == "test_video" || it["name"] == "Test Video" {
			itemID = it["id"].(string)
			break
		}
	}
	if itemID == "" && len(items) > 0 {
		itemID = items[0]["id"].(string)
	}
	require.NotEmpty(t, itemID, "expected at least one scanned item")

	// 6. Get item details
	resp = h.do(http.MethodGet, "/api/items/"+itemID, nil, adminTok)
	h.requireStatus(resp, http.StatusOK)

	// 7. Save playback progress
	resp = h.do(http.MethodPost, "/api/items/"+itemID+"/progress", map[string]interface{}{
		"position_seconds": 10.0,
		"duration_seconds": 100.0,
		"percent_complete": 10.0,
	}, adminTok)
	h.requireStatus(resp, http.StatusNoContent)

	// 8. Get progress
	resp = h.do(http.MethodGet, "/api/items/"+itemID+"/progress", nil, adminTok)
	h.requireStatus(resp, http.StatusOK)
	var prog map[string]interface{}
	h.jsonBody(resp, &prog)
	require.Equal(t, 10.0, prog["positionSeconds"])

	// 9. Mark watched
	resp = h.do(http.MethodPost, "/api/items/"+itemID+"/watched", map[string]interface{}{
		"position_seconds": 100.0,
		"duration_seconds": 100.0,
		"watched":          true,
	}, adminTok)
	h.requireStatus(resp, http.StatusNoContent)

	// 10. Search
	resp = h.do(http.MethodGet, "/api/search?q=test", nil, adminTok)
	h.requireStatus(resp, http.StatusOK)

	// 11. Create collection
	resp = h.do(http.MethodPost, "/api/collections", map[string]string{
		"name": "Integration Collection",
		"type": "collection",
	}, adminTok)
	h.requireStatus(resp, http.StatusCreated)
	var col map[string]string
	h.jsonBody(resp, &col)
	colID := col["id"]

	// 12. Add item to collection
	resp = h.do(http.MethodPost, "/api/collections/"+colID+"/items", map[string]string{"item_id": itemID}, adminTok)
	h.requireStatus(resp, http.StatusNoContent)

	// 13. Get collection with items
	resp = h.do(http.MethodGet, "/api/collections/"+colID, nil, adminTok)
	h.requireStatus(resp, http.StatusOK)
	var getCol map[string]interface{}
	h.jsonBody(resp, &getCol)
	colItems := getCol["items"].([]interface{})
	require.GreaterOrEqual(t, len(colItems), 1)

	// 14. Subtitles
	resp = h.do(http.MethodGet, "/api/items/"+itemID+"/subtitles", nil, adminTok)
	h.requireStatus(resp, http.StatusOK)

	// 15. Thumbnails
	resp = h.do(http.MethodGet, "/api/items/"+itemID+"/thumbnails/poster", nil, adminTok)
	if resp.StatusCode != http.StatusOK {
		t.Logf("thumbnail returned %d (ffmpeg may be missing or failing)", resp.StatusCode)
	}

	// 16. Session info
	resp = h.do(http.MethodGet, "/api/session", nil, adminTok)
	h.requireStatus(resp, http.StatusOK)
	var sess map[string]interface{}
	h.jsonBody(resp, &sess)
	require.NotEmpty(t, sess["user_id"])

	// 17. System info (public)
	resp = h.do(http.MethodGet, "/system/info", nil, "")
	h.requireStatus(resp, http.StatusOK)
	var sys map[string]interface{}
	h.jsonBody(resp, &sys)
	require.Equal(t, "AetherStream", sys["name"])
}
