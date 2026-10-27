#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf 'Usage: %s backend [--changed | --format-changed] [--base REF] [--fix]\n' "$0"
}

if [[ ${1:-} == --help ]]; then
  usage
  exit 0
fi
if [[ $# -lt 1 ]]; then usage >&2; exit 2; fi
module="$1"
shift
root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
case "$module" in
  backend) module_dir="$root_dir/apps/anchor" ;;
  *) usage >&2; exit 2 ;;
esac
mode=check
action=lint
scope=full
base_ref=origin/main
base_set=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --changed|--format-changed)
      if [[ $scope != full ]]; then usage >&2; exit 2; fi
      scope=changed_packages
      if [[ $1 == --format-changed ]]; then action=format; scope=changed_files; fi
      shift
      ;;
    --base)
      if [[ $# -lt 2 || -z $2 || $2 == -* ]]; then usage >&2; exit 2; fi
      base_ref="$2"
      base_set=true
      shift 2
      ;;
    --fix) mode=fix; shift ;;
    *) usage >&2; exit 2 ;;
  esac
done
if [[ $base_set == true && $scope == full ]]; then usage >&2; exit 2; fi

umask 077
log_dir="$(mktemp -d "${TMPDIR:-/tmp}/anchor-lint.XXXXXX")"
log_file="$log_dir/lint.log"
touch "$log_file"
SECONDS=0
version=unknown
comparison_base=none
fallback=none
targets=(./...)
target_count=1

report() {
  local exit_code="$1" reason="$2" status=FAILED coverage=iteration_only
  if [[ $exit_code == 0 ]]; then status=PASSED; fi
  if [[ $action == lint && $scope == full ]]; then coverage=full; fi
  printf '\nLINT %s\nmodule: %s\nmode: %s\naction: %s\nscope: %s\ncoverage: %s\ntarget_count: %s\nbase: %s\nfallback: %s\nversion: %s\nreason: %s\nexit_code: %s\nelapsed_seconds: %s\nlog: %s\n' \
    "$status" "$module" "$mode" "$action" "$scope" "$coverage" "$target_count" "$comparison_base" "$fallback" "$version" "$reason" "$exit_code" "$SECONDS" "$log_file"
  exit "$exit_code"
}

pin_file="$root_dir/.golangci-lint-version"
if [[ ! -f $pin_file ]]; then
  printf 'Missing version pin: %s\n' "$pin_file" >> "$log_file"
  report 2 setup_error
