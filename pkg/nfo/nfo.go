package nfo

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Movie represents a Kodi/Jellyfin-style movie NFO.
type Movie struct {
	XMLName     xml.Name  `xml:"movie"`
	Title       string    `xml:"title"`
	OriginalTitle string  `xml:"originaltitle,omitempty"`
	SortTitle   string    `xml:"sorttitle,omitempty"`
	Set         string    `xml:"set,omitempty"`
	Rating      float64   `xml:"rating,omitempty"`
	Year        int       `xml:"year,omitempty"`
	Top250      int       `xml:"top250,omitempty"`
	Votes       int       `xml:"votes,omitempty"`
	Outline     string    `xml:"outline,omitempty"`
	Plot        string    `xml:"plot,omitempty"`
	Tagline     string    `xml:"tagline,omitempty"`
	Runtime     int       `xml:"runtime,omitempty"`
	Thumb       string    `xml:"thumb,omitempty"`
	MPAA        string    `xml:"mpaa,omitempty"`
	PlayCount   int       `xml:"playcount,omitempty"`
	LastPlayed  string    `xml:"lastplayed,omitempty"`
	ID          string    `xml:"id,omitempty"`
	Genre       []string  `xml:"genre,omitempty"`
	Tag         []string  `xml:"tag,omitempty"`
	Director    []string  `xml:"director,omitempty"`
	Credits     []string  `xml:"credits,omitempty"`
	Actor       []Actor   `xml:"actor,omitempty"`
	FileInfo    *FileInfo `xml:"fileinfo,omitempty"`
}

// Actor represents an actor in NFO.
type Actor struct {
	Name  string `xml:"name"`
	Role  string `xml:"role,omitempty"`
	Order int    `xml:"order,omitempty"`
	Thumb string `xml:"thumb,omitempty"`
}

// FileInfo holds stream details.
type FileInfo struct {
	StreamDetails StreamDetails `xml:"streamdetails"`
}

// StreamDetails contains video/audio/subtitle info.
type StreamDetails struct {
	Video    []VideoStream    `xml:"video,omitempty"`
	Audio    []AudioStream    `xml:"audio,omitempty"`
	Subtitle []SubtitleStream `xml:"subtitle,omitempty"`
}

// VideoStream describes a video track.
type VideoStream struct {
	Codec        string `xml:"codec,omitempty"`
	Aspect       string `xml:"aspect,omitempty"`
	Width        int    `xml:"width,omitempty"`
	Height       int    `xml:"height,omitempty"`
	DurationInSecs int `xml:"durationinseconds,omitempty"`
}

// AudioStream describes an audio track.
type AudioStream struct {
	Codec    string `xml:"codec,omitempty"`
	Language string `xml:"language,omitempty"`
	Channels int    `xml:"channels,omitempty"`
}

// SubtitleStream describes a subtitle track.
type SubtitleStream struct {
	Language string `xml:"language,omitempty"`
}

// WriteMovieNFO serializes a Movie struct to an .nfo file.
func WriteMovieNFO(path string, movie *Movie) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	out, err := xml.MarshalIndent(movie, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal nfo: %w", err)
	}
	header := []byte(xml.Header + string(out) + "\n")
	return os.WriteFile(path, header, 0644)
}

// ReadMovieNFO parses a movie .nfo file.
func ReadMovieNFO(path string) (*Movie, error) {
	// Security: path should be validated by caller against library directories
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read nfo: %w", err)
	}
	var movie Movie
	if err := xml.Unmarshal(data, &movie); err != nil {
		return nil, fmt.Errorf("unmarshal nfo: %w", err)
	}
	return &movie, nil
}

// NFOPath returns the expected .nfo path for a given media file.
func NFOPath(mediaPath string) string {
	base := strings.TrimSuffix(mediaPath, filepath.Ext(mediaPath))
	return base + ".nfo"
}

// WriteEpisodeNFO serializes an Episode struct to an .nfo file.
func WriteEpisodeNFO(path string, ep *Episode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	out, err := xml.MarshalIndent(ep, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal nfo: %w", err)
	}
	header := []byte(xml.Header + string(out) + "\n")
	return os.WriteFile(path, header, 0644)
}

// ReadEpisodeNFO parses an episode .nfo file.
func ReadEpisodeNFO(path string) (*Episode, error) {
	// Security: path should be validated by caller against library directories
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read nfo: %w", err)
	}
	var ep Episode
	if err := xml.Unmarshal(data, &ep); err != nil {
		return nil, fmt.Errorf("unmarshal nfo: %w", err)
	}
	return &ep, nil
}

// Episode represents a TV episode NFO.
type Episode struct {
	XMLName     xml.Name `xml:"episodedetails"`
	Title       string   `xml:"title"`
	ShowTitle   string   `xml:"showtitle,omitempty"`
	Season      int      `xml:"season,omitempty"`
	Episode     int      `xml:"episode,omitempty"`
	Plot        string   `xml:"plot,omitempty"`
	Thumb       string   `xml:"thumb,omitempty"`
	Rating      float64  `xml:"rating,omitempty"`
	Year        int      `xml:"year,omitempty"`
	Aired       string   `xml:"aired,omitempty"`
	Runtime     int      `xml:"runtime,omitempty"`
	Director    []string `xml:"director,omitempty"`
	Credits     []string `xml:"credits,omitempty"`
	Actor       []Actor  `xml:"actor,omitempty"`
	FileInfo    *FileInfo `xml:"fileinfo,omitempty"`
}

// SimpleMovieFromMap builds a minimal Movie from a flat map (e.g. DB row).
func SimpleMovieFromMap(data map[string]interface{}) *Movie {
	m := &Movie{}
	if v, ok := data["name"].(string); ok {
		m.Title = v
	}
	if v, ok := data["durationSeconds"].(float64); ok {
		m.Runtime = int(v / 60)
	}
	if v, ok := data["width"].(int); ok {
		m.FileInfo = &FileInfo{StreamDetails: StreamDetails{Video: []VideoStream{{Width: v}}}}
	}
	if v, ok := data["height"].(int); ok && m.FileInfo != nil {
		m.FileInfo.StreamDetails.Video[0].Height = v
	}
	m.LastPlayed = time.Now().UTC().Format("2006-01-02")
	return m
}
