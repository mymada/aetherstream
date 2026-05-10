package dlna

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devuser/aetherstream/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *db.DB {
	tmpFile, err := os.CreateTemp("", "dlna-test-*.db")
	require.NoError(t, err)
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	database, err := db.New(tmpFile.Name())
	require.NoError(t, err)
	require.NoError(t, database.Migrate())
	return database
}

func TestNewServer(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	srv := NewServer(db, "127.0.0.1", 1901, "TestAether")
	assert.NotEmpty(t, srv.deviceUUID)
	assert.Equal(t, "TestAether", srv.friendlyName)
	assert.Equal(t, 1901, srv.httpPort)
}

func TestDeviceDescription(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	srv := NewServer(db, "127.0.0.1", 1901, "TestAether")
	
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/device/description.xml", nil)
	srv.handleDeviceDescription(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/xml", w.Header().Get("Content-Type"))
	
	body := w.Body.String()
	assert.Contains(t, body, "urn:schemas-upnp-org:device:MediaServer:1")
	assert.Contains(t, body, "TestAether")
	assert.Contains(t, body, srv.deviceUUID)
	assert.Contains(t, body, "ContentDirectory")
	assert.Contains(t, body, "ConnectionManager")
}

func TestContentDirectoryBrowseRoot(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Insert test libraries
	require.NoError(t, database.CreateLibrary("lib-1", "Movies", "/data/movies", "video"))
	require.NoError(t, database.CreateLibrary("lib-2", "Music", "/data/music", "audio"))

	srv := NewServer(database, "127.0.0.1", 1901, "TestAether")

	envelope := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:Browse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">
      <ObjectID>0</ObjectID>
      <BrowseFlag>BrowseDirectChildren</BrowseFlag>
      <Filter>*</Filter>
      <StartingIndex>0</StartingIndex>
      <RequestedCount>0</RequestedCount>
      <SortCriteria></SortCriteria>
    </u:Browse>
  </s:Body>
</s:Envelope>`)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ContentDirectory/control", strings.NewReader(envelope))
	r.Header.Set("Content-Type", "text/xml; charset=utf-8")
	srv.handleContentDirectory(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Movies")
	assert.Contains(t, body, "Music")
	assert.Contains(t, body, "BrowseResponse")
}

func TestContentDirectoryBrowseLibrary(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	require.NoError(t, database.CreateLibrary("lib-1", "Movies", "/data/movies", "video"))
	require.NoError(t, database.CreateItem("item-1", "lib-1", "/data/movies/test.mp4", "test.mp4", "video", "mp4", 1024*1024*100, 3600, 1920, 1080, "h264", "aac"))
	require.NoError(t, database.CreateItem("item-2", "lib-1", "/data/movies/song.mp3", "song.mp3", "audio", "mp3", 1024*1024*5, 180, 0, 0, "", "mp3"))

	srv := NewServer(database, "127.0.0.1", 1901, "TestAether")

	envelope := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:Browse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">
      <ObjectID>lib-1</ObjectID>
      <BrowseFlag>BrowseDirectChildren</BrowseFlag>
      <Filter>*</Filter>
      <StartingIndex>0</StartingIndex>
      <RequestedCount>0</RequestedCount>
      <SortCriteria></SortCriteria>
    </u:Browse>
  </s:Body>
</s:Envelope>`)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ContentDirectory/control", strings.NewReader(envelope))
	srv.handleContentDirectory(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "test.mp4")
	assert.Contains(t, body, "song.mp3")
	assert.Contains(t, body, "object.item.videoItem")
	assert.Contains(t, body, "object.item.audioItem")
	assert.Contains(t, body, "1920x1080")
	assert.Contains(t, body, "01:00:00")
}

