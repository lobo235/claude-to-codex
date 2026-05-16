# Cold Start From Claude Code

Use this guide when the machine already has Claude Code setup, but Codex is not installed, not logged in, or has never been configured with the bridge.

The goal is:

- keep Claude Code as the source of truth for existing MCP servers, skills, and slash commands
- install and authenticate Codex
- install `claude-to-codex`
- connect Codex to the bridge
- verify user-scoped and project-scoped Claude MCP servers from Codex

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
command -v codex-with-claude
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

Then verify:

```bash
codex mcp list
codex mcp get claude-bridge
```

If `claude-bridge` already exists, inspect it first:

```bash
codex mcp get claude-bridge
```

If it does not point at `claude-to-codex serve`, replace only that MCP entry:

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

## Sync Claude Skills And Commands

Run:

```bash
claude-to-codex sync-skills
claude-to-codex sync-commands
```

These commands are safe to repeat. They do not overwrite hand-written Codex skills.

## Verify Claude MCP Servers

From a normal shell:

```bash
claude-to-codex inspect
claude-to-codex inspect --tools
```

`inspect` should show:

- user-scoped servers from `~/.claude.json`
- project-scoped servers from `.mcp.json` when a project root is detected

If a child MCP server fails, fix that child server first. The bridge can continue when at least one child server works, but unavailable children will not expose tools to Codex.

## Verify Project-Scoped MCP Servers

Go to a project that has Claude project MCP config:

```bash
cd /path/to/project
test -f .mcp.json && echo "project MCP config exists"
codex-with-claude
```

Use `codex-with-claude` instead of `codex` for normal project launches. The wrapper sets `CLAUDE_BRIDGE_PROJECT_ROOT`, syncs Claude skills and slash commands, then starts Codex.

Inside Codex, ask it to list or inspect available MCP tools. You should see tools exposed by the `claude-bridge` MCP server.

## Recovery Drill

If you intentionally remove everything and want to rebuild from scratch:

```bash
codex logout || true
rm -rf ~/.codex
rm -f ~/.local/bin/claude-to-codex ~/.local/bin/codex-with-claude
rm -rf ~/dev/claude-to-codex
```

Then start again from Manual Checkpoint 1.

Do not delete `~/.claude.json`, `~/.claude/skills`, `~/.claude/commands`, or project `.mcp.json` files unless you also want to remove your Claude Code setup.
