package mcp

import (
	"testing"
	"time"

	"github.com/aparcero/mcp-maven/internal/domain"
)

func TestAnalyzeProjectHealthUsesStabilityFilterForLatestVersion(t *testing.T) {
	now := time.Now()
	repo := newTestMavenRepo(t, []testArtifact{
		{
			GroupID:    "org.example",
			ArtifactID: "demo-lib",
			Versions:   []string{"2.0.0-RC1", "1.9.0", "1.8.0"},
			Timestamps: map[string]time.Time{
				"2.0.0-RC1": now.AddDate(0, 0, -1),
				"1.9.0":     now.AddDate(0, 0, -7),
				"1.8.0":     now.AddDate(0, -1, 0),
			},
		},
	})
	defer repo.Close()

	server := newTestServer(t, repo.URL)

	data := callTool(t, server.handleAnalyzeProjectHealth, AnalyzeProjectHealthArgs{
		Dependencies: []DependencySpec{{
			GroupID:    "org.example",
			ArtifactID: "demo-lib",
			Version:    "1.8.0",
		}},
		StabilityFilter: domain.StabilityStableOnly,
	})

	dependencies := data["dependencies"].([]any)
	first := dependencies[0].(map[string]any)
	if first["latestVersion"] != "1.9.0" {
		t.Fatalf("expected latestVersion 1.9.0, got %v", first["latestVersion"])
	}
	if first["isStable"] != true {
		t.Fatalf("expected stable latest version, got %v", first["isStable"])
	}
}

func TestAnalyzeProjectHealthIncludesLicenseData(t *testing.T) {
	repo := newTestMavenRepo(t, []testArtifact{
		{
			GroupID:    "org.example",
			ArtifactID: "licensed-lib",
			Versions:   []string{"1.0.0"},
			Timestamps: map[string]time.Time{
				"1.0.0": time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			},
			POMBody: `<project><licenses><license><name>Apache License, Version 2.0</name><url>https://www.apache.org/licenses/LICENSE-2.0.txt</url></license></licenses></project>`,
		},
	})
	defer repo.Close()

	server := newTestServer(t, repo.URL)

	data := callTool(t, server.handleAnalyzeProjectHealth, AnalyzeProjectHealthArgs{
		Dependencies: []DependencySpec{{
			GroupID:    "org.example",
			ArtifactID: "licensed-lib",
			Version:    "1.0.0",
		}},
		IncludeLicenses: true,
	})

	dependencies := data["dependencies"].([]any)
	first := dependencies[0].(map[string]any)
	licenses := first["licenses"].([]any)
	license := licenses[0].(map[string]any)
	if license["name"] != "Apache License, Version 2.0" {
		t.Fatalf("expected Apache license name, got %v", license["name"])
	}
	if license["url"] != "https://www.apache.org/licenses/LICENSE-2.0.txt" {
		t.Fatalf("expected Apache license URL, got %v", license["url"])
	}
}
