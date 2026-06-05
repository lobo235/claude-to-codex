package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

func expandMCPServerConfig(cfg MCPServerConfig) (MCPServerConfig, error) {
	var err error
	if cfg.Type, err = expandEnvString(cfg.Type); err != nil {
		return MCPServerConfig{}, fmt.Errorf("type: %w", err)
	}
	if cfg.URL, err = expandEnvString(cfg.URL); err != nil {
		return MCPServerConfig{}, fmt.Errorf("url: %w", err)
	}
	if cfg.Command, err = expandEnvString(cfg.Command); err != nil {
		return MCPServerConfig{}, fmt.Errorf("command: %w", err)
	}
	for i, arg := range cfg.Args {
		cfg.Args[i], err = expandEnvString(arg)
		if err != nil {
			return MCPServerConfig{}, fmt.Errorf("args[%d]: %w", i, err)
		}
	}
	if cfg.Env != nil {
		env := make(map[string]string, len(cfg.Env))
		for _, key := range sortedKeys(cfg.Env) {
			value, err := expandEnvString(cfg.Env[key])
			if err != nil {
				return MCPServerConfig{}, fmt.Errorf("env.%s: %w", key, err)
			}
			env[key] = value
		}
		cfg.Env = env
	}
	if cfg.Headers != nil {
		headers := make(map[string]string, len(cfg.Headers))
		for _, key := range sortedKeys(cfg.Headers) {
			value, err := expandEnvString(cfg.Headers[key])
			if err != nil {
				return MCPServerConfig{}, fmt.Errorf("header %q %w", key, err)
			}
			headers[key] = value
		}
		cfg.Headers = headers
	}
	return cfg, nil
}

func expandEnvString(raw string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(raw); {
		if raw[i] != '$' {
			b.WriteByte(raw[i])
			i++
			continue
		}
		if i+1 >= len(raw) {
			b.WriteByte(raw[i])
			i++
			continue
		}
		if raw[i+1] == '{' {
			end := strings.IndexByte(raw[i+2:], '}')
			if end < 0 {
				return "", fmt.Errorf("malformed env reference")
			}
			name := raw[i+2 : i+2+end]
			if name == "" {
				return "", fmt.Errorf("empty env reference")
			}
			value, ok := os.LookupEnv(name)
			if !ok {
				return "", fmt.Errorf("missing env var %s", name)
			}
			b.WriteString(value)
			i += end + 3
			continue
		}
		if !isEnvNameStart(rune(raw[i+1])) {
			b.WriteByte(raw[i])
			i++
			continue
		}
		j := i + 2
		for j < len(raw) && isEnvNamePart(rune(raw[j])) {
			j++
		}
		name := raw[i+1 : j]
		value, ok := os.LookupEnv(name)
		if !ok {
			return "", fmt.Errorf("missing env var %s", name)
		}
		b.WriteString(value)
		i = j
	}
	return b.String(), nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
