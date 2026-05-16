# One-Shot Setup Prompt

Give this prompt to Claude Code or Codex on the machine where you want to try Codex with an existing Claude Code setup.

```text
I use Claude Code heavily and want to try Codex without losing my existing setup.

Please set up claude-to-codex for me end to end. claude-to-codex is a small Go project at:

https://github.com/lobo235/claude-to-codex

My goal:

- Codex should be able to use my Claude Code MCP servers.
- Codex should pick up my user-scoped Claude skills from ~/.claude/skills when possible.
- Codex should get generated skill wrappers for my Claude slash commands from ~/.claude/commands.
- Project-scoped Claude MCP servers from .mcp.json should work when I launch Codex from that project.

Please do this carefully:

1. Do not print secrets, tokens, auth files, or full environment variables.
2. Check whether git, Go, Node/npm, Claude Code, and Codex are installed.
3. If Codex is not installed, pause and tell me to install it. If npm is available, recommend:

   npm install -g @openai/codex

   If npm is unavailable or the install fails, point me to OpenAI's current Codex CLI install docs and stop until I confirm Codex is installed.

4. Run `codex --version` and `codex login status`.
5. If Codex is not logged in, run `codex login` only when I am present. Pause while I complete the browser/device login flow. If OpenAI says I need account access, subscription, billing, credits, or workspace changes, stop and tell me exactly what manual action is required before continuing.
6. Clone or update the repo at ~/dev/claude-to-codex.
7. Build and test it with make test build.
8. Install claude-to-codex and codex-with-claude into ~/.local/bin.
9. Make sure ~/.local/bin is on my PATH, or tell me the exact shell config line to add.
10. Configure Codex MCP by running:

   codex mcp add claude-bridge -- claude-to-codex serve

   If `claude-bridge` already exists, inspect it with `codex mcp get claude-bridge`. If it is wrong, remove and re-add only that MCP server entry. Preserve all other Codex config.

   If the MCP commands are not supported by my Codex version, safely update ~/.codex/config.toml so it contains:

   [mcp_servers.claude-bridge]
   command = "claude-to-codex"
   args = ["serve"]

   Preserve all existing Codex config.

11. Run:

   claude-to-codex inspect
   claude-to-codex inspect --tools
   claude-to-codex sync-skills
   claude-to-codex sync-commands

12. Explain any child MCP server failures in plain language.
13. Show me how to launch Codex using codex-with-claude from one of my projects.
14. Summarize exactly which files you changed and which commands I should use going forward.

If something is missing, install it only when it is safe and normal for this machine. Otherwise, stop and tell me the exact missing prerequisite.
```

## What Success Looks Like

After the setup works:

```bash
command -v claude-to-codex
command -v codex-with-claude
codex login status
codex mcp get claude-bridge
claude-to-codex inspect --tools
cd /path/to/a/project/with/.mcp.json
codex-with-claude
```

Codex should show a `claude-bridge` MCP server and the bridged tools should be available during a Codex session.
