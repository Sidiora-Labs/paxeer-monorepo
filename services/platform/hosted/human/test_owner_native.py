from contextlib import contextmanager
import copy
import hashlib
import json
from pathlib import Path
from types import SimpleNamespace
import subprocess
import sys
import tempfile
import unittest

from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

import guardians
import owner_native
import merged_owner
from merged_owner import LEDGER, OwnerClient, account_id, evm
from repair_server import LoopbackRpcServer
import provision


@contextmanager
def native_inputs(work):
    """Real merged producer inputs with a persisted, TLS loopback ledger peer."""
    server = LoopbackRpcServer(work)
    try:
        from test_repair_22_31_348 import bind, document, put
        funder, issuer = bind(server, bytes([11]) * 32, 11)
        salt = bytes([41]) * 32
        asset = hashlib.sha256(b'LX:ASSET:v1' + issuer + salt).hexdigest()
        server.rpc.send(11, 125, LEDGER, evm.calldata(
            'register(bytes32,bytes32,string,string,uint8,uint128,uint8,bytes)',
            '0x' + asset, salt, 'HUM', 'Human native', 0, 1000000, 1, b''))
        server.rpc.send(11, 125, LEDGER, evm.calldata('openAccount(bytes32)', '0x' + asset))
        source = account_id(funder, asset)
        server.rpc.send(11, 125, LEDGER, evm.calldata(
            'mint(bytes32,bytes32,uint128)', '0x' + asset, '0x' + source, 10000))
        secrets = work / 'secrets'
        secrets.mkdir(mode=0o700)
        for role in owner_native.GUARDIAN_ROLES:
            guardians.enroll(work, secrets, role, hashlib.sha256(role.encode()).hexdigest(), 1)
        guardians.assemble(work)
        root = work / 'human-evidence-input'
        seed = bytes([13]) * 32
        public = Ed25519PrivateKey.from_private_bytes(seed).public_key().public_bytes(Encoding.Raw, PublicFormat.Raw).hex()
        policy = provision.protected_json(root / 'recovery-policy.json')
        document(work / 'human-owner-result.json', dict(
            principal='did:layerx:' + public, did='did:layerx:' + public,
            recovery_root=policy['root'], recovery_threshold=policy['threshold'],
            recovery_delay_seconds=policy['delay_seconds']))
        put(root / 'owner.seed', seed)
        put(root / 'owner.evm', ('%064x' % 13).encode())
        put(root / 'funder.evm', ('%064x' % 11).encode())
        document(root / 'owner-native.json', dict(
            rpc_url=server.url, rpc_ca_file=str(server.ca), network_id=125, chain_id=125,
            confirmations=1, owner_seed_file=str(root / 'owner.seed'), evm_key_file=str(root / 'owner.evm'),
            asset_id=asset, funding_key_file=str(root / 'funder.evm'), funding_source=source, funding_amount=100))
        yield SimpleNamespace(work=work, inputs=root, server=server, asset=asset, source=source, public=public,
                              recovery=policy, account=account_id(public, asset))
    finally:
        server.close()


def republish(path, document):
    path.unlink()
    provision.write_json(path, document)


