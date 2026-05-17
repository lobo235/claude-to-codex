# Troubleshooting

This guide assumes `claude-to-codex` was installed from `~/dev/claude-to-codex` with `./install.sh` or `make install`.

Names used here:

- `cwc`: the daily command to type instead of `codex`
- `claude-to-codex`: the maintenance CLI and MCP server binary
- `claude-bridge`: the Codex MCP server entry that runs `claude-to-codex serve`
- `codex-with-claude`: the long-form alias for `cwc`

## Quick Diagnostics

```bash
command -v codex
codex --version
codex login status
codex mcp list
cwc --doctor
cwc --status
cwc --smoke-test
```

## `claude-to-codex: command not found`

The maintenance CLI is not installed or `~/.local/bin` is not on `PATH`.

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

Prefer the CLI:

```bash
codex mcp add claude-bridge -- claude-to-codex serve
codex mcp get claude-bridge
```

If an old `claude-bridge` entry already exists and points somewhere else, confirm it is safe to replace. It may be user-owned if the name was reused:

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

Launch Codex through `cwc` from inside the project:

```bash
cd /path/to/project
cwc
```

`cwc` sets `CLAUDE_BRIDGE_PROJECT_ROOT`. Without that environment variable, Codex may start `claude-bridge` from a different working directory and `claude-to-codex` may not find the intended `.mcp.json`.

`codex-with-claude` is installed too; it is the same launcher with a more explicit name.

## `inspect --tools` Reports Child Failures

This means `claude-to-codex` found the child MCP server config but could not connect to that child.

Common causes:

- The child MCP command is not installed or not on `PATH`.
- The child MCP server needs environment variables that are missing in the current shell.
- A project-scoped server expects to run from the project root, but Codex was launched directly instead of with `cwc`.
- An HTTP MCP URL is unreachable.
- HTTP MCP headers or auth values in the Claude config are stale.

`claude-bridge` uses a restricted environment for stdio MCP servers by
default. If a server needs a token or other credential, put that value
in that server's Claude MCP `env` config. For legacy servers that cannot
be configured that way, add `"x-claude-bridge-inherit-env": true` to
that one server. Use `CLAUDE_BRIDGE_INHERIT_ENV=1` only as a temporary
all-server compatibility escape hatch.

If at least one child connects, `claude-to-codex` keeps running and skips the unavailable child. If all children fail, `claude-bridge` startup fails.

## Tools Have Unexpected Names

When two child MCP servers expose the same tool name, `claude-to-codex` prefixes the exposed tool name with the child server name:

```text
serverName__toolName
```

The original child tool name is still used when `claude-to-codex` forwards the call.

## Claude Skills Did Not Appear In Codex

Run:

```bash
claude-to-codex sync-skills
```

Only valid skill names are synced. Names must match:

```text
^[a-z0-9][a-z0-9-]*$
```

`claude-to-codex` creates generated Codex-compatible skill wrappers under `~/.codex/skills/<name>/SKILL.md`. This gives Codex valid frontmatter even when the source Claude skill uses a different format.

`sync-skills` uses `codex exec` with a fast model to write a useful frontmatter description from Claude metadata and a bounded, sanitized preview, then records the source hash in the wrapper. Rerunning it should leave unchanged wrappers alone. If metadata generation fails, `claude-to-codex` falls back to a deterministic description so the skill still loads.

When `cwc` has to generate or refresh skill frontmatter, it prints progress to stderr, for example `generating frontmatter for <skill_name> [1/20 skills]`. Normal launches stay quiet when every generated wrapper is current.

`claude-to-codex` skips a Claude skill when a Codex skill with the same name already exists and is not generated by `claude-to-codex`. This protects hand-written Codex skills. It will replace an older matching symlink created by a previous version.

## Claude Agents Did Not Appear In Codex

Run:

```bash
claude-to-codex sync-agents
```

For project agents, run from the project through `cwc` or specify the project explicitly:

```bash
claude-to-codex sync-agents --project /path/to/project
```

User agents are read from `~/.claude/agents/*.md` and written as generated TOML files under `~/.codex/agents/*.toml`. Project agents are read from `.claude/agents/*.md` and written under `.codex/agents/*.toml`.

`claude-to-codex` skips malformed agent files, empty agent bodies, invalid filenames, hand-written Codex agents, and duplicate Codex agent names. Generated agents contain this marker:

```text
generated-by: claude-to-codex sync-agents
```

When `cwc` has to generate or refresh a sparse agent description, it prints progress to stderr, for example `generating description for agent <agent_name> [1/3 agents]`. Normal launches stay quiet when every generated agent is current.

## Claude Slash Commands Did Not Appear In Codex

Run:

```bash
claude-to-codex sync-commands
```

Commands are read from `~/.claude/commands/*.md`. `claude-to-codex` creates generated Codex skill wrappers under `~/.codex/skills/<command>/SKILL.md`.

`claude-to-codex` skips a command if a hand-written Codex skill already exists at the same path. It updates only files that contain the generated marker:

```text
generated-by: claude-to-codex sync-commands
```

## `cwc` Starts Codex But No Bridge Tools Work

Run this outside Codex:

```bash
claude-to-codex inspect --tools
```

If this works, Codex is probably not loading the MCP config. Check `~/.codex/config.toml` and restart Codex.

If this fails, fix the child MCP server failure first. The error output should name the child server and whether the failure happened while connecting or listing tools.

## Reset Generated Command Skills

To regenerate command skill wrappers created by `claude-to-codex`:

```bash
find ~/.codex/skills -name SKILL.md -print | xargs grep -l "generated-by: claude-to-codex sync-commands"
claude-to-codex sync-commands
```

Remove only generated files you intentionally want rebuilt. Do not delete hand-written Codex skills unless you know you no longer need them.

## Uninstall claude-to-codex

Preview the removal first:

```bash
cd ~/dev/claude-to-codex
cwc --uninstall --dry-run
```

Then remove claude-to-codex-owned files:

```bash
cwc --uninstall --yes
```

The uninstaller removes only installed `claude-to-codex`/`cwc` commands, generated Codex skill wrappers and agents containing `generated-by: claude-to-codex`, and the `claude-bridge` MCP entry when it points at `claude-to-codex`. It leaves unrelated Codex config, auth, agents, plugins, MCP entries, hand-written skills, and Claude Code files alone.
