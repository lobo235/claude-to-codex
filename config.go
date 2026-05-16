package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type ClaudeConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

type ProjectClaudeConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

type MCPServerConfig struct {
	Type       string            `json:"type,omitempty"`
	URL        string            `json:"url,omitempty"`
	Command    string            `json:"command,omitempty"`
	Args       []string          `json:"args,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	InheritEnv bool              `json:"x-claude-bridge-inherit-env,omitempty"`
}

type ScopedServer struct {
	Name    string
	Scope   string
	WorkDir string
	Config  MCPServerConfig
}

func loadClaudeServers(homeDir, projectRoot string) ([]ScopedServer, error) {
	var servers []ScopedServer

	userPath := filepath.Join(homeDir, ".claude.json")
	userServers, err := readMCPServers(userPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read user Claude MCP config: %w", err)
	}
	for _, name := range sortedKeys(userServers) {
		servers = append(servers, ScopedServer{Name: name, Scope: "user", Config: userServers[name]})
	}

	if projectRoot != "" {
		projectPath := filepath.Join(projectRoot, ".mcp.json")
		projectServers, err := readMCPServers(projectPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read project Claude MCP config: %w", err)
		}
		for _, name := range sortedKeys(projectServers) {
			servers = append(servers, ScopedServer{Name: name, Scope: "project", WorkDir: projectRoot, Config: projectServers[name]})
		}
	}

	return servers, nil
}

func readMCPServers(path string) (map[string]MCPServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg ClaudeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if cfg.MCPServers == nil {
		cfg.MCPServers = map[string]MCPServerConfig{}
	}
	return cfg.MCPServers, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
