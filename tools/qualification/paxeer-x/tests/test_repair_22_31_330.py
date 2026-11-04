"""Exercise the real build-manifest producers, file transport and consumer admission.

The artifact bytes here are committed source documents, deliberately not native
executables. These tests qualify the manifest interface, integrity and execution
selection; they do not substitute a Python implementation for a native lifecycle.
Native lifecycle evidence remains mandatory in the production verifier.
"""
import ast
from copy import deepcopy
import importlib.util
import json
from pathlib import Path
import shutil

import pytest

QUAL = Path(__file__).resolve().parents[1]
SOURCE = {'revision': '089fc62b48230622d7e72a1625ec77c092956fb1',
          'tree': 'b3c095a9646aebc3b7d5fd6bef04b6f14088ea12', 'dirty': False}
CAPS_SOURCE = {'revision': SOURCE['revision'], 'source_tree': SOURCE['tree']}
SETTINGS = {'PROGRAMS_TARGET_DIR': '/private/cargo-target',
            'PROGRAMS_RUNTIME_LIB': '/private/liblayerx_programs_sandbox.a'}


def load(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), QUAL / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def document(directory, name, source):
    """Use actual committed source bytes to exercise opaque artifact transport."""
    path = directory / name
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    shutil.copyfile(QUAL / source, path)
    path.chmod(0o600)
    return path


def replace_manifest(path, value):
    path.write_text(json.dumps(value))
    path.chmod(0o600)


def caps_publication(directory):
    consumer = load('caps-discovery')
    native = document(directory, 'native-input.txt', 'build-caps-discovery.sh')
    rust = document(directory, 'rust-input.txt', 'program-interface-producer.py')
    path = consumer.contract.publish(directory, CAPS_SOURCE, native, rust)
    return consumer, path


def interface_publication(directory):
    producer = load('program-interface-producer')
    record = producer.build_inputs(SOURCE, directory / 'native-build', 4, {})
    artifacts = {name: document(directory, name + '.txt', 'program-interface-producer.py')
                 for name in producer.ARTIFACTS}
    logs = [document(directory, f'log-{index}.txt', 'build-caps-discovery.sh')
            for index in range(producer.BUILD_LOG_COUNT)]
    path = producer.publish_manifest(directory, record, artifacts, logs)
    return load('program-interface'), path


def recovery_publication(directory):
    producer = load('storage-recovery-producer')
    scoped = {'tools/qualification/paxeer-x/storage-recovery-producer.py':
              producer.digest(QUAL / 'storage-recovery-producer.py')}
    record = producer.build_inputs(SOURCE, scoped, SETTINGS)
    # This is the publisher's exit input, not an assertion that a native build ran.
    record['exit'] = 0
    for name in producer.TESTS:
        document(directory, 'bin/' + name, 'storage-recovery-producer.py')
    document(directory, producer.LOG, 'build-caps-discovery.sh')
    path = producer.publish_manifest(directory, record)
    return load('storage-recovery'), path, scoped


def test_f009_untouched_producer_manifest_admitted(tmp_path):
    consumer, path = caps_publication(tmp_path)
    original = path.read_bytes()
    value = consumer.admit_manifest(path, CAPS_SOURCE)
    assert set(value['artifacts']) == {'native', 'rust'}
    assert value['build_logs'] == ['native-build.log', 'fixture-build.log', 'client-build.log']
    assert value['purpose'] == 'native-complete-caps-discovery'
    assert path.read_bytes() == original
    assert load('caps-discovery').admit_manifest(path, CAPS_SOURCE) == value
    with pytest.raises(FileExistsError):
        consumer.contract.publish(tmp_path, CAPS_SOURCE,
                                  value['artifacts']['native']['path'],
                                  value['artifacts']['rust']['path'])
    assert path.read_bytes() == original


def test_f009_shell_uses_shared_publication():
    # The actual shell build delegates publication to the function exercised above.
    script = (QUAL / 'build-caps-discovery.sh').read_text()
    body = script.split("<<'MANIFEST'\n", 1)[1].rsplit('\nMANIFEST', 1)[0]
    tree = ast.parse(body)
    assert any(isinstance(node, ast.Call) and isinstance(node.func, ast.Attribute)
               and isinstance(node.func.value, ast.Name) and node.func.value.id == 'contract'
               and node.func.attr == 'publish' for node in ast.walk(tree))
    assert 'layerxd' not in body


