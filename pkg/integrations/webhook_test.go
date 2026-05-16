package integrations

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func setupWebhookServer(t *testing.T) (*ArrService, *echo.Echo) {
	t.Helper()
	svc := NewArrService(ArrConfig{SonarrSecret: "s-secret", RadarrSecret: "r-secret", MediaRoot: "/tmp"})
	e := echo.New()
	svc.RegisterRoutes(e)
	return svc, e
}

func TestRegisterRoutes_HasEndpoints(t *testing.T) {
	_, e := setupWebhookServer(t)
	routes := e.Routes()
	var hasSonarr, hasRadarr bool
	for _, r := range routes {
		if r.Path == "/webhooks/sonarr" {
			hasSonarr = true
		}
		if r.Path == "/webhooks/radarr" {
			hasRadarr = true
		}
	}
	assert.True(t, hasSonarr, "sonarr webhook route missing")
	assert.True(t, hasRadarr, "radarr webhook route missing")
}

// --- Sonarr ---

func TestHandleSonarrWebhook_TestEvent(t *testing.T) {
	_, e := setupWebhookServer(t)
	body := []byte(`{"eventType":"Test"}`)
	sig := computeHMAC(body, "s-secret")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/sonarr", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHandleSonarrWebhook_DownloadEvent(t *testing.T) {
	_, e := setupWebhookServer(t)
	body := []byte(`{"eventType":"Download","series":{"title":"TestShow","path":"/tmp","tvdbId":1}}`)
	sig := computeHMAC(body, "s-secret")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/sonarr", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHandleSonarrWebhook_UnknownEvent(t *testing.T) {
	_, e := setupWebhookServer(t)
	body := []byte(`{"eventType":"Grab"}`)
	sig := computeHMAC(body, "s-secret")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/sonarr", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHandleSonarrWebhook_InvalidSignature(t *testing.T) {
	_, e := setupWebhookServer(t)
	body := []byte(`{"eventType":"Test"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/sonarr", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", "badsig")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandleSonarrWebhook_BadJSON(t *testing.T) {
	_, e := setupWebhookServer(t)
	body := []byte(`not json`)
	sig := computeHMAC(body, "s-secret")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/sonarr", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Radarr ---

func TestHandleRadarrWebhook_TestEvent(t *testing.T) {
	_, e := setupWebhookServer(t)
	body := []byte(`{"eventType":"Test"}`)
	sig := computeHMAC(body, "r-secret")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/radarr", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHandleRadarrWebhook_DownloadEvent(t *testing.T) {
	_, e := setupWebhookServer(t)
	body := []byte(`{"eventType":"Download","movie":{"title":"TestMovie","folderPath":"/tmp","tmdbId":1}}`)
	sig := computeHMAC(body, "r-secret")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/radarr", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHandleRadarrWebhook_InvalidSignature(t *testing.T) {
	_, e := setupWebhookServer(t)
	body := []byte(`{"eventType":"Test"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/radarr", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", "badsig")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandleRadarrWebhook_BadJSON(t *testing.T) {
	_, e := setupWebhookServer(t)
	body := []byte(`not json`)
	sig := computeHMAC(body, "r-secret")
	req := httptest.NewRequest(http.MethodPost, "/webhooks/radarr", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", sig)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- verifySignature edge cases ---

func TestVerifySignature_SecretSetNoSig(t *testing.T) {
	// Secret set but signature absent → accepted (caller didn't send a sig header)
	// The function only validates when BOTH secret AND signature are non-empty.
	assert.True(t, verifySignature([]byte("body"), "secret", ""))
}

func TestVerifySignature_NoSecretNoSig(t *testing.T) {
	// No secret and no signature → accept (open endpoint)
	assert.True(t, verifySignature([]byte("body"), "", ""))
}

// --- processDownload edge cases ---

func TestProcessDownload_EpisodeFile(t *testing.T) {
	svc := NewArrService(ArrConfig{MediaRoot: "/tmp"})
	payload := ArrWebhookPayload{
		Series: &struct {
			Title  string `json:"title"`
			Path   string `json:"path"`
			TVDBID int    `json:"tvdbId"`
		}{Title: "Show", Path: "/tmp"},
		EpisodeFile: &struct {
			RelativePath string `json:"relativePath"`
			Path         string `json:"path"`
		}{Path: "/tmp"},
	}
	err := svc.processDownload(payload, "tv")
	assert.NoError(t, err)
}

func TestProcessDownload_NoSeries(t *testing.T) {
	svc := NewArrService(ArrConfig{MediaRoot: "/tmp"})
	payload := ArrWebhookPayload{} // no Series, no Movie
	err := svc.processDownload(payload, "tv")
	assert.NoError(t, err)
}

func TestProcessDownload_MovieFile(t *testing.T) {
	svc := NewArrService(ArrConfig{MediaRoot: "/tmp"})
	payload := ArrWebhookPayload{
		Movie: &struct {
			Title  string `json:"title"`
			Path   string `json:"folderPath"`
			TMDBID int    `json:"tmdbId"`
		}{Title: "Movie", Path: "/tmp"},
		MovieFile: &struct {
			RelativePath string `json:"relativePath"`
			Path         string `json:"path"`
		}{Path: "/tmp"},
	}
	err := svc.processDownload(payload, "movie")
	assert.NoError(t, err)
}

// --- TriggerLibraryScan ---

func TestTriggerLibraryScan(t *testing.T) {
	svc := NewArrService(ArrConfig{})
	err := svc.TriggerLibraryScan("/tmp/media")
	assert.NoError(t, err)
}

// --- HealthCheck ---

func TestHealthCheck_NoSecrets(t *testing.T) {
	svc := NewArrService(ArrConfig{MediaRoot: "/data"})
	hc := svc.HealthCheck()
	assert.Equal(t, false, hc["sonarr_webhook"])
	assert.Equal(t, false, hc["radarr_webhook"])
	assert.Equal(t, "/data", hc["media_root"])
	assert.NotEmpty(t, hc["timestamp"])
}
