# Architecture

`claude-to-codex` is intentionally small. It has three responsibilities:

1. Load Claude MCP configuration.
2. Proxy MCP capabilities from child servers to Codex.
3. Mirror Claude user artifacts into Codex skill entries.

## CLI Commands

```text
claude-to-codex serve
claude-to-codex inspect [--tools]
claude-to-codex sync-skills
claude-to-codex sync-commands
claude-to-codex version
```

`serve` is the command Codex runs as an MCP server.

`inspect` is for diagnostics and setup validation.

`sync-skills` and `sync-commands` are safe to run repeatedly.

## Config Loading

The bridge loads:

- user-scoped MCP servers from `~/.claude.json`
- project-scoped MCP servers from `<project>/.mcp.json`

Project root detection prefers `CLAUDE_BRIDGE_PROJECT_ROOT`. If it is unset, the bridge walks upward from the current working directory looking for `.mcp.json`, `CLAUDE.md`, or `.git`.

Project-scoped stdio MCP servers run with their working directory set to the detected project root.

## MCP Proxying

Each Claude MCP server becomes a child MCP client session. The bridge registers one Codex-facing MCP server and forwards requests to the correct child.

Forwarded capability types:

- tools
- prompts
- resources
- resource templates
- completions
- resource subscriptions

Tool and prompt name collisions are resolved by prefixing the exposed name with the child server name:

```text
childName__originalName
```

## Failure Model

Child server startup is best effort.

If at least one child server connects, unavailable children are skipped and reported in logs or `inspect --tools`.

If all configured child servers fail, `serve` exits with an error.

## Artifact Sync

Claude skills are symlinked into Codex only when the target Codex skill does not already exist, or when it is already the matching symlink.

Claude slash commands are converted into generated Codex skill wrappers. Generated files include a marker so the bridge can update its own output without overwriting hand-written Codex skills.
