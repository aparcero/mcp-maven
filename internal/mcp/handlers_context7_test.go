package mcp

import (
	"testing"

	"github.com/aparcero/mcp-maven/internal/config"
)

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
