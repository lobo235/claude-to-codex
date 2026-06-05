package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
)

const bridgeProjectRootEnv = "CLAUDE_BRIDGE_PROJECT_ROOT"

func runBridgeEnvVars(args []string) error {
	projectRoot := currentProjectRoot()
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--project":
			i++
			if i >= len(args) {
				return fmt.Errorf("--project requires a value")
			}
			projectRoot = args[i]
		case strings.HasPrefix(arg, "--project="):
			projectRoot = strings.TrimPrefix(arg, "--project=")
		default:
			return fmt.Errorf("unknown bridge-env-vars argument %q", arg)
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	servers, err := loadClaudeServers(home, projectRoot)
	if err != nil {
		return err
	}
	envVars, err := bridgeEnvVarsForServers(servers)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(envVars)
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}

func bridgeEnvVarsForServers(servers []ScopedServer) ([]string, error) {
	names := map[string]bool{bridgeProjectRootEnv: true}
	for _, server := range servers {
		if err := collectMCPServerConfigEnvRefs(server.Config, names); err != nil {
			return nil, fmt.Errorf("%s-scope MCP server %q: %w", server.Scope, server.Name, err)
		}
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func collectMCPServerConfigEnvRefs(cfg MCPServerConfig, names map[string]bool) error {
	if err := collectEnvRefs(cfg.Type, names); err != nil {
		return fmt.Errorf("type: %w", err)
	}
	if err := collectEnvRefs(cfg.URL, names); err != nil {
		return fmt.Errorf("url: %w", err)
	}
	if err := collectEnvRefs(cfg.Command, names); err != nil {
		return fmt.Errorf("command: %w", err)
	}
	for i, arg := range cfg.Args {
		if err := collectEnvRefs(arg, names); err != nil {
			return fmt.Errorf("args[%d]: %w", i, err)
		}
	}
	for _, key := range sortedKeys(cfg.Env) {
		if err := collectEnvRefs(cfg.Env[key], names); err != nil {
			return fmt.Errorf("env.%s: %w", key, err)
		}
	}
	for _, key := range sortedKeys(cfg.Headers) {
		if err := collectEnvRefs(cfg.Headers[key], names); err != nil {
			return fmt.Errorf("header %q: %w", key, err)
		}
	}
	return nil
}

func collectEnvRefs(raw string, names map[string]bool) error {
	for i := 0; i < len(raw); {
		if raw[i] != '$' {
			i++
			continue
		}
		if i+1 >= len(raw) {
			i++
			continue
		}
		if raw[i+1] == '{' {
			end := strings.IndexByte(raw[i+2:], '}')
			if end < 0 {
				return fmt.Errorf("malformed env reference")
			}
			name := raw[i+2 : i+2+end]
			if name == "" {
				return fmt.Errorf("empty env reference")
			}
			names[name] = true
			i += end + 3
			continue
		}
		if !isEnvNameStart(rune(raw[i+1])) {
			i++
			continue
		}
		j := i + 2
		for j < len(raw) && isEnvNamePart(rune(raw[j])) {
			j++
		}
		names[raw[i+1:j]] = true
		i = j
	}
	return nil
}

func isEnvNameStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isEnvNamePart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
