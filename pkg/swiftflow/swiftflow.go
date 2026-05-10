package swiftflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client talks to SwiftFlow captive portal / QoS API
type Client struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewClient creates SwiftFlow API client
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// DeviceInfo from SwiftFlow captive portal
type DeviceInfo struct {
	DeviceID      string `json:"device_id"`
	UserID        string `json:"user_id"`
	IP            string `json:"ip_address"`
	MAC           string `json:"mac_address"`
	ClientType    string `json:"client_type"` // mobile, tablet, tv, desktop
	BandwidthKbps int    `json:"bandwidth_kbps"`
	Authenticated bool   `json:"authenticated"`
}

// GetDeviceInfo fetches device from SwiftFlow by IP or MAC
func (c *Client) GetDeviceInfo(ip, mac string) (*DeviceInfo, error) {
	url := fmt.Sprintf("%s/api/v1/devices?ip=%s&mac=%s", c.baseURL, ip, mac)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("swiftflow request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("swiftflow returned %d", resp.StatusCode)
	}

	var info DeviceInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// BandwidthUpdate pushes current bandwidth usage to SwiftFlow for QoS shaping
type BandwidthUpdate struct {
	SessionID     string `json:"session_id"`
	BandwidthKbps int    `json:"bandwidth_kbps"`
	Direction     string `json:"direction"` // "up" or "down"
}

// UpdateBandwidth reports streaming bandwidth to SwiftFlow QoS
func (c *Client) UpdateBandwidth(update BandwidthUpdate) error {
	body, _ := json.Marshal(update)
	url := c.baseURL + "/api/v1/qos/bandwidth"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("swiftflow bandwidth update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("swiftflow bandwidth update failed: %d", resp.StatusCode)
	}
	return nil
}

// AuthWebhookPayload from SwiftFlow captive portal post-auth redirect
type AuthWebhookPayload struct {
	UserID        string `json:"user_id"`
	DeviceID      string `json:"device_id"`
	IP            string `json:"ip_address"`
	MAC           string `json:"mac_address"`
	ClientType    string `json:"client_type"`
	BandwidthKbps int    `json:"bandwidth_kbps"`
	Token         string `json:"token"` // SwiftFlow session token
}

// ValidateWebhook verifies SwiftFlow webhook signature (placeholder)
func (c *Client) ValidateWebhook(payload []byte, signature string) bool {
	// TODO: HMAC-SHA256 validation with webhook_secret
	// For Phase 1: trust all webhooks from configured SwiftFlow
	return true
}
