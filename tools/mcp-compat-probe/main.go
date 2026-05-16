package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type probe struct {
	name string
	args map[string]any
}

type result struct {
	ListedTools []string     `json:"listedTools"`
	Calls       []callResult `json:"calls"`
	Skipped     []string     `json:"skipped,omitempty"`
	Errors      []string     `json:"errors,omitempty"`
}

type callResult struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
	Err  string `json:"err,omitempty"`
}

func main() {
	var bridge string
	var timeout time.Duration
	var allowedFile string
	flag.StringVar(&bridge, "bridge", "bin/claude-to-codex", "path to claude-to-codex binary")
	flag.DurationVar(&timeout, "timeout", 180*time.Second, "overall probe timeout")
	flag.StringVar(&allowedFile, "allowed-file", "", "file path used for filesystem read probes")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bridge, "serve")
	cmd.Env = os.Environ()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "mcp-compat-probe", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.CommandTransport{Command: cmd, TerminateDuration: 2 * time.Second}, nil)
	if err != nil {
		fail(result{Errors: []string{"connect bridge: " + err.Error()}})
	}
	defer session.Close()

	var out result
	available := map[string]bool{}
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			fail(result{Errors: []string{"list tools: " + err.Error()}})
		}
		out.ListedTools = append(out.ListedTools, tool.Name)
		available[tool.Name] = true
	}
	sort.Strings(out.ListedTools)

	for _, p := range probes(allowedFile) {
		if !available[p.name] {
			out.Skipped = append(out.Skipped, p.name)
			continue
		}
		_, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: p.name, Arguments: p.args})
		call := callResult{Name: p.name, OK: err == nil}
		if err != nil {
			call.Err = err.Error()
		}
		out.Calls = append(out.Calls, call)
	}
	sort.Strings(out.Skipped)
	write(out)
	for _, call := range out.Calls {
		if !call.OK {
			os.Exit(1)
		}
	}
}

func probes(allowedFile string) []probe {
	ps := []probe{
		{name: "get-structured-content", args: map[string]any{}},
		{name: "get_config", args: map[string]any{}},
		{name: "list_allowed_directories", args: map[string]any{}},
		{name: "read_graph", args: map[string]any{}},
		{name: "resolve-library-id", args: map[string]any{"libraryName": "react"}},
		{name: "sequentialthinking", args: map[string]any{
			"thought":           "compatibility probe",
			"thoughtNumber":     1,
			"totalThoughts":     1,
			"nextThoughtNeeded": false,
		}},
		{name: "browser_navigate", args: map[string]any{"url": "data:text/html,<title>probe</title><p>ok</p>"}},
		{name: "browser_snapshot", args: map[string]any{}},
		{name: "list_pages", args: map[string]any{}},
	}
	if allowedFile != "" {
		ps = append(ps,
			probe{name: "read_text_file", args: map[string]any{"path": allowedFile}},
			probe{name: "filesystem__read_text_file", args: map[string]any{"path": allowedFile}},
		)
	}
	return ps
}

func write(out result) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "encode result: %v\n", err)
		os.Exit(1)
	}
}

func fail(out result) {
	if len(out.Errors) == 0 {
		out.Errors = []string{"unknown failure"}
	}
	for i, err := range out.Errors {
		out.Errors[i] = strings.TrimSpace(err)
	}
	write(out)
	os.Exit(1)
}
