# claude-to-codex

`claude-to-codex` lets Claude Code users try Codex without rebuilding their local automation. It carries over Claude Code MCP servers, skills, slash commands, and project conventions into Codex.

## What To Type

Daily use:

```bash
cwc
```

Setup verification:

```bash
cwc --doctor
cwc --status
cwc --smoke-test
cwc --version
cwc --help
```

Uninstall preview:

```bash
cwc --uninstall --dry-run
```

## Names To Know

- `cwc`: the daily launcher. Run this from a project instead of `codex`.
- `claude-to-codex`: the maintenance CLI. It serves MCP, syncs skills/commands, and provides diagnostics.
- `claude-bridge`: the Codex MCP server entry that runs `claude-to-codex serve`.
- `codex-with-claude`: the same launcher as `cwc`, kept as a longer, explicit alias.

## Quick Start

```bash
git clone https://github.com/lobo235/claude-to-codex.git ~/dev/claude-to-codex
cd ~/dev/claude-to-codex
./install.sh
codex mcp add claude-bridge -- claude-to-codex serve
cwc --doctor
cwc
```

MCP is how Codex talks to external tools. See [docs/mcp-and-claude-bridge.md](docs/mcp-and-claude-bridge.md) for a short explanation of MCP and what `claude-bridge` does.

## Setup Paths

There are three supported ways to set this up. They should all end in the same complete setup state.

1. Read this README and run the commands yourself.
2. Paste [SETUP_PROMPT.md](SETUP_PROMPT.md) into Claude Code or Codex and let an agent do the setup.
3. Give an agent this repository URL and say "install this"; the agent should follow this README and use [SETUP_PROMPT.md](SETUP_PROMPT.md) as the detailed checklist.

If you are starting from Claude Code only, follow [docs/cold-start.md](docs/cold-start.md) first. That guide includes the manual OpenAI account, Codex install, and login checkpoints.

## Complete Setup Means

A complete setup has all of these:

- `claude-to-codex`, `cwc`, and `codex-with-claude` installed on `PATH`
- Codex installed and logged in
- Codex MCP entry `claude-bridge` configured to run `claude-to-codex serve`
- Claude skills, agents, and slash commands synced into generated Codex artifacts
- `cwc --doctor`, `cwc --status`, and `cwc --smoke-test` run without unexpected failures
- daily use is simply `cwc` from inside a project

## Install Details

Requirements:

- Linux, WSL/WSL2, or macOS
- Go matching `go.mod` or newer
- Git
- Node.js/npm if you want the standard Codex CLI npm install path
- Codex installed and available as `codex`
- Claude Code config already present if you want to bridge existing Claude setup

Install:

```bash
git clone https://github.com/lobo235/claude-to-codex.git ~/dev/claude-to-codex
cd ~/dev/claude-to-codex
./install.sh
```

If you prefer Make directly:

```bash
make test build install
```

To install somewhere other than `~/.local`, set `PREFIX`:

```bash
PREFIX=/usr/local ./install.sh
make PREFIX=/usr/local install
```

The install step places these commands in `$PREFIX/bin`:

- `claude-to-codex`: the maintenance CLI
- `cwc`: the daily launcher
- `codex-with-claude`: the same launcher with a more explicit name

