# Cold Start From Claude Code

Use this guide when the machine already has Claude Code setup, but Codex is not installed, not logged in, or has never been configured with `claude-to-codex`.

The install and uninstall scripts are intended for Linux, WSL/WSL2, and macOS.

The goal is:

- keep Claude Code as the source of truth for existing MCP servers, skills, and slash commands
- install and authenticate Codex
- install `claude-to-codex`
- configure the `claude-bridge` MCP entry
- verify user-scoped and project-scoped Claude MCP servers from Codex
- finish with `cwc --doctor`, `cwc --status`, and `cwc --smoke-test`

Names used in this guide:

- `cwc`: the daily command to type instead of `codex`
- `claude-to-codex`: the maintenance CLI and MCP server binary
- `claude-bridge`: the Codex MCP server entry that runs `claude-to-codex serve`
- `codex-with-claude`: the long-form alias for `cwc`

## Manual Checkpoint 1: OpenAI Account And Access

Before automation can finish, you need a working OpenAI or ChatGPT account that can use Codex CLI.

If you do not have that yet:

1. Create or sign in to your OpenAI/ChatGPT account.
2. Complete any subscription, billing, workspace, or access steps OpenAI requires for Codex CLI.
3. Return to this guide after the account is ready.

Agents should stop here and ask the human to complete this checkpoint if `codex login` cannot finish because of account access, billing, workspace, or entitlement issues.

## Manual Checkpoint 2: Install Codex CLI

Check whether Codex is already installed:

```bash
command -v codex
codex --version
```

If Codex is missing and npm is available, install the current OpenAI npm package:

```bash
npm install -g @openai/codex
```

If npm is not available, install Node.js/npm first or use the current official OpenAI Codex CLI install instructions:

- <https://help.openai.com/en/articles/11096431-openai-codex-ci-getting-started>

After installation:

```bash
command -v codex
codex --version
```

Agents should stop here if Codex is still not on `PATH`.

## Manual Checkpoint 3: Log In To Codex

Check login status:

```bash
codex login status
```

If not logged in, run:

```bash
codex login
```

Complete the browser or device-code flow. When finished:

```bash
codex login status
```

Expected result is a logged-in status. If login fails because the account needs additional OpenAI setup, return to Manual Checkpoint 1.

## Install claude-to-codex

Clone and install:

```bash
mkdir -p ~/dev
git clone https://github.com/lobo235/claude-to-codex.git ~/dev/claude-to-codex
cd ~/dev/claude-to-codex
./install.sh
```

If the repo already exists:

```bash
cd ~/dev/claude-to-codex
git pull --ff-only
./install.sh
```

Ensure the installed commands are on `PATH`:

```bash
command -v claude-to-codex
command -v cwc
```

If they are missing, add this to your shell config and open a new shell:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

## Configure Codex MCP

Use the Codex CLI:

```bash
codex mcp add claude-bridge -- claude-to-codex serve
```

MCP lets Codex talk to external tools. `claude-bridge` is the Codex MCP entry that runs `claude-to-codex`. See [mcp-and-claude-bridge.md](mcp-and-claude-bridge.md) for a short explanation.

Then verify:

```bash
codex mcp list
codex mcp get claude-bridge
```

If `claude-bridge` already exists, inspect it first:

```bash
codex mcp get claude-bridge
```

If it does not point at `claude-to-codex serve`, confirm it is safe to replace. It may be user-owned if the name was reused. Replace only that MCP entry:

```bash
codex mcp remove claude-bridge
codex mcp add claude-bridge -- claude-to-codex serve
```

If `codex mcp add` is unavailable in your Codex version, edit `~/.codex/config.toml` and add:

```toml
[mcp_servers.claude-bridge]
command = "claude-to-codex"
args = ["serve"]
```

Restart Codex after changing MCP config.

