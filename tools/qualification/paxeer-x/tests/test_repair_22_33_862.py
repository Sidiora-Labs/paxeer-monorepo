"""Task 22.33.862: file admission and recorded native-send path proofs.

The package sealing peer is a committed source/output file fixture, never a
substitute compiler. Browser and signing JavaScript is checked against its real
source contracts; this Python gate does not claim to execute Node or Chromium.
The actual HTTP consumer and SDK codec/verifier run against node-produced bytes,
with both sides stopped and reopened for recovery.
"""
from dataclasses import replace
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import stat
import sys

import pytest

QUAL = Path(__file__).resolve().parents[1]
ROOT = QUAL.parents[2]
sys.path.insert(0, str(QUAL))
from wallet_recovery_recording import RECORDING_TOKEN, RecordedGateway, transfer_recording


REPAIR_RECORD = {
    'task': '22.33.862',
    'findings': {
        'custody-recovery-build-path': 'test_custody_recovery_build_path_and_exact_seal',
        'custody-routing-bundle-private': 'test_private_bundle_transport_and_reopened_recovery',
        'signing-adapters-deleted-source': 'test_signing_adapters_current_native_source_and_driver',
        'demo-retired-custody': 'test_supported_demo_current_controls_and_exports',
    },
    'baseline': 'Not run: explicit gate order forbids a pre-implementation or direct pytest run.',
    'wiring_requests': [],
    'evidence': 'hf-done records the verified revision, command, exit and private log.',
}

# A source/output pair is file data for the sealer, not an executable replacement
# for the real SDK or gateway. Build command/config binding uses the real tree.
PACKAGE_FILES = {
    'package.json': '{"type":"module"}\n',
    'tsconfig.json': '{"compilerOptions":{"rootDir":"src","outDir":"dist"}}\n',
    'src/chain.ts': 'export const chainId = 125 as const;\n',
    'dist/chain.js': 'export const chainId = 125;\n',
    'dist/chain.d.ts': 'export declare const chainId: 125;\n',
}


