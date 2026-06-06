package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const fakeChildEnv = "CLAUDE_TO_CODEX_FAKE_CHILD"
const testServeEnv = "CLAUDE_TO_CODEX_TEST_SERVE"

type fakeToolOutput struct {
	Server string `json:"server"`
	Tool   string `json:"tool"`
	CWD    string `json:"cwd"`
	Env    string `json:"env,omitempty"`
}

func TestMain(m *testing.M) {
	if os.Getenv(testServeEnv) == "1" {
		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		if err := runServe(logger); err != nil {
			log.Fatal(err)
		}
		return
	}
	if name := os.Getenv(fakeChildEnv); name != "" {
		if err := runFakeChildServer(name); err != nil {
			log.Fatal(err)
		}
		return
	}
	os.Exit(m.Run())
}

func TestProxyListsAndCallsUserProjectAndCollisionTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	projectRoot := t.TempDir()

	session, closeProxy := startTestProxy(t, ctx, []ScopedServer{
		fakeServer("user", "user", ""),
		fakeServer("project", "project", projectRoot),
	})
	defer closeProxy()

	var names []string
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	wantNames := []string{
		"project__env_AMBIENT_SECRET_TOKEN",
		"project__env_CLAUDE_BRIDGE_PROJECT_ROOT",
		"project__env_EXPLICIT_TOKEN",
		"project__project_only",
		"project__shared",
		"user__env_AMBIENT_SECRET_TOKEN",
		"user__env_CLAUDE_BRIDGE_PROJECT_ROOT",
		"user__env_EXPLICIT_TOKEN",
		"user__shared",
		"user__user_only",
	}
	if got := stringsJoin(names); got != stringsJoin(wantNames) {
		t.Fatalf("tool names = %v, want %v", names, wantNames)
	}

	userOnly := callFakeTool(t, ctx, session, "user__user_only")
	if userOnly.Server != "user" || userOnly.Tool != "user_only" {
		t.Fatalf("user_only result = %#v", userOnly)
	}

	projectOnly := callFakeTool(t, ctx, session, "project__project_only")
	if projectOnly.Server != "project" || projectOnly.Tool != "project_only" {
		t.Fatalf("project_only result = %#v", projectOnly)
	}
	if projectOnly.CWD != projectRoot {
		t.Fatalf("project child cwd = %q, want %q", projectOnly.CWD, projectRoot)
	}

	userShared := callFakeTool(t, ctx, session, "user__shared")
	if userShared.Server != "user" || userShared.Tool != "shared" {
		t.Fatalf("user__shared result = %#v", userShared)
	}
	projectShared := callFakeTool(t, ctx, session, "project__shared")
	if projectShared.Server != "project" || projectShared.Tool != "shared" {
		t.Fatalf("project__shared result = %#v", projectShared)
	}
}

func TestProxyContinuesWhenOneChildCannotConnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	proxy := newProxyServer(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	failures := proxy.connectChildrenBestEffort(ctx, []ScopedServer{
		{Name: "broken", Scope: "project", Config: MCPServerConfig{Command: filepath.Join(t.TempDir(), "missing-mcp")}},
		fakeServer("user", "user", ""),
	})
	defer proxy.close()
	if len(failures) != 1 {
		t.Fatalf("failures = %d, want 1", len(failures))
	}
	if len(proxy.children) != 1 {
		t.Fatalf("connected children = %d, want 1", len(proxy.children))
	}

	session, closeSession := connectTestClient(t, ctx, proxy)
	defer closeSession()
	got := callFakeTool(t, ctx, session, "user__user_only")
	if got.Server != "user" {
		t.Fatalf("user_only server = %q, want user", got.Server)
	}
}

