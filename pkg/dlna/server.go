package dlna

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/devuser/aetherstream/pkg/db"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	ssdpMulticastAddr = "239.255.255.250:1900"
	ssdpMaxAge        = 1800
	upnpVersionMajor  = 1
	upnpVersionMinor  = 0
)

// Server implements a DLNA/UPnP MediaServer

type Server struct {
	httpPort    int
	httpHost    string
	deviceUUID  string
	friendlyName string
	db          *db.DB
	mediaDirs   []string
	
	ssdpConn    *net.UDPConn
	ssdpStop    chan struct{}
	httpServer  *http.Server
}

// NewServer creates a DLNA server
func NewServer(database *db.DB, host string, port int, friendlyName string) *Server {
	if friendlyName == "" {
		friendlyName = "AetherStream"
	}
	return &Server{
		httpPort:     port,
		httpHost:     host,
		deviceUUID:   uuid.New().String(),
		friendlyName: friendlyName,
		db:           database,
		ssdpStop:     make(chan struct{}),
	}
}

// Start launches SSDP and HTTP services
func (s *Server) Start() error {
	// Start HTTP server for UPnP services
	mux := http.NewServeMux()
	mux.HandleFunc("/device/description.xml", s.handleDeviceDescription)
	mux.HandleFunc("/ContentDirectory/control", s.handleContentDirectory)
	mux.HandleFunc("/ConnectionManager/control", s.handleConnectionManager)
	mux.HandleFunc("/content/", s.handleContentDelivery)

	s.httpServer = &http.Server{
		Addr:              fmt.Sprintf("%s:%d", s.httpHost, s.httpPort),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info().Str("addr", s.httpServer.Addr).Msg("DLNA HTTP starting")
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("DLNA HTTP server crashed")
		}
	}()

	// Start SSDP discovery
	if err := s.startSSDP(); err != nil {
		log.Warn().Err(err).Msg("SSDP discovery failed to start")
	}

	return nil
}

// Stop shuts down SSDP and HTTP
func (s *Server) Stop() error {
	close(s.ssdpStop)
	if s.ssdpConn != nil {
		s.ssdpConn.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.httpServer.Shutdown(ctx)
}

// --- SSDP Discovery ---

func (s *Server) startSSDP() error {
	addr, err := net.ResolveUDPAddr("udp4", ssdpMulticastAddr)
	if err != nil {
		return fmt.Errorf("resolve SSDP addr: %w", err)
	}

	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("listen multicast: %w", err)
	}
	s.ssdpConn = conn

	// Send initial NOTIFY announcements
	go s.ssdpAnnounceLoop()

	// Listen for M-SEARCH requests
	go s.ssdpListenLoop()

	return nil
}

func (s *Server) ssdpAnnounceLoop() {
	// Initial announcements
	for i := 0; i < 3; i++ {
		s.sendSSDPNotify()
		time.Sleep(100 * time.Millisecond)
	}

	ticker := time.NewTicker(ssdpMaxAge / 2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.sendSSDPNotify()
		case <-s.ssdpStop:
			return
		}
	}
}

func (s *Server) sendSSDPNotify() {
	msgs := []string{
		s.buildNotify("upnp:rootdevice"),
		s.buildNotify("urn:schemas-upnp-org:device:MediaServer:1"),
		s.buildNotify("urn:schemas-upnp-org:service:ContentDirectory:1"),
		s.buildNotify("urn:schemas-upnp-org:service:ConnectionManager:1"),
	}

	addr, _ := net.ResolveUDPAddr("udp4", ssdpMulticastAddr)
	for _, msg := range msgs {
		conn, err := net.DialUDP("udp4", nil, addr)
		if err != nil {
			continue
		}
		conn.Write([]byte(msg))
		conn.Close()
	}
}

