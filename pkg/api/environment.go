package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"
)

type fsEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "directory"
}

// handleGetDrives returns filesystem root entry points.
// On Linux: /, /home, /mnt/*, /media/*, /srv — whatever actually exists.
func (s *Server) handleGetDrives(c echo.Context) error {
	candidates := []string{"/", "/home", "/mnt", "/media", "/srv", "/data", "/opt"}

	// Also pick up immediate children of /mnt and /media (common mount points)
	for _, base := range []string{"/mnt", "/media"} {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				candidates = append(candidates, filepath.Join(base, e.Name()))
			}
		}
	}

	seen := map[string]bool{}
	var result []fsEntry
	for _, p := range candidates {
		if seen[p] {
			continue
		}
		seen[p] = true
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			continue
		}
		name := filepath.Base(p)
		if p == "/" {
			name = "/ (racine)"
		}
		result = append(result, fsEntry{Name: name, Path: p, Type: "directory"})
	}

	return c.JSON(http.StatusOK, result)
}

// handleGetDirectoryContents lists subdirectories of ?path=X.
func (s *Server) handleGetDirectoryContents(c echo.Context) error {
	path := c.QueryParam("path")
	if path == "" {
		path = "/"
	}

	// Resolve to absolute, block path traversal
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return echo.NewHTTPError(http.StatusBadRequest, "path must be absolute")
	}

	info, err := os.Stat(clean)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "path not found")
	}
	if !info.IsDir() {
		return echo.NewHTTPError(http.StatusBadRequest, "path is not a directory")
	}

	entries, err := os.ReadDir(clean)
	if err != nil {
		return echo.NewHTTPError(http.StatusForbidden, "cannot read directory")
	}

	var result []fsEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// Skip hidden dirs and common noise
		if strings.HasPrefix(name, ".") || name == "proc" || name == "sys" || name == "dev" || name == "run" {
			continue
		}
		result = append(result, fsEntry{
			Name: name,
			Path: filepath.Join(clean, name),
			Type: "directory",
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return c.JSON(http.StatusOK, result)
}

// handleGetParentPath returns the parent directory of ?path=X.
func (s *Server) handleGetParentPath(c echo.Context) error {
	path := c.QueryParam("path")
	if path == "" || path == "/" {
		return c.JSON(http.StatusOK, map[string]string{"path": "/"})
	}
	clean := filepath.Clean(path)
	parent := filepath.Dir(clean)
	return c.JSON(http.StatusOK, map[string]string{"path": parent})
}
