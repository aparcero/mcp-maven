package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/aparcero/mcp-maven/internal/config"
	"github.com/aparcero/mcp-maven/internal/mcp"
	"github.com/aparcero/mcp-maven/internal/observability"
)

var (
	// Version information (set via ldflags)
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Parse command-line flags
	flagHTTP := flag.Bool("http", false, "Run in HTTP mode")
	flagPort := flag.Int("port", 8080, "HTTP port (only for HTTP mode)")
	flagVersion := flag.Bool("version", false, "Show version information")
	flagHelp := flag.Bool("help", false, "Show help message")
	flag.Parse()

	// Handle version flag
	if *flagVersion {
		fmt.Printf("mcp-maven\n")
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Built: %s\n", date)
		os.Exit(0)
	}

	// Handle help flag
	if *flagHelp {
		printHelp()
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Override with command-line flags
	if *flagHTTP {
		cfg.Transport = config.TransportHTTP
		cfg.HTTPPort = *flagPort
	}

	// Set version from build info
	cfg.ServerVersion = version

	// Create server
	server, err := mcp.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create server: %v\n", err)
		os.Exit(1)
	}

	// Setup signal handling for graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Run server
	if err := server.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			observability.Info("Server stopped gracefully")
			return
		}
		observability.Error("Server error", "error", err)
		os.Exit(1)
	}

	observability.Info("Server stopped gracefully")
}

func printHelp() {
	fmt.Printf("mcp-maven - JVM dependency intelligence via Maven Central\n\n")
	fmt.Printf("Usage:\n")
	fmt.Printf("  mcp-maven [options]\n\n")
	fmt.Printf("Options:\n")
	fmt.Printf("  -http       Run in HTTP mode (default: STDIO)\n")
	fmt.Printf("  -port PORT  HTTP port for HTTP mode (default: 8080)\n")
	fmt.Printf("  -version    Show version information\n")
	fmt.Printf("  -help       Show this help message\n\n")
	fmt.Printf("Environment Variables:\n")
	fmt.Printf("  TRANSPORT           Transport mode: stdio or http (default: stdio)\n")
	fmt.Printf("  MAVEN_CENTRAL_URL   Maven Central base URL (default: https://repo1.maven.org/maven2)\n")
	fmt.Printf("  MAVEN_TIMEOUT       Request timeout (default: 10s)\n")
	fmt.Printf("  LOG_LEVEL           Log level: debug, info, warn, error (default: info)\n")
	fmt.Printf("  LOG_FORMAT          Log format: text or json (default: text)\n")
	fmt.Printf("  HTTP_PORT           HTTP port (default: 8080)\n")
	fmt.Printf("  CONTEXT7_ENABLED    Enable Context7 integration (default: false)\n")
	fmt.Printf("\n")
}
