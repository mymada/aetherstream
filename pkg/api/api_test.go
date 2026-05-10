package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/devuser/aetherstream/pkg/config"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/library"
	"github.com/devuser/aetherstream/pkg/securestore"
	"golang.org/x/crypto/bcrypt"
)

func TestHandleSystemInfo(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/system/info", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	cfg := config.Defaults()
	authSvc, _ := auth.NewService("test-secret-32-chars-long-for-testing!!", cfg.Auth.TokenTTL)
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()

	libMgr, _ := library.NewManager(dbConn, "")
	defer libMgr.Close()

	// Create secure store for test
	store, _ := securestore.NewStore("this-is-a-32-byte-key-for-testing!!")

	srv := NewServer(dbConn, authSvc, cfg, libMgr, store)
	err := srv.handleSystemInfo(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandleLogin(t *testing.T) {
	e := echo.New()
	cfg := config.Defaults()
	authSvc, _ := auth.NewService("test-secret-32-chars-long-for-testing!!", cfg.Auth.TokenTTL)
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()

	libMgr, _ := library.NewManager(dbConn, "")
	defer libMgr.Close()

	// Create secure store for test
	store, _ := securestore.NewStore("this-is-a-32-byte-key-for-testing!!")

	srv := NewServer(dbConn, authSvc, cfg, libMgr, store)

	// Valid login — create user in DB with bcrypt hash
	dbConn.Migrate()
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	dbConn.CreateUser("admin-1", "admin", string(hash), "admin")

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"admin","password":"admin"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := srv.handleLogin(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "token")

	// Invalid login
	req2 := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"bad","password":"bad"}`))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)

	err2 := srv.handleLogin(c2)
	assert.Error(t, err2)
	assert.Equal(t, http.StatusUnauthorized, err2.(*echo.HTTPError).Code)
}

func TestProtectedRoute(t *testing.T) {
	e := echo.New()
	cfg := config.Defaults()
	authSvc, _ := auth.NewService("test-secret-32-chars-long-for-testing!!", cfg.Auth.TokenTTL)
	dbConn, _ := db.New(":memory:")
	defer dbConn.Close()
	dbConn.Migrate()

	libMgr, _ := library.NewManager(dbConn, "")
	defer libMgr.Close()

	// Create secure store for test
	store, _ := securestore.NewStore("this-is-a-32-byte-key-for-testing!!")

	srv := NewServer(dbConn, authSvc, cfg, libMgr, store)
	srv.RegisterRoutes(e)

	// Without token
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// With valid token
	token, _ := authSvc.GenerateToken("admin-1", "admin", "admin")
	req2 := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
}