fi
version="$(cat "$pin_file")"
if [[ ! $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'Expected an exact version such as v2.14.0 in %s\n' "$pin_file" >> "$log_file"
  report 2 setup_error
fi
tool_version() {
  "$1" version 2>> "$log_file" | sed -nE 's/^golangci-lint has version ([^ ]+).*/\1/p' || true
}
cache_bin="${ANCHOR_TOOL_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/anchor}/golangci-lint/$version/golangci-lint"
install_hint="go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$version"
lint_bin=""
installed_version=""
# The pinned cache wins over PATH, so a stale global binary never runs.
if [[ -x $cache_bin ]]; then
  lint_bin="$cache_bin"
elif path_bin="$(command -v golangci-lint)"; then
  installed_version="$(tool_version "$path_bin")"
  if [[ ${installed_version#v} == "${version#v}" ]]; then lint_bin="$path_bin"; fi
fi
if [[ -z $lint_bin ]] && command -v go > /dev/null; then
  printf 'Installing golangci-lint %s into %s\n' "$version" "$(dirname "$cache_bin")" >> "$log_file"
  if (cd "$module_dir" && GOBIN="$(dirname "$cache_bin")" go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$version") >> "$log_file" 2>&1; then
    lint_bin="$cache_bin"
  fi
fi
if [[ -z $lint_bin && -z $installed_version ]]; then
  printf 'Install the pinned tool: %s\n' "$install_hint" >> "$log_file"
  report 2 tool_missing
fi
if [[ -z $lint_bin ]]; then
  printf 'Expected %s, found %s. Install: %s\n' "$version" "$installed_version" "$install_hint" >> "$log_file"
  report 2 version_mismatch
fi
if [[ $lint_bin == "$cache_bin" ]]; then installed_version="$(tool_version "$lint_bin")"; fi
if [[ ${installed_version#v} != "${version#v}" ]]; then
  printf 'Expected %s, found %s in %s. Install: %s\n' "$version" "${installed_version:-unknown}" "$lint_bin" "$install_hint" >> "$log_file"
  report 2 version_mismatch
fi

config="$root_dir/.golangci.yml"
if [[ ! -f $module_dir/go.mod || ! -f $config ]]; then
  printf 'Expected module file %s and lint configuration %s\n' "$module_dir/go.mod" "$config" >> "$log_file"
  report 2 setup_error
fi
if (cd "$module_dir" && "$lint_bin" config verify --config "$config") >> "$log_file" 2>&1; then
  :
else
  report "$?" config_error
fi

use_full_scope() {
  scope=full
  fallback="$1"
  targets=(./...)
  target_count=1
  printf 'Full scope fallback: %s\n' "$fallback" >> "$log_file"
}

add_target() {
  local candidate="$1" existing
  if [[ $target_count -gt 0 ]]; then
    for existing in "${targets[@]}"; do
      if [[ $existing == "$candidate" ]]; then return; fi
    done
  fi
  targets+=("$candidate")
  target_count=$((target_count + 1))
}

select_changes() {
  local resolved_base path relative directory parent nested
  targets=()
  target_count=0
  if ! resolved_base="$(git -C "$root_dir" rev-parse --verify --quiet --end-of-options "$base_ref^{commit}" 2>> "$log_file")"; then
    use_full_scope base_unavailable
    return
  fi
  if ! comparison_base="$(git -C "$root_dir" merge-base HEAD "$resolved_base" 2>> "$log_file")"; then
    comparison_base=none
    use_full_scope base_unavailable
    return
  fi
  if ! git -C "$root_dir" diff --no-ext-diff --no-renames --name-only -z "$comparison_base" -- > "$log_dir/paths" 2>> "$log_file" ||
     ! git -C "$root_dir" diff --cached --no-ext-diff --no-renames --name-only -z "$comparison_base" -- >> "$log_dir/paths" 2>> "$log_file" ||
     ! git -C "$root_dir" ls-files --others --exclude-standard -z -- >> "$log_dir/paths" 2>> "$log_file"; then
    use_full_scope change_detection_failed
    return
  fi
  while IFS= read -r -d '' path; do
    case "$path" in
      .golangci.*|.golangci-lint-version|go.mod|go.sum|go.work|go.work.sum|scripts/*|lint-fix.sh|Makefile|.github/workflows/*)
        use_full_scope validation_inputs_changed; return ;;
      clients/go/*.go|clients/go/go.mod|clients/go/go.sum)
        use_full_scope local_dependency_changed; return ;;
      apps/anchor/*) relative="${path#apps/anchor/}" ;;
      *) continue ;;
    esac
    case "$relative" in
      gorules/*)
        use_full_scope validation_inputs_changed; return ;;
      cmd/http/openapi.yaml|migrations/*|*codegen*.yaml|dbgen.go|generate_anchor.sh)
        use_full_scope generated_inputs_changed; return ;;
    esac
    case "$relative" in
      go.mod|go.sum|*/go.mod|*/go.sum|.golangci.*|*/.golangci.*)
        use_full_scope module_inputs_changed; return ;;
      *.gen.go|*_gen.go|*/gen/*.go|tools.go)
        use_full_scope generated_inputs_changed; return ;;
      *.go) ;;
      *) continue ;;
    esac
    if [[ ! -f $module_dir/$relative || -L $module_dir/$relative ]]; then
      use_full_scope removed_or_linked_go_file
      return
    fi
    if grep -q '^// Code generated .* DO NOT EDIT\.$' "$module_dir/$relative"; then
      use_full_scope generated_inputs_changed
      return
    fi
    directory="$(dirname "$relative")"
    parent="$module_dir/$directory"
    nested=false
    while [[ $parent != "$module_dir" && $parent != "$module_dir/." ]]; do
      if [[ -f $parent/go.mod ]]; then nested=true; break; fi
      parent="$(dirname "$parent")"
    done
    if [[ $nested == true ]]; then continue; fi
    if [[ $action == format ]]; then
      add_target "./$relative"
    elif [[ $directory == . ]]; then
      add_target .
    else
      add_target "./$directory"
    fi
  done < "$log_dir/paths"
}

if [[ $scope != full ]]; then
  select_changes
  if [[ $target_count == 0 ]]; then report 0 no_changes; fi
fi
if [[ $action == format ]]; then
  set -- fmt --config "$config" --color=never
  if [[ $mode == check ]]; then set -- "$@" --diff; fi
else
  set -- run --config "$config" --path-mode=abs --timeout 5m
  if [[ $mode == fix ]]; then set -- "$@" --fix; fi
fi
set -- "$@" "${targets[@]}"
printf 'Checking %s (%s, %s, %s, %s); log: %s\n' "$module" "$version" "$action" "$scope" "$mode" "$log_file"
printf 'Targets:\n' >> "$log_file"
printf '%q\n' "${targets[@]}" >> "$log_file"
if [[ $action == format ]]; then
  (cd "$module_dir" && exec "$lint_bin" "$@") > "$log_dir/format.diff" 2>> "$log_file" &
else
  (cd "$module_dir" && exec "$lint_bin" "$@") >> "$log_file" 2>&1 &
fi
lint_pid=$!

trap 'kill -TERM "$lint_pid" 2>/dev/null || true; wait "$lint_pid" 2>/dev/null || true; report 130 interrupted' INT
trap 'kill -TERM "$lint_pid" 2>/dev/null || true; wait "$lint_pid" 2>/dev/null || true; report 143 interrupted' TERM
if wait "$lint_pid"; then exit_code=0; else exit_code=$?; fi
trap - INT TERM
if [[ $action == format ]]; then
  cat "$log_dir/format.diff" >> "$log_file"
  if [[ $exit_code == 0 && $mode == check && -s $log_dir/format.diff ]]; then exit_code=1; fi
fi

case "$exit_code" in
  0) reason=none ;;
  1) reason=findings ;;
  4) reason=timeout ;;
  5) reason=no_go_files ;;
  6) reason=config_error ;;
  130|143) reason=interrupted ;;
  *)
    reason=analysis_error
    if [[ $exit_code == 3 ]] && grep -q 'parallel golangci-lint is running' "$log_file"; then
      reason=already_running
    fi
    ;;
esac
report "$exit_code" "$reason"
