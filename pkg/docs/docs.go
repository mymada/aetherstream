package docs

import (
	"embed"
	"net/http"

	"github.com/labstack/echo/v4"
)

//go:embed swagger.json
var swaggerJSON embed.FS

//go:embed swagger-ui.html
var swaggerUI embed.FS

// RegisterRoutes adds swagger documentation endpoints to the Echo router.
func RegisterRoutes(e *echo.Echo) {
	// Serve swagger.json
	e.GET("/swagger.json", func(c echo.Context) error {
		data, err := swaggerJSON.ReadFile("swagger.json")
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "swagger.json not found"})
		}
		return c.JSONBlob(http.StatusOK, data)
	})

	// Serve swagger UI
	e.GET("/swagger/*", func(c echo.Context) error {
		path := c.Param("*")
		if path == "" || path == "/" {
			path = "swagger-ui.html"
		}
		data, err := swaggerUI.ReadFile(path)
		if err != nil {
			return c.String(http.StatusNotFound, "not found")
		}
		return c.Blob(http.StatusOK, http.DetectContentType(data), data)
	})
}
