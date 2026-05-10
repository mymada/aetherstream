package config

import (
	"fmt"
	"os"
	"strconv"

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
	if port := os.Getenv("AETHERSTREAM_SERVER_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Server.Port = p
		}
	} else {
		// Default to 8081 for environments where 8080 is taken
		cfg.Server.Port = 8081
	}
	if host := os.Getenv("AETHERSTREAM_SERVER_HOST"); host != "" {
		cfg.Server.Host = host
	}
	if secret := os.Getenv("AETHERSTREAM_AUTH_SECRET"); secret != "" {
		cfg.Auth.Secret = secret
	} else {
		// Generate a default secret for Docker/demo environments
		cfg.Auth.Secret = os.Getenv("AETHERSTREAM_AUTH_SECRET")
		if cfg.Auth.Secret == "" {
			cfg.Auth.Secret = "aetherstream-default-secret-key-for-docker-environments-only-32chars"
		}
	}
	if ttl := os.Getenv("AETHERSTREAM_AUTH_TOKEN_TTL"); ttl != "" {
		if t, err := strconv.Atoi(ttl); err == nil {
			cfg.Auth.TokenTTL = t
		}
	}
	if dbPath := os.Getenv("AETHERSTREAM_DATABASE_PATH"); dbPath != "" {
		cfg.Database.Path = dbPath
	} else if dataDir := os.Getenv("DATA_DIR"); dataDir != "" {
		cfg.Database.Path = dataDir + "/aetherstream.db"
	}
	if mediaDir := os.Getenv("MEDIA_DIR"); mediaDir != "" {
		cfg.Server.StaticPath = mediaDir
	}
	if ffPath := os.Getenv("AETHERSTREAM_FFMPEG_PATH"); ffPath != "" {
		cfg.FFmpeg.Path = ffPath
	}
	if probePath := os.Getenv("AETHERSTREAM_FFMPEG_PROBE_PATH"); probePath != "" {
		cfg.FFmpeg.ProbePath = probePath
	}
	if maxJobs := os.Getenv("AETHERSTREAM_FFMPEG_MAX_JOBS"); maxJobs != "" {
		if m, err := strconv.Atoi(maxJobs); err == nil {
			cfg.FFmpeg.MaxJobs = m
		}
	}
	if hwaccel := os.Getenv("AETHERSTREAM_FFMPEG_HWACCEL"); hwaccel != "" {
		cfg.FFmpeg.HWAccel = hwaccel
	}

	return cfg, nil
}

// Defaults returns default configuration (exported for tests)
func Defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Port:       8081,
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
