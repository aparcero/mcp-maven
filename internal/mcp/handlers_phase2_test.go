package mcp

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aparcero/mcp-maven/internal/config"
	"github.com/aparcero/mcp-maven/internal/domain"
	"github.com/aparcero/mcp-maven/internal/mavencentral"
)

func TestHandleCheckMultipleDependencies(t *testing.T) {
	repo := newTestMavenRepo(t, []testArtifact{
		{
			GroupID:    "org.slf4j",
			ArtifactID: "slf4j-api",
			Versions:   []string{"2.0.9", "2.0.8"},
			Timestamps: map[string]time.Time{
				"2.0.9": time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC),
				"2.0.8": time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			GroupID:    "org.apache.commons",
			ArtifactID: "commons-lang3",
			Versions:   []string{"3.17.0", "3.16.0"},
			Timestamps: map[string]time.Time{
				"3.17.0": time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC),
				"3.16.0": time.Date(2024, time.December, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	})
	defer repo.Close()

	server := newTestServer(t, repo.URL)

	t.Run("multiple dependencies", func(t *testing.T) {
		data := callTool(t, server.handleCheckMultipleDependencies, CheckMultipleDependenciesArgs{
			Dependencies: []DependencySpec{
				{GroupID: "org.slf4j", ArtifactID: "slf4j-api", Version: "2.0.9"},
				{GroupID: "org.apache.commons", ArtifactID: "commons-lang3"},
			},
		})

		if data["count"] != float64(2) {
			t.Fatalf("expected count 2, got %v", data["count"])
		}

		deps := data["dependencies"].([]any)
		second := deps[1].(map[string]any)
		if second["latestVersion"] != "3.17.0" {
			t.Fatalf("expected latestVersion 3.17.0, got %v", second["latestVersion"])
		}
	})

	t.Run("empty dependencies", func(t *testing.T) {
		result, _, err := server.handleCheckMultipleDependencies(t.Context(), nil, CheckMultipleDependenciesArgs{})
		if err != nil {
			t.Fatalf("handleCheckMultipleDependencies returned unexpected error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected error for empty dependency list")
		}
	})
}

func TestHandleCheckMultipleDependenciesLimitsConcurrency(t *testing.T) {
	var inFlight int32
	var maxInFlight int32

	repo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/maven-metadata.xml") {
			http.NotFound(w, r)
			return
		}

		current := atomic.AddInt32(&inFlight, 1)
		for {
			max := atomic.LoadInt32(&maxInFlight)
			if current <= max || atomic.CompareAndSwapInt32(&maxInFlight, max, current) {
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
		defer atomic.AddInt32(&inFlight, -1)

		w.Header().Set("Content-Type", "application/xml")
		_ = xml.NewEncoder(w).Encode(mavencentral.MavenMetadata{
			GroupID:  "org.example",
			Artifact: "demo",
			Versioning: mavencentral.Versioning{
				Versions: mavencentral.Versions{Version: []string{"1.0.0"}},
			},
		})
	}))
	defer repo.Close()

	server := newTestServer(t, repo.URL, func(cfg *config.Config) {
		cfg.MaxConcurrentRequests = 2
	})

	args := CheckMultipleDependenciesArgs{Dependencies: []DependencySpec{
		{GroupID: "org.example", ArtifactID: "demo-a"},
		{GroupID: "org.example", ArtifactID: "demo-b"},
		{GroupID: "org.example", ArtifactID: "demo-c"},
		{GroupID: "org.example", ArtifactID: "demo-d"},
		{GroupID: "org.example", ArtifactID: "demo-e"},
	}}
	_ = callTool(t, server.handleCheckMultipleDependencies, args)

	if got := atomic.LoadInt32(&maxInFlight); got > 2 {
		t.Fatalf("expected at most 2 concurrent metadata requests, got %d", got)
	}
}

func TestHandleCheckMultipleDependenciesRejectsOversizedBatch(t *testing.T) {
	server := newTestServer(t, "https://repo1.maven.org/maven2", func(cfg *config.Config) {
		cfg.MaxDependencies = 2
	})

	result, _, err := server.handleCheckMultipleDependencies(t.Context(), nil, CheckMultipleDependenciesArgs{
		Dependencies: []DependencySpec{
			{GroupID: "org.example", ArtifactID: "one"},
			{GroupID: "org.example", ArtifactID: "two"},
			{GroupID: "org.example", ArtifactID: "three"},
		},
	})
	if err != nil {
		t.Fatalf("handleCheckMultipleDependencies returned unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected oversized batch to return an error result")
	}

	data := parseResultJSON(t, result)
	if data["errorType"] != "INVALID_ARGUMENTS" {
		t.Fatalf("expected INVALID_ARGUMENTS, got %v", data["errorType"])
	}
}