func TestContentDirectorySearch(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	srv := NewServer(database, "127.0.0.1", 1901, "TestAether")

	envelope := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:Search xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">
      <ContainerID>0</ContainerID>
      <SearchCriteria>(upnp:class = "object.item.videoItem")</SearchCriteria>
      <Filter>*</Filter>
      <StartingIndex>0</StartingIndex>
      <RequestedCount>0</RequestedCount>
      <SortCriteria></SortCriteria>
    </u:Search>
  </s:Body>
</s:Envelope>`

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ContentDirectory/control", strings.NewReader(envelope))
	srv.handleContentDirectory(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "SearchResponse")
}

func TestConnectionManagerGetProtocolInfo(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	srv := NewServer(db, "127.0.0.1", 1901, "TestAether")

	envelope := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:GetProtocolInfo xmlns:u="urn:schemas-upnp-org:service:ConnectionManager:1">
    </u:GetProtocolInfo>
  </s:Body>
</s:Envelope>`

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ConnectionManager/control", strings.NewReader(envelope))
	srv.handleConnectionManager(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "GetProtocolInfoResponse")
	assert.Contains(t, body, "video/mp4")
	assert.Contains(t, body, "audio/mp3")
}

func TestConnectionManagerGetCurrentConnectionIDs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	srv := NewServer(db, "127.0.0.1", 1901, "TestAether")

	envelope := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:GetCurrentConnectionIDs xmlns:u="urn:schemas-upnp-org:service:ConnectionManager:1">
    </u:GetCurrentConnectionIDs>
  </s:Body>
</s:Envelope>`

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/ConnectionManager/control", strings.NewReader(envelope))
	srv.handleConnectionManager(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ConnectionIDs")
}

func TestBuildDeviceDescriptionXML(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	srv := NewServer(db, "127.0.0.1", 1901, "TestAether")
	xmlStr := srv.buildDeviceDescription()

	// Verify it's valid XML
	var root struct {
		Device struct {
			DeviceType     string `xml:"deviceType"`
			FriendlyName   string `xml:"friendlyName"`
			Manufacturer   string `xml:"manufacturer"`
			ModelName      string `xml:"modelName"`
			UDN            string `xml:"UDN"`
			ServiceList struct {
				Service []struct {
					ServiceType string `xml:"serviceType"`
					ControlURL  string `xml:"controlURL"`
				} `xml:"service"`
			} `xml:"serviceList"`
		} `xml:"device"`
	}

	decoder := xml.NewDecoder(bytes.NewReader([]byte(xmlStr)))
	decoder.Strict = false
	require.NoError(t, decoder.Decode(&root))

	assert.Equal(t, "urn:schemas-upnp-org:device:MediaServer:1", root.Device.DeviceType)
	assert.Equal(t, "TestAether", root.Device.FriendlyName)
	assert.Equal(t, "AetherStream", root.Device.Manufacturer)
	assert.Equal(t, "AetherStream", root.Device.ModelName)
	assert.NotEmpty(t, root.Device.UDN)
	require.Len(t, root.Device.ServiceList.Service, 2)
	assert.Equal(t, "urn:schemas-upnp-org:service:ContentDirectory:1", root.Device.ServiceList.Service[0].ServiceType)
	assert.Equal(t, "/ContentDirectory/control", root.Device.ServiceList.Service[0].ControlURL)
	assert.Equal(t, "urn:schemas-upnp-org:service:ConnectionManager:1", root.Device.ServiceList.Service[1].ServiceType)
	assert.Equal(t, "/ConnectionManager/control", root.Device.ServiceList.Service[1].ControlURL)
}

func TestFormatDuration(t *testing.T) {
	assert.Equal(t, "01:00:00", formatDuration(3600))
	assert.Equal(t, "00:05:30", formatDuration(330))
	assert.Equal(t, "00:00:45", formatDuration(45))
	assert.Equal(t, "02:30:15", formatDuration(9015))
}

func TestExtractHeader(t *testing.T) {
	msg := "M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nST: upnp:rootdevice\r\nMX: 3\r\n"
	assert.Equal(t, "upnp:rootdevice", extractHeader(msg, "ST"))
	assert.Equal(t, "239.255.255.250:1900", extractHeader(msg, "HOST"))
	assert.Equal(t, "", extractHeader(msg, "NOTEXIST"))
}

func TestExtractSOAPAction(t *testing.T) {
	assert.Equal(t, "Browse", extractSOAPAction(`<u:Browse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">`))
	assert.Equal(t, "Search", extractSOAPAction(`<u:Search xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">`))
	assert.Equal(t, "", extractSOAPAction(`<u:Unknown xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">`))
}

