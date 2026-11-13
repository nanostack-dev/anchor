#!/usr/bin/env bash
# Run the Anchor backend of this checkout against the dev data services.
#
#   scripts/run-dev.sh
#
# The config comes from .env.dev in the primary checkout, which
# infra/scripts/dev-env.sh renders from 1Password: Postgres and Redis on the dev
# node over Tailscale, migrations off. When the file is missing, this script
# runs dev-env.sh first; that needs your own 1Password login. ENV_FILE picks
# another file. Variables already set in your shell win over the file.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
primary="$(git -C "$ROOT_DIR" worktree list --porcelain | sed -n '1s/^worktree //p')"
env_file="${ENV_FILE:-$primary/.env.dev}"

if [[ ! -f "$env_file" ]]; then
  dev_env="${NANOSTACK_INFRA_DIR:-$(dirname "$primary")/infra}/scripts/dev-env.sh"
  if [[ -n "${ENV_FILE:-}" || ! -x "$dev_env" ]]; then
    printf 'No env file at %s. Render it with infra/scripts/dev-env.sh anchor.\n' "$env_file" >&2
    exit 1
  fi
  (cd "$ROOT_DIR" && "$dev_env" anchor)
fi

# shellcheck disable=SC2016 # $1 expands in the child bash.
while IFS= read -r -d '' entry; do
  name="${entry%%=*}"
  [[ "$name" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || continue
  if [[ -z "${!name+set}" ]]; then
    export "${entry?}"
  fi
done < <(env -i HOME="$HOME" PATH="$PATH" bash -c 'set -a; . "$1"; set +a; env -0' _ "$env_file")

# The dev config allows only the dev web origins; anchor-ui runs on 3000.
export ALLOWED_ORIGIN="${ALLOWED_ORIGIN:-http://localhost:3000,http://127.0.0.1:3000}"

printf 'Anchor against dev (%s) on port %s\n' "$env_file" "${SERVER_PORT:-8080}"
cd "$ROOT_DIR/apps/anchor"
exec go run ./cmd
