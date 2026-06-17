package bridge

import (
	"reflect"
	"testing"
)

func TestBridgeEnvVarsForServersIncludesProjectRootAndClaudeConfigRefs(t *testing.T) {
	envVars, err := bridgeEnvVarsForServers([]ScopedServer{{
		Name:  "remote-tools",
		Scope: "project",
		Config: MCPServerConfig{
			Type:    "sse",
			URL:     "https://${MCP_HOST}/sse",
			Command: "$MCP_COMMAND",
			Args:    []string{"--token", "${REMOTE_TOOLS_TOKEN}"},
			Env: map[string]string{
				"CHILD_TOKEN": "$CHILD_TOKEN_SOURCE",
				"LITERAL":     "not-secret",
			},
			Headers: map[string]string{
				"Authorization": "Bearer ${REMOTE_AUTH_TOKEN}",
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"CHILD_TOKEN_SOURCE",
		"CLAUDE_BRIDGE_PROJECT_ROOT",
		"MCP_COMMAND",
		"MCP_HOST",
		"REMOTE_AUTH_TOKEN",
		"REMOTE_TOOLS_TOKEN",
	}
	if !reflect.DeepEqual(envVars, want) {
		t.Fatalf("env vars = %#v, want %#v", envVars, want)
	}
}

func TestBridgeEnvVarsForServersRejectsMalformedReferences(t *testing.T) {
	_, err := bridgeEnvVarsForServers([]ScopedServer{{
		Name:  "remote-tools",
		Scope: "project",
		Config: MCPServerConfig{
			Headers: map[string]string{
				"Authorization": "Bearer ${REMOTE_AUTH_TOKEN",
			},
		},
	}})
	if err == nil {
		t.Fatal("bridgeEnvVarsForServers accepted a malformed env reference")
	}
}
