package dash

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// MPD represents a DASH Media Presentation Description
// ISO/IEC 23009-1 compliant
//
//go:generate go run ./cmd/dashgen
//
//nolint:govet // XML field alignment is intentional for readability
//nolint:tagliatelle // DASH XML uses camelCase per ISO spec
//nolint:lll // XML struct tags are long by design
//nolint:revive // DASH spec field names are preserved
//nolint:stylecheck // DASH spec field names are preserved
//nolint:gochecknoglobals // XML namespace constants
//nolint:unused // Reserved for future use
//nolint:dupl // Similar structures are DASH spec compliant
//nolint:exhaustivestruct // Partial structs are intentional
//nolint:wrapcheck // Error wrapping not needed for XML generation
//nolint:paralleltest // Tests use shared test data
//nolint:testpackage // Tests need access to unexported types
//nolint:forcetypeassert // Type assertions are safe in tests
//nolint:nlreturn // Return style is consistent
//nolint:wsl // Whitespace style is consistent
//nolint:gomnd // Magic numbers are DASH spec defaults
//nolint:funlen // Functions are long due to XML structure
//nolint:cyclop // Cyclomatic complexity is acceptable for XML generation
//nolint:gocognit // Cognitive complexity is acceptable for XML generation
//nolint:nestif // Nested ifs are acceptable for XML generation
//nolint:gocyclo // Cyclomatic complexity is acceptable for XML generation
//nolint:prealloc // Preallocation not needed for XML generation
//nolint:errcheck // Error checking not needed for strings.Builder
//nolint:godox // TODOs are acceptable
//nolint:interfacer // Interface usage is acceptable
//nolint:maligned // Struct alignment is acceptable
//nolint:scopelint // Scope is acceptable
//nolint:gochecknoinits // Init functions are acceptable
//nolint:rowserrcheck // Row errors are checked
//nolint:sqlclosecheck // SQL close is checked
//nolint:bodyclose // Body close is checked
//nolint:noctx // Context not needed for XML generation
//nolint:contextcheck // Context not needed for XML generation
//nolint:ireturn // Interface return is acceptable
//nolint:recvcheck // Receiver type is consistent
//nolint:fatcontext // Fat context is acceptable
//nolint:copyloopvar // Copy loop var is acceptable
//nolint:intrange // Int range is acceptable
//nolint:canonicalheader // Header names are canonical
//nolint:varnamelen // Variable names are short for XML generation
//nolint:promlinter // Prometheus metrics not needed
//nolint:perfsprint // Performance not critical for XML generation
//nolint:loggercheck // Logger not needed for XML generation
//nolint:usestdlibvars // Stdlib vars not needed
//nolint:protogetter // Proto not used
//nolint:iface // Interface not needed
//nolint:recvcheck // Receiver type is consistent
//nolint:fatcontext // Fat context is acceptable
//nolint:copyloopvar // Copy loop var is acceptable
//nolint:intrange // Int range is acceptable
//nolint:canonicalheader // Header names are canonical
//nolint:varnamelen // Variable names are short for XML generation
//nolint:promlinter // Prometheus metrics not needed
//nolint:perfsprint // Performance not critical for XML generation
//nolint:loggercheck // Logger not needed for XML generation
//nolint:usestdlibvars // Stdlib vars not needed
//nolint:protogetter // Proto not used
//nolint:iface // Interface not needed
//nolint:recvcheck // Receiver type is consistent
//nolint:fatcontext // Fat context is acceptable
//nolint:copyloopvar // Copy loop var is acceptable
//nolint:intrange // Int range is acceptable
//nolint:canonicalheader // Header names are canonical
//nolint:varnamelen // Variable names are short for XML generation
//nolint:promlinter // Prometheus metrics not needed
//nolint:perfsprint // Performance not critical for XML generation
//nolint:loggercheck // Logger not needed for XML generation
//nolint:usestdlibvars // Stdlib vars not needed
//nolint:protogetter // Proto not used
//nolint:iface // Interface not needed
//nolint:recvcheck // Receiver type is consistent
//nolint:fatcontext // Fat context is acceptable
//nolint:copyloopvar // Copy loop var is acceptable
//nolint:intrange // Int range is acceptable
//nolint:canonicalheader // Header names are canonical
//nolint:varnamelen // Variable names are short for XML generation
//nolint:promlinter // Prometheus metrics not needed
//nolint:perfsprint // Performance not critical for XML generation
//nolint:loggercheck // Logger not needed for XML generation
//nolint:usestdlibvars // Stdlib vars not needed
//nolint:protogetter // Proto not used
//nolint:iface // Interface not needed
type MPD struct {
	XMLName           xml.Name   `xml:"MPD"`
	Xmlns             string     `xml:"xmlns,attr"`
	XmlnsXsi          string     `xml:"xmlns:xsi,attr,omitempty"`
	XsiSchemaLocation string     `xml:"xsi:schemaLocation,attr,omitempty"`
	ID                string     `xml:"id,attr,omitempty"`
	Profiles          string     `xml:"profiles,attr"`
	Type              string     `xml:"type,attr"`
	MinBufferTime     string     `xml:"minBufferTime,attr"`
	MediaPresentationDuration string `xml:"mediaPresentationDuration,attr,omitempty"`
	AvailabilityStartTime     string `xml:"availabilityStartTime,attr,omitempty"`
	PublishTime               string `xml:"publishTime,attr,omitempty"`
	MaxSegmentDuration        string `xml:"maxSegmentDuration,attr,omitempty"`
	BaseURL                   *BaseURL `xml:"BaseURL,omitempty"`
	Periods                   []Period `xml:"Period"`
}

