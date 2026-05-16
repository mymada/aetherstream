package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const envSecret = "test-secret-at-least-32-chars-long!!"

func setTestSecret(t *testing.T) {
	t.Helper()
	t.Setenv("AETHERSTREAM_AUTH_SECRET", envSecret)
}

func TestLoad_MissingSecret_ReturnsError(t *testing.T) {
	// Setting to empty triggers the "required" error
	t.Setenv("AETHERSTREAM_AUTH_SECRET", "")
	_, err := Load("/nonexistent/config.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AETHERSTREAM_AUTH_SECRET")
}

func TestLoad_ServerPortOverride(t *testing.T) {
	setTestSecret(t)
	t.Setenv("AETHERSTREAM_SERVER_PORT", "9999")
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, 9999, cfg.Server.Port)
}

func TestLoad_MetricsPortOverride(t *testing.T) {
	setTestSecret(t)
	t.Setenv("AETHERSTREAM_METRICS_PORT", "9191")
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, 9191, cfg.Server.MetricsPort)
}

func TestLoad_DLNAPortOverride(t *testing.T) {
	setTestSecret(t)
	t.Setenv("AETHERSTREAM_DLNA_PORT", "8888")
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, 8888, cfg.Server.DLNAPort)
}

func TestLoad_HostOverride(t *testing.T) {
	setTestSecret(t)
	t.Setenv("AETHERSTREAM_SERVER_HOST", "192.168.1.1")
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.1", cfg.Server.Host)
}

func TestLoad_TokenTTLOverride(t *testing.T) {
	setTestSecret(t)
	t.Setenv("AETHERSTREAM_AUTH_TOKEN_TTL", "48")
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, 48, cfg.Auth.TokenTTL)
}

func TestLoad_DatabasePathOverride(t *testing.T) {
	setTestSecret(t)
	t.Setenv("AETHERSTREAM_DATABASE_PATH", "/custom/db.sqlite")
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, "/custom/db.sqlite", cfg.Database.Path)
}

func TestLoad_DataDirOverride(t *testing.T) {
	setTestSecret(t)
	t.Setenv("DATA_DIR", "/data")
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, "/data/aetherstream.db", cfg.Database.Path)
}

func TestLoad_MediaDirOverride(t *testing.T) {
	setTestSecret(t)
	t.Setenv("MEDIA_DIR", "/media")
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, "/media", cfg.Server.StaticPath)
	assert.Equal(t, "/media", cfg.Server.MediaRoot)
}

func TestLoad_AllowedOriginsOverride(t *testing.T) {
	setTestSecret(t)
	t.Setenv("AETHERSTREAM_ALLOWED_ORIGINS", "http://a.com,http://b.com")
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, []string{"http://a.com", "http://b.com"}, cfg.Server.AllowedOrigins)
}

func TestLoad_WebUIPathOverride(t *testing.T) {
	setTestSecret(t)
	t.Setenv("AETHERSTREAM_WEB_UI_PATH", "/custom/ui")
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, "/custom/ui", cfg.Server.WebUIPath)
}

func TestLoad_FFmpegPathOverride(t *testing.T) {
	setTestSecret(t)
	t.Setenv("AETHERSTREAM_FFMPEG_PATH", "/usr/local/bin/ffmpeg")
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, "/usr/local/bin/ffmpeg", cfg.FFmpeg.Path)
}

func TestLoad_FFmpegProbePathOverride(t *testing.T) {
	setTestSecret(t)
	t.Setenv("AETHERSTREAM_FFMPEG_PROBE_PATH", "/usr/local/bin/ffprobe")
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, "/usr/local/bin/ffprobe", cfg.FFmpeg.ProbePath)
}

func TestLoad_FFmpegMaxJobsOverride(t *testing.T) {
	setTestSecret(t)
	t.Setenv("AETHERSTREAM_FFMPEG_MAX_JOBS", "8")
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, 8, cfg.FFmpeg.MaxJobs)
}

func TestLoad_FFmpegHWAccelOverride(t *testing.T) {
	setTestSecret(t)
	t.Setenv("AETHERSTREAM_FFMPEG_HWACCEL", "nvenc")
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, "nvenc", cfg.FFmpeg.HWAccel)
}

func TestLoad_DefaultPort_WhenNoEnv(t *testing.T) {
	setTestSecret(t)
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	// Default when AETHERSTREAM_SERVER_PORT is not set
	assert.Equal(t, 8081, cfg.Server.Port)
}

func TestLoad_DefaultWebUIPath_WhenNoEnv(t *testing.T) {
	setTestSecret(t)
	cfg, err := Load("/nonexistent/config.yaml")
	require.NoError(t, err)
	assert.Equal(t, "web/dist", cfg.Server.WebUIPath)
}

func TestConfig_Save_WritesFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	cfg := Defaults()
	err := cfg.Save(cfgPath)
	assert.NoError(t, err)

	info, err := os.Stat(cfgPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}

func TestConfig_Save_InvalidPath(t *testing.T) {
	cfg := Defaults()
	err := cfg.Save("/nonexistent/dir/config.yaml")
	assert.Error(t, err)
}

func TestLoad_WithValidYAMLFile(t *testing.T) {
	setTestSecret(t)
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	// Write koanf-compatible YAML (keys match koanf struct tags)
	yamlContent := "ffmpeg:\n  max_jobs: 7\n  hwaccel: vaapi\n"
	require.NoError(t, os.WriteFile(cfgPath, []byte(yamlContent), 0644))

	cfg, err := Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, 7, cfg.FFmpeg.MaxJobs)
	assert.Equal(t, "vaapi", cfg.FFmpeg.HWAccel)
}

func TestDefaults_FFmpegValues(t *testing.T) {
	cfg := Defaults()
	assert.Equal(t, "ffmpeg", cfg.FFmpeg.Path)
	assert.Equal(t, "ffprobe", cfg.FFmpeg.ProbePath)
	assert.Equal(t, 4, cfg.FFmpeg.MaxJobs)
	assert.Equal(t, "auto", cfg.FFmpeg.HWAccel)
}

func TestDefaults_ServerValues(t *testing.T) {
	cfg := Defaults()
	assert.Equal(t, 8081, cfg.Server.Port)
	assert.Equal(t, 9090, cfg.Server.MetricsPort)
	assert.Equal(t, 8082, cfg.Server.DLNAPort)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
}
