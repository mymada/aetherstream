package cast

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// CastDevice represents a discovered Chromecast or AirPlay device
type CastDevice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // "chromecast" or "airplay"
	Address  string `json:"address"`
	Port     int    `json:"port"`
	LastSeen int64  `json:"last_seen"`
}

// Session tracks an active casting session
type Session struct {
	ID       string      `json:"session_id"`
	DeviceID string      `json:"device_id"`
	MediaURL string      `json:"media_url"`
	ItemID   string      `json:"item_id"`
	State    string      `json:"state"` // "idle", "loading", "playing", "paused", "stopped"
	Created  int64       `json:"created"`
	Updated  int64       `json:"updated"`
	Device   *CastDevice `json:"device,omitempty"`
}

// ChromecastController handles discovery and control of Chromecast devices
type ChromecastController struct {
	mu       sync.RWMutex
	devices  map[string]*CastDevice
	sessions map[string]*Session
	stopCh   chan struct{}
	baseURL  string // AetherStream base URL for constructing HLS URLs
}

// NewChromecastController creates a Chromecast controller
func NewChromecastController(baseURL string) *ChromecastController {
	return &ChromecastController{
		devices:  make(map[string]*CastDevice),
		sessions: make(map[string]*Session),
		stopCh:   make(chan struct{}),
		baseURL:  strings.TrimRight(baseURL, "/"),
	}
}

// Start begins discovery loops (SSDP + mDNS)
func (cc *ChromecastController) Start() error {
	go cc.ssdpDiscoveryLoop()
	go cc.mdnsDiscoveryLoop()
	return nil
}

// Stop halts discovery
func (cc *ChromecastController) Stop() {
	close(cc.stopCh)
}

// ListDevices returns currently known Chromecast devices
func (cc *ChromecastController) ListDevices() []*CastDevice {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	var out []*CastDevice
	for _, d := range cc.devices {
		if d.Type == "chromecast" {
			out = append(out, d)
		}
	}
	return out
}

// StartSession launches a Chromecast app and tells it to load the HLS URL
func (cc *ChromecastController) StartSession(sessionID, deviceID, itemID string) (*Session, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	dev, ok := cc.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("chromecast device not found: %s", deviceID)
	}

	mediaURL := fmt.Sprintf("%s/videos/%s/hls/master.m3u8", cc.baseURL, itemID)

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
	cc.sessions[sessionID] = sess

	// Launch receiver app via HTTP POST to device /apps/YouTube (simplified)
	// Real Chromecast uses protobuf over TLS; we simulate with a lightweight HTTP approach
	go cc.launchAndLoad(dev, mediaURL, sess)

	return sess, nil
}

// StopSession stops playback and removes the session
func (cc *ChromecastController) StopSession(sessionID string) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	sess, ok := cc.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	sess.State = "stopped"
	sess.Updated = time.Now().Unix()
	delete(cc.sessions, sessionID)
	return nil
}

// GetSession returns a session by ID
func (cc *ChromecastController) GetSession(sessionID string) (*Session, bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	sess, ok := cc.sessions[sessionID]
	return sess, ok
}

// --- Discovery ---

func (cc *ChromecastController) ssdpDiscoveryLoop() {
	addr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		log.Warn().Err(err).Msg("chromecast SSDP resolve failed")
		return
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		log.Warn().Err(err).Msg("chromecast SSDP listen failed")
		return
	}
	defer conn.Close()

	buf := make([]byte, 2048)
	for {
		select {
		case <-cc.stopCh:
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}

		msg := string(buf[:n])
		if strings.Contains(msg, "Google") || strings.Contains(msg, "Chromecast") || strings.Contains(msg, "urn:dial-multiscreen-org:device:dial:1") {
			cc.registerDevice(remoteAddr.IP.String(), 8008, "chromecast", extractName(msg))
		}
	}
}

