package mcp

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aparcero/mcp-maven/internal/config"
	"github.com/aparcero/mcp-maven/internal/mavencentral"
	mcplib "github.com/modelcontextprotocol/go-sdk/mcp"
)

type testArtifact struct {
	GroupID    string
	ArtifactID string
	Versions   []string
	Timestamps map[string]time.Time
	POMBody    string
}

type testContext7Response struct {
	searchBody       string
	searchStatus     int
	contextBody      string
	contextStatus    int
	lastSearchQuery  url.Values
	lastContextQuery url.Values
}

func newTestMavenRepo(t *testing.T, artifacts []testArtifact) *httptest.Server {
	t.Helper()

	index := make(map[string]testArtifact, len(artifacts))
	for _, artifact := range artifacts {
		key := artifact.GroupID + ":" + artifact.ArtifactID
		index[key] = artifact
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if len(parts) < 3 {
			http.NotFound(w, r)
			return
		}

		artifactIdx := len(parts) - 2
		if strings.HasSuffix(r.URL.Path, ".pom") {
			artifactIdx = len(parts) - 3
		}
		artifactID := parts[artifactIdx]
		groupID := strings.Join(parts[:artifactIdx], ".")
		artifact, ok := index[groupID+":"+artifactID]
		if !ok {
			http.NotFound(w, r)
			return
		}

		switch {
		case strings.HasSuffix(r.URL.Path, "/maven-metadata.xml"):
			w.Header().Set("Content-Type", "application/xml")
			_ = xml.NewEncoder(w).Encode(mavencentral.MavenMetadata{
				GroupID:  artifact.GroupID,
				Artifact: artifact.ArtifactID,
				Versioning: mavencentral.Versioning{
					Latest:   artifact.Versions[0],
					Release:  artifact.Versions[0],
					Versions: mavencentral.Versions{Version: artifact.Versions},
				},
			})
		case strings.HasSuffix(r.URL.Path, ".pom"):
			version := parts[len(parts)-1]
			version = strings.TrimSuffix(version, ".pom")
			version = strings.TrimPrefix(version, artifactID+"-")

			ts, ok := artifact.Timestamps[version]
			if !ok {
				http.NotFound(w, r)
				return
			}

			w.Header().Set("Last-Modified", ts.UTC().Format(http.TimeFormat))
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/xml")
			if artifact.POMBody != "" {
				_, _ = w.Write([]byte(artifact.POMBody))
				return
			}
			_, _ = w.Write([]byte(`<project><licenses><license><name>Apache-2.0</name><url>https://www.apache.org/licenses/LICENSE-2.0.txt</url></license></licenses></project>`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func newTestContext7API(t *testing.T, response *testContext7Response) *httptest.Server {
	t.Helper()

	if response.searchStatus == 0 {
		response.searchStatus = http.StatusOK
	}
	if response.contextStatus == 0 {
		response.contextStatus = http.StatusOK
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/libs/search":
			response.lastSearchQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(response.searchStatus)
			_, _ = w.Write([]byte(response.searchBody))
		case "/api/v2/context":
			response.lastContextQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(response.contextStatus)
			_, _ = w.Write([]byte(response.contextBody))
		default:
			http.NotFound(w, r)
		}
	}))
}

func newTestConfig(repoURL string) *config.Config {
	return &config.Config{
		ServerName:    "test-server",
		ServerVersion: "1.0.0",
		Transport:     config.TransportSTDIO,
		LogLevel:      "error",
		MavenCentral: config.MavenCentralConfig{
			RepositoryBaseURL: repoURL,
			Timeout:           10 * time.Second,
			MaxResults:        100,
		},
	}
}

func newTestServer(t *testing.T, repoURL string, configure ...func(*config.Config)) *Server {
	t.Helper()

	cfg := newTestConfig(repoURL)
	for _, fn := range configure {
		fn(cfg)
	}

	server, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}
	return server
}

func parseResultJSON(t *testing.T, result *mcplib.CallToolResult) map[string]any {
	t.Helper()

	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Content) == 0 {
		t.Fatal("result.Content is empty")
	}

	textContent, ok := result.Content[0].(*mcplib.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &data); err != nil {
		t.Fatalf("failed to parse result JSON: %v; raw=%q", err, textContent.Text)
	}

	return data
}

func callTool[Args any](t *testing.T, handler func(context.Context, *mcplib.CallToolRequest, Args) (*mcplib.CallToolResult, any, error), args Args) map[string]any {
	t.Helper()

	result, _, err := handler(context.Background(), &mcplib.CallToolRequest{}, args)
	if err != nil {
		t.Fatalf("tool handler returned error: %v", err)
	}
	return parseResultJSON(t, result)
}
