# Architecture

`claude-to-codex` is intentionally small. It has three responsibilities:

1. Load Claude MCP configuration.
2. Proxy MCP capabilities from child servers to Codex.
3. Mirror Claude user and project artifacts into Codex entries.

It implements public Claude MCP compatibility, not local customization.
Project-specific domains, token names, credential files, and operator
wrappers stay outside this repository. See
[ADR 0001](adr/0001-public-claude-mcp-compatibility.md).

## Naming

- `cwc`: the daily launcher users type instead of `codex`
- `claude-to-codex`: the maintenance CLI and MCP server binary
- `claude-bridge`: the Codex MCP server entry that runs `claude-to-codex serve`
- `codex-with-claude`: the long-form alias for `cwc`

## CLI Commands

```text
claude-to-codex serve
claude-to-codex inspect [--tools]
claude-to-codex bridge-env-vars [--project <dir>]
claude-to-codex sync-skills
claude-to-codex sync-agents [--project <dir>|--project-only <dir>]
claude-to-codex sync-commands
claude-to-codex sync-artifacts [--project <dir>] [--artifact-cache-dir <dir>] [--codex-home <dir>]
claude-to-codex version
```

`serve` is the command Codex runs for the `claude-bridge` MCP server.

`inspect` is for diagnostics and setup validation.

`bridge-env-vars` prints the Codex `env_vars` array that `cwc` passes
as a per-session config override. It includes
`CLAUDE_BRIDGE_PROJECT_ROOT` plus variable names referenced by Claude MCP
config strings, including Claude local-scope entries stored under the
active project in `~/.claude.json`.

`sync-skills`, `sync-agents`, `sync-commands`, and `sync-artifacts` are safe to run repeatedly.

## Config Loading

`claude-to-codex` loads:

- user-scoped MCP servers from `~/.claude.json`
- Claude local-scope MCP servers from the active project's entry in
  `~/.claude.json`
- project-scoped MCP servers from `<project>/.mcp.json`

When multiple scopes define the same MCP server name, the more specific
scope wins: project `.mcp.json`, then Claude local scope, then user
scope.

Project root detection prefers `CLAUDE_BRIDGE_PROJECT_ROOT`. If it is unset, `claude-to-codex` walks upward from the current working directory looking for `.mcp.json`, `CLAUDE.md`, or `.git`.

Codex stdio MCP servers do not automatically inherit the launcher's full
environment. `cwc` therefore starts Codex with
`-c mcp_servers.claude-bridge.env_vars=...` so the Codex-managed
`claude-bridge` process receives the project root and any environment
variables referenced by the active Claude MCP configuration.

String values in MCP server config are expanded with `${VAR}` and `$VAR`
environment references when that child server connects. Missing variables
fail that child server closed with a diagnostic that names the missing
variable but not any configured secret values. Config loading itself keeps
raw Claude-shaped entries intact so one missing child credential does not
prevent unrelated child servers from starting.

Claude local-scope and project-scoped stdio MCP servers run with their
working directory set to the detected project root.

## MCP Child Environment

Stdio child MCP servers run with a restricted environment by default.
The bridge passes a small non-secret baseline (`PATH`, `HOME`, user,
shell, temp, locale, and certificate variables), then overlays the
server's explicit Claude MCP `env` values. Claude local-scope and
project-scoped servers also receive `CLAUDE_BRIDGE_PROJECT_ROOT`.

Ambient shell secrets are not inherited by default. Compatibility
escape hatches are available:

- per-server: `"x-claude-bridge-inherit-env": true`
- global: `CLAUDE_BRIDGE_INHERIT_ENV=1`

Diagnostic output redacts common token, key, secret, password,
authorization, credential, and signature patterns from URLs and errors.
HTTP MCP headers and stdio env values are not printed.

## MCP Proxying

Each Claude MCP server becomes a child MCP client session. `claude-to-codex` registers one Codex-facing MCP server and forwards requests to the correct child.

Supported child transports:

- stdio via `command` and optional `args`
- streamable HTTP via `url`, `type: "http"`, or `type: "streamable-http"`
- legacy MCP SSE via `type: "sse"` and `url`

HTTP and SSE entries may define `headers`. Header values are expanded from
the environment, attached to outbound requests, and omitted from normal
diagnostics.

Forwarded capability types:

- tools
- prompts
- resources
- resource templates
- completions
- resource subscriptions

Tool and prompt names are always exposed with the child server name as a
prefix so Codex-visible tools preserve their Claude MCP origin:

```text
childName__originalName
```

## Failure Model

Child server startup is best effort.

