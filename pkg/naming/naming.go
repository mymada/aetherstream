package naming

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ParseResult contains extracted metadata from filename
type ParseResult struct {
	Kind        string // movie, episode, music, unknown
	Title       string
	Year        int
	Season      int
	Episode     int
	EpisodeEnd  int    // for multi-episode files
	Artist      string
	Album       string
	TrackNumber int
}

var (
	// Movie patterns: Movie.Name.2024.1080p.mkv, Movie Name (2024).mkv, Movie.Name.(2024).mkv
	movieYearRe = regexp.MustCompile(`[\s\.\(\[](\d{4})[\s\.\)\]]`)
	movieYearDotRe = regexp.MustCompile(`\.(\d{4})\.`)  // The.Matrix.1999.1080p
	
	// TV patterns: Show.Name.S01E02.mkv, Show.Name.1x02.mkv, Show.Name.S01E02E03.mkv
	seasonEpisodeRe = regexp.MustCompile(`[Ss](\d{1,2})[Ee](\d{1,2})(?:[Ee](\d{1,2}))?`)
	seasonEpisodeXRe = regexp.MustCompile(`(\d{1,2})x(\d{1,2})(?:-(\d{1,2}))?`)
	
	// Music patterns: 01 - Artist - Title.mp3, Artist - Album - 01 - Title.mp3
	trackNumberRe = regexp.MustCompile(`^(\d{1,3})\s*[-–.]\s*`)
)

// ParseFilename analyzes a filename and returns structured metadata
func ParseFilename(path string) *ParseResult {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	
	// Try parsing BEFORE cleaning (dots are meaningful for year detection)
	// Try TV episode first (most specific)
	if r := parseEpisode(name); r != nil {
		r.Title = cleanName(r.Title)
		return r
	}

	// Try movie
	if r := parseMovie(name); r != nil {
		r.Title = cleanName(r.Title)
		return r
	}

	// Try music
	if r := parseMusic(name); r != nil {
		r.Title = cleanName(r.Title)
		return r
	}

	return &ParseResult{Kind: "unknown", Title: cleanName(name)}
}

func parseEpisode(name string) *ParseResult {
	// Pattern: S01E02 or S01E02E03 (multi-episode)
	m := seasonEpisodeRe.FindStringSubmatch(name)
	if m != nil {
		season, _ := strconv.Atoi(m[1])
		episode, _ := strconv.Atoi(m[2])
		title := extractTitle(name, m[0])
		
		r := &ParseResult{
			Kind:    "episode",
			Title:   title,
			Season:  season,
			Episode: episode,
		}
		if m[3] != "" {
			r.EpisodeEnd, _ = strconv.Atoi(m[3])
		}
		return r
	}

	// Pattern: 1x02 or 1x02-03
	m = seasonEpisodeXRe.FindStringSubmatch(name)
	if m != nil {
		season, _ := strconv.Atoi(m[1])
		episode, _ := strconv.Atoi(m[2])
		title := extractTitle(name, m[0])
		
		r := &ParseResult{
			Kind:    "episode",
			Title:   title,
			Season:  season,
			Episode: episode,
		}
		if m[3] != "" {
			r.EpisodeEnd, _ = strconv.Atoi(m[3])
		}
		return r
	}

	return nil
}

func parseMovie(name string) *ParseResult {
	// Try dot pattern first: Movie.Name.1999.1080p
	m := movieYearDotRe.FindStringSubmatch(name)
	if m != nil {
		year, _ := strconv.Atoi(m[1])
		if year >= 1900 && year <= 2030 {
			title := extractTitle(name, m[0])
			return &ParseResult{
				Kind:  "movie",
				Title: title,
				Year:  year,
			}
		}
	}

	// Try bracket/paren pattern: Movie Name (2024)
	m = movieYearRe.FindStringSubmatch(name)
	if m != nil {
		year, _ := strconv.Atoi(m[1])
		if year >= 1900 && year <= 2030 {
			title := extractTitle(name, m[0])
			return &ParseResult{
				Kind:  "movie",
				Title: title,
				Year:  year,
			}
		}
	}
	return nil
}

func parseMusic(name string) *ParseResult {
	m := trackNumberRe.FindStringSubmatch(name)
	if m != nil {
		track, _ := strconv.Atoi(m[1])
		title := strings.TrimSpace(name[len(m[0]):])
		return &ParseResult{
			Kind:        "music",
			Title:       title,
			TrackNumber: track,
		}
	}
	return nil
}

func extractTitle(name, pattern string) string {
	idx := strings.Index(name, pattern)
	if idx > 0 {
		title := name[:idx]
		return cleanName(title)
	}
	return cleanName(name)
}

func cleanName(name string) string {
	// Replace dots with spaces
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	
	// Remove common release tags
	tags := []string{
		"1080p", "720p", "480p", "2160p", "4K", "UHD", "HDR", "DV",
		"BluRay", "Blu-Ray", "WEB-DL", "WEBDL", "WEBRip", "HDTV",
		"x264", "x265", "HEVC", "AVC", "AAC", "AC3", "DTS", "DD5.1",
		"REPACK", "PROPER", "EXTENDED", "UNRATED", "DC",
		"H264", "H265", "10bit", "8bit",
	}
	
	for _, tag := range tags {
		name = strings.ReplaceAll(name, tag, "")
		name = strings.ReplaceAll(name, strings.ToLower(tag), "")
		name = strings.ReplaceAll(name, strings.ToUpper(tag), "")
	}
	
	// Clean up multiple spaces
	name = strings.Join(strings.Fields(name), " ")
	name = strings.TrimSpace(name)
	
	return name
}
