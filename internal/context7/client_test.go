package context7

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/aparcero/mcp-maven/internal/config"
)

// testServerConfig holds per-route response configuration for the fake API.
type testServerConfig struct {
	searchStatus  int
	searchBody    string
	contextStatus int
	contextBody   string

	// observed request state (populated by the handler)
	lastSearchQuery  url.Values
	lastContextQuery url.Values
	searchCalls      int
}

// newTestServer returns an httptest.Server emulating the Context7 API routes
// used by the client, plus a Client pointed at it.
func newTestServer(t *testing.T, srvCfg *testServerConfig) (*httptest.Server, *Client) {
	t.Helper()

	if srvCfg.searchStatus == 0 {
		srvCfg.searchStatus = http.StatusOK
	}
	if srvCfg.contextStatus == 0 {
		srvCfg.contextStatus = http.StatusOK
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/libs/search":
			srvCfg.searchCalls++
			srvCfg.lastSearchQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(srvCfg.searchStatus)
			_, _ = w.Write([]byte(srvCfg.searchBody))
		case "/api/v2/context":
			srvCfg.lastContextQuery = r.URL.Query()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(srvCfg.contextStatus)
			_, _ = w.Write([]byte(srvCfg.contextBody))
		default:
			http.NotFound(w, r)
		}
	}))

	client := NewClient(config.Context7Config{
		Enabled:   true,
		ServerURL: srv.URL,
		Timeout:   2 * time.Second,
	}).(*Client)

	return srv, client
}

func TestResolveLibraryIDParsesSearchResponse(t *testing.T) {
	t.Parallel()

	srvCfg := &testServerConfig{
		searchBody: `{"results":[{"id":"/spring-projects/spring-framework","title":"Spring Framework","description":"Core Spring docs","totalSnippets":1234,"trustScore":9.1,"benchmarkScore":88.5,"versions":["6.1.4","6.1.3"],"state":"active","lastUpdateDate":"2024-01-01"}],"searchFilterApplied":true}`,
	}
	srv, client := newTestServer(t, srvCfg)
	defer srv.Close()

	result, err := client.ResolveLibraryID(context.Background(), ResolveLibraryIDQuery{
		LibraryName: "spring-framework",
		Query:       "BeanFactory",
	})
	if err != nil {
		t.Fatalf("ResolveLibraryID failed: %v", err)
	}

	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	first := result.Results[0]
	if first.LibraryID != "/spring-projects/spring-framework" {
		t.Fatalf("unexpected libraryId: %q", first.LibraryID)
	}
	if first.SourceReputation != "High" {
		t.Fatalf("expected trust 9.1 -> High reputation, got %q", first.SourceReputation)
	}
	if !result.SearchFilterApplied {
		t.Fatal("expected SearchFilterApplied=true")
	}
	if got := srvCfg.lastSearchQuery.Get("libraryName"); got != "spring-framework" {
		t.Fatalf("expected libraryName query param, got %q", got)
	}
}

func TestResolveLibraryIDRequiresArguments(t *testing.T) {
	t.Parallel()

	srvCfg := &testServerConfig{}
	srv, client := newTestServer(t, srvCfg)
	defer srv.Close()

	if _, err := client.ResolveLibraryID(context.Background(), ResolveLibraryIDQuery{Query: "q"}); err == nil {
		t.Fatal("expected error when libraryName is empty")
	}
	if _, err := client.ResolveLibraryID(context.Background(), ResolveLibraryIDQuery{LibraryName: "x"}); err == nil {
		t.Fatal("expected error when query is empty")
	}
	if srvCfg.searchCalls != 0 {
		t.Fatalf("expected no search calls on validation failure, got %d", srvCfg.searchCalls)
	}
}