class NativeProducerRefusalTests(unittest.TestCase):
    def test_missing_native_inputs_name_the_file_and_publish_nothing(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            with self.assertRaises(provision.Refused) as refused:
                owner_native.produce(root)
            self.assertIn('owner-native.json', str(refused.exception))
            self.assertFalse((root / 'human-evidence-input/owner-registration.json').exists())

    def test_cli_missing_inputs_fail_without_traceback(self):
        with tempfile.TemporaryDirectory() as directory:
            result = subprocess.run([sys.executable, str(Path(provision.__file__)),
                                     '--produce-owner-registration', '--work-dir', directory],
                                    capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn('owner-native.json', result.stderr)
            self.assertNotIn('Traceback', result.stderr)

    def test_native_config_rejects_duplicate_and_unknown_fields(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            inputs = root / 'human-evidence-input'
            inputs.mkdir(mode=0o700)
            path = inputs / 'owner-native.json'
            for value in ('{"rpc_url":"a","rpc_url":"b"}', json.dumps({'unknown': True})):
                path.write_text(value)
                path.chmod(0o600)
                with self.assertRaises(provision.Refused):
                    owner_native.produce(root)
                self.assertFalse((inputs / 'owner-native-run').exists())
                self.assertFalse((inputs / 'owner-registration.json').exists())

    def test_reader_refuses_truncation_bounds_and_trailing_bytes(self):
        for data, maximum in ((b'\0\0\0\2a', 2), (b'\0\0\0\2ab', 1)):
            with self.assertRaises(provision.Refused):
                owner_native.Reader(data, 'native.receipt').span(maximum)
        with self.assertRaises(provision.Refused):
            owner_native.Reader(b'x', 'native.receipt').finish()

    def test_real_native_receipt_signature_and_tamper_refusals(self):
        path = Path(__file__).resolve().parents[4] / 'services/platform/hosted/authority/tests/fixtures/real-program-deploy-receipt.json'
        value = json.loads(path.read_text())
        raw = bytes.fromhex(value['receipt_hex'])
        public = bytes.fromhex(value['sequencer_public_key_hex'])
        result = owner_native.receipt_fields(raw, public, path)
        self.assertEqual(result['module'], 9)
        self.assertEqual(result['version'], 4)
        self.assertEqual(result['sequence'], 2)
        for changed in (raw[:-1], raw + b'\0', raw[:-1] + bytes([raw[-1] ^ 1])):
            with self.assertRaises(provision.Refused):
                owner_native.receipt_fields(changed, public, path)


class GuardianBindingConsumptionTests(unittest.TestCase):
    def test_signed_guardian_bindings_are_accepted_before_the_custody_credit(self):
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory).resolve()
            with native_inputs(work) as environment:
                root = environment.inputs
                document = json.loads((root / 'recovery-guardians.json').read_text())
                self.assertEqual(len(document['members']), len(owner_native.GUARDIAN_ROLES))
                owner_native.produce(work)
                registration = provision.owner_registration(work)
                self.assertEqual(registration['owner_account'], environment.account)
                self.assertEqual(environment.server.balance(environment.account)[0], 100)
                self.assertEqual(provision.funding_evidence(work)['funding_amount'], 100)

    def test_forged_guardian_binding_refuses_the_producer(self):
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory).resolve()
            with native_inputs(work) as environment:
                root = environment.inputs
                path = root / 'recovery-guardians.json'
                document = json.loads(path.read_text())
                member = document['members'][0]
                member['signature'] = '%0128x' % (int(member['signature'], 16) ^ 1)
                republish(path, document)
                with self.assertRaises(provision.Refused) as refused:
                    owner_native.produce(work)
                self.assertIn('owner-native.json', str(refused.exception))
                self.assertIn('guardian binding signature', str(refused.exception))
                self.assertEqual(len(environment.server.raw), 5)
                self.assertFalse((root / 'owner-native-run').exists())

    def test_unsigned_guardian_set_refuses_the_producer(self):
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory).resolve()
            with native_inputs(work) as environment:
                root = environment.inputs
                path = root / 'recovery-guardians.json'
                republish(path, dict(public_keys=json.loads(path.read_text())['public_keys']))
                with self.assertRaises(provision.Refused) as refused:
                    owner_native.produce(work)
                self.assertIn('owner-native.json', str(refused.exception))
                self.assertIn('guardian set', str(refused.exception))
                self.assertFalse((root / 'owner-native-run').exists())
                self.assertFalse((root / 'owner-registration.json').exists())
                self.assertEqual(len(environment.server.raw), 5)

    def test_substituted_guardian_set_refuses_the_recovery_commitment(self):
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory).resolve()
            with native_inputs(work) as environment:
                root = environment.inputs
                other = work / 'other'
                other.mkdir()
                secrets = other / 'secrets'
                secrets.mkdir(mode=0o700)
                for role in owner_native.GUARDIAN_ROLES:
                    guardians.enroll(other, secrets, role, hashlib.sha256(role.encode()).hexdigest(), 1)
                replacement = guardians.assemble(other)
                republish(root / 'recovery-guardians.json', replacement)
                with self.assertRaises(provision.Refused) as refused:
                    owner_native.produce(work)
                self.assertIn('signed guardian set', str(refused.exception))
                self.assertFalse((root / 'owner-registration.json').exists())
                self.assertEqual(len(environment.server.raw), 5)

    def test_rotated_guardian_set_is_accepted_with_its_own_recovery_policy(self):
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory).resolve()
            with native_inputs(work) as environment:
                root = environment.inputs
                secrets = work / 'secrets'
                guardians.enroll(work, secrets, 'sequencer', hashlib.sha256(b'sequencer').hexdigest(), 2)
                guardians.authorize(work, secrets, 'sequencer', 2)
                rotated = guardians.assemble(work, root / 'guardian-epoch-2')
                self.assertEqual(rotated['version'], 2)
                republish(root / 'recovery-guardians.json', rotated)
                republish(root / 'recovery-policy.json',
                          json.loads((root / 'guardian-epoch-2/recovery-policy.json').read_text()))
                owner = json.loads((work / 'human-owner-result.json').read_text())
                owner['recovery_root'] = json.loads((root / 'recovery-policy.json').read_text())['root']
                republish(work / 'human-owner-result.json', owner)
                owner_native.produce(work)
                registration = provision.owner_registration(work)
                proof = provision.funding_evidence(work)
                state = merged_owner.identity_state(proof['transactions']['recovery']['receipt'], environment.public)
                self.assertEqual(list(state[77:109]), owner['recovery_root'])
                self.assertEqual(registration['identity']['did'], 'did:layerx:' + environment.public)
                self.assertEqual(environment.server.balance(environment.account)[0], 100)


