#!/usr/bin/env bash
set -euo pipefail
umask 077
trap 'rc=$?; if (( rc != 0 )); then python3 docker/localnode/scripts/localnode_runtime.py fail build "$rc"; fi' EXIT
python3 docker/localnode/scripts/localnode_runtime.py check-stage
[[ "${ID:-0}" == 0 ]]
export LEDGER_ENABLED=false

# The coordinator serializes clean/build; shared run state is outside build/generated.
case "${SKIP_BUILD:-}" in
    ''|false|0)
        make -f chain.mk clean
        [[ ! -e build/paxd ]]
        if [[ "${MOCK_BALANCES:-false}" == true ]]; then
            make -f chain.mk build-linux BUILD_TAGS="mock_balances"
        else
            make -f chain.mk build-linux
        fi
        ;;
    true|1) echo "Using explicitly requested prebuilt local executable" ;;
    *) echo "Invalid SKIP_BUILD value" >&2; exit 1 ;;
esac
[[ -f build/paxd && -x build/paxd && ! -L build/paxd ]]
build/paxd version
# Snapshot the validated executable into this run before publishing build.complete.
python3 docker/localnode/scripts/localnode_runtime.py build-complete
