package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/devuser/aetherstream/pkg/cache"
)

// TVDbClient fetches TV series metadata from TheTVDB
type TVDbClient struct {
	apiKey  string
	client  *http.Client
	baseURL string
	cache   cache.Cache
	token   string
	tokenExpiry time.Time
}

// NewTVDbClient creates a TVDb API client
func NewTVDbClient(apiKey string) *TVDbClient {
	return &TVDbClient{
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: "https://api4.thetvdb.com/v4",
		cache:   cache.NewLRUCache(500),
	}
}

// SetCache allows injecting a custom cache implementation.
func (c *TVDbClient) SetCache(cc cache.Cache) {
	c.cache = cc
}

// authenticate fetches a JWT token from TVDb
func (c *TVDbClient) authenticate() error {
	if c.token != "" && time.Now().Before(c.tokenExpiry) {
		return nil
	}

	reqBody, _ := json.Marshal(map[string]string{"apikey": c.apiKey})
	resp, err := c.client.Post(c.baseURL+"/login", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("tvdb auth: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("tvdb auth %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("tvdb auth decode: %w", err)
	}

	c.token = result.Data.Token
	c.tokenExpiry = time.Now().Add(23 * time.Hour)
	return nil
}

// TVDbSeriesResult represents a TV series search result
type TVDbSeriesResult struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Overview    string `json:"overview"`
	FirstAired  string `json:"firstAired"`
	Image       string `json:"image"`
	Year        string `json:"year"`
}

// TVDbSearchResponse wraps search results
type TVDbSearchResponse struct {
	Data []TVDbSeriesResult `json:"data"`
}

// TVDbSeriesDetails full series info
type TVDbSeriesDetails struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Overview    string `json:"overview"`
	FirstAired  string `json:"firstAired"`
	Image       string `json:"image"`
	Genres      []struct {
		Name string `json:"name"`
	} `json:"genres"`
	Episodes []TVDbEpisode `json:"episodes"`
}

// TVDbEpisode represents a single episode
type TVDbEpisode struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Overview    string `json:"overview"`
	SeasonNumber int   `json:"seasonNumber"`
	EpisodeNumber int  `json:"episodeNumber"`
	Aired       string `json:"aired"`
}

// SearchSeries queries TVDb for TV series by name
func (c *TVDbClient) SearchSeries(name string) (*TVDbSeriesResult, error) {
	if err := c.authenticate(); err != nil {
		return nil, err
	}

	cacheKey := cache.MetadataKey("tvdb_search", name)
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached.(*TVDbSeriesResult), nil
	}

	q := url.Values{}
	q.Set("query", name)
	u := fmt.Sprintf("%s/search?type=series&%s", c.baseURL, q.Encode())

	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tvdb search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tvdb %d: %s", resp.StatusCode, string(body))
	}

	var result TVDbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("tvdb decode: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, nil
	}

	c.cache.Set(cacheKey, &result.Data[0], 1*time.Hour)
	return &result.Data[0], nil
}

// GetSeriesDetails fetches full series metadata by TVDb ID
func (c *TVDbClient) GetSeriesDetails(tvdbID int) (*TVDbSeriesDetails, error) {
	if err := c.authenticate(); err != nil {
		return nil, err
	}

	cacheKey := cache.MetadataKey("tvdb_series", fmt.Sprintf("%d", tvdbID))
	if cached, ok := c.cache.Get(cacheKey); ok {
		return cached.(*TVDbSeriesDetails), nil
	}

	u := fmt.Sprintf("%s/series/%d/extended", c.baseURL, tvdbID)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tvdb details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tvdb %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data TVDbSeriesDetails `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("tvdb decode: %w", err)
	}

	c.cache.Set(cacheKey, &result.Data, 1*time.Hour)
	return &result.Data, nil
}
