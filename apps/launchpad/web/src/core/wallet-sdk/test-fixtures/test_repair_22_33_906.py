import hashlib
import importlib.util
import json
from pathlib import Path
import shutil

import pytest


FIXTURES = Path(__file__).resolve().parent
ROOT = FIXTURES.parents[6]
WEB = 'apps/launchpad/web/src/'
OWNED_DIRECTORIES = (
    WEB + 'core/wallet-sdk',
    WEB + 'widgets/funding',
    WEB + 'widgets/wallet-recovery',
)
OWNED_FILES = (
    WEB + 'core/wagmi-config.ts',
    WEB + 'shell/wallet-button.tsx',
    WEB + 'shell/sign-in-modal.tsx',
    WEB + 'shell/wallet-modal.tsx',
)
REPRESENTATIVES = (
    WEB + 'core/wallet-sdk/test-fixtures/run-l25-phase4-check.py',
    WEB + 'core/wallet-sdk/test-fixtures/phase4-browser.mjs',
    WEB + 'widgets/funding/funding.ts',
    WEB + 'widgets/wallet-recovery/reconnect.ts',
    *OWNED_FILES,
)


@pytest.fixture(scope='module')
def checker():
    spec = importlib.util.spec_from_file_location(
        'phase4_source_checker', FIXTURES / 'run-l25-phase4-check.py'
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


@pytest.fixture(scope='module')
def owned_manifest():
    sources = set(ROOT / name for name in OWNED_FILES)
    for directory in OWNED_DIRECTORIES:
        sources.update(
            path for path in (ROOT / directory).rglob('*')
            if path.is_file()
            and path.suffix in {'.ts', '.tsx', '.py', '.cjs', '.mjs', '.toml', '.sol'}
        )
    return {
        str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest()
        for path in sorted(sources)
    }


def write_config(path, owned):
    path.write_text(json.dumps({'schemaVersion': 1, 'ownedSourceDigests': owned}))
    return path


def read_and_validate(checker, root, path):
    config = checker.load(path)
    return checker.validate_owned_sources(root, config['ownedSourceDigests'])


def copy_owned_sources(root, owned_manifest):
    for name in owned_manifest:
        destination = root / name
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / name, destination)


def test_phase4_source_layout_current_discovery(checker, owned_manifest, tmp_path):
    assert set(REPRESENTATIVES) <= set(owned_manifest)
    assert all(name.startswith(WEB) for name in owned_manifest)
    assert all((ROOT / name).is_file() for name in owned_manifest)
    config = write_config(tmp_path / 'admitted-config.json', owned_manifest)
    assert read_and_validate(checker, ROOT, config) == owned_manifest


@pytest.mark.parametrize('name', REPRESENTATIVES)
def test_phase4_source_layout_omitted_file(checker, owned_manifest, tmp_path, name):
    omitted = dict(owned_manifest)
    del omitted[name]
    config = write_config(tmp_path / 'omitted-config.json', omitted)
    with pytest.raises(ValueError, match='Owned source binding omits or adds actual lane files'):
        read_and_validate(checker, ROOT, config)


@pytest.mark.parametrize('name', REPRESENTATIVES)
def test_phase4_source_layout_changed_file(checker, owned_manifest, tmp_path, name):
    source_root = tmp_path / 'source'
    copy_owned_sources(source_root, owned_manifest)
    config = write_config(tmp_path / 'admitted-config.json', owned_manifest)
    changed = source_root / name
    changed.write_bytes(changed.read_bytes() + b'\n// changed after source admission\n')
    with pytest.raises(ValueError, match='Current source bytes changed: ' + name):
        read_and_validate(checker, source_root, config)


def test_phase4_source_layout_added_file(checker, owned_manifest, tmp_path):
    added = dict(owned_manifest)
    added[WEB + 'shell/wallet-connect-modal.tsx'] = hashlib.sha256(b'').hexdigest()
    config = write_config(tmp_path / 'added-config.json', added)
    with pytest.raises(ValueError, match='Owned source binding omits or adds actual lane files'):
        read_and_validate(checker, ROOT, config)


@pytest.mark.parametrize('directory', OWNED_DIRECTORIES)
def test_phase4_source_layout_unadmitted_source(checker, owned_manifest, tmp_path, directory):
    source_root = tmp_path / 'source'
    copy_owned_sources(source_root, owned_manifest)
    config = write_config(tmp_path / 'admitted-config.json', owned_manifest)
    added = source_root / directory / 'new-owned-source.ts'
    added.write_text('export const admissionRequired = true;\n')
    with pytest.raises(ValueError, match='Owned source binding omits or adds actual lane files'):
        read_and_validate(checker, source_root, config)


def test_phase4_source_layout_legacy_manifest(checker, owned_manifest, tmp_path):
    legacy = {
        name.replace('apps/launchpad/web/', 'client/apps/web/'): digest
        for name, digest in owned_manifest.items()
    }
    config = write_config(tmp_path / 'legacy-config.json', legacy)
    with pytest.raises(ValueError, match='Owned source binding omits or adds actual lane files'):
        read_and_validate(checker, ROOT, config)


@pytest.mark.parametrize('owned', [None, {}, [], 'not-a-manifest'])
def test_phase4_source_layout_requires_manifest(checker, tmp_path, owned):
    config = write_config(tmp_path / 'invalid-config.json', owned)
    with pytest.raises(ValueError, match='Exact current owned source manifest required'):
        read_and_validate(checker, ROOT, config)


def test_phase4_source_layout_missing_current_file(checker, owned_manifest, tmp_path):
    missing_root = tmp_path / 'source'
    for name in owned_manifest:
        if name == OWNED_FILES[2]:
            continue
        destination = missing_root / name
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / name, destination)
    config = write_config(tmp_path / 'admitted-config.json', owned_manifest)
    with pytest.raises(FileNotFoundError):
        read_and_validate(checker, missing_root, config)
