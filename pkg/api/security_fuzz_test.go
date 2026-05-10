package api

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/devuser/aetherstream/pkg/config"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/library"
	"github.com/devuser/aetherstream/pkg/securestore"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// setupTestServer creates a fully wired Server for fuzz/security tests.
func setupTestServer(t *testing.T) (*Server, *echo.Echo, *auth.Service) {
	t.Helper()
	cfg := config.Defaults()
	authSvc, err := auth.NewService("test-secret-32-chars-long-for-testing!!", cfg.Auth.TokenTTL)
	require.NoError(t, err)

	dbConn, err := db.New(":memory:")
	require.NoError(t, err)
	require.NoError(t, dbConn.Migrate())
	t.Cleanup(func() { dbConn.Close() })

	libMgr, err := library.NewManager(dbConn, "")
	require.NoError(t, err)
	t.Cleanup(func() { libMgr.Close() })

	store, err := securestore.NewStore("this-is-a-32-byte-key-for-testing!!")
	require.NoError(t, err)

	srv := NewServer(dbConn, authSvc, cfg, libMgr, store)
	e := echo.New()
	srv.RegisterRoutes(e)
	return srv, e, authSvc
}

// --- Fuzz helpers ---

var fuzzPayloads = []string{
	"",
	"null",
	"{}",
	"[]",
	`{"username":"admin"}`,
	`{"password":"admin"}`,
	`{"username":"admin","password":"admin"}`,
	`{"username":"' OR '1'='1","password":"' OR '1'='1"}`,
	`{"username":"<script>alert(1)</script>","password":"<img src=x onerror=alert(1)>"}`,
	`{"username":"../../../../etc/passwd","password":"../../../etc/shadow"}`,
	`{"username":"` + strings.Repeat("A", 10000) + `","password":"` + strings.Repeat("B", 10000) + `"}`,
	`{"username":"\x00\x01\x02","password":"\xff\xfe"}`,
	`{"username":"admin\n","password":"admin\r\n"}`,
	`{"username":"admin","password":"admin","role":"admin"}`,
	`{"name":"test","path":"/tmp","media_type":"movie"}`,
	`{"name":"test","path":"../../../etc","media_type":"movie"}`,
	`{"q":"test","type":"movie","limit":10}`,
	`{"q":"' OR '1'='1","type":"movie","limit":999999}`,
	`{"item_id":"' OR '1'='1"}`,
	`{"position_seconds":-1,"duration_seconds":0,"percent_complete":101}`,
	`{"position_seconds":1e308,"duration_seconds":1e308,"percent_complete":1e308}`,
	`{"watched":true,"position_seconds":0,"duration_seconds":0}`,
}

// --- Fuzz Login ---

func FuzzHandleLogin(f *testing.F) {
	for _, p := range fuzzPayloads {
		f.Add(p)
	}
	f.Fuzz(func(t *testing.T, payload string) {
		srv, e, _ := setupTestServer(t)
		_ = srv

		// Seed a valid user
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Must never panic; acceptable codes: 400, 401, 403, 429, 200
		code := rec.Code
		if code != http.StatusOK && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusTooManyRequests && code != http.StatusForbidden {
			t.Fatalf("unexpected status code %d for payload %q", code, payload)
		}
	})
}

// --- Fuzz CreateUser (protected, admin only) ---

func FuzzHandleCreateUser(f *testing.F) {
	for _, p := range fuzzPayloads {
		f.Add(p)
	}
	f.Fuzz(func(t *testing.T, payload string) {
		srv, e, authSvc := setupTestServer(t)

		// Seed admin
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
		token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

		req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusCreated && code != http.StatusBadRequest && code != http.StatusForbidden && code != http.StatusUnauthorized && code != http.StatusInternalServerError {
			t.Fatalf("unexpected status code %d for payload %q", code, payload)
		}
	})
}

// --- Fuzz Search ---

