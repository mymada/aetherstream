package metadata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// TMDbClient fetches movie/TV metadata from The Movie Database
type TMDbClient struct {
	apiKey string
	client *http.Client
	baseURL string
}

// NewTMDbClient creates a TMDb API client
func NewTMDbClient(apiKey string) *TMDbClient {
	return &TMDbClient{
		apiKey: apiKey,
		client: &http.Client{Timeout: 10 * time.Second},
		baseURL: "https://api.themoviedb.org/3",
	}
}

// TMDbMovieResult represents a movie search result
type TMDbMovieResult struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Overview    string  `json:"overview"`
	ReleaseDate string  `json:"release_date"`
	PosterPath  string  `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
	VoteAverage float64 `json:"vote_average"`
	GenreIDs    []int   `json:"genre_ids"`
}

// TMDbSearchResponse wraps search results
type TMDbSearchResponse struct {
	Results []TMDbMovieResult `json:"results"`
}

// TMDbMovieDetails full movie info
type TMDbMovieDetails struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Overview    string `json:"overview"`
	ReleaseDate string `json:"release_date"`
	Runtime     int    `json:"runtime"`
	PosterPath  string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
	VoteAverage float64 `json:"vote_average"`
	Genres      []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
}

// SearchMovie queries TMDb for movies by title + year
func (c *TMDbClient) SearchMovie(title string, year int) (*TMDbMovieResult, error) {
	q := url.Values{}
	q.Set("api_key", c.apiKey)
	q.Set("query", title)
	q.Set("language", "fr-FR")
	if year > 0 {
		q.Set("year", fmt.Sprintf("%d", year))
	}

	u := fmt.Sprintf("%s/search/movie?%s", c.baseURL, q.Encode())
	resp, err := c.client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("tmdb search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tmdb %d: %s", resp.StatusCode, string(body))
	}

	var result TMDbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("tmdb decode: %w", err)
	}

	if len(result.Results) == 0 {
		return nil, nil
	}

	return &result.Results[0], nil
}

// GetMovieDetails fetches full movie metadata by TMDb ID
func (c *TMDbClient) GetMovieDetails(tmdbID int) (*TMDbMovieDetails, error) {
	u := fmt.Sprintf("%s/movie/%d?api_key=%s&language=fr-FR", c.baseURL, tmdbID, c.apiKey)
	resp, err := c.client.Get(u)
	if err != nil {
		return nil, fmt.Errorf("tmdb details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tmdb %d: %s", resp.StatusCode, string(body))
	}

	var details TMDbMovieDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, fmt.Errorf("tmdb decode: %w", err)
	}

	return &details, nil
}

// PosterURL returns full poster image URL
func PosterURL(path string, size string) string {
	if path == "" {
		return ""
	}
	if size == "" {
		size = "w500"
	}
	return fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", size, path)
}
