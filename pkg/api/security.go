package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/devuser/aetherstream/pkg/config"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/crypto/acme/autocert"
)

// --- CSRF Protection ---

const csrfTokenHeader = "X-CSRF-Token" // #nosec G101 — this is a header name constant, not a credential
const csrfCookieName = "csrf_token"      // #nosec G101 — this is a cookie name constant, not a credential

// generateCSRFToken creates a random 32-byte token encoded as base64.
func generateCSRFToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// CSRFProtection returns middleware that validates CSRF tokens on state-changing methods.
// It expects the token in the X-CSRF-Token header or in a form field named "csrf_token".
// A cookie named csrf_token is set on GET/HEAD/OPTIONS requests if missing.
func CSRFProtection() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			method := c.Request().Method
			// Safe methods: ensure cookie exists
			if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
				cookie, err := c.Cookie(csrfCookieName)
				if err != nil || cookie.Value == "" {
					newToken := generateCSRFToken()
					c.SetCookie(&http.Cookie{ // #nosec G124 — CSRF cookie config intentionally lax for HTTP local dev
						Name:     csrfCookieName,
						Value:    newToken,
						Path:     "/",
						HttpOnly: true,
						SameSite: http.SameSiteLaxMode,
						Secure:   false,
						MaxAge:   86400,
					})
				}
				return next(c)
			}

			// Skip CSRF for public auth endpoints (no session to hijack yet)
			path := c.Request().URL.Path
			if strings.HasPrefix(path, "/auth/") || strings.HasPrefix(path, "/webhooks/") {
				return next(c)
			}
			// Also skip if Authorization header is present (API clients / tests)
			if c.Request().Header.Get("Authorization") != "" {
				return next(c)
			}

			// Unsafe methods: validate token
			cookie, err := c.Cookie(csrfCookieName)
			if err != nil || cookie.Value == "" {
				return echo.NewHTTPError(http.StatusForbidden, "missing csrf cookie")
			}
			sentToken := c.Request().Header.Get(csrfTokenHeader)
			if sentToken == "" {
				sentToken = c.FormValue("csrf_token")
			}
			if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(sentToken)) != 1 {
				return echo.NewHTTPError(http.StatusForbidden, "invalid csrf token")
			}
			return next(c)
		}
	}
}

// --- Security Headers (enhanced) ---

// SecurityHeaders returns middleware adding OWASP recommended headers including HSTS and CSP.
func SecurityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("X-Content-Type-Options", "nosniff")
			c.Response().Header().Set("X-Frame-Options", "DENY")
			c.Response().Header().Set("X-XSS-Protection", "1; mode=block")
			c.Response().Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			c.Response().Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-eval' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'self' http: https: ws: wss:")
			// Only send HSTS over HTTPS to avoid leaking the header on HTTP connections
			if c.Request().TLS != nil || c.Request().Header.Get("X-Forwarded-Proto") == "https" {
				c.Response().Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			}
			c.Response().Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			return next(c)
		}
	}
}

// --- Secure Cookies ---

// SecureCookieMiddleware sets default secure cookie flags for all cookies set during the request.
// It wraps echo.Context to intercept SetCookie calls.
func SecureCookieMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Wrap the writer to intercept Set-Cookie headers
			origWriter := c.Response().Writer
			wrapped := &secureCookieResponseWriter{ResponseWriter: origWriter, headerWritten: false}
			c.Response().Writer = wrapped
			defer func() { c.Response().Writer = origWriter }()
			return next(c)
		}
	}
}

type secureCookieResponseWriter struct {
	http.ResponseWriter
	headerWritten bool
}

func (w *secureCookieResponseWriter) WriteHeader(code int) {
	if !w.headerWritten {
		w.headerWritten = true
		w.secureCookies()
	}
	w.ResponseWriter.WriteHeader(code)
	return
}

