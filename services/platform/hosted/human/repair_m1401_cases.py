"""Focused real Python/shell paths selected by the required Go gate adapter."""
import copy
from contextlib import contextmanager
import hashlib
import importlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import time
import unittest

sys.dont_write_bytecode = True
os.environ['PYTHONDONTWRITEBYTECODE'] = '1'
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
sys.path.insert(0, str(HERE))

import history_material
import material
import merged_owner
import native_manifest
import owner_kms
import owner_native
import provision
from test_repair_22_31_348 import document, merged_environment, put, test_kms_network_registry


def snapshot(root):
    return {str(path.relative_to(root)): (path.read_bytes(), path.stat().st_mode & 0o777)
            for path in root.rglob('*') if path.is_file() and not path.is_symlink()}


def shell(script, **variables):
    return subprocess.run(['bash', '--noprofile', '--norc', '-c', 'set -euo pipefail\n' + script],
        env=dict(os.environ, REPO_ROOT=str(ROOT), **{key: str(value) for key, value in variables.items()}),
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, timeout=120)


@contextmanager
def configuration_environment(directory):
    changes = {'LAYERX_HUMAN_ONBOARDING_INITIAL_FUNDING': '100'}
    for name in ('TENANCY_DIGEST', 'AUTH_INDEX_KEY', 'STREAM_CURSOR_KEY'):
        key = 'LAYERX_HUMAN_' + name
        changes[key] = provision.protected_bytes(directory / key, 128).decode()
    before = {key: os.environ.get(key) for key in changes}
    os.environ.update(changes)
    try:
        yield
    finally:
        for key, value in before.items():
            if value is None:
                os.environ.pop(key, None)
            else:
                os.environ[key] = value


