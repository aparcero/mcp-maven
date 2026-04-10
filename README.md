# mcp-maven

An MCP server that gives AI agents read-only JVM dependency intelligence from Maven Central.

It answers questions such as:

- What is the latest stable version of this Maven artifact?
- Does this exact version exist?
- Which dependencies in this list have major, minor, or patch updates?
- How old are these dependencies, and which ones look stale?
- What does the release history of an artifact look like?
- Optionally, what Context7 documentation is available for a library?

The server supports STDIO for desktop and agent clients, plus Streamable HTTP for local services and hosted agent runtimes.

## Status

This project is ready to build from source. Release binaries and package-manager distribution can be added later without changing the MCP protocol surface.

## Features

- Latest version lookup with stability filtering
- Exact version existence checks
- Batch dependency checks with concurrency limits
- Current-vs-latest comparison with update classification
- Version timeline and release cadence analysis
- Dependency freshness and project health analysis
- Optional license lookup from artifact POM files
- Optional Context7 documentation lookup
- STDIO and Streamable HTTP transports
- Health endpoints for HTTP deployments
- In-memory TTL cache for Maven Central responses

## Requirements

- Go 1.26 or newer
- Network access to Maven Central
- Optional: `mise` for the task commands in `mise.toml`
- Optional: Context7 access if documentation tools are enabled

## Quick Start

Clone the repository and build the server:

```bash
git clone https://github.com/aparcero/mcp-maven.git
cd mcp-maven
mkdir -p bin
go build -o bin/mcp-maven ./cmd/mcp-maven
```

Check the binary:

```bash
./bin/mcp-maven -version
./bin/mcp-maven -help
```

With `mise`:

```bash
mise install
mise run build
```

## Configure an MCP Client

Most MCP clients that support STDIO servers accept a JSON block similar to this. Use an absolute path because agents often launch servers from a different working directory.

```json
{
  "mcpServers": {
    "maven": {
      "command": "/absolute/path/to/mcp-maven/bin/mcp-maven",
      "args": [],
      "env": {
        "LOG_LEVEL": "warn",
        "MAVEN_TIMEOUT": "10s"
      }
    }
  }
}
```

For development without building a binary:

```json
{
  "mcpServers": {
    "maven": {
      "command": "go",
      "args": ["run", "/absolute/path/to/mcp-maven/cmd/mcp-maven"],
      "env": {
        "LOG_LEVEL": "warn"
      }
    }
  }
}
```

Client notes:

- Claude Desktop, Cline, Continue, VS Code MCP extensions, IDE agents, and CLI agents usually use a variant of the `mcpServers` block above.
- If a client has separate fields for command, arguments, and environment, copy the same values into those fields.
- STDIO mode is intended to be launched by the MCP client. Running it directly in a terminal will wait for MCP protocol messages on stdin.
- Prefer `LOG_LEVEL=warn` or `LOG_LEVEL=error` in STDIO mode so logs do not clutter agent output.

## HTTP Mode

Run the Streamable HTTP transport locally:

```bash
./bin/mcp-maven -http -port 8080
```

Equivalent environment configuration:

```bash
TRANSPORT=http HTTP_PORT=8080 ./bin/mcp-maven
```

HTTP endpoints:

| Endpoint | Purpose |
| --- | --- |
| `POST /mcp` | Streamable HTTP MCP endpoint |
| `GET /health/live` | Liveness probe |
| `GET /health/ready` | Readiness probe, including Maven Central reachability |
| `GET /` | Basic server metadata |

HTTP mode does not add authentication. Bind it only to trusted local or private networks unless you place it behind your own access controls.

## Tools

| Tool | Availability | Purpose |
| --- | --- | --- |
| `get_latest_version` | Always | Find the latest version for a Maven artifact. |
| `check_version_exists` | Always | Check whether an exact artifact version exists. |
| `check_multiple_dependencies` | Always | Check many dependencies in one call. |
| `compare_dependency_versions` | Always | Compare current versions with latest available versions. |
| `get_version_timeline` | Always | Return recent versions with release dates and cadence details. |
| `analyze_dependency_age` | Always | Classify dependency freshness across a list. |
| `analyze_release_patterns` | Always | Summarize release cadence and prerelease strategy. |
| `analyze_project_health` | Always | Produce aggregate health, update, age, and optional license data. |
| `get_library_docs` | Registered always, requires Context7 enabled to succeed | Resolve and fetch documentation for a Maven library. |
| `resolve-library-id` | Context7 enabled only | Search Context7 for documentation library IDs. |
| `query-docs` | Context7 enabled only | Fetch Context7 documentation snippets by library ID. |
| `ping` | Always | Connectivity diagnostic. |

### Common Arguments

Maven dependency objects use this shape:

```json
{
  "groupId": "org.slf4j",
  "artifactId": "slf4j-api",
  "version": "2.0.9"
}
```

