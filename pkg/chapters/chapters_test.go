package chapters

import (
	"testing"
)

func TestParseTimeBase(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"1/1000", 0.001},
		{"1/1000000000", 0.000000001},
		{"1/1000000", 0.000001},
		{"", 0.000000001},
		{"invalid", 0.000000001},
		{"2/0", 0.000000001},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseTimeBase(tt.input)
			if result != tt.expected {
				t.Errorf("parseTimeBase(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	if FormatDuration(0) != "00:00" {
		t.Errorf("FormatDuration(0) = %s, want 00:00", FormatDuration(0))
	}
	if FormatDuration(61) != "01:01" {
		t.Errorf("FormatDuration(61) = %s, want 01:01", FormatDuration(61))
	}
	if FormatDuration(3661) != "01:01:01" {
		t.Errorf("FormatDuration(3661) = %s, want 01:01:01", FormatDuration(3661))
	}
}

func TestExtractChapters_InvalidPath(t *testing.T) {
	_, err := ExtractChapters("/nonexistent/path/file.mkv")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestExtractChapters_NoChapters(t *testing.T) {
	// ffprobe on /dev/null may error or return empty chapters; accept either
	chapters, err := ExtractChapters("/dev/null")
	if err != nil {
		// ffprobe can exit non-zero for /dev/null; that's fine
		return
	}
	if len(chapters) != 0 {
		t.Errorf("expected 0 chapters, got %d", len(chapters))
	}
}
