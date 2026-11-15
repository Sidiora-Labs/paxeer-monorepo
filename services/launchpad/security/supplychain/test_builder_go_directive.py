"""Launchpad module language versions fit the reviewed container builders."""

import re
from pathlib import Path

import pytest

from ci_tools import TOOLS


LAUNCHPAD = Path(__file__).resolve().parents[2]
ROOT = LAUNCHPAD.parents[1]
DOCKERFILES = tuple(
    ROOT / 'deploy' / 'launchpad' / name
    for name in ('Dockerfile.native', 'Dockerfile.svc')
)
MANIFESTS = tuple(sorted(LAUNCHPAD.rglob('go.mod'))) + (LAUNCHPAD / 'go.work',)


def _version(value):
    parts = tuple(int(part) for part in value.split('.'))
    return parts + (0,) * (3 - len(parts))


def _builder_version(path):
    versions = re.findall(
        r'^ARG BUILD_IMAGE=golang:(\d+\.\d+\.\d+)-bookworm'
        r'@sha256:[0-9a-f]{64}[ \t]*$',
        path.read_text(),
        re.MULTILINE,
    )
    assert len(versions) == 1, f'{path.name}: expected one digest-pinned Go builder'
    return versions[0]


def test_builder_versions_agree():
    versions = {path.name: _builder_version(path) for path in DOCKERFILES}
    assert len(set(versions.values())) == 1, f'Go builder versions differ: {versions}'
    assert TOOLS['go'] == (['version'], 'go' + next(iter(versions.values()))), (
        f'Reviewed Go tool version differs from container builders: {versions}'
    )


@pytest.mark.parametrize('dockerfile', DOCKERFILES, ids=lambda path: path.name)
def test_builder_version_check_matches_argument(dockerfile):
    version = _builder_version(dockerfile)
    checks = re.findall(
        r'test "\$\(go env GOVERSION\)" = "go(\d+\.\d+\.\d+)"',
        dockerfile.read_text(),
    )
    assert checks == [version], (
        f'{dockerfile.name}: Go version checks {checks} differ from builder {version}'
    )


@pytest.mark.parametrize(
    'manifest', MANIFESTS, ids=lambda path: str(path.relative_to(LAUNCHPAD))
)
def test_go_directive_does_not_exceed_builder(manifest):
    source = manifest.read_text()
    module = re.search(r'^module[ \t]+(\S+)', source, re.MULTILINE)
    name = module[1] if module else str(manifest.relative_to(ROOT))
    directives = re.findall(
        r'^go[ \t]+(\d+\.\d+(?:\.\d+)?)[ \t]*(?://[^\n]*)?$',
        source,
        re.MULTILINE,
    )
    assert len(directives) == 1, f'{name}: expected exactly one Go directive'
    for dockerfile in DOCKERFILES:
        builder = _builder_version(dockerfile)
        assert _version(directives[0]) <= _version(builder), (
            f'{name}: go {directives[0]} exceeds {dockerfile.name} builder {builder}'
        )
