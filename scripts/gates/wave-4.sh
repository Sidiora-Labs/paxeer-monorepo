#!/usr/bin/env bash
set -euo pipefail

targets=(
    workspace-build
    workspace-test
    core-qualification-environment
    beta-driver-test
)

if [[ $# -eq 1 && $1 == --list ]]; then
    printf 'make %s\n' "${targets[@]}"
    exit 0
fi
if [[ $# -ne 0 ]]; then
    printf 'Usage: %s [--list]\n' "$0" >&2
    exit 2
fi

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
commands=()
for target in "${targets[@]}"; do
    commands+=(-- make "$target")
done
exec python3 "$root/scripts/gates/run_gate.py" \
    --wave 4 --log-dir "${WAVE_GATE_LOG_DIR:-$root/.logs/wave-4}" \
    "${commands[@]}"
