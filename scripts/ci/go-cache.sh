#!/usr/bin/env bash
# Keeps a saved Go cache small: only what the run used, without module zips.
#
#   mark-unused  Before the run: date every build cache entry 2000-01-01. Go
#                moves an entry's time to now when it uses one older than an
#                hour, so after the run the entries still dated 2000 are the
#                ones this run never touched.
#   trim         Before saving: delete those untouched entries and the module
#                zips. Go builds from the extracted modules, so the zips
#                (about 175 of a cache's 490 MB) are dead weight. Without the
#                trim, main's caches carried every entry built in five days.
#
# Smaller caches restore faster and keep all the jobs inside the repository's
# 10 GB cache budget (docs/development/ci.md).
#
# Usage: scripts/ci/go-cache.sh mark-unused | trim
set -euo pipefail

readonly build_cache="$HOME/.cache/go-build"
readonly module_downloads="$HOME/go/pkg/mod/cache/download"
readonly unused_time=946684800 # 2000-01-01T00:00:00Z

# Go's build cache entries: action (-a) and output (-d) files.
cache_entries() {
  find "$build_cache" -type f -name '*-[ad]' "$@" 2> /dev/null
}

case "${1:-}" in
  mark-unused)
    cache_entries -exec touch -d "@$unused_time" {} + || true
    ;;
  trim)
    find "$module_downloads" -name '*.zip' -delete 2> /dev/null || true
    cache_entries ! -newermt "@$((unused_time + 1))" -delete || true
    ;;
  *)
    printf 'Usage: %s mark-unused | trim\n' "$0" >&2
    exit 2
    ;;
esac