func TestBatchHandlersRejectInvalidDependencyCoordinates(t *testing.T) {
	repo := newTestMavenRepo(t, nil)
	defer repo.Close()

	server := newTestServer(t, repo.URL)
	dependencies := []DependencySpec{{ArtifactID: "missing-group"}}

	assertInvalidCoordinates := func(t *testing.T, result map[string]any) {
		t.Helper()

		if result["errorType"] != "INVALID_COORDINATES" {
			t.Fatalf("expected INVALID_COORDINATES, got %v", result["errorType"])
		}
	}

	t.Run("check multiple dependencies", func(t *testing.T) {
		result, _, err := server.handleCheckMultipleDependencies(t.Context(), nil, CheckMultipleDependenciesArgs{
			Dependencies: dependencies,
		})
		if err != nil {
			t.Fatalf("handleCheckMultipleDependencies returned unexpected error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected invalid coordinates to return an error result")
		}
		assertInvalidCoordinates(t, parseResultJSON(t, result))
	})

	t.Run("compare dependency versions", func(t *testing.T) {
		result, _, err := server.handleCompareDependencyVersions(t.Context(), nil, CompareDependencyVersionsArgs{
			Dependencies: dependencies,
		})
		if err != nil {
			t.Fatalf("handleCompareDependencyVersions returned unexpected error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected invalid coordinates to return an error result")
		}
		assertInvalidCoordinates(t, parseResultJSON(t, result))
	})

	t.Run("analyze dependency age", func(t *testing.T) {
		result, _, err := server.handleAnalyzeDependencyAge(t.Context(), nil, AnalyzeDependencyAgeArgs{
			Dependencies: dependencies,
		})
		if err != nil {
			t.Fatalf("handleAnalyzeDependencyAge returned unexpected error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected invalid coordinates to return an error result")
		}
		assertInvalidCoordinates(t, parseResultJSON(t, result))
	})

	t.Run("analyze project health", func(t *testing.T) {
		result, _, err := server.handleAnalyzeProjectHealth(t.Context(), nil, AnalyzeProjectHealthArgs{
			Dependencies: dependencies,
		})
		if err != nil {
			t.Fatalf("handleAnalyzeProjectHealth returned unexpected error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected invalid coordinates to return an error result")
		}
		assertInvalidCoordinates(t, parseResultJSON(t, result))
	})
}

func TestHandleCompareDependencyVersions(t *testing.T) {
	repo := newTestMavenRepo(t, []testArtifact{
		{
			GroupID:    "org.springframework.boot",
			ArtifactID: "spring-boot-starter-parent",
			Versions:   []string{"4.0.0", "3.5.11", "3.5.10", "3.5.9"},
			Timestamps: map[string]time.Time{
				"4.0.0":  time.Date(2025, time.March, 15, 0, 0, 0, 0, time.UTC),
				"3.5.11": time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC),
				"3.5.10": time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC),
				"3.5.9":  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	})
	defer repo.Close()

	server := newTestServer(t, repo.URL)

	t.Run("major stable upgrade includes same-major fallback", func(t *testing.T) {
		data := callTool(t, server.handleCompareDependencyVersions, CompareDependencyVersionsArgs{
			Dependencies: []DependencySpec{
				{
					GroupID:    "org.springframework.boot",
					ArtifactID: "spring-boot-starter-parent",
					Version:    "3.5.9",
				},
			},
			StabilityFilter: domain.StabilityStableOnly,
		})

		deps := data["dependencies"].([]any)
		first := deps[0].(map[string]any)
		if first["latestVersion"] != "4.0.0" {
			t.Fatalf("expected latestVersion 4.0.0, got %v", first["latestVersion"])
		}
		if first["updateType"] != "major" {
			t.Fatalf("expected major update, got %v", first["updateType"])
		}

		fallback, ok := first["sameMajorStableFallback"].(map[string]any)
		if !ok {
			t.Fatalf("expected sameMajorStableFallback, got %v", first["sameMajorStableFallback"])
		}
		if fallback["latestVersion"] != "3.5.11" {
			t.Fatalf("expected fallback latestVersion 3.5.11, got %v", fallback["latestVersion"])
		}
		if fallback["updateType"] != "patch" {
			t.Fatalf("expected fallback patch update, got %v", fallback["updateType"])
		}
	})

	t.Run("missing current version omits update fields", func(t *testing.T) {
		data := callTool(t, server.handleCompareDependencyVersions, CompareDependencyVersionsArgs{
			Dependencies: []DependencySpec{
				{
					GroupID:    "org.springframework.boot",
					ArtifactID: "spring-boot-starter-parent",
				},
			},
		})

		deps := data["dependencies"].([]any)
		first := deps[0].(map[string]any)
		if _, exists := first["updateType"]; exists {
			t.Fatalf("did not expect updateType when no current version is supplied: %v", first["updateType"])
		}
		if first["latestVersion"] != "4.0.0" {
			t.Fatalf("expected latestVersion 4.0.0, got %v", first["latestVersion"])
		}
	})
}
