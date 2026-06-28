package context7

import (
	"fmt"
	"testing"
	"time"

	"github.com/aparcero/mcp-maven/internal/config"
)

func TestClassifySourceReputation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		score float64
		want  string
	}{
		{score: 9.5, want: "High"},
		{score: 8, want: "High"},
		{score: 7.9, want: "Medium"},
		{score: 5, want: "Medium"},
		{score: 1, want: "Low"},
		{score: 0.1, want: "Low"},
		{score: 0, want: "Unknown"},
		{score: -1, want: "Unknown"},
	}

	for _, tc := range cases {
		if got := classifySourceReputation(tc.score); got != tc.want {
			t.Fatalf("classifySourceReputation(%v) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestNormalizeVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want string
	}{
		{in: "1.2.3", want: "1.2.3"},
		{in: "v1.2.3", want: "1.2.3"},
		{in: "V1.2.3", want: "1.2.3"},
		{in: "1_2_3", want: "1.2.3"},
		{in: "  1.2.3  ", want: "1.2.3"},
	}

	for _, tc := range cases {
		if got := normalizeVersion(tc.in); got != tc.want {
			t.Fatalf("normalizeVersion(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveVersionedLibraryID(t *testing.T) {
	t.Parallel()

	match := LibraryMatch{
		LibraryID: "/spring-projects/spring-framework",
		Versions:  []string{"6.1.4", "6.1.3"},
	}

	t.Run("empty version returns base library id", func(t *testing.T) {
		t.Parallel()
		if got := resolveVersionedLibraryID(match, ""); got != "/spring-projects/spring-framework" {
			t.Fatalf("got %q, want base library id", got)
		}
	})

	t.Run("matching version appends versioned segment", func(t *testing.T) {
		t.Parallel()
		if got := resolveVersionedLibraryID(match, "6.1.4"); got != "/spring-projects/spring-framework/6.1.4" {
			t.Fatalf("got %q, want versioned library id", got)
		}
	})

	t.Run("normalized version forms match (v-prefix and underscores)", func(t *testing.T) {
		t.Parallel()
		if got := resolveVersionedLibraryID(match, "v6.1.4"); got != "/spring-projects/spring-framework/6.1.4" {
			t.Fatalf("got %q for v-prefixed input", got)
		}
	})

	t.Run("non-matching version falls back to base id", func(t *testing.T) {
		t.Parallel()
		if got := resolveVersionedLibraryID(match, "9.9.9"); got != "/spring-projects/spring-framework" {
			t.Fatalf("got %q, want base id for non-matching version", got)
		}
	})

	t.Run("trailing slash on library id is trimmed", func(t *testing.T) {
		t.Parallel()
		slashy := LibraryMatch{LibraryID: "/spring-projects/spring-framework/", Versions: []string{"6.1.4"}}
		if got := resolveVersionedLibraryID(slashy, "6.1.4"); got != "/spring-projects/spring-framework/6.1.4" {
			t.Fatalf("got %q, want no double slash", got)
		}
	})
}

func TestLibraryIDFromRedirect(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		redirect string
		want     string
	}{
		{name: "absolute path returned as-is", redirect: "/reactjs/react.dev", want: "/reactjs/react.dev"},
		{name: "libraryId query param extracted", redirect: "https://context7.com/api?libraryId=/foo/bar", want: "/foo/bar"},
		{name: "path used when no libraryId param", redirect: "https://context7.com/foo/bar", want: "/foo/bar"},
		{name: "empty for root path with no param", redirect: "https://context7.com/", want: "/"},
		{name: "empty for malformed url", redirect: "://not-a-url", want: ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := libraryIDFromRedirect(tc.redirect); got != tc.want {
				t.Fatalf("libraryIDFromRedirect(%q) = %q, want %q", tc.redirect, got, tc.want)
			}
		})
	}
}

func TestAPIError(t *testing.T) {
	t.Parallel()

	t.Run("message takes precedence over status", func(t *testing.T) {
		t.Parallel()
		err := &APIError{StatusCode: 404, Message: "not found here"}
		if got := err.Error(); got != "not found here" {
			t.Fatalf("got %q, want message", got)
		}
	})

	t.Run("nil returns empty string", func(t *testing.T) {
		t.Parallel()
		var err *APIError
		if got := err.Error(); got != "" {
			t.Fatalf("got %q, want empty for nil error", got)
		}
	})

	t.Run("missing message falls back to status code", func(t *testing.T) {
		t.Parallel()
		err := &APIError{StatusCode: 500}
		if got := err.Error(); got != "Context7 API error (500)" {
			t.Fatalf("got %q, want status fallback", got)
		}
	})
}

func TestAsAPIError(t *testing.T) {
	t.Parallel()

	t.Run("unwraps APIError", func(t *testing.T) {
		t.Parallel()
		original := &APIError{StatusCode: 429, Message: "rate limited", RetryAfter: 30 * time.Second}
		var target *APIError
		if !AsAPIError(original, &target) {
			t.Fatal("expected AsAPIError to return true for *APIError")
		}
		if target != original {
			t.Fatal("expected target to point to original error")
		}
	})

	t.Run("returns false for non-APIError", func(t *testing.T) {
		t.Parallel()
		other := fmt.Errorf("some other error")
		var target *APIError
		if AsAPIError(other, &target) {
			t.Fatal("expected AsAPIError to return false for a plain error")
		}
		if target != nil {
			t.Fatalf("expected target to remain nil, got %v", target)
		}
	})
}

func TestNewClientReturnsNoopWhenDisabled(t *testing.T) {
	t.Parallel()
	provider := NewClient(config.Context7Config{Enabled: false})
	if provider.Enabled() {
		t.Fatal("expected disabled provider to report Enabled()=false")
	}
}

func TestNoopProvider(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	provider := NewNoopProvider()

	if provider.Enabled() {
		t.Fatal("noop provider should report Enabled()=false")
	}
	if _, err := provider.ResolveLibraryID(ctx, ResolveLibraryIDQuery{}); err == nil {
		t.Fatal("expected error from noop ResolveLibraryID")
	}
	if _, err := provider.QueryDocs(ctx, QueryDocsQuery{}); err == nil {
		t.Fatal("expected error from noop QueryDocs")
	}
	if _, err := provider.GetLibraryDocs(ctx, DocsQuery{}); err == nil {
		t.Fatal("expected error from noop GetLibraryDocs")
	}
}
