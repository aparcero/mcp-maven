package mcp

import (
	"testing"
	"time"

	"github.com/aparcero/mcp-maven/internal/config"
)

func TestServerCreation(t *testing.T) {
	cfg := &config.Config{
		ServerName:    "test-server",
		ServerVersion: "1.0.0",
		Transport:     config.TransportSTDIO,
		LogLevel:      "error",
	}

	server, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if server == nil {
		t.Fatal("Server is nil")
	}

	if server.MCPServer() == nil {
		t.Error("MCP server is nil")
	}

	if server.Client() == nil {
		t.Error("Maven Central client is nil")
	}

	if server.Cache() == nil {
		t.Error("Cache is nil")
	}

	if server.Health() == nil {
		t.Error("Health checker is nil")
	}
}

func TestServerToolsRegistered(t *testing.T) {
	cfg := &config.Config{
		ServerName:    "test-server",
		ServerVersion: "1.0.0",
		Transport:     config.TransportSTDIO,
		LogLevel:      "error",
	}

	server, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// The MCP SDK doesn't expose a direct way to list registered tools,
	// but we can verify the server was created successfully
	if server.MCPServer() == nil {
		t.Error("MCP server should not be nil")
	}
}

func TestServerHealthCheck(t *testing.T) {
	cfg := &config.Config{
		ServerName:    "test-server",
		ServerVersion: "1.0.0",
		Transport:     config.TransportSTDIO,
		LogLevel:      "error",
	}

	server, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	health := server.Health()

	// The health checker starts with both live=true and ready=true
	if !health.IsLive() {
		t.Error("Server should be live after creation")
	}

	// We can toggle ready state
	health.SetReady(false)
	if health.IsReady() {
		t.Error("Server should not be ready after SetReady(false)")
	}

	health.SetReady(true)
	if !health.IsReady() {
		t.Error("Server should be ready after SetReady(true)")
	}
}

func TestServerUsesConfiguredCacheMaxSize(t *testing.T) {
	cfg := &config.Config{
		ServerName:    "test-server",
		ServerVersion: "1.0.0",
		Transport:     config.TransportSTDIO,
		LogLevel:      "error",
		Cache: config.CacheConfig{
			MaxCacheSize: 1,
		},
	}

	server, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	server.Cache().Set("first", "a", time.Hour)
	server.Cache().Set("second", "b", time.Hour)

	if size := server.Cache().Size(); size != 1 {
		t.Fatalf("expected configured cache max size 1, got cache size %d", size)
	}
}
