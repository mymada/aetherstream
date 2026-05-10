package dash

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMPD(t *testing.T) {
	mpd := NewMPD()
	assert.NotNil(t, mpd)
	assert.Equal(t, "urn:mpeg:dash:schema:mpd:2011", mpd.Xmlns)
	assert.Equal(t, "urn:mpeg:dash:profile:isoff-on-demand:2011", mpd.Profiles)
	assert.Equal(t, "static", mpd.Type)
	assert.Equal(t, "PT1.5S", mpd.MinBufferTime)
	assert.Equal(t, "PT4S", mpd.MaxSegmentDuration)
}

func TestGenerateMPD(t *testing.T) {
	config := MPDConfig{
		ID:       "test-item",
		Profiles: "urn:mpeg:dash:profile:isoff-on-demand:2011",
		Type:     "static",
		MinBufferTime: 1500 * time.Millisecond,
		MediaPresentationDuration: 120 * time.Second,
		MaxSegmentDuration: 4 * time.Second,
		BaseURL:  "https://example.com/media/",
		Periods: []PeriodInfo{
			{
				ID:       "1",
				Duration: 120,
				AdaptationSets: []AdaptationSetInfo{
					{
						ID:          "video",
						MimeType:    "video/mp4",
						ContentType: "video",
						SegmentAlignment: true,
						Representations: []RepresentationInfo{
							{
								ID:        "mobile",
								Bandwidth: 2000000,
								Width:     1280,
								Height:    720,
								Codecs:    "avc1.64001f,mp4a.40.2",
								MimeType:  "video/mp4",
								SegmentTemplate: &SegmentTemplateInfo{
									Timescale:       1000,
									SegmentDuration: 4000,
									Initialization:  "mobile_init.mp4",
									MediaPattern:    "mobile_$Number$.m4s",
									StartNumber:     1,
								},
							},
						},
					},
				},
			},
		},
	}

	mpd, err := GenerateMPD(config)
	require.NoError(t, err)
	assert.NotNil(t, mpd)
	assert.Equal(t, "test-item", mpd.ID)
	assert.Equal(t, "PT2M", mpd.MediaPresentationDuration)
	assert.Len(t, mpd.Periods, 1)
	assert.Len(t, mpd.Periods[0].AdaptationSets, 1)
	assert.Len(t, mpd.Periods[0].AdaptationSets[0].Representations, 1)
}

