package library

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devuser/aetherstream/pkg/db"
	"github.com/devuser/aetherstream/pkg/metadata"
)

// CollectionEngine generates automatic collections from library items
type CollectionEngine struct {
	db        *db.DB
	tmdb      *metadata.TMDbClient
	tvdb      *metadata.TVDbClient
}

// NewCollectionEngine creates a collection engine
func NewCollectionEngine(database *db.DB, tmdb *metadata.TMDbClient, tvdb *metadata.TVDbClient) *CollectionEngine {
	return &CollectionEngine{
		db:   database,
		tmdb: tmdb,
		tvdb: tvdb,
	}
}

// CollectionType represents the type of auto-collection
type CollectionType string

const (
	CollectionByGenre    CollectionType = "genre"
	CollectionByYear     CollectionType = "year"
	CollectionByActor    CollectionType = "actor"
	CollectionByDirector CollectionType = "director"
	CollectionByStudio   CollectionType = "studio"
)

// AutoCollection represents an automatically generated collection
type AutoCollection struct {
	Name     string   `json:"name"`
	Type     CollectionType `json:"type"`
	ItemIDs  []string `json:"item_ids"`
	PosterURL string  `json:"poster_url,omitempty"`
}

// GenerateCollections scans all items and creates auto-collections
func (e *CollectionEngine) GenerateCollections(ctx context.Context) ([]AutoCollection, error) {
	items, err := e.db.ListItemsWithLimit(10000)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}

	// Group by genre (from TMDb metadata if available)
	genreGroups := make(map[string][]string)
	yearGroups := make(map[string][]string)

	for _, item := range items {
		// Year-based collection
		if item.Year > 0 {
			decade := fmt.Sprintf("%ds", (item.Year/10)*10)
			yearGroups[decade] = append(yearGroups[decade], item.ID)
		}

		// Genre-based collection (from metadata if enriched)
		if item.Genre != "" {
			genres := strings.Split(item.Genre, ",")
			for _, g := range genres {
				g = strings.TrimSpace(g)
				if g != "" {
					genreGroups[g] = append(genreGroups[g], item.ID)
				}
			}
		}
	}

	var collections []AutoCollection

	// Create genre collections (min 3 items)
	for genre, ids := range genreGroups {
		if len(ids) >= 3 {
			collections = append(collections, AutoCollection{
				Name:    genre,
				Type:    CollectionByGenre,
				ItemIDs: ids,
			})
		}
	}

	// Create decade collections (min 3 items)
	for decade, ids := range yearGroups {
		if len(ids) >= 3 {
			collections = append(collections, AutoCollection{
				Name:    decade,
				Type:    CollectionByYear,
				ItemIDs: ids,
			})
		}
	}

	return collections, nil
}

// EnrichItemMetadata fetches TMDb/TVDb metadata for an item and updates the DB
func (e *CollectionEngine) EnrichItemMetadata(ctx context.Context, item *db.Item) error {
	if e.tmdb == nil {
		return nil
	}

	// Try TMDb movie search
	result, err := e.tmdb.SearchMovie(item.Name, item.Year)
	if err != nil {
		return fmt.Errorf("tmdb search: %w", err)
	}

	if result == nil {
		return nil
	}

	// Fetch full details
	details, err := e.tmdb.GetMovieDetails(result.ID)
	if err != nil {
		return fmt.Errorf("tmdb details: %w", err)
	}

	// Update item with metadata
	item.Overview = details.Overview
	item.PosterURL = metadata.PosterURL(details.PosterPath, "w500")
	item.BackdropURL = metadata.PosterURL(details.BackdropPath, "original")
	item.VoteAverage = details.VoteAverage
	item.Runtime = details.Runtime

	// Build genre string
	var genres []string
	for _, g := range details.Genres {
		genres = append(genres, g.Name)
	}
	item.Genre = strings.Join(genres, ", ")

	// Save to DB
	if err := e.db.UpdateItemMetadata(item); err != nil {
		return fmt.Errorf("update item: %w", err)
	}

	return nil
}

// ScheduleEnrichment runs metadata enrichment for all unenriched items
func (e *CollectionEngine) ScheduleEnrichment(ctx context.Context, batchSize int) error {
	items, err := e.db.ListUnenrichedItems(batchSize)
	if err != nil {
		return fmt.Errorf("list unenriched: %w", err)
	}

	for _, item := range items {
		if err := e.EnrichItemMetadata(ctx, &item); err != nil {
			// Log but continue
			continue
		}
		time.Sleep(250 * time.Millisecond) // Rate limit
	}

	return nil
}
