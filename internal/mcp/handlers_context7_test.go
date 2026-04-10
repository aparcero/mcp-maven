package mcp

import (
	"strings"
	"testing"
	"time"

	"github.com/aparcero/mcp-maven/internal/config"
	"github.com/aparcero/mcp-maven/internal/domain"
)

func TestHandleResolveLibraryID(t *testing.T) {
	context7API := &testContext7Response{
		searchBody: `{"results":[{"id":"/spring-projects/spring-framework","title":"Spring Framework","description":"Core Spring documentation","totalSnippets":1234,"trustScore":9.1,"benchmarkScore":88.5,"versions":["6.1.4","6.1.3"]}]}`,
	}
	context7Server := newTestContext7API(t, context7API)
	defer context7Server.Close()

	server := newTestServer(t, "https://repo1.maven.org/maven2", func(cfg *config.Config) {
		cfg.Context7.Enabled = true
		cfg.Context7.ServerURL = context7Server.URL
	})

	data := callTool(t, server.handleResolveLibraryID, ResolveLibraryIDArgs{
		LibraryName: "spring-framework",
		Query:       "How to configure BeanFactory",
	})

	results := data["results"].([]any)
	first := results[0].(map[string]any)
	if first["libraryId"] != "/spring-projects/spring-framework" {
		t.Fatalf("unexpected libraryId: %v", first["libraryId"])
	}
	if first["sourceReputation"] != "High" {
		t.Fatalf("unexpected sourceReputation: %v", first["sourceReputation"])
	}
	if got := context7API.lastSearchQuery.Get("libraryName"); got != "spring-framework" {
		t.Fatalf("expected libraryName query spring-framework, got %q", got)
	}
}

func TestHandleQueryDocs(t *testing.T) {
	context7API := &testContext7Response{
		contextBody: `{"codeSnippets":[{"codeTitle":"Example","codeDescription":"desc","codeLanguage":"go","codeTokens":12,"codeId":"id","pageTitle":"page","codeList":[{"language":"go","code":"fmt.Println(\"hi\")"}]}],"infoSnippets":[{"pageId":"page","breadcrumb":"guide","content":"Use the API like this","contentTokens":10}]}`,
	}
	context7Server := newTestContext7API(t, context7API)
	defer context7Server.Close()

	server := newTestServer(t, "https://repo1.maven.org/maven2", func(cfg *config.Config) {
		cfg.Context7.Enabled = true
		cfg.Context7.ServerURL = context7Server.URL
	})

	data := callTool(t, server.handleQueryDocs, QueryDocsArgs{
		LibraryID: "/spring-projects/spring-framework",
		Query:     "BeanFactory",
	})

	if data["libraryId"] != "/spring-projects/spring-framework" {
		t.Fatalf("unexpected libraryId: %v", data["libraryId"])
	}
	codeSnippets := data["codeSnippets"].([]any)
	if len(codeSnippets) != 1 {
		t.Fatalf("expected 1 code snippet, got %d", len(codeSnippets))
	}
	if got := context7API.lastContextQuery.Get("query"); got != "BeanFactory" {
		t.Fatalf("expected query BeanFactory, got %q", got)
	}
}

func TestHandleGetLibraryDocsUsesContext7Client(t *testing.T) {
	context7API := &testContext7Response{
		searchBody:  `{"results":[{"id":"/spring-projects/spring-framework","title":"Spring Framework","description":"Core Spring documentation","totalSnippets":1234,"trustScore":9.1,"benchmarkScore":88.5,"versions":["6.1.4","6.1.3"]}]}`,
		contextBody: `{"codeSnippets":[{"codeTitle":"Example","codeDescription":"desc","codeLanguage":"java","codeTokens":12,"codeId":"id","pageTitle":"page","codeList":[{"language":"java","code":"BeanFactory factory;"}]}],"infoSnippets":[]}`,
	}
	context7Server := newTestContext7API(t, context7API)
	defer context7Server.Close()

	server := newTestServer(t, "https://repo1.maven.org/maven2", func(cfg *config.Config) {
		cfg.Context7.Enabled = true
		cfg.Context7.ServerURL = context7Server.URL
	})

	data := callTool(t, server.handleGetLibraryDocs, GetLibraryDocsArgs{
		GroupID:    "org.springframework",
		ArtifactID: "spring-core",
		Version:    "6.1.4",
		Query:      "BeanFactory",
	})

	if data["status"] != "success" {
		t.Fatalf("expected success status, got %v", data["status"])
	}
	if data["libraryId"] != "/spring-projects/spring-framework/6.1.4" {
		t.Fatalf("unexpected libraryId: %v", data["libraryId"])
	}
	docURLs := data["documentationUrl"].(map[string]any)
	if docURLs["javadoc"] != "https://javadoc.io/doc/org.springframework/spring-core/6.1.4/" {
		t.Fatalf("unexpected javadoc URL: %v", docURLs["javadoc"])
	}
}

