# claude-to-codex

`claude-to-codex` helps Claude Code users try Codex without rebuilding their local automation from scratch.

It exposes Claude Code MCP servers to Codex, mirrors Claude Code skills into Codex skills, and creates Codex skill wrappers for Claude Code slash commands.

This is useful if you already invested in:

- user-scoped Claude MCP servers in `~/.claude.json`
- project-scoped Claude MCP servers in `.mcp.json`
- Claude Code skills in `~/.claude/skills`
- Claude Code slash commands in `~/.claude/commands`
- project conventions such as `CLAUDE.md`

## What It Does

`claude-to-codex serve` starts a Codex-facing MCP server named `claude-bridge`.

When Codex connects to that server, the bridge:

1. Reads user-scoped MCP servers from `~/.claude.json`.
2. Detects the active project root.
3. Reads project-scoped MCP servers from `<project>/.mcp.json`.
4. Connects to each configured Claude MCP server as a child MCP client.
5. Re-exposes child tools, prompts, resources, resource templates, completions, subscriptions, and reads through one Codex MCP server.

The `scripts/codex-with-claude` wrapper also syncs Claude user artifacts before launching Codex:

- `~/.claude/skills/<name>` is symlinked to `~/.codex/skills/<name>` when no Codex skill already exists.
- `~/.claude/commands/*.md` is mirrored into generated Codex skill wrappers under `~/.codex/skills/<command>/SKILL.md`.

Generated command skills point back to the Claude command source. Claude remains canonical. Hand-written Codex skills are not overwritten.

## Install

Requirements:

- Go matching `go.mod` or newer
- Git
- Node.js/npm if you want the standard Codex CLI npm install path
- Codex installed and available as `codex`
- Claude Code config already present if you want to bridge existing Claude setup

If you are starting from Claude Code only, follow [docs/cold-start.md](docs/cold-start.md) first. That guide includes the manual OpenAI account, Codex install, and login checkpoints.

Clone and install:

```bash
git clone https://github.com/lobo235/claude-to-codex.git ~/dev/claude-to-codex
cd ~/dev/claude-to-codex
./install.sh
```

If you prefer Make directly:

```bash
make test build install
```

The install step places these commands in `~/.local/bin`:

- `claude-to-codex`: the MCP proxy and sync CLI
- `codex-with-claude`: a wrapper around `codex` that detects the project root and syncs Claude artifacts

Make sure `~/.local/bin` is on your `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

## Configure Codex

Add the bridge as a Codex MCP server:

```bash
codex mcp add claude-bridge -- claude-to-codex serve
```

Or edit `~/.codex/config.toml` directly:

```toml
[mcp_servers.claude-bridge]
command = "claude-to-codex"
args = ["serve"]
```

Then launch Codex from a project with:

```bash
codex-with-claude
```

Use `codex-with-claude` instead of `codex` when you want project-scoped Claude MCP servers to load correctly. The wrapper sets `CLAUDE_BRIDGE_PROJECT_ROOT` before Codex starts the MCP server.

## Verify

From any project:

```bash
claude-to-codex inspect
claude-to-codex inspect --tools
claude-to-codex sync-skills
claude-to-codex sync-commands
codex-with-claude
```

`inspect` prints the detected project root and configured child MCP servers.

`inspect --tools` connects to child MCP servers and prints exposed tool names. If one child server fails but another works, the bridge reports the failed child and keeps going.

## Tool Names

If a tool or prompt name is unique across all child servers, Codex sees the original name.

If multiple child servers expose the same name, the bridge prefixes the exposed name with the child server name:

```text
wiki_get_article
project_mcp__status
filesystem__status
```

Calls are routed back to the original child server and original tool name.

## One-Shot Setup Prompt

For a less technical Claude Code user, give an agent the prompt in [SETUP_PROMPT.md](SETUP_PROMPT.md). It asks Claude Code or Codex to install the bridge, update Codex config safely, validate MCP connectivity, sync skills and commands, and report exactly what changed.

That prompt is written for a cold-start user who may need to pause for manual OpenAI signup, Codex installation, and browser login before automation can continue.

## Troubleshooting

Start with [docs/troubleshooting.md](docs/troubleshooting.md).

Common checks:

```bash
command -v claude-to-codex
command -v codex-with-claude
claude-to-codex version
claude-to-codex inspect --tools
```

If project-scoped MCP servers are missing, launch Codex with `codex-with-claude` from inside the project instead of launching `codex` directly.

## Security Notes

The bridge reads local Claude and Codex configuration files and starts the MCP servers already configured on your machine. It does not copy secrets into this repository and does not create a new remote service.

Treat `inspect --tools` like starting your MCP servers: it may execute local stdio MCP commands or connect to HTTP MCP endpoints configured in Claude.

Review generated Codex skills before relying on them. The generated skill wrappers instruct Codex to read the Claude command source at use time.

## Development

```bash
make lint
make test
make cover
make vuln
make build
make inspect
make inspect-tools
```

The binary version is embedded at build time from `git describe --tags --always --dirty`.

## Public Repo Checklist

Before publishing or tagging:

1. Review `LICENSE` and change it if MIT is not the intended license.
2. Confirm the GitHub repository path is `github.com/lobo235/claude-to-codex`; if not, update `go.mod` and README install commands.
3. Run `make lint test cover vuln build`.
4. Push to GitHub and confirm the CI workflow passes.

## Official Codex Links

- OpenAI Codex CLI getting started: <https://help.openai.com/en/articles/11096431-openai-codex-ci-getting-started>
- Codex CLI sign in with ChatGPT: <https://help.openai.com/en/articles/11381614-codex-cli-and-sign-in-withgpt>
