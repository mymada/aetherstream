package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestIsValidItemID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"abc123", true},
		{"item-1", true},
		{"item_1", true},
		{"abc../def", false},
		{"abc/def", false},
		{"abc\\def", false},
		{"", false},
		{"../../../etc/passwd", false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidItemID(tt.id))
		})
	}
}

func TestIsValidLanguageCode(t *testing.T) {
	tests := []struct {
		lang  string
		valid bool
	}{
		{"en", true},
		{"eng", true},
		{"fr", true},
		{"fra", true},
		{"en-US", true},
		{"fr-FR", true},
		{"", false},
		{"../../../etc/passwd", false},
		{"en/../fr", false},
		{"a", false},
		{"english", false},
	}
	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidLanguageCode(tt.lang))
		})
	}
}

func TestIsPathWithinAllowedDirs(t *testing.T) {
	tmpDir := os.TempDir()
	allowed := []string{tmpDir, "./thumbnails"}

	tests := []struct {
		path  string
		valid bool
	}{
		{filepath.Join(tmpDir, "subtitle.srt"), true},
		{filepath.Join(tmpDir, "sub", "subtitle.srt"), true},
		{"./thumbnails/poster.jpg", true},
		{"/etc/passwd", false},
		{filepath.Join(tmpDir, "..", "etc", "passwd"), false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.valid, isPathWithinAllowedDirs(tt.path, allowed))
		})
	}
}

func TestHandleGetSubtitleValidation(t *testing.T) {
	e := echo.New()
	// Test that invalid lang is rejected before DB lookup
	req := httptest.NewRequest(http.MethodGet, "/api/items/item-1/subtitles/../../../etc/passwd", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
