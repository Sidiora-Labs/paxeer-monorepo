#!/usr/bin/env bash
set -euo pipefail

commands=(
    'make workspace-build'
    'make workspace-test'
    'make human-test'
    'tools/containers/check.sh'
)

if [[ $# -eq 1 && $1 == --list ]]; then
    printf '%s\n' "${commands[@]}"
    exit 0
fi
if [[ $# -ne 0 ]]; then
    printf 'usage: scripts/gates/wave-5.sh [--list]\n' >&2
    exit 2
fi

ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
runner_args=()
for command in "${commands[@]}"; do
    read -r -a command_args <<< "$command"
    runner_args+=(-- "${command_args[@]}")
done

exec python3 "$ROOT/scripts/gates/run_gate.py" \
    --wave 5 --log-dir "${WAVE_GATE_LOG_DIR:-$ROOT/.logs/wave-5}" \
    "${runner_args[@]}"