def load(name):
    spec = importlib.util.spec_from_file_location('repair_862_' + name.replace('-', '_'), QUAL / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


RECOVERY = load('wallet-custody-recovery')
ROUTING = load('wallet-custody-routing')
SIGNING = load('wallet-signing-adapters')
DEMO = load('wallet-supported-demo')


def private(path, value):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    path.write_bytes(value if isinstance(value, bytes) else value.encode())
    path.chmod(0o600)
    return path


def package_tree(root):
    for package in ('sdk/wallet', 'services/wallet/gateway'):
        for relative, value in PACKAGE_FILES.items():
            private(root / package / relative, value)
    return root


def fixture_bundle(tmp_path):
    bundle = tmp_path / 'bundle'
    bundle.mkdir(mode=0o700)
    config = {
        'version': 1, 'isolated': True,
        'artifact_manifest': 'artifacts.json', 'attestor_source_manifest': 'attestors.json',
        'postgres_dump': 'retained.sql', 'node_binary': 'bin/node', 'postgres_bin_dir': 'bin/postgres',
        'services': [{'kind': 'chain', 'executable': 'bin/chain', 'argv': []}],
        'gateway_env': {'OWNER_TOKEN_FILE': '{fixture}/credentials/owner.token'},
    }
    private(bundle / 'wallet-custody-fixture.json', json.dumps(config))
    private(bundle / 'artifacts.json', '{}')
    private(bundle / 'attestors.json', '{}')
    # Opaque retained SQL input for file safety; no database executable is needed.
    private(bundle / 'retained.sql', (ROOT / 'services/wallet/gateway/migrations/019_drop_retired_layerx_custody.sql').read_bytes())
    private(bundle / 'credentials/owner.token', RECORDING_TOKEN)
    private(bundle / 'transfer.json', json.dumps(transfer_recording()))
    return bundle, config


def test_custody_recovery_build_path_and_exact_seal(tmp_path):
    assert RECOVERY.PACKAGE_PATHS == {'sdk': 'sdk/wallet', 'gateway': 'services/wallet/gateway'}
    commands = RECOVERY.build_commands('/approved/node')
    for package, (argv, cwd) in zip(RECOVERY.PACKAGE_PATHS.values(), commands):
        assert cwd == ROOT / package
        assert argv == ['/approved/node', str(cwd / 'node_modules/typescript/bin/tsc'), '-p', str(cwd / 'tsconfig.json')]
        config = json.loads(Path(argv[-1]).read_text())
        assert config['compilerOptions']['rootDir'] == 'src'
        assert config['compilerOptions']['outDir'] == 'dist'
        assert (cwd / 'package.json').is_file() and (cwd / 'src/index.ts').is_file()
    tree = package_tree(tmp_path / 'candidate')
    evidence = tmp_path / 'evidence'
    produced = RECOVERY.seal_candidate(evidence, tree)
    decoded = RECOVERY.load_candidate(evidence, tree)
    assert decoded == produced
    assert stat.S_IMODE((evidence / 'candidate.json').stat().st_mode) == 0o600
    assert set(decoded['artifacts']) == {package + '/' + name
        for package in RECOVERY.PACKAGE_PATHS.values() for name in PACKAGE_FILES if name.startswith('dist/')}
    for relative, digest in decoded['artifacts'].items():
        assert digest == hashlib.sha256((tree / relative).read_bytes()).hexdigest()
    # An obsolete output tree cannot enter the sealed set.
    private(tree / 'services/wallet/sdk/dist/retired.js', 'export const retired = true;')
    assert RECOVERY.load_candidate(evidence, tree) == produced


@pytest.mark.parametrize('relative', ['sdk/wallet/dist/chain.js', 'services/wallet/gateway/dist/chain.d.ts',
                                      'sdk/wallet/src/chain.ts', 'services/wallet/gateway/package.json'])
def test_package_seal_refuses_changed_source_or_generated_artifact(tmp_path, relative):
    tree = package_tree(tmp_path / 'candidate')
    RECOVERY.seal_candidate(tmp_path / 'evidence', tree)
    (tree / relative).write_text('changed after sealing')
    with pytest.raises(RuntimeError, match='seal mismatch'):
        RECOVERY.load_candidate(tmp_path / 'evidence', tree)


def test_package_seal_requires_both_output_trees(tmp_path):
    tree = package_tree(tmp_path / 'candidate')
    artifact = tree / 'sdk/wallet/dist/chain.js'
    artifact.unlink()
    artifact.symlink_to(tree / 'services/wallet/gateway/dist/chain.js')
    with pytest.raises(RuntimeError, match='symlinks'):
        RECOVERY.seal_candidate(tmp_path / 'evidence', tree)
    artifact.unlink()
    (tree / 'sdk/wallet/dist/chain.d.ts').unlink()
    with pytest.raises(RuntimeError, match='empty package'):
        RECOVERY.seal_candidate(tmp_path / 'evidence', tree)
    assert not (tmp_path / 'evidence/candidate.json').exists()


def test_private_bundle_transport_and_reopened_recovery(tmp_path):
    bundle, config = fixture_bundle(tmp_path)
    admitted, decoded = ROUTING.load_bundle(bundle)
    assert admitted == bundle and decoded == config
    copied = tmp_path / 'copied'
    ROUTING.copy_bundle(admitted, copied)
    assert ROUTING.load_bundle(copied)[1] == config
    for relative, directory in ROUTING.bundle_entries(copied):
        path = copied / relative
        assert path.stat().st_uid == os.getuid() and path.stat().st_mode & 0o077 == 0
        if not directory:
            assert path.read_bytes() == (bundle / relative).read_bytes()
    token = ROUTING.bundle_read(copied, 'credentials/owner.token').decode()
    case = json.loads(ROUTING.bundle_read(copied, 'transfer.json'))
    state = tmp_path / 'consumer.json'
    peer = RecordedGateway(tmp_path / 'gateway.json', case)
    with peer.serving() as url:
        client = RECOVERY.RecoveryQualification(url, token, state, case['rawTransaction'], case['pinnedKey'])
        assert client.submit()['must_not_resubmit'] is True
        assert client.recover() is None
        assert client.read_state()['phase'] == 'unknown'
    del client
    retained = peer.state.read_bytes()
    reopened = RecordedGateway(peer.state, case)
    with reopened.serving() as url:
        client = RECOVERY.RecoveryQualification(url, token, state, case['rawTransaction'], case['pinnedKey'])
        assert client.recover() == case['receiptJson']
        assert client.read_state()['phase'] == 'settled'
        with pytest.raises(RuntimeError, match='never resubmitted'):
            client.submit()
    assert reopened.state.read_bytes() == retained
    assert [path for path, _, _ in peer.requests] == ['/v1/wallet/send', '/rpc']
    assert [path for path, _, _ in reopened.requests] == ['/rpc']
    assert all(header == 'Bearer ' + RECORDING_TOKEN for _, header, _ in peer.requests + reopened.requests)


@pytest.mark.parametrize('relative', ['', 'credentials', 'credentials/owner.token', 'retained.sql',
                                      'wallet-custody-fixture.json', 'artifacts.json'])
def test_bundle_private_permissions_before_manifest_read(tmp_path, relative):
    bundle, _ = fixture_bundle(tmp_path)
    # Invalid JSON proves admission fails before even parsing the manifest.
    (bundle / 'wallet-custody-fixture.json').write_text('not JSON')
    path = bundle / relative
    path.chmod(0o755 if path.is_dir() else 0o644)
    with pytest.raises(RuntimeError, match='private caller-owned'):
        ROUTING.load_bundle(bundle)


@pytest.mark.parametrize('kind', ['file-link', 'directory-link', 'root-link', 'hardlink', 'fifo', 'env', 'owner'])
def test_bundle_refuses_unsafe_credentials_and_dump_entries(tmp_path, kind):
    bundle, _ = fixture_bundle(tmp_path)
    target = bundle / 'credentials/owner.token'
    if kind == 'file-link':
        target.unlink(); target.symlink_to(bundle / 'retained.sql')
    elif kind == 'directory-link':
        (bundle / 'alias').symlink_to(bundle / 'credentials', target_is_directory=True)
    elif kind == 'root-link':
        alias = tmp_path / 'alias'
        alias.symlink_to(bundle, target_is_directory=True)
        bundle = alias
    elif kind == 'hardlink':
        os.link(target, bundle / 'retained-token')
    elif kind == 'fifo':
        os.mkfifo(bundle / 'pipe', 0o600)
    elif kind == 'env':
        private(bundle / '.env.private', 'refused before any read')
    else:
        if os.getuid() == 0:
            os.chown(target, 1, -1)
        else:
            # A foreign-owned regular input available on every Unix host.
            target.unlink(); target.symlink_to('/etc/passwd')
    with pytest.raises((RuntimeError, OSError)):
        ROUTING.load_bundle(bundle)


@pytest.mark.parametrize('field,value', [
    ('artifact_manifest', '../outside.json'), ('attestor_source_manifest', '/etc/passwd'),
    ('postgres_dump', 'credentials/../retained.sql'), ('node_binary', '/usr/bin/node'),
    ('postgres_bin_dir', '../postgres'), ('postgres_dump', './retained.sql'),
])
def test_bundle_config_paths_stay_inside_private_root(tmp_path, field, value):
    bundle, config = fixture_bundle(tmp_path)
    config[field] = value
    private(bundle / 'wallet-custody-fixture.json', json.dumps(config))
    with pytest.raises(RuntimeError, match='contained relative'):
        ROUTING.load_bundle(bundle)


@pytest.mark.parametrize('value', ['/etc/passwd', '{fixture}/../outside', '{state}/../outside', 'prefix{fixture}/credentials/owner.token'])
def test_bundle_credential_paths_cannot_escape(tmp_path, value):
    bundle, config = fixture_bundle(tmp_path)
    config['gateway_env']['OWNER_TOKEN_FILE'] = value
    private(bundle / 'wallet-custody-fixture.json', json.dumps(config))
    with pytest.raises((RuntimeError, OSError)):
        ROUTING.load_bundle(bundle)


def test_bundle_rechecks_inputs_before_read_and_copy(tmp_path):
    bundle, _ = fixture_bundle(tmp_path)
    ROUTING.load_bundle(bundle)
    token = bundle / 'credentials/owner.token'
    token.unlink(); token.symlink_to(bundle / 'retained.sql')
    with pytest.raises((RuntimeError, OSError)):
        ROUTING.bundle_read(bundle, 'credentials/owner.token')
    with pytest.raises((RuntimeError, OSError)):
        ROUTING.copy_bundle(bundle, tmp_path / 'copied')
    assert not (tmp_path / 'copied').exists()


def test_signing_adapters_current_native_source_and_driver(tmp_path):
    paths = SIGNING.manifest_paths()
    assert 'apps/wallet/src/account/DepositFlow.tsx' not in paths
    assert 'services/wallet/gateway/test/e2e/wallet-signing-adapters.production.ts' not in paths
    assert set(SIGNING.NATIVE_FLOW_PATHS) <= paths
    assert 'tools/qualification/paxeer-x/wallet-signing-adapters.py' in paths
    assert all((ROOT / relative).is_file() for relative in paths)
    manifest = {relative: hashlib.sha256((ROOT / relative).read_bytes()).hexdigest() for relative in paths}
    SIGNING.validate_manifest(json.loads(json.dumps(manifest)))
    manifest['apps/wallet/src/account/KernelPlan.tsx'] = '0' * 64
    with pytest.raises(RuntimeError, match='source/prebuilt artifact mismatch'):
        SIGNING.validate_manifest(manifest)
    driver = SIGNING.DRIVER
    assert 'new sdk.WalletInterface' in driver and 'wallet.sendTransaction(tx)' in driver
    assert 'runtime.attestors.length, 5' in driver
    assert "control('stop-two')" in driver and "control('lose-quorum')" in driver
    assert "lastSend.status, 503" in driver
    assert "pending.must_not_resubmit, true" in driver
    assert "control('restart')" in driver and "saved.body.tx_hash" in driver
    assert "send_counts.every(count => count === 1)" in driver
    assert not re.search(r'(decode|encode)CustodyAuthorization|paxeer_prepareCustody|/sign-custody', driver)
    # Emitted driver transport carries the exact source-bound program.
    artifact = SIGNING.emit_driver(tmp_path)
    assert artifact.read_text() == driver
    assert stat.S_IMODE(artifact.stat().st_mode) == 0o600
    assert hashlib.sha256(artifact.read_bytes()).hexdigest() == hashlib.sha256(driver.encode()).hexdigest()


def test_supported_demo_current_controls_and_exports(tmp_path):
    contract = DEMO.validate_supported_surface()
    assert {'data-wallet-send', 'data-wallet-typed', 'data-kernel-plan'} <= set(contract['selectors'])
    assert {'EndpointClient', 'KernelClient', 'readSequencerReceipt', 'waitForSequencerReceipt'} <= set(contract['exports'])
    assert 'decodeCustodyAuthorization' not in DEMO.CONTRACT
    assert 'fixture.custody' not in DEMO.BROWSER and 'data-custody-' not in DEMO.BROWSER
    assert 'chromium.connectOverCDP' in DEMO.BROWSER
    assert 'await page.reload' in DEMO.BROWSER and 'await committed(retained)' in DEMO.BROWSER
    assert 'Reload recovery must poll' in DEMO.BROWSER
    assert 'changed.domain.chainId=126' in DEMO.BROWSER
    assert "[data-kernel-plan]" in DEMO.BROWSER and 'planned.plan_digest' in DEMO.BROWSER
    assert 'fixture.evm_transaction' in DEMO.BROWSER and 'await committed(evmHash)' in DEMO.BROWSER
    assert RECOVERY.assert_provider_contract()['code'] == 4200
    exported, browser = DEMO.emit_programs(tmp_path)
    assert exported.read_text() == DEMO.CONTRACT
    assert browser.read_text() == DEMO.BROWSER
    assert all(stat.S_IMODE(path.stat().st_mode) == 0o600 for path in (exported, browser))


def test_wrong_chain_and_tampered_receipts_never_settle(tmp_path):
    case = transfer_recording()
    raw, transaction, _, _ = RECOVERY.native_transaction(case['rawTransaction'])
    # Mutate only the chain through the real encoder. This negative input must
    # be refused before signature recovery or transport.
    _, signature = RECOVERY.decode_signed(raw)
    foreign = replace(transaction, chain_id=126)
    wrong_chain = foreign.attach_signature(signature, foreign.recover_signer(signature))
    with pytest.raises(RuntimeError, match='chain-125'):
        RECOVERY.native_transaction('0x' + wrong_chain.hex())
    corpus = json.loads((ROOT / 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json').read_text())
    assert corpus['provenance']['generator'] == 'rpc/sequencer_receipt_vectors_test.go'
    assert RECOVERY.native_transaction(case['rawTransaction'])[1].chain_id == 125
    peer = RecordedGateway(tmp_path / 'gateway.json', case)
    state = tmp_path / 'consumer.json'
    with peer.serving() as url:
        client = RECOVERY.RecoveryQualification(url, RECORDING_TOKEN, state, case['rawTransaction'], case['pinnedKey'])
        client.submit()
    reopened = RecordedGateway(peer.state, case)
    receipt = bytearray.fromhex(reopened.receipt['receipt'][2:])
    receipt[-1] ^= 1
    reopened.receipt['receipt'] = '0x' + receipt.hex()
    with reopened.serving() as url:
        client = RECOVERY.RecoveryQualification(url, RECORDING_TOKEN, state, case['rawTransaction'], case['pinnedKey'])
        with pytest.raises((RuntimeError, RECOVERY.SequencerReceiptVerificationError)):
            client.recover()
        assert client.read_state()['phase'] == 'unknown'
    assert [path for path, _, _ in reopened.requests] == ['/rpc']
    raw = bytearray(raw)
    raw[0] = 0
    with pytest.raises(ValueError):
        RECOVERY.native_transaction('0x' + raw.hex())


def test_task_record_maps_every_finding_to_an_executing_test():
    assert set(REPAIR_RECORD['findings']) == {'custody-recovery-build-path', 'custody-routing-bundle-private',
                                            'signing-adapters-deleted-source', 'demo-retired-custody'}
    assert all(callable(globals()[name]) for name in REPAIR_RECORD['findings'].values())
    assert REPAIR_RECORD['wiring_requests'] == []
