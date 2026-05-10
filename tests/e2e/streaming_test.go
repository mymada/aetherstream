package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/devuser/aetherstream/pkg/api"
	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/devuser/aetherstream/pkg/config"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/library"
	"github.com/devuser/aetherstream/pkg/securestore"
	"github.com/devuser/aetherstream/pkg/stream"
)

// TestEndToEndStreaming verifies the full video streaming pipeline using httptest.Server.
// Steps:
//  1. Start server with in-memory DB and temp media dir
//  2. Create admin user directly in DB
//  3. Login and obtain JWT token
//  4. Create a library
//  5. Scan media (inject a sample MP4 into the library path)
//  6. List items
//  7. Stream the video directly
//  8. Verify returned file is a valid MP4 (ftyp box)
//  9. Test HLS master playlist
// 10. Test DASH manifest
// Cleanup: remove temp DB and media dir, stop server.
func TestEndToEndStreaming(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// --- 1. Setup temp directories and in-memory DB ---
	tmpDir := t.TempDir()
	mediaDir := filepath.Join(tmpDir, "media")
	require.NoError(t, os.MkdirAll(mediaDir, 0755))

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.New(dbPath)
	require.NoError(t, err)
	defer database.Close()

	require.NoError(t, database.Migrate())

	// Config
	cfg := config.Defaults()
	cfg.Server.StaticPath = mediaDir
	cfg.Auth.Secret = "test-secret-32-chars-long-for-testing!!"
	cfg.Auth.TokenTTL = 24

	// Auth service
	authSvc, err := auth.NewService(cfg.Auth.Secret, cfg.Auth.TokenTTL)
	require.NoError(t, err)

	// Secure store
	store, err := securestore.NewStore("this-is-a-32-byte-key-for-testing!!")
	require.NoError(t, err)

	// Library manager
	libMgr, err := library.NewManager(database, "")
	require.NoError(t, err)
	defer libMgr.Close()

	// Echo server
	e := echo.New()
	e.HideBanner = true

	// Register API routes
	apiServer := api.NewServer(database, authSvc, cfg, libMgr, store)
	apiServer.RegisterRoutes(e)

	// Disable CSRF for e2e tests by replacing the middleware with a no-op
	// after routes are registered. Echo middleware runs in order; since
	// we replace the global middleware stack *before* the server starts,
	// we rebuild the middleware chain without CSRF.
	// Alternative: we bypass CSRF by first doing a GET to obtain the cookie,
	// then sending the cookie value in X-CSRF-Token header.
	// Here we use the bypass approach to keep the test realistic.

	// Register stream routes
	streamSrv := stream.NewServer(database, mediaDir)
	streamSrv.RegisterRoutes(e, authSvc.Middleware())
	stream.RegisterAdaptiveRoutes(e, database, mediaDir, authSvc.Middleware())

	// Start httptest server
	ts := httptest.NewServer(e)
	defer ts.Close()
	client := ts.Client()

	// --- 2. Create admin user directly in DB ---
	adminPassword := "adminpass123"
	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	require.NoError(t, err)
	adminID := "admin-001"
	require.NoError(t, database.CreateUser(adminID, "admin", string(hash), "admin"))

	// --- 3. Login and obtain JWT token ---
	// First, GET any endpoint to obtain the CSRF cookie (set by CSRFProtection middleware)
	csrfResp, err := client.Get(ts.URL + "/system/info")
	require.NoError(t, err)
	csrfResp.Body.Close()

	var csrfToken string
	for _, c := range csrfResp.Cookies() {
		if c.Name == "csrf_token" {
			csrfToken = c.Value
			break
		}
	}
	require.NotEmpty(t, csrfToken, "expected csrf_token cookie after GET /system/info")

	loginBody := map[string]string{
		"username": "admin",
		"password": adminPassword,
	}
	loginJSON, _ := json.Marshal(loginBody)
	loginReq, err := http.NewRequest(http.MethodPost, ts.URL+"/auth/login", bytes.NewReader(loginJSON))
	require.NoError(t, err)
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set("X-CSRF-Token", csrfToken)
	loginReq.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})

	resp, err := client.Do(loginReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var loginResp map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&loginResp))
	resp.Body.Close()
	token := loginResp["token"]
	require.NotEmpty(t, token, "expected token in login response")

	// Helper to make authenticated requests (also includes CSRF token/cookie)
	authReq := func(method, url string, body io.Reader) *http.Request {
		req, err := http.NewRequest(method, url, body)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-CSRF-Token", csrfToken)
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrfToken})
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return req
	}

	// --- 4. Create a library ---
	libBody := map[string]string{
		"name":       "Test Movies",
		"path":       mediaDir,
		"media_type": "movie",
	}
	libJSON, _ := json.Marshal(libBody)
	resp, err = client.Do(authReq(http.MethodPost, ts.URL+"/api/libraries", bytes.NewReader(libJSON)))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var libResp map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&libResp))
	resp.Body.Close()
	libID := libResp["id"]
	require.NotEmpty(t, libID)

	// --- 5. Inject a sample MP4 video into the library path ---
	// We create a minimal valid MP4 file (ftyp + moov + mdat boxes)
	videoFile := filepath.Join(mediaDir, "sample.mp4")
	require.NoError(t, createMinimalMP4(videoFile))

	// Manually insert the item into the DB (simulating scan result)
	itemID := "item-001"
	require.NoError(t, database.CreateItem(
		itemID,
		libID,
		videoFile,
		"sample.mp4",
		"movie",
		"mp4",
		0,
		120.0,
		1920,
		1080,
		"h264",
		"aac",
	))

	// Wait a moment for any async scan to settle
	time.Sleep(100 * time.Millisecond)

	// --- 6. List items ---
	resp, err = client.Do(authReq(http.MethodGet, ts.URL+"/api/items?library_id="+libID, nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var itemsResp []map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&itemsResp))
	resp.Body.Close()
	require.GreaterOrEqual(t, len(itemsResp), 1, "expected at least 1 item in library")

	// Find our manually inserted item
	var foundItem map[string]interface{}
	for _, it := range itemsResp {
		if it["id"] == itemID {
			foundItem = it
			break
		}
	}
	require.NotNil(t, foundItem, "expected manually inserted item to be present")
	require.Equal(t, itemID, foundItem["id"])

	// --- 7. Stream the video directly ---
	resp, err = client.Do(authReq(http.MethodGet, ts.URL+"/videos/"+itemID+"/stream", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Greater(t, len(body), 0, "stream body should not be empty")

	// --- 8. Verify the returned file is a valid MP4 (ftyp box signature) ---
	require.True(t, isMP4(body), "streamed file does not have MP4 ftyp signature")

	// --- 9. Test HLS master playlist ---
	resp, err = client.Do(authReq(http.MethodGet, ts.URL+"/videos/"+itemID+"/hls/master.m3u8", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	hlsBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	hlsStr := string(hlsBody)
	assert.True(t, strings.HasPrefix(hlsStr, "#EXTM3U"), "HLS playlist should start with #EXTM3U")
	assert.Contains(t, hlsStr, "#EXT-X-STREAM-INF", "HLS master playlist should contain stream info")

	// --- 10. Test DASH manifest ---
	resp, err = client.Do(authReq(http.MethodGet, ts.URL+"/videos/"+itemID+"/dash/manifest.mpd", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	dashBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	dashStr := string(dashBody)
	assert.True(t, strings.HasPrefix(dashStr, "<?xml"), "DASH manifest should start with XML declaration")
	assert.Contains(t, dashStr, "<MPD", "DASH manifest should contain MPD element")
	assert.Contains(t, dashStr, "</MPD>", "DASH manifest should close MPD element")

	// --- Cleanup ---
	// t.TempDir() and defer database.Close() / ts.Close() handle cleanup automatically.
	// Explicitly remove DB file to be thorough.
	_ = os.Remove(dbPath)
}

// createMinimalMP4 writes a tiny but structurally valid MP4 file.
// It contains ftyp, moov (with mvhd), and an empty mdat box.
func createMinimalMP4(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// ftyp box: "isom" major brand, minor version 0x200, compatible brands "isom" "mp41"
	ftyp := make([]byte, 24)
	ftyp[0] = 0x00
	ftyp[1] = 0x00
	ftyp[2] = 0x00
	ftyp[3] = 0x18 // size = 24
	copy(ftyp[4:8], []byte("ftyp"))
	copy(ftyp[8:12], []byte("isom"))
	ftyp[12] = 0x00
	ftyp[13] = 0x00
	ftyp[14] = 0x00
	ftyp[15] = 0x00 // minor version
	copy(ftyp[16:20], []byte("isom"))
	copy(ftyp[20:24], []byte("mp41"))

	// moov box with mvhd (movie header) — minimal 108 bytes total
	moovSize := 108
	moov := make([]byte, moovSize)
	moov[0] = byte(moovSize >> 24)
	moov[1] = byte(moovSize >> 16)
	moov[2] = byte(moovSize >> 8)
	moov[3] = byte(moovSize)
	copy(moov[4:8], []byte("moov"))

	// mvhd box inside moov (size 100)
	mvhdSize := 100
	mvhd := moov[8:]
	mvhd[0] = byte(mvhdSize >> 24)
	mvhd[1] = byte(mvhdSize >> 16)
	mvhd[2] = byte(mvhdSize >> 8)
	mvhd[3] = byte(mvhdSize)
	copy(mvhd[4:8], []byte("mvhd"))
	mvhd[8] = 0x00 // version 0
	mvhd[9] = 0x00
	mvhd[10] = 0x00
	mvhd[11] = 0x00 // flags
	// creation_time (4 bytes)
	mvhd[12] = 0x00
	mvhd[13] = 0x00
	mvhd[14] = 0x00
	mvhd[15] = 0x00
	// modification_time (4 bytes)
	mvhd[16] = 0x00
	mvhd[17] = 0x00
	mvhd[18] = 0x00
	mvhd[19] = 0x00
	// timescale (4 bytes) = 1000
	mvhd[20] = 0x00
	mvhd[21] = 0x00
	mvhd[22] = 0x03
	mvhd[23] = 0xE8
	// duration (4 bytes) = 2000
	mvhd[24] = 0x00
	mvhd[25] = 0x00
	mvhd[26] = 0x07
	mvhd[27] = 0xD0
	// rate (4 bytes) = 1.0
	mvhd[28] = 0x00
	mvhd[29] = 0x01
	mvhd[30] = 0x00
	mvhd[31] = 0x00
	// volume (2 bytes) = 1.0
	mvhd[32] = 0x00
	mvhd[33] = 0x01
	// reserved (10 bytes) = 0
	// matrix (36 bytes) = identity
	for i := 44; i < 80; i++ {
		mvhd[i] = 0x00
	}
	mvhd[44] = 0x00
	mvhd[45] = 0x01
	mvhd[46] = 0x00
	mvhd[47] = 0x00
	mvhd[60] = 0x00
	mvhd[61] = 0x01
	mvhd[62] = 0x00
	mvhd[63] = 0x00
	mvhd[76] = 0x40
	mvhd[77] = 0x00
	mvhd[78] = 0x00
	mvhd[79] = 0x00
	// pre_defined (24 bytes) = 0
	// next_track_id (4 bytes) = 1
	mvhd[96] = 0x00
	mvhd[97] = 0x00
	mvhd[98] = 0x00
	mvhd[99] = 0x01

	// mdat box (empty payload, just header)
	mdat := make([]byte, 8)
	mdat[0] = 0x00
	mdat[1] = 0x00
	mdat[2] = 0x00
	mdat[3] = 0x08 // size = 8
	copy(mdat[4:8], []byte("mdat"))

	if _, err := f.Write(ftyp); err != nil {
		return err
	}
	if _, err := f.Write(moov); err != nil {
		return err
	}
	if _, err := f.Write(mdat); err != nil {
		return err
	}
	return f.Sync()
}

// isMP4 checks if the byte slice starts with a valid ftyp box signature.
func isMP4(data []byte) bool {
	if len(data) < 12 {
		return false
	}
	// First 4 bytes: box size (big-endian uint32)
	size := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
	if size < 8 || int(size) > len(data) {
		return false
	}
	// Bytes 4-8: box type must be "ftyp"
	if string(data[4:8]) != "ftyp" {
		return false
	}
	// Major brand (bytes 8-12) should be a known brand
	brand := string(data[8:12])
	knownBrands := map[string]bool{
		"isom": true, "mp41": true, "mp42": true, "avc1": true,
		"M4V ": true, "M4A ": true, "M4P ": true, "M4B ": true,
		"qt  ": true, "3gp4": true, "3gp5": true, "3g2a": true,
	}
	return knownBrands[brand]
}
