package mavencentral

import (
	"fmt"
	"testing"

	"github.com/aparcero/mcp-maven/internal/cache"
	"github.com/aparcero/mcp-maven/internal/config"
)

// BenchmarkSortVersions measures the cost of sorting a large version set
// (300 entries, comparable to Spring/Hibernate) in descending order.
func BenchmarkSortVersions(b *testing.B) {
	versions := generateVersions(300)
	client := NewClient(config.MavenCentralConfig{}, cache.New(0))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.sortVersions(versions)
	}
}

// generateVersions builds a synthetic, mixed-qualifier version set of the
// given size so the benchmark exercises numeric and qualifier comparison.
func generateVersions(n int) []string {
	versions := make([]string, 0, n)
	for i := 0; i < n; i++ {
		major := i / 100
		minor := (i % 100) / 10
		patch := i % 10
		switch i % 4 {
		case 0:
			versions = append(versions, fmt.Sprintf("%d.%d.%d", major, minor, patch))
		case 1:
			versions = append(versions, fmt.Sprintf("%d.%d.%d-RC%d", major, minor, patch, patch))
		case 2:
			versions = append(versions, fmt.Sprintf("%d.%d.%d-M%d", major, minor, patch, patch))
		default:
			versions = append(versions, fmt.Sprintf("%d.%d.%d-SNAPSHOT", major, minor, patch))
		}
	}
	return versions
}