func TestQueryDocsFollowsRedirect(t *testing.T) {
	t.Parallel()

	// The redirect handler returns 301 with a redirectUrl on the first
	// /context call, then success on the second call (the redirect target).
	var contextCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/context" {
			http.NotFound(w, r)
			return
		}
		contextCalls++
		w.Header().Set("Content-Type", "application/json")
		if contextCalls == 1 {
			w.WriteHeader(http.StatusMovedPermanently)
			_, _ = w.Write([]byte(`{"redirectUrl":"/redirected/library"}`))
			return
		}
		_, _ = w.Write([]byte(`{"codeSnippets":[{"codeTitle":"Example","codeList":[{"language":"java","code":"x"}]}],"infoSnippets":[]}`))
	}))
	defer srv.Close()

	client := NewClient(config.Context7Config{
		Enabled:   true,
		ServerURL: srv.URL,
		Timeout:   2 * time.Second,
	}).(*Client)

	result, err := client.QueryDocs(context.Background(), QueryDocsQuery{
		LibraryID: "/original/library",
		Query:     "BeanFactory",
	})
	if err != nil {
		t.Fatalf("QueryDocs failed: %v", err)
	}
	if contextCalls != 2 {
		t.Fatalf("expected redirect to cause 2 context calls, got %d", contextCalls)
	}
	// The result reflects the redirected library ID (where the docs came from),
	// not the originally requested one.
	if result.LibraryID != "/redirected/library" {
		t.Fatalf("unexpected libraryId in result: %q", result.LibraryID)
	}
}

func TestQueryDocsRequiresArguments(t *testing.T) {
	t.Parallel()

	srv, client := newTestServer(t, &testServerConfig{})
	defer srv.Close()

	if _, err := client.QueryDocs(context.Background(), QueryDocsQuery{Query: "q"}); err == nil {
		t.Fatal("expected error when libraryId is empty")
	}
	if _, err := client.QueryDocs(context.Background(), QueryDocsQuery{LibraryID: "x"}); err == nil {
		t.Fatal("expected error when query is empty")
	}
}

func TestGetLibraryDocsOrchestratesResolveThenQuery(t *testing.T) {
	t.Parallel()

	srvCfg := &testServerConfig{
		searchBody:  `{"results":[{"id":"/spring-projects/spring-framework","title":"Spring Framework","versions":["6.1.4"]}]}`,
		contextBody: `{"codeSnippets":[{"codeTitle":"Example","codeList":[{"language":"java","code":"BeanFactory factory;"}]}],"infoSnippets":[]}`,
	}
	srv, client := newTestServer(t, srvCfg)
	defer srv.Close()

	result, err := client.GetLibraryDocs(context.Background(), DocsQuery{
		GroupID:    "org.springframework",
		ArtifactID: "spring-core",
		Version:    "6.1.4",
		Query:      "BeanFactory",
	})
	if err != nil {
		t.Fatalf("GetLibraryDocs failed: %v", err)
	}
	if result.LibraryID != "/spring-projects/spring-framework/6.1.4" {
		t.Fatalf("unexpected versioned libraryId: %q", result.LibraryID)
	}
	if len(result.Matches) != 1 {
		t.Fatalf("expected matches carried through from resolve, got %d", len(result.Matches))
	}
	if len(result.CodeSnippets) != 1 {
		t.Fatalf("expected 1 code snippet, got %d", len(result.CodeSnippets))
	}
}

func TestGetLibraryDocsDefaultsQueryWhenEmpty(t *testing.T) {
	t.Parallel()

	srvCfg := &testServerConfig{
		searchBody:  `{"results":[{"id":"/foo/bar","title":"Bar","versions":[]}]}`,
		contextBody: `{"codeSnippets":[],"infoSnippets":[]}`,
	}
	srv, client := newTestServer(t, srvCfg)
	defer srv.Close()

	if _, err := client.GetLibraryDocs(context.Background(), DocsQuery{
		ArtifactID: "spring-core",
	}); err != nil {
		t.Fatalf("GetLibraryDocs failed: %v", err)
	}
	if got := srvCfg.lastSearchQuery.Get("query"); got != "spring-core getting started overview" {
		t.Fatalf("expected default query, got %q", got)
	}
}

func TestGetLibraryDocsReturnsNotFoundWhenNoMatch(t *testing.T) {
	t.Parallel()

	srvCfg := &testServerConfig{searchBody: `{"results":[]}`}
	srv, client := newTestServer(t, srvCfg)
	defer srv.Close()

	_, err := client.GetLibraryDocs(context.Background(), DocsQuery{ArtifactID: "ghost"})
	if err == nil {
		t.Fatal("expected error when no library match found")
	}
	var apiErr *APIError
	if !AsAPIError(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected NOT_FOUND APIError, got %v", err)
	}
}

