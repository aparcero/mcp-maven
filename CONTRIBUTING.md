# Contributing

Thanks for improving `mcp-maven`.

## Development Setup

Install Go 1.26 or newer. `mise` is optional, but the repository task commands use it.

```bash
go test -v ./...
mkdir -p bin
go build -o bin/mcp-maven ./cmd/mcp-maven
```

With `mise`:

```bash
mise install
mise run check
```

For local environment overrides:

```bash
cp .env.example .env
```

Do not commit real `.env` files or credentials.

## Pull Requests

Before opening a pull request, run:

```bash
mise run check
```

If you do not use `mise`, run the equivalent commands:

```bash
go mod tidy
go test -v ./...
go test -count=1 -run Integration -v ./...
go tool golangci-lint run ./...
go tool govulncheck ./...
if git grep -n -I -E 'BEGIN (RSA|DSA|EC|OPENSSH|PRIVATE) KEY|AKIA[0-9A-Z]{16}|ghp_[A-Za-z0-9_]{36}|github_pat_[A-Za-z0-9_]{82,}|xox[baprs]-[A-Za-z0-9-]{10,}|sk-[A-Za-z0-9]{20,}' -- . ':!go.sum'; then
  echo "Potential secrets found in tracked files" >&2
  exit 1
fi
mkdir -p bin
go build -o bin/mcp-maven ./cmd/mcp-maven
```

Live Maven Central integration tests are opt-in and require network access:

```bash
MCP_MAVEN_LIVE_TESTS=1 mise run test-integration
```

## Code Style

- Keep MCP tool handlers small and return business errors as tool results.
- Add focused tests for behavior changes.
- Preserve JSON field names used by existing clients.
- Keep Context7 optional; Maven Central tools must work without it.
- Avoid introducing new external dependencies unless they remove meaningful complexity.

## Documentation

Update `README.md` when a change affects installation, configuration, tool arguments, tool outputs, or operational behavior.
