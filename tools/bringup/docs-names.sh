#!/usr/bin/env bash
set -euo pipefail

cd "$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
spec=spec/layerx-merge/spec.kvx
inventory=docs/site/docs/reference/public-rpc.md
docs=(docs docs/platform README.md ':(glob)sdk/platform/**/package-info.java' ':(glob)sdk/platform/**/README.md')
mapfile -d '' -t files < <(git ls-files -z -- "${docs[@]}")
[ "${#files[@]}" -gt 0 ] || { echo 'docs-names: no tracked documentation' >&2; exit 1; }
"${BRINGUP_INSPECT_BIN:-bringup-inspect}" docs-names "$inventory" "$spec" "${files[@]}"