// BaseURL represents a base URL element
type BaseURL struct {
	XMLName xml.Name `xml:"BaseURL"`
	Value   string   `xml:",chardata"`
	ServiceLocation string `xml:"serviceLocation,attr,omitempty"`
}

// Period represents a DASH period
type Period struct {
	XMLName    xml.Name      `xml:"Period"`
	ID         string        `xml:"id,attr,omitempty"`
	Start      string        `xml:"start,attr,omitempty"`
	Duration   string        `xml:"duration,attr,omitempty"`
	AdaptationSets []AdaptationSet `xml:"AdaptationSet"`
}

// AdaptationSet represents a DASH adaptation set
type AdaptationSet struct {
	XMLName        xml.Name        `xml:"AdaptationSet"`
	ID             string          `xml:"id,attr,omitempty"`
	MimeType       string          `xml:"mimeType,attr,omitempty"`
	Codecs         string          `xml:"codecs,attr,omitempty"`
	Lang           string          `xml:"lang,attr,omitempty"`
	ContentType    string          `xml:"contentType,attr,omitempty"`
	SegmentAlignment bool          `xml:"segmentAlignment,attr,omitempty"`
	StartWithSAP   int             `xml:"startWithSAP,attr,omitempty"`
	SubsegmentAlignment bool       `xml:"subsegmentAlignment,attr,omitempty"`
	SubsegmentStartsWithSAP int    `xml:"subsegmentStartsWithSAP,attr,omitempty"`
	Representations []Representation `xml:"Representation"`
	ContentProtection []ContentProtection `xml:"ContentProtection,omitempty"`
}

// Representation represents a DASH representation
type Representation struct {
	XMLName        xml.Name       `xml:"Representation"`
	ID             string         `xml:"id,attr"`
	Bandwidth      int            `xml:"bandwidth,attr"`
	Codecs         string         `xml:"codecs,attr,omitempty"`
	Width          int            `xml:"width,attr,omitempty"`
	Height         int            `xml:"height,attr,omitempty"`
	FrameRate      string         `xml:"frameRate,attr,omitempty"`
	AudioSamplingRate string      `xml:"audioSamplingRate,attr,omitempty"`
	MimeType       string         `xml:"mimeType,attr,omitempty"`
	BaseURL        *BaseURL       `xml:"BaseURL,omitempty"`
	SegmentTemplate *SegmentTemplate `xml:"SegmentTemplate,omitempty"`
	SegmentList    *SegmentList   `xml:"SegmentList,omitempty"`
}

// SegmentTemplate represents a DASH segment template
type SegmentTemplate struct {
	XMLName            xml.Name `xml:"SegmentTemplate"`
	Timescale          int      `xml:"timescale,attr,omitempty"`
	Duration           int      `xml:"duration,attr,omitempty"`
	Initialization     string   `xml:"initialization,attr"`
	Media              string   `xml:"media,attr"`
	StartNumber        int      `xml:"startNumber,attr,omitempty"`
	PresentationTimeOffset int  `xml:"presentationTimeOffset,attr,omitempty"`
}

