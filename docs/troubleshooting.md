# Troubleshooting

This guide assumes the bridge was installed from `~/dev/claude-to-codex` with `./install.sh` or `make install`.

## Quick Diagnostics

```bash
command -v codex
codex --version
codex login status
codex mcp list
command -v claude-to-codex
command -v codex-with-claude
claude-to-codex version
claude-to-codex inspect
claude-to-codex inspect --tools
```

## `claude-to-codex: command not found`

The bridge is not installed or `~/.local/bin` is not on `PATH`.

Run:

```bash
cd ~/dev/claude-to-codex
./install.sh
export PATH="$HOME/.local/bin:$PATH"
```

Then add the PATH line to your shell config, such as `~/.bashrc`, `~/.zshrc`, or equivalent.

## `codex: command not found`

Codex CLI is not installed or is not on `PATH`.

If npm is available:

```bash
npm install -g @openai/codex
```

Then verify:

```bash
command -v codex
codex --version
```

If npm is unavailable or installation fails, follow OpenAI's current Codex CLI install instructions.

## Codex Is Not Logged In

Run:

```bash
codex login status
codex login
```

Complete the browser or device login flow. If OpenAI reports missing access, billing, subscription, credits, or workspace eligibility, complete that manual account step first and then retry `codex login`.

## Codex Does Not Show `claude-bridge`

Check `~/.codex/config.toml` contains:

Prefer the CLI:

```bash
codex mcp add claude-bridge -- claude-to-codex serve
codex mcp get claude-bridge
```

If an old `claude-bridge` entry already exists and points somewhere else:

```bash
codex mcp remove claude-bridge
codex mcp add claude-bridge -- claude-to-codex serve
```

Or check `~/.codex/config.toml` contains:

```toml
[mcp_servers.claude-bridge]
command = "claude-to-codex"
args = ["serve"]
```

Then restart Codex. MCP servers are usually read when Codex starts.

## User MCP Servers Are Missing

User-scoped Claude MCP servers are read from `~/.claude.json`.

Check:

```bash
claude-to-codex inspect
```

If no user servers appear, confirm Claude Code has MCP servers in `~/.claude.json` under `mcpServers`.

## Project MCP Servers Are Missing

Project-scoped Claude MCP servers are read from `<project>/.mcp.json`.

Launch Codex through the wrapper from inside the project:

```bash
cd /path/to/project
codex-with-claude
```

The wrapper sets `CLAUDE_BRIDGE_PROJECT_ROOT`. Without that environment variable, Codex may start the MCP bridge from a different working directory and the bridge may not find the intended `.mcp.json`.

## `inspect --tools` Reports Child Failures

This means the bridge found the child MCP server config but could not connect to that child.

Common causes:

- The child MCP command is not installed or not on `PATH`.
- The child MCP server needs environment variables that are missing in the current shell.
- A project-scoped server expects to run from the project root, but Codex was launched directly instead of with `codex-with-claude`.
- An HTTP MCP URL is unreachable.
- HTTP MCP headers or auth values in the Claude config are stale.

If at least one child connects, the bridge keeps running and skips the unavailable child. If all children fail, bridge startup fails.

## Tools Have Unexpected Names

When two child MCP servers expose the same tool name, the bridge prefixes the exposed tool name with the child server name:

```text
serverName__toolName
```

The original child tool name is still used when the bridge forwards the call.

## Claude Skills Did Not Appear In Codex

Run:

```bash
claude-to-codex sync-skills
```

Only valid skill names are synced. Names must match:

```text
^[a-z0-9][a-z0-9-]*$
```

The bridge skips a Claude skill when a Codex skill with the same name already exists and is not the matching symlink. This protects hand-written Codex skills.

## Claude Slash Commands Did Not Appear In Codex

Run:

```bash
claude-to-codex sync-commands
```

Commands are read from `~/.claude/commands/*.md`. The bridge creates generated Codex skill wrappers under `~/.codex/skills/<command>/SKILL.md`.

The bridge skips a command if a hand-written Codex skill already exists at the same path. It updates only files that contain the generated marker:

```text
generated-by: claude-to-codex sync-commands
```

## `codex-with-claude` Starts Codex But No Bridge Tools Work

Run this outside Codex:

```bash
claude-to-codex inspect --tools
```

If this works, Codex is probably not loading the MCP config. Check `~/.codex/config.toml` and restart Codex.

If this fails, fix the child MCP server failure first. The error output should name the child server and whether the failure happened while connecting or listing tools.

## Reset Generated Command Skills

To regenerate bridge-created command skills:

```bash
find ~/.codex/skills -name SKILL.md -print | xargs grep -l "generated-by: claude-to-codex sync-commands"
claude-to-codex sync-commands
```

Remove only generated files you intentionally want rebuilt. Do not delete hand-written Codex skills unless you know you no longer need them.
