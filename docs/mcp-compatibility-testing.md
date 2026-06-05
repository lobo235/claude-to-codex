# MCP Compatibility Testing

`claude-bridge` should be tested as an MCP proxy, not only against the
small set of local servers a maintainer happens to use. Compatibility
coverage should verify:

- stdio servers started with `command`, `args`, `env`, and project
  working directories
- streamable HTTP servers with and without headers
- tools, prompts, resources, resource templates, completions, and
  subscriptions
- duplicate tool and prompt names across child servers
- partial failures where one child server is unavailable but others
  still register
- unusual schemas, large tool lists, and long-running tool calls

## Harness

Run the no-secret public smoke matrix:

```sh
scripts/mcp-compat-smoke
```

The script builds `bin/claude-to-codex`, creates an isolated temporary
`HOME`, writes a temporary Claude MCP config, and runs:

```sh
claude-to-codex inspect --tools
```

The built-in npm-based servers are pinned to exact package versions, but
the smoke script still executes registry packages as your local user. It
uses an isolated temporary `HOME`; it is not an OS sandbox.

The built-in matrix currently covers Everything, Chrome DevTools,
Context7, Desktop Commander, Filesystem, Memory, Playwright, and
Sequential Thinking. Expand it with `--config` when testing servers
that need credentials, local databases, Docker, or Python `uvx`.

This avoids modifying `~/.claude.json` or any real project `.mcp.json`.
The harness sets `CLAUDE_BRIDGE_OPERATION_TIMEOUT=180s` by default so
first-run `npx` package downloads do not exhaust the bridge's normal
30-second per-server/per-capability operation budget.
Use a custom matrix with Claude's normal config shape:

```sh
scripts/mcp-compat-smoke --config docs/mcp-compatibility.matrix.example.json
```

For functional validation, build and run the probe. It connects through
`claude-bridge`, lists tools, and calls a curated set of low-risk tools
when present:

```sh
go build -o bin/mcp-compat-probe ./tools/mcp-compat-probe
bin/mcp-compat-probe --help
```

Use a project-scoped fixture when validating a private MCP server:

```sh
scripts/mcp-compat-smoke --project-config /tmp/private-project.mcp.json
```

## Recommended Matrix

The first pass should cover these servers. Sources used to choose them:
the official Model Context Protocol reference server list, Playwright
MCP docs, Context7 docs, MCP.Directory's leaderboard, current Claude
Code MCP guides, and Reddit threads where Claude Code users discuss
their MCP workflows.

| Server | Transport | Why it matters | Secret required |
| --- | --- | --- | --- |
| Everything | stdio | Reference server with mixed capabilities for host/proxy testing. | no |
| Filesystem | stdio | Very common, configurable args, filesystem boundary behavior. | no |
| Memory | stdio | Common persistent local server. | no |
| Sequential Thinking | stdio | Very common Claude workflow server. | no |
| Time | stdio | Small reference server; useful baseline. | no |
| Fetch | stdio | Web fetch and conversion surface. | no |
| Git | stdio | Python/uvx server and repository-scoped args. | no |
| Playwright | stdio | Large browser-automation tool surface and heavier startup. | no |
| Context7 | streamable HTTP or stdio | Popular docs lookup server; exercises HTTP transport. | optional |
| GitHub | stdio or Docker | Very common authenticated API server. | yes |
| Brave Search | stdio | Popular search server; env-var auth. | yes |
| Postgres | stdio | Database URL argument and schema discovery. | usually |
| SQLite | stdio | Local database fixture and query tool surface. | no |
| Google Maps | stdio | Authenticated API server with many typed tools. | yes |
| Slack | stdio | OAuth/token-heavy SaaS integration. | yes |
| Sentry | stdio | SaaS API with organization/project scoping. | yes |
| Supabase | stdio | Popular app/database integration. | yes |
| Chrome DevTools | stdio | Browser/debugging server often compared with Playwright. | no |
| Desktop Commander | stdio | Frequently mentioned local automation server. | no |
| Private project MCP | project stdio | Homegrown project-scoped server used to validate local working directory, env, and private operational tooling. | usually |

## Private Project-Scope Fixture

Some compatibility coverage should come from private or homegrown MCP
servers, especially project-scoped servers that depend on working
directory, environment variables, and deployed services. Keep those
details out of public docs and use a temporary `.mcp.json` fixture:

```json
{
  "mcpServers": {
    "private-project-mcp": {
      "command": "/path/to/private/project/bin/private-mcp",
      "env": {
        "PRIVATE_SERVICE_URL": "https://service.example.com",
        "PRIVATE_SERVICE_TOKEN": "paste-test-token-here"
      }
    }
  }
}
```

Then run:

```sh
scripts/mcp-compat-smoke --project-config /tmp/private-project.mcp.json
```

Expected bridge behavior:

- `projectRoot` is the temporary project root from the harness.
- `servers` includes the project-scope private MCP server.
- `tools` includes the private server's exposed tools.
- missing or invalid service URL / token is reported as a
  child `connect` failure without masking other available servers.

## Failure Triage

Classify failures before changing bridge code:

- `connect`: child command missing, dependency install failed, missing
  env, HTTP URL unreachable, auth/header issue, or stdio startup wrote
  invalid data to stdout.
- `list_tools`: child connected but capability discovery failed.
- missing exposed name: child-prefix name mapping, registration, or schema
  translation issue.
- tool call failure after listing: routing issue, bad argument pass-through,
  child runtime behavior, or timeout handling.

For any regression, reduce it to a fixture that does not require private
user config before adding it to automated tests.

## Sources For Candidate Selection

- Official reference servers:
  https://github.com/modelcontextprotocol/servers
- Playwright MCP:
  https://playwright.dev/docs/getting-started-mcp
- Context7 MCP:
  https://context7.com/docs/installation
- MCP.Directory leaderboard:
  https://mcp.directory/servers/leaderboard
- Claude Code MCP guide:
  https://code.claude.com/docs/en/mcp
- Desktop Commander:
  https://desktopcommander.app/mcp
- Chrome DevTools MCP:
  https://www.npmjs.com/package/chrome-devtools-mcp
- Reddit discussion, Claude Code MCP servers:
  https://www.reddit.com/r/ClaudeCode/comments/1khezrp/mcp_servers/
- Reddit discussion, MCP tools with Claude Code:
  https://www.reddit.com/r/ClaudeAI/comments/1lubtez/what_mcp_tools_you_are_using_with_claude_code/
