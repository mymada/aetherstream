package probe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- normalizeVideoCodec ---

func TestNormalizeVideoCodec(t *testing.T) {
	cases := []struct{ in, out string }{
		{"h264", "h264"},
		{"avc", "h264"},
		{"avc1", "h264"},
		{"hevc", "hevc"},
		{"h265", "hevc"},
		{"vp9", "vp9"},
		{"av1", "av1"},
		{"mpeg4", "mpeg"},
		{"mpeg2video", "mpeg"},
		{"unknown_codec", "unknown_codec"},
		{"", ""},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.out, normalizeVideoCodec(tc.in), "input=%q", tc.in)
	}
}

// --- normalizeAudioCodec ---

func TestNormalizeAudioCodec(t *testing.T) {
	cases := []struct{ in, out string }{
		{"aac", "aac"},
		{"mp4a", "aac"},
		{"ac3", "ac3"},
		{"eac3", "eac3"},
		{"dts", "dts"},
		{"opus", "opus"},
		{"mp3", "mp3"},
		{"mp2", "mp3"},
		{"flac", "flac"},
		{"vorbis", "vorbis"},
		{"pcm_s16le", "pcm_s16le"},
		{"", ""},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.out, normalizeAudioCodec(tc.in), "input=%q", tc.in)
	}
}

// --- isHDRTransfer ---

func TestIsHDRTransfer(t *testing.T) {
	hdr := []string{"smpte2084", "SMPTE2084", "pq", "PQ", "arib-std-b67", "hlg", "HLG"}
	for _, s := range hdr {
		assert.True(t, isHDRTransfer(s), "expected HDR: %q", s)
	}
	sdr := []string{"bt709", "bt601", "gamma28", "linear", ""}
	for _, s := range sdr {
		assert.False(t, isHDRTransfer(s), "expected SDR: %q", s)
	}
}

// --- isHDRPixelFormat ---

func TestIsHDRPixelFormat(t *testing.T) {
	hdr := []string{
		"yuv420p10le", "yuv422p10le", "yuv444p10le",
		"yuv420p12le", "yuv422p12le", "yuv444p12le",
		"p010le", "p016le",
	}
	for _, pf := range hdr {
		assert.True(t, isHDRPixelFormat(pf), "expected HDR pixel format: %q", pf)
	}
	sdr := []string{"yuv420p", "yuv422p", "yuv444p", "yuvj420p", "rgb24", ""}
	for _, pf := range sdr {
		assert.False(t, isHDRPixelFormat(pf), "expected SDR pixel format: %q", pf)
	}
}

// --- isValidLanguageCode ---

func TestIsValidLanguageCode(t *testing.T) {
	valid := []string{"en", "fr", "eng", "fra", "zh-CN", "pt-BR", "ZH"}
	for _, l := range valid {
		assert.True(t, isValidLanguageCode(l), "expected valid: %q", l)
	}
	invalid := []string{"", "en1", "123", "en_US", "f r", "e!"}
	for _, l := range invalid {
		assert.False(t, isValidLanguageCode(l), "expected invalid: %q", l)
	}
}

// --- parseResult ---

func TestParseResult_VideoAndAudio(t *testing.T) {
	r := &FFProbeResult{
		Format: Format{
			FormatName: "matroska,webm",
			Duration:   "120.5",
			BitRate:    "5000000",
			Size:       "75000000",
			ProbeScore: 100,
		},
		Streams: []Stream{
			{
				Index:     0,
				CodecType: "video",
				CodecName: "hevc",
				Width:     1920,
				Height:    1080,
				PixelFormat: "yuv420p10le",
				Tags:      map[string]string{"color_transfer": "smpte2084", "color_primaries": "bt2020"},
				Disposition: Disposition{Default: 1},
			},
			{
				Index:         1,
				CodecType:     "audio",
				CodecName:     "aac",
				Channels:      2,
				ChannelLayout: "stereo",
				SampleRate:    "48000",
				Tags:          map[string]string{"language": "eng"},
				Disposition:   Disposition{Default: 1},
			},
		},
	}

	info := parseResult(r, "/media/movie.mkv")
	require.NotNil(t, info)
	assert.Equal(t, "mkv", info.Container)
	assert.Equal(t, 120.5, info.Duration)
	assert.Equal(t, int64(5000000), info.BitRate)
	assert.Equal(t, int64(75000000), info.Size)
	assert.Equal(t, "hevc", info.VideoCodec)
	assert.Equal(t, "aac", info.AudioCodec)
	assert.True(t, info.IsHDR)
	assert.Equal(t, "smpte2084", info.ColorTransfer)
	assert.Equal(t, "bt2020", info.ColorPrimaries)
	require.NotNil(t, info.Video)
	assert.Equal(t, 1920, info.Video.Width)
	assert.Equal(t, 1080, info.Video.Height)
	require.NotNil(t, info.Audio)
	assert.Equal(t, "aac", info.Audio.Codec)
	assert.Equal(t, "eng", info.Audio.Language)
	assert.True(t, info.Audio.Default)
}

