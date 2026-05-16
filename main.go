package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: %s serve|inspect|sync-commands|sync-skills|version", os.Args[0])
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	switch os.Args[1] {
	case "serve":
		if err := runServe(logger); err != nil {
			fatalf("%v", err)
		}
	case "inspect":
		if err := runInspect(os.Args[2:], logger); err != nil {
			fatalf("%v", err)
		}
	case "sync-commands":
		if err := runSyncCommands(os.Args[2:]); err != nil {
			fatalf("%v", err)
		}
	case "sync-skills":
		if err := runSyncSkills(os.Args[2:]); err != nil {
			fatalf("%v", err)
		}
	case "version", "--version", "-v":
		fmt.Printf("claude-to-codex version %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
	default:
		fatalf("unknown command %q", os.Args[1])
	}
}

func runServe(logger *slog.Logger) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	projectRoot := currentProjectRoot()
	servers, err := loadClaudeServers(home, projectRoot)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	proxy := newProxyServer(logger)
	if err := proxy.connectChildren(ctx, servers); err != nil {
		return err
	}
	defer proxy.close()

	srv := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "claude-bridge", Version: version}, proxy.serverOptions())
	if err := proxy.register(ctx, srv); err != nil {
		return err
	}
	logger.Info("starting Codex Claude MCP bridge", "project_root", projectRoot, "connected_children", len(proxy.children), "configured_children", len(servers))
	return srv.Run(context.Background(), &mcpsdk.StdioTransport{})
}

func runInspect(args []string, logger *slog.Logger) error {
	includeTools := false
	for _, arg := range args {
		switch arg {
		case "--tools":
			includeTools = true
		default:
			return fmt.Errorf("unknown inspect flag %q", arg)
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	projectRoot := currentProjectRoot()
	servers, err := loadClaudeServers(home, projectRoot)
	if err != nil {
		return err
	}
	type serverOut struct {
		Name    string `json:"name"`
		Scope   string `json:"scope"`
		Kind    string `json:"kind"`
		Command string `json:"command,omitempty"`
		URL     string `json:"url,omitempty"`
		WorkDir string `json:"workDir,omitempty"`
	}
	type toolOut struct {
		Name         string `json:"name"`
		OriginalName string `json:"originalName"`
		Server       string `json:"server"`
		Scope        string `json:"scope"`
	}
	type errorOut struct {
		Server    string `json:"server"`
		Scope     string `json:"scope"`
		Operation string `json:"operation"`
		Error     string `json:"error"`
	}
	out := struct {
		ProjectRoot string      `json:"projectRoot"`
		Servers     []serverOut `json:"servers"`
		Tools       []toolOut   `json:"tools,omitempty"`
		Errors      []errorOut  `json:"errors,omitempty"`
	}{ProjectRoot: projectRoot}
	for _, server := range servers {
		kind := "stdio"
		if server.Config.URL != "" || strings.EqualFold(server.Config.Type, "http") || strings.EqualFold(server.Config.Type, "streamable-http") {
			kind = "http"
		}
		out.Servers = append(out.Servers, serverOut{Name: server.Name, Scope: server.Scope, Kind: kind, Command: server.Config.Command, URL: server.Config.URL, WorkDir: server.WorkDir})
	}
	if includeTools {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		proxy := newProxyServer(logger)
		for _, failure := range proxy.connectChildrenBestEffort(ctx, servers) {
			out.Errors = append(out.Errors, errorOut{Server: failure.server.Name, Scope: failure.server.Scope, Operation: failure.operation, Error: failure.err.Error()})
		}
		defer proxy.close()
		tools, failures := proxy.inspectTools(ctx)
		for _, failure := range failures {
			out.Errors = append(out.Errors, errorOut{Server: failure.server.Name, Scope: failure.server.Scope, Operation: failure.operation, Error: failure.err.Error()})
		}
		for _, tool := range tools {
			out.Tools = append(out.Tools, toolOut{Name: tool.exposedName, OriginalName: tool.originalName, Server: tool.server.Name, Scope: tool.server.Scope})
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func currentProjectRoot() string {
	if projectRoot := os.Getenv("CLAUDE_BRIDGE_PROJECT_ROOT"); projectRoot != "" {
		return projectRoot
	}
	projectRoot, _ := findProjectRoot(mustGetwd())
	return projectRoot
}

func findProjectRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if exists(filepath.Join(dir, ".mcp.json")) || exists(filepath.Join(dir, "CLAUDE.md")) || exists(filepath.Join(dir, ".git")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start, nil
		}
		dir = parent
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "claude-to-codex: "+format+"\n", args...)
	os.Exit(1)
}
