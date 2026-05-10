package docs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestRegisterRoutes_SwaggerJSON(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e)

	req := httptest.NewRequest(http.MethodGet, "/swagger.json", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "openapi")
}

func TestRegisterRoutes_SwaggerUI_Root(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e)

	req := httptest.NewRequest(http.MethodGet, "/swagger/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "swagger-ui")
}

func TestRegisterRoutes_SwaggerUI_NotFound(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e)

	req := httptest.NewRequest(http.MethodGet, "/swagger/nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not found")
}
