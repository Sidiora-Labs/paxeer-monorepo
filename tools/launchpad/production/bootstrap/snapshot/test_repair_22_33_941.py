import contextlib
import hashlib
import io
import json
from pathlib import Path
import sys

import pytest

import snapshot


REPO_ROOT = Path(__file__).resolve().parents[5]
OWNED_SOURCES = (
    'tools/launchpad/production/bootstrap/build/check.py',
    'tools/launchpad/production/bootstrap/build/manifest.json',
    'sdk/launchpad/protocol/coverage/permissions.go',
    'sdk/launchpad/protocol/coverage/authorities.json',
    'apps/launchpad/mobile/src/runtime/session.ts',
)
GENERATED_FILES = (
    'build/app.o', 'coverage/report.json', 'dist/app.js', 'runtime/db', 'logs/state',
    'consensus/build/app.o', 'sdk/build/app.o', 'wasm/build/app.o',
    'interchain/build/app.o', 'contracts/bridge/evm/build/app.o', 'vue/dist/app.js',
    'apps/launchpad/web/build/app.js', 'apps/launchpad/web/dist/app.js',
    'apps/launchpad/web/logs/state', 'contracts/launchpad/coverage/report.json',
    'contracts/launchpad/dist/app.js', 'sdk/launchpad/packages/dist/index.js',
    'sdk/launchpad/packages/sidiora-launchpad-v3/dist/index.js',
    'nested/node_modules/pkg/index.js', 'nested/.next/chunk.js',
    'nested/.turbo/cache', 'nested/.cache/data', 'nested/__pycache__/module.pyc',
    'nested/.venv/config', 'nested/venv/config', 'nested/.git/config',
)
SECRET_FILES = (
    '.env', 'config/.env.production', 'config/id_rsa', 'config/id_ed25519',
    'config/id_ecdsa', 'config/.npmrc', 'config/.pypirc', 'config/.netrc',
    'config/credentials.json', 'config/service-account.json', 'config/tls.pem',
    'config/tls.key', 'config/account.p12', 'config/account.pfx',
    'config/account.keystore', 'nested/secrets/account.json',
    'nested/.secrets/account.json', 'nested/credentials/account.json',
    'tools/launchpad/production/bootstrap/build/.env.local',
    'sdk/launchpad/protocol/coverage/signing.key',
    'apps/launchpad/mobile/src/runtime/credentials.json',
)


def put(root, relative, content):
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(content)
    return path


def run_cli(*arguments):
    stdout, stderr = io.StringIO(), io.StringIO()
    previous = sys.argv
    try:
        sys.argv = [str(Path(snapshot.__file__)), *map(str, arguments)]
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            code = snapshot.main()
    finally:
        sys.argv = previous
    return code, stdout.getvalue(), stderr.getvalue()


