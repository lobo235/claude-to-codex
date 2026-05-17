package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncClaudeAgentCreatesUserCodexAgent(t *testing.T) {
	t.Setenv("CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER", "1")
	home := t.TempDir()
	sourceDir := filepath.Join(home, ".claude", "agents")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(sourceDir, "security-reviewer.md")
	if err := os.WriteFile(sourcePath, []byte(`---
name: security-reviewer
description: Reviews code changes for concrete security regressions.
tools: Read, Grep, Bash
color: red
---

Review auth, secrets, and input handling.
`), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := syncClaudeAgents(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "created" {
		t.Fatalf("results = %#v, want one created agent", results)
	}
	data, err := os.ReadFile(filepath.Join(home, ".codex", "agents", "security-reviewer.toml"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, want := range []string{
		generatedAgentMarker,
		`name = "security_reviewer"`,
		`description = "Reviews code changes for concrete security regressions."`,
		"# source-sha256: ",
		"# source-claude-name: security-reviewer",
		"# claude-frontmatter.color: red",
		"Original Claude tool constraint: Read, Grep, Bash.",
		"Review auth, secrets, and input handling.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("generated agent missing %q:\n%s", want, body)
		}
	}
}

func TestSyncClaudeAgentUpdatesGeneratedAgentWhenSourceChanges(t *testing.T) {
	t.Setenv("CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER", "1")
	home := t.TempDir()
	sourceDir := filepath.Join(home, ".claude", "agents")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(sourceDir, "reviewer.md")
	if err := os.WriteFile(sourcePath, []byte("Initial body.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := syncClaudeAgents(home); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte("Updated body.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := syncClaudeAgents(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "updated" {
		t.Fatalf("results = %#v, want updated", results)
	}
	data, err := os.ReadFile(filepath.Join(home, ".codex", "agents", "reviewer.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Updated body.") {
		t.Fatalf("generated agent was not updated:\n%s", data)
	}
}

func TestSyncClaudeAgentDoesNotOverwriteHandWrittenAgent(t *testing.T) {
	t.Setenv("CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER", "1")
	home := t.TempDir()
	sourceDir := filepath.Join(home, ".claude", "agents")
	targetDir := filepath.Join(home, ".codex", "agents")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "reviewer.md"), []byte("Claude body.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	original := `name = "reviewer"
description = "Hand-written agent."
developer_instructions = "Keep this."
`
	targetPath := filepath.Join(targetDir, "reviewer.toml")
	if err := os.WriteFile(targetPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := syncClaudeAgents(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "skipped" {
		t.Fatalf("results = %#v, want skipped", results)
	}
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("hand-written agent was overwritten:\n%s", data)
	}
}

func TestSyncClaudeAgentSkipsSymlinkedTarget(t *testing.T) {
	t.Setenv("CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER", "1")
	home := t.TempDir()
	sourceDir := filepath.Join(home, ".claude", "agents")
	targetDir := filepath.Join(home, ".codex", "agents")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "reviewer.md"), []byte("Claude body.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outsidePath := filepath.Join(home, "outside.toml")
	original := "# " + generatedAgentMarker + "\n# source-sha256: old\n# codex-name: reviewer\n"
	if err := os.WriteFile(outsidePath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsidePath, filepath.Join(targetDir, "reviewer.toml")); err != nil {
		t.Fatal(err)
	}

	results, err := syncClaudeAgents(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "skipped" || !strings.Contains(results[0].Reason, "symlink") {
		t.Fatalf("results = %#v, want symlink skip", results)
	}
	data, err := os.ReadFile(outsidePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("symlink target was overwritten:\n%s", data)
	}
}

func TestSyncClaudeAgentsRemovesStaleGeneratedAgent(t *testing.T) {
	t.Setenv("CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER", "1")
	home := t.TempDir()
	sourceDir := filepath.Join(home, ".claude", "agents")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(sourceDir, "reviewer.md")
	if err := os.WriteFile(sourcePath, []byte("Claude body.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := syncClaudeAgents(home); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(sourcePath); err != nil {
		t.Fatal(err)
	}

	results, err := syncClaudeAgents(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "deleted" {
		t.Fatalf("results = %#v, want deleted", results)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "agents", "reviewer.toml")); !os.IsNotExist(err) {
		t.Fatalf("stale generated agent still exists or stat failed: %v", err)
	}
}

func TestSyncClaudeAgentsProjectScope(t *testing.T) {
	t.Setenv("CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER", "1")
	home := t.TempDir()
	project := t.TempDir()
	sourceDir := filepath.Join(project, ".claude", "agents")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "project-reviewer.md"), []byte("Project body.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := syncClaudeAgentsWithProgress(home, project, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Scope != "project" || results[0].Status != "created" {
		t.Fatalf("results = %#v, want project created", results)
	}
	if _, err := os.Stat(filepath.Join(project, ".codex", "agents", "project-reviewer.toml")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "agents", "project-reviewer.toml")); !os.IsNotExist(err) {
		t.Fatalf("project agent was written to user scope: %v", err)
	}
}

func TestSyncClaudeAgentsProjectScopeUsesRelativePathsAndRedactsUnknownFrontmatter(t *testing.T) {
	t.Setenv("CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER", "1")
	home := t.TempDir()
	project := t.TempDir()
	sourceDir := filepath.Join(project, ".claude", "agents")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "project-reviewer.md"), []byte(`---
description: Reviews project-specific changes.
api_token: super-secret-value
note: token=inline-secret ok=yes
---

Project body.
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := syncClaudeAgentsWithProgress(home, project, "", nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(project, ".codex", "agents", "project-reviewer.toml"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if strings.Contains(body, project) {
		t.Fatalf("project absolute path leaked:\n%s", body)
	}
	if !strings.Contains(body, ".claude/agents/project-reviewer.md") {
		t.Fatalf("relative source path missing:\n%s", body)
	}
	for _, secret := range []string{"super-secret-value", "inline-secret"} {
		if strings.Contains(body, secret) {
			t.Fatalf("frontmatter secret leaked:\n%s", body)
		}
	}
}

func TestSyncClaudeAgentsSkipsMalformedAgent(t *testing.T) {
	t.Setenv("CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER", "1")
	home := t.TempDir()
	sourceDir := filepath.Join(home, ".claude", "agents")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "broken.md"), []byte("---\ndescription: nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := syncClaudeAgents(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "skipped" || !strings.Contains(results[0].Reason, "unterminated") {
		t.Fatalf("results = %#v, want malformed skip", results)
	}
}

func TestSyncClaudeAgentsQuietProgressOnlyForGeneratedDescriptions(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	sourceDir := filepath.Join(home, ".claude", "agents")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "needs-description.md"), []byte("Review security issues.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var progress bytes.Buffer
	if _, err := syncClaudeAgentsWithProgress(home, "", "", &progress); err != nil {
		t.Fatal(err)
	}
	out := progress.String()
	if !strings.Contains(out, "generating description for agent needs-description [1/1 agents]") {
		t.Fatalf("progress missing start:\n%s", out)
	}
	if !strings.Contains(out, "generated description for agent needs-description [1/1 agents done]") {
		t.Fatalf("progress missing finish:\n%s", out)
	}

	progress.Reset()
	if _, err := syncClaudeAgentsWithProgress(home, "", "", &progress); err != nil {
		t.Fatal(err)
	}
	if progress.String() != "" {
		t.Fatalf("unchanged sync emitted progress:\n%s", progress.String())
	}
}
