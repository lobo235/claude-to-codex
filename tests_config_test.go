package main

import (
	"os"
	"path/filepath"
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

func TestNameMapperPreservesUniqueAndPrefixesCollisions(t *testing.T) {
	mapper := newNameMapper([]string{"wiki_get", "status", "status"})
	if got := mapper.exposed("wiki", "wiki_get"); got != "wiki_get" {
		t.Fatalf("unique name = %q", got)
	}
	if got := mapper.exposed("project-tools", "status"); got != "project-tools__status" {
		t.Fatalf("collision name = %q", got)
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
