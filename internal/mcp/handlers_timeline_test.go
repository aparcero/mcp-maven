package mcp

import (
	"testing"
	"time"
)

func TestHandleGetVersionTimelineMarksMajorVersionTransitionsAsBreaking(t *testing.T) {
	repo := newTestMavenRepo(t, []testArtifact{
		{
			GroupID:    "org.example",
			ArtifactID: "demo-lib",
			Versions:   []string{"3.0.0", "2.1.0", "2.0.0"},
			Timestamps: map[string]time.Time{
				"3.0.0": time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC),
				"2.1.0": time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC),
				"2.0.0": time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	})
	defer repo.Close()

	server := newTestServer(t, repo.URL)

	data := callTool(t, server.handleGetVersionTimeline, GetVersionTimelineArgs{
		GroupID:    "org.example",
		ArtifactID: "demo-lib",
		Limit:      3,
	})

	timeline := data["versionTimeline"].([]any)
	first := timeline[0].(map[string]any)
	if first["version"] != "3.0.0" {
		t.Fatalf("expected first version 3.0.0, got %v", first["version"])
	}
	if first["isBreakingChange"] != true {
		t.Fatalf("expected 3.0.0 transition to be marked breaking, got %v", first["isBreakingChange"])
	}
}

func TestAnalyzeDependencyAgeDoesNotClassifyMissingTimestampsAsStale(t *testing.T) {
	repo := newTestMavenRepo(t, []testArtifact{
		{
			GroupID:    "org.example",
			ArtifactID: "no-pom-timestamp",
			Versions:   []string{"1.0.0"},
		},
	})
	defer repo.Close()

	server := newTestServer(t, repo.URL)

	data := callTool(t, server.handleAnalyzeDependencyAge, AnalyzeDependencyAgeArgs{
		Dependencies: []DependencySpec{
			{GroupID: "org.example", ArtifactID: "no-pom-timestamp"},
		},
	})

	if data["successfulAnalysis"] != float64(0) {
		t.Fatalf("expected no successful analyses when timestamps are unavailable, got %v", data["successfulAnalysis"])
	}

	deps := data["dependencies"].([]any)
	first := deps[0].(map[string]any)
	if first["status"] == "success" {
		t.Fatalf("expected missing timestamp not to be classified as a successful stale dependency: %v", first)
	}
	if _, exists := first["ageClassification"]; exists {
		t.Fatalf("did not expect ageClassification when timestamp is unavailable: %v", first["ageClassification"])
	}
}

func TestHandleAnalyzeReleasePatternsSummarizesCadenceAndStrategy(t *testing.T) {
	repo := newTestMavenRepo(t, []testArtifact{
		{
			GroupID:    "org.example",
			ArtifactID: "release-heavy",
			Versions:   []string{"2.0.0-RC3", "2.0.0-RC2", "2.0.0-RC1", "1.9.0", "1.8.0"},
			Timestamps: map[string]time.Time{
				"2.0.0-RC3": time.Date(2025, time.May, 1, 0, 0, 0, 0, time.UTC),
				"2.0.0-RC2": time.Date(2025, time.April, 1, 0, 0, 0, 0, time.UTC),
				"2.0.0-RC1": time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC),
				"1.9.0":     time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC),
				"1.8.0":     time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	})
	defer repo.Close()

	server := newTestServer(t, repo.URL)

	data := callTool(t, server.handleAnalyzeReleasePatterns, AnalyzeReleasePatternsArgs{
		GroupID:    "org.example",
		ArtifactID: "release-heavy",
		Limit:      5,
	})

	if data["dependency"] != "org.example:release-heavy" {
		t.Fatalf("expected dependency org.example:release-heavy, got %v", data["dependency"])
	}
	if data["totalAnalyzed"] != float64(5) {
		t.Fatalf("expected totalAnalyzed 5, got %v", data["totalAnalyzed"])
	}
	if data["releaseStrategy"] != "rc-heavy" {
		t.Fatalf("expected rc-heavy release strategy, got %v", data["releaseStrategy"])
	}

	breakdown := data["versionBreakdown"].(map[string]any)
	if breakdown["stable"] != float64(2) || breakdown["rc"] != float64(3) {
		t.Fatalf("expected 2 stable and 3 rc versions, got %v", breakdown)
	}

	cadence := data["releaseCadence"].(map[string]any)
	if cadence["averageIntervalDays"].(float64) <= 0 {
		t.Fatalf("expected positive average interval, got %v", cadence["averageIntervalDays"])
	}
	if cadence["consistency"] != "regular" {
		t.Fatalf("expected regular cadence, got %v", cadence["consistency"])
	}
}
