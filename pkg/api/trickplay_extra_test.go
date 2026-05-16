package api

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerateTrickplayVTT_Header(t *testing.T) {
	vtt := generateTrickplayVTT(0, "item-1")
	assert.True(t, strings.HasPrefix(vtt, "WEBVTT\n\n"), "must start with WEBVTT header")
}

func TestGenerateTrickplayVTT_Intervals(t *testing.T) {
	vtt := generateTrickplayVTT(25, "item-abc")
	// 25 seconds → 3 cues: 0-10, 10-20, 20-25
	assert.Equal(t, 3, strings.Count(vtt, "-->"))
	assert.Contains(t, vtt, "00:00:00.000 --> 00:00:10.000")
	assert.Contains(t, vtt, "00:00:10.000 --> 00:00:20.000")
	assert.Contains(t, vtt, "00:00:20.000 --> 00:00:25.000")
	assert.Contains(t, vtt, "/api/items/item-abc/thumbnails/trickplay?t=0")
}

func TestGenerateTrickplayVTT_ZeroDuration(t *testing.T) {
	vtt := generateTrickplayVTT(0, "item-z")
	// No cues for zero duration
	assert.Equal(t, 0, strings.Count(vtt, "-->"))
}

func TestGenerateTrickplayVTT_ExactInterval(t *testing.T) {
	vtt := generateTrickplayVTT(30, "item-e")
	// 30 seconds → 3 cues: 0-10, 10-20, 20-30
	assert.Equal(t, 3, strings.Count(vtt, "-->"))
	assert.Contains(t, vtt, "00:00:20.000 --> 00:00:30.000")
}

func TestFormatVTTTime(t *testing.T) {
	tests := []struct {
		seconds  float64
		expected string
	}{
		{0, "00:00:00.000"},
		{1.5, "00:00:01.500"},
		{60, "00:01:00.000"},
		{3661.123, "01:01:01.123"},
		{3600, "01:00:00.000"},
		{90.25, "00:01:30.250"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatVTTTime(tt.seconds), "seconds: %v", tt.seconds)
	}
}

func TestIsPrivateIP(t *testing.T) {
	private := []string{
		"10.0.0.1",
		"10.255.255.255",
		"172.16.0.1",
		"172.31.255.255",
		"192.168.0.1",
		"192.168.1.100",
		"127.0.0.1",
		"127.0.0.255",
		"169.254.1.1",
		"::1",
		"fc00::1",
		"fd00::1",
	}
	for _, ip := range private {
		assert.True(t, isPrivateIP(ip), "expected private: %s", ip)
	}

	public := []string{
		"8.8.8.8",
		"1.1.1.1",
		"203.0.113.1",
		"2001:db8::1",
	}
	for _, ip := range public {
		assert.False(t, isPrivateIP(ip), "expected public: %s", ip)
	}

	assert.False(t, isPrivateIP("not-an-ip"))
	assert.False(t, isPrivateIP(""))
}

func TestBruteForceLimiter_AllowedInitially(t *testing.T) {
	b := newBruteForceLimiter()
	defer b.Stop()
	assert.True(t, b.allowed("ip:1.2.3.4"))
}

func TestBruteForceLimiter_LockedAfterRecords(t *testing.T) {
	b := newBruteForceLimiter()
	defer b.Stop()

	key := "ip:5.6.7.8"
	// Record enough attempts to trigger lockout (delay = 2^count seconds)
	for i := 0; i < 5; i++ {
		b.record(key)
	}
	assert.False(t, b.allowed(key))
}

func TestBruteForceLimiter_Reset(t *testing.T) {
	b := newBruteForceLimiter()
	defer b.Stop()

	key := "user:testuser"
	for i := 0; i < 5; i++ {
		b.record(key)
	}
	assert.False(t, b.allowed(key))

	b.reset(key)
	assert.True(t, b.allowed(key))
}

func TestBruteForceLimiter_Stop_Idempotent(t *testing.T) {
	b := newBruteForceLimiter()
	assert.NotPanics(t, func() {
		b.Stop()
		b.Stop()
		b.Stop()
	})
}

func TestBruteForceLimiter_MaxDelay(t *testing.T) {
	b := newBruteForceLimiter()
	defer b.Stop()

	key := "ip:9.9.9.9"
	// Record many times — delay should cap at maxDelay (30min)
	for i := 0; i < 30; i++ {
		b.record(key)
	}
	b.mu.RLock()
	a := b.attempts[key]
	b.mu.RUnlock()

	assert.NotNil(t, a)
	assert.LessOrEqual(t, a.lockedUntil.Sub(a.lastAttempt), b.maxDelay+time.Second)
}
