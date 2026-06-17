package bridge

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunSyncArtifactsCachesAndMaterializesUserSkillsCommands(t *testing.T) {
	t.Setenv("CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER", "1")
	home := t.TempDir()
	cacheDir := t.TempDir()
	codexHome := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(artifactCacheEnv, cacheDir)
	t.Setenv("CODEX_HOME", codexHome)

	writeTestFile(t, filepath.Join(home, ".claude", "skills", "example-skill", "SKILL.md"), "---\nname: example-skill\n---\n\nUse the example skill.\n")
	writeTestFile(t, filepath.Join(home, ".claude", "commands", "wiki-onboard.md"), "---\ndescription: Onboard the wiki\n---\n\nCommand body.\n")

	if err := runSyncArtifacts([]string{"--quiet"}); err != nil {
		t.Fatal(err)
	}

	skill := readTestFile(t, filepath.Join(codexHome, "skills", "example-skill", "SKILL.md"))
	if !strings.Contains(skill, generatedClaudeSkillMarker) {
		t.Fatalf("materialized skill missing marker:\n%s", skill)
	}
	command := readTestFile(t, filepath.Join(codexHome, "skills", "wiki-onboard", "SKILL.md"))
	if !strings.Contains(command, generatedCommandSkillMarker) {
		t.Fatalf("materialized command missing marker:\n%s", command)
	}
	cacheSkill := readTestFile(t, filepath.Join(automationCacheRoot(cacheDir), "skills", "example-skill", "SKILL.md"))
	if !strings.Contains(cacheSkill, generatedClaudeSkillMarker) {
		t.Fatalf("cached skill missing marker:\n%s", cacheSkill)
	}
}

func TestSyncArtifactsMaterializesUserAgents(t *testing.T) {
	t.Setenv("CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER", "1")
	home := t.TempDir()
	cacheDir := t.TempDir()
	codexHome := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".claude", "agents", "security-reviewer.md"), `---
description: Reviews code changes for concrete security regressions.
---

Review auth and input handling.
`)

	if _, err := syncAndMaterializeArtifacts(home, "", cacheDir, codexHome, nil); err != nil {
		t.Fatal(err)
	}
	agent := readTestFile(t, filepath.Join(codexHome, "agents", "security-reviewer.toml"))
	if !strings.Contains(agent, generatedAgentMarker) || !strings.Contains(agent, `name = "security_reviewer"`) {
		t.Fatalf("materialized agent missing expected content:\n%s", agent)
	}
}

func TestSyncArtifactsProjectAgentsDoNotWriteProjectCodex(t *testing.T) {
	t.Setenv("CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER", "1")
	home := t.TempDir()
	project := t.TempDir()
	cacheDir := t.TempDir()
	codexHome := t.TempDir()
	writeTestFile(t, filepath.Join(project, ".claude", "agents", "project-reviewer.md"), "Review project-specific changes.\n")

	if _, err := syncAndMaterializeArtifacts(home, project, cacheDir, codexHome, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(project, ".codex")); !os.IsNotExist(err) {
		t.Fatalf("automation mode wrote project .codex or stat failed: %v", err)
	}
	agent := readTestFile(t, filepath.Join(codexHome, "agents", "project-reviewer.toml"))
	if !strings.Contains(agent, generatedAgentMarker) || !strings.Contains(agent, ".claude/agents/project-reviewer.md") {
		t.Fatalf("materialized project agent missing expected content:\n%s", agent)
	}
}

func TestSyncArtifactsLeavesUnchangedCacheFilesAlone(t *testing.T) {
	t.Setenv("CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER", "1")
	home := t.TempDir()
	cacheDir := t.TempDir()
	codexHome := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".claude", "skills", "example-skill", "SKILL.md"), "Use the example skill.\n")

	if _, err := syncAndMaterializeArtifacts(home, "", cacheDir, codexHome, nil); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(automationCacheRoot(cacheDir), "skills", "example-skill", "SKILL.md")
	before, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if _, err := syncAndMaterializeArtifacts(home, "", cacheDir, codexHome, nil); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("unchanged cache file was rewritten: before=%s after=%s", before.ModTime(), after.ModTime())
	}
}

func TestSyncArtifactsCacheContainsNoRuntimeState(t *testing.T) {
	t.Setenv("CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER", "1")
	home := t.TempDir()
	cacheDir := t.TempDir()
	codexHome := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".claude", "skills", "example-skill", "SKILL.md"), "Use the example skill.\n")
	writeTestFile(t, filepath.Join(home, ".codex", "auth.json"), `{"token":"secret"}`)
	writeTestFile(t, filepath.Join(home, ".codex", "config.toml"), `model = "gpt-5.5"`)
	writeTestFile(t, filepath.Join(home, ".codex", "sessions", "session.jsonl"), `{"event":"secret"}`)
	writeTestFile(t, filepath.Join(home, ".codex", "logs", "codex.log"), "secret\n")

	if _, err := syncAndMaterializeArtifacts(home, "", cacheDir, codexHome, nil); err != nil {
		t.Fatal(err)
	}
	forbiddenFiles := map[string]bool{
		"auth.json":   true,
		"config.toml": true,
	}
	forbiddenDirs := map[string]bool{
		"sessions": true,
		"logs":     true,
		"log":      true,
	}
	if err := filepath.WalkDir(cacheDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if forbiddenFiles[entry.Name()] {
			t.Fatalf("runtime file copied into artifact cache: %s", path)
		}
		if entry.IsDir() && forbiddenDirs[entry.Name()] {
			t.Fatalf("runtime directory copied into artifact cache: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
