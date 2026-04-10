package mavencentral

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aparcero/mcp-maven/internal/cache"
	"github.com/aparcero/mcp-maven/internal/config"
	"github.com/aparcero/mcp-maven/internal/domain"
)

func TestGetAllVersionsUsesConfiguredCacheTTL(t *testing.T) {
	var metadataRequests int32
	repo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/org/example/demo/maven-metadata.xml" {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&metadataRequests, 1)
		w.Header().Set("Content-Type", "application/xml")
		_ = xml.NewEncoder(w).Encode(MavenMetadata{
			GroupID:  "org.example",
			Artifact: "demo",
			Versioning: Versioning{
				Versions: Versions{Version: []string{"1.0.0"}},
			},
		})
	}))
	defer repo.Close()

	client := NewClient(config.MavenCentralConfig{
		RepositoryBaseURL: repo.URL,
		Timeout:           time.Second,
		MaxResults:        100,
	}, cache.New(10), config.CacheConfig{
		AllVersionsTTL:    time.Nanosecond,
		VersionChecksTTL:  time.Hour,
		TimestampCacheTTL: time.Hour,
	})

	coord := &domain.Coordinates{GroupID: "org.example", ArtifactID: "demo", Packaging: "jar"}
	if _, err := client.GetAllVersions(context.Background(), coord); err != nil {
		t.Fatalf("first GetAllVersions failed: %v", err)
	}
	time.Sleep(time.Millisecond)
	if _, err := client.GetAllVersions(context.Background(), coord); err != nil {
		t.Fatalf("second GetAllVersions failed: %v", err)
	}

	if got := atomic.LoadInt32(&metadataRequests); got != 2 {
		t.Fatalf("expected configured TTL to expire cache entry between calls, got %d metadata requests", got)
	}
}