func TestRestrictedChildEnvExcludesAmbientSecretsAndKeepsExplicitEnv(t *testing.T) {
	t.Setenv("AMBIENT_SECRET_TOKEN", "ambient-secret")
	t.Setenv("PATH", os.Getenv("PATH"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	proxy := newProxyServer(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := proxy.connectChildren(ctx, []ScopedServer{{
		Name:   "user",
		Scope:  "user",
		Config: MCPServerConfig{Command: os.Args[0], Env: map[string]string{fakeChildEnv: "user", "EXPLICIT_TOKEN": "explicit-secret"}},
	}}); err != nil {
		t.Fatal(err)
	}
	defer proxy.close()
	session, closeSession := connectTestClient(t, ctx, proxy)
	defer closeSession()

	if got := callFakeTool(t, ctx, session, "user__env_EXPLICIT_TOKEN"); got.Env != "explicit-secret" {
		t.Fatalf("explicit env = %q, want explicit-secret", got.Env)
	}
	if got := callFakeTool(t, ctx, session, "user__env_AMBIENT_SECRET_TOKEN"); got.Env != "" {
		t.Fatalf("ambient secret leaked into restricted child env: %q", got.Env)
	}
}

func TestPerServerInheritEnvAllowsAmbientEnv(t *testing.T) {
	t.Setenv("AMBIENT_SECRET_TOKEN", "ambient-secret")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	proxy := newProxyServer(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := proxy.connectChildren(ctx, []ScopedServer{{
		Name:   "user",
		Scope:  "user",
		Config: MCPServerConfig{Command: os.Args[0], Env: map[string]string{fakeChildEnv: "user"}, InheritEnv: true},
	}}); err != nil {
		t.Fatal(err)
	}
	defer proxy.close()
	session, closeSession := connectTestClient(t, ctx, proxy)
	defer closeSession()

	if got := callFakeTool(t, ctx, session, "user__env_AMBIENT_SECRET_TOKEN"); got.Env != "ambient-secret" {
		t.Fatalf("ambient env = %q, want ambient-secret", got.Env)
	}
}

func TestGlobalInheritEnvAllowsAmbientEnv(t *testing.T) {
	t.Setenv("AMBIENT_SECRET_TOKEN", "ambient-secret")
	t.Setenv(inheritEnvVar, "1")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	proxy := newProxyServer(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := proxy.connectChildren(ctx, []ScopedServer{{
		Name:   "user",
		Scope:  "user",
		Config: MCPServerConfig{Command: os.Args[0], Env: map[string]string{fakeChildEnv: "user"}},
	}}); err != nil {
		t.Fatal(err)
	}
	defer proxy.close()
	session, closeSession := connectTestClient(t, ctx, proxy)
	defer closeSession()

	if got := callFakeTool(t, ctx, session, "user__env_AMBIENT_SECRET_TOKEN"); got.Env != "ambient-secret" {
		t.Fatalf("ambient env = %q, want ambient-secret", got.Env)
	}
}

func TestProjectChildReceivesProjectRootEnv(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	projectRoot := t.TempDir()
	session, closeProxy := startTestProxy(t, ctx, []ScopedServer{fakeServer("project", "project", projectRoot)})
	defer closeProxy()

	if got := callFakeTool(t, ctx, session, "project__env_CLAUDE_BRIDGE_PROJECT_ROOT"); got.Env != projectRoot {
		t.Fatalf("project root env = %q, want %q", got.Env, projectRoot)
	}
}

func TestProxyPerChildConnectTimeoutDoesNotStarveLaterChildren(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	proxy := newProxyServer(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	proxy.operationTimeout = 100 * time.Millisecond
	failures := proxy.connectChildrenBestEffort(ctx, []ScopedServer{
		{Name: "slow", Scope: "user", Config: MCPServerConfig{Command: "sh", Args: []string{"-c", "sleep 5"}}},
		fakeServer("user", "user", ""),
	})
	defer proxy.close()
	if len(failures) != 1 || failures[0].server.Name != "slow" {
		t.Fatalf("failures = %#v, want one slow failure", failures)
	}
	if len(proxy.children) != 1 || proxy.children[0].Name != "user" {
		t.Fatalf("connected children = %#v, want later user child connected", proxy.children)
	}

	session, closeSession := connectTestClient(t, ctx, proxy)
	defer closeSession()
	got := callFakeTool(t, ctx, session, "user__user_only")
	if got.Server != "user" {
		t.Fatalf("user_only server = %q, want user", got.Server)
	}
}

func TestProxyChildSurvivesConnectContextCancellation(t *testing.T) {
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelConnect()
	proxy := newProxyServer(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := proxy.connectChildren(connectCtx, []ScopedServer{fakeServer("project", "project", "")}); err != nil {
		t.Fatal(err)
	}
	defer proxy.close()

	serverCtx, cancelServer := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelServer()
	session, closeSession := connectTestClient(t, serverCtx, proxy)
	defer closeSession()

	cancelConnect()
	got := callFakeTool(t, serverCtx, session, "project__project_only")
	if got.Server != "project" || got.Tool != "project_only" {
		t.Fatalf("project_only result = %#v", got)
	}
}

func TestInspectToolsReportsExposedToolsAndFailures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "project")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(home, ".claude.json"), map[string]any{
		"mcpServers": map[string]any{
			"user": fakeServerConfig("user"),
		},
	})
	writeJSON(t, filepath.Join(project, ".mcp.json"), map[string]any{
		"mcpServers": map[string]any{
			"project": fakeServerConfig("project"),
			"broken":  map[string]any{"command": filepath.Join(tmp, "missing-mcp")},
		},
	})
	servers, err := loadClaudeServers(home, project)
	if err != nil {
		t.Fatal(err)
	}
	proxy := newProxyServer(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	failures := proxy.connectChildrenBestEffort(ctx, servers)
	defer proxy.close()
	tools, toolFailures := proxy.inspectTools(ctx)
	failures = append(failures, toolFailures...)
	if len(failures) != 1 || failures[0].server.Name != "broken" {
		t.Fatalf("failures = %#v, want one broken server", failures)
	}
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.exposedName)
	}
	sort.Strings(names)
	wantNames := []string{
		"project__env_AMBIENT_SECRET_TOKEN",
		"project__env_CLAUDE_BRIDGE_PROJECT_ROOT",
		"project__env_EXPLICIT_TOKEN",
		"project__project_only",
		"project__shared",
		"user__env_AMBIENT_SECRET_TOKEN",
		"user__env_CLAUDE_BRIDGE_PROJECT_ROOT",
		"user__env_EXPLICIT_TOKEN",
		"user__shared",
		"user__user_only",
	}
	if got := stringsJoin(names); got != stringsJoin(wantNames) {
		t.Fatalf("inspect tools = %v, want %v", names, wantNames)
	}
}

func startTestProxy(t *testing.T, ctx context.Context, servers []ScopedServer) (*mcpsdk.ClientSession, func()) {
	t.Helper()
	proxy := newProxyServer(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := proxy.connectChildren(ctx, servers); err != nil {
		t.Fatal(err)
	}
	session, closeSession := connectTestClient(t, ctx, proxy)
	return session, func() {
		closeSession()
		proxy.close()
	}
}

func connectTestClient(t *testing.T, ctx context.Context, proxy *proxyServer) (*mcpsdk.ClientSession, func()) {
	t.Helper()
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "bridge-test", Version: "0"}, proxy.serverOptions())
	if err := proxy.register(ctx, server); err != nil {
		t.Fatal(err)
	}
	serverCtx, cancelServer := context.WithCancel(ctx)
	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Run(serverCtx, serverTransport)
	}()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "bridge-test-client", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	return session, func() {
		_ = session.Close()
		cancelServer()
		select {
		case <-serverDone:
		case <-time.After(2 * time.Second):
			t.Fatal("test proxy server did not exit")
		}
	}
}

func fakeServer(name, scope, workDir string) ScopedServer {
	return ScopedServer{Name: name, Scope: scope, WorkDir: workDir, Config: fakeServerMCPConfig(name)}
}

func fakeServerMCPConfig(name string) MCPServerConfig {
	return MCPServerConfig{Command: os.Args[0], Env: map[string]string{fakeChildEnv: name}}
}

func fakeServerConfig(name string) map[string]any {
	return map[string]any{"command": os.Args[0], "env": map[string]string{fakeChildEnv: name}}
}

func callFakeTool(t *testing.T, ctx context.Context, session *mcpsdk.ClientSession, name string) fakeToolOutput {
	t.Helper()
	res, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	var out fakeToolOutput
	data, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode structured content: %v; raw=%s", err, data)
	}
	return out
}

func runFakeChildServer(name string) error {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fake-" + name, Version: "0"}, nil)
	for _, toolName := range []string{name + "_only", "shared", "env_EXPLICIT_TOKEN", "env_AMBIENT_SECRET_TOKEN", "env_CLAUDE_BRIDGE_PROJECT_ROOT"} {
		toolName := toolName
		mcpsdk.AddTool[map[string]any, fakeToolOutput](server, &mcpsdk.Tool{Name: toolName, Description: "fake " + toolName}, func(ctx context.Context, req *mcpsdk.CallToolRequest, in map[string]any) (*mcpsdk.CallToolResult, fakeToolOutput, error) {
			wd, err := os.Getwd()
			if err != nil {
				return nil, fakeToolOutput{}, err
			}
			var envValue string
			if key, ok := strings.CutPrefix(req.Params.Name, "env_"); ok {
				envValue = os.Getenv(key)
			}
			return nil, fakeToolOutput{Server: name, Tool: req.Params.Name, CWD: wd, Env: envValue}, nil
		})
	}
	return server.Run(context.Background(), &mcpsdk.StdioTransport{})
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func stringsJoin(values []string) string {
	data, _ := json.Marshal(values)
	return string(data)
}
