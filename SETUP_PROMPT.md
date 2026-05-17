# One-Shot Setup Prompt

Give this prompt to Claude Code or Codex on the machine where you want to try Codex with an existing Claude Code setup.

```text
I use Claude Code heavily and want to try Codex without losing my existing setup.

Please set up claude-to-codex for me end to end. claude-to-codex is a small Go project at:

https://github.com/lobo235/claude-to-codex

If I gave you only the repository URL and asked you to "install this", follow the README, use this prompt as the checklist, and do not stop until the "What Success Looks Like" checks pass or a manual prerequisite blocks progress.

My goal:

- Codex should be able to use my Claude Code MCP servers.
- Codex should pick up my user-scoped Claude skills from ~/.claude/skills when possible.
- Codex should pick up my user-scoped Claude agents from ~/.claude/agents when possible.
- Codex should get generated skill wrappers for my Claude slash commands from ~/.claude/commands.
- Project-scoped Claude agents from .claude/agents should work when I launch Codex with `cwc` from that project.
- Project-scoped Claude MCP servers from .mcp.json should work when I launch Codex with `cwc` from that project.

Names to use consistently:

- `cwc` is the daily command I type instead of `codex`.
- `claude-to-codex` is the maintenance CLI and MCP server binary.
- `claude-bridge` is the Codex MCP server entry that runs `claude-to-codex serve`.
- `codex-with-claude` is only the long-form alias for `cwc`.

Please do this carefully. This setup must be idempotent: it should be safe to rerun after partial setup, after an upgrade, or after Codex reports skill-loading warnings. Preserve existing user configuration unless a step explicitly says to replace a claude-to-codex-owned file or the `claude-bridge` MCP entry. Do not delete or rewrite unrelated Codex config, auth, agents, plugins, MCP entries, or hand-written skills.

1. Do not print secrets, tokens, auth files, or full environment variables.
2. Check whether git, Go, Node/npm, Claude Code, and Codex are installed.
3. If Codex is not installed, pause and tell me to install it. If npm is available, recommend:

   npm install -g @openai/codex

   If npm is unavailable or the install fails, point me to OpenAI's current Codex CLI install docs and stop until I confirm Codex is installed.

4. Run `codex --version` and `codex login status`.
5. If Codex is not logged in, run `codex login` only when I am present. Pause while I complete the browser/device login flow. If OpenAI says I need account access, subscription, billing, credits, or workspace changes, stop and tell me exactly what manual action is required before continuing.
6. Clone or update the repo at ~/dev/claude-to-codex.
7. Build and test it with make test build.
8. Install `claude-to-codex`, `cwc`, and `codex-with-claude` into ~/.local/bin.
9. Make sure ~/.local/bin is on my PATH, or tell me the exact shell config line to add.
10. Configure Codex MCP by running:

   codex mcp add claude-bridge -- claude-to-codex serve

   MCP lets Codex talk to external tools. `claude-bridge` is the Codex MCP entry that runs `claude-to-codex`. If I need more context, show me `docs/mcp-and-claude-bridge.md`.

   If `claude-bridge` already exists, inspect it with `codex mcp get claude-bridge`. If it already points at `claude-to-codex serve`, leave it alone. If it points somewhere else, stop and ask me before replacing it because it may be user-owned. Preserve all other Codex config.

   If the MCP commands are not supported by my Codex version, safely update ~/.codex/config.toml so it contains:

   [mcp_servers.claude-bridge]
   command = "claude-to-codex"
   args = ["serve"]

   Preserve all existing Codex config.

11. Run:

   claude-to-codex inspect
   claude-to-codex inspect --tools
   claude-to-codex sync-skills
   claude-to-codex sync-agents
   claude-to-codex sync-commands
   cwc --doctor
   cwc --status
   cwc --smoke-test

   Then verify the generated Codex skills and agents:

   - Claude skills from `~/.claude/skills/<name>/SKILL.md` should appear as generated wrappers at `~/.codex/skills/<name>/SKILL.md`.
   - Claude agents from `~/.claude/agents/*.md` should appear as generated TOML at `~/.codex/agents/*.toml`.
   - Project Claude agents from `.claude/agents/*.md` should appear as generated TOML at `.codex/agents/*.toml` when using `cwc` from that project.
   - Generated wrappers must start with valid Codex YAML frontmatter delimited by `---`, including at least `name` and `description`.
   - Generated agents must be valid Codex TOML with `name`, `description`, and `developer_instructions`.
   - `sync-skills` should use Codex headless mode (`codex exec`) with a fast model to generate useful descriptions from Claude metadata and a bounded, sanitized preview. If I need a different fast model, set `CLAUDE_TO_CODEX_FRONTMATTER_MODEL`.
   - `sync-agents` may use the same bounded-preview generation path to generate useful descriptions for sparse Claude agents.
   - Generated skill wrappers and agents should include a source hash so rerunning setup does not rewrite unchanged artifacts.
   - Do not symlink raw Claude skill directories into `~/.codex/skills`; raw Claude skill frontmatter may be missing or use YAML that Codex rejects.
   - If Codex reports "Skipped loading skill(s) due to invalid SKILL.md files", inspect each named file. If it was generated by `claude-to-codex` or is an older symlink created by this project, rerun `claude-to-codex sync-skills` after updating this repo; if it is hand-written, report the exact frontmatter issue and leave it for me to approve.

12. Explain any child MCP server failures in plain language.
13. Show me the short-form commands: `cwc`, `cwc --doctor`, `cwc --status`, `cwc --smoke-test`, `cwc --version`, `cwc --help`, `cwc --install`, and `cwc --uninstall --dry-run`.
14. Show me how to uninstall safely with `cwc --uninstall --dry-run` and `cwc --uninstall --yes`. Explain that uninstall removes only claude-to-codex-owned commands, generated skill wrappers, generated agents, and the `claude-bridge` MCP entry when it points at `claude-to-codex`.
15. Summarize exactly which files you changed and which commands I should use going forward.

If something is missing, install it only when it is safe and normal for this machine. Otherwise, stop and tell me the exact missing prerequisite.
```

## What Success Looks Like

After the setup works:

```bash
command -v claude-to-codex
command -v cwc
codex login status
codex mcp get claude-bridge
cd /path/to/a/project/with/.mcp.json
cwc --doctor
cwc --status
cwc --smoke-test
cwc
```

Codex should show a `claude-bridge` MCP server and the Claude MCP tools should be available during a Codex session.
