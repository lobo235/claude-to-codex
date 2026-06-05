# ADR 0001: Public Claude MCP Compatibility Boundary

## Status

Accepted.

## Context

This repository is intended to be a public bridge for people who already
have Claude-shaped MCP configuration and want to use those servers from
Codex. It must not grow project-local or operator-local assumptions.

Real installations can have custom auth headers, token names, domains,
wrappers, or secret-loading conventions. Those belong in the user's shell,
their project `.mcp.json`, their local launcher, or another private
operator layer. The bridge should not know any of those names.

## Decision

`claude-to-codex` implements Claude MCP compatibility, not local
customization.

The public bridge may:

- Parse Claude-shaped `mcpServers` entries.
- Support generic transports: stdio, streamable HTTP, and SSE.
- Pass declared `url`, `headers`, `env`, `command`, and `args` through to
  the configured MCP server.
- Expand `${VAR}` and `$VAR` from the bridge process environment.
- Fail a child server closed when required env references are missing.
- Redact secret-looking values in diagnostics.

The public bridge must not:

- Load credentials from operator-specific files.
- Know project-specific domains, hostnames, token prefixes, database names,
  or secret paths.
- Special-case one user's MCP server behavior.
- Commit local smoke-test artifacts that mention private infrastructure.

## Guardrail

Public code and docs must use generic examples such as `example.com`,
`example.invalid`, `REMOTE_TOOLS_TOKEN`, and `project-tools`. Local
operator-specific smoke-test notes and private identifier scans should
stay untracked or ignored; they must not become required public CI steps.

## Consequences

A user's custom MCP server is supported only through generic config:
the user provides the env vars, headers, URL, command, and args in the
normal Claude MCP shape. If credentials are missing, the relevant child
server is unavailable with a clear redacted diagnostic; other child
servers can still start.
