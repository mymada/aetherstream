package integrations

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArrService_HealthCheck(t *testing.T) {
	svc := NewArrService(ArrConfig{
		SonarrSecret: "secret1",
		RadarrSecret: "secret2",
		MediaRoot:    "/media",
	})
	hc := svc.HealthCheck()
	assert.Equal(t, true, hc["sonarr_webhook"])
	assert.Equal(t, true, hc["radarr_webhook"])
	assert.Equal(t, "/media", hc["media_root"])
}

func TestVerifySignature(t *testing.T) {
	body := []byte("test payload")
	secret := "mysecret"

	// Empty secret / signature should pass
	assert.True(t, verifySignature(body, "", ""))

	// Valid signature
	assert.True(t, verifySignature(body, secret, computeHMAC(body, secret)))

	// Invalid signature
	assert.False(t, verifySignature(body, secret, "bad"))
}

func computeHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestProcessDownload_TV(t *testing.T) {
	svc := NewArrService(ArrConfig{MediaRoot: "/tmp"})
	payload := ArrWebhookPayload{
		EventType: "Download",
		Series: &struct {
			Title  string `json:"title"`
			Path   string `json:"path"`
			TVDBID int    `json:"tvdbId"`
		}{Title: "Test Show", Path: "/tmp", TVDBID: 123},
	}
	err := svc.processDownload(payload, "tv")
	assert.NoError(t, err)
}

func TestProcessDownload_Movie(t *testing.T) {
	svc := NewArrService(ArrConfig{MediaRoot: "/tmp"})
	payload := ArrWebhookPayload{
		EventType: "Download",
		Movie: &struct {
			Title  string `json:"title"`
			Path   string `json:"folderPath"`
			TMDBID int    `json:"tmdbId"`
		}{Title: "Test Movie", Path: "/tmp", TMDBID: 456},
	}
	err := svc.processDownload(payload, "movie")
	assert.NoError(t, err)
}
