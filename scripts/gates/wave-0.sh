#!/usr/bin/env bash
set -euo pipefail

dry_run=(make -n build test)
build=(make workspace-build)
lint=(make workspace-lint)

if [[ $# -eq 1 && $1 == --list ]]; then
    printf '%s\n' "${dry_run[*]}" "${build[*]}" "${lint[*]}"
    exit 0
fi
if [[ $# -ne 0 ]]; then
    printf 'Usage: %s [--list]\n' "$0" >&2
    exit 2
fi

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
umask 077
log_dir=$(mktemp -d "${TMPDIR:-/tmp}/wave-0.XXXXXXXX")
exec python3 "$script_dir/run_gate.py" --wave 0 --log-dir "$log_dir" \
    -- "${dry_run[@]}" -- "${build[@]}" -- "${lint[@]}"
