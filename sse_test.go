package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestProxyConnectsClaudeStyleSSEServerAndProxiesToolCall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fixture := newClaudeStyleSSEFixture(t, "Bearer secret-token")
	defer fixture.Close()

	proxy := newProxyServer(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := proxy.connectChildren(ctx, []ScopedServer{{
		Name:  "remote",
		Scope: "project",
		Config: MCPServerConfig{
			Type: "sse",
			URL:  fixture.URL + "/sse",
			Headers: map[string]string{
				"Authorization": "Bearer secret-token",
			},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	defer proxy.close()

	session, closeSession := connectTestClient(t, ctx, proxy)
	defer closeSession()

	var tool *mcpsdk.Tool
	for next, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		if next.Name == "remote__example_read" {
			tool = next
		}
	}
	if tool == nil {
		t.Fatal("remote__example_read was not exposed")
	}
	if tool.Description != "read example data" {
		t.Fatalf("description = %q", tool.Description)
	}
	schema, ok := tool.InputSchema.(map[string]any)
	if !ok || schema["type"] != "object" {
		t.Fatalf("input schema = %#v", tool.InputSchema)
	}

	res, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "remote__example_read",
		Arguments: map[string]any{
			"target":    "sample",
			"statement": "SELECT 3 AS item_count",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	structured, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content = %#v", res.StructuredContent)
	}
	if structured["target"] != "sample" || structured["item_count"] != float64(3) {
		t.Fatalf("structured content = %#v", structured)
	}

	for _, method := range []string{"initialize", "notifications/initialized", "tools/list", "tools/call"} {
		if !slices.Contains(fixture.Methods(), method) {
			t.Fatalf("JSON-RPC method %q was not observed; methods=%v", method, fixture.Methods())
		}
	}
	if got := fixture.AuthenticatedRequests(); got < 2 {
		t.Fatalf("authenticated requests = %d, want at least GET and POST", got)
	}
}

func TestServeProcessUsesCodexForwardedProjectEnvForSSEHeader(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fixture := newClaudeStyleSSEFixture(t, "Bearer codex-forwarded-token")
	defer fixture.Close()

	tmp := t.TempDir()
	home := tmp + "/home"
	project := tmp + "/project"
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, project+"/.mcp.json", map[string]any{
		"mcpServers": map[string]any{
			"remote-tools": map[string]any{
				"type": "sse",
				"url":  fixture.URL + "/sse",
				"headers": map[string]string{
					"Authorization": "Bearer ${CODEX_STYLE_REMOTE_TOKEN}",
				},
			},
		},
	})

	cmd := exec.CommandContext(ctx, os.Args[0])
	cmd.Env = []string{
		"CLAUDE_TO_CODEX_TEST_SERVE=1",
		"CLAUDE_BRIDGE_PROJECT_ROOT=" + project,
		"CODEX_STYLE_REMOTE_TOKEN=codex-forwarded-token",
		"HOME=" + home,
		"PATH=" + os.Getenv("PATH"),
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "codex-style-test-client", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.CommandTransport{Command: cmd, TerminateDuration: 2 * time.Second}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	var names []string
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, tool.Name)
	}
	if !slices.Contains(names, "remote-tools__example_read") {
		t.Fatalf("tools = %v, want remote-tools__example_read", names)
	}

	res, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "remote-tools__example_read",
		Arguments: map[string]any{
			"target":    "sample",
			"statement": "SELECT 3 AS item_count",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	structured, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content = %#v", res.StructuredContent)
	}
	if structured["target"] != "sample" || structured["item_count"] != float64(3) {
		t.Fatalf("structured content = %#v", structured)
	}
	if got := fixture.AuthenticatedRequests(); got < 2 {
		t.Fatalf("authenticated requests = %d, want at least GET and POST", got)
	}
}

func TestProxyToolCallWrapsLateSSEDisconnectWithChildContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fixture := newClaudeStyleSSEFixture(t, "Bearer secret-token")
	fixture.closeOnToolCall = true
	defer fixture.Close()

	proxy := newProxyServer(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := proxy.connectChildren(ctx, []ScopedServer{{
		Name:  "remote",
		Scope: "project",
		Config: MCPServerConfig{
			Type: "sse",
			URL:  fixture.URL + "/sse",
			Headers: map[string]string{
				"Authorization": "Bearer secret-token",
			},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	defer proxy.close()

	session, closeSession := connectTestClient(t, ctx, proxy)
	defer closeSession()

	_, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "remote__example_read",
		Arguments: map[string]any{
			"target":    "sample",
			"statement": "SELECT 3 AS item_count",
		},
	})
	if err == nil {
		t.Fatal("CallTool succeeded after child SSE stream closed")
	}
	text := err.Error()
	for _, want := range []string{
		`project-scope MCP server "remote"`,
		`tools/call`,
		`remote__example_read`,
		`example_read`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("error = %q, want %q", text, want)
		}
	}
}

func TestProxyToolCallTimeoutReportsChildContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fixture := newClaudeStyleSSEFixture(t, "Bearer secret-token")
	fixture.toolCallDelay = 100 * time.Millisecond
	defer fixture.Close()

	proxy := newProxyServer(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := proxy.connectChildren(ctx, []ScopedServer{{
		Name:  "remote",
		Scope: "project",
		Config: MCPServerConfig{
			Type: "sse",
			URL:  fixture.URL + "/sse",
			Headers: map[string]string{
				"Authorization": "Bearer secret-token",
			},
		},
	}}); err != nil {
		t.Fatal(err)
	}
	defer proxy.close()

	session, closeSession := connectTestClient(t, ctx, proxy)
	defer closeSession()

	proxy.operationTimeout = 10 * time.Millisecond
	_, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "remote__example_read",
		Arguments: map[string]any{
			"target":    "sample",
			"statement": "SELECT 3 AS item_count",
		},
	})
	if err == nil {
		t.Fatal("CallTool succeeded after child operation timeout")
	}
	text := err.Error()
	for _, want := range []string{
		`project-scope MCP server "remote"`,
		`tools/call`,
		`context deadline exceeded`,
		`timed out`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("error = %q, want %q", text, want)
		}
	}
}