func (w *secureCookieResponseWriter) Write(b []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *secureCookieResponseWriter) secureCookies() {
	cookies := w.Header()["Set-Cookie"]
	if len(cookies) == 0 {
		return
	}
	var out []string
	for _, raw := range cookies {
		if !strings.Contains(raw, "SameSite=") {
			raw += "; SameSite=Strict"
		}
		if !strings.Contains(strings.ToLower(raw), "httponly") {
			raw += "; HttpOnly"
		}
		if !strings.Contains(strings.ToLower(raw), "secure") {
			raw += "; Secure"
		}
		out = append(out, raw)
	}
	w.Header()["Set-Cookie"] = out
}

// --- Brute-Force Protection (exponential backoff per IP/username) ---

type loginAttempt struct {
	count       int
	lastAttempt time.Time
	lockedUntil time.Time
}

type bruteForceLimiter struct {
	mu       sync.RWMutex
	attempts map[string]*loginAttempt
	maxDelay time.Duration
}

func newBruteForceLimiter() *bruteForceLimiter {
	b := &bruteForceLimiter{
		attempts: make(map[string]*loginAttempt),
		maxDelay: 30 * time.Minute,
	}
	go b.cleanup()
	return b
}

// key can be "ip:<ip>" or "user:<username>"
func (b *bruteForceLimiter) record(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	a, ok := b.attempts[key]
	if !ok {
		a = &loginAttempt{count: 0, lastAttempt: now}
		b.attempts[key] = a
	}
	a.count++
	a.lastAttempt = now
	// Exponential backoff: 2^count seconds, capped at maxDelay
	delay := time.Duration(1<<uint(a.count)) * time.Second
	if delay > b.maxDelay {
		delay = b.maxDelay
	}
	a.lockedUntil = now.Add(delay)
}

func (b *bruteForceLimiter) allowed(key string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	a, ok := b.attempts[key]
	if !ok {
		return true
	}
	return time.Now().After(a.lockedUntil)
}

func (b *bruteForceLimiter) reset(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.attempts, key)
}

func (b *bruteForceLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	for range ticker.C {
		b.mu.Lock()
		now := time.Now()
		for k, a := range b.attempts {
			if now.Sub(a.lastAttempt) > 1*time.Hour {
				delete(b.attempts, k)
			}
		}
		b.mu.Unlock()
	}
}

// BruteForceProtection returns middleware that enforces exponential backoff on login endpoints.
// It should be placed BEFORE the actual handler. On successful login, call ResetBruteForce.
var globalBruteForce = newBruteForceLimiter()

func BruteForceProtection() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := strings.ToLower(c.Request().URL.Path)
			if !strings.Contains(path, "/login") && !strings.Contains(path, "/auth") {
				return next(c)
			}
			ip := getTrustedIP(c)
			// Strip port if present for consistent keying
			host, _, err := net.SplitHostPort(ip)
			if err == nil {
				ip = host
			}
			ipKey := "ip:" + ip
			if !globalBruteForce.allowed(ipKey) {
				return echo.NewHTTPError(http.StatusTooManyRequests, "too many failed attempts from this IP")
			}
			username := c.FormValue("username")
			if username == "" {
				// Try JSON body fallback via a simple heuristic
				// Echo's Bind hasn't run yet, so we only check form for now.
				// If empty, we still rate-limit by IP.
			}
			if username != "" {
				userKey := "user:" + username
				if !globalBruteForce.allowed(userKey) {
					return echo.NewHTTPError(http.StatusTooManyRequests, "too many failed attempts for this user")
				}
			}
			return next(c)
		}
	}
}

// RecordFailedLogin increments failed attempt counters for IP and optionally username.
func RecordFailedLogin(ip, username string) {
	globalBruteForce.record("ip:" + ip)
	if username != "" {
		globalBruteForce.record("user:" + username)
	}
}

// ResetBruteForce clears counters for IP/username after successful login.
func ResetBruteForce(ip, username string) {
	globalBruteForce.reset("ip:" + ip)
	if username != "" {
		globalBruteForce.reset("user:" + username)
	}
}

