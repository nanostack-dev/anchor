#!/usr/bin/env bash
# Claude Code PostToolUse hook: formats the file an Edit/Write just changed. Never blocks.
set -uo pipefail
input="$(cat)"
file="$(printf '%s' "$input" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("tool_input",{}).get("file_path",""))' 2>/dev/null)"
root="$(git -C "$(dirname "$file")" rev-parse --show-toplevel 2>/dev/null)" || exit 0
[[ -f $file && -f $root/scripts/lint.sh ]] || exit 0
format_go() {
  local version bin module="$root/apps/anchor"
  version="$(cat "$root/.golangci-lint-version")"
  bin="${ANCHOR_TOOL_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/anchor}/golangci-lint/$version/golangci-lint"
  if [[ ! -x $bin ]]; then
    bin="$(command -v golangci-lint)" || return 0
    # Only the pinned version formats; scripts/lint.sh installs it otherwise.
    "$bin" version 2>/dev/null | grep -q "version ${version#v} " || return 0
  fi
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