func (s *Server) buildNotify(nt string) string {
	location := fmt.Sprintf("http://%s:%d/device/description.xml", s.httpHost, s.httpPort)
	return fmt.Sprintf(
		"NOTIFY * HTTP/1.1\r\n"+
			"HOST: %s\r\n"+
			"NT: %s\r\n"+
			"NTS: ssdp:alive\r\n"+
			"LOCATION: %s\r\n"+
			"USN: uuid:%s::%s\r\n"+
			"CACHE-CONTROL: max-age=%d\r\n"+
			"SERVER: AetherStream UPnP/1.0\r\n"+
			"\r\n",
		ssdpMulticastAddr, nt, location, s.deviceUUID, nt, ssdpMaxAge,
	)
}

func (s *Server) ssdpListenLoop() {
	buf := make([]byte, 2048)
	for {
		select {
		case <-s.ssdpStop:
			return
		default:
		}

		s.ssdpConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remoteAddr, err := s.ssdpConn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}

		msg := string(buf[:n])
		if strings.Contains(msg, "M-SEARCH") {
			s.handleMSEARCH(msg, remoteAddr)
		}
	}
}

func (s *Server) handleMSEARCH(msg string, remoteAddr *net.UDPAddr) {
	st := extractHeader(msg, "ST")
	if st == "" {
		return
	}

	// Respond to generic or specific searches
	matched := st == "ssdp:all" ||
		st == "upnp:rootdevice" ||
		st == "urn:schemas-upnp-org:device:MediaServer:1" ||
		st == "urn:schemas-upnp-org:service:ContentDirectory:1" ||
		st == "urn:schemas-upnp-org:service:ConnectionManager:1"

	if !matched {
		return
	}

	location := fmt.Sprintf("http://%s:%d/device/description.xml", s.httpHost, s.httpPort)
	usn := fmt.Sprintf("uuid:%s::%s", s.deviceUUID, st)
	if st == "upnp:rootdevice" || st == "ssdp:all" {
		usn = fmt.Sprintf("uuid:%s::upnp:rootdevice", s.deviceUUID)
	}

	response := fmt.Sprintf(
		"HTTP/1.1 200 OK\r\n"+
			"CACHE-CONTROL: max-age=%d\r\n"+
			"DATE: %s\r\n"+
			"EXT:\r\n"+
			"LOCATION: %s\r\n"+
			"SERVER: AetherStream UPnP/1.0\r\n"+
			"ST: %s\r\n"+
			"USN: %s\r\n"+
			"Content-Length: 0\r\n"+
			"\r\n",
		ssdpMaxAge, time.Now().Format(http.TimeFormat), location, st, usn,
	)

	conn, err := net.DialUDP("udp4", nil, remoteAddr)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.Write([]byte(response))
}

func extractHeader(msg, name string) string {
	lines := strings.Split(msg, "\r\n")
	prefix := name + ":"
	for _, line := range lines {
		if strings.HasPrefix(strings.ToUpper(line), strings.ToUpper(prefix)) {
			return strings.TrimSpace(line[len(prefix):])
		}
	}
	return ""
}

// --- Device Description ---

func (s *Server) handleDeviceDescription(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml")
	w.Write([]byte(s.buildDeviceDescription()))
}