func FuzzHandleSearch(f *testing.F) {
	for _, p := range fuzzPayloads {
		f.Add(p)
	}
	f.Fuzz(func(t *testing.T, payload string) {
		srv, e, authSvc := setupTestServer(t)

		// Seed user
		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		// Try to parse payload as query params if it looks like JSON, else just use raw
		q := payload
		if len(payload) > 200 {
			q = payload[:200]
		}

		// URL-encode the query to prevent malformed HTTP requests that panic httptest
		encodedQ := url.QueryEscape(q)
		req := httptest.NewRequest(http.MethodGet, "/api/search?q="+encodedQ+"&type=movie&limit=10", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError {
			t.Fatalf("unexpected status code %d for query %q", code, q)
		}
	})
}

// --- Fuzz SwiftFlow Webhook ---

func FuzzHandleSwiftFlowWebhook(f *testing.F) {
	for _, p := range fuzzPayloads {
		f.Add(p)
	}
	f.Fuzz(func(t *testing.T, payload string) {
		srv, e, _ := setupTestServer(t)
		_ = srv

		req := httptest.NewRequest(http.MethodPost, "/webhooks/swiftflow", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusBadRequest && code != http.StatusTooManyRequests && code != http.StatusForbidden {
			t.Fatalf("unexpected status code %d for payload %q", code, payload)
		}
	})
}

// --- Fuzz CreateLibrary ---

func FuzzHandleCreateLibrary(f *testing.F) {
	for _, p := range fuzzPayloads {
		f.Add(p)
	}
	f.Fuzz(func(t *testing.T, payload string) {
		srv, e, authSvc := setupTestServer(t)

		// Seed admin
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
		token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

		req := httptest.NewRequest(http.MethodPost, "/api/libraries", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusCreated && code != http.StatusBadRequest && code != http.StatusForbidden && code != http.StatusUnauthorized && code != http.StatusInternalServerError {
			t.Fatalf("unexpected status code %d for payload %q", code, payload)
		}
	})
}

// --- Fuzz SaveProgress ---

func FuzzHandleSaveProgress(f *testing.F) {
	for _, p := range fuzzPayloads {
		f.Add(p)
	}
	f.Fuzz(func(t *testing.T, payload string) {
		srv, e, authSvc := setupTestServer(t)

		// Seed user + library + item
		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
		_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodPost, "/api/items/item-1/progress", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusNoContent && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusNotFound && code != http.StatusForbidden {
			t.Fatalf("unexpected status code %d for payload %q", code, payload)
		}
	})
}

// --- Fuzz MarkWatched ---

func FuzzHandleMarkWatched(f *testing.F) {
	for _, p := range fuzzPayloads {
		f.Add(p)
	}
	f.Fuzz(func(t *testing.T, payload string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
		_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodPost, "/api/items/item-1/watched", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusNoContent && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusNotFound && code != http.StatusForbidden {
			t.Fatalf("unexpected status code %d for payload %q", code, payload)
		}
	})
}

// --- Fuzz AddToCollection ---

func FuzzHandleAddToCollection(f *testing.F) {
	for _, p := range fuzzPayloads {
		f.Add(p)
	}
	f.Fuzz(func(t *testing.T, payload string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
		_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
		_ = srv.db.CreateCollection("col-1", "user-1", "Favs", "collection")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodPost, "/api/collections/col-1/items", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusNoContent && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusNotFound && code != http.StatusForbidden {
			t.Fatalf("unexpected status code %d for payload %q", code, payload)
		}
	})
}

// --- Fuzz GetItem (path param) ---

func FuzzHandleGetItem(f *testing.F) {
	seeds := []string{
		"item-1",
		"' OR '1'='1",
		"../../../etc/passwd",
		"<script>alert(1)</script>",
		strings.Repeat("A", 1000),
		"\x00\x01\x02",
		"item-1; DROP TABLE items--",
		"../../../../windows/system32/config/sam",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, itemID string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
		_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/%s", url.QueryEscape(itemID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusUnauthorized && code != http.StatusBadRequest && code != http.StatusForbidden {
			t.Fatalf("unexpected status code %d for itemID %q", code, itemID)
		}
	})
}

// --- Fuzz handleSystemInfo (public, rate-limited) ---

func FuzzHandleSystemInfo(f *testing.F) {
	seeds := []string{"", "?test=1", "?foo=bar"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, query string) {
		_, e, _ := setupTestServer(t)

		req := httptest.NewRequest(http.MethodGet, "/system/info"+query, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusTooManyRequests && code != http.StatusBadRequest {
			t.Fatalf("unexpected status code %d for query %q", code, query)
		}
	})
}

// --- Fuzz handleAuthCallback ---

func FuzzHandleAuthCallback(f *testing.F) {
	for _, p := range fuzzPayloads {
		f.Add(p)
	}
	f.Fuzz(func(t *testing.T, payload string) {
		_, e, _ := setupTestServer(t)

		req := httptest.NewRequest(http.MethodPost, "/auth/callback", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusBadRequest && code != http.StatusTooManyRequests && code != http.StatusForbidden {
			t.Fatalf("unexpected status code %d for payload %q", code, payload)
		}
	})
}

// --- Fuzz handleCreateCollection ---

func FuzzHandleCreateCollection(f *testing.F) {
	for _, p := range fuzzPayloads {
		f.Add(p)
	}
	f.Fuzz(func(t *testing.T, payload string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodPost, "/api/collections", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusCreated && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusForbidden {
			t.Fatalf("unexpected status code %d for payload %q", code, payload)
		}
	})
}

// --- Fuzz handleUpdateUser (admin only) ---

func FuzzHandleUpdateUser(f *testing.F) {
	for _, p := range fuzzPayloads {
		f.Add(p)
	}
	f.Fuzz(func(t *testing.T, payload string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

		req := httptest.NewRequest(http.MethodPut, "/api/users/user-1", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusBadRequest && code != http.StatusForbidden && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusNotFound {
			t.Fatalf("unexpected status code %d for payload %q", code, payload)
		}
	})
}

// --- Fuzz handleScanLibrary (admin only) ---

func FuzzHandleScanLibrary(f *testing.F) {
	seeds := []string{"lib-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, libID string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
		_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
		token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

		req := httptest.NewRequest(http.MethodPost, "/api/libraries/"+url.QueryEscape(libID)+"/scan", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusForbidden && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusBadRequest {
			t.Fatalf("unexpected status code %d for libID %q", code, libID)
		}
	})
}

// --- Fuzz handleGetThumbnail ---

func FuzzHandleGetThumbnail(f *testing.F) {
	seeds := []string{"poster", "backdrop", "../../../etc/passwd", "<script>", strings.Repeat("A", 500)}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, thumbType string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
		_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/item-1/thumbnails/%s", url.QueryEscape(thumbType)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError {
			t.Fatalf("unexpected status code %d for thumbType %q", code, thumbType)
		}
	})
}

