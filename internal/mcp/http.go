package mcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aparcero/mcp-maven/internal/observability"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HTTPServer wraps the MCP server with HTTP transport.
type HTTPServer struct {
	mcpServer  *Server
	httpServer *http.Server
	health     *observability.HealthChecker
	handler    http.Handler
}

// NewHTTPServer creates a new HTTP server.
func NewHTTPServer(mcpServer *Server) *HTTPServer {
	health := mcpServer.Health()

	mux := http.NewServeMux()

	// Register health check endpoints
	health.RegisterHTTPHandlers(mux)

	mcpHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return mcpServer.MCPServer()
	}, &mcpsdk.StreamableHTTPOptions{JSONResponse: true})
	mux.Handle("/mcp", mcpHandler)

	// Root endpoint with server info
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{
			"name": "%s",
			"version": "%s",
			"transport": "http",
			"endpoints": {
				"mcp": "/mcp",
				"health": "/health/live",
				"ready": "/health/ready"
			}
		}`, mcpServer.config.ServerName, mcpServer.config.ServerVersion)
	})

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", mcpServer.config.HTTPPort),
		Handler: mux,
	}

	return &HTTPServer{
		mcpServer:  mcpServer,
		httpServer: httpServer,
		health:     health,
		handler:    mux,
	}
}

// Run starts the HTTP server.
func (h *HTTPServer) Run(ctx context.Context) error {
	observability.Info("Starting HTTP server",
		"port", h.mcpServer.config.HTTPPort,
		"version", h.mcpServer.config.ServerVersion)

	// Mark server as ready
	h.health.SetReady(true)

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := h.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for context cancellation
	select {
	case <-ctx.Done():
		observability.Info("Shutting down HTTP server")
		// Graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := h.httpServer.Shutdown(shutdownCtx); err != nil {
			h.health.SetReady(false)
			return err
		}
		h.health.SetReady(false)
		return nil
	case err := <-errChan:
		h.health.SetReady(false)
		return err
	}
}

// Handler returns the HTTP handler used by the server.
func (h *HTTPServer) Handler() http.Handler {
	return h.handler
}
