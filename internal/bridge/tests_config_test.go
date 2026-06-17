package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadClaudeServersKeepsUserAndProjectScope(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"mcpServers":{"wiki":{"type":"http","url":"https://example.invalid/mcp"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".mcp.json"), []byte(`{"mcpServers":{"project-tools":{"command":"project-mcp","args":["--config",".project-mcp.yaml"]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	servers, err := loadClaudeServers(home, project)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(servers))
	}
	if servers[0].Name != "wiki" || servers[0].Scope != "user" {
		t.Fatalf("first server = %#v", servers[0])
	}
	if servers[1].Name != "project-tools" || servers[1].Scope != "project" {
		t.Fatalf("second server = %#v", servers[1])
	}
}

func TestLoadClaudeServersIncludesClaudeLocalScope(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "repo")
	for _, dir := range []string{home, project} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	config := `{
		"mcpServers": {
			"user-tools": {"command":"user-mcp"}
		},
		"projects": {
			"` + project + `": {
				"mcpServers": {
					"local-tools": {"command":"local-mcp"}
				}
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	servers, err := loadClaudeServers(home, project)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(servers))
	}
	if servers[0].Name != "user-tools" || servers[0].Scope != "user" || servers[0].WorkDir != "" {
		t.Fatalf("user server = %#v", servers[0])
	}
	if servers[1].Name != "local-tools" || servers[1].Scope != "local" || servers[1].WorkDir != project {
		t.Fatalf("local server = %#v", servers[1])
	}
}

func TestLoadClaudeServersLetsLocalScopeOverrideUserScope(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "repo")
	for _, dir := range []string{home, project} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	config := `{
		"mcpServers": {
			"wiki": {"type":"http","url":"https://user.example.invalid/mcp","headers":{"Authorization":"Bearer user-token"}}
		},
		"projects": {
			"` + project + `": {
				"mcpServers": {
					"wiki": {"type":"http","url":"https://project.example.invalid/mcp","headers":{"Authorization":"Bearer ${WIKI_TOKEN}"}}
				}
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	servers, err := loadClaudeServers(home, project)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(servers))
	}
	server := servers[0]
	if server.Name != "wiki" || server.Scope != "local" || server.WorkDir != project {
		t.Fatalf("server = %#v, want local wiki override", server)
	}
	if got := server.Config.URL; got != "https://project.example.invalid/mcp" {
		t.Fatalf("url = %q, want project override", got)
	}
	if got := server.Config.Headers["Authorization"]; got != "Bearer ${WIKI_TOKEN}" {
		t.Fatalf("authorization header = %q", got)
	}
}

func TestProjectClaudeConfigPrefersExactProjectPath(t *testing.T) {
	project := filepath.Join(t.TempDir(), "repo")
	cfg := ClaudeConfig{Projects: map[string]ProjectClaudeConfig{
		project: {
			MCPServers: map[string]MCPServerConfig{"exact": {Command: "exact-mcp"}},
		},
		project + string(os.PathSeparator) + ".": {
			MCPServers: map[string]MCPServerConfig{"cleaned": {Command: "cleaned-mcp"}},
		},
	}}

	projectConfig, ok, err := projectClaudeConfig(cfg, project)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("project config not found")
	}
	if _, ok := projectConfig.MCPServers["exact"]; !ok {
		t.Fatalf("project config = %#v, want exact path entry", projectConfig.MCPServers)
	}
}

func TestProjectClaudeConfigRejectsAmbiguousCleanedPaths(t *testing.T) {
	project := filepath.Join(t.TempDir(), "repo")
	cfg := ClaudeConfig{Projects: map[string]ProjectClaudeConfig{
		project + string(os.PathSeparator) + ".": {
			MCPServers: map[string]MCPServerConfig{"first": {Command: "first-mcp"}},
		},
		project + string(os.PathSeparator) + ".." + string(os.PathSeparator) + filepath.Base(project): {
			MCPServers: map[string]MCPServerConfig{"second": {Command: "second-mcp"}},
		},
	}}

	_, ok, err := projectClaudeConfig(cfg, project)
	if err == nil {
		t.Fatal("projectClaudeConfig accepted ambiguous cleaned project paths")
	}
	if ok {
		t.Fatal("projectClaudeConfig reported a match despite ambiguity")
	}
	if !strings.Contains(err.Error(), "ambiguous local Claude MCP project entries") {
		t.Fatalf("error = %q", err)
	}
}

func TestLoadClaudeServersLetsProjectMCPOverrideClaudeLocalScope(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "repo")
	for _, dir := range []string{home, project} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	config := `{
		"projects": {
			"` + project + `": {
				"mcpServers": {
					"tools": {"command":"local-mcp"}
				}
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".mcp.json"), []byte(`{"mcpServers":{"tools":{"command":"project-mcp"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	servers, err := loadClaudeServers(home, project)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(servers))
	}
	if servers[0].Name != "tools" || servers[0].Scope != "project" || servers[0].Config.Command != "project-mcp" {
		t.Fatalf("server = %#v, want project .mcp.json override", servers[0])
	}
}

func TestLoadClaudeServersOnlyLoadsRequestedProjectRoot(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	projectA := filepath.Join(tmp, "project-a")
	projectB := filepath.Join(tmp, "project-b")
	for _, dir := range []string{home, projectA, projectB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(projectA, ".mcp.json"), []byte(`{"mcpServers":{"project-a-tools":{"command":"project-a-mcp"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectB, ".mcp.json"), []byte(`{"mcpServers":{"project-b-tools":{"command":"project-b-mcp"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	servers, err := loadClaudeServers(home, projectA)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(servers))
	}
	if servers[0].Name != "project-a-tools" || servers[0].Scope != "project" || servers[0].WorkDir != projectA {
		t.Fatalf("server = %#v, want only project-a server scoped to project-a", servers[0])
	}

	servers, err = loadClaudeServers(home, projectB)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(servers))
	}
	if servers[0].Name != "project-b-tools" || servers[0].Scope != "project" || servers[0].WorkDir != projectB {
		t.Fatalf("server = %#v, want only project-b server scoped to project-b", servers[0])
	}
}

func TestNameMapperPrefixesEveryChildName(t *testing.T) {
	mapper := newNameMapper([]string{"wiki_get", "status", "status"})
	if got := mapper.exposed("wiki", "wiki_get"); got != "wiki__wiki_get" {
		t.Fatalf("prefixed unique name = %q", got)
	}
	if got := mapper.exposed("project-tools", "status"); got != "project-tools__status" {
		t.Fatalf("prefixed repeated name = %q", got)
	}
}

func TestReadMCPServersParsesBridgeInheritEnvExtension(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".mcp.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"legacy":{"command":"legacy-mcp","x-claude-bridge-inherit-env":true}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	servers, err := readMCPServers(path)
	if err != nil {
		t.Fatal(err)
	}
	if !servers["legacy"].InheritEnv {
		t.Fatalf("inherit env extension was not parsed: %#v", servers["legacy"])
	}
}

func TestReadMCPServersParsesSSEServerWithHeaders(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".mcp.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"remote-tools":{"type":"sse","url":"https://example.invalid/sse","headers":{"Authorization":"Bearer ${REMOTE_TOOLS_TOKEN}"}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	servers, err := readMCPServers(path)
	if err != nil {
		t.Fatal(err)
	}
	server := servers["remote-tools"]
	if server.Type != "sse" || server.URL != "https://example.invalid/sse" {
		t.Fatalf("sse server = %#v", server)
	}
	if got := server.Headers["Authorization"]; got != "Bearer ${REMOTE_TOOLS_TOKEN}" {
		t.Fatalf("authorization header = %q", got)
	}
}

func TestExpandMCPServerConfigExpandsEnvReferences(t *testing.T) {
	t.Setenv("MCP_HOST", "example.invalid")
	t.Setenv("REMOTE_TOOLS_TOKEN", "secret-token")

	cfg, err := expandMCPServerConfig(MCPServerConfig{
		Type: "sse",
		URL:  "https://${MCP_HOST}/sse",
		Headers: map[string]string{
			"Authorization": "Bearer $REMOTE_TOOLS_TOKEN",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.URL; got != "https://example.invalid/sse" {
		t.Fatalf("expanded URL = %q", got)
	}
	if got := cfg.Headers["Authorization"]; got != "Bearer secret-token" {
		t.Fatalf("expanded Authorization = %q", got)
	}
}

func TestExpandMCPServerConfigFailsClosedOnMissingEnvReference(t *testing.T) {
	_, err := expandMCPServerConfig(MCPServerConfig{
		Type: "sse",
		URL:  "https://example.invalid/sse",
		Headers: map[string]string{
			"Authorization": "Bearer ${MISSING_REMOTE_TOOLS_TOKEN}",
		},
	})
	if err == nil {
		t.Fatal("expandMCPServerConfig succeeded with a missing env reference")
	}
	if !strings.Contains(err.Error(), "MISSING_REMOTE_TOOLS_TOKEN") {
		t.Fatalf("error = %q, want missing env var name", err)
	}
}

func TestLoadClaudeServersDoesNotExpandOrFailOnMissingEnvReference(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	project := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".mcp.json"), []byte(`{"mcpServers":{"remote-tools":{"type":"sse","url":"https://example.invalid/sse","headers":{"Authorization":"Bearer ${MISSING_REMOTE_TOOLS_TOKEN}"}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	servers, err := loadClaudeServers(home, project)
	if err != nil {
		t.Fatalf("loadClaudeServers should preserve raw config and defer expansion to child connect: %v", err)
	}
	if len(servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(servers))
	}
	if got := servers[0].Config.Headers["Authorization"]; got != "Bearer ${MISSING_REMOTE_TOOLS_TOKEN}" {
		t.Fatalf("raw Authorization header = %q", got)
	}
}

func TestCurrentProjectRootFallsBackToWorkingDirectory(t *testing.T) {
	t.Setenv("CLAUDE_BRIDGE_PROJECT_ROOT", "")
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	nested := filepath.Join(repo, "subdir")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".mcp.json"), []byte(`{"mcpServers":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	if got := currentProjectRoot(); got != repo {
		t.Fatalf("project root = %q, want %q", got, repo)
	}
}

func TestCurrentProjectRootPrefersEnv(t *testing.T) {
	tmp := t.TempDir()
	envRoot := filepath.Join(tmp, "env-root")
	if err := os.MkdirAll(envRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_BRIDGE_PROJECT_ROOT", envRoot)
	if got := currentProjectRoot(); got != envRoot {
		t.Fatalf("project root = %q, want %q", got, envRoot)
	}
}