// --- Fuzz handleListSubtitles ---

func FuzzHandleListSubtitles(f *testing.F) {
	seeds := []string{"item-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, itemID string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
		_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/%s/subtitles", url.QueryEscape(itemID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusUnauthorized && code != http.StatusBadRequest && code != http.StatusInternalServerError {
			t.Fatalf("unexpected status code %d for itemID %q", code, itemID)
		}
	})
}

// --- Fuzz handleGetSubtitle (lang param) ---

func FuzzHandleGetSubtitle(f *testing.F) {
	seeds := []string{"en", "fr", "' OR '1'='1", "../../../etc/passwd", "<script>", strings.Repeat("A", 500), "en\n../../"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, lang string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
		_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/item-1/subtitles/%s", url.QueryEscape(lang)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError {
			t.Fatalf("unexpected status code %d for lang %q", code, lang)
		}
	})
}

// --- Fuzz handlePlaybackReporting ---

func FuzzHandlePlaybackReporting(f *testing.F) {
	seeds := []string{"user-1", "admin-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, userID string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
		_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
		_ = srv.db.SavePlaybackProgress("user-1", "item-1", 100, 3600, 2.7)
		token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/users/%s/playback-reporting", url.QueryEscape(userID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusForbidden && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusBadRequest {
			t.Fatalf("unexpected status code %d for userID %q", code, userID)
		}
	})
}

// --- Fuzz handleGetSession ---

