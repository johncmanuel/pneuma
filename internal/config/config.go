package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

// DatabaseConfig holds SQLite settings.
type DatabaseConfig struct {
	Path string `toml:"path"`
}

// LibraryConfig holds music library settings.
type LibraryConfig struct {
	WatchFolders []string `toml:"watch_folders"`
}

// ArtworkConfig holds artwork cache settings.
type ArtworkConfig struct {
	CacheDir  string `toml:"cache_dir"`
	MaxSizeMB int    `toml:"max_size_mb"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	// SecretKey is used to sign session tokens. Auto-generated on first run.
	SecretKey string `toml:"secret_key"`
}

// TranscodingConfig holds paths for external audio tools.
type TranscodingConfig struct {
	// FFmpegPath is the path to the ffmpeg binary (used for loudness analysis).
	FFmpegPath string `toml:"ffmpeg_path"`
	// FpcalcPath is the path to the Chromaprint fpcalc binary.
	FpcalcPath string `toml:"fpcalc_path"`
}

// UploadConfig holds music upload settings.
type UploadConfig struct {
	// Dir is the directory where user-uploaded files are stored.
	Dir string `toml:"dir"`
	// MaxSizeMB is the maximum upload size in megabytes.
	MaxSizeMB int `toml:"max_size_mb"`
}

// Config is the root application configuration.
type Config struct {
	Server      ServerConfig      `toml:"server"`
	Database    DatabaseConfig    `toml:"database"`
	Library     LibraryConfig     `toml:"library"`
	Artwork     ArtworkConfig     `toml:"artwork"`
	Auth        AuthConfig        `toml:"auth"`
	Upload      UploadConfig      `toml:"upload"`
	Transcoding TranscodingConfig `toml:"transcoding"`
}

// DefaultConfig returns a Config with safe defaults derived from the user's home
// directory. The SecretKey is freshly generated.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".pneuma")
	return &Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8989,
		},
		Database: DatabaseConfig{
			Path: filepath.Join(dataDir, "pneuma.db"),
		},
		Library: LibraryConfig{
			WatchFolders: []string{filepath.Join(home, "Music")},
		},
		Artwork: ArtworkConfig{
			CacheDir:  filepath.Join(dataDir, "artwork"),
			MaxSizeMB: 500,
		},
		Auth: AuthConfig{
			SecretKey: generateKey(),
		},
		Upload: UploadConfig{
			Dir:       filepath.Join(dataDir, "uploads"),
			MaxSizeMB: 500,
		},
		Transcoding: TranscodingConfig{
			FFmpegPath: "ffmpeg",
			FpcalcPath: "fpcalc",
		},
	}
}

// Load reads the TOML config file at path and overlays it onto DefaultConfig.
// If the file does not exist, defaults are returned without error.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save writes cfg as TOML to path, creating parent directories as needed.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// DefaultPath returns the canonical config file location.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pneuma", "config.toml")
}

func generateKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "change-me-please"
	}
	return hex.EncodeToString(b)
}
