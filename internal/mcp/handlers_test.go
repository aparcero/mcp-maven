package mcp

import (
	"testing"
	"time"

	"github.com/aparcero/mcp-maven/internal/domain"
)

func TestHandleGetLatestVersion(t *testing.T) {
	repo := newTestMavenRepo(t, []testArtifact{
		{
			GroupID:    "org.slf4j",
			ArtifactID: "slf4j-api",
			Versions:   []string{"2.1.0-RC1", "2.0.9", "2.0.8"},
			Timestamps: map[string]time.Time{
				"2.1.0-RC1": time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC),
				"2.0.9":     time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC),
				"2.0.8":     time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	})
	defer repo.Close()

	server := newTestServer(t, repo.URL)

	t.Run("stable filter ignores rc releases", func(t *testing.T) {
		data := callTool(t, server.handleGetLatestVersion, GetLatestVersionArgs{
			GroupID:         "org.slf4j",
			ArtifactID:      "slf4j-api",
			StabilityFilter: domain.StabilityStableOnly,
		})

		if data["groupId"] != "org.slf4j" {
			t.Fatalf("expected groupId org.slf4j, got %v", data["groupId"])
		}
		if data["latestVersion"] != "2.0.9" {
			t.Fatalf("expected stable latestVersion 2.0.9, got %v", data["latestVersion"])
		}
		if data["stability"] != "stable" {
			t.Fatalf("expected stability stable, got %v", data["stability"])
		}
	})

	t.Run("invalid coordinates", func(t *testing.T) {
		result, _, err := server.handleGetLatestVersion(t.Context(), nil, GetLatestVersionArgs{
			GroupID:    "",
			ArtifactID: "slf4j-api",
		})
		if err != nil {
			t.Fatalf("handleGetLatestVersion returned unexpected error: %v", err)
		}
		if !result.IsError {
			t.Fatal("expected error result for invalid coordinates")
		}
	})
}

func TestHandleGetLatestVersionReturnsNotFoundForMissingArtifact(t *testing.T) {
	repo := newTestMavenRepo(t, nil)
	defer repo.Close()

	server := newTestServer(t, repo.URL)

	result, _, err := server.handleGetLatestVersion(t.Context(), nil, GetLatestVersionArgs{
		GroupID:    "org.missing",
		ArtifactID: "missing-artifact",
	})
	if err != nil {
		t.Fatalf("handleGetLatestVersion returned unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing artifact")
	}

	data := parseResultJSON(t, result)
	if data["errorType"] != "NOT_FOUND" {
		t.Fatalf("expected NOT_FOUND errorType, got %v", data["errorType"])
	}
}

func TestHandleCheckVersionExists(t *testing.T) {
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
	})
	defer repo.Close()

	server := newTestServer(t, repo.URL)

	t.Run("existing version", func(t *testing.T) {
		data := callTool(t, server.handleCheckVersionExists, CheckVersionExistsArgs{
			GroupID:    "org.slf4j",
			ArtifactID: "slf4j-api",
			Version:    "2.0.9",
		})

		if data["exists"] != true {
			t.Fatalf("expected exists=true, got %v", data["exists"])
		}
		if data["type"] != "stable" {
			t.Fatalf("expected type stable, got %v", data["type"])
		}
	})

	t.Run("non existing version", func(t *testing.T) {
		data := callTool(t, server.handleCheckVersionExists, CheckVersionExistsArgs{
			GroupID:    "org.slf4j",
			ArtifactID: "slf4j-api",
			Version:    "99.99.99",
		})

		if data["exists"] != false {
			t.Fatalf("expected exists=false, got %v", data["exists"])
		}
	})
}

func TestHandlePing(t *testing.T) {
	server := newTestServer(t, "https://repo1.maven.org/maven2")

	data := callTool(t, server.handlePing, PingArgs{Message: "hello"})

	if data["message"] != "pong" {
		t.Fatalf("expected message pong, got %v", data["message"])
	}
	if data["echo"] != "hello" {
		t.Fatalf("expected echo hello, got %v", data["echo"])
	}
	if data["server"] != "test-server" {
		t.Fatalf("expected server test-server, got %v", data["server"])
	}
}
