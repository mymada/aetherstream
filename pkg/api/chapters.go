package api

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"github.com/devuser/aetherstream/pkg/db"
	"github.com/labstack/echo/v4"
)

// Chapter represents a video chapter marker.
type Chapter struct {
	Name     string  `json:"name"`
	Position float64 `json:"position"` // seconds
}

// handleListChapters returns chapter markers for an item.
func (s *Server) handleListChapters(c echo.Context) error {
	itemID := c.Param("id")

	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	// Parse chapters from metadata if available
	chapters := ParseChapters(item.Chapters)
	return c.JSON(http.StatusOK, chapters)
}

// handleGetChapterAt returns the chapter at a given timestamp.
func (s *Server) handleGetChapterAt(c echo.Context) error {
	itemID := c.Param("id")
	atStr := c.QueryParam("at")
	at, err := strconv.ParseFloat(atStr, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid at parameter")
	}

	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	chapters := ParseChapters(item.Chapters)
	var current *Chapter
	for i := range chapters {
		if chapters[i].Position <= at {
			current = &chapters[i]
		}
	}

	if current == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"chapter": nil,
			"at":      at,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"chapter": *current,
		"at":      at,
	})
}

// handleScanChapters extracts chapters from a media file via ffprobe and stores them.
func (s *Server) handleScanChapters(c echo.Context) error {
	itemID := c.Param("id")

	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}

	go func() {
		chapters, err := extractChaptersFFprobe(item.Path)
		if err != nil || len(chapters) == 0 {
			return
		}
		_ = s.db.SaveChapters(itemID, chapters)
	}()

	return c.JSON(http.StatusAccepted, map[string]string{
		"status": "scanning",
	})
}

// extractChaptersFFprobe runs ffprobe to extract chapter markers from a media file.
func extractChaptersFFprobe(path string) ([]db.Chapter, error) {
	// #nosec G204 — path is a validated library file path
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_chapters",
		path,
	).Output()
	if err != nil {
		return nil, err
	}

	var result struct {
		Chapters []struct {
			ID        int               `json:"id"`
			StartTime string            `json:"start_time"`
			EndTime   string            `json:"end_time"`
			Tags      map[string]string `json:"tags"`
		} `json:"chapters"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}

	chapters := make([]db.Chapter, 0, len(result.Chapters))
	for i, ch := range result.Chapters {
		start, _ := strconv.ParseFloat(ch.StartTime, 64)
		end, _ := strconv.ParseFloat(ch.EndTime, 64)
		title := ch.Tags["title"]
		if title == "" {
			title = "Chapter " + strconv.Itoa(i+1)
		}
		chapters = append(chapters, db.Chapter{
			ChapterIndex:    i,
			Title:           title,
			StartSeconds:    start,
			EndSeconds:      end,
			DurationSeconds: end - start,
		})
	}
	return chapters, nil
}

// ParseChapters parses a semicolon-separated chapter string.
// Format: "Intro=0;Opening=120;Main=300;Ending=1800"
func ParseChapters(raw string) []Chapter {
	if raw == "" {
		return []Chapter{}
	}

	var chapters []Chapter
	parts := strings.Split(raw, ";")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		pos, err := strconv.ParseFloat(strings.TrimSpace(kv[1]), 64)
		if err != nil {
			continue
		}
		chapters = append(chapters, Chapter{
			Name:     strings.TrimSpace(kv[0]),
			Position: pos,
		})
	}

	return chapters
}