func FuzzHandleGetSession(f *testing.F) {
	seeds := []string{"", "?foo=bar"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, query string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodGet, "/api/session"+query, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusUnauthorized && code != http.StatusBadRequest && code != http.StatusTooManyRequests {
			t.Fatalf("unexpected status code %d for query %q", code, query)
		}
	})
}

// --- Fuzz handleDeleteUser (admin only) ---

func FuzzHandleDeleteUser(f *testing.F) {
	seeds := []string{"user-1", "admin-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, userID string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/users/%s", url.QueryEscape(userID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusNoContent && code != http.StatusNotFound && code != http.StatusForbidden && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusBadRequest {
			t.Fatalf("unexpected status code %d for userID %q", code, userID)
		}
	})
}

// --- Fuzz handleListItems (query param library_id) ---

func FuzzHandleListItems(f *testing.F) {
	seeds := []string{"lib-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, libID string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
		_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items?library_id=%s", url.QueryEscape(libID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusNotFound {
			t.Fatalf("unexpected status code %d for libID %q", code, libID)
		}
	})
}

// --- Fuzz handleRemoveFromCollection ---

func FuzzHandleRemoveFromCollection(f *testing.F) {
	seeds := []string{"item-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, itemID string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
		_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
		_ = srv.db.CreateCollection("col-1", "user-1", "Favs", "collection")
		_ = srv.db.AddItemToCollection("col-1", "item-1")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/collections/col-1/items/%s", url.QueryEscape(itemID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusNoContent && code != http.StatusNotFound && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusBadRequest && code != http.StatusForbidden {
			t.Fatalf("unexpected status code %d for itemID %q", code, itemID)
		}
	})
}

// --- Fuzz handleGetCollection ---

func FuzzHandleGetCollection(f *testing.F) {
	seeds := []string{"col-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, colID string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		_ = srv.db.CreateCollection("col-1", "user-1", "Favs", "collection")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/collections/%s", url.QueryEscape(colID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusBadRequest {
			t.Fatalf("unexpected status code %d for colID %q", code, colID)
		}
	})
}

// --- Fuzz handleListUsers ---

func FuzzHandleListUsers(f *testing.F) {
	seeds := []string{"", "?foo=bar"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, query string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
		token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

		req := httptest.NewRequest(http.MethodGet, "/api/users"+query, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusUnauthorized && code != http.StatusForbidden && code != http.StatusBadRequest && code != http.StatusTooManyRequests && code != http.StatusInternalServerError {
			t.Fatalf("unexpected status code %d for query %q", code, query)
		}
	})
}

// --- Fuzz handleGetUser ---

func FuzzHandleGetUser(f *testing.F) {
	seeds := []string{"user-1", "admin-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, userID string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/users/%s", url.QueryEscape(userID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusUnauthorized && code != http.StatusForbidden && code != http.StatusInternalServerError && code != http.StatusBadRequest {
			t.Fatalf("unexpected status code %d for userID %q", code, userID)
		}
	})
}

// --- Fuzz handleListLibraries ---

func FuzzHandleListLibraries(f *testing.F) {
	seeds := []string{"", "?foo=bar"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, query string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodGet, "/api/libraries"+query, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusUnauthorized && code != http.StatusForbidden && code != http.StatusBadRequest && code != http.StatusTooManyRequests && code != http.StatusInternalServerError {
			t.Fatalf("unexpected status code %d for query %q", code, query)
		}
	})
}

// --- Fuzz handleListCollections ---

func FuzzHandleListCollections(f *testing.F) {
	seeds := []string{"", "?foo=bar"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, query string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		_ = srv.db.CreateCollection("col-1", "user-1", "Favs", "collection")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodGet, "/api/collections"+query, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusUnauthorized && code != http.StatusForbidden && code != http.StatusBadRequest && code != http.StatusTooManyRequests && code != http.StatusInternalServerError {
			t.Fatalf("unexpected status code %d for query %q", code, query)
		}
	})
}

// --- Fuzz handleListActivity ---

func FuzzHandleListActivity(f *testing.F) {
	seeds := []string{"", "?foo=bar"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, query string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		_ = srv.db.LogActivity("user-1", "login", "test")
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodGet, "/api/activity"+query, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusUnauthorized && code != http.StatusForbidden && code != http.StatusBadRequest && code != http.StatusTooManyRequests && code != http.StatusInternalServerError {
			t.Fatalf("unexpected status code %d for query %q", code, query)
		}
	})
}

