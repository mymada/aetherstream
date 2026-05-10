package device

import (
	"database/sql"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestStore(t *testing.T) *Store {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	store := NewStore(db)
	require.NoError(t, store.Migrate())
	return store
}

func TestRegisterAndGet(t *testing.T) {
	store := setupTestStore(t)
	id := uuid.New().String()
	require.NoError(t, store.Register(id, "Living Room", "dev-1", "192.168.1.10", TrustUser))

	d, err := store.GetByDeviceID("dev-1")
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, "dev-1", d.DeviceID)
	assert.Equal(t, TrustUser, d.TrustLevel)
	assert.Equal(t, "192.168.1.10", d.IP)
}

func TestGetByIP(t *testing.T) {
	store := setupTestStore(t)
	id := uuid.New().String()
	require.NoError(t, store.Register(id, "Kitchen", "dev-2", "192.168.1.20", TrustAdmin))

	d, err := store.GetByIP("192.168.1.20")
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, "dev-2", d.DeviceID)
	assert.Equal(t, TrustAdmin, d.TrustLevel)
}

func TestUpdateTrust(t *testing.T) {
	store := setupTestStore(t)
	id := uuid.New().String()
	require.NoError(t, store.Register(id, "Test", "dev-3", "10.0.0.1", TrustGuest))
	require.NoError(t, store.UpdateTrust("dev-3", TrustBlocked))

	d, err := store.GetByDeviceID("dev-3")
	require.NoError(t, err)
	require.NotNil(t, d)
	assert.Equal(t, TrustBlocked, d.TrustLevel)
}

func TestDelete(t *testing.T) {
	store := setupTestStore(t)
	id := uuid.New().String()
	require.NoError(t, store.Register(id, "Test", "dev-4", "10.0.0.2", TrustUser))
	require.NoError(t, store.Delete("dev-4"))

	d, err := store.GetByDeviceID("dev-4")
	require.NoError(t, err)
	assert.Nil(t, d)
}

func TestIsAllowed_Blocked(t *testing.T) {
	store := setupTestStore(t)
	id := uuid.New().String()
	require.NoError(t, store.Register(id, "Test", "dev-5", "10.0.0.3", TrustBlocked))

	allowed, level, err := store.IsAllowed("dev-5", "")
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, TrustBlocked, level)
}

func TestIsAllowed_Unknown(t *testing.T) {
	store := setupTestStore(t)
	allowed, level, err := store.IsAllowed("unknown", "")
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, TrustGuest, level)
}

func TestList(t *testing.T) {
	store := setupTestStore(t)
	require.NoError(t, store.Register(uuid.New().String(), "A", "d1", "1.1.1.1", TrustUser))
	require.NoError(t, store.Register(uuid.New().String(), "B", "d2", "2.2.2.2", TrustGuest))

	devices, err := store.List()
	require.NoError(t, err)
	assert.Len(t, devices, 2)
}

func TestMiddleware(t *testing.T) {
	store := setupTestStore(t)
	require.NoError(t, store.Register(uuid.New().String(), "Test", "dev-6", "127.0.0.1", TrustUser))

	middleware := store.Middleware()
	e := echo.New()
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	req.Header.Set("X-Device-ID", "dev-6")

	err := middleware(func(c echo.Context) error { return nil })(c)
	require.NoError(t, err)
	assert.Equal(t, TrustUser, c.Get("device_level"))
}

func TestRequireTrust(t *testing.T) {
	store := setupTestStore(t)
	require.NoError(t, store.Register(uuid.New().String(), "Test", "dev-7", "127.0.0.1", TrustGuest))

	middleware := store.Middleware()
	requireTrust := RequireTrust(TrustUser)

	e := echo.New()
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	req.Header.Set("X-Device-ID", "dev-7")

	// First run device middleware
	err := middleware(func(c echo.Context) error {
		// Then run trust middleware
		return requireTrust(func(c2 echo.Context) error { return nil })(c)
	})(c)

	assert.Error(t, err)
	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, 403, httpErr.Code)
}
