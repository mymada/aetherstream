package livetv

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/devuser/aetherstream/pkg/cache"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Channel represents a TV channel with streaming source
type Channel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Number     int    `json:"number"`
	SourceURL  string `json:"source_url"`
	SourceType string `json:"source_type"` // iptv, dvb, http
	LogoURL    string `json:"logo_url,omitempty"`
	Enabled    bool   `json:"enabled"`
}

// Manager handles live TV channels, EPG, recording, timeshift
type Manager struct {
	db        *db.DB
	channels  map[string]*Channel
	mu        sync.RWMutex
	cache     cache.Cache
	recDir    string
	bufferDir string
}

// EPGProgram represents a TV program from XMLTV
type EPGProgram struct {
	ChannelID   string    `json:"channel_id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Start       time.Time `json:"start"`
	Stop        time.Time `json:"stop"`
	Category    string    `json:"category,omitempty"`
}

// NewManager creates a live TV manager
func NewManager(database *db.DB, recDir, bufferDir string) *Manager {
	return &Manager{
		db:        database,
		channels:  make(map[string]*Channel),
		cache:     cache.NewLRUCache(500),
		recDir:    recDir,
		bufferDir: bufferDir,
	}
}

// AddChannel adds a new channel
func (m *Manager) AddChannel(name string, number int, sourceURL, sourceType string) (*Channel, error) {
	ch := &Channel{
		ID:         uuid.New().String(),
		Name:       name,
		Number:     number,
		SourceURL:  sourceURL,
		SourceType: sourceType,
		Enabled:    true,
	}
	m.mu.Lock()
	m.channels[ch.ID] = ch
	m.mu.Unlock()
	return ch, nil
}

// GetChannel returns a channel by ID
func (m *Manager) GetChannel(id string) (*Channel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ch, ok := m.channels[id]
	if !ok {
		return nil, fmt.Errorf("channel not found: %s", id)
	}
	return ch, nil
}

// ListChannels returns all enabled channels sorted by number
func (m *Manager) ListChannels() []*Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Channel
	for _, ch := range m.channels {
		if ch.Enabled {
			result = append(result, ch)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Number < result[j].Number
	})
	return result
}

// StreamProxy proxies the channel source to the HTTP response
func (m *Manager) StreamProxy(ch *Channel, w http.ResponseWriter, r *http.Request) error {
	if !ch.Enabled {
		return fmt.Errorf("channel disabled")
	}

	resp, err := http.Get(ch.SourceURL)
	if err != nil {
		return fmt.Errorf("source unreachable: %w", err)
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		if strings.HasPrefix(k, "Content-") || k == "Accept-Ranges" {
			w.Header()[k] = v
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	return err
}

// --- EPG ---

// XMLTVChannel represents a channel in XMLTV
type XMLTVChannel struct {
	ID          string `xml:"id,attr"`
	DisplayName string `xml:"display-name"`
	Icon        struct {
		Src string `xml:"src,attr"`
	} `xml:"icon"`
}

// XMLTVProgramme represents a programme in XMLTV
type XMLTVProgramme struct {
	Channel  string `xml:"channel,attr"`
	Start    string `xml:"start,attr"`
	Stop     string `xml:"stop,attr"`
	Title    string `xml:"title"`
	Desc     string `xml:"desc"`
	Category string `xml:"category"`
}

// XMLTV represents the root XMLTV document
type XMLTV struct {
	Channels   []XMLTVChannel   `xml:"channel"`
	Programmes []XMLTVProgramme `xml:"programme"`
}

// ParseEPG parses an XMLTV file and loads programs into cache
func (m *Manager) ParseEPG(path string) error {
	// Security: validate path is within expected EPG directories (caller should ensure this)
	data, err := os.ReadFile(path) // #nosec G304 - caller must validate path against EPG dirs
	if err != nil {
		return fmt.Errorf("read epg: %w", err)
	}

	var xmltv XMLTV
	if err := xml.Unmarshal(data, &xmltv); err != nil {
		return fmt.Errorf("parse epg: %w", err)
	}

	// Aggregate per channel
	channelMap := make(map[string][]EPGProgram)
	for _, p := range xmltv.Programmes {
		start, _ := parseXMLTVTime(p.Start)
		stop, _ := parseXMLTVTime(p.Stop)
		prog := EPGProgram{
			ChannelID:   p.Channel,
			Title:       p.Title,
			Description: p.Desc,
			Start:       start,
			Stop:        stop,
			Category:    p.Category,
		}
		channelMap[p.Channel] = append(channelMap[p.Channel], prog)
	}

	for chID, programs := range channelMap {
		m.cache.Set(cache.EPGKey(chID), programs, 30*time.Minute)
	}

	log.Info().Int("channels", len(xmltv.Channels)).Int("programmes", len(xmltv.Programmes)).Msg("EPG loaded")
	return nil
}

// GetEPG returns programs for a channel ID
func (m *Manager) GetEPG(channelID string) []EPGProgram {
	if cached, ok := m.cache.Get(cache.EPGKey(channelID)); ok {
		return cached.([]EPGProgram)
	}
	return nil
}

// GetCurrentProgram returns the program currently airing
func (m *Manager) GetCurrentProgram(channelID string) *EPGProgram {
	now := time.Now()
	programs := m.GetEPG(channelID)
	for _, p := range programs {
		if now.After(p.Start) && now.Before(p.Stop) {
			return &p
		}
	}
	return nil
}

// parseXMLTVTime parses XMLTV time format (YYYYMMDDHHMMSS +0000)
func parseXMLTVTime(s string) (time.Time, error) {
	if len(s) < 14 {
		return time.Time{}, fmt.Errorf("invalid xmltv time")
	}
	// Try common format
	layout := "20060102150405 -0700"
	if len(s) > 14 && s[14] == ' ' {
		return time.Parse(layout, s)
	}
	// Compact format
	layout = "20060102150405"
	return time.ParseInLocation(layout, s[:14], time.UTC)
}

// --- Recording ---

// Recording represents an active or completed recording
type Recording struct {
	ID        string    `json:"id"`
	ChannelID string    `json:"channel_id"`
	StartTime time.Time `json:"start_time"`
	StopTime  time.Time `json:"stop_time"`
	FilePath  string    `json:"file_path"`
	Status    string    `json:"status"` // scheduled, recording, completed, failed
	mu        sync.Mutex
}

// ScheduleRecording schedules a recording
func (m *Manager) ScheduleRecording(channelID string, start, stop time.Time) (*Recording, error) {
	ch, err := m.GetChannel(channelID)
	if err != nil {
		return nil, err
	}

	recID := uuid.New().String()
	rec := &Recording{
		ID:        recID,
		ChannelID: ch.ID,
		StartTime: start,
		StopTime:  stop,
		FilePath:  filepath.Join(m.recDir, recID+".ts"),
		Status:    "scheduled",
	}

	return rec, nil
}

// StartRecording starts an immediate recording
func (m *Manager) StartRecording(channelID string, duration time.Duration) (*Recording, error) {
	ch, err := m.GetChannel(channelID)
	if err != nil {
		return nil, err
	}

	rec := &Recording{
		ID:        uuid.New().String(),
		ChannelID: ch.ID,
		StartTime: time.Now(),
		StopTime:  time.Now().Add(duration),
		FilePath:  filepath.Join(m.recDir, uuid.New().String()+".ts"),
		Status:    "recording",
	}

	// Start background recording with context for cancellation/timeout
	go m.recordStream(ch, rec, duration)

	return rec, nil
}

// recordStream records a channel to disk with proper lifecycle management
func (m *Manager) recordStream(ch *Channel, rec *Recording, duration time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), duration+30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ch.SourceURL, nil)
	if err != nil {
		log.Error().Err(err).Str("channel", ch.Name).Msg("recording request failed")
		rec.setStatus("failed")
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().Err(err).Str("channel", ch.Name).Msg("recording failed")
		rec.setStatus("failed")
		return
	}
	defer resp.Body.Close()

	f, err := os.OpenFile(rec.FilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		log.Error().Err(err).Str("path", rec.FilePath).Msg("recording file create failed")
		rec.setStatus("failed")
		return
	}
	defer f.Close()

	// Copy with context cancellation
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		log.Error().Err(err).Str("recording", rec.ID).Msg("recording copy failed")
		rec.setStatus("failed")
		return
	}
	rec.setStatus("completed")
	log.Info().Str("recording", rec.ID).Str("channel", ch.Name).Msg("recording completed")
}

func (r *Recording) setStatus(status string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Status = status
}

// --- Timeshift ---

// TimeshiftBuffer provides a circular buffer for live pause/rewind
type TimeshiftBuffer struct {
	mu       sync.Mutex
	segments []Segment
	maxSize  int
	dir      string
}

// Segment represents a buffered chunk
type Segment struct {
	Path      string
	StartTime time.Time
	Duration  time.Duration
}

// NewTimeshiftBuffer creates a circular buffer
func NewTimeshiftBuffer(dir string, maxSegments int) *TimeshiftBuffer {
	_ = os.MkdirAll(dir, 0750)
	return &TimeshiftBuffer{
		segments: make([]Segment, 0, maxSegments),
		maxSize:  maxSegments,
		dir:      dir,
	}
}

// WriteSegment adds a segment to the buffer, evicting oldest if full
func (b *TimeshiftBuffer) WriteSegment(data []byte, duration time.Duration) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	path := filepath.Join(b.dir, fmt.Sprintf("segment_%d.ts", time.Now().Unix()))
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", err
	}

	seg := Segment{
		Path:      path,
		StartTime: time.Now(),
		Duration:  duration,
	}

	if len(b.segments) >= b.maxSize {
		// Remove oldest
		oldest := b.segments[0]
		if err := os.Remove(oldest.Path); err != nil {
			log.Error().Err(err).Str("path", oldest.Path).Msg("failed to remove old segment")
		}
		b.segments = b.segments[1:]
	}

	b.segments = append(b.segments, seg)
	return path, nil
}

// GetPlaylist returns an HLS playlist for the buffered content
func (b *TimeshiftBuffer) GetPlaylist() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	sb.WriteString("#EXT-X-TARGETDURATION:10\n")
	sb.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")

	for _, seg := range b.segments {
		sb.WriteString(fmt.Sprintf("#EXTINF:%.1f,\n%s\n", seg.Duration.Seconds(), seg.Path))
	}

	return sb.String()
}