func TestParseResult_HDRFromPixelFormat(t *testing.T) {
	r := &FFProbeResult{
		Format: Format{FormatName: "mp4", Duration: "0", BitRate: "0", Size: "0"},
		Streams: []Stream{
			{
				Index:       0,
				CodecType:   "video",
				CodecName:   "h264",
				PixelFormat: "yuv420p10le",
				Tags:        map[string]string{},
				Disposition: Disposition{},
			},
		},
	}
	info := parseResult(r, "/media/video.mp4")
	assert.True(t, info.IsHDR, "HDR should be detected from pixel format")
}

func TestParseResult_MultipleAudio(t *testing.T) {
	r := &FFProbeResult{
		Format: Format{FormatName: "matroska", Duration: "60", BitRate: "8000000", Size: "60000000"},
		Streams: []Stream{
			{Index: 0, CodecType: "video", CodecName: "h264", Tags: map[string]string{}, Disposition: Disposition{}},
			{Index: 1, CodecType: "audio", CodecName: "ac3", Channels: 6, Tags: map[string]string{"language": "eng"}, Disposition: Disposition{Default: 1}},
			{Index: 2, CodecType: "audio", CodecName: "aac", Channels: 2, Tags: map[string]string{"language": "fra"}, Disposition: Disposition{}},
		},
	}
	info := parseResult(r, "/media/multi.mkv")
	assert.Len(t, info.AllAudio, 2)
	assert.Equal(t, "ac3", info.AudioCodec)
	assert.Equal(t, 0, info.AllAudio[0].SubIndex)
	assert.Equal(t, 1, info.AllAudio[1].SubIndex)
}

func TestParseResult_Subtitles(t *testing.T) {
	r := &FFProbeResult{
		Format: Format{FormatName: "matroska", Duration: "90", BitRate: "0", Size: "0"},
		Streams: []Stream{
			{Index: 0, CodecType: "video", CodecName: "h264", Tags: map[string]string{}, Disposition: Disposition{}},
			{Index: 1, CodecType: "subtitle", CodecName: "subrip", Tags: map[string]string{"language": "eng", "title": "English"}, Disposition: Disposition{Default: 1, Forced: 0}},
			{Index: 2, CodecType: "subtitle", CodecName: "ass", Tags: map[string]string{"language": "fra", "title": "French"}, Disposition: Disposition{Forced: 1}},
		},
	}
	info := parseResult(r, "/media/subs.mkv")
	assert.Len(t, info.Subtitles, 2)
	assert.Equal(t, 0, info.Subtitles[0].SubIndex)
	assert.Equal(t, "eng", info.Subtitles[0].Language)
	assert.True(t, info.Subtitles[0].Default)
	assert.Equal(t, 1, info.Subtitles[1].SubIndex)
	assert.True(t, info.Subtitles[1].Forced)
}

func TestParseResult_ContainerNoExt(t *testing.T) {
	r := &FFProbeResult{
		Format:  Format{FormatName: "mp4", Duration: "0", BitRate: "0", Size: "0"},
		Streams: []Stream{},
	}
	info := parseResult(r, "/media/videofile")
	assert.Equal(t, "", info.Container) // no extension
}

// --- ExtractSubtitleByIndex error cases ---

func TestExtractSubtitleByIndex_InvalidIndex(t *testing.T) {
	_, err := ExtractSubtitleByIndex("/any/path.mkv", -1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid subtitle index")

	_, err = ExtractSubtitleByIndex("/any/path.mkv", 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid subtitle index")
}
