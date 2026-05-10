package metadata

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/devuser/aetherstream/pkg/cache"
)

// MusicBrainzBaseURL is the public API endpoint
const MusicBrainzBaseURL = "https://musicbrainz.org/ws/2"

// MusicBrainzClient fetches music metadata from MusicBrainz
// with built-in rate limiting (1 req/sec) and an in-memory cache.
type MusicBrainzClient struct {
	client  *http.Client
	baseURL string
	mu      sync.Mutex
	lastReq time.Time

	cache   cache.Cache
}

// NewMusicBrainzClient creates a new MusicBrainz API client.
func NewMusicBrainzClient() *MusicBrainzClient {
	return &MusicBrainzClient{
		client:  &http.Client{Timeout: 15 * time.Second},
		baseURL: MusicBrainzBaseURL,
		cache:   cache.NewLRUCache(500),
	}
}

// SetCache allows injecting a custom cache implementation (e.g. for tests).
func (c *MusicBrainzClient) SetCache(cc cache.Cache) {
	c.cache = cc
}

// rateLimit enforces the MusicBrainz rate limit of 1 request per second.
func (c *MusicBrainzClient) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	elapsed := time.Since(c.lastReq)
	if elapsed < time.Second {
		time.Sleep(time.Second - elapsed)
	}
	c.lastReq = time.Now()
}

func (c *MusicBrainzClient) getCached(key string) (interface{}, bool) {
	return c.cache.Get(key)
}

func (c *MusicBrainzClient) setCache(key string, value interface{}, ttl time.Duration) {
	c.cache.Set(key, value, ttl)
}

func (c *MusicBrainzClient) doRequest(reqURL string) ([]byte, error) {
	c.rateLimit()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AetherStream/1.0 ( https://github.com/devuser/aetherstream )")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("musicbrainz %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// --- Domain structs ---

// MBArtist represents a MusicBrainz artist.
type MBArtist struct {
	ID          string   `json:"id" xml:"id,attr"`
	Name        string   `json:"name" xml:"name"`
	SortName    string   `json:"sort-name" xml:"sort-name"`
	Type        string   `json:"type" xml:"type,attr"`
	Country     string   `json:"country" xml:"country"`
	Disambiguation string `json:"disambiguation" xml:"disambiguation"`
	Tags        []MBTag  `json:"tags" xml:"tag-list>tag"`
}

// MBTag is a genre/style tag.
type MBTag struct {
	Name  string `json:"name" xml:"name"`
	Count int    `json:"count" xml:"count,attr"`
}

// MBRelease (album) represents a MusicBrainz release.
type MBRelease struct {
	ID           string         `json:"id" xml:"id,attr"`
	Title        string         `json:"title" xml:"title"`
	Date         string         `json:"date" xml:"date"`
	Country      string         `json:"country" xml:"country"`
	TrackCount   int            `json:"track-count" xml:"track-count,attr"`
	ArtistCredit []MBNameCredit `json:"artist-credit" xml:"artist-credit>name-credit"`
	Media        []MBMedium     `json:"media" xml:"medium-list>medium"`
	Tags         []MBTag        `json:"tags" xml:"tag-list>tag"`
}

// MBNameCredit holds artist reference in a release.
type MBNameCredit struct {
	Artist MBArtist `json:"artist" xml:"artist"`
	Name   string   `json:"name" xml:"name,attr"`
}

// MBMedium represents a disc / medium in a release.
type MBMedium struct {
	Position   int       `json:"position" xml:"position"`
	Format     string    `json:"format" xml:"format"`
	TrackCount int       `json:"track-count" xml:"track-count,attr"`
	Tracks     []MBTrack `json:"tracks" xml:"track-list>track"`
}

// MBTrack represents a recording/track.
type MBTrack struct {
	ID        string      `json:"id" xml:"id,attr"`
	Title     string      `json:"title" xml:"title"`
	Position  int         `json:"position" xml:"position"`
	Length    int         `json:"length" xml:"length"`
	Recording MBRecording `json:"recording" xml:"recording"`
}

// MBRecording is the underlying recording entity.
type MBRecording struct {
	ID           string         `json:"id" xml:"id,attr"`
	Title        string         `json:"title" xml:"title"`
	Length       int            `json:"length" xml:"length"`
	ArtistCredit []MBNameCredit `json:"artist-credit" xml:"artist-credit>name-credit"`
}

// --- Search responses (JSON) ---

// mbArtistSearchResponse wraps artist search results.
type mbArtistSearchResponse struct {
	Artists []MBArtist `json:"artists"`
	Count   int        `json:"count"`
}

// mbReleaseSearchResponse wraps release search results.
type mbReleaseSearchResponse struct {
	Releases []MBRelease `json:"releases"`
	Count    int         `json:"count"`
}

// mbRecordingSearchResponse wraps recording search results.
type mbRecordingSearchResponse struct {
	Recordings []MBRecording `json:"recordings"`
	Count      int           `json:"count"`
}

// --- XML wrappers for lookup endpoints ---

type mbXMLArtist struct {
	XMLName xml.Name `xml:"metadata"`
	Artist  MBArtist `xml:"artist"`
}

type mbXMLRelease struct {
	XMLName xml.Name  `xml:"metadata"`
	Release MBRelease `xml:"release"`
}

type mbXMLRecording struct {
	XMLName   xml.Name    `xml:"metadata"`
	Recording MBRecording `xml:"recording"`
}

// --- Public API ---

// SearchArtist searches for artists by name.
func (c *MusicBrainzClient) SearchArtist(name string, limit int) ([]MBArtist, error) {
	if limit <= 0 {
		limit = 5
	}
	cacheKey := cache.MetadataKey("artist_search", name+":"+fmt.Sprintf("%d", limit))
	if cached, ok := c.getCached(cacheKey); ok {
		return cached.([]MBArtist), nil
	}

	q := url.Values{}
	q.Set("query", fmt.Sprintf(`artist:"%s"`, name))
	q.Set("fmt", "json")
	q.Set("limit", fmt.Sprintf("%d", limit))

	u := fmt.Sprintf("%s/artist/?%s", c.baseURL, q.Encode())
	body, err := c.doRequest(u)
	if err != nil {
		return nil, err
	}

	var resp mbArtistSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("musicbrainz artist decode: %w", err)
	}

	c.setCache(cacheKey, resp.Artists, 1*time.Hour)
	return resp.Artists, nil
}

// SearchRelease searches for releases (albums) by title.
func (c *MusicBrainzClient) SearchRelease(title string, limit int) ([]MBRelease, error) {
	if limit <= 0 {
		limit = 5
	}
	cacheKey := cache.MetadataKey("release_search", title+":"+fmt.Sprintf("%d", limit))
	if cached, ok := c.getCached(cacheKey); ok {
		return cached.([]MBRelease), nil
	}

	q := url.Values{}
	q.Set("query", fmt.Sprintf(`release:"%s"`, title))
	q.Set("fmt", "json")
	q.Set("limit", fmt.Sprintf("%d", limit))

	u := fmt.Sprintf("%s/release/?%s", c.baseURL, q.Encode())
	body, err := c.doRequest(u)
	if err != nil {
		return nil, err
	}

	var resp mbReleaseSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("musicbrainz release decode: %w", err)
	}

	c.setCache(cacheKey, resp.Releases, 1*time.Hour)
	return resp.Releases, nil
}