Make sure `$PREFIX/bin` is on your `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

That command is correct for the default `PREFIX=$HOME/.local`; adjust it if you choose a different prefix.

## Configure Details

Add `claude-bridge` as a Codex MCP server:

```bash
codex mcp add claude-bridge -- claude-to-codex serve
```

Or edit `~/.codex/config.toml` directly:

```toml
[mcp_servers.claude-bridge]
command = "claude-to-codex"
args = ["serve"]
```

If `claude-bridge` already exists and points at `claude-to-codex serve`, leave it alone. If it points somewhere else, stop and confirm it is safe to replace because it may be user-owned.

Use `cwc` for normal project launches. It computes the environment
variable names referenced by Claude MCP config and starts Codex with a
session-scoped `mcp_servers.claude-bridge.env_vars` override so
project-scoped `.mcp.json` entries and HTTP/SSE headers can reach the
`claude-bridge` process. It forwards variable names only; values stay in
the launching environment.

If a project `.mcp.json` references values from a private env file, source
that file before launching `cwc`. Changing a token, env file, `.mcp.json`,
or project root requires a fresh Codex session because an already-running
Codex-managed `claude-bridge` keeps its original environment.

## Verify

Preferred checks:

```bash
cwc --doctor
cwc --status
cwc --smoke-test
```

Lower-level checks:

```bash
claude-to-codex inspect
claude-to-codex inspect --tools
claude-to-codex bridge-env-vars --project "$PWD"
claude-to-codex sync-skills
claude-to-codex sync-agents
claude-to-codex sync-commands
claude-to-codex sync-artifacts --artifact-cache-dir /tmp/cwc-cache --project "$PWD"
```

Then launch Codex from a project:

```bash
cwc
```

Use `cwc` instead of `codex` when you want project-scoped Claude MCP
servers to load correctly. The launcher sets `CLAUDE_BRIDGE_PROJECT_ROOT`
and passes Codex a per-session `claude-bridge` `env_vars` override for
that project root plus any `${VAR}` / `$VAR` references in Claude MCP
config.

`cwc` reserves only its top-level maintenance options: `--doctor`,
`--status`, `--smoke-test`, `--install`, `--uninstall`, `--version`,
and `--help`. Any other arguments are passed through to `codex`. If a
future Codex top-level flag needs one of those names, use `cwc --
<args...>` to force pass-through.

`cwc --help` prints `codex --help` first, then appends the
`cwc`-specific options. `cwc --version` prints the installed
`claude-to-codex` version and exits without launching Codex.

## Secret Scanning

This repo includes a Gitleaks pre-commit hook:

```bash
sudo apt install pre-commit
pre-commit install
pre-commit run gitleaks --all-files
```

On macOS, use `brew install pre-commit`. If you prefer Python tooling,
install `pre-commit` with `pipx` or your normal Python package manager.

The same full-repo scan is available through Make:

```bash
make secrets
```

If the hook reports a real secret, rotate it before committing. If it
reports a confirmed false positive, prefer changing the example value;
use `.gitleaksignore` only for a specific fingerprint that cannot be
rewritten cleanly.

## MCP Security

Project `.mcp.json` files are trusted code: stdio MCP servers can run
local commands. Run `cwc` only inside projects whose MCP configuration
you trust.

`claude-bridge` starts stdio MCP servers with a restricted environment
by default. It passes a small non-secret baseline such as `PATH`,
`HOME`, temp, locale, and certificate variables, then adds only the
server's explicit `env` values from Claude MCP config. Ambient shell
secrets such as cloud tokens, API keys, and database URLs are not passed
to every child MCP process.

If a legacy MCP server requires ambient shell environment variables,
prefer opting in for only that server:

```json
{
  "mcpServers": {
    "legacy-tool": {
      "command": "legacy-mcp",
      "x-claude-bridge-inherit-env": true
    }
  }
}
```

For temporary compatibility debugging, this global escape hatch restores
full environment inheritance for all stdio MCP children:

```bash
CLAUDE_BRIDGE_INHERIT_ENV=1 cwc
```

HTTP MCP headers and stdio `env` values are never printed by normal
diagnostics. Diagnostic URLs and error text are redacted for common
token, key, secret, password, authorization, credential, and signature
patterns.

## What It Does

`claude-to-codex serve` starts a Codex-facing MCP server named `claude-bridge`.

When Codex connects to `claude-bridge`, `claude-to-codex`:

1. Reads user-scoped MCP servers from `~/.claude.json`.
2. Detects the active project root.
3. Reads project-scoped MCP servers from `<project>/.mcp.json`.
4. Connects to each configured Claude MCP server as a child MCP client.
5. Re-exposes child tools, prompts, resources, resource templates, completions, subscriptions, and reads through one Codex MCP server.

Claude-style HTTP MCP entries are supported for streamable HTTP and
legacy SSE transports. For SSE, use `type: "sse"` with `url` and optional
`headers`:

```json
{
  "mcpServers": {
    "remote-tools": {
      "type": "sse",
      "url": "https://example.com/sse",
      "headers": {
        "Authorization": "Bearer ${REMOTE_TOOLS_TOKEN}"
      }
    }
  }
}
```

String values in MCP server config support `${VAR}` and `$VAR` environment
expansion. Missing variables fail that child server closed before the
bridge connects to it; unrelated child servers can still start.
When launched with `cwc`, those variable names are forwarded to the
Codex-managed `claude-bridge` MCP process for that session. The forwarded
entries are variable names, not secret values; the values must already be
present in the environment that launches `cwc`.

The `cwc` launcher also syncs Claude user and project artifacts before launching Codex:

- `~/.claude/skills/<name>/SKILL.md` is mirrored into a generated Codex-compatible wrapper at `$CODEX_HOME/skills/<name>/SKILL.md`, or `~/.codex/skills/<name>/SKILL.md` when `CODEX_HOME` is unset.
- `~/.claude/agents/*.md` is mirrored into generated Codex agent TOML under `$CODEX_HOME/agents/*.toml`, or `~/.codex/agents/*.toml` when `CODEX_HOME` is unset.
- `<project>/.claude/agents/*.md` is mirrored into generated Codex agent TOML under `<project>/.codex/agents/*.toml`.
- `~/.claude/commands/*.md` is mirrored into generated Codex skill wrappers under `$CODEX_HOME/skills/<command>/SKILL.md`, or `~/.codex/skills/<command>/SKILL.md` when `CODEX_HOME` is unset.

Generated skill wrappers and agent TOML files point back to the Claude source. Claude remains canonical. Hand-written Codex skills and agents are not overwritten. Descriptions are generated with `codex exec` using a fast model, then cached by source hash so unchanged artifacts are not rewritten on every launch. The generator receives only Claude metadata and a bounded, sanitized preview, not the full skill or agent body.

Set `CLAUDE_TO_CODEX_FRONTMATTER_MODEL` to override the default fast model, or set `CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER=1` to force deterministic fallback descriptions.

For headless automation, set `CLAUDE_TO_CODEX_ARTIFACT_CACHE_DIR`.
When that variable is present, `cwc` runs `sync-artifacts` instead of the
desktop sync sequence. Generated wrappers and agent TOML are refreshed in
a schema/versioned persistent cache, then materialized into `CODEX_HOME`
if set, otherwise `$HOME/.codex`. The cache contains only generated
artifacts, not Codex auth, config, sessions, logs, or other runtime state.
Project agents are cached outside the repository and materialized into
`$CODEX_HOME/agents`, so automation mode does not write
`<project>/.codex/agents`.

Auto-agents workers should mount persistent storage at
`/mnt/fast/auto-agents` and keep `$HOME`/`CODEX_HOME` allocation-local:

```bash
HOME="$ALLOC_HOME" \
CODEX_HOME="$ALLOC_CODEX_HOME" \
CLAUDE_TO_CODEX_ARTIFACT_CACHE_DIR=/mnt/fast/auto-agents/cwc-cache \
cwc exec ...
```

When `cwc` has to generate frontmatter or agent descriptions, it prints progress such as:

```text
generating frontmatter for <skill_name> [1/20 skills]
generating description for agent <agent_name> [1/3 agents]
```

## For Agents

If the user gives you this repository URL and says "install this":

1. Follow this README.
2. Use [SETUP_PROMPT.md](SETUP_PROMPT.md) as the detailed checklist.
3. Preserve unrelated Codex and Claude Code state.
4. End with `cwc --doctor`, `cwc --status`, `cwc --smoke-test`, and tell the user daily use is `cwc`.

## Uninstall

To remove claude-to-codex files and return to plain Codex/Claude Code:

```bash
cd ~/dev/claude-to-codex
cwc --uninstall --yes
```

For a preview:

```bash
cwc --uninstall --dry-run
```

The uninstaller removes installed `claude-to-codex`/`cwc` commands, generated Codex skill wrappers and agents containing `generated-by: claude-to-codex`, and the `claude-bridge` MCP entry only when it points at `claude-to-codex`. It does not remove unrelated Codex config, auth, agents, plugins, MCP entries, hand-written skills, or Claude Code files.

## Tool Names

Codex sees child tools and prompts as native tools on the configured
`claude-bridge` MCP server. To preserve the original Claude MCP server
identity, `claude-to-codex` always prefixes bridged tool and prompt names
with the child server name:

```text
wiki__wiki_get_article
project_mcp__status
filesystem__list
```

Calls are routed back to the original child server and original tool name.
In Codex's function-tool UI, the full native tool name includes the
Codex MCP server plus this exposed child-prefixed name.

## Links

- [One-shot setup prompt](SETUP_PROMPT.md)
- [Cold start from Claude Code](docs/cold-start.md)
- [Troubleshooting](docs/troubleshooting.md)
- [MCP and `claude-bridge`](docs/mcp-and-claude-bridge.md)

## Security Notes

`claude-to-codex` reads local Claude and Codex configuration files and starts the MCP servers already configured on your machine. It does not copy secrets into this repository and does not create a new remote service.

Treat `inspect --tools` like starting your MCP servers: it may execute local stdio MCP commands or connect to HTTP MCP endpoints configured in Claude.

Review generated Codex skills and agents before relying on them. The generated artifacts instruct Codex to use the Claude source as canonical context.

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
