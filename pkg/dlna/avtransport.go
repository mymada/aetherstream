package dlna

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// AVTransportState represents the current transport state
type AVTransportState string

const (
	AVStateStopped          AVTransportState = "STOPPED"
	AVStatePlaying          AVTransportState = "PLAYING"
	AVStatePaused           AVTransportState = "PAUSED_PLAYBACK"
	AVStateTransitioning    AVTransportState = "TRANSITIONING"
	AVStateNoMedia          AVTransportState = "NO_MEDIA_PRESENT"
)

// AVTransportInfo holds the state of an AVTransport instance
type AVTransportInfo struct {
	InstanceID      int
	TransportState  AVTransportState
	TransportStatus string
	PlaybackStorageMedium string
	CurrentURI      string
	CurrentURIMetaData string
	RelativeTimePosition string
	AbsoluteTimePosition string
	RelativeCounterPosition string
	AbsoluteCounterPosition string
	Volume          int
	LastUpdate      time.Time
}

var avTransportInstances = make(map[int]*AVTransportInfo)
var avTransportInstanceID = 0

func getOrCreateAVTransport(instanceID int) *AVTransportInfo {
	if info, ok := avTransportInstances[instanceID]; ok {
		return info
	}
	info := &AVTransportInfo{
		InstanceID:      instanceID,
		TransportState:  AVStateNoMedia,
		TransportStatus: "OK",
		Volume:          100,
		LastUpdate:      time.Now(),
	}
	avTransportInstances[instanceID] = info
	return info
}

func (s *Server) handleAVTransport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeSOAPError(w, 401, "Invalid args")
		return
	}
	defer r.Body.Close()

	action := extractSOAPActionHeader(r.Header.Get("SOAPAction"))
	if action == "" {
		action = extractXMLTag(string(body), "Action")
	}

	log.Debug().Str("action", action).Msg("AVTransport SOAP action")

	switch action {
	case "SetAVTransportURI":
		s.handleSetAVTransportURI(w, body)
	case "Play":
		s.handlePlay(w, body)
	case "Pause":
		s.handlePause(w, body)
	case "Stop":
		s.handleStop(w, body)
	case "Seek":
		s.handleSeek(w, body)
	case "GetPositionInfo":
		s.handleGetPositionInfo(w, body)
	case "GetTransportInfo":
		s.handleGetTransportInfo(w, body)
	case "GetMediaInfo":
		s.handleGetMediaInfo(w, body)
	default:
		s.writeSOAPError(w, 401, "Invalid action: "+action)
	}
}

func (s *Server) handleSetAVTransportURI(w http.ResponseWriter, body []byte) {
	instanceID := extractInstanceID(body)
	uri := extractXMLTag(string(body), "CurrentURI")

	info := getOrCreateAVTransport(instanceID)
	info.CurrentURI = uri
	info.TransportState = AVStateStopped
	info.LastUpdate = time.Now()

	log.Info().Str("uri", uri).Int("instance", instanceID).Msg("AVTransport URI set")

	s.writeAVTransportResponse(w, "SetAVTransportURI")
}

func (s *Server) handlePlay(w http.ResponseWriter, body []byte) {
	instanceID := extractInstanceID(body)
	info := getOrCreateAVTransport(instanceID)

	if info.CurrentURI == "" {
		s.writeSOAPError(w, 701, "Transition not available")
		return
	}

	info.TransportState = AVStatePlaying
	info.LastUpdate = time.Now()

	log.Info().Int("instance", instanceID).Msg("AVTransport Play")
	s.writeAVTransportResponse(w, "Play")
}

func (s *Server) handlePause(w http.ResponseWriter, body []byte) {
	instanceID := extractInstanceID(body)
	info := getOrCreateAVTransport(instanceID)

	if info.TransportState != AVStatePlaying {
		s.writeSOAPError(w, 701, "Transition not available")
		return
	}

	info.TransportState = AVStatePaused
	info.LastUpdate = time.Now()

	log.Info().Int("instance", instanceID).Msg("AVTransport Pause")
	s.writeAVTransportResponse(w, "Pause")
}

