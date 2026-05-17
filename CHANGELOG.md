# Changelog

## [Unreleased]

## [v0.2.0] - 2026-05-16

### Added

- Add `sync-agents` to bridge Claude Code agents into Codex subagent TOML
  files.
- Sync user-scoped Claude agents from `~/.claude/agents/*.md` to
  `~/.codex/agents/*.toml`.
- Sync project-scoped Claude agents from `.claude/agents/*.md` to
  `.codex/agents/*.toml` when launched through `cwc`.
- Show Claude agent and generated Codex agent counts in `cwc --status`
  and `cwc --doctor`.

### Changed

- Run `sync-agents` during `cwc` startup after skill sync and before
  slash-command sync.
- Generate skill and agent descriptions from Claude metadata plus a
  bounded, sanitized preview instead of full source bodies.
- Keep generated Codex agent filenames hyphenated while using
  underscore-style Codex agent `name` values.
- Pin MCP compatibility smoke-test npm packages to exact versions.
- Pin GitHub and self-hosted workflow actions to commit SHAs.
- Update setup, architecture, cold-start, troubleshooting, and MCP docs
  for agent sync and the hardened metadata generation path.

### Security

- Reject symlinked generated Codex agent TOML targets.
- Reject symlinked generated Codex skill `SKILL.md` targets for skill and
  slash-command sync.
- Redact sensitive-looking unknown Claude agent frontmatter values in
  generated Codex agent TOML.
- Use relative source paths for generated project agent TOML.
- Redact URL path segments following sensitive names such as `token`,
  `key`, `secret`, and `signature`.
- Use `mktemp` for temporary uninstall MCP inspection files.

## v0.1.1 - 2026-05-16

### Security

- Restrict stdio MCP child process environments by default. Child MCPs
  receive only a small non-secret baseline plus explicit per-server
  `env` values from Claude MCP config.
- Add per-server env inheritance escape hatch:
  `"x-claude-bridge-inherit-env": true`.
- Add global compatibility escape hatch:
  `CLAUDE_BRIDGE_INHERIT_ENV=1`.
- Pass `CLAUDE_BRIDGE_PROJECT_ROOT` only to project-scoped child MCP
  servers.
- Redact common token, key, secret, password, authorization,
  credential, session, and signature patterns from diagnostic URLs and
  error output.
- Document `.mcp.json` as trusted code and explain authenticated MCP
  migration guidance.

## v0.1.0 - 2026-05-16

Initial tagged release.

### Added

- `claude-to-codex` maintenance CLI and `claude-bridge` MCP server.
- `cwc` / `codex-with-claude` launcher that syncs Claude Code skills and slash commands before starting Codex.
- User-scoped and project-scoped Claude MCP loading from `~/.claude.json` and project `.mcp.json`.
- MCP proxying for tools, prompts, resources, resource templates, completions, subscriptions, and resource reads.
- Tool and prompt collision handling through server-name prefixes.
- Diagnostics: `doctor`, `status`, `smoke-test`, and `inspect --tools`.
- Safe install and uninstall workflows that preserve unrelated Codex and Claude Code state.
- Generated Codex skill wrappers for Claude skills and slash commands, with source hashes for idempotent sync.
- Broad MCP compatibility smoke harness and documentation.
- Per-server/per-capability MCP operation timeouts via `CLAUDE_BRIDGE_OPERATION_TIMEOUT`.
- Gitleaks pre-commit configuration and `make secrets`.

### Verified

- Unit tests, linting, vulnerability scan, secret scan, shell syntax checks, and broad public MCP smoke testing pass for this release.
