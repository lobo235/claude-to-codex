# MCP And `claude-bridge`

MCP means Model Context Protocol. It is a standard way for an AI coding tool to talk to local or remote tools.

In this project:

- Claude Code already knows about your MCP servers through `~/.claude.json` and project `.mcp.json` files.
- Codex does not read those Claude Code files directly.
- `claude-to-codex serve` starts one Codex MCP server named `claude-bridge`.
- `claude-bridge` reads your Claude MCP configuration, connects to those Claude MCP servers, and exposes their tools to Codex.

The daily command is:

```bash
cwc
```

`cwc` starts Codex after syncing Claude skills, agents, and commands into Codex-compatible artifacts. It also sets the project root so Claude local-scope MCP entries, project `.mcp.json`, and `.claude/agents` files are found. For the Codex-managed `claude-bridge` MCP process, `cwc` passes a per-session `env_vars` override containing `CLAUDE_BRIDGE_PROJECT_ROOT` plus any `${VAR}` or `$VAR` references found in Claude MCP config.

Those `env_vars` entries are variable names, not secret values. If a
Claude local-scope entry or project `.mcp.json` uses a header such as
`"Authorization": "Bearer ${REMOTE_TOOLS_TOKEN}"`, that token must be
present in the environment that launches `cwc`. For example, if a
private env file owns the value, source it before starting a fresh Codex
session:

```bash
cd /path/to/project
set -a
source ~/.private-mcp/env
set +a
cwc
```

Changing a token, env file, or project root does not update an already
running Codex-managed `claude-bridge` process. Restart Codex from the
intended project with `cwc` after those changes.

Claude local-scope MCP entries and project `.mcp.json` files are trusted
code. Their stdio MCP servers can run local commands, so use `cwc` only
in projects whose MCP configuration you trust.

For stdio MCP servers, `claude-bridge` does not pass the full shell
environment by default. It passes a small non-secret baseline plus the
server's explicit Claude MCP `env` values. If a legacy MCP server needs
full environment inheritance, prefer the per-server
`"x-claude-bridge-inherit-env": true` escape hatch; use
`CLAUDE_BRIDGE_INHERIT_ENV=1` only for temporary all-server
compatibility debugging.

The Codex config entry looks like this:

```toml
[mcp_servers.claude-bridge]
command = "claude-to-codex"
args = ["serve"]
```

That tells Codex: "when I need the `claude-bridge` MCP server, run `claude-to-codex serve`."

Codex sees the bridged tools as native tools on the `claude-bridge` MCP
server. The exposed tool names include the Claude child MCP server name,
for example `project_db__query_read`, so the model can tell which child
server will receive the call.

Proxied child tool calls use the bridge child-operation timeout
(`CLAUDE_BRIDGE_OPERATION_TIMEOUT`, default `30s`). If a child tool call
fails after tools were listed, current versions return an error that names
the child scope, child server, original tool name, exposed tool name, and
a hint for common causes such as missing env vars, auth failures, timeouts,
or closed HTTP/SSE connections. When a closed or stale child session is
detected, `claude-bridge` reconnects only that affected child server. It
does not blindly retry the current `tools/call`, because the bridge cannot
prove whether the child received a mutating request, but future calls use
the refreshed child session. See [troubleshooting](troubleshooting.md) for
recovery steps.
