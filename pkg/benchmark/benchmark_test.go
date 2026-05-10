package benchmark

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/devuser/aetherstream/pkg/api"
	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/devuser/aetherstream/pkg/config"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/library"
	"github.com/devuser/aetherstream/pkg/securestore"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"strings"
)

// BenchmarkSystemInfo measures API response time for /system/info
func BenchmarkSystemInfo(b *testing.B) {
	e := echo.New()
	cfg := config.Defaults()
	authSvc, _ := auth.NewService("test-secret-32-chars-long-for-testing!!", cfg.Auth.TokenTTL)
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	libMgr, _ := library.NewManager(dbConn, "")
	defer libMgr.Close()
	store, _ := securestore.NewStore("this-is-a-32-byte-key-for-testing!!")

	srv := api.NewServer(dbConn, authSvc, cfg, libMgr, store)
	srv.RegisterRoutes(e)

	req := httptest.NewRequest(http.MethodGet, "/system/info", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(b, http.StatusOK, rec.Code)
	}
}

// BenchmarkLogin measures login endpoint performance
func BenchmarkLogin(b *testing.B) {
	e := echo.New()
	cfg := config.Defaults()
	authSvc, _ := auth.NewService("test-secret-32-chars-long-for-testing!!", cfg.Auth.TokenTTL)
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	// Seed user with correct bcrypt hash for "admin"
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	dbConn.CreateUser("admin-1", "admin", string(hash), "admin")

	libMgr, _ := library.NewManager(dbConn, "")
	defer libMgr.Close()
	store, _ := securestore.NewStore("this-is-a-32-byte-key-for-testing!!")

	srv := api.NewServer(dbConn, authSvc, cfg, libMgr, store)
	srv.RegisterRoutes(e)

	body := `{"username":"admin","password":"admin"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(b, http.StatusOK, rec.Code)
	}
}

// BenchmarkProtectedRoute measures authenticated endpoint
func BenchmarkProtectedRoute(b *testing.B) {
	e := echo.New()
	cfg := config.Defaults()
	authSvc, _ := auth.NewService("test-secret-32-chars-long-for-testing!!", cfg.Auth.TokenTTL)
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	libMgr, _ := library.NewManager(dbConn, "")
	defer libMgr.Close()
	store, _ := securestore.NewStore("this-is-a-32-byte-key-for-testing!!")

	srv := api.NewServer(dbConn, authSvc, cfg, libMgr, store)
	srv.RegisterRoutes(e)

	token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(b, http.StatusOK, rec.Code)
	}
}

// BenchmarkDBInsert measures database write performance
func BenchmarkDBInsert(b *testing.B) {
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	require.NoError(b, dbConn.Migrate())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := dbConn.CreateUser(fmt.Sprintf("user-%d", i), fmt.Sprintf("user%d", i), "hash", "user")
		require.NoError(b, err)
	}
}

// BenchmarkDBQuery measures database read performance
func BenchmarkDBQuery(b *testing.B) {
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	require.NoError(b, dbConn.Migrate())

	// Seed 1000 users
	for i := 0; i < 1000; i++ {
		dbConn.CreateUser(fmt.Sprintf("user-%d", i), fmt.Sprintf("user%d", i), "hash", "user")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := dbConn.ListUsers()
		require.NoError(b, err)
	}
}

// TestStartupTime measures application startup
func TestStartupTime(t *testing.T) {
	start := time.Now()

	cfg := config.Defaults()
	authSvc, _ := auth.NewService("test-secret-32-chars-long-for-testing!!", cfg.Auth.TokenTTL)
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	libMgr, _ := library.NewManager(dbConn, "")
	defer libMgr.Close()
	store, _ := securestore.NewStore("this-is-a-32-byte-key-for-testing!!")

	srv := api.NewServer(dbConn, authSvc, cfg, libMgr, store)
	e := echo.New()
	srv.RegisterRoutes(e)

	elapsed := time.Since(start)
	t.Logf("Startup time: %v", elapsed)
	assert.Less(t, elapsed, 2*time.Second, "startup should be under 2 seconds")
}

// TestMemoryUsage measures memory footprint
func TestMemoryUsage(t *testing.T) {
	var m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	cfg := config.Defaults()
	authSvc, _ := auth.NewService("test-secret-32-chars-long-for-testing!!", cfg.Auth.TokenTTL)
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	libMgr, _ := library.NewManager(dbConn, "")
	defer libMgr.Close()
	store, _ := securestore.NewStore("this-is-a-32-byte-key-for-testing!!")

	srv := api.NewServer(dbConn, authSvc, cfg, libMgr, store)
	e := echo.New()
	srv.RegisterRoutes(e)

	var m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m2)

	alloc := m2.TotalAlloc - m1.TotalAlloc
	heap := m2.HeapAlloc - m1.HeapAlloc

	t.Logf("Memory alloc: %d bytes (%.2f MB)", alloc, float64(alloc)/1024/1024)
	t.Logf("Heap delta: %d bytes (%.2f MB)", heap, float64(heap)/1024/1024)

	assert.Less(t, alloc, uint64(50*1024*1024), "total alloc should be under 50MB")
}
