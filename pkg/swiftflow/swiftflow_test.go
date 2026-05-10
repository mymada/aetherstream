package swiftflow

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("http://swiftflow.local", "secret123")
	if c.baseURL != "http://swiftflow.local" {
		t.Errorf("baseURL = %q, want http://swiftflow.local", c.baseURL)
	}
	if c.apiKey != "secret123" {
		t.Errorf("apiKey = %q, want secret123", c.apiKey)
	}
	if c.client == nil {
		t.Error("http client is nil")
	}
	if c.client.Timeout == 0 {
		t.Error("client timeout not set")
	}
}

func TestGetDeviceInfoSuccess(t *testing.T) {
	expected := DeviceInfo{
		DeviceID:      "dev-1",
		UserID:        "user-1",
		IP:            "192.168.1.10",
		MAC:           "aa:bb:cc:dd:ee:ff",
		ClientType:    "tv",
		BandwidthKbps: 5000,
		Authenticated: true,
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %q, want GET", r.Method)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer testkey" {
			t.Errorf("Authorization = %q, want Bearer testkey", auth)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expected)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "testkey")
	info, err := c.GetDeviceInfo("192.168.1.10", "aa:bb:cc:dd:ee:ff")
	if err != nil {
		t.Fatal(err)
	}
	if info.DeviceID != expected.DeviceID {
		t.Errorf("DeviceID = %q, want %q", info.DeviceID, expected.DeviceID)
	}
	if info.BandwidthKbps != expected.BandwidthKbps {
		t.Errorf("BandwidthKbps = %d, want %d", info.BandwidthKbps, expected.BandwidthKbps)
	}
}

func TestGetDeviceInfoNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "testkey")
	_, err := c.GetDeviceInfo("1.2.3.4", "00:00:00:00:00:00")
	if err == nil {
		t.Error("expected error for 404")
	}
}

func TestUpdateBandwidthSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer testkey" {
			t.Errorf("Authorization = %q, want Bearer testkey", auth)
		}
		var update BandwidthUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			t.Fatal(err)
		}
		if update.SessionID != "sess-1" {
			t.Errorf("SessionID = %q, want sess-1", update.SessionID)
		}
		if update.BandwidthKbps != 2000 {
			t.Errorf("BandwidthKbps = %d, want 2000", update.BandwidthKbps)
		}
		if update.Direction != "down" {
			t.Errorf("Direction = %q, want down", update.Direction)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "testkey")
	err := c.UpdateBandwidth(BandwidthUpdate{
		SessionID:     "sess-1",
		BandwidthKbps: 2000,
		Direction:     "down",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateBandwidthFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, "testkey")
	err := c.UpdateBandwidth(BandwidthUpdate{SessionID: "sess-1", BandwidthKbps: 100, Direction: "up"})
	if err == nil {
		t.Error("expected error for 400")
	}
}

func TestValidateWebhook(t *testing.T) {
	c := NewClient("http://swiftflow.local", "key")
	if !c.ValidateWebhook([]byte("payload"), "sig") {
		t.Error("ValidateWebhook should return true (placeholder)")
	}
	if !c.ValidateWebhook([]byte{}, "") {
		t.Error("ValidateWebhook should return true for empty payload")
	}
}