Use `cwc` for normal project launches. It computes the `env_vars` Codex
must forward to the `claude-bridge` MCP process for the current project,
including `CLAUDE_BRIDGE_PROJECT_ROOT` and any `${VAR}` / `$VAR`
references in Claude MCP config.

Those `env_vars` entries are variable names, not secret values. If a
project `.mcp.json` references values from a private env file, source that
file before launching `cwc`. Changing a token, env file, `.mcp.json`, or
project root requires a fresh Codex session.

## Sync Claude Skills, Agents, And Commands

Run:

```bash
claude-to-codex sync-skills
claude-to-codex sync-agents
claude-to-codex sync-commands
```

These commands are safe to repeat. They do not overwrite hand-written Codex skills or agents.

`sync-skills` writes generated Codex-compatible wrappers, so Claude skills with missing or incompatible frontmatter can still load in Codex. It uses `codex exec` with a fast model to generate useful frontmatter descriptions from Claude metadata and a bounded, sanitized preview, then records a source hash so reruns are idempotent. If an older bridge version created matching skill symlinks, `sync-skills` replaces those symlinks with generated wrappers.

The first `cwc` launch may print progress while frontmatter is generated, such as `generating frontmatter for <skill_name> [1/20 skills]`. Later launches stay quiet when nothing changed.

`sync-agents` converts Claude agents from `~/.claude/agents/*.md` into Codex subagent TOML files at `$CODEX_HOME/agents/*.toml`, or `~/.codex/agents/*.toml` when `CODEX_HOME` is unset. When launched through `cwc`, project agents from `.claude/agents/*.md` are also synced into `.codex/agents/*.toml` for the current project. Agent descriptions are generated from bounded previews and cached by source hash.

## Verify Claude MCP Servers

From a normal shell:

```bash
claude-to-codex inspect
claude-to-codex inspect --tools
claude-to-codex bridge-env-vars --project "$PWD"
```

`inspect` should show:

- user-scoped servers from `~/.claude.json`
- Claude local-scope servers from the active project's entry in
  `~/.claude.json`
- project-scoped servers from `.mcp.json` when a project root is detected

If the same server name appears in multiple scopes, project `.mcp.json`
overrides Claude local scope, and Claude local scope overrides user
scope.

If a child MCP server fails, fix that child server first. `claude-to-codex` can continue when at least one child server works, but unavailable children will not expose tools to Codex.

## Verify Project-Scoped MCP Servers

Go to a project that has Claude project MCP config:

```bash
cd /path/to/project
test -f .mcp.json && echo "project MCP config exists"
cwc
```

Use `cwc` instead of `codex` for normal project launches. The launcher
sets `CLAUDE_BRIDGE_PROJECT_ROOT`, computes the Codex `env_vars` needed
by `claude-bridge`, syncs Claude skills, agents, and slash commands, then
starts Codex. `codex-with-claude` remains available as the explicit
long-form alias.

Inside Codex, ask it to list or inspect available MCP tools. You should see tools exposed by the `claude-bridge` MCP server.

## Final Verification

Run:

```bash
cwc --doctor
cwc --status
cwc --smoke-test
```

If those checks look good, daily use is:

```bash
cwc
```

## Uninstall And Recovery

To remove only claude-to-codex-owned files:

```bash
cd ~/dev/claude-to-codex
cwc --uninstall --dry-run
cwc --uninstall --yes
```

The uninstaller leaves unrelated Codex config, auth, agents, plugins, MCP entries, hand-written skills, and Claude Code files alone.

If you intentionally remove everything and want to rebuild from scratch:

```bash
cd ~/dev/claude-to-codex
./uninstall.sh --yes
rm -rf ~/dev/claude-to-codex
```

Then start again from Manual Checkpoint 1.

Do not delete `~/.codex`, `~/.claude.json`, `~/.claude/skills`, `~/.claude/commands`, or project `.mcp.json` files unless you also want to remove your normal Codex or Claude Code setup.
