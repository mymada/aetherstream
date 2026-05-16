package dlna

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setupAVServer(t *testing.T) *Server {
	t.Helper()
	database := setupTestDB(t)
	t.Cleanup(func() { database.Close() })
	srv := NewServer(database, "127.0.0.1", 1901, "TestAV")
	t.Cleanup(func() {
		for k := range avTransportInstances {
			delete(avTransportInstances, k)
		}
	})
	return srv
}

func avTransportPost(srv *Server, body string, soapAction string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/AVTransport/control", strings.NewReader(body))
	r.Header.Set("Content-Type", "text/xml; charset=utf-8")
	if soapAction != "" {
		r.Header.Set("SOAPAction", `"urn:schemas-upnp-org:service:AVTransport:1#`+soapAction+`"`)
	}
	srv.handleAVTransport(w, r)
	return w
}

func TestAVTransport_SetAVTransportURI(t *testing.T) {
	srv := setupAVServer(t)
	body := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>
		<u:SetAVTransportURI xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
			<InstanceID>0</InstanceID>
			<CurrentURI>http://example.com/video.mp4</CurrentURI>
		</u:SetAVTransportURI></s:Body></s:Envelope>`
	w := avTransportPost(srv, body, "SetAVTransportURI")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "SetAVTransportURIResponse")
	assert.Equal(t, AVStateStopped, avTransportInstances[0].TransportState)
	assert.Equal(t, "http://example.com/video.mp4", avTransportInstances[0].CurrentURI)
}

func TestAVTransport_Play_NoURI(t *testing.T) {
	srv := setupAVServer(t)
	body := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>
		<u:Play xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
			<InstanceID>1</InstanceID><Speed>1</Speed>
		</u:Play></s:Body></s:Envelope>`
	w := avTransportPost(srv, body, "Play")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Fault")
}

func TestAVTransport_Play_WithURI(t *testing.T) {
	srv := setupAVServer(t)
	// Set URI first
	avTransportInstances[2] = &AVTransportInfo{
		InstanceID:     2,
		TransportState: AVStateStopped,
		CurrentURI:     "http://example.com/video.mp4",
	}
	body := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>
		<u:Play xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
			<InstanceID>2</InstanceID><Speed>1</Speed>
		</u:Play></s:Body></s:Envelope>`
	w := avTransportPost(srv, body, "Play")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "PlayResponse")
	assert.Equal(t, AVStatePlaying, avTransportInstances[2].TransportState)
}

func TestAVTransport_Pause_NotPlaying(t *testing.T) {
	srv := setupAVServer(t)
	avTransportInstances[3] = &AVTransportInfo{
		InstanceID:     3,
		TransportState: AVStateStopped,
	}
	body := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>
		<u:Pause xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
			<InstanceID>3</InstanceID>
		</u:Pause></s:Body></s:Envelope>`
	w := avTransportPost(srv, body, "Pause")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Fault")
}

func TestAVTransport_Pause_WhenPlaying(t *testing.T) {
	srv := setupAVServer(t)
	avTransportInstances[4] = &AVTransportInfo{
		InstanceID:     4,
		TransportState: AVStatePlaying,
		CurrentURI:     "http://example.com/v.mp4",
	}
	body := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>
		<u:Pause xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
			<InstanceID>4</InstanceID>
		</u:Pause></s:Body></s:Envelope>`
	w := avTransportPost(srv, body, "Pause")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "PauseResponse")
	assert.Equal(t, AVStatePaused, avTransportInstances[4].TransportState)
}

func TestAVTransport_Stop(t *testing.T) {
	srv := setupAVServer(t)
	avTransportInstances[5] = &AVTransportInfo{
		InstanceID:     5,
		TransportState: AVStatePlaying,
	}
	body := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>
		<u:Stop xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
			<InstanceID>5</InstanceID>
		</u:Stop></s:Body></s:Envelope>`
	w := avTransportPost(srv, body, "Stop")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "StopResponse")
	assert.Equal(t, AVStateStopped, avTransportInstances[5].TransportState)
}

func TestAVTransport_Seek_RelTime(t *testing.T) {
	srv := setupAVServer(t)
	avTransportInstances[6] = &AVTransportInfo{InstanceID: 6, TransportState: AVStatePlaying}
	body := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>
		<u:Seek xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
			<InstanceID>6</InstanceID>
			<Unit>REL_TIME</Unit>
			<Target>0:01:30</Target>
		</u:Seek></s:Body></s:Envelope>`
	w := avTransportPost(srv, body, "Seek")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "SeekResponse")
	assert.Equal(t, "0:01:30", avTransportInstances[6].RelativeTimePosition)
}

func TestAVTransport_Seek_AbsTime(t *testing.T) {
	srv := setupAVServer(t)
	avTransportInstances[7] = &AVTransportInfo{InstanceID: 7, TransportState: AVStatePlaying}
	body := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>
		<u:Seek xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
			<InstanceID>7</InstanceID>
			<Unit>ABS_TIME</Unit>
			<Target>0:02:00</Target>
		</u:Seek></s:Body></s:Envelope>`
	w := avTransportPost(srv, body, "Seek")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "0:02:00", avTransportInstances[7].AbsoluteTimePosition)
}

