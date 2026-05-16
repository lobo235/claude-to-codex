#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
prefix="${PREFIX:-$HOME/.local}"
bin_dir="$prefix/bin"

if ! command -v go >/dev/null 2>&1; then
  printf 'install.sh: Go is required but was not found on PATH.\n' >&2
  exit 1
fi

make -C "$repo_dir" test build

install -d "$bin_dir"
install -m 0755 "$repo_dir/bin/claude-to-codex" "$bin_dir/claude-to-codex"
install -m 0755 "$repo_dir/scripts/codex-with-claude" "$bin_dir/codex-with-claude"

printf 'Installed:\n'
printf '  %s\n' "$bin_dir/claude-to-codex"
printf '  %s\n' "$bin_dir/codex-with-claude"

case ":$PATH:" in
  *":$bin_dir:"*) ;;
  *)
    printf '\nAdd this to your shell config if it is not already present:\n'
    if [[ "$bin_dir" == "$HOME/.local/bin" ]]; then
      printf '  export PATH="$HOME/.local/bin:$PATH"\n'
    else
      printf '  export PATH="%s:$PATH"\n' "$bin_dir"
    fi
    ;;
esac

if ! command -v codex >/dev/null 2>&1; then
  printf '\nWarning: codex was not found on PATH. Install Codex before using codex-with-claude.\n'
fi

printf '\nConfigure Codex MCP with:\n\n'
printf 'codex mcp add claude-bridge -- claude-to-codex serve\n'
printf '\nOr add this to ~/.codex/config.toml if the Codex MCP command is unavailable:\n\n'
printf '[mcp_servers.claude-bridge]\n'
printf 'command = "claude-to-codex"\n'
printf 'args = ["serve"]\n'
