package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	// DefaultMaxConcurrentRequests is the default number of concurrent upstream requests.
	DefaultMaxConcurrentRequests = 8
	// DefaultMaxDependencies is the default maximum dependency batch size.
	DefaultMaxDependencies = 100
	// DefaultMaxCacheSize is the default maximum number of cache entries.
	DefaultMaxCacheSize = 10000
)

// Config holds all configuration for the Maven Tools MCP server.
type Config struct {
	// Server configuration
	ServerName    string
	ServerVersion string
	Transport     TransportType

	// Maven Central configuration
	MavenCentral MavenCentralConfig

	// Cache configuration
	Cache CacheConfig

	// Context7 configuration
	Context7 Context7Config

	// Observability configuration
	LogLevel       string
	HTTPPort       int
	MetricsEnabled bool

	MaxConcurrentRequests int
	MaxDependencies       int
}

// TransportType represents the transport mode.
type TransportType string

const (
	TransportSTDIO TransportType = "stdio"
	TransportHTTP  TransportType = "http"
)

// MavenCentralConfig holds Maven Central connection settings.
type MavenCentralConfig struct {
	RepositoryBaseURL string
	Timeout           time.Duration
	MaxResults        int
}

// CacheConfig holds cache settings.
type CacheConfig struct {
	AllVersionsTTL    time.Duration
	VersionChecksTTL  time.Duration
	HistoricalDataTTL time.Duration
	TimestampCacheTTL time.Duration
	MaxCacheSize      int
}

// Context7Config holds Context7 integration settings.
type Context7Config struct {
	Enabled   bool
	APIKey    string
	ServerURL string
	Timeout   time.Duration
}

// Load loads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		ServerName:    getEnv("SERVER_NAME", "io.github.aparcero/mcp-maven"),
		ServerVersion: getEnv("SERVER_VERSION", "1.0.0"),
		Transport:     TransportType(getEnv("TRANSPORT", string(TransportSTDIO))),

		MavenCentral: MavenCentralConfig{
			RepositoryBaseURL: getEnv("MAVEN_CENTRAL_URL", "https://repo1.maven.org/maven2"),
			Timeout:           parseDuration(getEnv("MAVEN_TIMEOUT", "10s"), 10*time.Second),
			MaxResults:        parseIntOrDefault(getEnv("MAVEN_MAX_RESULTS", "100"), 100),
		},

		Cache: CacheConfig{
			AllVersionsTTL:    parseDuration(getEnv("CACHE_ALL_VERSIONS_TTL", "1h"), 1*time.Hour),
			VersionChecksTTL:  parseDuration(getEnv("CACHE_VERSION_CHECKS_TTL", "6h"), 6*time.Hour),
			HistoricalDataTTL: parseDuration(getEnv("CACHE_HISTORICAL_DATA_TTL", "24h"), 24*time.Hour),
			TimestampCacheTTL: parseDuration(getEnv("CACHE_TIMESTAMP_TTL", "24h"), 24*time.Hour),
			MaxCacheSize:      parseIntOrDefault(getEnv("CACHE_MAX_SIZE", "10000"), DefaultMaxCacheSize),
		},

		Context7: Context7Config{
			Enabled:   parseBool(getEnv("CONTEXT7_ENABLED", "false")),
			APIKey:    getEnv("CONTEXT7_API_KEY", ""),
			ServerURL: getEnv("CONTEXT7_SERVER_URL", "https://context7.com"),
			Timeout:   parseDuration(getEnv("CONTEXT7_TIMEOUT", "30s"), 30*time.Second),
		},

		LogLevel:       getEnv("LOG_LEVEL", "info"),
		HTTPPort:       parseIntOrDefault(getEnv("HTTP_PORT", "8080"), 8080),
		MetricsEnabled: parseBool(getEnv("METRICS_ENABLED", "false")),

		MaxConcurrentRequests: parseIntOrDefault(getEnv("MAX_CONCURRENT_REQUESTS", "8"), DefaultMaxConcurrentRequests),
		MaxDependencies:       parseIntOrDefault(getEnv("MAX_DEPENDENCIES", "100"), DefaultMaxDependencies),
	}

	cfg.ApplyDefaults()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// ApplyDefaults fills unset optional configuration values.
func (c *Config) ApplyDefaults() {
	if c.MavenCentral.Timeout == 0 {
		c.MavenCentral.Timeout = 10 * time.Second
	}
	if c.MavenCentral.MaxResults == 0 {
		c.MavenCentral.MaxResults = 100
	}
	if c.Cache.AllVersionsTTL == 0 {
		c.Cache.AllVersionsTTL = time.Hour
	}
	if c.Cache.VersionChecksTTL == 0 {
		c.Cache.VersionChecksTTL = 6 * time.Hour
	}
	if c.Cache.HistoricalDataTTL == 0 {
		c.Cache.HistoricalDataTTL = 24 * time.Hour
	}
	if c.Cache.TimestampCacheTTL == 0 {
		c.Cache.TimestampCacheTTL = 24 * time.Hour
	}
	if c.Cache.MaxCacheSize == 0 {
		c.Cache.MaxCacheSize = DefaultMaxCacheSize
	}
	if c.Context7.Timeout == 0 {
		c.Context7.Timeout = 30 * time.Second
	}
	if c.MaxConcurrentRequests == 0 {
		c.MaxConcurrentRequests = DefaultMaxConcurrentRequests
	}
	if c.MaxDependencies == 0 {
		c.MaxDependencies = DefaultMaxDependencies
	}
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	switch c.Transport {
	case TransportSTDIO, TransportHTTP:
		// Valid
	default:
		return fmt.Errorf("invalid transport type: %s", c.Transport)
	}

	if c.MavenCentral.Timeout < 0 {
		return fmt.Errorf("maven central timeout must be positive")
	}

	if c.HTTPPort < 1 || c.HTTPPort > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", c.HTTPPort)
	}
	if c.MaxConcurrentRequests < 0 {
		return fmt.Errorf("max concurrent requests must be positive")
	}
	if c.MaxDependencies < 0 {
		return fmt.Errorf("max dependencies must be positive")
	}

	return nil
}

// IsSTDIO returns true if the transport is STDIO.
func (c *Config) IsSTDIO() bool {
	return c.Transport == TransportSTDIO
}

// IsHTTP returns true if the transport is HTTP.
func (c *Config) IsHTTP() bool {
	return c.Transport == TransportHTTP
}

// getEnv gets an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseIntOrDefault parses an integer from a string with a default value.
func parseIntOrDefault(value string, defaultValue int) int {
	if value == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return i
}

// parseBool parses a boolean from a string.
func parseBool(value string) bool {
	switch value {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}

// parseDuration parses a duration from a string with a default value.
func parseDuration(value string, defaultDuration time.Duration) time.Duration {
	if value == "" {
		return defaultDuration
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return defaultDuration
	}
	return d
}
