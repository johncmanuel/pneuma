package config

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

// DatabaseConfig holds SQLite settings.
type DatabaseConfig struct {
	Path         string `toml:"path"`
	MaxOpenConns int    `toml:"max_open_conns"`
}

// LibraryConfig holds music library settings.
type LibraryConfig struct {
	WatchFolders        []string `toml:"watch_folders"`
	ScanIntervalMinutes int      `toml:"scan_interval_minutes"`
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
	// FFmpegPath is the path to the ffmpeg binary
	FFmpegPath string `toml:"ffmpeg_path"`
	// FpcalcPath is the path to the Chromaprint fpcalc binary.
	FpcalcPath string `toml:"fpcalc_path"`
	// CacheDir is where cached transcoded streams are written.
	CacheDir string `toml:"cache_dir"`
	// CacheMaxSizeMB is the max on-disk transcode cache size in MB.
	CacheMaxSizeMB int `toml:"cache_max_size_mb"`
	// MaxConcurrentJobs limits concurrent ffmpeg transcode jobs.
	MaxConcurrentJobs int `toml:"max_concurrent_jobs"`
}

// UploadConfig holds music upload settings.
type UploadConfig struct {
	// Dir is the directory where user-uploaded files are stored.
	Dir string `toml:"dir"`
	// MaxSizeMB is the maximum upload size in megabytes.
	MaxSizeMB int `toml:"max_size_mb"`
	// QueueCapacity is the maximum number of queued upload jobs.
	QueueCapacity int `toml:"queue_capacity"`
}

// RateLimitingConfig controls the application-layer rate limiter.
// Self-hosters who use a reverse proxy (e.g. Nginx, Caddy) can disable
// this and rely on the proxy's built-in rate limiting instead.
type RateLimitingConfig struct {
	// Enabled toggles the application-layer rate limiter on/off.
	// Default: true.
	Enabled bool `toml:"enabled"`
}

// Config is the root application configuration.
type Config struct {
	Server       ServerConfig       `toml:"server"`
	Database     DatabaseConfig     `toml:"database"`
	Library      LibraryConfig      `toml:"library"`
	Artwork      ArtworkConfig      `toml:"artwork"`
	Auth         AuthConfig         `toml:"auth"`
	Upload       UploadConfig       `toml:"upload"`
	Transcoding  TranscodingConfig  `toml:"transcoding"`
	RateLimiting RateLimitingConfig `toml:"rate_limiting"`
}

const (
	// Server defaults

	ServerHostDefault = "127.0.0.1" // NOTE: will need to use 0.0.0.0 if using this in docker
	ServerPortDefault = 8989

	// Config file names

	ConfigFileName            = "config.toml"
	ConfigDatabaseName        = "pneuma.db"
	ConfigMusicDirName        = "music"
	ConfigArtworkDirName      = "artwork"
	ConfigCachePlaylistArtDir = "playlist-artwork"
	ConfigCacheTranscodeDir   = "transcode-cache"
	ConfigUploadDirName       = "uploads"
	ConfigDefaultDataDirName  = ".pneuma"

	// Environment variable names

	EnvDataDir               = "PNEUMA_DATA_DIR"
	EnvServerHost            = "PNEUMA_SERVER_HOST"
	EnvServerPort            = "PNEUMA_SERVER_PORT"
	EnvDatabasePath          = "PNEUMA_DATABASE_PATH"
	EnvDatabaseMaxOpenConns  = "PNEUMA_DATABASE_MAX_OPEN_CONNS"
	EnvAuthSecretKey         = "PNEUMA_AUTH_SECRET_KEY"
	EnvLibraryWatchFolders   = "PNEUMA_LIBRARY_WATCH_FOLDERS"
	EnvLibraryScanInterval   = "PNEUMA_LIBRARY_SCAN_INTERVAL_MINUTES"
	EnvArtworkCacheDir       = "PNEUMA_ARTWORK_CACHE_DIR"
	EnvArtworkMaxSizeMB      = "PNEUMA_ARTWORK_MAX_SIZE_MB"
	EnvUploadDir             = "PNEUMA_UPLOAD_DIR"
	EnvUploadMaxSizeMB       = "PNEUMA_UPLOAD_MAX_SIZE_MB"
	EnvUploadQueueCapacity   = "PNEUMA_UPLOAD_QUEUE_CAPACITY"
	EnvTranscodingFFmpegPath = "PNEUMA_TRANSCODING_FFMPEG_PATH"
	EnvTranscodingFpcalcPath = "PNEUMA_TRANSCODING_FPCALC_PATH"
	EnvTranscodingCacheDir   = "PNEUMA_TRANSCODING_CACHE_DIR"
	EnvTranscodingCacheMaxMB = "PNEUMA_TRANSCODING_CACHE_MAX_SIZE_MB"
	EnvTranscodingMaxJobs    = "PNEUMA_TRANSCODING_MAX_CONCURRENT_JOBS"
	EnvRateLimitingEnabled   = "PNEUMA_RATE_LIMITING_ENABLED"

	// Playlist artwork caching

	// 5 MB
	PlaylistMaxArtSizeBytes = 5 << 20
	PlaylistMaxArtDim       = 400
)