func (s *Server) buildDeviceDescription() string {
	location := fmt.Sprintf("http://%s:%d", s.httpHost, s.httpPort)
	output := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <specVersion>
    <major>%d</major>
    <minor>%d</minor>
  </specVersion>
  <URLBase>%s</URLBase>
  <device>
    <deviceType>urn:schemas-upnp-org:device:MediaServer:1</deviceType>
    <friendlyName>%s</friendlyName>
    <manufacturer>AetherStream</manufacturer>
    <manufacturerURL>https://aetherstream.dev</manufacturerURL>
    <modelDescription>AetherStream Media Server</modelDescription>
    <modelName>AetherStream</modelName>
    <modelNumber>1.0</modelNumber>
    <UDN>uuid:%s</UDN>
    <iconList>
      <icon>
        <mimetype>image/png</mimetype>
        <width>48</width>
        <height>48</height>
        <depth>24</depth>
        <url>/icon48.png</url>
      </icon>
    </iconList>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:ContentDirectory:1</serviceType>
        <serviceId>urn:upnp-org:serviceId:ContentDirectory</serviceId>
        <controlURL>/ContentDirectory/control</controlURL>
        <eventSubURL>/ContentDirectory/event</eventSubURL>
        <SCPDURL>/ContentDirectory/scpd.xml</SCPDURL>
      </service>
      <service>
        <serviceType>urn:schemas-upnp-org:service:ConnectionManager:1</serviceType>
        <serviceId>urn:upnp-org:serviceId:ConnectionManager</serviceId>
        <controlURL>/ConnectionManager/control</controlURL>
        <eventSubURL>/ConnectionManager/event</eventSubURL>
        <SCPDURL>/ConnectionManager/scpd.xml</SCPDURL>
      </service>
    </serviceList>
  </device>
</root>`, upnpVersionMajor, upnpVersionMinor, location, xmlEscape(s.friendlyName), s.deviceUUID)
	return output
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// --- ContentDirectory SOAP ---

type soapEnvelope struct {
	XMLName xml.Name    `xml:"s:Envelope"`
	XmlnsS  string      `xml:"xmlns:s,attr"`
	Body    soapBody    `xml:"s:Body"`
}

type soapBody struct {
	Browse              browseRequest              `xml:"u:Browse"`
	GetProtocolInfo       getProtocolInfoRequest       `xml:"u:GetProtocolInfo"`
	GetCurrentConnectionIDs getCurrentConnectionIDsRequest `xml:"u:GetCurrentConnectionIDs"`
}

type browseRequest struct {
	XMLName        xml.Name `xml:"u:Browse"`
	XmlnsU         string   `xml:"xmlns:u,attr"`
	ObjectID       string   `xml:"ObjectID"`
	BrowseFlag     string   `xml:"BrowseFlag"`
	Filter         string   `xml:"Filter"`
	StartingIndex  int      `xml:"StartingIndex"`
	RequestedCount int      `xml:"RequestedCount"`
	SortCriteria   string   `xml:"SortCriteria"`
}

type getProtocolInfoRequest struct {
	XMLName xml.Name `xml:"u:GetProtocolInfo"`
	XmlnsU  string   `xml:"xmlns:u,attr"`
}

type getCurrentConnectionIDsRequest struct {
	XMLName xml.Name `xml:"u:GetCurrentConnectionIDs"`
	XmlnsU  string   `xml:"xmlns:u,attr"`
}

type browseResponse struct {
	XMLName       xml.Name `xml:"u:BrowseResponse"`
	XmlnsU        string   `xml:"xmlns:u,attr"`
	Result        string   `xml:"Result"`
	NumberReturned int     `xml:"NumberReturned"`
	TotalMatches   int     `xml:"TotalMatches"`
	UpdateID       int      `xml:"UpdateID"`
}

type didlLite struct {
	XMLName   xml.Name    `xml:"DIDL-Lite"`
	XmlnsDc   string      `xml:"xmlns:dc,attr"`
	XmlnsUpnp string      `xml:"xmlns:upnp,attr"`
	Xmlns     string      `xml:"xmlns,attr"`
	Items     []didlItem  `xml:"item"`
	Containers []didlContainer `xml:"container"`
}

type didlItem struct {
	XMLName     xml.Name `xml:"item"`
	ID          string   `xml:"id,attr"`
	ParentID    string   `xml:"parentID,attr"`
	Restricted  int      `xml:"restricted,attr"`
	Title       string   `xml:"dc:title"`
	Class       string   `xml:"upnp:class"`
	Res         *didlRes `xml:"res"`
}

type didlContainer struct {
	XMLName       xml.Name `xml:"container"`
	ID            string   `xml:"id,attr"`
	ParentID      string   `xml:"parentID,attr"`
	Restricted    int      `xml:"restricted,attr"`
	ChildCount    int      `xml:"childCount,attr"`
	Title         string   `xml:"dc:title"`
	Class         string   `xml:"upnp:class"`
}

type didlRes struct {
	ProtocolInfo string `xml:"protocolInfo,attr"`
	Size         int64  `xml:"size,attr"`
	Duration     string `xml:"duration,attr,omitempty"`
	Resolution   string `xml:"resolution,attr,omitempty"`
	Value        string `xml:",chardata"`
}

func (s *Server) handleContentDirectory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, _ := io.ReadAll(r.Body)
	action := extractSOAPAction(string(body))

	switch action {
	case "Browse":
		s.handleBrowse(w, r, body)
	case "Search":
		s.handleSearch(w, r, body)
	default:
		s.writeSOAPError(w, 401, "Invalid Action")
	}
}

func extractSOAPAction(body string) string {
	if idx := strings.Index(body, "u:Browse"); idx != -1 {
		return "Browse"
	}
	if idx := strings.Index(body, "u:Search"); idx != -1 {
		return "Search"
	}
	if idx := strings.Index(body, "u:GetProtocolInfo"); idx != -1 {
		return "GetProtocolInfo"
	}
	if idx := strings.Index(body, "u:GetCurrentConnectionIDs"); idx != -1 {
		return "GetCurrentConnectionIDs"
	}
	if idx := strings.Index(body, "u:GetCurrentConnectionInfo"); idx != -1 {
		return "GetCurrentConnectionInfo"
	}
	return ""
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request, body []byte) {
	req, err := parseBrowseRequest(body)
	if err != nil {
		s.writeSOAPError(w, 402, "Invalid Args")
		return
	}

	result, total, err := s.buildBrowseResult(req.ObjectID, req.BrowseFlag, req.StartingIndex, req.RequestedCount)
	if err != nil {
		s.writeSOAPError(w, 501, "Action Failed")
		return
	}

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("EXT", "")
	w.WriteHeader(http.StatusOK)

	output := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:BrowseResponse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">
      <Result>%s</Result>
      <NumberReturned>%d</NumberReturned>
      <TotalMatches>%d</TotalMatches>
      <UpdateID>%d</UpdateID>
    </u:BrowseResponse>
  </s:Body>
</s:Envelope>`, xmlEscape(result), total, total, 1)
	w.Write([]byte(output))
}