func TestCompareDependencyVersionsAddsContext7Guidance(t *testing.T) {
	repo := newTestMavenRepo(t, []testArtifact{
		{
			GroupID:    "org.springframework.boot",
			ArtifactID: "spring-boot-starter-parent",
			Versions:   []string{"4.0.0", "3.5.11", "3.5.9"},
			Timestamps: map[string]time.Time{
				"4.0.0":  time.Date(2025, time.March, 15, 0, 0, 0, 0, time.UTC),
				"3.5.11": time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC),
				"3.5.9":  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	})
	defer repo.Close()

	server := newTestServer(t, repo.URL, func(cfg *config.Config) {
		cfg.Context7.Enabled = true
	})

	data := callTool(t, server.handleCompareDependencyVersions, CompareDependencyVersionsArgs{
		Dependencies: []DependencySpec{{
			GroupID:    "org.springframework.boot",
			ArtifactID: "spring-boot-starter-parent",
			Version:    "3.5.9",
		}},
		StabilityFilter: domain.StabilityStableOnly,
	})

	dependencies := data["dependencies"].([]any)
	first := dependencies[0].(map[string]any)
	guidance := first["context7Guidance"].(map[string]any)
	instructions := guidance["orchestrationInstructions"].(string)
	if !strings.Contains(instructions, "resolve-library-id") || !strings.Contains(instructions, "query-docs") {
		t.Fatalf("unexpected guidance instructions: %s", instructions)
	}
}

func TestAnalyzeDependencyAgeAddsContext7GuidanceForStaleDependencies(t *testing.T) {
	repo := newTestMavenRepo(t, []testArtifact{
		{
			GroupID:    "org.example",
			ArtifactID: "legacy-lib",
			Versions:   []string{"1.0.0"},
			Timestamps: map[string]time.Time{
				"1.0.0": time.Now().AddDate(-2, 0, 0),
			},
		},
	})
	defer repo.Close()

	server := newTestServer(t, repo.URL, func(cfg *config.Config) {
		cfg.Context7.Enabled = true
	})

	data := callTool(t, server.handleAnalyzeDependencyAge, AnalyzeDependencyAgeArgs{
		Dependencies: []DependencySpec{{GroupID: "org.example", ArtifactID: "legacy-lib"}},
	})

	dependencies := data["dependencies"].([]any)
	first := dependencies[0].(map[string]any)
	guidance := first["context7Guidance"].(map[string]any)
	instructions := guidance["orchestrationInstructions"].(string)
	if !strings.Contains(instructions, "modernization") {
		t.Fatalf("unexpected modernization guidance: %s", instructions)
	}
}

func TestAnalyzeProjectHealthAddsContext7GuidanceForStaleDependencies(t *testing.T) {
	repo := newTestMavenRepo(t, []testArtifact{
		{
			GroupID:    "org.example",
			ArtifactID: "legacy-lib",
			Versions:   []string{"1.0.0"},
			Timestamps: map[string]time.Time{
				"1.0.0": time.Now().AddDate(-2, 0, 0),
			},
		},
	})
	defer repo.Close()

	server := newTestServer(t, repo.URL, func(cfg *config.Config) {
		cfg.Context7.Enabled = true
	})

	data := callTool(t, server.handleAnalyzeProjectHealth, AnalyzeProjectHealthArgs{
		Dependencies: []DependencySpec{{GroupID: "org.example", ArtifactID: "legacy-lib"}},
	})

	dependencies := data["dependencies"].([]any)
	first := dependencies[0].(map[string]any)
	guidance := first["context7Guidance"].(map[string]any)
	instructions := guidance["orchestrationInstructions"].(string)
	if !strings.Contains(instructions, "modernization") {
		t.Fatalf("unexpected project health guidance: %s", instructions)
	}
}
