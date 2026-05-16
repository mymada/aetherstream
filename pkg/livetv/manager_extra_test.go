package livetv

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devuser/aetherstream/pkg/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEPG_CacheHit(t *testing.T) {
	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	programs := []EPGProgram{
		{ChannelID: "ch1", Title: "News", Start: time.Now().Add(-1 * time.Hour), Stop: time.Now().Add(1 * time.Hour)},
	}
	m.cache.Set(cache.EPGKey("ch1"), programs, 30*time.Minute)

	result := m.GetEPG("ch1")
	require.Len(t, result, 1)
	assert.Equal(t, "News", result[0].Title)
}

func TestGetEPG_CacheMiss(t *testing.T) {
	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	result := m.GetEPG("nonexistent-channel")
	assert.Nil(t, result)
}

func TestGetCurrentProgram_Found(t *testing.T) {
	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	now := time.Now()
	programs := []EPGProgram{
		{ChannelID: "ch2", Title: "Old Show", Start: now.Add(-3 * time.Hour), Stop: now.Add(-1 * time.Hour)},
		{ChannelID: "ch2", Title: "Current Show", Start: now.Add(-30 * time.Minute), Stop: now.Add(30 * time.Minute)},
		{ChannelID: "ch2", Title: "Future Show", Start: now.Add(1 * time.Hour), Stop: now.Add(2 * time.Hour)},
	}
	m.cache.Set(cache.EPGKey("ch2"), programs, 30*time.Minute)

	prog := m.GetCurrentProgram("ch2")
	require.NotNil(t, prog)
	assert.Equal(t, "Current Show", prog.Title)
}

func TestGetCurrentProgram_NotFound(t *testing.T) {
	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	prog := m.GetCurrentProgram("no-channel")
	assert.Nil(t, prog)
}

func TestGetCurrentProgram_NoProgramAiring(t *testing.T) {
	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	now := time.Now()
	programs := []EPGProgram{
		{ChannelID: "ch3", Title: "Past Show", Start: now.Add(-2 * time.Hour), Stop: now.Add(-1 * time.Hour)},
	}
	m.cache.Set(cache.EPGKey("ch3"), programs, 30*time.Minute)

	prog := m.GetCurrentProgram("ch3")
	assert.Nil(t, prog)
}

func TestScheduleRecording_NotFound(t *testing.T) {
	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	_, err := m.ScheduleRecording("nonexistent-channel", time.Now(), time.Now().Add(time.Hour))
	assert.Error(t, err)
}

func TestStartRecording_NotFound(t *testing.T) {
	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	_, err := m.StartRecording("nonexistent-channel", time.Hour)
	assert.Error(t, err)
}

func TestRecording_SetStatus(t *testing.T) {
	rec := &Recording{ID: "rec-1", Status: "scheduled"}
	rec.setStatus("recording")
	assert.Equal(t, "recording", rec.Status)
	rec.setStatus("completed")
	assert.Equal(t, "completed", rec.Status)
}

func TestStreamProxy_DisabledChannel(t *testing.T) {
	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	ch := &Channel{ID: "ch-disabled", Name: "Disabled", Enabled: false, SourceURL: "http://example.com/stream"}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/stream", nil)
	err := m.StreamProxy(ch, w, r)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestStreamProxy_SourceProxied(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp2t")
		fmt.Fprint(w, "stream-data")
	}))
	defer backend.Close()

	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	ch := &Channel{ID: "ch-live", Name: "Live", Enabled: true, SourceURL: backend.URL}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/stream", nil)
	err := m.StreamProxy(ch, w, r)
	assert.NoError(t, err)
	assert.Contains(t, w.Body.String(), "stream-data")
}

func TestParseEPG_ValidFile(t *testing.T) {
	xmltv := `<?xml version="1.0" encoding="utf-8"?>
<tv>
  <channel id="ch-news"><display-name>News</display-name></channel>
  <programme channel="ch-news" start="20240101120000 +0000" stop="20240101130000 +0000">
    <title>Midday News</title>
    <desc>Latest news</desc>
    <category>news</category>
  </programme>
</tv>`

	tmpDir := t.TempDir()
	epgPath := filepath.Join(tmpDir, "epg.xml")
	require.NoError(t, os.WriteFile(epgPath, []byte(xmltv), 0644))

	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	err := m.ParseEPG(epgPath)
	require.NoError(t, err)

	programs := m.GetEPG("ch-news")
	require.Len(t, programs, 1)
	assert.Equal(t, "Midday News", programs[0].Title)
	assert.Equal(t, "Latest news", programs[0].Description)
}

func TestParseEPG_FileNotFound(t *testing.T) {
	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	err := m.ParseEPG("/nonexistent/epg.xml")
	assert.Error(t, err)
}

func TestParseEPG_InvalidXML(t *testing.T) {
	tmpDir := t.TempDir()
	badPath := filepath.Join(tmpDir, "bad.xml")
	require.NoError(t, os.WriteFile(badPath, []byte("not xml"), 0644))

	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	err := m.ParseEPG(badPath)
	assert.Error(t, err)
}

func TestParseXMLTVTime_TooShort(t *testing.T) {
	_, err := parseXMLTVTime("2024")
	assert.Error(t, err)
}

func TestParseXMLTVTime_CompactFormat(t *testing.T) {
	tm, err := parseXMLTVTime("20240315143000")
	require.NoError(t, err)
	assert.Equal(t, 2024, tm.Year())
	assert.Equal(t, time.March, tm.Month())
	assert.Equal(t, 15, tm.Day())
	assert.Equal(t, 14, tm.Hour())
}

func TestTimeshiftBuffer_GetPlaylist_Empty(t *testing.T) {
	buf := NewTimeshiftBuffer(t.TempDir(), 5)
	playlist := buf.GetPlaylist()
	assert.Contains(t, playlist, "#EXTM3U")
	assert.Contains(t, playlist, "#EXT-X-TARGETDURATION:10")
	assert.NotContains(t, playlist, "#EXTINF")
}

func TestTimeshiftBuffer_GetPlaylist_WithSegments(t *testing.T) {
	tmpDir := t.TempDir()
	buf := NewTimeshiftBuffer(tmpDir, 5)

	_, err := buf.WriteSegment([]byte("ts-data-1"), 6*time.Second)
	require.NoError(t, err)
	_, err = buf.WriteSegment([]byte("ts-data-2"), 8*time.Second)
	require.NoError(t, err)

	playlist := buf.GetPlaylist()
	assert.Contains(t, playlist, "#EXTINF:6.0,")
	assert.Contains(t, playlist, "#EXTINF:8.0,")
}
