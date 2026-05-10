
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestDebug_Login(t *testing.T) {
	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "test.db")
	database, err := db.New(dbPath)
	require.NoError(t, err)
	require.NoError(t, database.Migrate())
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 0, Host: "127.0.0.1", StaticPath: testDir},
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

	// Create admin user directly in DB using same hashPassword approach
	adminID := "admin-" + fmt.Sprintf("%d", time.Now().UnixNano())
	out, err := os.ReadFile("/dev/null") // dummy
	_ = out
	hashCmd := []string{"python3", "-c", "import bcrypt, sys; print(bcrypt.hashpw(sys.argv[1].encode(), bcrypt.gensalt(rounds=4)).decode())", "adminpass"}
	result, _ := os.ReadFile("/dev/null")
	_ = result
	_ = hashCmd
	// Just use bcrypt directly via exec
	// Actually, let's use a simple approach: generate hash with a helper
	// Skip for now and use the same approach as harness
	// But we can just use a precomputed bcrypt hash
	precomputedHash := "$2a$04$abcdefghijklmnopqrstuuxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	require.NoError(t, database.CreateUser(adminID, "admin", precomputedHash, "admin"))

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "adminpass"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	fmt.Printf("DEBUG Status: %d\n", rec.Result().StatusCode)
	fmt.Printf("DEBUG Body: %s\n", rec.Body.String())
	fmt.Printf("DEBUG Headers: %v\n", rec.Result().Header)
	require.Equal(t, http.StatusOK, rec.Result().StatusCode)
}