// parseBrowseRequest extracts Browse parameters from any SOAP body using regex-like string extraction
// because xml.Unmarshal with arbitrary namespaces is brittle.
type browseReqParsed struct {
	ObjectID       string
	BrowseFlag     string
	StartingIndex  int
	RequestedCount int
}

func parseBrowseRequest(body []byte) (browseReqParsed, error) {
	var req browseReqParsed
	s := string(body)

	req.ObjectID = extractXMLTag(s, "ObjectID")
	req.BrowseFlag = extractXMLTag(s, "BrowseFlag")
	if si := extractXMLTag(s, "StartingIndex"); si != "" {
		v, _ := strconv.Atoi(si)
		req.StartingIndex = v
	}
	if rc := extractXMLTag(s, "RequestedCount"); rc != "" {
		v, _ := strconv.Atoi(rc)
		req.RequestedCount = v
	}

	return req, nil
}

func extractXMLTag(xmlStr, tag string) string {
	start := "<" + tag + ">"
	end := "</" + tag + ">"
	i := strings.Index(xmlStr, start)
	if i == -1 {
		return ""
	}
	j := strings.Index(xmlStr[i:], end)
	if j == -1 {
		return ""
	}
	return xmlStr[i+len(start) : i+j]
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request, body []byte) {
	// For now, search returns empty results (minimal implementation)
	result := `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"></DIDL-Lite>`
	
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	output := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:SearchResponse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">
      <Result>%s</Result>
      <NumberReturned>0</NumberReturned>
      <TotalMatches>0</TotalMatches>
      <UpdateID>1</UpdateID>
    </u:SearchResponse>
  </s:Body>
</s:Envelope>`, xmlEscape(result))
	w.Write([]byte(output))
}

func (s *Server) buildBrowseResult(objectID, browseFlag string, startIndex, count int) (string, int, error) {
	didl := didlLite{
		XmlnsDc:   "http://purl.org/dc/elements/1.1/",
		XmlnsUpnp: "urn:schemas-upnp-org:metadata-1-0/upnp/",
		Xmlns:     "urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/",
	}

	if objectID == "0" || objectID == "" {
		// Root container: list libraries as containers
		libs, err := s.db.ListLibraries()
		if err != nil {
			return "", 0, err
		}

		for _, lib := range libs {
			id, _ := lib["id"].(string)
			name, _ := lib["name"].(string)
			didl.Containers = append(didl.Containers, didlContainer{
				ID:         id,
				ParentID:   "0",
				Restricted: 1,
				ChildCount: 0, // Will be populated if needed
				Title:      name,
				Class:      "object.container.storageFolder",
			})
		}
	} else {
		// Library container: list items
		items, err := s.db.ListItemsByLibrary(objectID)
		if err != nil {
			return "", 0, err
		}

		for _, item := range items {
			itemID, _ := item["id"].(string)
			name, _ := item["name"].(string)
			mediaType, _ := item["mediaType"].(string)
			itemPath, _ := item["path"].(string)
			sizeBytes, _ := item["sizeBytes"].(int64)
			duration, _ := item["durationSeconds"].(float64)
			width, _ := item["width"].(int)
			height, _ := item["height"].(int)
			container, _ := item["container"].(string)

			upnpClass := "object.item.videoItem"
			if mediaType == "audio" {
				upnpClass = "object.item.audioItem"
			} else if mediaType == "image" {
				upnpClass = "object.item.imageItem"
			}

			res := &didlRes{
				ProtocolInfo: fmt.Sprintf("http-get:*:video/%s:*", container),
				Size:         sizeBytes,
				Value:        fmt.Sprintf("http://%s:%d/content/%s", s.httpHost, s.httpPort, itemID),
			}
			if duration > 0 {
				res.Duration = formatDuration(duration)
			}
			if width > 0 && height > 0 {
				res.Resolution = fmt.Sprintf("%dx%d", width, height)
			}

			// Adjust protocol info for audio/image
			if mediaType == "audio" {
				res.ProtocolInfo = fmt.Sprintf("http-get:*:audio/%s:*", container)
			} else if mediaType == "image" {
				res.ProtocolInfo = fmt.Sprintf("http-get:*:image/%s:*", container)
			}

			_ = itemPath // may be used later for direct file protocol info

			didl.Items = append(didl.Items, didlItem{
				ID:         itemID,
				ParentID:   objectID,
				Restricted: 1,
				Title:      name,
				Class:      upnpClass,
				Res:        res,
			})
		}
	}

	output, err := xml.Marshal(didl)
	if err != nil {
		return "", 0, err
	}

	total := len(didl.Containers) + len(didl.Items)
	return string(output), total, nil
}

func formatDuration(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// --- ConnectionManager SOAP ---

func (s *Server) handleConnectionManager(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, _ := io.ReadAll(r.Body)
	action := extractSOAPAction(string(body))

	switch action {
	case "GetProtocolInfo":
		s.handleGetProtocolInfo(w)
	case "GetCurrentConnectionIDs":
		s.handleGetCurrentConnectionIDs(w)
	case "GetCurrentConnectionInfo":
		s.handleGetCurrentConnectionInfo(w)
	default:
		s.writeSOAPError(w, 401, "Invalid Action")
	}
}

func (s *Server) handleGetProtocolInfo(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	output := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:GetProtocolInfoResponse xmlns:u="urn:schemas-upnp-org:service:ConnectionManager:1">
      <Source>http-get:*:video/mp4:*,http-get:*:video/mkv:*,http-get:*:video/avi:*,http-get:*:audio/mp3:*,http-get:*:audio/aac:*,http-get:*:image/jpeg:*,http-get:*:image/png:*</Source>
      <Sink></Sink>
    </u:GetProtocolInfoResponse>
  </s:Body>
</s:Envelope>`
	w.Write([]byte(output))
}

