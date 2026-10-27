#!/usr/bin/env bash
# Claude Code PreToolUse hook: refuses sleep-and-check CI polling loops.
set -uo pipefail
command="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("tool_input",{}).get("command",""))' 2>/dev/null)"
if grep -Eq '(^|[;&|[:space:]])sleep[[:space:]]+[0-9]' <<< "$command" \
  && grep -Eq 'gh[[:space:]]+(run|pr[[:space:]]+checks|api)' <<< "$command"; then
  echo 'CI polling loop refused. Keep working while CI runs: the desktop CI monitor wakes the session on a failure; elsewhere run `gh pr checks <n> --watch --fail-fast` once with run_in_background. See docs/development/agent-workflow.md.' >&2
  exit 2
fi
exit 0