type claudeStyleSSEFixture struct {
	*httptest.Server

	t           *testing.T
	authHeader  string
	events      chan []byte
	closeStream chan struct{}

	mu                    sync.Mutex
	methods               []string
	authenticatedRequests int
	closeOnToolCall       bool
	toolCallDelay         time.Duration
	closeOnce             sync.Once
}

func newClaudeStyleSSEFixture(t *testing.T, authHeader string) *claudeStyleSSEFixture {
	t.Helper()
	fixture := &claudeStyleSSEFixture{
		t:           t,
		authHeader:  authHeader,
		events:      make(chan []byte, 100),
		closeStream: make(chan struct{}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", fixture.handleSSE)
	mux.HandleFunc("/message", fixture.handleMessage)
	fixture.Server = httptest.NewServer(mux)
	return fixture
}

func (f *claudeStyleSSEFixture) Methods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.methods)
}

func (f *claudeStyleSSEFixture) AuthenticatedRequests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.authenticatedRequests
}

func (f *claudeStyleSSEFixture) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	if !f.checkAuth(w, r) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	fmt.Fprint(w, "event: endpoint\r\n")
	fmt.Fprint(w, "data: /message?sessionId=test-session\r\n\r\n")
	flusher.Flush()

	for {
		select {
		case data := <-f.events:
			fmt.Fprintf(w, "event: message\r\ndata: %s\r\n\r\n", data)
			flusher.Flush()
		case <-f.closeStream:
			return
		case <-r.Context().Done():
			return
		}
	}
}

func (f *claudeStyleSSEFixture) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if !f.checkAuth(w, r) {
		return
	}

	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON-RPC", http.StatusBadRequest)
		return
	}
	f.recordMethod(req.Method)
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	response := map[string]any{"jsonrpc": "2.0", "id": req.ID}
	switch req.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &params)
		response["result"] = map[string]any{
			"protocolVersion": params.ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "sse-fixture", "version": "0"},
		}
	case "tools/list":
		response["result"] = map[string]any{
			"tools": []map[string]any{{
				"name":        "example_read",
				"description": "read example data",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"target":    map[string]any{"type": "string"},
						"statement": map[string]any{"type": "string"},
					},
					"required": []string{"target", "statement"},
				},
			}},
		}
	case "tools/call":
		if f.toolCallDelay > 0 {
			select {
			case <-time.After(f.toolCallDelay):
			case <-r.Context().Done():
				return
			}
		}
		if f.closeOnToolCall {
			f.closeOnce.Do(func() {
				close(f.closeStream)
			})
			w.WriteHeader(http.StatusAccepted)
			return
		}
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &params)
		if params.Name != "example_read" {
			response["error"] = map[string]any{"code": -32602, "message": "unknown tool"}
			break
		}
		response["result"] = map[string]any{
			"content": []map[string]any{{
				"type": "text",
				"text": "ok",
			}},
			"structuredContent": map[string]any{
				"target":     params.Arguments["target"],
				"item_count": 3,
			},
		}
	case "prompts/list":
		response["result"] = map[string]any{"prompts": []any{}}
	case "resources/list":
		response["result"] = map[string]any{"resources": []any{}}
	case "resources/templates/list":
		response["result"] = map[string]any{"resourceTemplates": []any{}}
	default:
		response["error"] = map[string]any{"code": -32601, "message": "method not found"}
	}
	data, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "encode response", http.StatusInternalServerError)
		return
	}
	f.events <- data
	w.WriteHeader(http.StatusAccepted)
}

func (f *claudeStyleSSEFixture) checkAuth(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Authorization") != f.authHeader {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	f.mu.Lock()
	f.authenticatedRequests++
	f.mu.Unlock()
	return true
}

func (f *claudeStyleSSEFixture) recordMethod(method string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.methods = append(f.methods, method)
}

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}
