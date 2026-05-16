package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncClaudeCommandCreatesGeneratedSkillWrapper(t *testing.T) {
	home := t.TempDir()
	commandDir := filepath.Join(home, ".claude", "commands")
	if err := os.MkdirAll(commandDir, 0o755); err != nil {
		t.Fatal(err)
	}
	commandPath := filepath.Join(commandDir, "wiki-onboard.md")
	if err := os.WriteFile(commandPath, []byte(`---
description: Onboard the current project to the wiki
argument-hint: "[project-slug]"
---

Command body.
`), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := syncClaudeCommands(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "created" {
		t.Fatalf("results = %#v, want one created command", results)
	}
	data, err := os.ReadFile(filepath.Join(home, ".codex", "skills", "wiki-onboard", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	if !strings.Contains(body, generatedSkillMarker) {
		t.Fatalf("generated skill missing marker:\n%s", body)
	}
	if !strings.Contains(body, commandPath) {
		t.Fatalf("generated skill missing source path:\n%s", body)
	}
}

func TestSyncClaudeCommandDoesNotOverwriteHandWrittenSkill(t *testing.T) {
	home := t.TempDir()
	commandDir := filepath.Join(home, ".claude", "commands")
	skillDir := filepath.Join(home, ".codex", "skills", "wiki-onboard")
	if err := os.MkdirAll(commandDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commandDir, "wiki-onboard.md"), []byte("---\ndescription: test\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	original := []byte("hand written")
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), original, 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := syncClaudeCommands(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "skipped" {
		t.Fatalf("results = %#v, want skipped", results)
	}
	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(original) {
		t.Fatalf("hand-written skill was overwritten: %q", data)
	}
}

func TestSyncClaudeSkillCreatesSymlink(t *testing.T) {
	home := t.TempDir()
	claudeSkill := filepath.Join(home, ".claude", "skills", "example-mcp-skill")
	if err := os.MkdirAll(claudeSkill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeSkill, "SKILL.md"), []byte("---\nname: example-mcp-skill\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := syncClaudeSkills(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "created" {
		t.Fatalf("results = %#v, want one created skill", results)
	}
	dest := filepath.Join(home, ".codex", "skills", "example-mcp-skill")
	target, err := os.Readlink(dest)
	if err != nil {
		t.Fatal(err)
	}
	if target != claudeSkill {
		t.Fatalf("symlink target = %q, want %q", target, claudeSkill)
	}
}

func TestSyncClaudeSkillDoesNotOverwriteExistingCodexSkill(t *testing.T) {
	home := t.TempDir()
	claudeSkill := filepath.Join(home, ".claude", "skills", "example-mcp-skill")
	codexSkill := filepath.Join(home, ".codex", "skills", "example-mcp-skill")
	if err := os.MkdirAll(claudeSkill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(codexSkill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeSkill, "SKILL.md"), []byte("---\nname: example-mcp-skill\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexSkill, "SKILL.md"), []byte("codex native"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := syncClaudeSkills(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != "skipped" {
		t.Fatalf("results = %#v, want skipped", results)
	}
}