`version` is optional for tools that only need artifact-level metadata.

`stabilityFilter` accepts:

| Value | Behavior |
| --- | --- |
| `PREFER_STABLE` | Default. Prefer stable releases, but return prerelease versions if no stable version exists. |
| `STABLE_ONLY` | Return only stable releases. |
| `ALL` | Include stable, RC, beta, alpha, milestone, snapshot, and other prerelease versions. |

Freshness classifications:

| Value | Age |
| --- | --- |
| `fresh` | Less than 30 days old |
| `current` | 30 to 90 days old |
| `aging` | 90 to 365 days old |
| `stale` | More than 365 days old |

### Example Tool Calls

Get the latest stable version:

```json
{
  "name": "get_latest_version",
  "arguments": {
    "groupId": "org.slf4j",
    "artifactId": "slf4j-api",
    "stabilityFilter": "STABLE_ONLY"
  }
}
```

Check an exact version:

```json
{
  "name": "check_version_exists",
  "arguments": {
    "groupId": "org.slf4j",
    "artifactId": "slf4j-api",
    "version": "2.0.9"
  }
}
```

Compare a dependency list:

```json
{
  "name": "compare_dependency_versions",
  "arguments": {
    "dependencies": [
      {
        "groupId": "org.slf4j",
        "artifactId": "slf4j-api",
        "version": "1.7.30"
      },
      {
        "groupId": "org.apache.commons",
        "artifactId": "commons-lang3",
        "version": "3.12.0"
      }
    ],
    "stabilityFilter": "PREFER_STABLE"
  }
}
```

Analyze project health and include license data:

```json
{
  "name": "analyze_project_health",
  "arguments": {
    "dependencies": [
      {
        "groupId": "org.slf4j",
        "artifactId": "slf4j-api",
        "version": "2.0.9"
      },
      {
        "groupId": "org.apache.commons",
        "artifactId": "commons-lang3",
        "version": "3.14.0"
      }
    ],
    "includeLicenses": true,
    "stabilityFilter": "STABLE_ONLY"
  }
}
```

Fetch a version timeline:

```json
{
  "name": "get_version_timeline",
  "arguments": {
    "groupId": "org.slf4j",
    "artifactId": "slf4j-api",
    "limit": 30
  }
}
```

## Response Format

Successful tools return JSON in MCP text content. The exact fields vary by tool, but successful responses include the requested Maven coordinates, versions, and analysis fields.

Tool errors are returned as MCP tool errors, also with a JSON body:

```json
{
  "status": "error",
  "errorType": "NOT_FOUND",
  "message": "No versions found for org.example:missing-artifact:"
}
```

Common error types:

| Error | Meaning |
| --- | --- |
| `INVALID_COORDINATES` | The Maven coordinates are incomplete or malformed. |
| `INVALID_ARGUMENTS` | A required argument is missing or a batch is too large. |
| `NOT_FOUND` | Maven Central or Context7 did not find a match. |
| `UPSTREAM_UNAVAILABLE` | Maven Central or Context7 could not be reached or returned an upstream failure. |
| `SERVICE_UNAVAILABLE` | An optional provider, usually Context7, is disabled. |
| `RATE_LIMITED` | Context7 returned rate limiting. |
| `INTERNAL_ERROR` | The server could not encode or process an unexpected internal result. |

## Configuration

Copy `.env.example` to `.env` for local development if you use `mise`. Real `.env` files are ignored by git.

| Variable | Default | Description |
| --- | --- | --- |
| `SERVER_NAME` | `io.github.aparcero/mcp-maven` | MCP server implementation name. |
| `SERVER_VERSION` | `1.0.0` | Version used when not overridden by build flags. |
| `TRANSPORT` | `stdio` | `stdio` or `http`. |
| `HTTP_PORT` | `8080` | HTTP port when using HTTP mode. |
| `MAVEN_CENTRAL_URL` | `https://repo1.maven.org/maven2` | Maven repository base URL. Can point to a mirror or proxy. |
| `MAVEN_TIMEOUT` | `10s` | Timeout for Maven repository requests. |
| `MAVEN_MAX_RESULTS` | `100` | Maximum versions kept from Maven metadata. |
| `MAX_CONCURRENT_REQUESTS` | `8` | Per-tool concurrency limit for batch work. |
| `MAX_DEPENDENCIES` | `100` | Maximum dependencies accepted by batch tools. |
| `CACHE_ALL_VERSIONS_TTL` | `1h` | TTL for artifact version lists. |
| `CACHE_VERSION_CHECKS_TTL` | `6h` | TTL for exact version existence checks. |
| `CACHE_HISTORICAL_DATA_TTL` | `24h` | TTL for version timestamp and timeline data. |
| `CACHE_TIMESTAMP_TTL` | `24h` | TTL for individual timestamp lookups. |
| `CACHE_MAX_SIZE` | `10000` | Maximum in-memory cache entries. |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |
| `LOG_FORMAT` | `text` | `text` or `json`. |
| `CONTEXT7_ENABLED` | `false` | Enables Context7 documentation integration. |
| `CONTEXT7_API_KEY` | empty | Optional Context7 API key. |
| `CONTEXT7_SERVER_URL` | `https://context7.com` | Context7 API base URL. |
| `CONTEXT7_TIMEOUT` | `30s` | Timeout for Context7 requests. |
| `METRICS_ENABLED` | `false` | Reserved for future metrics support; no metrics endpoint is currently exposed. |

