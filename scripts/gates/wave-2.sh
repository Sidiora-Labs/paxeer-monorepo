#!/usr/bin/env bash
set -euo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
targets=(workspace-build workspace-test core-test-all agent-test)

if [[ $# -eq 1 && $1 == --list ]]; then
    printf 'make %s\n' "${targets[@]}"
    exit 0
fi
if [[ $# -ne 0 ]]; then
    printf 'Usage: %s [--list]\n' "$0" >&2
    exit 2
fi

commands=()
for target in "${targets[@]}"; do
    commands+=(-- make "$target")
done

exec python3 "$root/scripts/gates/run_gate.py" \
    --wave 2 --log-dir "$root/.logs/wave-2" "${commands[@]}"
