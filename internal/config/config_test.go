package config

import (
	"strings"
	"testing"
	"time"
)

var configEnvKeys = []string{
	"SERVER_NAME",
	"SERVER_VERSION",
	"TRANSPORT",
	"HTTP_PORT",
	"MAVEN_CENTRAL_URL",
	"MAVEN_TIMEOUT",
	"MAVEN_MAX_RESULTS",
	"CACHE_ALL_VERSIONS_TTL",
	"CACHE_VERSION_CHECKS_TTL",
	"CACHE_HISTORICAL_DATA_TTL",
	"CACHE_TIMESTAMP_TTL",
	"CACHE_MAX_SIZE",
	"LOG_LEVEL",
	"METRICS_ENABLED",
	"MAX_CONCURRENT_REQUESTS",
	"MAX_DEPENDENCIES",
	"CONTEXT7_ENABLED",
	"CONTEXT7_API_KEY",
	"CONTEXT7_SERVER_URL",
	"CONTEXT7_TIMEOUT",
}

func TestLoadUsesDocumentedDefaults(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.ServerName != "io.github.aparcero/mcp-maven" {
		t.Fatalf("unexpected ServerName: %s", cfg.ServerName)
	}
	if cfg.Transport != TransportSTDIO {
		t.Fatalf("expected stdio transport, got %s", cfg.Transport)
	}
	if cfg.HTTPPort != 8080 {
		t.Fatalf("expected HTTP port 8080, got %d", cfg.HTTPPort)
	}
	if cfg.MavenCentral.RepositoryBaseURL != "https://repo1.maven.org/maven2" {
		t.Fatalf("unexpected Maven Central URL: %s", cfg.MavenCentral.RepositoryBaseURL)
	}
	if cfg.MavenCentral.Timeout != 10*time.Second {
		t.Fatalf("expected Maven timeout 10s, got %s", cfg.MavenCentral.Timeout)
	}
	if cfg.Context7.Enabled {
		t.Fatal("expected Context7 to be disabled by default")
	}
	if cfg.MaxConcurrentRequests != DefaultMaxConcurrentRequests {
		t.Fatalf("expected default concurrency %d, got %d", DefaultMaxConcurrentRequests, cfg.MaxConcurrentRequests)
	}
	if cfg.MaxDependencies != DefaultMaxDependencies {
		t.Fatalf("expected default max dependencies %d, got %d", DefaultMaxDependencies, cfg.MaxDependencies)
	}
}

func TestLoadReadsEnvironmentOverrides(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("SERVER_NAME", "custom-server")
	t.Setenv("SERVER_VERSION", "2.0.0")
	t.Setenv("TRANSPORT", "http")
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("MAVEN_CENTRAL_URL", "https://repo.example.test/maven2")
	t.Setenv("MAVEN_TIMEOUT", "5s")
	t.Setenv("MAVEN_MAX_RESULTS", "25")
	t.Setenv("CACHE_MAX_SIZE", "123")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("METRICS_ENABLED", "true")
	t.Setenv("MAX_CONCURRENT_REQUESTS", "3")
	t.Setenv("MAX_DEPENDENCIES", "7")
	t.Setenv("CONTEXT7_ENABLED", "true")
	t.Setenv("CONTEXT7_API_KEY", "test-key")
	t.Setenv("CONTEXT7_SERVER_URL", "https://context7.example.test")
	t.Setenv("CONTEXT7_TIMEOUT", "12s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if cfg.ServerName != "custom-server" || cfg.ServerVersion != "2.0.0" {
		t.Fatalf("unexpected server identity: %s %s", cfg.ServerName, cfg.ServerVersion)
	}
	if !cfg.IsHTTP() || cfg.HTTPPort != 9090 {
		t.Fatalf("expected HTTP transport on port 9090, got %s port %d", cfg.Transport, cfg.HTTPPort)
	}
	if cfg.MavenCentral.RepositoryBaseURL != "https://repo.example.test/maven2" {
		t.Fatalf("unexpected Maven Central URL: %s", cfg.MavenCentral.RepositoryBaseURL)
	}
	if cfg.MavenCentral.Timeout != 5*time.Second || cfg.MavenCentral.MaxResults != 25 {
		t.Fatalf("unexpected Maven config: timeout=%s maxResults=%d", cfg.MavenCentral.Timeout, cfg.MavenCentral.MaxResults)
	}
	if cfg.Cache.MaxCacheSize != 123 {
		t.Fatalf("expected cache size 123, got %d", cfg.Cache.MaxCacheSize)
	}
	if cfg.LogLevel != "debug" || !cfg.MetricsEnabled {
		t.Fatalf("unexpected observability config: log=%s metrics=%v", cfg.LogLevel, cfg.MetricsEnabled)
	}
	if cfg.MaxConcurrentRequests != 3 || cfg.MaxDependencies != 7 {
		t.Fatalf("unexpected batch limits: concurrency=%d max=%d", cfg.MaxConcurrentRequests, cfg.MaxDependencies)
	}
	if !cfg.Context7.Enabled || cfg.Context7.APIKey != "test-key" || cfg.Context7.Timeout != 12*time.Second {
		t.Fatalf("unexpected Context7 config: %+v", cfg.Context7)
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr string
	}{
		{name: "transport", key: "TRANSPORT", value: "grpc", wantErr: "invalid transport type"},
		{name: "port", key: "HTTP_PORT", value: "70000", wantErr: "invalid HTTP port"},
		{name: "concurrency", key: "MAX_CONCURRENT_REQUESTS", value: "-1", wantErr: "max concurrent requests"},
		{name: "max dependencies", key: "MAX_DEPENDENCIES", value: "-1", wantErr: "max dependencies"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv(tt.key, tt.value)

			_, err := Load()
			if err == nil {
				t.Fatal("expected Load to return an error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()

	for _, key := range configEnvKeys {
		t.Setenv(key, "")
	}
}
