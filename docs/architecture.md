# Architecture

`claude-to-codex` is intentionally small. It has three responsibilities:

1. Load Claude MCP configuration.
2. Proxy MCP capabilities from child servers to Codex.
3. Mirror Claude user and project artifacts into Codex entries.

## Naming

- `cwc`: the daily launcher users type instead of `codex`
- `claude-to-codex`: the maintenance CLI and MCP server binary
- `claude-bridge`: the Codex MCP server entry that runs `claude-to-codex serve`
- `codex-with-claude`: the long-form alias for `cwc`

## CLI Commands

```text
claude-to-codex serve
claude-to-codex inspect [--tools]
claude-to-codex sync-skills
claude-to-codex sync-agents [--project <dir>|--project-only <dir>]
claude-to-codex sync-commands
claude-to-codex version
```

`serve` is the command Codex runs for the `claude-bridge` MCP server.

`inspect` is for diagnostics and setup validation.

`sync-skills`, `sync-agents`, and `sync-commands` are safe to run repeatedly.

## Config Loading

`claude-to-codex` loads:

- user-scoped MCP servers from `~/.claude.json`
- project-scoped MCP servers from `<project>/.mcp.json`

Project root detection prefers `CLAUDE_BRIDGE_PROJECT_ROOT`. If it is unset, `claude-to-codex` walks upward from the current working directory looking for `.mcp.json`, `CLAUDE.md`, or `.git`.

Project-scoped stdio MCP servers run with their working directory set to the detected project root.

## MCP Child Environment

Stdio child MCP servers run with a restricted environment by default.
The bridge passes a small non-secret baseline (`PATH`, `HOME`, user,
shell, temp, locale, and certificate variables), then overlays the
server's explicit Claude MCP `env` values. Project-scoped servers also
receive `CLAUDE_BRIDGE_PROJECT_ROOT`.

Ambient shell secrets are not inherited by default. Compatibility
escape hatches are available:

- per-server: `"x-claude-bridge-inherit-env": true`
- global: `CLAUDE_BRIDGE_INHERIT_ENV=1`

Diagnostic output redacts common token, key, secret, password,
authorization, credential, and signature patterns from URLs and errors.
HTTP MCP headers and stdio env values are not printed.

## MCP Proxying

Each Claude MCP server becomes a child MCP client session. `claude-to-codex` registers one Codex-facing MCP server and forwards requests to the correct child.

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

Claude skills are converted into generated Codex-compatible skill wrappers. The generated file supplies valid Codex frontmatter, points back to the Claude `SKILL.md`, and includes a source snapshot so Claude skills with missing or incompatible frontmatter still load in Codex.

`sync-skills` asks `codex exec` to generate a concise, useful `description` with a fast model. The generator receives only Claude metadata and a bounded, sanitized preview, not the full skill body, and it runs from a temporary directory. Set `CLAUDE_TO_CODEX_FRONTMATTER_MODEL` to override the default model, or `CLAUDE_TO_CODEX_DISABLE_CODEX_FRONTMATTER=1` to use deterministic fallback descriptions. Generated wrappers include `source-sha256`; if the source hash is unchanged, the wrapper is left untouched.

When a skill wrapper needs generated frontmatter, `sync-skills --quiet` still writes progress to stderr so `cwc` is not silent during slow first-run work. It stays silent for unchanged wrappers.

Claude agents are converted into generated Codex subagent TOML files. User agents from `~/.claude/agents/*.md` sync to `~/.codex/agents/*.toml`. Project agents from `<project>/.claude/agents/*.md` sync to `<project>/.codex/agents/*.toml` when `sync-agents --project <dir>` runs through `cwc`.

Generated agent filenames keep Claude's hyphenated filename, while the Codex `name` field uses underscores. Claude `tools` frontmatter is preserved as advisory instruction text, not converted into Codex permissions. Generated agents omit `model`, `model_reasoning_effort`, and `sandbox_mode` so they inherit the active Codex session.

`sync-agents` updates and deletes only generated TOML files containing the claude-to-codex marker. Hand-written Codex agents are skipped. Sparse agent descriptions use the same bounded preview generation path as skills; slow generation writes progress to stderr in quiet mode.

Claude slash commands are converted into generated Codex skill wrappers. Generated files include a marker so `claude-to-codex` can update its own output without overwriting hand-written Codex skills.

## Install And Uninstall Boundaries

Install writes only `claude-to-codex` and launcher commands into `PREFIX/bin` and tells the user how to configure `claude-bridge`. Uninstall removes only claude-to-codex-owned commands, generated skill wrappers and agents containing the claude-to-codex marker, and the `claude-bridge` MCP entry when it points at `claude-to-codex`.

Uninstall must not remove unrelated Codex config, auth, agents, plugins, MCP entries, hand-written skills, or Claude Code files. All install, uninstall, config, and sync paths are intended to be idempotent for repeated testing.