func (s *Server) handleGetCurrentConnectionIDs(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	output := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:GetCurrentConnectionIDsResponse xmlns:u="urn:schemas-upnp-org:service:ConnectionManager:1">
      <ConnectionIDs>0</ConnectionIDs>
    </u:GetCurrentConnectionIDsResponse>
  </s:Body>
</s:Envelope>`
	w.Write([]byte(output))
}

func (s *Server) handleGetCurrentConnectionInfo(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	output := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:GetCurrentConnectionInfoResponse xmlns:u="urn:schemas-upnp-org:service:ConnectionManager:1">
      <RcsID>0</RcsID>
      <AVTransportID>0</AVTransportID>
      <ProtocolInfo>http-get:*:video/mp4:*</ProtocolInfo>
      <PeerConnectionManager></PeerConnectionManager>
      <PeerConnectionID>-1</PeerConnectionID>
      <Direction>Input</Direction>
      <Status>OK</Status>
    </u:GetCurrentConnectionInfoResponse>
  </s:Body>
</s:Envelope>`
	w.Write([]byte(output))
}

func (s *Server) writeSOAPError(w http.ResponseWriter, code int, desc string) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	output := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <s:Fault>
      <faultcode>s:Client</faultcode>
      <faultstring>UPnPError</faultstring>
      <detail>
        <UPnPError xmlns="urn:schemas-upnp-org:control-1-0">
          <errorCode>%d</errorCode>
          <errorDescription>%s</errorDescription>
        </UPnPError>
      </detail>
    </s:Fault>
  </s:Body>