## Context7 Documentation

Context7 integration is optional and disabled by default. Maven Central tools work without it.

Enable it with:

```bash
CONTEXT7_ENABLED=true CONTEXT7_API_KEY=your-token ./bin/mcp-maven
```

When enabled:

- `resolve-library-id` and `query-docs` are registered as raw Context7-style tools.
- `get_library_docs` resolves a Maven coordinate to likely documentation and returns matching snippets.
- Some dependency analysis tools add migration or modernization guidance for outdated dependencies.

## Network and Privacy

The server does not read your project files. It only processes the dependency coordinates provided by the MCP client.

Outbound requests:

- Maven Central or the configured `MAVEN_CENTRAL_URL`
- Context7 only when `CONTEXT7_ENABLED=true`

If you use a corporate repository proxy, set `MAVEN_CENTRAL_URL` to the proxy base URL.

## What This Server Does Not Do

- It does not parse `pom.xml`, Gradle files, or lockfiles by itself.
- It does not resolve transitive dependencies.
- It does not run vulnerability scanning.
- It does not decide whether an update is safe for your application.
- It does not authenticate HTTP requests in HTTP mode.

Agents should treat the output as dependency intelligence and combine it with build, test, changelog, and vulnerability data before changing production dependencies.

## Development

Useful commands:

```bash
mise run check
mise run tidy
mise run test
mise run test-integration
mise run test-race
mise run lint
mise run vulncheck
mise run secret-scan
mise run build
mise run build-all
```

Equivalent Go commands:

```bash
go mod tidy
go test -v ./...
go test -count=1 -run Integration -v ./...
go test -race ./...
go tool golangci-lint run ./...
go tool govulncheck ./...
if git grep -n -I -E 'BEGIN (RSA|DSA|EC|OPENSSH|PRIVATE) KEY|AKIA[0-9A-Z]{16}|ghp_[A-Za-z0-9_]{36}|github_pat_[A-Za-z0-9_]{82,}|xox[baprs]-[A-Za-z0-9-]{10,}|sk-[A-Za-z0-9]{20,}' -- . ':!go.sum'; then
  echo "Potential secrets found in tracked files" >&2
  exit 1
fi
mkdir -p bin
go build -o bin/mcp-maven ./cmd/mcp-maven
```

Run a single package or test:

```bash
go test -v ./internal/domain/...
go test -v -run TestHandlePing ./internal/mcp/
```

Run the opt-in live Maven Central integration test:

```bash
MCP_MAVEN_LIVE_TESTS=1 mise run test-integration
MCP_MAVEN_LIVE_TESTS=1 go test -count=1 -run Integration -v ./...
```

Build a release-style binary with version metadata:

```bash
mkdir -p bin
go build -trimpath \
  -ldflags "-s -w -X main.version=v0.1.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o bin/mcp-maven ./cmd/mcp-maven
```

## Troubleshooting

`server exits with invalid configuration`

Check `TRANSPORT`, `HTTP_PORT`, `LOG_LEVEL`, and duration values such as `MAVEN_TIMEOUT`.

`Context7 tools are missing`

Set `CONTEXT7_ENABLED=true`. The raw `resolve-library-id` and `query-docs` tools are only registered when Context7 is enabled.

`get_library_docs returns SERVICE_UNAVAILABLE`

Context7 is disabled. Maven Central tools still work.

`HTTP readiness fails`

`/health/ready` checks Maven Central by fetching metadata for a known artifact. Check network access, proxy settings, and `MAVEN_CENTRAL_URL`.

`STDIO client does not show tools`

Use an absolute binary path, restart the client after changing its MCP config, and reduce logs with `LOG_LEVEL=warn`.

## Contributing

Issues and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for development expectations.

Before opening a pull request, run:

```bash
mise run check
```

## Security

Please do not publish vulnerability details in public issues. See [SECURITY.md](SECURITY.md).

## License

MIT. See [LICENSE](LICENSE).

## Acknowledgments

- Original Java implementation: [arvindand/maven-tools-mcp](https://github.com/arvindand/maven-tools-mcp)
- [Model Context Protocol](https://modelcontextprotocol.io/)
- [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk)
