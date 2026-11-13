"""Exercise the real filesystem inventory/codec and qualifier CLI without toolchains."""
import importlib.util
import json
from pathlib import Path
import re
import shlex
import shutil

import pytest


BOOTSTRAP = Path(__file__).resolve().parents[1]
ROOT = BOOTSTRAP.parents[3]


def load_module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


baseline = load_module('repair939_baseline', BOOTSTRAP / 'baseline.py')
controls = load_module('repair939_controls', BOOTSTRAP / 'test_baseline.py')
qualify = load_module('repair939_qualify', BOOTSTRAP.parent / 'qualify.py')


def copy_source(root, relative):
    destination = root / relative
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(ROOT / relative, destination)
    return destination


def write_package(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, sort_keys=True) + '\n')


def inventory_files(root):
    return list(baseline.source_files(root, root / 'private-evidence'))


def hashes(root):
    return {path.relative_to(root).as_posix(): baseline.sha(path) for path in inventory_files(root)}


@pytest.fixture
def go_root(tmp_path):
    copy_source(tmp_path, 'services/launchpad/go.work')
    members = baseline.PRODUCTION_GO_MODULES | baseline.TOOL_GO_MODULES | {
        'apps/launchpad/web/src/core/test-fixtures/operation-envelope',
    }
    for member in members:
        for filename in ('go.mod', 'go.sum'):
            copy_source(tmp_path, member + '/' + filename)
    return tmp_path


@pytest.fixture
def npm_root(tmp_path):
    copy_source(tmp_path, 'apps/launchpad/pnpm-workspace.yaml')
    manifests = [ROOT / 'apps/launchpad/package.json',
                 ROOT / 'apps/launchpad/web/package.json',
                 ROOT / 'apps/launchpad/mobile/package.json',
                 *sorted((ROOT / 'sdk/launchpad/packages').glob('*/package.json'))]
    for manifest in manifests:
        copy_source(tmp_path, manifest.relative_to(ROOT))
    # A committed fixture's valid codec is readable but it is outside membership.
    copy_source(tmp_path, 'apps/launchpad/qualification/image-dependencies/fixture/package.json')
    return tmp_path


def test_module_inventory_matches_canonical_workspace_and_separates_tools(go_root):
    before = hashes(go_root)
    failures = []
    production, auxiliary = baseline.go_inventory(go_root, inventory_files(go_root), failures.append)
    assert failures == []
    assert {path.parent.relative_to(go_root).as_posix() for path in production} == baseline.PRODUCTION_GO_MODULES
    assert len(production) == 17
    assert {path.parent.relative_to(go_root).as_posix() for path in auxiliary} == baseline.TOOL_GO_MODULES | {
        'apps/launchpad/web/src/core/test-fixtures/operation-envelope',
    }
    assert hashes(go_root) == before


@pytest.mark.parametrize('damage', ['missing-module', 'missing-checksum', 'missing-production-checksum', 'workspace-drift'])
def test_module_inventory_rejects_missing_members_checksums_and_same_count_drift(go_root, damage):
    if damage == 'missing-module':
        (go_root / 'services/launchpad/api/go.mod').unlink()
        expected = 'Missing required go.mod: services/launchpad/api'
    elif damage == 'missing-checksum':
        (go_root / 'tools/launchpad/production/load/eventdriver/go.sum').unlink()
        expected = 'Missing go.sum: tools/launchpad/production/load/eventdriver/go.mod'
    elif damage == 'missing-production-checksum':
        (go_root / 'services/launchpad/api/go.sum').unlink()
        expected = 'Missing go.sum: services/launchpad/api/go.mod'
    else:
        workspace = go_root / 'services/launchpad/go.work'
        workspace.write_text(workspace.read_text().replace('./api\n', './unreviewed-api\n'))
        expected = 'Go workspace membership mismatch:'
    before = hashes(go_root)
    failures = []
    baseline.go_inventory(go_root, inventory_files(go_root), failures.append)
    assert any(message.startswith(expected) for message in failures), failures
    assert hashes(go_root) == before


def test_module_inventory_rejects_unreviewed_production_members(go_root):
    directory = go_root / 'services/launchpad/unreviewed-service'
    directory.mkdir()
    for name in ('go.mod', 'go.sum'):
        shutil.copyfile(go_root / 'services/launchpad/api' / name, directory / name)
    before = hashes(go_root)
    failures = []
    baseline.go_inventory(go_root, inventory_files(go_root), failures.append)
    assert 'Unreviewed Go module outside production/tool inventory: services/launchpad/unreviewed-service' in failures
    assert hashes(go_root) == before


def test_workspace_inventory_includes_apps_and_moved_sdks_without_mutation(npm_root):
    before = hashes(npm_root)
    failures = []
    manifests = baseline.npm_workspace_manifests(npm_root, inventory_files(npm_root), failures.append)
    assert failures == []
    relative = {path.relative_to(npm_root).as_posix() for path, data in manifests}
    expected = {'apps/launchpad/package.json', 'apps/launchpad/web/package.json', 'apps/launchpad/mobile/package.json'}
    expected.update(path.relative_to(ROOT).as_posix() for path in (ROOT / 'sdk/launchpad/packages').glob('*/package.json'))
    assert relative == expected
    names = {data['name'] for path, data in manifests}
    assert {'@sidiora/web', '@sidiora/mobile', '@kindle/sdk', '@kindle/sdk-v2'} <= names
    assert hashes(npm_root) == before


