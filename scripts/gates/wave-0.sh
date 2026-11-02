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
root=$(cd -- "$script_dir/../.." && pwd)
umask 077
exec python3 "$script_dir/run_gate.py" --wave 0 --log-dir "${GATE_LOG_DIR:-$root/logs/waves/0}" \
    -- "${dry_run[@]}" -- "${build[@]}" -- "${lint[@]}"
