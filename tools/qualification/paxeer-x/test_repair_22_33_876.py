"""Task 22.33.876: real qualification manifest and committed-proof admission.

Finding proofs: f014=test_f014_retained_transport; f060=test_f060_manifest_round_trip;
f015=test_f015_chain_commitment; f045=test_f045_projection_manifest;
f020=test_f020_transitive_inventory; f023=test_f023_retained_executor;
f046=test_f046_peer_manifest; f047=test_f047_presentation_manifest.

The corrected Python gate supersedes the generated Go gate (gotcha 2984).
No baseline gate was run: the ordered instructions prohibit verification before
implementation, and this proving file did not exist in the starting tree.
Publication tests use the running Python interpreter's actual executable bytes
as opaque file-transport inputs; they neither execute nor claim native builds.
Chain evidence is the committed Go producer transcript, with encoding conversion
only, and is verified by the retained SDK's production Comet verifier.

Wiring: services/agent/crates/layerx-client/tests/caps_discovery.rs (agent client domain): restore the missing real discovery test target with committed-block mutation markers.
Wiring: tests/qualification/lxp_wallet_caps_discovery.c (native runtime domain): drive retained paxd discovery transport instead of the retired daemon LNI interface.
Wiring: services/agent/crates/layerx-agentd/tests/native_program_human_peer.rs (agent domain): supply the absent real retained owner/peer target to the source-bound build producer.
Wiring: services/agent/crates/layerx-agentd/tests/native_program_presentation.rs (agent domain): supply the absent real presentation target to the source-bound build producer.
"""
import ast
import base64
from copy import deepcopy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import sys

from cryptography.exceptions import InvalidSignature
import pytest

QUAL = Path(__file__).resolve().parent
ROOT = QUAL.parents[2]
sys.path.insert(0, str(QUAL))