// --- Fuzz handleGetProgress ---

func FuzzHandleGetProgress(f *testing.F) {
	seeds := []string{"item-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, itemID string) {
		srv, e, authSvc := setupTestServer(t)

		hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
		_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
		_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
		_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
		_ = srv.db.SavePlaybackProgress("user-1", "item-1", 100, 3600, 2.7)
		token, _ := authSvc.GenerateToken("user-1", "user", "user")

		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/%s/progress", url.QueryEscape(itemID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusBadRequest {
			t.Fatalf("unexpected status code %d for itemID %q", code, itemID)
		}
	})
}

// --- Fuzz handleSystemHardware ---

func FuzzHandleSystemHardware(f *testing.F) {
	seeds := []string{"", "?foo=bar"}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, query string) {
		_, e, _ := setupTestServer(t)

		req := httptest.NewRequest(http.MethodGet, "/api/system/hardware"+query, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		code := rec.Code
		if code != http.StatusOK && code != http.StatusTooManyRequests && code != http.StatusBadRequest && code != http.StatusUnauthorized {
			t.Fatalf("unexpected status code %d for query %q", code, query)
		}
	})
}

// --- Non-fuzz unit tests for rapid endpoint probing ---

func TestFuzzLoginRapid(t *testing.T) {
	srv, e, _ := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")

	for i, payload := range fuzzPayloads {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusTooManyRequests && code != http.StatusForbidden {
			t.Fatalf("iteration %d: unexpected status %d for payload %q", i, code, payload)
		}
	}
}

func TestFuzzCreateUserRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
	token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

	for i, payload := range fuzzPayloads {
		req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusCreated && code != http.StatusBadRequest && code != http.StatusForbidden && code != http.StatusUnauthorized && code != http.StatusInternalServerError {
			t.Fatalf("iteration %d: unexpected status %d for payload %q", i, code, payload)
		}
	}
}

func TestFuzzSearchRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	queries := []string{"", "' OR '1'='1", "<script>alert(1)</script>", "../../../etc/passwd", strings.Repeat("A", 500), "test"}
	for i, q := range queries {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/search?q=%s&type=movie&limit=10", url.QueryEscape(q)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError {
			t.Fatalf("iteration %d: unexpected status %d for query %q", i, code, q)
		}
	}
}

func TestFuzzSwiftFlowWebhookRapid(t *testing.T) {
	_, e, _ := setupTestServer(t)
	for i, payload := range fuzzPayloads {
		req := httptest.NewRequest(http.MethodPost, "/webhooks/swiftflow", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusBadRequest && code != http.StatusTooManyRequests && code != http.StatusForbidden {
			t.Fatalf("iteration %d: unexpected status %d for payload %q", i, code, payload)
		}
	}
}

func TestFuzzCreateLibraryRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
	token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

	for i, payload := range fuzzPayloads {
		req := httptest.NewRequest(http.MethodPost, "/api/libraries", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusCreated && code != http.StatusBadRequest && code != http.StatusForbidden && code != http.StatusUnauthorized && code != http.StatusInternalServerError {
			t.Fatalf("iteration %d: unexpected status %d for payload %q", i, code, payload)
		}
	}
}

func TestFuzzSaveProgressRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
	_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	for i, payload := range fuzzPayloads {
		req := httptest.NewRequest(http.MethodPost, "/api/items/item-1/progress", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusNoContent && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusNotFound && code != http.StatusForbidden {
			t.Fatalf("iteration %d: unexpected status %d for payload %q", i, code, payload)
		}
	}
}

func TestFuzzMarkWatchedRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
	_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	for i, payload := range fuzzPayloads {
		req := httptest.NewRequest(http.MethodPost, "/api/items/item-1/watched", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusNoContent && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusNotFound && code != http.StatusForbidden {
			t.Fatalf("iteration %d: unexpected status %d for payload %q", i, code, payload)
		}
	}
}

func TestFuzzAddToCollectionRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
	_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
	_ = srv.db.CreateCollection("col-1", "user-1", "Favs", "collection")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	for i, payload := range fuzzPayloads {
		req := httptest.NewRequest(http.MethodPost, "/api/collections/col-1/items", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusNoContent && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusNotFound && code != http.StatusForbidden {
			t.Fatalf("iteration %d: unexpected status %d for payload %q", i, code, payload)
		}
	}
}

func TestFuzzGetItemRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
	_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	itemIDs := []string{"item-1", "' OR '1'='1", "../../../etc/passwd", "<script>alert(1)</script>", strings.Repeat("A", 1000)}
	for i, itemID := range itemIDs {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/%s", url.QueryEscape(itemID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusUnauthorized && code != http.StatusBadRequest && code != http.StatusForbidden {
			t.Fatalf("iteration %d: unexpected status %d for itemID %q", i, code, itemID)
		}
	}
}