@pytest.mark.parametrize('change', ['daemon', 'missing', 'schema', 'source', 'logs'])
def test_f009_refuses_foreign_or_incomplete_manifest(tmp_path, change):
    consumer, path = caps_publication(tmp_path)
    value = json.loads(path.read_text())
    if change == 'daemon':
        value['artifacts']['layerxd'] = deepcopy(value['artifacts']['native'])
    elif change == 'missing':
        del value['artifacts']['rust']
    elif change == 'schema':
        value['version'] = 2
    elif change == 'source':
        value['source_tree'] = '0' * 40
    else:
        value['build_logs'].pop()
    replace_manifest(path, value)
    with pytest.raises(RuntimeError):
        consumer.admit_manifest(path, CAPS_SOURCE)


def test_f009_artifact_mutation_refused(tmp_path):
    consumer, path = caps_publication(tmp_path)
    value = consumer.admit_manifest(path, CAPS_SOURCE)
    Path(value['artifacts']['rust']['path']).write_bytes(b'changed')
    with pytest.raises(RuntimeError, match='digest mismatch'):
        consumer.admit_manifest(path, CAPS_SOURCE)


def test_f010_untouched_producer_manifest_admitted(tmp_path):
    consumer, path = interface_publication(tmp_path)
    before = path.read_bytes()
    value = consumer.admit_manifest(path, SOURCE)
    assert set(value['artifacts']) == {'native-library', 'sandbox-staticlib',
                                      'protocol-tests', 'legacy-tests'}
    assert len(value['logs']) == len(value['commands']) == 3
    assert 'required_native_cases' not in value
    assert value['required_protocol_cases'] == sorted(consumer.producer.PROTOCOL_CASES)
    assert value['required_legacy_cases'] == sorted(consumer.producer.LEGACY_CASES)
    assert 'layerxd' not in sum(value['commands'], [])
    suites = consumer.interface_suites(value['artifacts'])
    assert [name for name, _, _ in suites] == ['protocol', 'legacy']
    assert suites[0][1][0] == value['artifacts']['protocol-tests']['path']
    assert suites[1][1][0] == value['artifacts']['legacy-tests']['path']
    assert 'native_receipt_authorized_interface_reads' in suites[0][2]
    assert suites[1][2] == consumer.producer.LEGACY_CASES
    assert path.read_bytes() == before
    assert load('program-interface').admit_manifest(path, SOURCE) == value


@pytest.mark.parametrize('change', ['schema', 'source', 'cases', 'features', 'profiles',
                                   'artifact', 'extra-artifact', 'logs'])
def test_f010_keeps_all_retained_admission_checks(tmp_path, change):
    consumer, path = interface_publication(tmp_path)
    value = json.loads(path.read_text())
    if change == 'schema':
        value['schema'] += '-foreign'
    elif change == 'source':
        value['source']['tree'] = '0' * 40
    elif change == 'cases':
        value['required_protocol_cases'].remove('native_receipt_authorized_interface_reads')
    elif change == 'features':
        value['features']['registry-tests'] = ['foreign']
    elif change == 'profiles':
        value['profiles']['incremental'] = True
    elif change == 'artifact':
        del value['artifacts']['sandbox-staticlib']
    elif change == 'extra-artifact':
        value['artifacts']['native'] = deepcopy(value['artifacts']['protocol-tests'])
    else:
        value['logs'].pop()
    replace_manifest(path, value)
    with pytest.raises(ValueError):
        consumer.admit_manifest(path, SOURCE)


@pytest.mark.parametrize('target', ['protocol-tests', 'native-library', 'log'])
def test_f010_artifact_or_build_log_mutation_refused(tmp_path, target):
    consumer, path = interface_publication(tmp_path)
    value = consumer.admit_manifest(path, SOURCE)
    saved = value['logs'][0] if target == 'log' else value['artifacts'][target]
    Path(saved['path']).write_bytes(b'changed')
    with pytest.raises(ValueError, match='differs from producer'):
        consumer.admit_manifest(path, SOURCE)