def create(root, output):
    code, stdout, stderr = run_cli('create', '--root', root, '--output', output)
    assert code == 0, stderr
    manifest = json.loads(output.read_bytes())
    assert json.loads(stdout) == {'action': 'create', 'sha256': manifest['sha256']}
    assert snapshot.validate_manifest(manifest) == manifest
    body = {key: value for key, value in manifest.items() if key != 'sha256'}
    encoded = json.dumps(body, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode()
    assert hashlib.sha256(encoded).hexdigest() == manifest['sha256']
    return manifest


@pytest.fixture
def source(tmp_path):
    root = tmp_path / 'source'
    root.mkdir()
    for relative in OWNED_SOURCES:
        put(root, relative, (REPO_ROOT / relative).read_bytes()).chmod(0o644)
    return root


@pytest.mark.parametrize('relative', OWNED_SOURCES)
@pytest.mark.parametrize('operation', ('add', 'modify', 'delete', 'executable'))
def test_provenance_excludes_owned_source_mutations(source, tmp_path, relative, operation):
    path = source / relative
    original = path.read_bytes()
    if operation == 'add':
        path.unlink()
    baseline_path = tmp_path / 'baseline.json'
    baseline = create(source, baseline_path)
    baseline_bytes = baseline_path.read_bytes()
    assert set(baseline['files']) == set(OWNED_SOURCES) - ({relative} if operation == 'add' else set())
    code, stdout, stderr = run_cli('verify', '--root', source, '--baseline', baseline_path)
    assert code == 0, stderr
    assert json.loads(stdout) == {'action': 'verify', 'sha256': baseline['sha256']}

    if operation == 'add':
        path.write_bytes(original)
        path.chmod(0o644)
    elif operation == 'modify':
        path.write_bytes(original + b'\n// provenance mutation\n')
    elif operation == 'delete':
        path.unlink()
    else:
        path.chmod(0o755)

    candidate = create(source, tmp_path / 'candidate.json')
    assert candidate['sha256'] != baseline['sha256']
    if operation != 'delete':
        assert candidate['files'][relative] == {
            'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
            'size': path.stat().st_size,
            'executable': operation == 'executable',
        }
    code, stdout, stderr = run_cli('verify', '--root', source, '--baseline', baseline_path)
    assert code == 1
    assert stdout == ''
    assert 'source drift detected' in stderr

    diff_path = tmp_path / 'diff.json'
    code, stdout, stderr = run_cli(
        'diff', '--root', source, '--baseline', baseline_path,
        '--owned', 'peer/**', '--output', diff_path,
    )
    assert code == 1
    assert stdout == ''
    assert 'change outside owned paths: ' + relative in stderr
    assert not diff_path.exists()

    code, stdout, stderr = run_cli(
        'diff', '--root', source, '--baseline', baseline_path,
        '--owned', str(Path(relative).parent) + '/**', '--output', diff_path,
    )
    assert code == 0, stderr
    diff = json.loads(diff_path.read_bytes())
    assert json.loads(stdout) == {'action': 'diff', 'sha256': diff['sha256']}
    assert diff['baseline_sha256'] == baseline['sha256']
    assert diff['candidate_sha256'] == candidate['sha256']
    assert diff['changes'] == [{
        'path': relative,
        'before': baseline['files'].get(relative),
        'after': candidate['files'].get(relative),
    }]
    assert diff['sha256'] == snapshot.digest({key: value for key, value in diff.items() if key != 'sha256'})
    assert baseline_path.read_bytes() == baseline_bytes


def test_generated_outputs_and_secrets_remain_excluded(source, tmp_path):
    baseline_path = tmp_path / 'baseline.json'
    baseline = create(source, baseline_path)
    for relative in GENERATED_FILES + SECRET_FILES:
        put(source, relative, b'excluded contents must not enter provenance')
    put(source, 'sdk/launchpad/protocol/coverage/debug.log', b'log contents')
    put(source, 'tools/launchpad/production/bootstrap/build/check.pyc', b'bytecode')
    assert create(source, tmp_path / 'candidate.json') == baseline
    code, stdout, stderr = run_cli('verify', '--root', source, '--baseline', baseline_path)
    assert code == 0, stderr
    assert json.loads(stdout)['sha256'] == baseline['sha256']


@pytest.mark.parametrize('directory', ('build', 'coverage', 'runtime', 'dist', 'logs'))
def test_output_names_in_other_source_locations_are_inventoried(tmp_path, directory):
    root = tmp_path / 'source'
    original = (REPO_ROOT / OWNED_SOURCES[0]).read_bytes()
    relative = 'owned/' + directory + '/source.py'
    put(root, relative, original)
    put(root, directory + '-source/source.py', original)
    baseline = create(root, tmp_path / 'baseline.json')
    assert set(baseline['files']) == {relative, directory + '-source/source.py'}
    assert baseline['files'][relative]['sha256'] == hashlib.sha256(original).hexdigest()
    put(root, relative, original + b'\n# mutation\n')
    candidate = create(root, tmp_path / 'candidate.json')
    with pytest.raises(ValueError, match='change outside owned paths: ' + relative):
        snapshot.owned_diff(baseline, candidate, [directory + '/**'])


@pytest.mark.parametrize('relative', OWNED_SOURCES)
@pytest.mark.parametrize('directory_link', (False, True))
def test_symlinks_in_owned_source_directories_fail_closed(source, tmp_path, relative, directory_link):
    external = tmp_path / 'external'
    external.mkdir()
    (external / 'private.txt').write_bytes(b'external private contents')
    path = source / relative
    if directory_link:
        path = path.parent / 'linked-directory'
        path.symlink_to(external, target_is_directory=True)
    else:
        path.unlink()
        path.symlink_to(external / 'private.txt')
    output = tmp_path / 'baseline.json'
    code, stdout, stderr = run_cli('create', '--root', source, '--output', output)
    assert code == 1
    assert stdout == ''
    assert 'source symlink rejected: ' + path.relative_to(source).as_posix() in stderr
    assert not output.exists()


def test_excluded_symlinks_do_not_expose_external_secrets(source, tmp_path):
    baseline = create(source, tmp_path / 'baseline.json')
    external = tmp_path / 'private'
    external.mkdir()
    (external / 'account.json').write_bytes(b'private contents')
    (source / 'secrets').symlink_to(external, target_is_directory=True)
    (source / '.env').symlink_to(external / 'account.json')
    (source / 'build').symlink_to(external, target_is_directory=True)
    assert create(source, tmp_path / 'candidate.json') == baseline


@pytest.mark.parametrize('relative', ('coverage/report.json', 'config/tls.key'))
def test_manifest_consumer_rejects_excluded_paths(source, tmp_path, relative):
    manifest = create(source, tmp_path / 'baseline.json')
    manifest['files'][relative] = dict(manifest['files'][OWNED_SOURCES[0]])
    manifest['sha256'] = snapshot.digest({key: value for key, value in manifest.items() if key != 'sha256'})
    with pytest.raises(ValueError, match='excluded path in manifest: ' + relative):
        snapshot.validate_manifest(manifest)


def test_cli_retains_immutable_baselines_and_rejects_tampering(source, tmp_path):
    output = tmp_path / 'baseline.json'
    manifest = create(source, output)
    original = output.read_bytes()
    code, stdout, stderr = run_cli('create', '--root', source, '--output', output)
    assert code == 1
    assert stdout == ''
    assert output.read_bytes() == original

    manifest['files'][OWNED_SOURCES[0]]['sha256'] = '0' * 64
    tampered = tmp_path / 'tampered.json'
    tampered.write_text(json.dumps(manifest))
    code, stdout, stderr = run_cli('verify', '--root', source, '--baseline', tampered)
    assert code == 1
    assert stdout == ''
    assert 'manifest digest mismatch' in stderr

    inside = source / 'manifest.json'
    code, stdout, stderr = run_cli('create', '--root', source, '--output', inside)
    assert code == 1
    assert stdout == ''
    assert 'manifest output must be outside source root' in stderr
    assert not inside.exists()