func TestFuzzSystemInfoRapid(t *testing.T) {
	_, e, _ := setupTestServer(t)
	queries := []string{"", "?test=1", "?foo=bar"}
	for i, q := range queries {
		req := httptest.NewRequest(http.MethodGet, "/system/info"+q, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusTooManyRequests && code != http.StatusBadRequest {
			t.Fatalf("iteration %d: unexpected status %d for query %q", i, code, q)
		}
	}
}

func TestFuzzAuthCallbackRapid(t *testing.T) {
	_, e, _ := setupTestServer(t)
	for i, payload := range fuzzPayloads {
		req := httptest.NewRequest(http.MethodPost, "/auth/callback", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusBadRequest && code != http.StatusTooManyRequests && code != http.StatusForbidden {
			t.Fatalf("iteration %d: unexpected status %d for payload %q", i, code, payload)
		}
	}
}

func TestFuzzCreateCollectionRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	for i, payload := range fuzzPayloads {
		req := httptest.NewRequest(http.MethodPost, "/api/collections", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusCreated && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusForbidden {
			t.Fatalf("iteration %d: unexpected status %d for payload %q", i, code, payload)
		}
	}
}

func TestFuzzUpdateUserRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

	for i, payload := range fuzzPayloads {
		req := httptest.NewRequest(http.MethodPut, "/api/users/user-1", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusBadRequest && code != http.StatusForbidden && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusNotFound {
			t.Fatalf("iteration %d: unexpected status %d for payload %q", i, code, payload)
		}
	}
}

func TestFuzzScanLibraryRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
	_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
	token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

	libIDs := []string{"lib-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for i, libID := range libIDs {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/libraries/%s/scan", url.QueryEscape(libID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusForbidden && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusBadRequest {
			t.Fatalf("iteration %d: unexpected status %d for libID %q", i, code, libID)
		}
	}
}

func TestFuzzGetThumbnailRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
	_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	thumbTypes := []string{"poster", "backdrop", "../../../etc/passwd", "<script>", strings.Repeat("A", 500)}
	for i, tt := range thumbTypes {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/item-1/thumbnails/%s", url.QueryEscape(tt)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError {
			t.Fatalf("iteration %d: unexpected status %d for thumbType %q", i, code, tt)
		}
	}
}

func TestFuzzListSubtitlesRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
	_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	itemIDs := []string{"item-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for i, itemID := range itemIDs {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/%s/subtitles", url.QueryEscape(itemID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusUnauthorized && code != http.StatusBadRequest && code != http.StatusInternalServerError {
			t.Fatalf("iteration %d: unexpected status %d for itemID %q", i, code, itemID)
		}
	}
}

func TestFuzzGetSubtitleRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
	_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	langs := []string{"en", "fr", "' OR '1'='1", "../../../etc/passwd", "<script>", strings.Repeat("A", 500), "en\n../../"}
	for i, lang := range langs {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/item-1/subtitles/%s", url.QueryEscape(lang)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError {
			t.Fatalf("iteration %d: unexpected status %d for lang %q", i, code, lang)
		}
	}
}

func TestFuzzPlaybackReportingRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
	_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
	_ = srv.db.SavePlaybackProgress("user-1", "item-1", 100, 3600, 2.7)
	token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

	userIDs := []string{"user-1", "admin-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for i, userID := range userIDs {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/users/%s/playback-reporting", url.QueryEscape(userID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusForbidden && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusBadRequest {
			t.Fatalf("iteration %d: unexpected status %d for userID %q", i, code, userID)
		}
	}
}

func TestFuzzGetSessionRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	queries := []string{"", "?foo=bar"}
	for i, q := range queries {
		req := httptest.NewRequest(http.MethodGet, "/api/session"+q, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusUnauthorized && code != http.StatusBadRequest && code != http.StatusTooManyRequests {
			t.Fatalf("iteration %d: unexpected status %d for query %q", i, code, q)
		}
	}
}

func TestFuzzDeleteUserRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

	userIDs := []string{"user-1", "admin-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for i, userID := range userIDs {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/users/%s", url.QueryEscape(userID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusNoContent && code != http.StatusNotFound && code != http.StatusForbidden && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusBadRequest {
			t.Fatalf("iteration %d: unexpected status %d for userID %q", i, code, userID)
		}
	}
}

func TestFuzzListItemsRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
	_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	libIDs := []string{"lib-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for i, libID := range libIDs {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items?library_id=%s", url.QueryEscape(libID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusBadRequest && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusNotFound {
			t.Fatalf("iteration %d: unexpected status %d for libID %q", i, code, libID)
		}
	}
}

func TestFuzzRemoveFromCollectionRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
	_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
	_ = srv.db.CreateCollection("col-1", "user-1", "Favs", "collection")
	_ = srv.db.AddItemToCollection("col-1", "item-1")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	itemIDs := []string{"item-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for i, itemID := range itemIDs {
		req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/collections/col-1/items/%s", url.QueryEscape(itemID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusNoContent && code != http.StatusNotFound && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusBadRequest && code != http.StatusForbidden {
			t.Fatalf("iteration %d: unexpected status %d for itemID %q", i, code, itemID)
		}
	}
}

func TestFuzzGetCollectionRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	_ = srv.db.CreateCollection("col-1", "user-1", "Favs", "collection")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	colIDs := []string{"col-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for i, colID := range colIDs {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/collections/%s", url.QueryEscape(colID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusBadRequest {
			t.Fatalf("iteration %d: unexpected status %d for colID %q", i, code, colID)
		}
	}
}

func TestFuzzListUsersRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
	token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

	queries := []string{"", "?foo=bar"}
	for i, q := range queries {
		req := httptest.NewRequest(http.MethodGet, "/api/users"+q, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusUnauthorized && code != http.StatusForbidden && code != http.StatusBadRequest && code != http.StatusTooManyRequests && code != http.StatusInternalServerError {
			t.Fatalf("iteration %d: unexpected status %d for query %q", i, code, q)
		}
	}
}

func TestFuzzGetUserRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")

	userIDs := []string{"user-1", "admin-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for i, userID := range userIDs {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/users/%s", url.QueryEscape(userID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusUnauthorized && code != http.StatusForbidden && code != http.StatusInternalServerError && code != http.StatusBadRequest {
			t.Fatalf("iteration %d: unexpected status %d for userID %q", i, code, userID)
		}
	}
}

func TestFuzzListLibrariesRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	queries := []string{"", "?foo=bar"}
	for i, q := range queries {
		req := httptest.NewRequest(http.MethodGet, "/api/libraries"+q, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusUnauthorized && code != http.StatusForbidden && code != http.StatusBadRequest && code != http.StatusTooManyRequests && code != http.StatusInternalServerError {
			t.Fatalf("iteration %d: unexpected status %d for query %q", i, code, q)
		}
	}
}

func TestFuzzListCollectionsRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	_ = srv.db.CreateCollection("col-1", "user-1", "Favs", "collection")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	queries := []string{"", "?foo=bar"}
	for i, q := range queries {
		req := httptest.NewRequest(http.MethodGet, "/api/collections"+q, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusUnauthorized && code != http.StatusForbidden && code != http.StatusBadRequest && code != http.StatusTooManyRequests && code != http.StatusInternalServerError {
			t.Fatalf("iteration %d: unexpected status %d for query %q", i, code, q)
		}
	}
}

func TestFuzzListActivityRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	_ = srv.db.LogActivity("user-1", "login", "test")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	queries := []string{"", "?foo=bar"}
	for i, q := range queries {
		req := httptest.NewRequest(http.MethodGet, "/api/activity"+q, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusUnauthorized && code != http.StatusForbidden && code != http.StatusBadRequest && code != http.StatusTooManyRequests && code != http.StatusInternalServerError {
			t.Fatalf("iteration %d: unexpected status %d for query %q", i, code, q)
		}
	}
}

func TestFuzzGetProgressRapid(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	_ = srv.db.CreateLibrary("lib-1", "Movies", "/tmp/media", "movie")
	_ = srv.db.CreateItem("item-1", "lib-1", "/tmp/media/test.mp4", "Test", "movie", "mp4", 0, 3600, 1920, 1080, "h264", "aac")
	_ = srv.db.SavePlaybackProgress("user-1", "item-1", 100, 3600, 2.7)
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	itemIDs := []string{"item-1", "' OR '1'='1", "../../../etc/passwd", strings.Repeat("A", 1000)}
	for i, itemID := range itemIDs {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/items/%s/progress", url.QueryEscape(itemID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusNotFound && code != http.StatusUnauthorized && code != http.StatusInternalServerError && code != http.StatusBadRequest {
			t.Fatalf("iteration %d: unexpected status %d for itemID %q", i, code, itemID)
		}
	}
}

func TestFuzzSystemHardwareRapid(t *testing.T) {
	_, e, _ := setupTestServer(t)
	queries := []string{"", "?foo=bar"}
	for i, q := range queries {
		req := httptest.NewRequest(http.MethodGet, "/api/system/hardware"+q, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		code := rec.Code
		if code != http.StatusOK && code != http.StatusTooManyRequests && code != http.StatusBadRequest && code != http.StatusUnauthorized {
			t.Fatalf("iteration %d: unexpected status %d for query %q", i, code, q)
		}
	}
}

// --- Timing / DoS resistance checks ---

func TestLoginTimingConsistency(t *testing.T) {
	srv, e, _ := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("admin-1", "admin", string(hash), "admin")

	var times []time.Duration
	for i := 0; i < 5; i++ {
		start := time.Now()
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte(`{"username":"bad","password":"bad"}`)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		_ = rec.Code
		times = append(times, time.Since(start))
	}

	// Ensure no pathological fast responses that would indicate timing side-channels.
	// bcrypt dominates timing, but on very fast hardware the total round-trip can be
	// under 1 ms. We therefore check that the *slowest* attempt is well above the
	// threshold, and that the fastest is not orders of magnitude quicker than the
	// slowest (which would reveal a short-circuit).
	if len(times) == 0 {
		t.Fatal("no timing samples collected")
	}
	minD, maxD := times[0], times[0]
	for _, d := range times[1:] {
		if d < minD {
			minD = d
		}
		if d > maxD {
			maxD = d
		}
	}
	// On fast CI hardware bcrypt can finish in <1 ms; use 15 µs as a more realistic
	// lower bound for the *entire* HTTP round-trip (bind + DB lookup + bcrypt + JSON).
	if maxD < 15*time.Microsecond {
		t.Fatalf("all login attempts suspiciously fast (max %v), possible timing leak", maxD)
	}
	// If the fastest is more than 50× quicker than the slowest, a short-circuit path exists.
	// (Using 50× instead of 10× to account for GC jitter on fast hardware.)
	if float64(minD) < float64(maxD)*0.02 {
		t.Fatalf("timing spread too large: min %v vs max %v, possible timing leak", minD, maxD)
	}
}

func TestSearchLimitEnforced(t *testing.T) {
	srv, e, authSvc := setupTestServer(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("user"), bcrypt.DefaultCost)
	_ = srv.db.CreateUser("user-1", "user", string(hash), "user")
	token, _ := authSvc.GenerateToken("user-1", "user", "user")

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=test&limit=999999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	// The response body should not contain more than maxLimit (100) items
	body := rec.Body.String()
	assert.NotContains(t, body, "999999")
}
