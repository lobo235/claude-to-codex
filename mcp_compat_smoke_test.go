package main

import (
	"encoding/json"
	"os/exec"
	"testing"
)

func TestMCPCompatSmokeDefaultConfigPinsUVXFixtures(t *testing.T) {
	cmd := exec.Command("bash", "scripts/mcp-compat-smoke", "--print-default-config")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("print default config: %v\n%s", err, out)
	}

	var config struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(out, &config); err != nil {
		t.Fatalf("parse default config: %v\n%s", err, out)
	}

	wantPackages := map[string]string{
		"fetch":  "mcp-server-fetch==2026.6.4",
		"time":   "mcp-server-time==2026.6.4",
		"git":    "mcp-server-git==2026.6.4",
		"sqlite": "mcp-server-sqlite==2025.4.25",
	}
	for name, wantPackage := range wantPackages {
		server, ok := config.MCPServers[name]
		if !ok {
			t.Fatalf("default config missing %q server; got %#v", name, config.MCPServers)
		}
		if server.Command != "uvx" {
			t.Fatalf("%s command = %q, want uvx", name, server.Command)
		}
		if len(server.Args) == 0 || server.Args[0] != wantPackage {
			t.Fatalf("%s package = %#v, want first arg %q", name, server.Args, wantPackage)
		}
	}
}