// DefaultDataDir returns the canonical base directory for app data.
// It checks PNEUMA_DATA_DIR, then falls back to ~/.pneuma (Unix) or %USERPROFILE%/.pneuma (Windows).
func DefaultDataDir() string {
	if dir := os.Getenv(EnvDataDir); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ConfigDefaultDataDirName)
}

// DefaultConfig returns a Config with safe defaults derived from dataDir.
// The SecretKey is freshly generated.
func DefaultConfig(dataDir string) *Config {
	if dataDir == "" {
		dataDir = DefaultDataDir()
	}

	return &Config{
		Server: ServerConfig{
			Host: ServerHostDefault,
			Port: ServerPortDefault,
		},
		Database: DatabaseConfig{
			Path:         filepath.Join(dataDir, ConfigDatabaseName),
			MaxOpenConns: 1,
		},
		Library: LibraryConfig{
			WatchFolders:        []string{filepath.Join(dataDir, ConfigMusicDirName)},
			ScanIntervalMinutes: 120,
		},
		Artwork: ArtworkConfig{
			CacheDir:  filepath.Join(dataDir, ConfigArtworkDirName),
			MaxSizeMB: 500,
		},
		Auth: AuthConfig{
			SecretKey: generateKey(),
		},
		Upload: UploadConfig{
			Dir:           filepath.Join(dataDir, ConfigUploadDirName),
			MaxSizeMB:     500,
			QueueCapacity: 256,
		},
		// fpcalc will be used for fingerprinting in the future (a stretch goal atm)
		Transcoding: TranscodingConfig{
			FFmpegPath:        "ffmpeg",
			FpcalcPath:        "fpcalc",
			CacheDir:          filepath.Join(dataDir, ConfigCacheTranscodeDir),
			CacheMaxSizeMB:    2048,
			MaxConcurrentJobs: 1,
		},
		RateLimiting: RateLimitingConfig{
			Enabled: true,
		},
	}
}

// applyEnvOverrides parses PNEUMA_* environment variables and overwrites config fields.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv(EnvServerPort); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv(EnvServerHost); v != "" {
		cfg.Server.Host = v
	}
	if v := os.Getenv(EnvDatabasePath); v != "" {
		cfg.Database.Path = v
	}
	if v := os.Getenv(EnvDatabaseMaxOpenConns); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Database.MaxOpenConns = n
		}
	}
	if v := os.Getenv(EnvAuthSecretKey); v != "" {
		cfg.Auth.SecretKey = v
	}
	if v := os.Getenv(EnvLibraryWatchFolders); v != "" {
		cfg.Library.WatchFolders = strings.Split(v, ",")
	}
	if v := os.Getenv(EnvLibraryScanInterval); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Library.ScanIntervalMinutes = p
		}
	}
	if v := os.Getenv(EnvArtworkCacheDir); v != "" {
		cfg.Artwork.CacheDir = v
	}
	if v := os.Getenv(EnvArtworkMaxSizeMB); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Artwork.MaxSizeMB = p
		}
	}
	if v := os.Getenv(EnvUploadDir); v != "" {
		cfg.Upload.Dir = v
	}
	if v := os.Getenv(EnvUploadMaxSizeMB); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Upload.MaxSizeMB = p
		}
	}
	if v := os.Getenv(EnvUploadQueueCapacity); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Upload.QueueCapacity = p
		}
	}
	if v := os.Getenv(EnvTranscodingFFmpegPath); v != "" {
		cfg.Transcoding.FFmpegPath = v
	}
	if v := os.Getenv(EnvTranscodingFpcalcPath); v != "" {
		cfg.Transcoding.FpcalcPath = v
	}
	if v := os.Getenv(EnvTranscodingCacheDir); v != "" {
		cfg.Transcoding.CacheDir = v
	}
	if v := os.Getenv(EnvTranscodingCacheMaxMB); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Transcoding.CacheMaxSizeMB = p
		}
	}
	if v := os.Getenv(EnvTranscodingMaxJobs); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Transcoding.MaxConcurrentJobs = p
		}
	}
	if v := os.Getenv(EnvRateLimitingEnabled); v != "" {
		// Accept "false", "0", "no" as falsy; anything else (including "true", "1") keeps it enabled.
		v = strings.ToLower(strings.TrimSpace(v))
		cfg.RateLimiting.Enabled = v != "false" && v != "0" && v != "no"
	}
}

// Load reads the TOML config file at path and overlays it onto DefaultConfig.
// The resulting config is always written back to disk so that any
// auto-generated values (especially the JWT secret) are persisted on the
// first run and on upgrades where new fields are added.
func Load(path string, dataDir string) (*Config, error) {
	cfg := DefaultConfig(dataDir)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if err == nil {
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	applyEnvOverrides(cfg)

	// Always save back so auto-generated fields are persisted
	// even when the file pre-existed without them.
	if err := Save(path, cfg); err != nil {
		slog.Warn("could not persist config to disk (read-only filesystem?)", "path", path, "err", err)
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

func generateKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		slog.Error("failed to create secret key")
		return "change-me-please"
	}
	return hex.EncodeToString(b)
}