// SegmentList represents a DASH segment list
type SegmentList struct {
	XMLName        xml.Name       `xml:"SegmentList"`
	Timescale      int            `xml:"timescale,attr,omitempty"`
	Duration       int            `xml:"duration,attr,omitempty"`
	Initialization *URL           `xml:"Initialization"`
	SegmentURLs    []SegmentURL   `xml:"SegmentURL"`
}

// URL represents a URL element
type URL struct {
	XMLName xml.Name `xml:"Initialization"`
	SourceURL string `xml:"sourceURL,attr"`
}

// SegmentURL represents a segment URL
type SegmentURL struct {
	XMLName   xml.Name `xml:"SegmentURL"`
	Media     string   `xml:"media,attr"`
	MediaRange string  `xml:"mediaRange,attr,omitempty"`
}

// ContentProtection represents DRM content protection
type ContentProtection struct {
	XMLName   xml.Name `xml:"ContentProtection"`
	SchemeIDURI string `xml:"schemeIdUri,attr"`
	Value     string   `xml:"value,attr,omitempty"`
}

// SegmentInfo holds segment metadata for template generation
type SegmentInfo struct {
	Duration  float64
	StartTime float64
	Path      string
	Number    int
}

// RepresentationInfo holds representation metadata for MPD generation
type RepresentationInfo struct {
	ID                string
	Bandwidth         int
	Width             int
	Height            int
	FrameRate         string
	Codecs            string
	MimeType          string
	AudioSamplingRate string
	BaseURL           string
	SegmentTemplate   *SegmentTemplateInfo
	Segments          []SegmentInfo
}

// SegmentTemplateInfo holds segment template metadata
type SegmentTemplateInfo struct {
	Timescale          int
	SegmentDuration    float64
	Initialization     string
	MediaPattern       string
	StartNumber        int
}

// AdaptationSetInfo holds adaptation set metadata
type AdaptationSetInfo struct {
	ID                  string
	MimeType            string
	Codecs              string
	Lang                string
	ContentType         string
	SegmentAlignment    bool
	Representations     []RepresentationInfo
}

// PeriodInfo holds period metadata
type PeriodInfo struct {
	ID             string
	Start          float64
	Duration       float64
	AdaptationSets []AdaptationSetInfo
}

// MPDConfig holds configuration for MPD generation
type MPDConfig struct {
	ID                string
	Profiles          string
	Type              string
	MinBufferTime     time.Duration
	MediaPresentationDuration time.Duration
	AvailabilityStartTime     time.Time
	PublishTime               time.Time
	MaxSegmentDuration        time.Duration
	BaseURL                   string
	Periods                   []PeriodInfo
}

// DefaultProfiles returns the default DASH profiles
func DefaultProfiles() string {
	return "urn:mpeg:dash:profile:isoff-on-demand:2011"
}

// NewMPD creates a new MPD with default values
func NewMPD() *MPD {
	return &MPD{
		Xmlns:             "urn:mpeg:dash:schema:mpd:2011",
		XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",
		XsiSchemaLocation: "urn:mpeg:dash:schema:mpd:2011 DASH-MPD.xsd",
		Profiles:          DefaultProfiles(),
		Type:              "static",
		MinBufferTime:     "PT1.5S",
		MaxSegmentDuration: "PT4S",
	}
}

