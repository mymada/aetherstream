package livetv

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_AddChannel(t *testing.T) {
	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	ch, err := m.AddChannel("France 2", 2, "http://example.com/f2.m3u8", "iptv")
	require.NoError(t, err)
	assert.NotEmpty(t, ch.ID)
	assert.Equal(t, "France 2", ch.Name)
	assert.Equal(t, 2, ch.Number)
	assert.True(t, ch.Enabled)
}

func TestManager_GetChannel(t *testing.T) {
	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	ch, _ := m.AddChannel("TF1", 1, "http://example.com/tf1.m3u8", "iptv")

	found, err := m.GetChannel(ch.ID)
	require.NoError(t, err)
	assert.Equal(t, ch.Name, found.Name)

	_, err = m.GetChannel("invalid")
	assert.Error(t, err)
}

func TestManager_ListChannels(t *testing.T) {
	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	m.AddChannel("Channel 3", 3, "http://example.com/c3.m3u8", "iptv")
	m.AddChannel("Channel 1", 1, "http://example.com/c1.m3u8", "iptv")
	m.AddChannel("Channel 2", 2, "http://example.com/c2.m3u8", "iptv")

	channels := m.ListChannels()
	require.Len(t, channels, 3)
	assert.Equal(t, 1, channels[0].Number)
	assert.Equal(t, 2, channels[1].Number)
	assert.Equal(t, 3, channels[2].Number)
}

func TestParseXMLTVTime(t *testing.T) {
	tm, err := parseXMLTVTime("20240101120000 +0000")
	require.NoError(t, err)
	assert.Equal(t, 2024, tm.Year())
	assert.Equal(t, time.January, tm.Month())
	assert.Equal(t, 1, tm.Day())
	assert.Equal(t, 12, tm.Hour())

	tm2, err := parseXMLTVTime("20240101120000")
	require.NoError(t, err)
	assert.Equal(t, 2024, tm2.Year())
}

func TestTimeshiftBuffer(t *testing.T) {
	buf := NewTimeshiftBuffer("/tmp/timeshift", 3)
	require.NotNil(t, buf)

	path1, err := buf.WriteSegment([]byte("segment1"), 5*time.Second)
	require.NoError(t, err)
	assert.Contains(t, path1, "segment_")
	time.Sleep(1 * time.Second)

	path2, err := buf.WriteSegment([]byte("segment2"), 5*time.Second)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	path3, err := buf.WriteSegment([]byte("segment3"), 5*time.Second)
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

	// 4th segment should evict first
	path4, err := buf.WriteSegment([]byte("segment4"), 5*time.Second)
	require.NoError(t, err)

	playlist := buf.GetPlaylist()
	assert.Contains(t, playlist, "#EXTM3U")
	assert.Contains(t, playlist, path2)
	assert.Contains(t, playlist, path3)
	assert.Contains(t, playlist, path4)
	assert.NotContains(t, playlist, path1) // evicted
}

func TestScheduleRecording(t *testing.T) {
	m := NewManager(nil, "/tmp/rec", "/tmp/buf")
	ch, _ := m.AddChannel("Test", 1, "http://example.com/test.m3u8", "iptv")

	start := time.Now()
	stop := start.Add(1 * time.Hour)
	rec, err := m.ScheduleRecording(ch.ID, start, stop)
	require.NoError(t, err)
	assert.Equal(t, "scheduled", rec.Status)
	assert.Equal(t, ch.ID, rec.ChannelID)
}

func TestEPGProgram(t *testing.T) {
	prog := EPGProgram{
		ChannelID:   "ch1",
		Title:       "News",
		Description: "Evening news",
		Start:       time.Now().Add(-1 * time.Hour),
		Stop:        time.Now().Add(1 * time.Hour),
		Category:    "news",
	}
	assert.Equal(t, "News", prog.Title)
	assert.Equal(t, "news", prog.Category)
}