def test_f010_native_evidence_is_mandatory_and_source_bound(tmp_path):
    consumer = load('program-interface')
    with pytest.raises(ValueError, match='required'):
        consumer.native_fixtures(None, SOURCE, tmp_path)
    path = tmp_path / 'native-evidence.json'
    consumer.producer.write_private(path, {
        'schema': 'paxeer-x.program-interface-native-evidence.v1',
        'source': dict(SOURCE, tree='0' * 40), 'artifacts': {}})
    with pytest.raises(ValueError, match='does not bind'):
        consumer.native_fixtures(path, SOURCE, tmp_path / 'inputs')
    value = json.loads(path.read_text())
    value['source'] = SOURCE
    replace_manifest(path, value)
    with pytest.raises(ValueError, match='inventory'):
        consumer.native_fixtures(path, SOURCE, tmp_path / 'inputs')
    assert not (tmp_path / 'inputs').exists()


def test_f010_native_publisher_refuses_absent_real_evidence(tmp_path):
    producer = load('program-interface-producer')
    with pytest.raises(FileNotFoundError):
        producer.publish_native_evidence(tmp_path, SOURCE, tmp_path / 'native-cases')
    assert not (tmp_path / 'native-evidence.json').exists()


def test_f011_untouched_producer_manifest_admitted(tmp_path):
    consumer, path, scoped = recovery_publication(tmp_path)
    before = path.read_bytes()
    value = consumer.admit_manifest(path, SOURCE, scoped)
    names = ('lxp_test_recovery', 'lxp_test_log_durability', 'lxp_test_batch_wal_recovery')
    assert value['command'] == ['make', '-j3',
                               'PROGRAMS_TARGET_DIR=' + SETTINGS['PROGRAMS_TARGET_DIR'],
                               'PROGRAMS_RUNTIME_LIB=' + SETTINGS['PROGRAMS_RUNTIME_LIB']] + [
                                   'build/tests/' + name for name in names]
    assert set(value['artifacts']) == set(names)
    assert consumer.recovery_suites(tmp_path) == [
        (name, [str(tmp_path / 'bin' / name)]) for name in names]
    assert path.read_bytes() == before
    assert load('storage-recovery').admit_manifest(path, SOURCE, scoped) == value


@pytest.mark.parametrize('change', ['daemon', 'target', 'setting', 'jobs', 'cwd', 'source',
                                   'sources', 'exit', 'inventory', 'log'])
def test_f011_refuses_command_provenance_and_inventory_changes(tmp_path, change):
    consumer, path, scoped = recovery_publication(tmp_path)
    value = json.loads(path.read_text())
    if change == 'daemon':
        value['command'].insert(4, 'layerxd')
    elif change == 'target':
        value['command'][-1] = 'build/tests/foreign'
    elif change == 'setting':
        value['command'][2] = 'FOREIGN_TARGET_DIR=/tmp/foreign'
    elif change == 'jobs':
        value['command'][1] = '-j4'
    elif change == 'cwd':
        value['cwd'] = '/foreign'
    elif change == 'source':
        value['tree'] = '0' * 40
    elif change == 'sources':
        value['sources'] = {}
    elif change == 'exit':
        value['exit'] = 1
    elif change == 'inventory':
        del value['artifacts']['lxp_test_recovery']
    else:
        value['log_sha256'] = '0' * 64
    replace_manifest(path, value)
    with pytest.raises(ValueError):
        consumer.admit_manifest(path, SOURCE, scoped)


def test_f011_retained_artifact_mutation_refused(tmp_path):
    consumer, path, scoped = recovery_publication(tmp_path)
    consumer.admit_manifest(path, SOURCE, scoped)
    (tmp_path / 'bin' / 'lxp_test_batch_wal_recovery').write_bytes(b'changed')
    with pytest.raises(ValueError, match='differs from producer'):
        consumer.admit_manifest(path, SOURCE, scoped)


@pytest.mark.parametrize('publication', [caps_publication, interface_publication, recovery_publication])
def test_private_manifest_and_symlink_refusals(tmp_path, publication):
    result = publication(tmp_path)
    consumer, path = result[:2]
    args = (CAPS_SOURCE,) if publication is caps_publication else (SOURCE,)
    if publication is recovery_publication:
        args += (result[2],)
    path.chmod(0o644)
    with pytest.raises((ValueError, RuntimeError), match='private'):
        consumer.admit_manifest(path, *args)
    path.chmod(0o600)
    link = tmp_path / 'manifest-link.json'
    link.symlink_to(path)
    with pytest.raises(OSError):
        consumer.admit_manifest(link, *args)
