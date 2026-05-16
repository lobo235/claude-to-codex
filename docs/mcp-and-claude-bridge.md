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

`cwc` starts Codex after syncing Claude skills and commands into Codex-compatible skill wrappers. It also sets the project root so project `.mcp.json` files are found.

The Codex config entry looks like this:

```toml
[mcp_servers.claude-bridge]
command = "claude-to-codex"
args = ["serve"]
```

That tells Codex: "when I need the `claude-bridge` MCP server, run `claude-to-codex serve`."
