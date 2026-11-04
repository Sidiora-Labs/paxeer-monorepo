#!/usr/bin/env bash
# Committed archive producer and durable runtime recovery consumers.
# paxeer-x-services: mirror-publisher mirror-signer sequencer
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../../.."
exec timeout 20m python3 - "$@" <<'PY'
import re
import subprocess
import sys

CASES = (
    'publisher::tests::a_v4_archive_round_trips_a_committed_block',
    'publisher::tests::batch_era_archive_versions_are_refused',
    'runtime::tests::the_runtime_spools_committed_archives_in_height_order',
    'runtime::tests::clean_restart_resumes_after_the_contiguous_spool_prefix',
    'runtime::tests::startup_refuses_corrupt_archive_bytes',
    'runtime::tests::startup_refuses_gapped_batch_metadata',
    'runtime::tests::checkpoint_refusal_latches_until_verified_evidence',
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
        raise SystemExit('usage: 24.14.sh build|verify')
    base = ['cargo', 'test', '--locked', '--manifest-path', 'services/interop/Cargo.toml',
            '-p', 'layerx-mirror', '--lib']
    if build:
        subprocess.run(base + ['--no-run'], check=True, timeout=1200)
        return
    total = 0
    for name in CASES:
        result = subprocess.run(base + [name, '--', '--exact', '--nocapture'],
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
