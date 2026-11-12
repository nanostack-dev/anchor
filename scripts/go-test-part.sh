#!/usr/bin/env bash
# Runs one part of a Go package's top-level tests, so one suite can split over
# several CI runners (docs/development/ci.md).
#
# Usage:   scripts/go-test-part.sh <package dir> <part> <parts>
# Example: GO_TEST_TIMES=scripts/go-test-times.txt \
#            scripts/go-test-part.sh apps/anchor/cmd/it/ct 0 16
#
# Environment:
#   GO_TEST_TIMES  file of "<test> <seconds>" lines. With it, the parts are
#                  balanced by time; without it, part p takes every <parts>th
#                  test from offset p.
#   GO_TEST_FLAGS  extra `go test` flags (CI sets -parallel 8).
#
# The part fails unless every selected test ran, so a name the -run filter
# misses can never pass silently.
set -euo pipefail

readonly gotestsum=gotest.tools/gotestsum@v1.13.0
# A test missing from GO_TEST_TIMES counts this many seconds.
readonly default_seconds=0.5

usage() {
  printf 'Usage: %s <package dir> <part> <parts>\n' "$0" >&2
  exit 2
}

# Prints every top-level test name of the package in the current directory.
# The names come from the sources: `go test -list` would run TestMain, which
# starts the containers.
list_tests() {
  grep -hoE '^func Test[A-Za-z0-9_]+\(t \*testing\.T\)' ./*_test.go |
    sed -E 's/^func (Test[A-Za-z0-9_]+).*/\1/' |
    sort -u
}

# Reads test names on stdin; prints "<seconds> <test>" for each, from the
# times file.
add_seconds() {
  local times_file="$1"
  awk -v times="$times_file" -v fallback="$default_seconds" '
    BEGIN {
      while ((getline line < times) > 0) {
        if (line ~ /^#/) continue
        split(line, field, " ")
        seconds[field[1]] = field[2]
      }
    }
    { print (($0 in seconds) ? seconds[$0] : fallback), $0 }'
}

# Reads "<seconds> <test>" lines on stdin and prints the tests of one part.
# Longest test first, each test goes to the part with the least time so far
# (the lowest part number on a tie), so every part takes about as long.
pick_by_time() {
  local part="$1" parts="$2"
  LC_ALL=C sort -k1,1gr -k2,2 |
    awk -v parts="$parts" -v part="$part" '
      {
        lightest = 0
        for (i = 1; i < parts; i++) if (load[i] < load[lightest]) lightest = i
        load[lightest] += $1
        if (lightest == part) print $2
      }'
}

# Reads test names on stdin and prints every <parts>th one from offset <part>.
pick_round_robin() {
  local part="$1" parts="$2"
  awk -v parts="$parts" -v part="$part" 'NR % parts == part'
}

# Prints the slow tests, then every test's work (its own time plus its direct
# subtests') as WORK lines: scripts/go-test-times.txt is regenerated from
# those lines (its header has the command).
report_times() {
  local json="$1"
  printf 'top-level tests over 5 s:\n'
  go run "$gotestsum" tool slowest --jsonfile "$json" --threshold 5s | awk '$2 !~ /\//' || true

  if ! command -v jq > /dev/null; then return; fi
  if [[ -n ${GITHUB_ACTIONS:-} ]]; then printf '::group::work per test\n'; fi
  jq -rs '
    [.[] | select(.Action == "pass" and .Test)]
    | group_by(.Test | split("/")[0])[]
    | "WORK \(.[0].Test | split("/")[0]) \(map(select(.Test | split("/") | length <= 2) | .Elapsed) | add)"
  ' "$json" || true
  if [[ -n ${GITHUB_ACTIONS:-} ]]; then printf '::endgroup::\n'; fi
}

main() {
  if [[ $# -ne 3 || ! $2 =~ ^[0-9]+$ || ! $3 =~ ^[1-9][0-9]*$ || $2 -ge $3 ]]; then
    usage
  fi
  local dir="$1" part="$2" parts="$3"

  # Resolve the times file before leaving the current directory.
  local times_file=""
  if [[ -n ${GO_TEST_TIMES:-} ]]; then
    times_file="$(cd "$(dirname "$GO_TEST_TIMES")" && pwd)/$(basename "$GO_TEST_TIMES")"
  fi
  cd "$dir"

  local selected
  if [[ -n $times_file ]]; then
    selected="$(list_tests | add_seconds "$times_file" | pick_by_time "$part" "$parts")"
  else
    selected="$(list_tests | pick_round_robin "$part" "$parts")"
  fi
  if [[ -z $selected ]]; then
    printf 'No tests selected in %s, part %s of %s\n' "$dir" "$part" "$parts" >&2
    exit 1
  fi

  local -a flags=()
  read -ra flags <<< "${GO_TEST_FLAGS:-}"
  local pattern json rc=0
  pattern="^($(paste -sd'|' - <<< "$selected"))\$"
  json="$(mktemp)"
  go run "$gotestsum" --format pkgname-and-test-fails --jsonfile "$json" -- \
    ${flags[@]+"${flags[@]}"} -run "$pattern" . || rc=$?

  # A top-level test that passed or skipped has one such line.
  local want ran
  want="$(wc -l <<< "$selected")"
  ran="$(grep -cE '"Action":"(pass|skip)","Package":"[^"]*","Test":"Test[^/"]*"' "$json" || true)"
  printf 'ran %s of %s selected tests\n' "$ran" "$want"

  report_times "$json"

  if [[ $ran -ne $want ]]; then exit 1; fi
  exit "$rc"
}

main "$@"
