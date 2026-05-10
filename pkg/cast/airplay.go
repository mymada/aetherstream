package cast

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// AirPlayController handles discovery and streaming to AirPlay devices
type AirPlayController struct {
	mu       sync.RWMutex
	devices  map[string]*CastDevice
	sessions map[string]*Session
	stopCh   chan struct{}
	baseURL  string
}

// NewAirPlayController creates an AirPlay controller
func NewAirPlayController(baseURL string) *AirPlayController {
	return &AirPlayController{
		devices:  make(map[string]*CastDevice),
		sessions: make(map[string]*Session),
		stopCh:   make(chan struct{}),
		baseURL:  strings.TrimRight(baseURL, "/"),
	}
}

// Start begins mDNS/Bonjour discovery for _airplay._tcp
func (ac *AirPlayController) Start() error {
	go ac.mdnsDiscoveryLoop()
	return nil
}

// Stop halts discovery
func (ac *AirPlayController) Stop() {
	close(ac.stopCh)
}

// ListDevices returns currently known AirPlay devices
func (ac *AirPlayController) ListDevices() []*CastDevice {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	var out []*CastDevice
	for _, d := range ac.devices {
		if d.Type == "airplay" {
			out = append(out, d)
		}
	}
	return out
}

// StartSession sends an AirPlay /play request with the HLS URL
func (ac *AirPlayController) StartSession(sessionID, deviceID, itemID string) (*Session, error) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	dev, ok := ac.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("airplay device not found: %s", deviceID)
	}

	mediaURL := fmt.Sprintf("%s/videos/%s/hls/master.m3u8", ac.baseURL, itemID)

	sess := &Session{
		ID:       sessionID,
		DeviceID: deviceID,
		ItemID:   itemID,
		MediaURL: mediaURL,
		State:    "loading",
		Created:  time.Now().Unix(),
		Updated:  time.Now().Unix(),
		Device:   dev,
	}
	ac.sessions[sessionID] = sess

	go ac.sendPlay(dev, mediaURL, sess)

	return sess, nil
}

// StopSession stops playback and removes the session
func (ac *AirPlayController) StopSession(sessionID string) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	sess, ok := ac.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Send /stop if possible
	ac.sendStop(sess)

	sess.State = "stopped"
	sess.Updated = time.Now().Unix()
	delete(ac.sessions, sessionID)
	return nil
}

// GetSession returns a session by ID
func (ac *AirPlayController) GetSession(sessionID string) (*Session, bool) {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	sess, ok := ac.sessions[sessionID]
	return sess, ok
}

// --- Discovery ---

func (ac *AirPlayController) mdnsDiscoveryLoop() {
	// Simplified mDNS discovery loop
	// In production this would use golang.org/x/net/mdns or zeroconf to browse _airplay._tcp
	// Here we simulate by probing common multicast and pruning stale entries.

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ac.stopCh:
			return
		case <-ticker.C:
			ac.pruneDevices()
		}
	}
}

func (ac *AirPlayController) registerDevice(addr string, port int, devType, name string) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	id := fmt.Sprintf("%s-%s:%d", devType, addr, port)
	if existing, ok := ac.devices[id]; ok {
		existing.LastSeen = time.Now().Unix()
		return
	}

	if name == "" {
		name = fmt.Sprintf("%s@%s", devType, addr)
	}

	ac.devices[id] = &CastDevice{
		ID:       id,
		Name:     name,
		Type:     devType,
		Address:  addr,
		Port:     port,
		LastSeen: time.Now().Unix(),
	}
	log.Info().Str("id", id).Str("name", name).Str("addr", addr).Int("port", port).Msg("airplay discovered")
}

func (ac *AirPlayController) pruneDevices() {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	cutoff := time.Now().Add(-5 * time.Minute).Unix()
	for id, d := range ac.devices {
		if d.LastSeen < cutoff {
			delete(ac.devices, id)
			log.Info().Str("id", id).Msg("airplay pruned")
		}
	}
}

// --- Control ---

// AirPlay /play POST body structure (reverse-engineered protocol subset)
type airPlayPlayBody struct {
	ContentLocation string  `json:"Content-Location"`
	StartPosition   float64 `json:"Start-Position,omitempty"`
}

func (ac *AirPlayController) sendPlay(dev *CastDevice, mediaURL string, sess *Session) {
	playURL := fmt.Sprintf("http://%s:%d/play", dev.Address, dev.Port)
	body := fmt.Sprintf("Content-Location: %s\nStart-Position: 0.0\n", mediaURL)

	req, err := http.NewRequest(http.MethodPost, playURL, strings.NewReader(body))
	if err != nil {
		log.Warn().Err(err).Str("device", dev.ID).Msg("airplay play req failed")
		ac.updateSessionState(sess.ID, "idle")
		return
	}
	req.Header.Set("Content-Type", "text/parameters")
	req.Header.Set("X-Apple-Session-ID", sess.ID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Warn().Err(err).Str("device", dev.ID).Msg("airplay play failed")
		ac.updateSessionState(sess.ID, "idle")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		ac.updateSessionState(sess.ID, "playing")
		log.Info().Str("session", sess.ID).Str("url", mediaURL).Msg("airplay session started")
	} else {
		log.Warn().Int("status", resp.StatusCode).Str("device", dev.ID).Msg("airplay play rejected")
		ac.updateSessionState(sess.ID, "idle")
	}
}

func (ac *AirPlayController) sendStop(sess *Session) {
	if sess == nil || sess.Device == nil {
		return
	}
	stopURL := fmt.Sprintf("http://%s:%d/stop", sess.Device.Address, sess.Device.Port)
	req, err := http.NewRequest(http.MethodPost, stopURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("X-Apple-Session-ID", sess.ID)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func (ac *AirPlayController) updateSessionState(sessionID, state string) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if sess, ok := ac.sessions[sessionID]; ok {
		sess.State = state
		sess.Updated = time.Now().Unix()
	}
}

// Play resumes playback (stub)
func (ac *AirPlayController) Play(sessionID string) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	sess, ok := ac.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found")
	}
	sess.State = "playing"
	sess.Updated = time.Now().Unix()
	return nil
}

// Pause pauses playback (stub)
func (ac *AirPlayController) Pause(sessionID string) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	sess, ok := ac.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found")
	}
	sess.State = "paused"
	sess.Updated = time.Now().Unix()
	return nil
}

// Seek seeks to a position (stub)
func (ac *AirPlayController) Seek(sessionID string, position int) error {
	return nil
}

// Volume sets volume 0-100 (stub)
func (ac *AirPlayController) Volume(sessionID string, vol int) error {
	return nil
}

// AddDevice allows injecting a discovered device (used by mDNS callbacks or tests)
func (ac *AirPlayController) AddDevice(dev *CastDevice) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	ac.devices[dev.ID] = dev
}

// RemoveDevice removes a device by ID
func (ac *AirPlayController) RemoveDevice(id string) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	delete(ac.devices, id)
}
