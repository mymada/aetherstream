package performance

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/labstack/echo/v4"
)

func newBrotliWriter(w io.Writer, level int) io.WriteCloser {
	return brotli.NewWriterLevel(w, level)
}

// BrotliMiddleware returns Echo middleware that compresses responses with Brotli
// for clients that accept "br" encoding. Falls back to identity for others.
func BrotliMiddleware(level int) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			if req.Method != http.MethodGet && req.Method != http.MethodHead {
				return next(c)
			}
			accept := req.Header.Get("Accept-Encoding")
			if !strings.Contains(accept, "br") {
				return next(c)
			}

			rec := newResponseRecorder(c.Response().Writer)
			c.Response().Writer = rec
			err := next(c)
			c.Response().Writer = rec.original
			if err != nil {
				rec.flush()
				return err
			}

			body := rec.body.String()
			if len(body) < 256 {
				rec.flush()
				return nil
			}

			var buf bytes.Buffer
			bw := newBrotliWriter(&buf, level)
			if _, err := bw.Write([]byte(body)); err != nil {
				rec.flush()
				return err
			}
			if err := bw.Close(); err != nil {
				rec.flush()
				return err
			}

			c.Response().Header().Set("Content-Encoding", "br")
			c.Response().Header().Set("Vary", "Accept-Encoding")
			c.Response().Header().Del("Content-Length")
			if rec.status != 0 {
				c.Response().WriteHeader(rec.status)
			}
			_, err = c.Response().Write(buf.Bytes())
			return err
		}
	}
}

// responseRecorder captures response body for compression.
type responseRecorder struct {
	original http.ResponseWriter
	status   int
	header   http.Header
	body     *strings.Builder
	flushed  bool
	mu       sync.Mutex
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		original: w,
		header:   make(http.Header),
		body:     new(strings.Builder),
	}
}

func (r *responseRecorder) Header() http.Header {
	if r.original != nil {
		return r.original.Header()
	}
	return r.header
}

func (r *responseRecorder) WriteHeader(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = status
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(p)
}

func (r *responseRecorder) flush() {
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
