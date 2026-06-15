package main

import (
	"strings"
	"testing"
)

func TestBuildChildEnvUsesRestrictedBaselineAndExplicitEnv(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("HOME", "/home/test")
	t.Setenv("GITHUB_TOKEN", "ambient")
	env := buildChildEnv(ScopedServer{
		Scope:  "user",
		Config: MCPServerConfig{Env: map[string]string{"EXPLICIT_TOKEN": "explicit"}},
	})
	joined := "\n" + strings.Join(env, "\n") + "\n"
	if !strings.Contains(joined, "\nPATH=/usr/bin\n") || !strings.Contains(joined, "\nHOME=/home/test\n") {
		t.Fatalf("baseline env missing: %#v", env)
	}
	if !strings.Contains(joined, "\nEXPLICIT_TOKEN=explicit\n") {
		t.Fatalf("explicit env missing: %#v", env)
	}
	if strings.Contains(joined, "GITHUB_TOKEN=ambient") {
		t.Fatalf("ambient secret leaked into restricted env: %#v", env)
	}
}

func TestBuildChildEnvAddsProjectRootForProjectServers(t *testing.T) {
	env := buildChildEnv(ScopedServer{Scope: "project", WorkDir: "/tmp/project"})
	if !containsEnv(env, "CLAUDE_BRIDGE_PROJECT_ROOT=/tmp/project") {
		t.Fatalf("project root env missing: %#v", env)
	}
}

func TestBuildChildEnvAddsProjectRootForLocalServers(t *testing.T) {
	env := buildChildEnv(ScopedServer{Scope: "local", WorkDir: "/tmp/project"})
	if !containsEnv(env, "CLAUDE_BRIDGE_PROJECT_ROOT=/tmp/project") {
		t.Fatalf("project root env missing: %#v", env)
	}
}

func TestBuildChildEnvPerServerInheritEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ambient")
	env := buildChildEnv(ScopedServer{Config: MCPServerConfig{InheritEnv: true}})
	if !containsEnv(env, "GITHUB_TOKEN=ambient") {
		t.Fatalf("ambient env missing with per-server inherit: %#v", env)
	}
}

func TestBuildChildEnvGlobalInheritEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ambient")
	t.Setenv(inheritEnvVar, "1")
	env := buildChildEnv(ScopedServer{})
	if !containsEnv(env, "GITHUB_TOKEN=ambient") {
		t.Fatalf("ambient env missing with global inherit: %#v", env)
	}
}

func TestRedactURL(t *testing.T) {
	got := redactURL("https://user:pass@example.com/token/pathsecret/mcp?token=abc&library=react&signature=def")
	if strings.Contains(got, "user") || strings.Contains(got, "pass") || strings.Contains(got, "abc") || strings.Contains(got, "def") || strings.Contains(got, "pathsecret") {
		t.Fatalf("url was not redacted: %s", got)
	}
	if !strings.Contains(got, "library=react") {
		t.Fatalf("non-sensitive query was unexpectedly redacted: %s", got)
	}
}

func TestRedactSensitive(t *testing.T) {
	input := "failed Authorization: Bearer abc123 token=xyz password: hunter2 https://example.com/mcp?api_key=supersecretvalue&ok=yes"
	got := redactSensitive(input)
	for _, secret := range []string{"abc123", "xyz", "hunter2", "supersecretvalue"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q still present in %q", secret, got)
		}
	}
	if !strings.Contains(got, "ok=yes") {
		t.Fatalf("non-sensitive URL query missing from %q", got)
	}
}

func TestRedactSensitiveRedactsRawTokenLikeValues(t *testing.T) {
	input := "failed Authorization: plain-secret github_pat_1234567890abcdefghijklmnopqrstuvwxyz eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.signature sk-proj-abcdefghijklmnopqrstuvwxyz123456 tkns_1234567890abcdefghijklmnopqrstuvwxyz"
	got := redactSensitive(input)
	for _, secret := range []string{
		"plain-secret",
		"github_pat_1234567890abcdefghijklmnopqrstuvwxyz",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.signature",
		"sk-proj-abcdefghijklmnopqrstuvwxyz123456",
		"tkns_1234567890abcdefghijklmnopqrstuvwxyz",
	} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q still present in %q", secret, got)
		}
	}
}

func TestRedactSensitiveKeepsMissingHeaderEnvDiagnosticReadable(t *testing.T) {
	input := `expand config: header "Authorization" missing env var REMOTE_TOOLS_TOKEN`
	if got := redactSensitive(input); got != input {
		t.Fatalf("diagnostic was over-redacted: %q", got)
	}
}

func containsEnv(env []string, want string) bool {
	for _, entry := range env {
		if entry == want {
			return true
		}
	}
	return false
}