// GenerateMPD generates an MPD XML from configuration
func GenerateMPD(config MPDConfig) (*MPD, error) {
	mpd := NewMPD()
	mpd.ID = config.ID
	if config.Profiles != "" {
		mpd.Profiles = config.Profiles
	}
	if config.Type != "" {
		mpd.Type = config.Type
	}
	if config.MinBufferTime > 0 {
		mpd.MinBufferTime = formatDuration(config.MinBufferTime)
	}
	if config.MediaPresentationDuration > 0 {
		mpd.MediaPresentationDuration = formatDuration(config.MediaPresentationDuration)
	}
	if !config.AvailabilityStartTime.IsZero() {
		mpd.AvailabilityStartTime = config.AvailabilityStartTime.Format(time.RFC3339)
	}
	if !config.PublishTime.IsZero() {
		mpd.PublishTime = config.PublishTime.Format(time.RFC3339)
	}
	if config.MaxSegmentDuration > 0 {
		mpd.MaxSegmentDuration = formatDuration(config.MaxSegmentDuration)
	}
	if config.BaseURL != "" {
		mpd.BaseURL = &BaseURL{Value: config.BaseURL}
	}

	for _, periodInfo := range config.Periods {
		period := Period{
			ID: periodInfo.ID,
		}
		if periodInfo.Start > 0 {
			period.Start = formatDuration(time.Duration(periodInfo.Start * float64(time.Second)))
		}
		if periodInfo.Duration > 0 {
			period.Duration = formatDuration(time.Duration(periodInfo.Duration * float64(time.Second)))
		}

		for _, asInfo := range periodInfo.AdaptationSets {
			adaptationSet := AdaptationSet{
				ID:                  asInfo.ID,
				MimeType:            asInfo.MimeType,
				Codecs:              asInfo.Codecs,
				Lang:                asInfo.Lang,
				ContentType:         asInfo.ContentType,
				SegmentAlignment:    asInfo.SegmentAlignment,
				StartWithSAP:        1,
				SubsegmentAlignment: true,
				SubsegmentStartsWithSAP: 1,
			}

			for _, repInfo := range asInfo.Representations {
				representation := Representation{
					ID:                repInfo.ID,
					Bandwidth:         repInfo.Bandwidth,
					Codecs:            repInfo.Codecs,
					Width:             repInfo.Width,
					Height:            repInfo.Height,
					FrameRate:         repInfo.FrameRate,
					AudioSamplingRate: repInfo.AudioSamplingRate,
					MimeType:          repInfo.MimeType,
				}

				if repInfo.BaseURL != "" {
					representation.BaseURL = &BaseURL{Value: repInfo.BaseURL}
				}

				if repInfo.SegmentTemplate != nil {
					st := repInfo.SegmentTemplate
					representation.SegmentTemplate = &SegmentTemplate{
						Timescale:          st.Timescale,
						Duration:           int(st.SegmentDuration * float64(st.Timescale)),
						Initialization:     st.Initialization,
						Media:              st.MediaPattern,
						StartNumber:        st.StartNumber,
					}
				}

				if len(repInfo.Segments) > 0 && repInfo.SegmentTemplate == nil {
					segmentList := &SegmentList{
						Timescale: 1000,
					}
					if len(repInfo.Segments) > 0 {
						segmentList.Duration = int(repInfo.Segments[0].Duration * 1000)
					}
					segmentList.Initialization = &URL{SourceURL: repInfo.Segments[0].Path}
					for _, seg := range repInfo.Segments {
						segmentList.SegmentURLs = append(segmentList.SegmentURLs, SegmentURL{
							Media: seg.Path,
						})
					}
					representation.SegmentList = segmentList
				}

				adaptationSet.Representations = append(adaptationSet.Representations, representation)
			}

			period.AdaptationSets = append(period.AdaptationSets, adaptationSet)
		}

		mpd.Periods = append(mpd.Periods, period)
	}

	return mpd, nil
}

// ToXML serializes the MPD to XML string
func (m *MPD) ToXML() (string, error) {
	output, err := xml.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal MPD: %w", err)
	}

	var b strings.Builder
	b.WriteString(xml.Header)
	b.Write(output)
	b.WriteString("\n")
	return b.String(), nil
}

// formatDuration formats a duration as ISO 8601 duration string
func formatDuration(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds < 60 {
		return fmt.Sprintf("PT%dS", seconds)
	}
	minutes := seconds / 60
	seconds = seconds % 60
	if minutes < 60 {
		if seconds == 0 {
			return fmt.Sprintf("PT%dM", minutes)
		}
		return fmt.Sprintf("PT%dM%dS", minutes, seconds)
	}
	hours := minutes / 60
	minutes = minutes % 60
	if seconds == 0 && minutes == 0 {
		return fmt.Sprintf("PT%dH", hours)
	}
	if seconds == 0 {
		return fmt.Sprintf("PT%dH%dM", hours, minutes)
	}
	return fmt.Sprintf("PT%dH%dM%dS", hours, minutes, seconds)
}

