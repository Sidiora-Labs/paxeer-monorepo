#!/usr/bin/env bash
# Committed-block archive freshness from the retained full-node and mirror.
# paxeer-x-services: mirror-publisher mirror-signer sequencer
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../../.."
exec timeout 20m python3 - "$@" <<'PY'
import re
import subprocess
import sys

CASES = (
    'publisher::tests::a_v4_archive_round_trips_a_committed_block',
    'runtime::tests::the_runtime_spools_committed_archives_in_height_order',
    'publisher::tests::a_fresh_gate_withholds_unreconciled_journal_progress',
)


INTEGRATION_CASES = (
    ('committed_freshness', 'freshness_reports_committed_lag_in_blocks'),
    ('committed_source', 'the_finalized_tag_gives_the_latest_committed_coordinate'),
    ('committed_source', 'a_committed_block_is_acquired_with_every_receipt_verified'),
)


def executed_counts(output, name):
    summaries = re.findall(r"^test result: ok\. (\d+) passed; (\d+) failed; (\d+) ignored;", output, re.M)
    passed = re.findall(r"^test " + re.escape(name) + r" \.{3} ok$", output, re.M)
    if summaries != [('1', '0', '0')] or len(passed) != 1:
        raise ValueError('exact nonempty unskipped Rust case required: ' + name)
    return int(summaries[0][0])


def main():
    build = sys.argv[1:] == ['build']
    if sys.argv[1:] not in ([], ['verify'], ['build']):
        raise SystemExit('usage: 18.3.sh build|verify')
    base = ['cargo', 'test', '--locked', '--manifest-path', 'services/interop/Cargo.toml',
            '-p', 'layerx-mirror']
    if build:
        subprocess.run(base + ['--lib', '--no-run'], check=True, timeout=1200)
        for target in sorted({target for target, _ in INTEGRATION_CASES}):
            subprocess.run(base + ['--test', target, '--no-run'], check=True, timeout=1200)
        return
    total = 0
    for name in CASES:
        result = subprocess.run(base + ['--lib'] + [name, '--', '--exact', '--nocapture'],
                                stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                text=True, timeout=1200)
        print(result.stdout, end='', flush=True)
        if result.returncode:
            raise SystemExit(result.returncode if result.returncode > 0 else 1)
        total += executed_counts(result.stdout, name)
    for target, name in INTEGRATION_CASES:
        result = subprocess.run(base + ['--test', target, name, '--', '--exact', '--nocapture'],
                                stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                text=True, timeout=1200)
        print(result.stdout, end='', flush=True)
        if result.returncode:
            raise SystemExit(result.returncode if result.returncode > 0 else 1)
        total += executed_counts(result.stdout, name)
    print(f'PAXEER_X_GATE tests={total} skipped=0')


if __name__ == '__main__':
    main()
PY
