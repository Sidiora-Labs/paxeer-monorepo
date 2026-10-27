#!/usr/bin/env bash
set -euo pipefail

commands=(
    'make workspace-ci'
    'make beta-qualify'
    'python3 tools/paxeer-x/tests/retired_callers_test.py --deleted'
    'scripts/ci/beta-ledger-check.sh --revisions'
)

if [[ $# -eq 1 && $1 == --list ]]; then
    printf '%s\n' "${commands[@]}"
    exit 0
fi
if [[ $# -ne 0 ]]; then
    printf 'usage: scripts/gates/wave-programme.sh [--list]\n' >&2
    exit 2
fi

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
log_dir="${GATE_LOG_DIR:-$root/.logs/wave-programme}"
runner_arguments=()
for command in "${commands[@]}"; do
    read -r -a arguments <<< "$command"
    runner_arguments+=(-- "${arguments[@]}")
done

exec python3 "$root/scripts/gates/run_gate.py" \
    --wave programme --log-dir "$log_dir" "${runner_arguments[@]}"
