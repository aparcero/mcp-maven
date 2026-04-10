package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/aparcero/mcp-maven/internal/config"
)

func TestHTTPServerExposesStreamableMCPEndpoint(t *testing.T) {
	server := newTestServer(t, "https://repo1.maven.org/maven2", func(cfg *config.Config) {
		cfg.Transport = config.TransportHTTP
	})

	httpServer := NewHTTPServer(server)
	testServer := httptest.NewServer(httpServer.Handler())
	defer testServer.Close()

	reqBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/mcp", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("failed to build initialize request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("initialize request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode initialize response: %v", err)
	}

	result := payload["result"].(map[string]any)
	serverInfo := result["serverInfo"].(map[string]any)
	if serverInfo["name"] != "test-server" {
		t.Fatalf("expected serverInfo.name test-server, got %v", serverInfo["name"])
	}

	healthResp, err := http.Get(testServer.URL + "/health/live")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer func() { _ = healthResp.Body.Close() }()

	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("expected health status 200, got %d", healthResp.StatusCode)
	}
}

func TestHTTPServerCallsToolThroughStreamableMCP(t *testing.T) {
	server := newTestServer(t, "https://repo1.maven.org/maven2", func(cfg *config.Config) {
		cfg.Transport = config.TransportHTTP
	})

	httpServer := NewHTTPServer(server)
	testServer := httptest.NewServer(httpServer.Handler())
	defer testServer.Close()

	initPayload := postMCPJSON(t, testServer.URL+"/mcp", "", http.StatusOK, []byte(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2025-03-26",
			"capabilities": {},
			"clientInfo": {"name": "mcp-maven-test", "version": "1.0.0"}
		}
	}`))

	sessionID := initPayload.sessionID
	if sessionID == "" {
		t.Fatal("expected initialize response to include Mcp-Session-Id")
	}

	_ = postMCPJSON(t, testServer.URL+"/mcp", sessionID, http.StatusAccepted, []byte(`{
		"jsonrpc": "2.0",
		"method": "notifications/initialized",
		"params": {}
	}`))

	callPayload := postMCPJSON(t, testServer.URL+"/mcp", sessionID, http.StatusOK, []byte(`{
		"jsonrpc": "2.0",
		"id": 2,
		"method": "tools/call",
		"params": {
			"name": "ping",
			"arguments": {"message": "through-http"}
		}
	}`))

	result := callPayload.body["result"].(map[string]any)
	content := result["content"].([]any)
	firstContent := content[0].(map[string]any)
	if firstContent["type"] != "text" {
		t.Fatalf("expected text content, got %v", firstContent["type"])
	}

	var toolResult map[string]any
	if err := json.Unmarshal([]byte(firstContent["text"].(string)), &toolResult); err != nil {
		t.Fatalf("failed to decode tool text JSON: %v", err)
	}
	if toolResult["message"] != "pong" {
		t.Fatalf("expected pong message, got %v", toolResult["message"])
	}
	if toolResult["echo"] != "through-http" {
		t.Fatalf("expected echo through-http, got %v", toolResult["echo"])
	}
}

func TestHTTPServerShutdownAllowsActiveRequestsToFinish(t *testing.T) {
	port := freeTCPPort(t)
	server := newTestServer(t, "https://repo1.maven.org/maven2", func(cfg *config.Config) {
		cfg.Transport = config.TransportHTTP
		cfg.HTTPPort = port
	})

	httpServer := NewHTTPServer(server)
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/block", func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-releaseRequest
		w.WriteHeader(http.StatusNoContent)
	})
	httpServer.httpServer.Handler = mux

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.Run(ctx)
	}()

	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)
	waitForHTTPReady(t, baseURL)

	requestErrCh := make(chan error, 1)
	go func() {
		resp, err := http.Get(baseURL + "/block")
		if err != nil {
			requestErrCh <- err
			return
		}
		defer func() { _ = resp.Body.Close() }()
		requestErrCh <- nil
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocking request to start")
	}

	cancel()

	select {
	case err := <-errCh:
		if errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("shutdown deadline expired before active request could finish: %v", err)
		}
		t.Fatalf("server returned before active request was released: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseRequest)

	select {
	case err := <-requestErrCh:
		if err != nil {
			t.Fatalf("blocking request failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for blocking request to finish")
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected graceful shutdown, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for HTTP server shutdown")
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to allocate free TCP port: %v", err)
	}
	defer func() { _ = listener.Close() }()

	return listener.Addr().(*net.TCPAddr).Port
}

func waitForHTTPReady(t *testing.T, baseURL string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/")
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for HTTP server at %s", baseURL)
}

type mcpHTTPResponse struct {
	body      map[string]any
	sessionID string
}

func postMCPJSON(t *testing.T, url, sessionID string, expectedStatus int, body []byte) mcpHTTPResponse {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to build MCP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Accept", "text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("MCP request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read MCP response body: %v", err)
	}

	if resp.StatusCode != expectedStatus {
		t.Fatalf("expected MCP status %d, got %d; body=%s", expectedStatus, resp.StatusCode, string(responseBody))
	}

	payload := map[string]any{}
	if len(bytes.TrimSpace(responseBody)) > 0 {
		if err := json.Unmarshal(responseBody, &payload); err != nil {
			t.Fatalf("failed to decode MCP response JSON: %v; body=%s", err, string(responseBody))
		}
	}

	return mcpHTTPResponse{
		body:      payload,
		sessionID: resp.Header.Get("Mcp-Session-Id"),
	}
}
