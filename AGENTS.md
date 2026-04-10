# AGENTS.md

## Build Commands

Build tool: **mise** (tasks defined in `mise.toml`)

```bash
mise run build          # mkdir -p bin && go build -o bin/mcp-maven ./cmd/mcp-maven
mise run build-all      # builds bin/mcp-maven + bin/mcp-maven-noc7 (with -tags noc7)
mise run run            # go run ./cmd/mcp-maven (STDIO mode)
mise run tidy           # go mod tidy
mise run check          # tidy + tests + integration tests + lint + vulncheck + secret scan + build
```

## Test Commands

```bash
mise run test                        # go test -v ./...
mise run test-integration            # go test -count=1 -run Integration -v ./...
mise run test-integration-live       # live Maven Central integration tests
mise run test-race                   # go test -race ./...
go test -v ./...                     # all tests
go test -v ./internal/domain/...     # single package
go test -v -run TestHandlePing ./internal/mcp/  # single test
go test -short -v ./...              # unit tests only
```

## Lint

```bash
mise run lint           # golangci-lint run ./...
mise run vulncheck      # govulncheck ./...
mise run secret-scan    # high-confidence tracked-file secret scan
```

Run `mise run check` after any code change.

## Project Structure

```
cmd/mcp-maven/main.go          # Entry point (flags, signal handling, bootstrap)
internal/
  cache/                        # TTL in-memory cache (sync.RWMutex)
  config/config.go              # Env-var config with defaults, validation
  context7/                     # Context7 HTTP client, DocsProvider interface, noop fallback
  domain/                       # Core types: Coordinates, Stability, Freshness, VersionComparator
  mavencentral/                 # Maven Central HTTP client, metadata XML parsing
  mcp/                          # MCP server, tool handlers, HTTP transport
  observability/                # slog wrapper, HealthChecker
```

## Code Style

### Imports

Three groups separated by blank lines: stdlib, project packages, external dependencies.

```go
import (
    "context"
    "fmt"

    "github.com/aparcero/mcp-maven/internal/config"
    "github.com/aparcero/mcp-maven/internal/domain"

    "github.com/modelcontextprotocol/go-sdk/mcp"
)
```

Use aliases to disambiguate: `ctx7 "github.com/aparcero/mcp-maven/internal/context7"`.

### Naming

- Go exports: PascalCase. Variables: camelCase.
- JSON tags: camelCase (`json:"groupId"`, `json:"latestVersion"`).
- JSONSchema tags inline: `jsonschema:"required,Description text"`.
- Acronyms stay capitalized: `GroupID`, `ArtifactID`, `APIKey`.
- String enum constants: `StabilityStable`, `UpdateMajor`, `FreshnessStale`.
- Handler methods: `handle<X>` on `*Server` (e.g., `handleGetLatestVersion`).
- Args structs: `<ToolName>Args` (e.g., `GetLatestVersionArgs`).

### Types

- String-based enums dominate: `type Stability string`, `type Freshness string`, `type UpdateType string`.
- Each enum has a `String()` method.
- Factory functions for domain types: `NewVersionComparator()`, `ParseCoordinates()`.
- `context7.DocsProvider` is an interface; `mavencentral.Client` is a concrete struct.

### Error Handling

- Wrap errors with `fmt.Errorf("context: %w", err)`.
- Custom error type only in `context7`: `APIError` struct with `StatusCode`, `Code`, `Message`.
- Handlers return business errors via `errorResult("ERROR_CODE", "message"), nil, nil` — never as Go errors. The third return is only for framework-level failures.

Error codes used: `INVALID_COORDINATES`, `INVALID_ARGUMENTS`, `NOT_FOUND`, `UPSTREAM_UNAVAILABLE`, `INTERNAL_ERROR`, `SERVICE_UNAVAILABLE`, `RATE_LIMITED`.

### Comments

GoDoc comments on all exported declarations. No inline comments within function bodies. Comment style: `// FunctionName does X.`

### Handler Pattern

Every handler follows this signature:

```go
func (s *Server) handleX(ctx context.Context, req *mcp.CallToolRequest, args XArgs) (*mcp.CallToolResult, any, error)
```

Return `successResult(data), nil, nil` or `errorResult(code, msg), nil, nil`.

### Concurrency

Use `sync.WaitGroup` + index-captured goroutines for parallel dependency checks. Protect shared slices with `sync.Mutex`. Always propagate `ctx` to downstream calls.

## Testing Conventions

### Test Style

- Tests are in the same package (`package mcp`, not `package mcp_test`).
- Stdlib only — no assertion libraries. Use `t.Fatalf` for fail-fast assertions.
- Call `t.Helper()` in all test helpers.

### Test Helpers (test_helpers_test.go)

- `newTestMavenRepo(t, artifacts)` — `httptest.Server` serving fake Maven metadata/POMs.
- `newTestContext7API(t, response)` — `httptest.Server` for Context7 endpoints.
- `newTestServer(t, repoURL, ...configure)` — creates `Server` with optional config mutation.
- `callTool[Args](t, handler, args)` — generic helper that invokes a handler and returns parsed JSON.

### Writing a New Test

```go
func TestHandleNewTool(t *testing.T) {
    repo := newTestMavenRepo(t, []testArtifact{{...}})
    defer repo.Close()

    server := newTestServer(t, repo.URL)

    data := callTool(t, server.handleNewTool, NewToolArgs{...})
    if data["field"] != expected {
        t.Fatalf("expected X, got %v", data["field"])
    }
}
```

### Adding a New MCP Tool

1. Define an `<ToolName>Args` struct in the appropriate `handlers_*.go` file with `json` and `jsonschema` tags.
2. Add a `handle<ToolName>` method on `*Server` with the standard handler signature.
3. Register it in `registerTools()` in `server.go` via `mcp.AddTool`.
4. Add tests using `callTool` and `newTestMavenRepo`.
