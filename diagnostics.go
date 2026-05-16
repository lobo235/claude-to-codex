package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runDoctor(args []string, logger *slog.Logger) error {
	if len(args) != 0 {
		return fmt.Errorf("unexpected doctor arguments: %s", strings.Join(args, " "))
	}
	report, err := collectDiagnosticReport(logger, true)
	if err != nil {
		return err
	}
	printDiagnosticReport("cwc doctor", report)
	return nil
}

func runStatus(args []string, logger *slog.Logger) error {
	if len(args) != 0 {
		return fmt.Errorf("unexpected status arguments: %s", strings.Join(args, " "))
	}
	report, err := collectDiagnosticReport(logger, false)
	if err != nil {
		return err
	}
	fmt.Printf("cwc: %s\n", installedStatus(report.CWCPath))
	fmt.Printf("claude-to-codex: %s\n", installedStatus(report.CLIPath))
	fmt.Printf("codex: %s\n", codexStatus(report))
	fmt.Printf("claude-bridge: %s\n", bridgeStatus(report))
	fmt.Printf("Claude MCP servers: %d found\n", len(report.Servers))
	fmt.Printf("Project root: %s\n", report.ProjectRoot)
	return nil
}

func runSmokeTest(args []string, logger *slog.Logger) error {
	if len(args) != 0 {
		return fmt.Errorf("unexpected smoke-test arguments: %s", strings.Join(args, " "))
	}
	report, err := collectDiagnosticReport(logger, true)
	if err != nil {
		return err
	}
	if len(report.Servers) == 0 {
		fmt.Println("smoke-test: no Claude MCP servers found; claude-bridge can start only after MCP servers are configured")
		return nil
	}
	if report.ConnectedChildren > 0 {
		fmt.Printf("smoke-test: ok, connected %d/%d Claude MCP server(s)\n", report.ConnectedChildren, len(report.Servers))
		return nil
	}
	fmt.Printf("smoke-test: failed, connected 0/%d Claude MCP server(s)\n", len(report.Servers))
	for _, failure := range report.Failures {
		fmt.Printf("- %s/%s %s: %s\n", failure.Scope, failure.Name, failure.Operation, failure.Error)
	}
	return nil
}

type diagnosticReport struct {
	CLIPath           string
	CWCPath           string
	CodexPath         string
	CodexLoggedIn     bool
	CodexLoginUnknown bool
	BridgeConfigured  bool
	BridgeLooksOurs   bool
	ProjectRoot       string
	Servers           []ScopedServer
	ConnectedChildren int
	Failures          []diagnosticFailure
}

type diagnosticFailure struct {
	Name      string
	Scope     string
	Operation string
	Error     string
}

func collectDiagnosticReport(logger *slog.Logger, connect bool) (diagnosticReport, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return diagnosticReport{}, err
	}
	report := diagnosticReport{
		CLIPath:     lookPath("claude-to-codex"),
		CWCPath:     lookPath("cwc"),
		CodexPath:   lookPath("codex"),
		ProjectRoot: currentProjectRoot(),
	}
	report.CodexLoggedIn, report.CodexLoginUnknown = codexLoggedIn()
	report.BridgeConfigured, report.BridgeLooksOurs = codexBridgeConfigured()
	servers, err := loadClaudeServers(home, report.ProjectRoot)
	if err != nil {
		return diagnosticReport{}, err
	}
	report.Servers = servers
	if connect && len(servers) > 0 {
		proxy := newProxyServer(logger)
		failures := proxy.connectChildrenBestEffort(context.Background(), servers)
		defer proxy.close()
		report.ConnectedChildren = len(proxy.children)
		for _, failure := range failures {
			report.Failures = append(report.Failures, diagnosticFailure{
				Name:      failure.server.Name,
				Scope:     failure.server.Scope,
				Operation: failure.operation,
				Error:     failure.err.Error(),
			})
		}
	}
	return report, nil
}

func printDiagnosticReport(title string, report diagnosticReport) {
	fmt.Println(title)
	printCheck("cwc launcher on PATH", report.CWCPath != "", installedStatus(report.CWCPath))
	printCheck("claude-to-codex CLI on PATH", report.CLIPath != "", installedStatus(report.CLIPath))
	printCheck("Codex CLI on PATH", report.CodexPath != "", installedStatus(report.CodexPath))
	printCheck("Codex login", report.CodexLoggedIn, codexStatus(report))
	printCheck("claude-bridge MCP entry", report.BridgeConfigured && report.BridgeLooksOurs, bridgeStatus(report))
	fmt.Printf("- Project root: %s\n", report.ProjectRoot)
	fmt.Printf("- Claude MCP servers found: %d\n", len(report.Servers))
	if report.ConnectedChildren > 0 || len(report.Failures) > 0 {
		fmt.Printf("- Claude MCP servers connected: %d/%d\n", report.ConnectedChildren, len(report.Servers))
	}
	for _, failure := range report.Failures {
		fmt.Printf("  - %s/%s %s failed: %s\n", failure.Scope, failure.Name, failure.Operation, failure.Error)
	}
	fmt.Println()
	fmt.Println("Next commands:")
	fmt.Println("- Daily launch: cwc")
	fmt.Println("- Status: cwc --status")
	fmt.Println("- Smoke test: cwc --smoke-test")
	fmt.Println("- Uninstall preview: cwc --uninstall --dry-run")
}

func printCheck(label string, ok bool, detail string) {
	mark := "OK"
	if !ok {
		mark = "NEEDS ATTENTION"
	}
	fmt.Printf("- %s: %s (%s)\n", label, mark, detail)
}

func installedStatus(path string) string {
	if path == "" {
		return "not found"
	}
	return "found at " + path
}

func codexStatus(report diagnosticReport) string {
	if report.CodexPath == "" {
		return "not installed"
	}
	if report.CodexLoggedIn {
		return "installed and logged in"
	}
	if report.CodexLoginUnknown {
		return "installed; login status unknown"
	}
	return "installed but not logged in"
}

func bridgeStatus(report diagnosticReport) string {
	if !report.BridgeConfigured {
		return "not configured"
	}
	if report.BridgeLooksOurs {
		return "configured for claude-to-codex"
	}
	return "configured but does not appear to point at claude-to-codex"
}

func lookPath(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

func codexLoggedIn() (bool, bool) {
	if lookPath("codex") == "" {
		return false, false
	}
	out, err := exec.Command("codex", "login", "status").CombinedOutput()
	if err != nil {
		return false, false
	}
	text := strings.ToLower(string(out))
	if strings.Contains(text, "not logged") || strings.Contains(text, "logged out") {
		return false, false
	}
	if strings.Contains(text, "logged") || strings.Contains(text, "authenticated") {
		return true, false
	}
	return false, true
}

func codexBridgeConfigured() (bool, bool) {
	if lookPath("codex") != "" {
		out, err := exec.Command("codex", "mcp", "get", "claude-bridge").CombinedOutput()
		if err == nil {
			text := string(out)
			return true, strings.Contains(text, "claude-to-codex")
		}
	}
	cfgPath := filepath.Join(os.Getenv("HOME"), ".codex", "config.toml")
	data, err := os.ReadFile(cfgPath)
	if errors.Is(err, os.ErrNotExist) {
		return false, false
	}
	if err != nil {
		return false, false
	}
	text := string(data)
	if !strings.Contains(text, "claude-bridge") {
		return false, false
	}
	return true, strings.Contains(text, "claude-to-codex")
}