@pytest.mark.parametrize('relative', ['apps/launchpad/web/package.json', 'sdk/launchpad/packages/sdk/package.json'])
@pytest.mark.parametrize('section', ['dependencies', 'devDependencies', 'peerDependencies', 'optionalDependencies'])
@pytest.mark.parametrize('protocol', ['workspace:', 'file:', 'link:'])
def test_workspace_edges_resolve_and_reject_missing_targets(npm_root, relative, section, protocol):
    path = npm_root / relative
    data = json.loads(path.read_text())
    target = path.parent / 'repair-local-target'
    if protocol == 'workspace:':
        name, version = '@kindle/sdk-v2', 'workspace:*'
    else:
        target.mkdir()
        write_package(target / 'package.json', {'name': 'repair-local-target', 'version': '1.0.0'})
        name, version = 'repair-local-target', protocol + './repair-local-target'
    data.setdefault(section, {})[name] = version
    write_package(path, data)
    before = hashes(npm_root)
    failures = []
    baseline.npm_workspace_manifests(npm_root, inventory_files(npm_root), failures.append)
    assert failures == []
    assert hashes(npm_root) == before
    if protocol == 'workspace:':
        data[section]['missing-workspace'] = 'workspace:*'
        write_package(path, data)
        expected = 'Unresolved npm workspace package missing-workspace in ' + str(path)
    else:
        (target / 'package.json').unlink()
        target.rmdir()
        expected = 'Unresolved npm local dependency repair-local-target in ' + str(path)
    before = hashes(npm_root)
    failures = []
    baseline.npm_workspace_manifests(npm_root, inventory_files(npm_root), failures.append)
    assert expected in failures
    assert hashes(npm_root) == before


@pytest.mark.parametrize('damage', ['missing-manifest', 'missing-name', 'duplicate-name'])
def test_workspace_inventory_rejects_missing_manifests_and_names(npm_root, damage):
    if damage == 'missing-manifest':
        (npm_root / 'apps/launchpad/mobile/package.json').unlink()
        expected = 'Missing npm workspace manifest for mobile'
    else:
        path = npm_root / 'sdk/launchpad/packages/sdk-v2/package.json'
        data = json.loads(path.read_text())
        data['name'] = None if damage == 'missing-name' else '@kindle/sdk'
        write_package(path, data)
        expected = 'workspace package name is required' if damage == 'missing-name' else 'duplicate workspace package name'
    before = hashes(npm_root)
    failures = []
    baseline.npm_workspace_manifests(npm_root, inventory_files(npm_root), failures.append)
    assert any(expected in failure for failure in failures), failures
    assert hashes(npm_root) == before


def test_source_fixture_uses_launchpad_homes_and_preserves_exclusions():
    # Execute the repaired pre-existing assertion itself, in process.
    controls.BaselineControlFlow('test_source_manifest_excludes_secrets_generated_and_evidence').test_source_manifest_excludes_secrets_generated_and_evidence()
    controls.BaselineControlFlow('test_real_pin_file_aliases_and_reject_multiple_versions').test_real_pin_file_aliases_and_reject_multiple_versions()


def test_bootstrap_instructions_use_real_qualifier_registry_and_evidence_layout():
    readme = (BOOTSTRAP / 'README.md').read_text()
    commands = re.findall(r'```sh\n(.*?)\n```', readme, re.S)
    command = next(command for command in commands if command.startswith('python3 tools/launchpad/production/qualify.py'))
    argv = shlex.split(command.replace('\\\n', ' '))
    args = qualify.argument_parser().parse_args(argv[2:])
    defaults = qualify.argument_parser().parse_args(['--task', '0.1', '--spec', str(args.spec), '--require-current-source'])
    assert args.task == '0.1' and args.require_current_source
    assert args.registry == defaults.registry == Path('tools/launchpad/production/bootstrap/check-registry.json')
    assert args.evidence_dir == Path('$QUALIFICATION_OUTPUT_DIR')
    assert defaults.evidence_dir == Path('launchpad/evidence/production-system/qualification')
    assert args.contract_root == Path('contracts/launchpad')
    registry = json.loads((ROOT / args.registry).read_text())
    check = next(check for check in registry['tasks'][args.task]['checks'] if check['id'] == 'baseline-provenance')
    assert check['argv'] == ['python3', str((BOOTSTRAP / 'baseline.py').relative_to(ROOT))]
    assert check['required'] is True
    evidence = ROOT / args.evidence_dir / args.task / 'recorded-run'
    expanded = qualify.expand_tokens('{source_root}|{evidence_dir}|{contract_root}', ROOT, evidence, ROOT / args.contract_root)
    assert expanded.split('|') == [str(ROOT), str(evidence), str(ROOT / 'contracts/launchpad')]
    assert args.spec == Path('$LAUNCHPAD_SPEC_FILE')
    assert 'Set `LAUNCHPAD_SPEC_FILE`' in readme
    assert 'source command inventory' in readme
    assert 'seventeen reviewed production members' in readme
    assert 'separate\nevidence' in readme
    for name in ('result.json', 'progress.json', 'source-before.json', 'source-after.json', 'baseline.json', 'npm_workspace_manifests', 'auxiliary_go_modules'):
        assert name in readme
    pins = baseline.read_pins(BOOTSTRAP / 'toolchain/versions.txt')
    assert all(pins[name] == version for name, version in baseline.PINNED.items())
    assert (ROOT / args.contract_root).is_dir()
    assert (ROOT / 'deploy/launchpad/production/bootstrap/compose.yml').is_file()
