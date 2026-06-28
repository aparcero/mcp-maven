package mcp

import (
	"context"
	"fmt"

	"github.com/aparcero/mcp-maven/internal/cache"
	"github.com/aparcero/mcp-maven/internal/config"
	"github.com/aparcero/mcp-maven/internal/context7"
	"github.com/aparcero/mcp-maven/internal/domain"
	"github.com/aparcero/mcp-maven/internal/mavencentral"
	"github.com/aparcero/mcp-maven/internal/observability"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server represents the Maven Tools MCP server.
type Server struct {
	config    *config.Config
	mcpServer *mcp.Server
	client    *mavencentral.Client
	docs      context7.DocsProvider
	cache     *cache.Cache
	health    *observability.HealthChecker
}

// New creates a new Maven Tools MCP server.
func New(cfg *config.Config) (*Server, error) {
	cfg.ApplyDefaults()

	// Initialize logging
	if err := observability.Init(cfg.LogLevel); err != nil {
		return nil, fmt.Errorf("failed to initialize logging: %w", err)
	}

	// Initialize cache
	c := cache.New(cfg.Cache.MaxCacheSize)

	// Initialize Maven Central client
	mavenClient := mavencentral.NewClient(cfg.MavenCentral, c, cfg.Cache)

	// Initialize health checker
	health := observability.NewHealthChecker()

	// Initialize Context7 client/provider
	docsProvider := context7.NewClient(cfg.Context7)

	// Register Maven Central health check
	health.RegisterCheck("maven_central", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.MavenCentral.Timeout)
		defer cancel()
		// Simple check - try to fetch metadata for a known artifact
		coord, _ := domain.ParseCoordinates("org.slf4j:slf4j-api")
		_, err := mavenClient.GetAllVersions(ctx, coord)
		return err
	})

	// Create MCP server
	impl := &mcp.Implementation{
		Name:    cfg.ServerName,
		Version: cfg.ServerVersion,
	}

	serverOpts := &mcp.ServerOptions{}
	mcpServer := mcp.NewServer(impl, serverOpts)

	s := &Server{
		config:    cfg,
		mcpServer: mcpServer,
		client:    mavenClient,
		docs:      docsProvider,
		cache:     c,
		health:    health,
	}

	// Register tools
	if err := s.registerTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	return s, nil
}

// registerTools registers all MCP tools.
func (s *Server) registerTools() error {
	// Register Phase 1 tools
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_latest_version",
		Description: "Get the latest version of a Maven artifact from Maven Central, with optional stability filtering.",
	}, s.handleGetLatestVersion)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "check_version_exists",
		Description: "Check if a specific version of a Maven artifact exists in Maven Central.",
	}, s.handleCheckVersionExists)

	// Register Phase 2 tools
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "check_multiple_dependencies",
		Description: "Check multiple dependencies for existence and get their latest versions concurrently.",
	}, s.handleCheckMultipleDependencies)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "compare_dependency_versions",
		Description: "Compare current versions of dependencies against their latest available versions.",
	}, s.handleCompareDependencyVersions)

	// Register Phase 3 tools (Historical Analytics)
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_version_timeline",
		Description: "Get a timeline of versions with release dates and patterns.",
	}, s.handleGetVersionTimeline)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "analyze_dependency_age",
		Description: "Analyze the age of multiple dependencies and classify their freshness.",
	}, s.handleAnalyzeDependencyAge)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "analyze_release_patterns",
		Description: "Analyze release patterns for a Maven artifact including cadence and strategy.",
	}, s.handleAnalyzeReleasePatterns)

	// Register Phase 4 tools (Project Health)
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "analyze_project_health",
		Description: "Comprehensive health analysis for multiple dependencies including age, updates, and recommendations.",
	}, s.handleAnalyzeProjectHealth)

	// Register Phase 5 tools (Context7 compatibility wrapper)
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_library_docs",
		Description: "Resolve a Maven coordinate in Context7 and fetch matching documentation, with links to javadoc.io and Maven Central.",
	}, s.handleGetLibraryDocs)

	// Add a ping tool for testing
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "ping",
		Description: "A simple ping tool to test connectivity.",
	}, s.handlePing)

	return nil
}

// Run starts the MCP server on the appropriate transport.
func (s *Server) Run(ctx context.Context) error {
	observability.Info("Starting Maven Tools MCP server",
		"transport", s.config.Transport,
		"version", s.config.ServerVersion)

	switch s.config.Transport {
	case config.TransportSTDIO:
		return s.runSTDIO(ctx)
	case config.TransportHTTP:
		return s.runHTTP(ctx)
	default:
		return fmt.Errorf("unsupported transport: %s", s.config.Transport)
	}
}

// runSTDIO runs the server in STDIO mode.
func (s *Server) runSTDIO(ctx context.Context) error {
	// Mark server as ready
	s.health.SetReady(true)

	transport := &mcp.StdioTransport{}
	err := s.mcpServer.Run(ctx, transport)

	// Mark server as not ready on exit
	s.health.SetReady(false)

	return err
}

// runHTTP runs the server in HTTP mode.
func (s *Server) runHTTP(ctx context.Context) error {
	httpServer := NewHTTPServer(s)
	return httpServer.Run(ctx)
}

// MCPServer returns the underlying MCP server.
func (s *Server) MCPServer() *mcp.Server {
	return s.mcpServer
}

// Client returns the Maven Central client.
func (s *Server) Client() *mavencentral.Client {
	return s.client
}

// Cache returns the cache instance.
func (s *Server) Cache() *cache.Cache {
	return s.cache
}

// Docs returns the Context7 provider.
func (s *Server) Docs() context7.DocsProvider {
	return s.docs
}

// Config returns the server configuration.
func (s *Server) Config() *config.Config {
	return s.config
}

// Health returns the health checker.
func (s *Server) Health() *observability.HealthChecker {
	return s.health
}