</s:Envelope>`, code, xmlEscape(desc))
	w.Write([]byte(output))
}

// --- Content Delivery ---

func (s *Server) handleContentDelivery(w http.ResponseWriter, r *http.Request) {
	itemID := strings.TrimPrefix(r.URL.Path, "/content/")
	if itemID == "" {
		http.Error(w, "missing item ID", http.StatusBadRequest)
		return
	}
	item, err := s.db.GetItemByID(itemID)
	if err != nil {
		http.Error(w, "item not found", http.StatusNotFound)
		return
	}

	// Serve file with appropriate content type
	container, _ := item["container"].(string)
	contentType := "application/octet-stream"
	switch container {
	case "mp4":
		contentType = "video/mp4"
	case "mkv":
		contentType = "video/x-matroska"
	case "avi":
		contentType = "video/x-msvideo"
	case "mp3":
		contentType = "audio/mpeg"
	case "aac":
		contentType = "audio/aac"
	case "jpeg", "jpg":
		contentType = "image/jpeg"
	case "png":
		contentType = "image/png"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", item["name"]))
	w.Header().Set("Accept-Ranges", "bytes")
	itemPath, ok := item["path"].(string)
	if !ok || itemPath == "" {
		http.Error(w, "invalid item path", http.StatusInternalServerError)
		return
	}
	// Validate path is within media directories to prevent path traversal
	mediaDirs := s.mediaDirs
	if len(mediaDirs) == 0 {
		mediaDirs = []string{"./media"} // default media directory
	}
	if !isPathInMediaDirs(itemPath, mediaDirs) {
		http.Error(w, "path not in media directories", http.StatusForbidden)
		return
	}
	// #nosec G703 - path validated above by isPathInMediaDirs
	http.ServeFile(w, r, itemPath)
}

// isPathInMediaDirs checks if a path is within any of the allowed media directories
func isPathInMediaDirs(path string, mediaDirs []string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	for _, dir := range mediaDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir+string(filepath.Separator)) || absPath == absDir {
			return true
		}
	}
	return false
}
