package encoder

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- extractValue ---

func TestExtractValue(t *testing.T) {
	cases := []struct {
		line     string
		key      string
		expected string
	}{
		{"color_primaries=bt2020", "color_primaries=", "bt2020"},
		{"color_transfer=smpte2084", "color_transfer=", "smpte2084"},
		{"hdr_format=HDR10", "hdr_format=", "HDR10"},
		{"no_key_here", "missing=", ""},
		{`codec_name="hevc"`, "codec_name=", "hevc"},
	}
	for _, tc := range cases {
		got := extractValue(tc.line, tc.key)
		assert.Equal(t, tc.expected, got, "line=%q key=%q", tc.line, tc.key)
	}
}

// --- DefaultToneMapping ---

func TestDefaultToneMapping(t *testing.T) {
	opts := DefaultToneMapping()
	assert.Equal(t, ToneMapHable, opts.Mode)
	assert.Equal(t, 0.5, opts.Desat)
	assert.Equal(t, 100, opts.PeakNits)
	assert.Equal(t, 0, opts.SourceNits)
}

// --- BuildToneMappingFilter ---

func TestBuildToneMappingFilter(t *testing.T) {
	opts := ToneMappingOptions{Mode: ToneMapHable, Desat: 0.5, PeakNits: 100}
	filter := BuildToneMappingFilter(opts)
	assert.Contains(t, filter, "tonemap=hable")
	assert.Contains(t, filter, "desat=0.50")
	assert.Contains(t, filter, "peak=100")
	assert.Contains(t, filter, "zscale")

	// Empty mode falls back to hable
	opts2 := ToneMappingOptions{}
	f2 := BuildToneMappingFilter(opts2)
	assert.Contains(t, f2, "tonemap=hable")
	// Zero peak → 100
	assert.Contains(t, f2, "peak=100")

	// Desat clamping
	opts3 := ToneMappingOptions{Mode: ToneMapReinhard, Desat: 1.5, PeakNits: 200}
	f3 := BuildToneMappingFilter(opts3)
	assert.Contains(t, f3, "desat=1.00")

	opts4 := ToneMappingOptions{Mode: ToneMapClip, Desat: -0.5, PeakNits: 50}
	f4 := BuildToneMappingFilter(opts4)
	assert.Contains(t, f4, "desat=0.00")
}

// --- ApplyToneMappingToArgs ---

func TestApplyToneMappingToArgs_NoVf(t *testing.T) {
	args := []string{"ffmpeg", "-i", "input.mkv", "-c:v", "libx264", "output.mp4"}
	opts := DefaultToneMapping()
	result := ApplyToneMappingToArgs(args, opts)
	// Should have -vf inserted
	vfIdx := -1
	for i, a := range result {
		if a == "-vf" {
			vfIdx = i
			break
		}
	}
	assert.GreaterOrEqual(t, vfIdx, 0, "expected -vf flag")
	assert.Contains(t, result[vfIdx+1], "tonemap=hable")
}

func TestApplyToneMappingToArgs_ExistingVf(t *testing.T) {
	opts := DefaultToneMapping()
	args := []string{"ffmpeg", "-i", "input.mkv", "-vf", "scale=1920:1080", "output.mp4"}
	result := ApplyToneMappingToArgs(args, opts)
	// The existing -vf value should be extended
	for i, a := range result {
		if a == "-vf" {
			assert.Contains(t, result[i+1], "scale=1920:1080")
			assert.Contains(t, result[i+1], "tonemap=hable")
			return
		}
	}
	t.Fatal("-vf not found in result")
}

// --- parseHDRInfo ---

func TestParseHDRInfo_HDR10(t *testing.T) {
	raw := `color_primaries=bt2020
color_transfer=smpte2084
colorspace=bt2020nc
max_cll=1000
max_fall=400
side_data_type=HDR Dynamic Metadata SMPTE2094-40 (HDR10+)
`
	info := parseHDRInfo(raw)
	assert.True(t, info.IsHDR)
	assert.Equal(t, "bt2020", info.ColorPrimaries)
	assert.Equal(t, "smpte2084", info.ColorTransfer)
	assert.Equal(t, 1000, info.MaxCLL)
	assert.Equal(t, 400, info.MaxFALL)
	assert.Equal(t, "HDR10", info.HDRFormat)
	assert.NotEmpty(t, info.SideData)
}

func TestParseHDRInfo_HLG(t *testing.T) {
	raw := `color_primaries=bt2020
color_transfer=arib-std-b67
colorspace=bt2020nc
`
	info := parseHDRInfo(raw)
	assert.True(t, info.IsHDR)
	assert.Equal(t, "HLG", info.HDRFormat)
}

func TestParseHDRInfo_SDR(t *testing.T) {
	raw := `color_primaries=bt709
color_transfer=bt709
colorspace=bt709
`
	info := parseHDRInfo(raw)
	assert.False(t, info.IsHDR)
	assert.Equal(t, "", info.HDRFormat)
}

func TestParseHDRInfo_ExplicitFormat(t *testing.T) {
	raw := `color_primaries=bt2020
color_transfer=smpte2084
hdr_format=HDR10+
`
	info := parseHDRInfo(raw)
	assert.True(t, info.IsHDR)
	assert.Equal(t, "HDR10+", info.HDRFormat)
}

func TestParseHDRInfo_MasteringDisplay(t *testing.T) {
	raw := `mastering_display=G(13250,34500)B(7500,3000)R(34000,16000)WP(15635,16450)L(10000000,1)
`
	info := parseHDRInfo(raw)
	assert.NotEmpty(t, info.MasteringDisplay)
}

