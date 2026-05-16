package api

import (
	"net/http"

	"github.com/devuser/aetherstream/pkg/autocollections"
	"github.com/labstack/echo/v4"
)

// handleAutoCollections returns or refreshes auto-generated collections for the current user.
// GET  /api/auto-collections           — list existing auto-collections
// POST /api/auto-collections?group=X   — rebuild collections grouped by field (genre/year/actor)
func (s *Server) handleAutoCollections(c echo.Context) error {
	userID := getUserID(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	acDB := autocollections.NewDB(s.db.DB)
	if err := acDB.Migrate(); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "auto-collections table: "+err.Error())
	}

	if c.Request().Method == http.MethodPost {
		groupBy := c.QueryParam("group")
		if groupBy == "" {
			groupBy = "media_type"
		}
		if err := acDB.RefreshAutoCollections(userID, groupBy); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "refresh: "+err.Error())
		}
	}

	collections, err := acDB.ListAutoCollections(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if collections == nil {
		collections = []*autocollections.AutoCollection{}
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"collections": collections,
	})
}

// handleEnrichMetadata triggers TMDb metadata enrichment for items missing metadata.
func (s *Server) handleEnrichMetadata(c echo.Context) error {
	var missing int
	_ = s.db.QueryRow(
		"SELECT COUNT(*) FROM items WHERE overview = '' OR overview IS NULL",
	).Scan(&missing)

	// Trigger a library scan on all libraries, which re-fetches metadata for unenriched items.
	go func() {
		if s.library == nil {
			return
		}
		libs, err := s.db.ListLibraries()
		if err != nil {
			return
		}
		for _, lib := range libs {
			_ = s.library.ScanLibrary(lib.ID)
		}
	}()

	return c.JSON(http.StatusAccepted, map[string]interface{}{
		"status":  "queued",
		"pending": missing,
		"message": "Metadata enrichment started in background",
	})
}