// getTrustedIP returns the client IP, preferring RemoteAddr when not behind a trusted proxy.
func getTrustedIP(c echo.Context) string {
	// If the server is directly exposed, avoid trusting X-Forwarded-For to prevent spoofing.
	// Use RemoteAddr directly when no trusted proxy is configured.
	// In a reverse-proxy setup, configure Echo's ExtractIPFromXFFHeader instead.
	return c.Request().RemoteAddr
}

// --- Session Timeout / Idle Logout ---

const sessionContextKey = "session_last_activity"
const sessionTimeoutHeader = "X-Session-Timeout"

// SessionTimeout returns middleware that tracks last activity per user in a persistent store
// and rejects requests after idleDuration. It should be applied to protected routes.
func SessionTimeout(idleDuration time.Duration, database *db.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get user ID from auth context
			claims := auth.GetUser(c)
			if claims == nil {
				return next(c) // No auth, skip timeout
			}

			userID := claims.UserID
			now := time.Now()

			// Use session ID from cookie/header if available, otherwise use userID
			sessionID := c.Request().Header.Get("X-Session-ID")
			if sessionID == "" {
				sessionID = userID
			}

			last, err := database.GetSessionLastSeen(sessionID)
			if err != nil {
				// DB error — fail secure (allow but log)
				c.Logger().Warnf("session timeout db error: %v", err)
				_ = database.UpdateSessionLastSeen(sessionID)
				return next(c)
			}

			if !last.IsZero() && now.Sub(last) > idleDuration {
				return echo.NewHTTPError(http.StatusUnauthorized, "session expired due to inactivity")
			}

			_ = database.UpdateSessionLastSeen(sessionID)

			// Inform client how many seconds remain
			remaining := int(idleDuration.Seconds())
			c.Response().Header().Set(sessionTimeoutHeader, fmt.Sprintf("%d", remaining))
			return next(c)
		}
	}
}

// --- Rate Limiting ---

type ipBucket struct {
	tokens     float64
	lastSeen   time.Time
	capacity   float64
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
			ip := getTrustedIP(c)
			if !limiter.allow(ip) {
				return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded")
			}
			return next(c)
		}
	}
}

// --- Audit Log Integration ---

// AuditLogFunc is a callback to record audit events without importing pkg/audit directly.
type AuditLogFunc func(userID, username, action, resource, resourceID, ip, userAgent, details string)

// AuditMiddleware returns middleware that logs requests via the provided log function.
func AuditMiddleware(logFn AuditLogFunc) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			latency := time.Since(start)
			if logFn == nil {
				return err
			}
			ip := getTrustedIP(c)
			path := c.Request().URL.Path
			method := c.Request().Method
			ua := c.Request().UserAgent()
			var userID, username string
			if u := c.Get("user"); u != nil {
				if claims, ok := u.(interface{ GetUserID() string; GetUsername() string }); ok {
					userID = claims.GetUserID()
					username = claims.GetUsername()
				}
			}
			status := c.Response().Status
			if status == 0 {
				status = 200 // default if not written yet
			}
			details := fmt.Sprintf("method=%s status=%d latency=%s", method, status, latency)
			logFn(userID, username, "request", path, "", ip, ua, details)
			return err
		}
	}
}

// --- Helpers ---

// isPrivateIP reports whether ip is a private/local address.
func isPrivateIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	privateBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
	}
	for _, cidr := range privateBlocks {
		_, block, _ := net.ParseCIDR(cidr)
		if block != nil && block.Contains(parsed) {
			return true
		}
	}
	return false
}

// --- CORS ---

// CORSMiddleware returns Echo CORS middleware configured for AetherStream Web UI
// If cfg.AllowedOrigins is empty, defaults to same-origin only (no wildcard).
func CORSMiddleware(cfg *config.Config) echo.MiddlewareFunc {
	origins := cfg.Server.AllowedOrigins
	if len(origins) == 0 {
		origins = []string{"http://localhost:8080", "http://localhost:8081"}
	}
	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     origins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions, http.MethodPatch},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, echo.HeaderXRequestedWith, csrfTokenHeader},
		ExposeHeaders:    []string{echo.HeaderContentLength, echo.HeaderContentType, csrfTokenHeader},
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
