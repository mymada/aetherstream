package config

import (
	"fmt"
	"os"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig   `koanf:"server"`
	Database DatabaseConfig `koanf:"database"`
	Auth     AuthConfig     `koanf:"auth"`
	SwiftFlow SwiftFlowConfig `koanf:"swiftflow"`
	FFmpeg   FFmpegConfig   `koanf:"ffmpeg"`
}

// ServerConfig HTTP server settings
type ServerConfig struct {
	Port        int    `koanf:"port"`
	Host        string `koanf:"host"`
	StaticPath  string `koanf:"static_path"`
}

// DatabaseConfig SQLite settings
type DatabaseConfig struct {
	Path string `koanf:"path"`
}

// AuthConfig JWT settings
type AuthConfig struct {
	Secret     string `koanf:"secret"`
	TokenTTL   int    `koanf:"token_ttl_hours"`
}

// SwiftFlowConfig integration settings
type SwiftFlowConfig struct {
	BaseURL     string `koanf:"base_url"`
	APIKey      string `koanf:"api_key"`
	WebhookSecret string `koanf:"webhook_secret"`
}

// FFmpegConfig external binary settings
type FFmpegConfig struct {
	Path       string `koanf:"path"`
	ProbePath  string `koanf:"probe_path"`
	MaxJobs    int    `koanf:"max_jobs"`
	HWAccel    string `koanf:"hwaccel"` // auto, none, vaapi, nvenc, qsv, amf
}

// Load reads config from YAML file, then env overrides
func Load(path string) (*Config, error) {
	k := koanf.New(".")
	
	// 1. Start with defaults
	cfg := Defaults()

	// 2. Override from YAML if exists
	if err := k.Load(file.Provider(path), yaml.Parser()); err == nil {
		if err := k.Unmarshal("", cfg); err != nil {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
	}

	// 3. Env overrides (AETHERSTREAM_SERVER_PORT, AETHERSTREAM_AUTH_SECRET, etc.)
	// koanf env provider would go here — keeping simple for now

	return cfg, nil
}

// Defaults returns default configuration (exported for tests)
func Defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Port:       8096,
			Host:       "0.0.0.0",
			StaticPath: "./web/static",
		},
		Database: DatabaseConfig{
			Path: "./data/aetherstream.db",
		},
		Auth: AuthConfig{
			Secret:     os.Getenv("AETHERSTREAM_AUTH_SECRET"),
			TokenTTL:   24,
		},
		SwiftFlow: SwiftFlowConfig{
			BaseURL: os.Getenv("AETHERSTREAM_SWIFTFLOW_URL"),
			APIKey:  os.Getenv("AETHERSTREAM_SWIFTFLOW_KEY"),
			WebhookSecret: os.Getenv("AETHERSTREAM_SWIFTFLOW_WEBHOOK_SECRET"),
		},
		FFmpeg: FFmpegConfig{
			Path:      "ffmpeg",
			ProbePath: "ffprobe",
			MaxJobs:   4,
			HWAccel:   "auto",
		},
	}
}