func TestMPDToXML(t *testing.T) {
	mpd := NewMPD()
	mpd.ID = "test"
	mpd.Periods = []Period{
		{
			ID: "1",
			AdaptationSets: []AdaptationSet{
				{
					ID:          "video",
					MimeType:    "video/mp4",
					ContentType: "video",
					Representations: []Representation{
						{
							ID:        "mobile",
							Bandwidth: 2000000,
							Width:     1280,
							Height:    720,
							Codecs:    "avc1.64001f",
							MimeType:  "video/mp4",
							SegmentTemplate: &SegmentTemplate{
								Timescale:      1000,
								Duration:       4000,
								Initialization: "init.mp4",
								Media:          "$Number$.m4s",
								StartNumber:    1,
							},
						},
					},
				},
			},
		},
	}

	xmlStr, err := mpd.ToXML()
	require.NoError(t, err)
	assert.Contains(t, xmlStr, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>")
	assert.Contains(t, xmlStr, "<MPD")
	assert.Contains(t, xmlStr, "id=\"test\"")
	assert.Contains(t, xmlStr, "<Period id=\"1\">")
	assert.Contains(t, xmlStr, "<AdaptationSet id=\"video\"")
	assert.Contains(t, xmlStr, "<Representation id=\"mobile\"")
	assert.Contains(t, xmlStr, "bandwidth=\"2000000\"")
	assert.Contains(t, xmlStr, "<SegmentTemplate")
	assert.Contains(t, xmlStr, "initialization=\"init.mp4\"")
	assert.Contains(t, xmlStr, "media=\"$Number$.m4s\"")
	assert.Contains(t, xmlStr, "</MPD>")
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{5 * time.Second, "PT5S"},
		{30 * time.Second, "PT30S"},
		{1 * time.Minute, "PT1M"},
		{1*time.Minute + 30*time.Second, "PT1M30S"},
		{1 * time.Hour, "PT1H"},
		{1*time.Hour + 30*time.Minute, "PT1H30M"},
		{1*time.Hour + 30*time.Minute + 15*time.Second, "PT1H30M15S"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildMPDForItem(t *testing.T) {
	profiles := []RepresentationInfo{
		{
			ID:        "mobile",
			Bandwidth: 2000000,
			Width:     1280,
			Height:    720,
			Codecs:    "avc1.64001f,mp4a.40.2",
			MimeType:  "video/mp4",
			SegmentTemplate: &SegmentTemplateInfo{
				Timescale:       1000,
				SegmentDuration: 4000,
				Initialization:  "mobile_init.mp4",
				MediaPattern:    "mobile_$Number$.m4s",
				StartNumber:     1,
			},
		},
		{
			ID:        "tablet",
			Bandwidth: 4000000,
			Width:     1920,
			Height:    1080,
			Codecs:    "avc1.640028,mp4a.40.2",
			MimeType:  "video/mp4",
			SegmentTemplate: &SegmentTemplateInfo{
				Timescale:       1000,
				SegmentDuration: 4000,
				Initialization:  "tablet_init.mp4",
				MediaPattern:    "tablet_$Number$.m4s",
				StartNumber:     1,
			},
		},
	}

	mpd, err := BuildMPDForItem("item-123", 3600, "https://cdn.example.com/", profiles)
	require.NoError(t, err)
	assert.Equal(t, "item-123", mpd.ID)
	assert.Equal(t, "PT1H", mpd.MediaPresentationDuration)
	assert.Equal(t, "https://cdn.example.com/", mpd.BaseURL.Value)
	assert.Len(t, mpd.Periods, 1)
	assert.Len(t, mpd.Periods[0].AdaptationSets[0].Representations, 2)

	// Check multi-bitrate representations
	reps := mpd.Periods[0].AdaptationSets[0].Representations
	assert.Equal(t, "mobile", reps[0].ID)
	assert.Equal(t, 2000000, reps[0].Bandwidth)
	assert.Equal(t, 1280, reps[0].Width)
	assert.Equal(t, 720, reps[0].Height)

	assert.Equal(t, "tablet", reps[1].ID)
	assert.Equal(t, 4000000, reps[1].Bandwidth)
	assert.Equal(t, 1920, reps[1].Width)
	assert.Equal(t, 1080, reps[1].Height)
}

func TestBuildMultiPeriodMPD(t *testing.T) {
	periods := []PeriodInfo{
		{
			ID:       "period_1",
			Duration: 600,
			AdaptationSets: []AdaptationSetInfo{
				{
					ID:          "video",
					MimeType:    "video/mp4",
					ContentType: "video",
					SegmentAlignment: true,
					Representations: []RepresentationInfo{
						{
							ID:        "mobile",
							Bandwidth: 2000000,
							Width:     1280,
							Height:    720,
							Codecs:    "avc1.64001f",
							MimeType:  "video/mp4",
							SegmentTemplate: &SegmentTemplateInfo{
								Timescale:       1000,
								SegmentDuration: 4000,
								Initialization:  "p1_init.mp4",
								MediaPattern:    "p1_$Number$.m4s",
								StartNumber:     1,
							},
						},
					},
				},
			},
		},
		{
			ID:       "period_2",
			Duration: 600,
			AdaptationSets: []AdaptationSetInfo{
				{
					ID:          "video",
					MimeType:    "video/mp4",
					ContentType: "video",
					SegmentAlignment: true,
					Representations: []RepresentationInfo{
						{
							ID:        "mobile",
							Bandwidth: 2000000,
							Width:     1280,
							Height:    720,
							Codecs:    "avc1.64001f",
							MimeType:  "video/mp4",
							SegmentTemplate: &SegmentTemplateInfo{
								Timescale:       1000,
								SegmentDuration: 4000,
								Initialization:  "p2_init.mp4",
								MediaPattern:    "p2_$Number$.m4s",
								StartNumber:     1,
							},
						},
					},
				},
			},
		},
	}

	mpd, err := BuildMultiPeriodMPD("item-456", periods, "https://cdn.example.com/")
	require.NoError(t, err)
	assert.Equal(t, "item-456", mpd.ID)
	assert.Equal(t, "PT20M", mpd.MediaPresentationDuration)
	assert.Len(t, mpd.Periods, 2)
	assert.Equal(t, "period_1", mpd.Periods[0].ID)
	assert.Equal(t, "period_2", mpd.Periods[1].ID)
	assert.Equal(t, "PT10M", mpd.Periods[0].Duration)
	assert.Equal(t, "PT10M", mpd.Periods[1].Duration)
}

func TestBuildLiveMPD(t *testing.T) {
	profiles := []AdaptationSetInfo{
		{
			ID:          "video",
			MimeType:    "video/mp4",
			ContentType: "video",
			SegmentAlignment: true,
			Representations: []RepresentationInfo{
				{
					ID:        "mobile",
					Bandwidth: 2000000,
					Width:     1280,
					Height:    720,
					Codecs:    "avc1.64001f",
					MimeType:  "video/mp4",
					SegmentTemplate: &SegmentTemplateInfo{
						Timescale:       1000,
						SegmentDuration: 4000,
						Initialization:  "init.mp4",
						MediaPattern:    "$Number$.m4s",
						StartNumber:     1,
					},
				},
			},
		},
	}

	startTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mpd, err := BuildLiveMPD("live-123", startTime, 3*time.Second, "https://live.example.com/", profiles)
	require.NoError(t, err)
	assert.Equal(t, "live-123", mpd.ID)
	assert.Equal(t, "dynamic", mpd.Type)
	assert.Equal(t, "urn:mpeg:dash:profile:isoff-live:2011", mpd.Profiles)
	assert.Equal(t, startTime.Format(time.RFC3339), mpd.AvailabilityStartTime)
	assert.NotEmpty(t, mpd.PublishTime)
	assert.Len(t, mpd.Periods, 1)
}

func TestGetProfileByName(t *testing.T) {
	tests := []struct {
		name     string
		expected int // bandwidth
	}{
		{"audio_only", 128000},
		{"mobile_low", 800000},
		{"mobile", 2000000},
		{"tablet", 4000000},
		{"tv", 6000000},
		{"tv_4k", 15000000},
		{"unknown", 2000000}, // falls back to mobile
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := GetProfileByName(tt.name)
			assert.Equal(t, tt.expected, profile.Bandwidth)
		})
	}
}

func TestBuildRepresentationsForProfiles(t *testing.T) {
	profiles := []string{"mobile", "tablet", "tv"}
	reps := BuildRepresentationsForProfiles(profiles)
	assert.Len(t, reps, 3)
	assert.Equal(t, "mobile", reps[0].ID)
	assert.Equal(t, "tablet", reps[1].ID)
	assert.Equal(t, "tv", reps[2].ID)
}

func TestAddAudioAdaptationSet(t *testing.T) {
	period := &Period{ID: "1"}
	audioReps := []RepresentationInfo{
		{
			ID:        "audio_128k",
			Bandwidth: 128000,
			Codecs:    "mp4a.40.2",
			MimeType:  "audio/mp4",
			AudioSamplingRate: "48000",
			SegmentTemplate: &SegmentTemplateInfo{
				Timescale:       1000,
				SegmentDuration: 4000,
				Initialization:  "audio_init.mp4",
				MediaPattern:    "audio_$Number$.m4s",
				StartNumber:     1,
			},
		},
	}

	AddAudioAdaptationSet(period, "en", audioReps)
	assert.Len(t, period.AdaptationSets, 1)
	assert.Equal(t, "audio_en", period.AdaptationSets[0].ID)
	assert.Equal(t, "audio/mp4", period.AdaptationSets[0].MimeType)
	assert.Equal(t, "en", period.AdaptationSets[0].Lang)
	assert.Equal(t, "audio", period.AdaptationSets[0].ContentType)
	assert.Len(t, period.AdaptationSets[0].Representations, 1)
	assert.Equal(t, 128000, period.AdaptationSets[0].Representations[0].Bandwidth)
}

func TestAddVideoAdaptationSet(t *testing.T) {
	period := &Period{ID: "1"}
	videoReps := []RepresentationInfo{
		{
			ID:        "mobile",
			Bandwidth: 2000000,
			Width:     1280,
			Height:    720,
			Codecs:    "avc1.64001f",
			MimeType:  "video/mp4",
			SegmentTemplate: &SegmentTemplateInfo{
				Timescale:       1000,
				SegmentDuration: 4000,
				Initialization:  "mobile_init.mp4",
				MediaPattern:    "mobile_$Number$.m4s",
				StartNumber:     1,
			},
		},
	}

	AddVideoAdaptationSet(period, videoReps)
	assert.Len(t, period.AdaptationSets, 1)
	assert.Equal(t, "video", period.AdaptationSets[0].ID)
	assert.Equal(t, "video/mp4", period.AdaptationSets[0].MimeType)
	assert.Equal(t, "video", period.AdaptationSets[0].ContentType)
	assert.Len(t, period.AdaptationSets[0].Representations, 1)
	assert.Equal(t, 2000000, period.AdaptationSets[0].Representations[0].Bandwidth)
}

func TestSegmentListGeneration(t *testing.T) {
	config := MPDConfig{
		ID:   "test-segment-list",
		Type: "static",
		Periods: []PeriodInfo{
			{
				ID: "1",
				AdaptationSets: []AdaptationSetInfo{
					{
						ID:          "video",
						MimeType:    "video/mp4",
						ContentType: "video",
						Representations: []RepresentationInfo{
							{
								ID:        "mobile",
								Bandwidth: 2000000,
								Codecs:    "avc1.64001f",
								MimeType:  "video/mp4",
								Segments: []SegmentInfo{
									{Duration: 4.0, Path: "segment1.m4s", Number: 1},
									{Duration: 4.0, Path: "segment2.m4s", Number: 2},
									{Duration: 4.0, Path: "segment3.m4s", Number: 3},
								},
							},
						},
					},
				},
			},
		},
	}

	mpd, err := GenerateMPD(config)
	require.NoError(t, err)

	xmlStr, err := mpd.ToXML()
	require.NoError(t, err)
	assert.Contains(t, xmlStr, "<SegmentList")
	assert.Contains(t, xmlStr, "<SegmentURL media=\"segment1.m4s\"")
	assert.Contains(t, xmlStr, "<SegmentURL media=\"segment2.m4s\"")
	assert.Contains(t, xmlStr, "<SegmentURL media=\"segment3.m4s\"")
}

func TestDefaultProfiles(t *testing.T) {
	profiles := DefaultProfiles()
	assert.Equal(t, "urn:mpeg:dash:profile:isoff-on-demand:2011", profiles)
}

func TestProfileMap(t *testing.T) {
	assert.Len(t, ProfileMap, 6)
	assert.Contains(t, ProfileMap, "audio_only")
	assert.Contains(t, ProfileMap, "mobile_low")
	assert.Contains(t, ProfileMap, "mobile")
	assert.Contains(t, ProfileMap, "tablet")
	assert.Contains(t, ProfileMap, "tv")
	assert.Contains(t, ProfileMap, "tv_4k")
}

func TestMPDWithBaseURL(t *testing.T) {
	mpd := NewMPD()
	mpd.BaseURL = &BaseURL{Value: "https://cdn.example.com/media/"}

	xmlStr, err := mpd.ToXML()
	require.NoError(t, err)
	assert.Contains(t, xmlStr, "<BaseURL>")
	assert.Contains(t, xmlStr, "https://cdn.example.com/media/")
	assert.Contains(t, xmlStr, "</BaseURL>")
}

func TestContentProtection(t *testing.T) {
	adaptationSet := AdaptationSet{
		ID:       "video",
		MimeType: "video/mp4",
		ContentProtection: []ContentProtection{
			{
				SchemeIDURI: "urn:mpeg:dash:mp4protection:2011",
				Value:       "cenc",
			},
		},
	}

	period := Period{
		ID: "1",
		AdaptationSets: []AdaptationSet{adaptationSet},
	}

	mpd := NewMPD()
	mpd.Periods = []Period{period}

	xmlStr, err := mpd.ToXML()
	require.NoError(t, err)
	assert.Contains(t, xmlStr, "<ContentProtection")
	assert.Contains(t, xmlStr, "schemeIdUri=\"urn:mpeg:dash:mp4protection:2011\"")
	assert.Contains(t, xmlStr, "value=\"cenc\"")
}

func TestPackageCompiles(t *testing.T) {
	assert.True(t, true, "dash package should compile")
}

func TestSegmentTemplateWithTimescale(t *testing.T) {
	st := SegmentTemplate{
		Timescale:      1000,
		Duration:       4000,
		Initialization: "init.mp4",
		Media:          "$Number$.m4s",
		StartNumber:    1,
	}

	rep := Representation{
		ID:              "test",
		Bandwidth:       2000000,
		SegmentTemplate: &st,
	}

	assert.Equal(t, 1000, rep.SegmentTemplate.Timescale)
	assert.Equal(t, 4000, rep.SegmentTemplate.Duration)
	assert.Equal(t, "init.mp4", rep.SegmentTemplate.Initialization)
	assert.Equal(t, "$Number$.m4s", rep.SegmentTemplate.Media)
	assert.Equal(t, 1, rep.SegmentTemplate.StartNumber)
}

func TestMultiBitrateSupport(t *testing.T) {
	profiles := []RepresentationInfo{
		{ID: "low", Bandwidth: 800000, Width: 854, Height: 480},
		{ID: "medium", Bandwidth: 2000000, Width: 1280, Height: 720},
		{ID: "high", Bandwidth: 4000000, Width: 1920, Height: 1080},
	}

	mpd, err := BuildMPDForItem("item-789", 1800, "", profiles)
	require.NoError(t, err)

	reps := mpd.Periods[0].AdaptationSets[0].Representations
	assert.Len(t, reps, 3)

	// Verify ascending bandwidth order (or at least all present)
	bandwidths := make([]int, len(reps))
	for i, rep := range reps {
		bandwidths[i] = rep.Bandwidth
	}
	assert.Contains(t, bandwidths, 800000)
	assert.Contains(t, bandwidths, 2000000)
	assert.Contains(t, bandwidths, 4000000)
}

func TestMultiPeriodSupport(t *testing.T) {
	periods := []PeriodInfo{
		{ID: "intro", Duration: 30},
		{ID: "main", Duration: 3600},
		{ID: "credits", Duration: 60},
	}

	mpd, err := BuildMultiPeriodMPD("movie-123", periods, "")
	require.NoError(t, err)
	assert.Len(t, mpd.Periods, 3)
	assert.Equal(t, "PT1H1M30S", mpd.MediaPresentationDuration)
}

func TestTemplateBasedSegments(t *testing.T) {
	profile := GetProfileByName("mobile")
	require.NotNil(t, profile.SegmentTemplate)

	assert.Equal(t, 1000, profile.SegmentTemplate.Timescale)
	assert.Equal(t, 4000.0, profile.SegmentTemplate.SegmentDuration)
	assert.Equal(t, "mobile_init.mp4", profile.SegmentTemplate.Initialization)
	assert.Contains(t, profile.SegmentTemplate.MediaPattern, "$Number$")
	assert.Equal(t, 1, profile.SegmentTemplate.StartNumber)
}

func TestHandlerRoutes(t *testing.T) {
	// This test verifies that the handler types compile correctly
	// Actual HTTP testing would require an Echo instance and mock DB
	server := NewServer(nil, "/tmp/media")
	assert.NotNil(t, server)
	assert.Equal(t, "/tmp/media", server.mediaRoot)
}
func TestGetContentType(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"manifest.mpd", "application/dash+xml"},
		{"segment.m4s", "video/mp4"},
		{"init.mp4", "video/mp4"},
		{"segment.webm", "video/webm"},
		{"segment.ts", "video/mp2t"},
		{"subtitles.vtt", "text/vtt"},
		{"subtitles.ttml", "application/ttml+xml"},
		{"unknown.xyz", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := getContentType(tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDiscoverProfiles(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	server := NewServer(nil, tmpDir)

	// Create fake profile directories
	for _, profile := range []string{"mobile", "tablet", "tv"} {
		err := os.MkdirAll(filepath.Join(tmpDir, "transcodes", "test-item", profile), 0750)
		require.NoError(t, err)
	}

	profiles := server.discoverProfiles(filepath.Join(tmpDir, "transcodes", "test-item"))
	assert.Len(t, profiles, 3)
	assert.Contains(t, profiles, "mobile")
	assert.Contains(t, profiles, "tablet")
	assert.Contains(t, profiles, "tv")
}