func TestAVTransport_GetPositionInfo(t *testing.T) {
	srv := setupAVServer(t)
	avTransportInstances[8] = &AVTransportInfo{
		InstanceID:           8,
		TransportState:       AVStatePlaying,
		CurrentURI:           "http://example.com/v.mp4",
		RelativeTimePosition: "0:00:42",
	}
	body := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>
		<u:GetPositionInfo xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
			<InstanceID>8</InstanceID>
		</u:GetPositionInfo></s:Body></s:Envelope>`
	w := avTransportPost(srv, body, "GetPositionInfo")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "GetPositionInfoResponse")
	assert.Contains(t, w.Body.String(), "0:00:42")
}

func TestAVTransport_GetTransportInfo(t *testing.T) {
	srv := setupAVServer(t)
	avTransportInstances[9] = &AVTransportInfo{
		InstanceID:      9,
		TransportState:  AVStatePlaying,
		TransportStatus: "OK",
	}
	body := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>
		<u:GetTransportInfo xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
			<InstanceID>9</InstanceID>
		</u:GetTransportInfo></s:Body></s:Envelope>`
	w := avTransportPost(srv, body, "GetTransportInfo")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "GetTransportInfoResponse")
	assert.Contains(t, w.Body.String(), "PLAYING")
}

func TestAVTransport_GetMediaInfo(t *testing.T) {
	srv := setupAVServer(t)
	avTransportInstances[10] = &AVTransportInfo{
		InstanceID: 10,
		CurrentURI: "http://example.com/media.mp4",
	}
	body := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>
		<u:GetMediaInfo xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
			<InstanceID>10</InstanceID>
		</u:GetMediaInfo></s:Body></s:Envelope>`
	w := avTransportPost(srv, body, "GetMediaInfo")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "GetMediaInfoResponse")
	assert.Contains(t, w.Body.String(), "http://example.com/media.mp4")
}

func TestAVTransport_UnknownAction(t *testing.T) {
	srv := setupAVServer(t)
	body := `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>
		<u:UnknownAction xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
			<InstanceID>0</InstanceID>
		</u:UnknownAction></s:Body></s:Envelope>`
	w := avTransportPost(srv, body, "UnknownAction")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Fault")
}

func TestAVTransport_MethodNotAllowed(t *testing.T) {
	srv := setupAVServer(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/AVTransport/control", nil)
	srv.handleAVTransport(w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

// --- Pure helper function tests ---

func TestExtractSOAPActionHeader(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"urn:schemas-upnp-org:service:AVTransport:1#Play"`, "Play"},
		{`"urn:schemas-upnp-org:service:ContentDirectory:1#Browse"`, "Browse"},
		{"", ""},
		{"NoHash", ""},
		{"#OnlyHash", "OnlyHash"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, extractSOAPActionHeader(tt.input), "input: %q", tt.input)
	}
}

func TestExtractInstanceID(t *testing.T) {
	body := []byte(`<u:Play><InstanceID>3</InstanceID></u:Play>`)
	assert.Equal(t, 3, extractInstanceID(body))

	bodyZero := []byte(`<u:Play><InstanceID>0</InstanceID></u:Play>`)
	assert.Equal(t, 0, extractInstanceID(bodyZero))

	bodyMissing := []byte(`<u:Play></u:Play>`)
	assert.Equal(t, 0, extractInstanceID(bodyMissing))
}

func TestXMLEscape(t *testing.T) {
	assert.Equal(t, "&lt;b&gt;&amp;test&lt;/b&gt;", xmlEscape("<b>&test</b>"))
	assert.Equal(t, "plain", xmlEscape("plain"))
	assert.Equal(t, "", xmlEscape(""))
}

func TestGetOrCreateAVTransport_CreatesNew(t *testing.T) {
	t.Cleanup(func() { delete(avTransportInstances, 99) })
	info := getOrCreateAVTransport(99)
	assert.Equal(t, 99, info.InstanceID)
	assert.Equal(t, AVStateNoMedia, info.TransportState)
	assert.Equal(t, 100, info.Volume)
}

func TestGetOrCreateAVTransport_ReturnsExisting(t *testing.T) {
	t.Cleanup(func() { delete(avTransportInstances, 88) })
	existing := &AVTransportInfo{InstanceID: 88, TransportState: AVStatePlaying, Volume: 50}
	avTransportInstances[88] = existing
	got := getOrCreateAVTransport(88)
	assert.Equal(t, existing, got)
}