func TestSOAPErrorResponse(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	srv := NewServer(db, "127.0.0.1", 1901, "TestAether")
	w := httptest.NewRecorder()
	srv.writeSOAPError(w, 401, "Invalid Action")

	assert.Equal(t, http.StatusOK, w.Code) // UPnP returns 200 with fault body
	body := w.Body.String()
	assert.Contains(t, body, "UPnPError")
	assert.Contains(t, body, "401")
	assert.Contains(t, body, "Invalid Action")
}

func TestContentDeliveryNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	srv := NewServer(db, "127.0.0.1", 1901, "TestAether")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/content/nonexistent", nil)
	srv.handleContentDelivery(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestContentDeliveryFile(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Create a temp file to serve
	tmpFile, err := os.CreateTemp("", "dlna-content-*.mp4")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	_, err = tmpFile.WriteString("fake video content")
	require.NoError(t, err)
	tmpFile.Close()

	require.NoError(t, database.CreateLibrary("lib-1", "Movies", "/data/movies", "video"))
	require.NoError(t, database.CreateItem("item-1", "lib-1", tmpFile.Name(), "test.mp4", "video", "mp4", 18, 10, 1920, 1080, "h264", "aac"))

	srv := NewServer(database, "127.0.0.1", 1901, "TestAether")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/content/item-1", nil)
	srv.handleContentDelivery(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "video/mp4", w.Header().Get("Content-Type"))
	assert.Equal(t, "bytes", w.Header().Get("Accept-Ranges"))
	body, _ := io.ReadAll(w.Result().Body)
	assert.Equal(t, "fake video content", string(body))
}

func TestSSDPNotifyMessage(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	srv := NewServer(db, "127.0.0.1", 1901, "TestAether")
	msg := srv.buildNotify("upnp:rootdevice")

	assert.Contains(t, msg, "NOTIFY * HTTP/1.1")
	assert.Contains(t, msg, "NT: upnp:rootdevice")
	assert.Contains(t, msg, "NTS: ssdp:alive")
	assert.Contains(t, msg, srv.deviceUUID)
	assert.Contains(t, msg, "CACHE-CONTROL: max-age=")
}

func TestStartAndStop(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	srv := NewServer(db, "127.0.0.1", 1902, "TestAether")
	require.NoError(t, srv.Start())

	// Give the HTTP server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Verify HTTP endpoint is up
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:1902/device/description.xml"))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	require.NoError(t, srv.Stop())
}

func TestContentDirectoryInvalidMethod(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	srv := NewServer(db, "127.0.0.1", 1901, "TestAether")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ContentDirectory/control", nil)
	srv.handleContentDirectory(w, r)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestConnectionManagerInvalidMethod(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	srv := NewServer(db, "127.0.0.1", 1901, "TestAether")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ConnectionManager/control", nil)
	srv.handleConnectionManager(w, r)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestConnectionManagerUnknownAction(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	srv := NewServer(db, "127.0.0.1", 1901, "TestAether")
	w := httptest.NewRecorder()
	envelope := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:UnknownAction xmlns:u="urn:schemas-upnp-org:service:ConnectionManager:1">
    </u:UnknownAction>
  </s:Body>
</s:Envelope>`
	r := httptest.NewRequest(http.MethodPost, "/ConnectionManager/control", strings.NewReader(envelope))
	srv.handleConnectionManager(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "UPnPError")
}