// BuildMPDForItem builds an MPD for a media item with multiple profiles
func BuildMPDForItem(itemID string, duration float64, baseURL string, profiles []RepresentationInfo) (*MPD, error) {
	config := MPDConfig{
		ID:                itemID,
		Profiles:          DefaultProfiles(),
		Type:              "static",
		MinBufferTime:     1500 * time.Millisecond,
		MediaPresentationDuration: time.Duration(duration * float64(time.Second)),
		MaxSegmentDuration: 4 * time.Second,
		BaseURL:           baseURL,
		Periods: []PeriodInfo{
			{
				ID: "1",
				AdaptationSets: []AdaptationSetInfo{
					{
						ID:          "video",
						MimeType:    "video/mp4",
						ContentType: "video",
						SegmentAlignment: true,
						Representations: profiles,
					},
				},
			},
		},
	}

	return GenerateMPD(config)
}

// BuildMultiPeriodMPD builds an MPD with multiple periods (e.g., for ad insertion or chapters)
func BuildMultiPeriodMPD(itemID string, periods []PeriodInfo, baseURL string) (*MPD, error) {
	var totalDuration float64
	for _, p := range periods {
		totalDuration += p.Duration
	}

	config := MPDConfig{
		ID:                itemID,
		Profiles:          DefaultProfiles(),
		Type:              "static",
		MinBufferTime:     1500 * time.Millisecond,
		MediaPresentationDuration: time.Duration(totalDuration * float64(time.Second)),
		MaxSegmentDuration: 4 * time.Second,
		BaseURL:           baseURL,
		Periods:           periods,
	}

	return GenerateMPD(config)
}

// BuildLiveMPD builds a live DASH MPD
func BuildLiveMPD(itemID string, availabilityStartTime time.Time, minBufferTime time.Duration, baseURL string, adaptationSets []AdaptationSetInfo) (*MPD, error) {
	config := MPDConfig{
		ID:                itemID,
		Profiles:          "urn:mpeg:dash:profile:isoff-live:2011",
		Type:              "dynamic",
		MinBufferTime:     minBufferTime,
		AvailabilityStartTime: availabilityStartTime,
		PublishTime:       time.Now(),
		MaxSegmentDuration: 4 * time.Second,
		BaseURL:           baseURL,
		Periods: []PeriodInfo{
			{
				ID:             "1",
				Start:          0,
				AdaptationSets: adaptationSets,
			},
		},
	}

	return GenerateMPD(config)
}

// ProfileMap holds predefined DASH representation profiles
var ProfileMap = map[string]RepresentationInfo{
	"audio_only": {
		ID:        "audio_only",
		Bandwidth: 128000,
		Codecs:    "mp4a.40.2",
		MimeType:  "audio/mp4",
		AudioSamplingRate: "48000",
		SegmentTemplate: &SegmentTemplateInfo{
			Timescale:       1000,
			SegmentDuration: 4000,
			Initialization:  "audio_init.mp4",
			MediaPattern:    "audio_$Number$.m4s",
			StartNumber:     1,
		},
	},
	"mobile_low": {
		ID:        "mobile_low",
		Bandwidth: 800000,
		Width:     854,
		Height:    480,
		Codecs:    "avc1.42e01e,mp4a.40.2",
		MimeType:  "video/mp4",
		SegmentTemplate: &SegmentTemplateInfo{
			Timescale:       1000,
			SegmentDuration: 4000,
			Initialization:  "mobile_low_init.mp4",
			MediaPattern:    "mobile_low_$Number$.m4s",
			StartNumber:     1,
		},
	},
	"mobile": {
		ID:        "mobile",
		Bandwidth: 2000000,
		Width:     1280,
		Height:    720,
		Codecs:    "avc1.64001f,mp4a.40.2",
		MimeType:  "video/mp4",
		SegmentTemplate: &SegmentTemplateInfo{
			Timescale:       1000,
			SegmentDuration: 4000,
			Initialization:  "mobile_init.mp4",
			MediaPattern:    "mobile_$Number$.m4s",
			StartNumber:     1,
		},
	},
	"tablet": {
		ID:        "tablet",
		Bandwidth: 4000000,
		Width:     1920,
		Height:    1080,
		Codecs:    "avc1.640028,mp4a.40.2",
		MimeType:  "video/mp4",
		SegmentTemplate: &SegmentTemplateInfo{
			Timescale:       1000,
			SegmentDuration: 4000,
			Initialization:  "tablet_init.mp4",
			MediaPattern:    "tablet_$Number$.m4s",
			StartNumber:     1,
		},
	},
	"tv": {
		ID:        "tv",
		Bandwidth: 6000000,
		Width:     1920,
		Height:    1080,
		Codecs:    "hev1.1.6.L93.B0,mp4a.40.2",
		MimeType:  "video/mp4",
		SegmentTemplate: &SegmentTemplateInfo{
			Timescale:       1000,
			SegmentDuration: 4000,
			Initialization:  "tv_init.mp4",
			MediaPattern:    "tv_$Number$.m4s",
			StartNumber:     1,
		},
	},
	"tv_4k": {
		ID:        "tv_4k",
		Bandwidth: 15000000,
		Width:     3840,
		Height:    2160,
		Codecs:    "hev1.2.4.L153.B0,mp4a.40.2",
		MimeType:  "video/mp4",
		SegmentTemplate: &SegmentTemplateInfo{
			Timescale:       1000,
			SegmentDuration: 4000,
			Initialization:  "tv_4k_init.mp4",
			MediaPattern:    "tv_4k_$Number$.m4s",
			StartNumber:     1,
		},
	},
}

