package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/crypto/acme/autocert"
)

// SecurityHeaders middleware adds OWASP recommended headers
func SecurityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Content-Type-Options", "nosniff")
			c.Response().Header().Set("X-Frame-Options", "DENY")
			c.Response().Header().Set("X-XSS-Protection", "1; mode=block")
			c.Response().Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			c.Response().Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'")
			return next(c)
		}
	}
}

// --- Rate Limiting ---

type ipBucket struct {
	tokens    float64
	lastSeen  time.Time
	capacity  float64
	refillRate float64
}

type ipRateLimiter struct {
	mu       sync.RWMutex
	buckets  map[string]*ipBucket
	capacity float64
	refill   float64
}

func newIPRateLimiter(capacity, refillPerMin float64) *ipRateLimiter {
	rl := &ipRateLimiter{
		buckets:  make(map[string]*ipBucket),
		capacity: capacity,
		refill:   refillPerMin / 60.0,
	}
	go rl.cleanup()
	return rl
}

func (rl *ipRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[ip]
	if !ok {
		b = &ipBucket{
			tokens:     rl.capacity,
			lastSeen:   now,
			capacity:   rl.capacity,
			refillRate: rl.refill,
		}
		rl.buckets[ip] = b
	}

	elapsed := now.Sub(b.lastSeen).Seconds()
	b.lastSeen = now
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (rl *ipRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, b := range rl.buckets {
			if now.Sub(b.lastSeen) > 10*time.Minute {
				delete(rl.buckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimitByIP returns middleware that rate-limits by IP with configurable limits
func RateLimitByIP(requestsPerMin int) echo.MiddlewareFunc {
	limiter := newIPRateLimiter(float64(requestsPerMin), float64(requestsPerMin))
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			if !limiter.allow(ip) {
				return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded")
			}
			return next(c)
		}
	}
}

// --- CORS ---

// CORSMiddleware returns Echo CORS middleware configured for AetherStream Web UI
func CORSMiddleware() echo.MiddlewareFunc {
	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000", "http://localhost:8080", "https://localhost:5173", "https://localhost:8080", "*"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions, http.MethodPatch},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, echo.HeaderXRequestedWith},
		ExposeHeaders:    []string{echo.HeaderContentLength, echo.HeaderContentType},
		AllowCredentials: true,
		MaxAge:           86400,
	})
}

// --- Let's Encrypt / TLS ---

// AutoTLSManager creates an autocert.Manager for Let's Encrypt
func AutoTLSManager(domains []string, cacheDir string) *autocert.Manager {
	if cacheDir == "" {
		cacheDir = "./certs"
	}
	return &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domains...),
		Cache:      autocert.DirCache(cacheDir),
	}
}
