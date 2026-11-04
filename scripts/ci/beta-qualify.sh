#!/usr/bin/env bash
set -euo pipefail
umask 077

# Resolve one eligible task without splitting or rewriting its verify command.
beta_task_command() {
    python3 - "$@" <<'PY_TASK_COMMAND'
import json
from pathlib import Path
import re
import sys


def focused_task_command(path, task):
    sections = {}
    section = None
    for line in Path(path).read_text().splitlines():
        match = re.fullmatch(r'\[([^]]+)\]\s*', line)
        if match:
            section = match[1]
            if section in sections:
                raise ValueError('duplicate spec section')
            sections[section] = {}
        elif section and re.match(r'^[a-z_][a-z0-9_]*\s*=', line):
            key, value = line.split('=', 1)
            key = key.strip()
            if key in sections[section]:
                raise ValueError('duplicate spec key')
            sections[section][key] = value.strip()
    if not re.fullmatch(r'[0-9]+(?:\.[0-9]+)*', task):
        raise ValueError('one explicit task ID required')
    block = sections.get('task.' + task)
    if block is None:
        raise ValueError('task absent from selected spec')
    if json.loads(block['status']) not in ('pending', 'in_progress'):
        raise ValueError('selected task is not pending or in progress')
    requires = json.loads(block['requires'])
    if not isinstance(requires, list) or not all(isinstance(item, str) for item in requires):
        raise ValueError('invalid task dependencies')
    for dependency in requires:
        parent = sections.get('task.' + dependency)
        if parent is None or json.loads(parent['status']) != 'done':
            raise ValueError('selected task has an unfinished dependency')
    command = json.loads(block['verify_cmd'])
    if not isinstance(command, str) or not re.fullmatch(r'make(?: [A-Za-z0-9_][A-Za-z0-9_-]*)+', command):
        raise ValueError('source-run requires an exact plain make verify_cmd')
    return command


if __name__ == '__main__':
    try:
        if len(sys.argv) != 3:
            raise ValueError('usage: beta-qualify.sh --print-command SPEC TASK')
        print(focused_task_command(sys.argv[1], sys.argv[2]))
    except (OSError, ValueError, KeyError, TypeError) as error:
        print('beta-qualify: ' + str(error), file=sys.stderr)
        sys.exit(2)
PY_TASK_COMMAND
}

beta_qualify_focused() {
    local root spec='' ledger='' directory=${PAXEER_X_EVIDENCE_DIR:-} task='' timeout=600 kind=file
    root=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
    while [ "$#" -gt 0 ]; do
        case $1 in
        --spec|--ledger|--evidence-root|--task|--timeout|--evidence-kind)
            [ "$#" -ge 2 ] || return 2
            case $1 in
            --spec) spec=$2 ;; --ledger) ledger=$2 ;; --evidence-root) directory=$2 ;;
            --task) task=$2 ;; --timeout) timeout=$2 ;; --evidence-kind) kind=$2 ;;
            esac
            shift 2 ;;
        --) shift; break ;;
        -h|--help)
            echo 'usage: beta-qualify.sh --spec PATH --task ID --ledger PRIVATE_PATH --evidence-root PRIVATE_DIR [--timeout SECONDS] -- "make TARGET"'
            echo '       beta-qualify.sh --print-command SPEC TASK'
            return 0 ;;
        *) break ;;
        esac
    done
    [ -n "$spec" ] && [ -n "$ledger" ] && [ -n "$directory" ] && [ -n "$task" ] && [ "$#" -gt 0 ] || {
        echo 'beta-qualify: explicit spec, task, private ledger and evidence root required' >&2
        return 2
    }
    exec bash "$root/scripts/ci/beta-ledger-check.sh" --source-run --root "$root" \
        --evidence-root "$directory" --spec "$spec" --task "$task" --ledger "$ledger" \
        --timeout "$timeout" --evidence-kind "$kind" -- "$@"
}

if [ "${1:-}" = --print-command ]; then
    shift
    beta_task_command "$@"
else
    beta_qualify_focused "$@"
fi