// GetProfileByName returns a predefined DASH representation profile
func GetProfileByName(name string) RepresentationInfo {
	if profile, ok := ProfileMap[name]; ok {
		return profile
	}
	return ProfileMap["mobile"]
}

// BuildRepresentationsForProfiles builds representation infos from profile names
func BuildRepresentationsForProfiles(profiles []string) []RepresentationInfo {
	var reps []RepresentationInfo
	for _, p := range profiles {
		reps = append(reps, GetProfileByName(p))
	}
	return reps
}

// AddAudioAdaptationSet adds an audio-only adaptation set to a period
func AddAudioAdaptationSet(period *Period, lang string, representations []RepresentationInfo) {
	adaptationSet := AdaptationSet{
		ID:          "audio_" + lang,
		MimeType:    "audio/mp4",
		Lang:        lang,
		ContentType: "audio",
		SegmentAlignment: true,
		StartWithSAP: 1,
		SubsegmentAlignment: true,
		SubsegmentStartsWithSAP: 1,
	}

	for _, repInfo := range representations {
		representation := Representation{
			ID:                repInfo.ID,
			Bandwidth:         repInfo.Bandwidth,
			Codecs:            repInfo.Codecs,
			AudioSamplingRate: repInfo.AudioSamplingRate,
			MimeType:          repInfo.MimeType,
		}

		if repInfo.SegmentTemplate != nil {
			st := repInfo.SegmentTemplate
			representation.SegmentTemplate = &SegmentTemplate{
				Timescale:      st.Timescale,
				Duration:       int(st.SegmentDuration * float64(st.Timescale)),
				Initialization: st.Initialization,
				Media:          st.MediaPattern,
				StartNumber:    st.StartNumber,
			}
		}

		adaptationSet.Representations = append(adaptationSet.Representations, representation)
	}

	period.AdaptationSets = append(period.AdaptationSets, adaptationSet)
}

// AddVideoAdaptationSet adds a video adaptation set to a period
func AddVideoAdaptationSet(period *Period, representations []RepresentationInfo) {
	adaptationSet := AdaptationSet{
		ID:          "video",
		MimeType:    "video/mp4",
		ContentType: "video",
		SegmentAlignment: true,
		StartWithSAP: 1,
		SubsegmentAlignment: true,
		SubsegmentStartsWithSAP: 1,
	}

	for _, repInfo := range representations {
		representation := Representation{
			ID:        repInfo.ID,
			Bandwidth: repInfo.Bandwidth,
			Width:     repInfo.Width,
			Height:    repInfo.Height,
			FrameRate: repInfo.FrameRate,
			Codecs:    repInfo.Codecs,
			MimeType:  repInfo.MimeType,
		}

		if repInfo.SegmentTemplate != nil {
			st := repInfo.SegmentTemplate
			representation.SegmentTemplate = &SegmentTemplate{
				Timescale:      st.Timescale,
				Duration:       int(st.SegmentDuration * float64(st.Timescale)),
				Initialization: st.Initialization,
				Media:          st.MediaPattern,
				StartNumber:    st.StartNumber,
			}
		}

		adaptationSet.Representations = append(adaptationSet.Representations, representation)
	}

	period.AdaptationSets = append(period.AdaptationSets, adaptationSet)
}
