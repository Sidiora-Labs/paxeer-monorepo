#!/usr/bin/env bash
set -euo pipefail

commands=(
    'make workspace-build'
    'make workspace-test'
    'make workspace-lint'
    'make core-test-all'
    'python3 tools/paxeer-x/tests/retired_callers_test.py --deleted'
    'make programs-test'
)

if [[ $# -eq 1 && $1 == --list ]]; then
    printf '%s\n' "${commands[@]}"
    exit 0
fi
if [[ $# -ne 0 ]]; then
    printf 'Usage: %s [--list]\n' "$0" >&2
    exit 2
fi

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
arguments=()
for command in "${commands[@]}"; do
    read -r -a command_arguments <<< "$command"
    arguments+=(-- "${command_arguments[@]}")
done

exec python3 "$root/scripts/gates/run_gate.py" \
    --wave 6 --log-dir "$root/.logs/wave-6" "${arguments[@]}"
