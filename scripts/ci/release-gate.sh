#!/usr/bin/env bash
# Run a task-bound release gate with private source evidence.
set -euo pipefail
umask 077

usage() {
    echo 'usage: release-gate.sh --job ID --spec PATH --task ID --ledger PRIVATE_PATH --evidence-root PRIVATE_DIR [--timeout SECONDS] [--evidence-kind file|directory] -- make TARGET [TARGET...]'
}

job=''
spec=''
task=''
ledger=''
directory=${PAXEER_X_EVIDENCE_DIR:-}
timeout=600
kind=file
while [ "$#" -gt 0 ]; do
    case $1 in
    --job|--spec|--task|--ledger|--evidence-root|--timeout|--evidence-kind)
        [ "$#" -ge 2 ] || { usage >&2; exit 2; }
        case $1 in
        --job) job=$2 ;; --spec) spec=$2 ;; --task) task=$2 ;;
        --ledger) ledger=$2 ;; --evidence-root) directory=$2 ;;
        --timeout) timeout=$2 ;; --evidence-kind) kind=$2 ;;
        esac
        shift 2
        ;;
    --)
        shift
        break
        ;;
    -h|--help)
        usage
        exit 0
        ;;
    *)
        usage >&2
        exit 2
        ;;
    esac
done
[ -n "$job" ] && [ -n "$spec" ] && [ -n "$task" ] && \
    [ -n "$ledger" ] && [ -n "$directory" ] && [ "$#" -ge 2 ] || {
    echo 'release-gate: explicit job, spec, task, private ledger and evidence root required' >&2
    usage >&2
    exit 2
}
case $job in
*[!A-Za-z0-9_-]*) echo 'release-gate: job id must be [A-Za-z0-9_-]+' >&2; exit 2 ;;
esac
[ "$1" = make ] || { echo 'release-gate: expected make targets' >&2; exit 2; }

root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
echo "release-gate: $job; task $task; private ledger $ledger" >&2
exec bash "$root/scripts/ci/beta-ledger-check.sh" --source-run --root "$root" \
    --spec "$spec" --task "$task" --ledger "$ledger" --evidence-root "$directory" \
    --timeout "$timeout" --evidence-kind "$kind" -- "$*"