class MergedOwnerSuccessTests(unittest.TestCase):
    def test_actual_funding_and_resumed_identity(self):
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory).resolve()
            with native_inputs(work) as e:
                source_before = e.server.balance(e.source)[0]
                owner_native.produce(work)
                registration = provision.owner_registration(work, owner_did='did:layerx:' + e.public)
                proof = provision.funding_evidence(work)
                self.assertEqual(e.server.balance(e.account)[0], 100)
                self.assertEqual(e.server.balance(e.source)[0], source_before - 100)
                self.assertEqual(proof['funding_source'], e.source)
                self.assertEqual(proof['funding_amount'], 100)
                self.assertEqual(set(proof['transactions']), {'binding', 'identity', 'recovery', 'account', 'funding'})
                state = merged_owner.identity_state(proof['transactions']['recovery']['receipt'], e.public)
                self.assertEqual(list(state[77:109]), e.recovery['root'])
                self.assertEqual(int.from_bytes(state[109:111], 'big'), e.recovery['threshold'])
                self.assertEqual(int.from_bytes(state[183:191], 'big'), e.recovery['delay_seconds'])
                retained = {p.relative_to(work): p.read_bytes() for p in e.inputs.rglob('*') if p.is_file()}
                raw = dict(e.server.raw)
                identities = dict(e.server.identities)
                balances = copy.deepcopy(e.server.accounts)
                e.server.restart()
                owner_native.produce(work)
                reopened = provision.owner_registration(work, owner_did='did:layerx:' + e.public)
                self.assertEqual(reopened, registration)
                self.assertEqual(provision.funding_evidence(work), proof)
                context = provision.protected_json(e.inputs / 'native-context.json')
                client = OwnerClient(context, e.server.ca.read_bytes(), 13, e.inputs / 'owner-native-run')
                self.assertEqual(client.rpc.eth_call(LEDGER, 'balanceOf(bytes32,bytes32)(uint128)',
                    '0x' + e.account, '0x' + e.asset), [100])
                self.assertEqual(e.server.raw, raw)
                self.assertEqual(e.server.identities, identities)
                self.assertEqual(e.server.accounts, balances)
                self.assertEqual({p.relative_to(work): p.read_bytes() for p in e.inputs.rglob('*') if p.is_file()}, retained)

    def test_missing_funding_evidence_and_wrong_identity_are_refused(self):
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory).resolve()
            with native_inputs(work) as e:
                owner_native.produce(work)
                original = {p: p.read_bytes() for p in e.inputs.rglob('*') if p.is_file()}
                raw = dict(e.server.raw)
                evidence_path = e.inputs / 'merged-owner-evidence.json'
                evidence_path.unlink()
                with self.assertRaisesRegex(provision.Refused, 'merged-owner-evidence.json'):
                    provision.funding_evidence(work)
                owner_native.protected_write(evidence_path, original[evidence_path])
                with self.assertRaisesRegex(provision.Refused, 'DID binding'):
                    provision.owner_registration(work, owner_did='did:layerx:' + 'ff' * 32)
                owner_path = work / 'human-owner-result.json'
                owner = provision.protected_json(owner_path)
                republish(owner_path, dict(owner, did='did:layerx:' + 'ff' * 32))
                with self.assertRaisesRegex(provision.Refused, 'original owner DID'):
                    owner_native.produce(work)
                republish(owner_path, owner)
                self.assertEqual(e.server.raw, raw)
                self.assertEqual({p: p.read_bytes() for p in e.inputs.rglob('*') if p.is_file()}, original)