If at least one child server connects, unavailable children are skipped and reported in logs or `inspect --tools`.

If all configured child servers fail, `serve` exits with an error.

Child capability calls use the same child-operation timeout budget as
startup and listing. Tool-call failures are wrapped before being returned
to Codex so the error identifies the child scope, child server, operation,
original tool name, exposed tool name, and a hint for common env, auth,
timeout, or closed-connection failures.

## Artifact Sync

Claude skills are converted into generated Codex-compatible skill wrappers under `$CODEX_HOME/skills`, or `~/.codex/skills` when `CODEX_HOME` is unset. The generated file supplies valid Codex frontmatter, points back to the Claude `SKILL.md`, and includes a source snapshot so Claude skills with missing or incompatible frontmatter still load in Codex.

`sync-skills` asks `codex exec` to generate a concise, useful `description` with a fast model. The generator receives only Claude metadata and a bounded, sanitized preview, not the full skill body, and it runs from a temporary directory. Set `CLAUDE_TO_CODEX_FRONTMATTER_MODEL` to override the default model, or `CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER=1` to use deterministic fallback descriptions. Generated wrappers include `source-sha256`; if the source hash is unchanged, the wrapper is left untouched.

When a skill wrapper needs generated frontmatter, `sync-skills --quiet` still writes progress to stderr so `cwc` is not silent during slow first-run work. It stays silent for unchanged wrappers.

Claude agents are converted into generated Codex subagent TOML files. User agents from `~/.claude/agents/*.md` sync to `$CODEX_HOME/agents/*.toml`, or `~/.codex/agents/*.toml` when `CODEX_HOME` is unset. Project agents from `<project>/.claude/agents/*.md` sync to `<project>/.codex/agents/*.toml` when `sync-agents --project <dir>` runs through `cwc`.

Generated agent filenames keep Claude's hyphenated filename, while the Codex `name` field uses underscores. Claude `tools` frontmatter is preserved as advisory instruction text, not converted into Codex permissions. Generated agents omit `model`, `model_reasoning_effort`, and `sandbox_mode` so they inherit the active Codex session.

`sync-agents` updates and deletes only generated TOML files containing the claude-to-codex marker. Hand-written Codex agents are skipped. Sparse agent descriptions use the same bounded preview generation path as skills; slow generation writes progress to stderr in quiet mode.

Claude slash commands are converted into generated Codex skill wrappers under `$CODEX_HOME/skills`, or `~/.codex/skills` when `CODEX_HOME` is unset. Generated files include a marker so `claude-to-codex` can update its own output without overwriting hand-written Codex skills.

`sync-artifacts` is the headless automation interface. It writes generated
skills, slash-command wrappers, user agents, and optional project agents
into a persistent cache under
`<cache>/schema-v1/claude-to-codex-<version>/...`, then materializes only
marked generated artifacts into `CODEX_HOME` or the `--codex-home`
destination. The cache refresh is protected by an atomic `mkdir` lock so
multiple workers can share an NFS cache without rewriting the same
generated files concurrently.

Automation cache mode is enabled by setting
`CLAUDE_TO_CODEX_ARTIFACT_CACHE_DIR` before launching `cwc`. In that mode,
`cwc` runs `sync-artifacts --project <root>` instead of the desktop
`sync-skills` / `sync-agents` / `sync-commands` sequence. The persistent
cache stores only generated wrappers and agent TOML files. It does not
copy Codex `auth.json`, `config.toml`, sessions, logs, or other runtime
state. Project agents are cached outside the repository and materialized
into the allocation-local `CODEX_HOME/agents`; automation mode does not
write `<project>/.codex/agents`.

Auto-agents workers should mount persistent storage at
`/mnt/fast/auto-agents`, keep `$HOME` and `CODEX_HOME` allocation-local,
and launch `cwc` like:

```bash
HOME="$ALLOC_HOME" \
CODEX_HOME="$ALLOC_CODEX_HOME" \
CLAUDE_TO_CODEX_ARTIFACT_CACHE_DIR=/mnt/fast/auto-agents/cwc-cache \
cwc exec ...
```

## Install And Uninstall Boundaries

Install writes only `claude-to-codex` and launcher commands into `PREFIX/bin` and tells the user how to configure `claude-bridge`. Uninstall removes only claude-to-codex-owned commands, generated skill wrappers and agents containing the claude-to-codex marker, and the `claude-bridge` MCP entry when it points at `claude-to-codex`.

Uninstall must not remove unrelated Codex config, auth, agents, plugins, MCP entries, hand-written skills, or Claude Code files. All install, uninstall, config, and sync paths are intended to be idempotent for repeated testing.
