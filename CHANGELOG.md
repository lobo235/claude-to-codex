# Changelog

## [Unreleased]

## [v0.4.1] - 2026-06-15

### Fixed

- Load Claude local-scope MCP entries stored under the active project in
  `~/.claude.json`, include their environment references in
  `bridge-env-vars`, and let them override same-name user-scoped MCP
  entries before falling back to project `.mcp.json` precedence.
- Fail closed on ambiguous equivalent local-scope project paths and pass
  `CLAUDE_BRIDGE_PROJECT_ROOT` to Claude local-scope stdio MCP servers.

## [v0.4.0] - 2026-06-13

### Added

- Expand the built-in no-secret MCP compatibility smoke matrix to include
  Fetch, Time, Git, and SQLite via pinned `uvx` packages, and add
  `--probe` for optional functional tool-call validation.

### Changed

- Generate Codex skill frontmatter and agent descriptions through an
  isolated, tool-free `codex exec` invocation that ignores user Codex config
  and inherits only a restricted baseline environment.

## [v0.3.5] - 2026-06-09

### Added

- Add an internal sanitized public mirror publishing flow and move
  self-hosted CI configuration under `.internal`.

### Changed

- Sanitize public mirror terminology used in release tooling and tests.

### Fixed

- Honor active `CODEX_HOME` when syncing Claude user skills, slash
  commands, and agents into generated Codex artifacts.

## [v0.3.4] - 2026-06-09

### Added

- Add `sync-artifacts` for headless automation workers: generated Claude
  skills, slash commands, user agents, and project agents are refreshed in
  a schema/versioned persistent cache and materialized into allocation-local
  `CODEX_HOME`.
- Have `cwc` use `sync-artifacts` automatically when
  `CLAUDE_TO_CODEX_ARTIFACT_CACHE_DIR` is set.

### Security

- Keep Codex auth, config, sessions, logs, and project `.codex` runtime
  state out of the automation artifact cache.

## [v0.3.3] - 2026-06-09

### Fixed

- Preserve the Codex `claude-bridge` command and arguments when `cwc`
  injects per-session `env_vars`, so the bridge remains usable in fresh
  sessions that need project-scoped MCP environment forwarding.

## [v0.3.2] - 2026-06-06

### Changed

- Refresh MCP docs for targeted probe calls, child `tools/call` timeout
  and error wrapping behavior, and fresh-session requirements after
  environment or project-root changes.

## [v0.3.1] - 2026-06-06

### Added

- Add targeted `mcp-compat-probe -call` support for replaying specific
  child MCP tool calls through `claude-to-codex serve`.
- Add regression coverage for Codex-launched project SSE servers whose
  headers depend on environment variables forwarded into the
  `claude-bridge` process.

### Changed

- Apply the bridge child-operation timeout to proxied child `tools/call`
  requests.
- Wrap child `tools/call` failures with project/user scope, child server
  name, original tool name, exposed tool name, and actionable hints for
  missing env vars, auth failures, timeouts, and closed SSE connections.
- Document that `cwc` forwards variable names, not secret values, and
  that token, env file, or project root changes require a fresh Codex
  session.

## [v0.3.0] - 2026-06-05

### Added

- Add Claude-style SSE MCP transport support with optional HTTP headers.
- Add `${VAR}` and `$VAR` expansion for Claude MCP server config strings,
  including HTTP/SSE headers, while failing only the affected child server
  when a required variable is missing.
- Add `bridge-env-vars` to compute the Codex `env_vars` list needed by
  `claude-bridge` for the active Claude MCP configuration.
- Add ADR 0001 documenting the public Claude MCP compatibility boundary.

### Changed

- Have `cwc` pass a session-scoped
  `mcp_servers.claude-bridge.env_vars` override to Codex so
  project-scoped MCP config and header variables reach the
  Codex-managed `claude-bridge` process.
- Always expose bridged tools and prompts with the child server name
  prefix so Codex-visible tool names preserve their Claude MCP origin.
- Update setup, architecture, cold-start, troubleshooting, and MCP docs
  for SSE, env forwarding, child-prefixed tool names, and the public
  compatibility boundary.

### Security

- Redact raw token-like and JWT-like values in diagnostics.
- Sanitize all Claude skill and agent metadata before sending bounded
  description-generation prompts to `codex exec`, including private-looking
  domains, absolute paths, and credential-like environment variable names.
- Sanitize generated and source agent descriptions before writing generated
  Codex agent TOML.
- Require Go 1.26.4 so release builds use a toolchain with the current
  standard-library vulnerability fixes.
- Keep local private-boundary scanner scripts ignored so operator-specific
  checks do not leak into public release artifacts.

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
- Pin public and self-hosted workflow actions to commit SHAs.
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