func TestGetLibraryDocsRequiresArtifactID(t *testing.T) {
	t.Parallel()

	srv, client := newTestServer(t, &testServerConfig{})
	defer srv.Close()

	if _, err := client.GetLibraryDocs(context.Background(), DocsQuery{ArtifactID: "  "}); err == nil {
		t.Fatal("expected error when artifactId is empty")
	}
}

func TestGetJSONMapsNon2xxToAPIError(t *testing.T) {
	t.Parallel()

	srvCfg := &testServerConfig{
		searchStatus: http.StatusTooManyRequests,
		searchBody:   `{"error":"rate_limited","message":"slow down"}`,
	}
	srv, client := newTestServer(t, srvCfg)
	defer srv.Close()

	_, err := client.ResolveLibraryID(context.Background(), ResolveLibraryIDQuery{
		LibraryName: "spring",
		Query:       "q",
	})
	if err == nil {
		t.Fatal("expected error for 429")
	}
	var apiErr *APIError
	if !AsAPIError(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", apiErr.StatusCode)
	}
	if apiErr.Code != "rate_limited" {
		t.Fatalf("expected code rate_limited, got %q", apiErr.Code)
	}
}

func TestDecodeAPIErrorParsesRetryAfter(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"slow down"}`))
	}))
	defer srv.Close()

	client := NewClient(config.Context7Config{
		Enabled:   true,
		ServerURL: srv.URL,
		Timeout:   2 * time.Second,
	}).(*Client)

	_, err := client.ResolveLibraryID(context.Background(), ResolveLibraryIDQuery{
		LibraryName: "spring",
		Query:       "q",
	})
	var apiErr *APIError
	if !AsAPIError(err, &apiErr) {
		t.Fatalf("expected *APIError, got %v", err)
	}
	if apiErr.RetryAfter != 120*time.Second {
		t.Fatalf("expected Retry-After 120s, got %v", apiErr.RetryAfter)
	}
}

func TestGetJSONSendsAPIKeyHeaders(t *testing.T) {
	t.Parallel()

	var seenAuth, seenContext7Key string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenContext7Key = r.Header.Get("Context7-API-Key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	client := NewClient(config.Context7Config{
		Enabled:   true,
		ServerURL: srv.URL,
		APIKey:    "secret-token",
		Timeout:   2 * time.Second,
	}).(*Client)

	if _, err := client.ResolveLibraryID(context.Background(), ResolveLibraryIDQuery{
		LibraryName: "spring",
		Query:       "q",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenAuth != "Bearer secret-token" {
		t.Fatalf("expected Bearer auth header, got %q", seenAuth)
	}
	if seenContext7Key != "secret-token" {
		t.Fatalf("expected Context7-API-Key header, got %q", seenContext7Key)
	}
}

func TestNewClientAppliesBaseURLAndTimeout(t *testing.T) {
	t.Parallel()

	t.Run("empty server url falls back to default", func(t *testing.T) {
		t.Parallel()
		client := NewClient(config.Context7Config{Enabled: true, ServerURL: ""}).(*Client)
		if client.baseURL != defaultBaseURL {
			t.Fatalf("expected default base URL, got %q", client.baseURL)
		}
	})

	t.Run("trailing slash trimmed", func(t *testing.T) {
		t.Parallel()
		client := NewClient(config.Context7Config{Enabled: true, ServerURL: "https://example.com/"}).(*Client)
		if client.baseURL != "https://example.com" {
			t.Fatalf("expected trimmed base URL, got %q", client.baseURL)
		}
	})

	t.Run("non-positive timeout falls back to 30s", func(t *testing.T) {
		t.Parallel()
		client := NewClient(config.Context7Config{Enabled: true, Timeout: 0}).(*Client)
		if client.httpClient.Timeout != 30*time.Second {
			t.Fatalf("expected 30s timeout, got %v", client.httpClient.Timeout)
		}
	})
}
