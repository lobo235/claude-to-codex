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

`cwc` starts Codex after syncing Claude skills, agents, and commands into Codex-compatible artifacts. It also sets the project root so project `.mcp.json` and `.claude/agents` files are found.

Project `.mcp.json` files are trusted code. A project-scoped stdio MCP
server can run local commands, so use `cwc` only in projects whose MCP
configuration you trust.

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
