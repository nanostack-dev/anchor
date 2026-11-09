#!/usr/bin/env bash
# Claude Code PostToolUse hook: formats the file an Edit/Write just changed. Never blocks.
set -uo pipefail
input="$(cat)"
file="$(printf '%s' "$input" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("tool_input",{}).get("file_path",""))' 2>/dev/null)"
root="$(git -C "$(dirname "$file")" rev-parse --show-toplevel 2>/dev/null)" || exit 0
[[ -f $file && -f $root/scripts/lint.sh ]] || exit 0
format_go() {
  local bin module="$root/apps/anchor"
  # Only the linter pinned in the tool module formats, as scripts/lint.sh runs it.
  bin="$(cd "$root/apps/anchor" && GOWORK=off go tool -modfile=tools/golangci-lint/go.mod -n golangci-lint 2>/dev/null)" || return 0
  [[ -x $bin ]] || return 0
  [[ $file == "$root"/clients/go/* ]] && module="$root/clients/go"
  (cd "$module" && "$bin" fmt --config "$root/.golangci.yml" "$file") > /dev/null 2>&1
}
case "$file" in
  *.go) format_go ;;
  "$root"/anchor-ui/*.ts | "$root"/anchor-ui/*.tsx)
    biome="$root/anchor-ui/node_modules/@biomejs/biome/bin/biome"
    [[ -f $biome ]] && (cd "$root/anchor-ui" && node "$biome" format --write --no-errors-on-unmatched "$file") > /dev/null 2>&1 ;;
esac
exit 0
