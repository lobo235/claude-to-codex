# Changelog

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
