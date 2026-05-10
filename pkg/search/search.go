package search

import (
	"github.com/devuser/aetherstream/pkg/db"
)

// Searcher wraps DB FTS5 search.
type Searcher struct {
	db *db.DB
}

// NewSearcher creates a new search service.
func NewSearcher(database *db.DB) *Searcher {
	return &Searcher{db: database}
}

// SearchItems queries the FTS5 index.
// query: free-text search string.
// mediaType: filter by items.media_type (e.g. "movie", "video"); empty means all types.
// limit: max results; defaults to 20 if <= 0.
func (s *Searcher) SearchItems(query, mediaType string, limit int) ([]db.Item, error) {
	return s.db.SearchItemsFTS(query, mediaType, limit)
}
