#!/usr/bin/env bash
set -euo pipefail

targets=(workspace-build workspace-test core-test-all test-sanitizer-suite)

if [[ $# -eq 1 && $1 == --list ]]; then
    printf 'make %s\n' "${targets[@]}"
    exit 0
fi
if [[ $# -ne 0 ]]; then
    printf 'Usage: %s [--list]\n' "$0" >&2
    exit 2
fi

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
commands=()
for target in "${targets[@]}"; do
    commands+=(-- make "$target")
done

exec python3 "$ROOT/scripts/gates/run_gate.py" --wave 1 \
    --log-dir "${GATE_LOG_DIR:-$ROOT/.logs/gates}" "${commands[@]}"