class RepairM1401(unittest.TestCase):
    @contextmanager
    def environment(self):
        with tempfile.TemporaryDirectory(prefix='human-m1401-') as temporary:
            with merged_environment(Path(temporary)) as e:
                yield e

    def case_HumanDocCustody(self):
        with self.environment() as e:
            guide = (HERE / 'README.md').read_text()
            current = guide.split('## Remaining cluster migration barriers\n', 1)[0]
            for name in ('layerx.human.merged-owner.v1', 'layerx.human.owner-bundle.v3',
                         'merged-owner-evidence.json', 'native-context.json', 'rpc-ca.pem',
                         'owner-registration.json', 'ledger-funding', '--funding-evidence',
                         'eth_getTransactionReceipt', 'eth_getBlockByNumber', 'ten seconds',
                         '65536 bytes', 'explicit migration', 'before broadcast'):
                self.assertIn(name, current)
            self.assertNotIn('ledger movement-source generation', current)
            self.assertNotIn('checkpoint_authority', current)
            self.assertEqual(set(provision.funding_evidence(e.work)['transactions']),
                             {'binding', 'identity', 'recovery', 'account', 'funding'})
            self.assertEqual(material.verify_bundle(e.bundle, 125, 125)['records'], 2)
            self.assertEqual(e.policy['principal_policy']['principals'][0]['identities'],
                             [e.registration['identity']])
            self.assertEqual(provision.protected_json(e.installed / 'identity/recovery-policy.json'), e.recovery)
            self.assertEqual(set(e.policy['movement']), {'LEDGER_ASSET_ID', 'LEDGER_OWNER_ACCOUNT',
                             'PAXEER_LEDGER_CONTRACT', 'PAXEER_PAYMENT_CONTRACT'})

    def case_HistoryFinalityMaterial(self):
        with self.environment() as e:
            before = snapshot(e.installed)
            history_material.produce(e.work, e.installed, 125, 125)
            self.assertEqual(snapshot(e.installed), before)
            for role in history_material.CHAIN_ROLES:
                self.assertEqual(history_material.verify(e.installed / role, e.context, 125, 125),
                                 provision.funding_evidence(e.work))
                for name in history_material.CHAIN_FILES:
                    self.assertEqual((e.installed / role / name).read_bytes(), (e.inputs / name).read_bytes())
            e.server.restart()
            importlib.reload(history_material)
            history_material.produce(e.work, e.installed, 125, 125)
            self.assertEqual(snapshot(e.installed), before)
            with self.assertRaisesRegex(ValueError, 'independently pinned'):
                history_material.verify(e.installed / 'agent', dict(e.context, chain_id=126), 125, 125)
            transaction = provision.funding_evidence(e.work)['transactions']['funding']['receipt']['transactionHash']
            original = copy.deepcopy(e.server.receipts[transaction])
            try:
                e.server.receipts[transaction]['status'] = '0x0'
                with self.assertRaisesRegex(ValueError, 'successful transaction receipt'):
                    history_material.produce(e.work, e.installed, 125, 125)
            finally:
                e.server.receipts[transaction] = original
            self.assertEqual(snapshot(e.installed), before)
            legacy = put(e.work / 'genesis/genesis-handover-trust.lxt', b'retained original trust')
            retained = snapshot(e.work)
            with self.assertRaisesRegex(ValueError, 'explicit migration'):
                history_material.produce(e.work, e.installed, 125, 125)
            self.assertEqual(snapshot(e.work), retained)
            self.assertEqual(legacy.read_bytes(), b'retained original trust')
            self.assertEqual(snapshot(e.installed), before)

    def case_MaterialShellTrustHistory(self):
        from cryptography import x509
        from cryptography.hazmat.primitives.serialization import Encoding
        with self.environment() as e:
            secrets = e.private / 'secrets'
            private = secrets / 'human'
            shutil.copytree(e.installed, private)
            put(private / 'kms/kms-seal', os.urandom(32))
            put(secrets / 'module-registry.json', e.registry.read_bytes())
            document(secrets / 'retained-context.json', dict(network_id=125, chain_id=125))
            ca = e.private / 'ca'
            put(ca / 'ca.key', (e.private / 'rpc-server.pem').read_bytes())
            put(ca / 'ca.crt', e.server.ca.read_bytes())
            certificate = x509.load_pem_x509_certificate(e.server.ca.read_bytes())
            put(ca / 'ca.der', certificate.public_bytes(Encoding.DER))
            result = shell('source "$REPO_ROOT/services/platform/hosted/human/material.sh"\n'
                           'retained_material_inventory save\nhuman_secrets_generate\n',
                           SECRETS_DIR=secrets, CA_DIR=ca)
            self.assertEqual(result.returncode, 0, result.stderr)
            before = snapshot(secrets)
            seal = (private / 'kms/kms-seal').read_bytes()
            result = shell('source "$REPO_ROOT/services/platform/hosted/human/material.sh"\n'
                           'human_secrets_generate\n', SECRETS_DIR=secrets, CA_DIR=ca)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(snapshot(secrets), before)
            for role in history_material.CHAIN_ROLES:
                history_material.verify(private / role, e.context, 125, 125)
                self.assertFalse((private / role / 'trust-history').exists())
            legacy = put(private / 'agent/trust-history', b'original pre-merge trust')
            result = shell('source "$REPO_ROOT/services/platform/hosted/human/material.sh"\n'
                           'retained_material_inventory save\nhuman_secrets_generate\n',
                           SECRETS_DIR=secrets, CA_DIR=ca)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('explicit migration', result.stderr)
            self.assertEqual((private / 'kms/kms-seal').read_bytes(), seal)
            self.assertEqual(legacy.read_bytes(), b'original pre-merge trust')
            script = (HERE / 'material.sh').read_text()
            self.assertNotIn('cp "$SECRETS_DIR/trust-history"', script)
            self.assertIn('history_material.py" "$WORK_DIR"', script)
            self.assertNotIn('handover-finality.conf', (HERE / 'material.py').read_text())

    def case_NativeManifestRetiredInput(self):
        import yaml
        with self.environment() as e:
            config = provision.protected_json(e.inputs / 'owner-native.json')
            document(e.inputs / 'owner-evm.json', config)
            node = next(value for value in yaml.safe_load_all((ROOT / 'services/platform/hosted/node/deployment.yaml').read_text())
                        if value['kind'] == 'StatefulSet')
            before = copy.deepcopy(node)
            retained = (e.inputs / 'owner-native.json').read_bytes()
            recovery = (e.inputs / 'recovery-policy.json').read_bytes()
            for _ in range(2):
                returned = native_manifest.configure(e.work, 125, None, None, node)
                self.assertIs(returned, node)
                self.assertEqual(node, before)
                importlib.reload(native_manifest)
            self.assertEqual((e.inputs / 'owner-native.json').read_bytes(), retained)
            self.assertEqual((e.inputs / 'recovery-policy.json').read_bytes(), recovery)
            self.assertFalse((e.inputs / 'custody-credit.bin').exists())
            for changes in ({'network_id': 126}, {'rpc_url': 'http://localhost'},
                            {'unexpected': True}, {'funding_amount': 0}, {'chain_id': True}):
                (e.inputs / 'owner-evm.json').write_bytes(provision.encode_json(dict(config, **changes)))
                with self.assertRaises(ValueError):
                    native_manifest.configure(e.work, 125, None, None, node)
                self.assertEqual(node, before)
                self.assertEqual((e.inputs / 'owner-native.json').read_bytes(), retained)
            (e.inputs / 'owner-evm.json').write_bytes(provision.encode_json(config))
            ca = Path(config['rpc_ca_file'])
            ca.chmod(0o640)
            try:
                with self.assertRaisesRegex(ValueError, 'protected bounded file'):
                    native_manifest.configure(e.work, 125, None, None, node)
            finally:
                ca.chmod(0o600)
            changed = dict(config, funding_amount=config['funding_amount'] + 1)
            (e.inputs / 'owner-evm.json').write_bytes(provision.encode_json(changed))
            with self.assertRaisesRegex(ValueError, 'identical retained'):
                native_manifest.configure(e.work, 125, None, None, node)
            (e.inputs / 'owner-evm.json').write_bytes(provision.encode_json(config))
            put(e.inputs / 'custody-credit.bin', b'original custody record')
            before_files = snapshot(e.work)
            with self.assertRaisesRegex(ValueError, 'explicit migration'):
                native_manifest.configure(e.work, 125, None, None, node)
            self.assertEqual(snapshot(e.work), before_files)

    def case_OwnerKmsPrepareBounds(self):
        with self.environment() as e:
            peer_root = e.private / 'child-peer'
            peer_root.mkdir(mode=0o700)
            peer = peer_root / 'prepare-peer'
            shutil.copyfile(HERE / 'repair_m1401_peer.py', peer)
            peer.chmod(0o700)
            request = dict(email='owner@example.test', display_name='Owner', idempotency_key='retained-owner')
            document(e.inputs / 'owner-request.json', dict(request, now=1))
            configuration = e.installed / 'config'
            state = put(e.private / 'retained-kms-state', os.urandom(128))
            original = snapshot(e.work)
            custody = state.read_bytes()
            for mode in ('stall', 'closed-pipes', 'stdout', 'stderr', 'refuse'):
                document_path = peer_root / 'peer-control.json'
                document_path.write_bytes(provision.encode_json(dict(mode=mode, request=request)))
                document_path.chmod(0o600)
                started = time.monotonic()
                with configuration_environment(configuration):
                    with self.assertRaisesRegex(ValueError, 'deadline|output exceeds bound|real KMS sponsor preparation'):
                        owner_kms.prepare(e.work, str(peer), configuration)
                elapsed = time.monotonic() - started
                self.assertLess(elapsed, owner_kms.PREPARE_DEADLINE_SECONDS + 3)
                if mode in ('stall', 'closed-pipes'):
                    self.assertGreaterEqual(elapsed, owner_kms.PREPARE_DEADLINE_SECONDS - 0.1)
                self.assertEqual(snapshot(e.work), original)
                self.assertEqual(state.read_bytes(), custody)
                pid = int((peer_root / 'peer-pid').read_text())
                with self.assertRaises(ProcessLookupError):
                    os.kill(pid, 0)
                self.assertFalse((e.inputs / 'owner-kms.json').exists())
            # Success transports a real merged-owner producer output through the
            # same bounded pipes. The current Rust KMS exporter mismatch is an
            # explicit owning-domain wiring request, not a fabricated export.
            (peer_root / 'peer-control.json').write_bytes(provision.encode_json(dict(
                mode='success', request=request, producer_output=str(e.inputs / 'owner-registration.json'))))
            actual = owner_kms.prepare_child(str(peer), request)
            self.assertEqual(actual, (e.inputs / 'owner-registration.json').read_bytes())
            self.assertEqual(json.loads(actual), e.registration)
            self.assertEqual(snapshot(e.work), original)
            self.assertEqual(state.read_bytes(), custody)
            # Run the existing real TLS/registry success and refusal coverage.
            test_kms_network_registry(e)

    def case_ProvisionMovementSource(self):
        with self.environment() as e:
            evidence = provision.funding_evidence(e.work)
            self.assertEqual(evidence['asset_id'], e.asset)
            self.assertEqual(evidence['funding_source'], e.source)
            self.assertEqual(evidence['funding_amount'], 100)
            manifest = e.work / 'producer-manifest.json'
            self.assertTrue(provision.producer_manifest(e.work, e.registry, e.work / 'registry-journal', manifest))
            entries = provision.protected_json(manifest)['inputs']
            funding, = [entry for entry in entries if entry['input'] == 'ledger-funding']
            self.assertEqual(funding['path'], 'human-evidence-input/merged-owner-evidence.json')
            self.assertEqual(funding['sha256'], hashlib.sha256((e.inputs / 'merged-owner-evidence.json').read_bytes()).hexdigest())
            self.assertEqual(funding['status'], 'present')
            self.assertNotIn('movement-policy', {entry['input'] for entry in entries})
            before = snapshot(e.work / 'human-evidence')
            context = (e.inputs / 'native-context.json').read_bytes()
            try:
                changed = dict(e.context, rpc_ca_sha256='ab' * 32)
                (e.inputs / 'native-context.json').write_bytes(provision.encode_json(changed))
                with self.assertRaisesRegex(ValueError, 'independently pinned'):
                    provision.funding_evidence(e.work)
            finally:
                (e.inputs / 'native-context.json').write_bytes(context)
            config = (e.inputs / 'owner-native.json').read_bytes()
            try:
                value = json.loads(config)
                value['funding_amount'] += 1
                (e.inputs / 'owner-native.json').write_bytes(provision.encode_json(value))
                with self.assertRaisesRegex(ValueError, 'configured ledger funding binding'):
                    provision.funding_evidence(e.work)
            finally:
                (e.inputs / 'owner-native.json').write_bytes(config)
            provision.assemble(e.work, e.registry, e.asset, e.work / 'registry-journal')
            self.assertEqual(snapshot(e.work / 'human-evidence'), before)
            self.assertEqual(material.verify_bundle(e.bundle, 125, 125)['records'], 2)
            legacy = put(e.inputs / 'movement-source.json', b'original legacy funding input')
            with self.assertRaisesRegex(ValueError, 'explicit migration'):
                provision.funding_evidence(e.work)
            self.assertEqual(legacy.read_bytes(), b'original legacy funding input')
            self.assertEqual(snapshot(e.work / 'human-evidence'), before)

    def case_ProvisionRetiredDaemon(self):
        with self.environment() as e:
            document(e.inputs / 'owner-evm.json', provision.protected_json(e.inputs / 'owner-native.json'))
            transactions = dict(e.server.raw)
            balances = {account: e.server.balance(account) for account in (e.source, e.registration['owner_account'])}
            before = snapshot(e.inputs / 'owner-native-run')
            for _ in range(2):
                result = shell('source "$REPO_ROOT/services/platform/hosted/human/provision.sh"\n'
                               'human_native_owner_prepare\nhuman_native_provision\n',
                               WORK_DIR=e.work, NODE_NETWORK_ID=125, PAXEER_CHAIN_ID=125)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(dict(e.server.raw), transactions)
                self.assertEqual({account: e.server.balance(account) for account in balances}, balances)
                self.assertEqual(snapshot(e.inputs / 'owner-native-run'), before)
                self.assertEqual(provision.owner_registration(e.work), e.registration)
                e.server.restart()
            state = merged_owner.identity_state(provision.funding_evidence(e.work)['transactions']['recovery']['receipt'], e.public)
            self.assertEqual(list(state[77:109]), e.recovery['root'])
            self.assertEqual(int.from_bytes(state[109:111], 'big'), e.recovery['threshold'])
            self.assertEqual(int.from_bytes(state[183:191], 'big'), e.recovery['delay_seconds'])
            source = (HERE / 'provision.sh').read_text()
            for retired in ('-c layerxd', "==\'layerxd\'", 'layerxctl', 'layerx-guarantor-checkpoint-authority',
                            'guarantor-id', '--movement-source'):
                self.assertNotIn(retired, source)
            record = (HERE / 'repair_22_33_1401.kvx').read_text()
            self.assertIn('registry_producer', record)
            self.assertIn('services/platform/hosted/node/deployment.yaml', record)

    def case_MaterialTestRetiredNode(self):
        from test_material import MaterialTests
        names = ('test_ledger_funding_binds_current_bundle_and_retained_state',
                 'test_payment_components_ignore_retired_addresses_and_refuse_custody_policy',
                 'test_bootstrap_has_no_unpublished_human_secret_dependencies',
                 'test_generated_bootstrap_and_protected_inputs',
                 'test_owner_policy_refuses_malformed_zero_and_legacy_limit_ids')
        suite = unittest.TestSuite(MaterialTests(name) for name in names)
        result = unittest.TextTestRunner(verbosity=2).run(suite)
        self.assertEqual(result.testsRun, len(names))
        self.assertFalse(result.skipped)
        self.assertTrue(result.wasSuccessful())


if __name__ == '__main__':
    assert len(sys.argv) == 2
    suite = unittest.TestSuite([RepairM1401('case_' + sys.argv[1])])
    result = unittest.TextTestRunner(verbosity=2).run(suite)
    if result.testsRun != 1 or result.skipped or not result.wasSuccessful():
        raise SystemExit(1)
