package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	assert.NotNil(t, m)
	assert.NotNil(t, m.Registry)
	assert.NotNil(t, m.HTTPRequestsTotal)
	assert.NotNil(t, m.HTTPRequestDuration)
	assert.NotNil(t, m.ActiveStreams)
	assert.NotNil(t, m.DBConnections)
	assert.NotNil(t, m.MemoryUsage)
	assert.NotNil(t, m.DBQueriesTotal)
	assert.NotNil(t, m.CacheHitsTotal)
}

func TestEchoMiddleware(t *testing.T) {
	m := NewMetrics()
	e := echo.New()
	e.Use(m.EchoMiddleware())

	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMetricsHandler(t *testing.T) {
	m := NewMetrics()
	handler := m.MetricsHandler()
	assert.NotNil(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "go_gc_duration_seconds")
}

func TestUpdateMemory(t *testing.T) {
	m := NewMetrics()
	m.UpdateMemory()
	// Should not panic
}
