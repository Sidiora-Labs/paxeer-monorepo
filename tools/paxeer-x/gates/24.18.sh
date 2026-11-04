#!/usr/bin/env bash
# Embedded paxd foundation and retained consensus/receipt qualification.
# paxeer-x-services: sequencer platform-receipt-authority
set -euo pipefail
umask 077
cd "$(dirname "${BASH_SOURCE[0]}")/../../.."
case "${1:-verify}" in
build)
    : "${PAXEER_X_FOUNDATION_ARTIFACT_MANIFEST:?private merged-runtime manifest output required}"
    [[ $(basename -- "$PAXEER_X_FOUNDATION_ARTIFACT_MANIFEST") == manifest.json ]] || {
        echo 'foundation output must be the standard bundle manifest.json' >&2; exit 1;
    }
    python3 tools/bringup/tests/foundation-artifacts.py --build \
        --output "$(dirname -- "$PAXEER_X_FOUNDATION_ARTIFACT_MANIFEST")"
    ;;
verify)
    timeout 15m python3 - <<'PY'
import json
import re
import subprocess

CORPORA = (
    ('./custodyproof', (
        'TestRealStateCreditAndClockBounds',
        'TestRealStateCreditRefusesMissingAuthority',
        'TestNativeBankCommittedStateAndRecovery',
        'TestNativeBankMalformedProofRefusal',
    )),
    ('./consensus/internal/consensus/types', (
        'TestLeaderPinRoundRobinFromActivation',
    )),
    ('./modules/sequencer/keeper', (
        'TestReceiptKeysGovernanceOnly',
        'TestReceiptKeysRotationBoundary',
    )),
    ('./node/sequencer', (
        'TestStandbyAckBeforeRelease',
        'TestStandbyNoAckNoReceipt',
    )),
)


def executed_counts(output, names):
    events = [json.loads(line) for line in output.splitlines() if line.strip()]
    if not events or events[-1].get('Action') != 'pass' or events[-1].get('Test'):
        raise ValueError('successful package completion required')
    starts = [row['Test'] for row in events if row.get('Test') and row.get('Action') == 'run']
    ends = [row['Test'] for row in events if row.get('Test') and row.get('Action') == 'pass']
    if any(row.get('Action') in ('skip', 'fail') for row in events):
        raise ValueError('failed or skipped Go case refused')
    if (not starts or len(starts) != len(set(starts)) or sorted(starts) != sorted(ends)
            or {name for name in starts if '/' not in name} != set(names)):
        raise ValueError('exact nonempty Go corpus required')
    return len(ends)


def main():
    total = 0
    for package, names in CORPORA:
        pattern = '^(' + '|'.join(re.escape(name) for name in names) + ')$'
        result = subprocess.run(['go', 'test', '-json', '-count=1', '-run', pattern, package],
                                stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                text=True, timeout=900)
        print(result.stdout, end='', flush=True)
        if result.returncode:
            raise SystemExit(result.returncode if result.returncode > 0 else 1)
        total += executed_counts(result.stdout, names)
    print(f'PAXEER_X_GATE tests={total} skipped=0')


if __name__ == '__main__':
    main()
PY
    ;;
*) echo 'usage: 24.18.sh build|verify' >&2; exit 2 ;;
esac