func (s *Server) handleStop(w http.ResponseWriter, body []byte) {
	instanceID := extractInstanceID(body)
	info := getOrCreateAVTransport(instanceID)

	info.TransportState = AVStateStopped
	info.RelativeTimePosition = "0:00:00"
	info.LastUpdate = time.Now()

	log.Info().Int("instance", instanceID).Msg("AVTransport Stop")
	s.writeAVTransportResponse(w, "Stop")
}

func (s *Server) handleSeek(w http.ResponseWriter, body []byte) {
	instanceID := extractInstanceID(body)
	target := extractXMLTag(string(body), "Target")
	unit := extractXMLTag(string(body), "Unit")

	info := getOrCreateAVTransport(instanceID)

	if unit == "REL_TIME" || unit == "ABS_TIME" {
		info.RelativeTimePosition = target
		info.AbsoluteTimePosition = target
	}
	info.LastUpdate = time.Now()

	log.Info().Int("instance", instanceID).Str("target", target).Msg("AVTransport Seek")
	s.writeAVTransportResponse(w, "Seek")
}

func (s *Server) handleGetPositionInfo(w http.ResponseWriter, body []byte) {
	instanceID := extractInstanceID(body)
	info := getOrCreateAVTransport(instanceID)

	response := fmt.Sprintf(`<u:GetPositionInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <Track>1</Track>
      <TrackDuration>0:00:00</TrackDuration>
      <TrackMetaData></TrackMetaData>
      <TrackURI>%s</TrackURI>
      <RelTime>%s</RelTime>
      <AbsTime>%s</AbsTime>
      <RelCount>0</RelCount>
      <AbsCount>0</AbsCount>
    </u:GetPositionInfoResponse>`,
		xmlEscape(info.CurrentURI),
		info.RelativeTimePosition,
		info.AbsoluteTimePosition)

	s.writeSOAPResponse(w, response)
}

func (s *Server) handleGetTransportInfo(w http.ResponseWriter, body []byte) {
	instanceID := extractInstanceID(body)
	info := getOrCreateAVTransport(instanceID)

	response := fmt.Sprintf(`<u:GetTransportInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <CurrentTransportState>%s</CurrentTransportState>
      <CurrentTransportStatus>%s</CurrentTransportStatus>
      <CurrentSpeed>1</CurrentSpeed>
    </u:GetTransportInfoResponse>`,
		info.TransportState,
		info.TransportStatus)

	s.writeSOAPResponse(w, response)
}

func (s *Server) handleGetMediaInfo(w http.ResponseWriter, body []byte) {
	instanceID := extractInstanceID(body)
	info := getOrCreateAVTransport(instanceID)

	response := fmt.Sprintf(`<u:GetMediaInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <NrTracks>1</NrTracks>
      <MediaDuration>0:00:00</MediaDuration>
      <CurrentURI>%s</CurrentURI>
      <CurrentURIMetaData></CurrentURIMetaData>
      <NextURI></NextURI>
      <NextURIMetaData></NextURIMetaData>
      <PlayMedium>NETWORK</PlayMedium>
      <RecordMedium>NOT_IMPLEMENTED</RecordMedium>
      <WriteStatus>NOT_IMPLEMENTED</WriteStatus>
    </u:GetMediaInfoResponse>`, xmlEscape(info.CurrentURI))

	s.writeSOAPResponse(w, response)
}

func (s *Server) writeAVTransportResponse(w http.ResponseWriter, actionName string) {
	response := fmt.Sprintf(`<u:%sResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"></u:%sResponse>`, actionName, actionName)
	s.writeSOAPResponse(w, response)
}

func (s *Server) writeSOAPResponse(w http.ResponseWriter, bodyContent string) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("EXT", "")
	output := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    %s
  </s:Body>
</s:Envelope>`, bodyContent)
	_, _ = w.Write([]byte(output))
}

func extractSOAPActionHeader(header string) string {
	header = strings.Trim(header, `"`)
	idx := strings.LastIndex(header, "#")
	if idx != -1 {
		return header[idx+1:]
	}
	return ""
}

func extractInstanceID(body []byte) int {
	s := string(body)
	idStr := extractXMLTag(s, "InstanceID")
	if idStr == "" {
		return 0
	}
	id, _ := strconv.Atoi(idStr)
	return id
}
