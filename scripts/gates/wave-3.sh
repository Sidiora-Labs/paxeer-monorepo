#!/usr/bin/env bash
set -euo pipefail

targets=(workspace-build workspace-test agent-test interop-test programs-test)

if [[ $# -eq 1 && $1 == --list ]]; then
    printf 'make %s\n' "${targets[@]}"
    exit 0
fi
if [[ $# -ne 0 ]]; then
    printf 'usage: %s [--list]\n' "$0" >&2
    exit 2
fi

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
commands=()
for target in "${targets[@]}"; do
    commands+=(-- make "$target")
done
log_dir=$(mktemp -d "${TMPDIR:-/tmp}/layerx-wave-3.XXXXXXXX")
exec python3 "$root/scripts/gates/run_gate.py" --wave 3 --log-dir "$log_dir" "${commands[@]}"