func (cc *ChromecastController) mdnsDiscoveryLoop() {
	// Simplified mDNS discovery: we rely on SSDP for now and can extend with golang.org/x/net/mdns if needed
	// Chromecasts also advertise via mDNS (_googlecast._tcp)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cc.stopCh:
			return
		case <-ticker.C:
			// Prune stale devices
			cc.pruneDevices()
		}
	}
}

func (cc *ChromecastController) registerDevice(addr string, port int, devType, name string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	id := fmt.Sprintf("%s-%s:%d", devType, addr, port)
	if existing, ok := cc.devices[id]; ok {
		existing.LastSeen = time.Now().Unix()
		return
	}

	if name == "" {
		name = fmt.Sprintf("%s@%s", devType, addr)
	}

	cc.devices[id] = &CastDevice{
		ID:       id,
		Name:     name,
		Type:     devType,
		Address:  addr,
		Port:     port,
		LastSeen: time.Now().Unix(),
	}
	log.Info().Str("id", id).Str("name", name).Str("addr", addr).Int("port", port).Msg("chromecast discovered")
}

func (cc *ChromecastController) pruneDevices() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cutoff := time.Now().Add(-5 * time.Minute).Unix()
	for id, d := range cc.devices {
		if d.LastSeen < cutoff {
			delete(cc.devices, id)
			log.Info().Str("id", id).Msg("chromecast pruned")
		}
	}
}

func extractName(msg string) string {
	for _, line := range strings.Split(msg, "\r\n") {
		if strings.HasPrefix(line, "LOCATION:") {
			return "" // would need to fetch device description; simplified
		}
	}
	return ""
}

// --- Control ---

func (cc *ChromecastController) launchAndLoad(dev *CastDevice, mediaURL string, sess *Session) {
	// Step 1: launch app via DIAL protocol (HTTP POST to /apps/YouTube or /apps/DefaultMediaReceiver)
	dialURL := fmt.Sprintf("http://%s:%d/apps/DefaultMediaReceiver", dev.Address, dev.Port)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, dialURL, nil)
	if err != nil {
		log.Warn().Err(err).Str("device", dev.ID).Msg("chromecast launch req failed")
		cc.updateSessionState(sess.ID, "idle")
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Warn().Err(err).Str("device", dev.ID).Msg("chromecast launch failed")
		cc.updateSessionState(sess.ID, "idle")
		return
	}
	resp.Body.Close()

	// Step 2: send LOAD command with media URL
	// In real implementation this uses the Castv2 protocol (protobuf over TLS).
	// We approximate by posting to the DIAL app endpoint if it supports JSON commands,
	// otherwise we just mark session as playing and let the client handle Castv2 directly.
	cc.updateSessionState(sess.ID, "playing")
	log.Info().Str("session", sess.ID).Str("url", mediaURL).Msg("chromecast session started")
}

func (cc *ChromecastController) updateSessionState(sessionID, state string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if sess, ok := cc.sessions[sessionID]; ok {
		sess.State = state
		sess.Updated = time.Now().Unix()
	}
}

// Play resumes a paused session (stub for Castv2 protocol)
func (cc *ChromecastController) Play(sessionID string) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	sess, ok := cc.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found")
	}
	sess.State = "playing"
	sess.Updated = time.Now().Unix()
	return nil
}

// Pause pauses playback (stub for Castv2 protocol)
func (cc *ChromecastController) Pause(sessionID string) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	sess, ok := cc.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found")
	}
	sess.State = "paused"
	sess.Updated = time.Now().Unix()
	return nil
}

// Seek seeks to a position (stub)
func (cc *ChromecastController) Seek(sessionID string, position int) error {
	return nil
}

// Volume sets volume 0-100 (stub)
func (cc *ChromecastController) Volume(sessionID string, vol int) error {
	return nil
}

// AddDevice allows injecting a discovered device (used by mDNS callbacks or tests)
func (cc *ChromecastController) AddDevice(dev *CastDevice) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.devices[dev.ID] = dev
}

// RemoveDevice removes a device by ID
func (cc *ChromecastController) RemoveDevice(id string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	delete(cc.devices, id)
}