func TestParseHDRInfo_Empty(t *testing.T) {
	info := parseHDRInfo("")
	assert.False(t, info.IsHDR)
}

// --- parseDolbyVisionInfo ---

func TestParseDolbyVisionInfo_Detected(t *testing.T) {
	raw := `codec_name=hevc
side_data_type=Dolby Vision Metadata
profile=5
level=09
rpu_present=1
bl_present=1
el_present=0
`
	info := parseDolbyVisionInfo(raw)
	assert.True(t, info.IsDolbyVision)
	assert.Equal(t, "hevc", info.Profile.Codec)
	assert.Equal(t, 5, info.Profile.Profile)
	assert.Equal(t, "09", info.Profile.Level)
	assert.True(t, info.Profile.RPUPresent)
	assert.True(t, info.Profile.BLPresent)
	assert.False(t, info.Profile.ELPresent)
	assert.Equal(t, "BL+RPU", info.Profile.Compatibility)
}

func TestParseDolbyVisionInfo_WithEL(t *testing.T) {
	raw := `side_data_type=dolby vision metadata
profile=7
rpu_present=1
bl_present=1
el_present=1
`
	info := parseDolbyVisionInfo(raw)
	assert.True(t, info.IsDolbyVision)
	assert.Equal(t, "BL+EL+RPU", info.Profile.Compatibility)
}

func TestParseDolbyVisionInfo_ExplicitCompatibility(t *testing.T) {
	raw := `side_data_type=Dolby Vision Metadata
compatibility=BL+RPU
`
	info := parseDolbyVisionInfo(raw)
	assert.True(t, info.IsDolbyVision)
	assert.Equal(t, "BL+RPU", info.Profile.Compatibility)
}

func TestParseDolbyVisionInfo_NotDolby(t *testing.T) {
	raw := `codec_name=h264
color_primaries=bt709
`
	info := parseDolbyVisionInfo(raw)
	assert.False(t, info.IsDolbyVision)
	assert.Equal(t, "", info.Profile.Compatibility)
}

func TestParseDolbyVisionInfo_BLOnly(t *testing.T) {
	raw := `side_data_type=Dolby Vision Metadata
rpu_present=0
bl_present=1
el_present=0
`
	info := parseDolbyVisionInfo(raw)
	assert.True(t, info.IsDolbyVision)
	assert.Equal(t, "BL", info.Profile.Compatibility)
}

// --- DefaultDolbyVisionOptions ---

func TestDefaultDolbyVisionOptions(t *testing.T) {
	opts := DefaultDolbyVisionOptions()
	assert.False(t, opts.PreserveDolbyVision)
	assert.True(t, opts.FallbackToHDR10)
	assert.True(t, opts.FallbackToSDR)
	assert.Equal(t, ToneMapHable, opts.ToneMapOpts.Mode)
}

// --- BuildDolbyVisionFilter ---

func TestBuildDolbyVisionFilter_Nil(t *testing.T) {
	assert.Equal(t, "", BuildDolbyVisionFilter(nil, DefaultDolbyVisionOptions()))
}

func TestBuildDolbyVisionFilter_NotDV(t *testing.T) {
	dv := &DolbyVisionInfo{IsDolbyVision: false}
	assert.Equal(t, "", BuildDolbyVisionFilter(dv, DefaultDolbyVisionOptions()))
}

func TestBuildDolbyVisionFilter_Preserve(t *testing.T) {
	dv := &DolbyVisionInfo{IsDolbyVision: true}
	opts := DolbyVisionTranscodeOptions{PreserveDolbyVision: true}
	assert.Equal(t, "copy", BuildDolbyVisionFilter(dv, opts))
}

func TestBuildDolbyVisionFilter_FallbackHDR10(t *testing.T) {
	dv := &DolbyVisionInfo{IsDolbyVision: true}
	opts := DolbyVisionTranscodeOptions{FallbackToHDR10: true}
	assert.Equal(t, "dolbyvision=dovi=1:el=0", BuildDolbyVisionFilter(dv, opts))
}

func TestBuildDolbyVisionFilter_FallbackSDR(t *testing.T) {
	dv := &DolbyVisionInfo{IsDolbyVision: true}
	opts := DolbyVisionTranscodeOptions{
		FallbackToSDR: true,
		ToneMapOpts:   DefaultToneMapping(),
	}
	filter := BuildDolbyVisionFilter(dv, opts)
	assert.True(t, strings.Contains(filter, "tonemap=hable"), "expected tone mapping filter")
}

func TestBuildDolbyVisionFilter_NoFallback(t *testing.T) {
	dv := &DolbyVisionInfo{IsDolbyVision: true}
	opts := DolbyVisionTranscodeOptions{}
	assert.Equal(t, "", BuildDolbyVisionFilter(dv, opts))
}

// --- DolbyVisionSupportedProfiles / IsProfileSupported ---

func TestDolbyVisionSupportedProfiles(t *testing.T) {
	profiles := DolbyVisionSupportedProfiles()
	assert.Contains(t, profiles, 5)
	assert.Contains(t, profiles, 8)
}

func TestIsProfileSupported(t *testing.T) {
	assert.True(t, IsProfileSupported(5))
	assert.True(t, IsProfileSupported(8))
	assert.False(t, IsProfileSupported(7))
	assert.False(t, IsProfileSupported(0))
}
