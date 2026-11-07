import base64
import os
import re
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
MODULES = (
    'sdk/launchpad/protocol',
    'sdk/launchpad/shared',
    'services/launchpad/api',
    'services/launchpad/discovery',
    'services/launchpad/gateway',
    'services/launchpad/indexer',
    'services/launchpad/livestream',
    'services/launchpad/metadata',
    'services/launchpad/notifications',
    'services/launchpad/pnl-tracker',
    'services/launchpad/ranking-algo',
    'services/launchpad/security/control',
    'services/launchpad/security',
    'services/launchpad/social',
    'services/launchpad/stats-workers',
    'services/launchpad/trading-charts',
    'services/launchpad/user',
)


def test_go_checksums_support_readonly_resolution(tmp_path):
    assert len(MODULES) == len(set(MODULES)) == 17
    manifests = [ROOT / module / 'go.mod' for module in MODULES]
    checksums = [ROOT / module / 'go.sum' for module in MODULES]
    manifests.append(ROOT / 'services/launchpad/go.work')
    checksums.append(ROOT / 'services/launchpad/go.work.sum')
    for path in checksums:
        records = path.read_text().splitlines()
        assert records, f'{path} must contain module checksums'
        assert len(records) == len(set(records)), f'{path} repeats checksum records'
        for record in records:
            assert re.fullmatch(
                r'\S+ v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?'
                r'(?:\+[0-9A-Za-z.-]+)?(?:/go.mod)? h1:[A-Za-z0-9+/]{43}=',
                record,
            ), f'{path} contains an invalid checksum record: {record!r}'
            digest = record.split(' h1:', 1)[1]
            assert len(base64.b64decode(digest, validate=True)) == 32

    snapshots = {path: path.read_bytes() for path in manifests + checksums}
    cache = tmp_path / 'modules'
    cache.mkdir()
    assert not any(cache.iterdir()), 'module cache must start empty'
    environment = {
        **os.environ,
        'GOWORK': 'off',
        'GO111MODULE': 'on',
        'GOFLAGS': '-mod=readonly',
        'GOMODCACHE': str(cache),
    }
    try:
        for module in MODULES:
            for command in (
                ['go', 'list', '-mod=readonly', '-deps', '-test', './...'],
                ['go', 'mod', 'verify'],
            ):
                result = subprocess.run(
                    command,
                    cwd=ROOT / module,
                    env=environment,
                    capture_output=True,
                    text=True,
                    timeout=300,
                    check=False,
                )
                assert result.returncode == 0, (
                    f'{module}: {command} exited {result.returncode}\n'
                    f'{result.stdout}\n{result.stderr}'
                )
                changed = [
                    str(path.relative_to(ROOT))
                    for path, before in snapshots.items()
                    if not path.is_file() or path.read_bytes() != before
                ]
                assert not changed, f'{module}: {command} changed {changed}'
    finally:
        changed = [
            str(path.relative_to(ROOT))
            for path, before in snapshots.items()
            if not path.is_file() or path.read_bytes() != before
        ]
        assert not changed, f'readonly resolution changed {changed}'
