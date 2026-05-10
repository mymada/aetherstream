package performance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/devuser/aetherstream/pkg/cache"
)

// etagCacheKey generates a cache key for ETag storage.
func etagCacheKey(path, query string) string {
	return fmt.Sprintf("etag:%s?%s", path, query)
}

// ETagMiddleware returns Echo middleware that generates ETag headers for GET/HEAD responses
// and handles If-None-Match / If-Modified-Since for 304 Not Modified.
// It uses the existing LRU cache to store ETags and last-modified timestamps.
func ETagMiddleware(cache cache.Cache) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			if req.Method != http.MethodGet && req.Method != http.MethodHead {
				return next(c)
			}

			path := req.URL.Path
			query := req.URL.RawQuery
			key := etagCacheKey(path, query)

			// Execute handler first to capture response body for hashing
			rec := newETagRecorder(c.Response().Writer)
			c.Response().Writer = rec
			err := next(c)
			c.Response().Writer = rec.original

			if err != nil || rec.status >= 300 || rec.status == 0 {
				if rec.status == 0 {
					rec.status = http.StatusOK
				}
				// Write buffered data on error / redirect
				rec.flush()
				return err
			}

			if rec.status == 0 {
				rec.status = http.StatusOK
			}

			bodyStr := rec.body.String()
			body := []byte(bodyStr)
			etag := computeETag(body)
			lastMod := time.Now().UTC().Format(http.TimeFormat)

			// Store in cache for 5 minutes
			if cache != nil {
				cache.Set(key, map[string]string{
					"etag":         etag,
					"lastModified": lastMod,
				}, 5*time.Minute)
			}

			// Check conditional headers
			ifNoneMatch := req.Header.Get("If-None-Match")
			ifModifiedSince := req.Header.Get("If-Modified-Since")

			match := false
			if ifNoneMatch != "" {
				match = ifNoneMatch == etag || ifNoneMatch == "*"
			} else if ifModifiedSince != "" {
				if t, err := http.ParseTime(ifModifiedSince); err == nil {
					if time.Now().UTC().Before(t.Add(5 * time.Minute)) {
						match = true
					}
				}
			}
			if match {
				c.Response().Header().Set("ETag", etag)
				c.Response().Header().Set("Last-Modified", lastMod)
				c.Response().WriteHeader(http.StatusNotModified)
				return nil
			}

			c.Response().Header().Set("ETag", etag)
			c.Response().Header().Set("Last-Modified", lastMod)
			c.Response().Header().Set("Cache-Control", "private, must-revalidate")
			rec.flush()
			return nil
		}
	}
}

// etagRecorder captures response body for ETag computation.
type etagRecorder struct {
	original http.ResponseWriter
	status   int
	header   http.Header
	body     *strings.Builder
	flushed  bool
	mu       sync.Mutex
}

func newETagRecorder(w http.ResponseWriter) *etagRecorder {
	return &etagRecorder{
		original: w,
		header:   make(http.Header),
		body:     new(strings.Builder),
	}
}

func (r *etagRecorder) Header() http.Header {
	if r.original != nil {
		return r.original.Header()
	}
	return r.header
}

func (r *etagRecorder) WriteHeader(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = status
}

func (r *etagRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(p)
}

func (r *etagRecorder) flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.flushed {
		return
	}
	r.flushed = true
	if r.status != 0 {
		r.original.WriteHeader(r.status)
	}
	if r.body.Len() > 0 {
		_, _ = io.WriteString(r.original, r.body.String())
	}
}

var etagPool = sync.Pool{
	New: func() interface{} {
		return sha256.New()
	},
}

func computeETag(data []byte) string {
	h := etagPool.Get().(hash.Hash)
	h.Reset()
	_, _ = h.Write(data)
	sum := h.Sum(nil)
	etagPool.Put(h)
	return fmt.Sprintf("\"%s\"", hex.EncodeToString(sum)[:16])
}
