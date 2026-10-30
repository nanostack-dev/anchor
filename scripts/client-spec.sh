#!/usr/bin/env bash
# Writes the client spec: openapi.yaml without its server-only Go hints, so a
# generated client uses plain types.
#
# - Delete each `x-go-type:` except `x-go-type: string`.
# - Delete each `x-go-type-import:` with its `path:` child (N;d), so no
#   orphaned mapping is left behind.
# - A server-only nullable.Nullable hint also drops the
#   x-go-type-skip-optional-pointer line right after it, so the client keeps a
#   pointer that can send null.
#
# Usage: scripts/client-spec.sh <openapi.yaml> <output.yaml>
set -euo pipefail

sed -e '/x-go-type: nullable\./{N;/x-go-type-skip-optional-pointer:/d;}' \
  -e '/x-go-type: string/!{/x-go-type:/d;}' \
  -e '/x-go-type-import:/{N;d;}' "$1" >"$2"
