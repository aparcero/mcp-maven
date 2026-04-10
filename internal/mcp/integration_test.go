package mcp

import (
	"os"
	"testing"

	"github.com/aparcero/mcp-maven/internal/domain"
)

func TestIntegrationLiveMavenCentralGetLatestVersion(t *testing.T) {
	if os.Getenv("MCP_MAVEN_LIVE_TESTS") != "1" {
		t.Skip("set MCP_MAVEN_LIVE_TESTS=1 to run live Maven Central integration tests")
	}

	server := newTestServer(t, "https://repo1.maven.org/maven2")

	data := callTool(t, server.handleGetLatestVersion, GetLatestVersionArgs{
		GroupID:         "org.slf4j",
		ArtifactID:      "slf4j-api",
		StabilityFilter: domain.StabilityStableOnly,
	})

	if data["latestVersion"] == "" {
		t.Fatalf("expected live Maven Central to return a latest version, got %v", data)
	}
	if data["isStable"] != true {
		t.Fatalf("expected stable latest version from STABLE_ONLY, got %v", data["isStable"])
	}
}