def load(name):
    spec = importlib.util.spec_from_file_location('repair_876_' + name.replace('-', '_'), QUAL / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


CUSTODY = load('human_native_custody')
DISCOVERY = load('caps-discovery')
MOVEMENT = load('movement-executor-readiness')
SOURCE = {'revision': 'aa5e0820b3652890f3812a220f4b91f4f8a45387',
          'tree': 'ab' * 20, 'dirty': False}
CAPS_SOURCE = {'revision': SOURCE['revision'], 'source_tree': SOURCE['tree']}


def private(path, data):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(data if isinstance(data, bytes) else data.encode())
    path.chmod(0o600)
    return path


def rewrite(path, value):
    private(path, json.dumps(value))


def shell_functions(name, delimiter):
    text = (QUAL / name).read_text()
    body = text.split("<<'" + delimiter + "'\n", 1)[1].split('\n' + delimiter, 1)[0]
    tree = ast.parse(body)
    declarations = ast.Module(body=[node for node in tree.body
                                   if isinstance(node, (ast.Import, ast.ImportFrom, ast.FunctionDef))],
                              type_ignores=[])
    namespace = {}
    exec(compile(declarations, str(QUAL / name), 'exec'), namespace)
    return namespace, tree


def test_f014_retained_transport(tmp_path):
    producer, _ = shell_functions('build-caps-codecs.sh', 'PY')
    stage = tmp_path / 'stage'
    stage.mkdir()
    original = Path(sys.executable).resolve(strict=True)
    saved = producer['stage_capability_transport'](original, stage)
    assert saved == {'path': str(stage / 'paxd'),
                     'sha256': hashlib.sha256(original.read_bytes()).hexdigest()}
    assert (stage / 'paxd').read_bytes() == original.read_bytes()
    assert (stage / 'paxd').stat().st_mode & 0o777 == 0o755
    with pytest.raises(RuntimeError, match='producer'):
        producer['stage_capability_transport'](stage / 'paxd', stage)
    link = tmp_path / 'untrusted-link'
    link.symlink_to(original)
    with pytest.raises(RuntimeError, match='transport'):
        producer['stage_capability_transport'](link, stage)
    absent = tmp_path / 'absent'
    with pytest.raises(RuntimeError, match='transport'):
        producer['stage_capability_transport'](absent, stage)
    script = (QUAL / 'build-caps-codecs.sh').read_text()
    assert 'layerxd' not in script and 'layerx-genesis-build' not in script
    assert "'method':'px_getCapabilities'" in script
    assert '--test committed_caps --no-run' in script
    assert '--test budget_schema --no-run' in script
    assert 'tests/qualification/lxp_wallet_caps_vectors.c' in script
    assert 'compiled artifact changed' in script


def discovery_publication(tmp_path):
    producer, tree = shell_functions('build-caps-discovery.sh', 'MANIFEST')
    assert any(isinstance(node, ast.Call) and isinstance(node.func, ast.Name)
               and node.func.id == 'publish_and_admit' for node in ast.walk(tree))
    native = private(tmp_path / 'native-input', (ROOT / 'tests/qualification/lxp_wallet_caps_discovery.c').read_bytes())
    rust = private(tmp_path / 'rust-input', (ROOT / 'services/agent/crates/layerx-client/tests/committed_caps.rs').read_bytes())
    path = producer['publish_and_admit'](tmp_path, CAPS_SOURCE, native, rust)
    return path, native, rust


def test_f060_manifest_round_trip(tmp_path):
    path, native, rust = discovery_publication(tmp_path)
    before = path.read_bytes()
    value = DISCOVERY.admit_manifest(path, CAPS_SOURCE)
    assert set(value['artifacts']) == {'native', 'rust'}
    assert value['artifacts']['native']['sha256'] == hashlib.sha256(native.read_bytes()).hexdigest()
    assert value['artifacts']['rust']['sha256'] == hashlib.sha256(rust.read_bytes()).hexdigest()
    assert load('caps-discovery').admit_manifest(path, CAPS_SOURCE) == value
    assert path.read_bytes() == before
    with pytest.raises(FileExistsError):
        DISCOVERY.contract.publish(tmp_path, CAPS_SOURCE, native, rust)
    assert path.read_bytes() == before


@pytest.mark.parametrize('mutation', ['native', 'rust', 'revision', 'tree', 'extra', 'missing', 'permissions', 'symlink'])
def test_f060_manifest_refusals(tmp_path, mutation):
    path, native, rust = discovery_publication(tmp_path)
    value = json.loads(path.read_text())
    if mutation in ('native', 'rust'):
        artifact = native if mutation == 'native' else rust
        artifact.write_bytes(artifact.read_bytes() + b'changed')
    elif mutation == 'revision':
        value['revision'] = '00' * 20
    elif mutation == 'tree':
        value['source_tree'] = '00' * 20
    elif mutation == 'extra':
        value['artifacts']['layerxd'] = value['artifacts']['native']
    elif mutation == 'missing':
        del value['artifacts']['rust']
    elif mutation == 'permissions':
        path.chmod(0o644)
    else:
        link = tmp_path / 'manifest-link'
        link.symlink_to(path)
        path = link
    if mutation in ('revision', 'tree', 'extra', 'missing'):
        rewrite(path, value)
    with pytest.raises((RuntimeError, OSError)):
        DISCOVERY.admit_manifest(path, CAPS_SOURCE)


def chain_recording():
    path = ROOT / 'services/human/crates/layerx-human-service/src/journeys/repair_22_31_153_transcript.json'
    document = json.loads(path.read_text())
    assert 'consensus/types header, votes and transaction proof' in document['producer']
    record = document['records'][0]
    header = deepcopy(record['material']['signedHeader']['header'])
    commit = deepcopy(record['material']['signedHeader']['commit'])
    commit['height'] = int(commit['height'])
    for signature in commit['signatures']:
        signature['signature'] = base64.b64decode(signature['signature']).hex()
    validators = [{'address': validator['address'],
                   'public_key': base64.b64decode(validator['pubKey']['value']).hex(),
                   'voting_power': int(validator['votingPower'])}
                  for validator in record['validators']['validators']]
    transaction = record['batch'][2:]
    proof = {'index': int(record['proof']['index']), 'total': int(record['proof']['total']),
             'data': transaction, 'aunts': [base64.b64decode(aunt).hex() for aunt in record['proof']['aunts']]}
    value = {'chain_id': header['chain_id'], 'transaction': transaction,
             'evidence': {'header': header, 'commit': commit, 'proof': proof}}
    trust = {'chain_id': header['chain_id'], 'height': record['height'], 'validators': validators}
    return value, trust, record['appHash'][2:]


def test_f015_chain_commitment(tmp_path):
    value, trust, root = chain_recording()
    first = DISCOVERY.admit_chain_evidence(value, trust, root)
    assert first.height == trust['height']
    assert first.app_hash.hex() == root
    path = private(tmp_path / 'committed.json', json.dumps(value))
    # Close all inputs, reload the consumer, and recover the identical committed observation.
    recovered = load('caps-discovery').admit_chain_evidence(json.loads(path.read_text()), trust, root)
    assert recovered == first


@pytest.mark.parametrize('mutation', ['network', 'root', 'height', 'header', 'transaction', 'proof', 'signature', 'validator', 'round'])
def test_f015_chain_commitment_refusals(mutation):
    value, trust, root = chain_recording()
    minimum = 1
    if mutation == 'network':
        value['chain_id'] += '-foreign'
    elif mutation == 'root':
        root = '00' * 32
    elif mutation == 'height':
        minimum = trust['height'] + 1
    elif mutation == 'header':
        value['evidence']['header']['app_hash'] = '00' * 32
    elif mutation == 'transaction':
        value['transaction'] = '00' + value['transaction'][2:]
    elif mutation == 'proof':
        value['evidence']['proof']['index'] = 1
    elif mutation == 'signature':
        value['evidence']['commit']['signatures'][0]['signature'] = '00' * 64
    elif mutation == 'validator':
        trust['validators'][0]['public_key'] = '00' * 32
    else:
        value['evidence']['commit']['round'] += 1
    with pytest.raises((RuntimeError, ValueError, InvalidSignature)):
        DISCOVERY.admit_chain_evidence(value, trust, root, minimum)


SOURCE_FILES = ('services/human/Cargo.toml', 'services/agent/Cargo.toml',
                'services/wallet/attestor/go.mod', 'programs/Cargo.toml',
                'services/agent/crates/layerx-agentd/src/lib.rs',
                'services/human/crates/layerx-human-service/src/health.rs',
                'rpc/px_discovery.go')


def source_checkout(directory):
    for name in SOURCE_FILES:
        private(directory / name, (ROOT / name).read_bytes())
    return CUSTODY.BUILD.capture_files(directory, SOURCE, SOURCE_FILES)


def publication(directory, module):
    checkout = directory / 'checkout'
    source = source_checkout(checkout)
    log = private(directory / 'build.log', (QUAL / 'build-caps-discovery.sh').read_bytes())
    original = Path(sys.executable).resolve(strict=True)
    artifacts = {}
    for target in module.TARGETS:
        target_path = directory / target
        shutil.copyfile(original, target_path)
        target_path.chmod(0o700)
        artifacts[target] = target_path
    path = CUSTODY.publish_test_manifest(directory / 'manifest.json', source, source,
                                         ['cargo', 'test', '--locked', '--no-run'], 0, log, artifacts, checkout)
    return path, checkout, source, artifacts, log


def manifest_round_trip(directory, name):
    module = load(name)
    path, checkout, source, artifacts, _ = publication(directory, module)
    observed = module.admit_build_manifest(path, source, checkout)
    assert observed == artifacts
    assert load(name).admit_build_manifest(path, source, checkout) == artifacts
    script = (QUAL / (name + '.py')).read_text()
    main = next(node for node in ast.parse(script).body if isinstance(node, ast.FunctionDef) and node.name == 'main')
    main_text = ast.get_source_segment(script, main)
    assert '.glob(' not in main_text
    assert main_text.index('admit_build_manifest(') < main_text.index('subprocess.run(')
    assert 'build_test_targets(' in main_text


def test_f045_projection_manifest(tmp_path):
    manifest_round_trip(tmp_path, 'human-projection-facts')


def test_f046_peer_manifest(tmp_path):
    manifest_round_trip(tmp_path, 'native-program-human-peer')


def test_f047_presentation_manifest(tmp_path):
    manifest_round_trip(tmp_path, 'native-program-presentation')


@pytest.mark.parametrize('name', ['human-projection-facts', 'native-program-human-peer', 'native-program-presentation'])
@pytest.mark.parametrize('mutation', ['revision', 'source', 'addition', 'executable', 'missing', 'extra', 'log', 'sidecar', 'public', 'link', 'nonexecutable'])
def test_source_bound_manifest_refusals(tmp_path, name, mutation):
    module = load(name)
    path, checkout, source, artifacts, log = publication(tmp_path, module)
    value = json.loads(path.read_text())
    current = deepcopy(source)
    if mutation == 'revision':
        current['candidate']['revision'] = '00' * 20
    elif mutation == 'source':
        changed = checkout / 'services/human/crates/layerx-human-service/src/health.rs'
        changed.write_bytes(changed.read_bytes() + b'\n// changed production source\n')
    elif mutation == 'addition':
        new = 'services/agent/crates/layerx-agentd/src/added.rs'
        private(checkout / new, (ROOT / SOURCE_FILES[4]).read_bytes())
        current = CUSTODY.BUILD.capture_files(checkout, SOURCE, (*SOURCE_FILES, new))
    elif mutation == 'executable':
        artifact = next(iter(artifacts.values()))
        artifact.write_bytes(artifact.read_bytes() + b'changed executable')
    elif mutation == 'missing':
        del value['artifacts'][module.TARGETS[0]]
        rewrite(path, value)
    elif mutation == 'extra':
        value['artifacts']['foreign-test'] = next(iter(value['artifacts'].values()))
        rewrite(path, value)
    elif mutation == 'log':
        log.write_bytes(log.read_bytes() + b'changed log')
    elif mutation == 'sidecar':
        sidecar = Path(next(iter(value['artifacts'].values()))['build_sidecar']['path'])
        sidecar.write_bytes(sidecar.read_bytes() + b' ')
    elif mutation == 'public':
        path.chmod(0o644)
    elif mutation == 'link':
        link = tmp_path / 'link.json'
        link.symlink_to(path)
        path = link
    else:
        next(iter(artifacts.values())).chmod(0o600)
    with pytest.raises((RuntimeError, ValueError, OSError)):
        module.admit_build_manifest(path, current, checkout)


def test_f020_transitive_inventory(tmp_path):
    source_checkout(tmp_path)
    for name in ('tools/qualification/paxeer-x/human_native_custody.py',
                 'apps/web/e2e/software-authenticator.ts',
                 'services/interop/crates/layerx-portable/src/lib.rs',
                 'services/platform/hosted/internal/Cargo.toml', 'go.mod', 'go.sum'):
        private(tmp_path / name, (ROOT / name).read_bytes())
    before = CUSTODY.inventory(tmp_path)
    assert set(SOURCE_FILES) <= set(before)
    digest = CUSTODY.source_digest(before)
    material = CUSTODY.custody_source_record(before, SOURCE['revision'])
    manifest = private(tmp_path / 'build-source.json', json.dumps(material))
    material = json.loads(manifest.read_text())
    assert CUSTODY.admit_custody_source(material, before, SOURCE['revision'], SOURCE['revision']) == digest
    for name in ('services/human/new-production.rs', 'services/agent/new-consumer.rs',
                 'services/wallet/attestor/new-consumer.go', 'programs/new-module.rs',
                 'services/interop/new-codec.rs', 'services/platform/new-transport.rs',
                 'rpc/new-codec.go', 'layerxproof/verify/new-proof.go'):
        path = private(tmp_path / name, (ROOT / SOURCE_FILES[4]).read_bytes())
        assert CUSTODY.source_digest(CUSTODY.inventory(tmp_path)) != digest
        with pytest.raises(RuntimeError, match='source binding'):
            CUSTODY.admit_custody_source(material, CUSTODY.inventory(tmp_path), SOURCE['revision'], SOURCE['revision'])
        path.unlink()
        assert CUSTODY.inventory(tmp_path) == before
    changed = tmp_path / SOURCE_FILES[4]
    original = changed.read_bytes()
    changed.write_bytes(original + b'\n// uncommitted change\n')
    assert CUSTODY.source_digest(CUSTODY.inventory(tmp_path)) != digest
    with pytest.raises(RuntimeError, match='source binding'):
        CUSTODY.admit_custody_source(material, CUSTODY.inventory(tmp_path), SOURCE['revision'], SOURCE['revision'])
    changed.write_bytes(original)
    assert CUSTODY.inventory(tmp_path) == before
    excluded = private(tmp_path / 'services/human/target/build-output', b'compiler output')
    assert excluded.is_file()
    assert CUSTODY.inventory(tmp_path) == before
    (tmp_path / SOURCE_FILES[4]).unlink()
    assert CUSTODY.source_digest(CUSTODY.inventory(tmp_path)) != digest
    with pytest.raises(RuntimeError, match='source binding'):
        CUSTODY.admit_custody_source(material, CUSTODY.inventory(tmp_path), SOURCE['revision'], SOURCE['revision'])


def test_f023_retained_executor(tmp_path):
    executable = Path(sys.executable).resolve(strict=True)
    document = {'schema': MOVEMENT.SCHEMA, 'revision': SOURCE['revision'], 'artifacts': {
        name: {'path': str(executable), 'sha256': MOVEMENT.digest(executable)} for name in MOVEMENT.ARTIFACTS}}
    assert MOVEMENT.admit_artifacts(document, SOURCE['revision']) == document['artifacts']
    files = {'probe': 'proof_human_kms', 'kms_cases': 'provider', 'deadline_cases': 'repair_22_33_25'}
    for target, test in MOVEMENT.CASES:
        source = ROOT / 'services/human/crates/layerx-human-kms/tests' / (files[target] + '.rs')
        assert '\nfn ' + test + '(' in source.read_text()
    tests = {test for _, test in MOVEMENT.CASES}
    assert {'proof_human_kms', 'executor_certificate_cannot_authorize_or_manage_keys',
            'external_signature_verification_and_executor_journal_recovery',
            'fragmented_handshake_and_frame_release_all_permits',
            'store_admission_has_the_same_deadline'} <= tests
    assert 'layerx-human-movement-provider' not in (QUAL / 'movement-executor-readiness.py').read_text()
    changed = deepcopy(document)
    changed['artifacts']['probe']['sha256'] = '00' * 32
    with pytest.raises(RuntimeError, match='changed'):
        MOVEMENT.admit_artifacts(changed, SOURCE['revision'])
    changed = deepcopy(document)
    del changed['artifacts']['probe']
    with pytest.raises(RuntimeError, match='inventory'):
        MOVEMENT.admit_artifacts(changed, SOURCE['revision'])
    with pytest.raises(RuntimeError, match='revision'):
        MOVEMENT.admit_artifacts(document, '00' * 20)


def test_compiler_artifact_selection_requires_real_test_targets():
    # Cargo's documented compiler-artifact transport, with the current executable
    # as an opaque digest input; neither a compiler nor a test process is invoked.
    row = {'reason': 'compiler-artifact', 'target': {'name': 'human_frame', 'kind': ['test']},
           'profile': {'test': True}, 'executable': str(Path(sys.executable).resolve())}
    output = json.dumps(row)
    assert CUSTODY.cargo_test_artifacts(output, ('human_frame',)) == {'human_frame': row['executable']}
    for altered in ({**row, 'executable': None}, {**row, 'profile': {'test': False}},
                    {**row, 'target': {'name': 'human_frame', 'kind': ['bin']}}):
        with pytest.raises(RuntimeError, match='compiler-emitted'):
            CUSTODY.cargo_test_artifacts(json.dumps(altered), ('human_frame',))
    with pytest.raises(RuntimeError, match='compiler-emitted'):
        CUSTODY.cargo_test_artifacts(output + '\n' + output, ('human_frame',))
    with pytest.raises(RuntimeError, match='compiler-emitted'):
        CUSTODY.cargo_test_artifacts(output, ('human_frame', 'native_program_human_peer'))


@pytest.mark.parametrize('failure', ['build-exit', 'source-changed'])
def test_manifest_producer_refuses_unqualified_build(tmp_path, failure):
    source = source_checkout(tmp_path / 'checkout')
    after = deepcopy(source)
    code = 1 if failure == 'build-exit' else 0
    if failure == 'source-changed':
        after['candidate']['revision'] = '00' * 20
    log = private(tmp_path / 'build.log', (QUAL / 'build-caps-discovery.sh').read_bytes())
    manifest = tmp_path / 'manifest.json'
    with pytest.raises((RuntimeError, ValueError)):
        CUSTODY.publish_test_manifest(manifest, source, after, ['cargo', 'test', '--no-run'],
                                      code, log, {'human_frame': Path(sys.executable).resolve()},
                                      tmp_path / 'checkout')
    assert not manifest.exists()
    assert not manifest.with_name(manifest.name + '.build.json').exists()