// SearchTrack searches for recordings/tracks by title.
func (c *MusicBrainzClient) SearchTrack(title string, limit int) ([]MBRecording, error) {
	if limit <= 0 {
		limit = 5
	}
	cacheKey := cache.MetadataKey("track_search", title+":"+fmt.Sprintf("%d", limit))
	if cached, ok := c.getCached(cacheKey); ok {
		return cached.([]MBRecording), nil
	}

	q := url.Values{}
	q.Set("query", fmt.Sprintf(`recording:"%s"`, title))
	q.Set("fmt", "json")
	q.Set("limit", fmt.Sprintf("%d", limit))

	u := fmt.Sprintf("%s/recording/?%s", c.baseURL, q.Encode())
	body, err := c.doRequest(u)
	if err != nil {
		return nil, err
	}

	var resp mbRecordingSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("musicbrainz recording decode: %w", err)
	}

	c.setCache(cacheKey, resp.Recordings, 1*time.Hour)
	return resp.Recordings, nil
}

// GetArtist fetches full artist details by MBID (XML endpoint).
func (c *MusicBrainzClient) GetArtist(mbid string) (*MBArtist, error) {
	cacheKey := cache.MetadataKey("artist", mbid)
	if cached, ok := c.getCached(cacheKey); ok {
		return cached.(*MBArtist), nil
	}

	u := fmt.Sprintf("%s/artist/%s?inc=tags", c.baseURL, mbid)
	body, err := c.doRequest(u)
	if err != nil {
		return nil, err
	}

	var wrapper mbXMLArtist
	if err := xml.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("musicbrainz artist xml decode: %w", err)
	}

	c.setCache(cacheKey, &wrapper.Artist, 1*time.Hour)
	return &wrapper.Artist, nil
}

// GetRelease fetches full release (album) details by MBID with tracks (XML endpoint).
func (c *MusicBrainzClient) GetRelease(mbid string) (*MBRelease, error) {
	cacheKey := cache.MetadataKey("release", mbid)
	if cached, ok := c.getCached(cacheKey); ok {
		return cached.(*MBRelease), nil
	}

	u := fmt.Sprintf("%s/release/%s?inc=recordings+tags+artist-credits", c.baseURL, mbid)
	body, err := c.doRequest(u)
	if err != nil {
		return nil, err
	}

	var wrapper mbXMLRelease
	if err := xml.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("musicbrainz release xml decode: %w", err)
	}

	c.setCache(cacheKey, &wrapper.Release, 1*time.Hour)
	return &wrapper.Release, nil
}

// GetRecording fetches full recording details by MBID (XML endpoint).
func (c *MusicBrainzClient) GetRecording(mbid string) (*MBRecording, error) {
	cacheKey := cache.MetadataKey("recording", mbid)
	if cached, ok := c.getCached(cacheKey); ok {
		return cached.(*MBRecording), nil
	}

	u := fmt.Sprintf("%s/recording/%s?inc=artist-credits+tags", c.baseURL, mbid)
	body, err := c.doRequest(u)
	if err != nil {
		return nil, err
	}

	var wrapper mbXMLRecording
	if err := xml.Unmarshal(body, &wrapper); err != nil {
		return nil, fmt.Errorf("musicbrainz recording xml decode: %w", err)
	}

	c.setCache(cacheKey, &wrapper.Recording, 1*time.Hour)
	return &wrapper.Recording, nil
}

// --- Helpers ---

// CoverArtURL returns the Cover Art Archive URL for a release MBID.
func CoverArtURL(releaseMBID string) string {
	if releaseMBID == "" {
		return ""
	}
	return fmt.Sprintf("https://coverartarchive.org/release/%s/front-250", releaseMBID)
}

// JoinArtistNames extracts artist names from artist-credit slice.
func JoinArtistNames(credits []MBNameCredit) string {
	var names []string
	for _, c := range credits {
		n := strings.TrimSpace(c.Name)
		if n == "" {
			n = strings.TrimSpace(c.Artist.Name)
		}
		if n != "" {
			names = append(names, n)
		}
	}
	return strings.Join(names, ", ")
}
