"""Task 22.33.861: qualification clients, real codecs, durable loopback peers.

Task record (kept here because no spec or separate record path is in scope):
- Base: aa5e0820b3652890f3812a220f4b91f4f8a45387.
- Baseline: no unchanged-tree test run is claimed. The explicit gate order
  prohibits testing before implementation and running verify_cmd by hand.
- movement-profile-custody: test_movement_bank_proof_survives_stop_reopen,
  test_movement_refuses_authority_substitution, test_movement_refuses_uncommitted_or_forged,
  test_movement_committed_proof_pin_refuses_changed_recovery.
- transfer-evidence-private: test_transfer_requires_explicit_private_directory,
  test_transfer_exclusive_private_outputs, test_transfer_refuses_checkout_and_symlink.
- all-form-abi-five: test_all_form_inventory_and_real_lifecycle_codec,
  test_all_form_refusal_evidence_remains_strict.
- language-abi-five: test_language_tables_match_every_frozen_host_surface,
  test_language_linker_requires_historical_and_v5_guests.
- native-interface-abi-five: test_native_registry_sdk_consumer_and_reopen,
  test_native_registry_refuses_substituted_interface, test_native_case_inventory_includes_v5.
- native-interface-retired-artifacts: test_native_merged_manifest_and_artifact_identity.
- ramp-batch-authority: test_ramp_committed_destination_and_recovery,
  test_ramp_refuses_wrong_order_bindings, test_ramp_refuses_forged_authority,
  test_ramp_order_digest_preserves_context_and_principals.
- identity-router-prerequisite: test_identity_direct_gateway_node_and_reopen,
  test_identity_refuses_foreign_chain_and_conflicting_owner.

Wiring requests (no out-of-scope source is changed):
- Programs SDK domain 13, programs/sdk/abi-manifest.tsv: publish the ABI-5 row
  from canonical host tables with coordinated C/AssemblyScript lint support;
  the historical TSV remains exactly checked, and an explicit complete private
  manifest is produced for qualification, with all five frozen vectors checked.
- Programs/registry domains 13/8, tools/qualification/paxeer-x/program-interface-producer.py
  and services/platform/cli/tests/real_node/native_interfaces.rs: emit the v2 merged
  paxd/registry/bank-funding manifest and all ABI-5 process/consumer/restart cases.
- Programs domain 13, programs/crates/layerx-programs-runtime/src/ffi_interface.rs
  and programs/crates/layerx-programs-registry/src/interface.rs: add the missing
  ABI-5 native interface domain/admission; current parsers end at DOMAIN_V4.
- Human/hosted domains, services/platform/hosted/human/material.py: provision
  bank/chain/genesis/receipt-key authority instead of the retired custody projection.
- Ramp domain 8, services/platform/ramps/toolkit/src/lib.rs and its receipt-verifier
  producer: connect the committed destination proof contract; preserve independent
  source verification and durable Rust order/context binding and recovery.
- Gateway domain 8, unified identity fixture producer: bind gateway and paxd
  executable digests and supply direct node/genesis plus reopened gateway targets.

Proof boundary: original signed node receipts are replayed unchanged; the Python
SDK verifies their signatures, transaction identity and signed logs. In-process
peers persist records in SQLite and are stopped and reopened. Registry transport
uses the SDK lifecycle producer, canonical ABI encoder, real HTTP and SDK interface
consumer. Native C execution, Rust host instantiation, generated language binaries,
consensus inclusion and live funded deployments are not executed by this Python
only gate. The qualifier continues to require those native artifacts and cases.
Gotcha: baseline and separate record/ledger edits conflict with the explicit scope
and gate order; observations and wiring requests are recorded here instead.
"""
from contextlib import contextmanager
from copy import deepcopy
from dataclasses import replace
from datetime import datetime, timedelta, timezone
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
import os
from pathlib import Path
import re
import sqlite3
import ssl
import stat
import sys
import threading
import urllib.request

import pytest
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.x509.oid import NameOID

QUAL = Path(__file__).resolve().parents[1]
ROOT = QUAL.parents[2]
sys.path.insert(0, str(QUAL))
import merged_qualification as merged
from layerx_sdk.evm_transaction import decode_signed, encode_call
from layerx_sdk.program_lifecycle import NativeProgramDeploy
from layerx_sdk.verifier import SequencerReceiptVerificationError


def load(name):
    spec = importlib.util.spec_from_file_location('repair_861_' + name.replace('-', '_'), QUAL / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


MOVEMENT = load('movement-custody-profile')
TRANSFER = load('program_transfer_authority')
ALL_FORM = load('programs_all_form_validation')
LANGUAGE = load('programs_language_abi')
NATIVE = load('programs_native_interfaces')
RAMP = load('ramp-source-settlement')
IDENTITY = load('unified-account-identity')
AUTH = 'Bearer public-recording-session-22-33-861'
GENESIS = '0x' + 'ab' * 32


def write_private(path, value):
    path = Path(path)
    data = value if isinstance(value, bytes) else json.dumps(value).encode()
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as stream:
        stream.write(data)
    return path


def corpus():
    value = json.loads((ROOT / 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json').read_text())
    assert value['provenance']['generator'] == 'rpc/sequencer_receipt_vectors_test.go'
    return {row['id']: deepcopy(row) for row in value['cases']
            if row['id'] in ('V1-pax-transfer', 'V1-layerxledger-pay-exact', 'V1-layerxprograms-call')}


def pins(record):
    key = record['pinnedKey']
    return [{'key_id': key['keyId'][2:], 'public_key': key['publicKey'][2:],
             'first_height': key['firstHeight'], 'last_height': key['lastHeight']}]


def result_abi(types, values):
    return '0x' + encode_call('recordedResult', types, values)[4:].hex()


class Peer:
    """Recorded node and in-process registry; persisted codec output, no replacement receipt signatures."""
    def __init__(self, path, tls=None):
        self.path, self.tls = path, tls
        self.requests, self.errors = [], []
        self.damage = None
        self.db = sqlite3.connect(path, check_same_thread=False)
        self.db.execute('CREATE TABLE IF NOT EXISTS state (id INTEGER PRIMARY KEY, value TEXT NOT NULL)')
        row = self.db.execute('SELECT value FROM state WHERE id=1').fetchone()
        self.data = json.loads(row[0]) if row else {
            'chain': '0x7d', 'genesis': GENESIS, 'records': corpus(), 'programs': {}, 'identity': None}
        self.persist()
        self.server = None

    def persist(self):
        self.db.execute('INSERT OR REPLACE INTO state VALUES (1, ?)', (json.dumps(self.data, sort_keys=True),))
        self.db.commit()

    def node(self):
        context = ssl.create_default_context(cafile=self.tls['ca']) if self.tls else None
        return merged.CHAIN.Node(self.url, pins(self.data['records']['V1-pax-transfer']),
                                 {'Authorization': AUTH}, context)

    def execution(self, record):
        row = deepcopy(record['receiptJson'])
        row.update(rung='committed', blockNumber=row['height'],
                   blockHash='0x' + hashlib.sha256(('recorded-block-' + row['height']).encode()).hexdigest())
        return row

    def record(self, reference):
        return next((row for row in self.data['records'].values() if row['receiptJson']['txHash'] == reference), None)

    def publish(self, deploy):
        # Transport payload was emitted by the real SDK; decode it before storing registry metadata.
        decoded = NativeProgramDeploy.decode(deploy.encode())
        self.data['programs'][decoded.program_id.hex()] = {
            'payload': decoded.encode().hex(), 'version': 1,
            'deployment_receipt_digest': self.data['records']['V1-layerxprograms-call']['receiptDigest'][2:]}
        self.persist()
        return decoded

    def interface(self, program):
        entry = self.data['programs'][program]
        deploy = NativeProgramDeploy.decode(bytes.fromhex(entry['payload']))
        return {'program_id': program, 'version': entry['version'], 'code_hash': deploy.new_hash.hex(),
                'abi_version': deploy.guest_abi, 'interface': deploy.interface.hex(),
                'interface_digest': hashlib.sha256(deploy.interface).hexdigest(),
                'deployment_receipt_digest': entry['deployment_receipt_digest'],
                'state_root': 'cd' * 32, 'observed_sequence': 12, 'observed_at': 1000,
                'valid_through': 2000, 'source': {'status': 'unpublished'},
                'verification': 'deployment-interface-and-current-head-verified'}

    def identity(self, method, params):
        account = self.data['identity']
        assert params[0] in (account['evm_address'], account['layerx_did'])
        if method == 'px_resolveAccount':
            return deepcopy(account)
        result = {'account': deepcopy(account)}
        if method == 'px_getAccount':
            result.update(paxeer={'address': account['evm_address']},
                          layerx={'account_id': account['layerx_account'],
                                  'name': 'agent:' + account['layerx_did'] + ':main'})
        elif method == 'px_getBalances':
            result['balances'] = [{'layerx': {'account_id': account['layerx_account'],
                'name': 'agent:' + account['layerx_did'] + ':main', 'balance': '1000000'}}]
        else:
            assert method == 'px_getUnifiedHistory'
            result['accounts'] = [{'side': 'paxeer', 'account': account['evm_address']},
                                  {'side': 'layerx', 'account': account['layerx_account']}]
            result['items'] = [{'transaction_hash': self.data['records']['V1-pax-transfer']['receiptJson']['txHash']}]
        return result

    def rpc(self, request):
        assert set(request) == {'jsonrpc', 'id', 'method', 'params'} and request['jsonrpc'] == '2.0'
        assert type(request['id']) is int and isinstance(request['params'], list)
        method, params = request['method'], request['params']
        if method == 'eth_chainId':
            result = self.data['chain']
        elif method == 'eth_getBlockByNumber':
            assert params[1] is False
            if params[0] == '0x0':
                result = {'number': '0x0', 'hash': self.data['genesis'], 'transactions': []}
            else:
                rows = [self.execution(record) for record in self.data['records'].values()]
                row = next(row for row in rows if row['blockNumber'] == params[0])
                result = {'number': row['blockNumber'], 'hash': row['blockHash'],
                          'transactions': [item['txHash'] for item in rows if item['blockNumber'] == params[0]]}
        elif method in ('pax_sequencerReceipt', 'eth_getRawTransactionByHash', 'eth_getTransactionReceipt'):
            record = self.record(params[0])
            if record is None:
                return {'jsonrpc': '2.0', 'id': request['id'], 'error': {'code': -32001, 'message': 'unknown transaction'}}
            row = self.execution(record)
            result = row if method == 'pax_sequencerReceipt' else record['rawTransaction']
            if method == 'eth_getTransactionReceipt':
                result = {'transactionHash': row['txHash'], 'blockHash': row['blockHash'],
                          'blockNumber': row['blockNumber'], 'transactionIndex': row['index'],
                          'status': row['status'], 'from': row['from'], 'to': row['to'], 'logs': row['logs']}
        elif method == 'eth_call':
            assert params[1] in ('latest', 'finalized') and params[0]['to'] == merged.CHAIN.PROGRAMS
            raw = bytes.fromhex(params[0]['data'][2:])
            program = raw[4:36].hex()
            entry = self.data['programs'][program]
            deploy = NativeProgramDeploy.decode(bytes.fromhex(entry['payload']))
            if raw == encode_call('getProgram', ['bytes32'], [deploy.program_id]):
                result = result_abi(['uint8', 'bytes32', 'bytes32', 'uint16', 'uint32', 'bool'],
                                    [deploy.policy, deploy.authority, deploy.new_hash, deploy.guest_abi, entry['version'], True])
            else:
                assert raw == encode_call('getInterface', ['bytes32'], [deploy.program_id])
                result = result_abi(['uint32', 'bytes32', 'bytes'],
                                    [entry['version'], hashlib.sha256(deploy.interface).digest(), deploy.interface])
        elif method.startswith('px_'):
            result = self.identity(method, params)
        else:
            raise AssertionError('unexpected method: ' + method)
        answer = {'jsonrpc': '2.0', 'id': request['id'], 'result': result}
        if self.damage:
            self.damage(method, answer)
        return answer

    @contextmanager
    def serving(self):
        peer = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def reply(self, code, result):
                raw = json.dumps(result, separators=(',', ':')).encode()
                self.send_response(code)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)

            def do_POST(self):
                try:
                    assert self.path == '/' and self.headers.get_content_type() == 'application/json'
                    assert self.headers.get('Authorization') in (AUTH, None)
                    length = int(self.headers['Content-Length'])
                    assert 0 < length <= 1_048_576
                    request = json.loads(self.rfile.read(length))
                    peer.requests.append((self.path, self.headers.get('Authorization'), deepcopy(request)))
                    self.reply(200, peer.rpc(request))
                except Exception as error:
                    peer.errors.append(error)
                    self.reply(500, {'error': 'recording mismatch'})

            def do_GET(self):
                try:
                    match = re.fullmatch(r'/v1/programs/registry/([0-9a-f]{64})/interface', self.path)
                    assert match and self.headers.get('Authorization') == AUTH
                    peer.requests.append((self.path, self.headers.get('Authorization'), None))
                    value = peer.interface(match[1])
                    if peer.damage:
                        peer.damage('registry-interface', value)
                    self.reply(200, value)
                except Exception as error:
                    peer.errors.append(error)
                    self.reply(500, {'error': 'recording mismatch'})

        self.server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        if self.tls:
            context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
            context.load_cert_chain(self.tls['cert'], self.tls['key'])
            self.server.socket = context.wrap_socket(self.server.socket, server_side=True)
        self.url = ('https' if self.tls else 'http') + '://127.0.0.1:' + str(self.server.server_port)
        thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        thread.start()
        try:
            yield self
        finally:
            self.server.shutdown()
            self.server.server_close()
            thread.join(timeout=5)
            assert not thread.is_alive()
            self.db.close()
            assert not self.errors


@pytest.fixture
def tls(tmp_path):
    import ipaddress
    key = ec.generate_private_key(ec.SECP256R1())
    name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, 'Qualification loopback')])
    now = datetime.now(timezone.utc)
    cert = (x509.CertificateBuilder().subject_name(name).issuer_name(name).public_key(key.public_key())
            .serial_number(x509.random_serial_number()).not_valid_before(now - timedelta(minutes=1))
            .not_valid_after(now + timedelta(days=1))
            .add_extension(x509.BasicConstraints(ca=True, path_length=None), True)
            .add_extension(x509.SubjectAlternativeName([x509.IPAddress(ipaddress.ip_address('127.0.0.1'))]), False)
            .sign(key, hashes.SHA256()))
    cert_path = write_private(tmp_path / 'tls.pem', cert.public_bytes(serialization.Encoding.PEM))
    key_path = write_private(tmp_path / 'tls-key.pem', key.private_bytes(
        serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, serialization.NoEncryption()))
    return {'ca': str(cert_path), 'cert': str(cert_path), 'key': str(key_path)}


def bank_fixture(tmp_path):
    record = corpus()['V1-pax-transfer']
    profile = {'version': 1, 'chain_id': 125, 'genesis_hash': GENESIS, 'module': 'bank',
               'asset': 'uhpx', 'owner': record['receiptJson']['from'], 'receipt_keys': pins(record)}
    path = write_private(tmp_path / 'bank.json', profile)
    state = tmp_path / 'movement-state'
    state.mkdir(mode=0o700)
    return record, profile, path, state


def test_movement_bank_proof_survives_stop_reopen(tmp_path):
    record, profile, path, state = bank_fixture(tmp_path)
    reference = record['receiptJson']['txHash']
    with Peer(tmp_path / 'peer.db').serving() as peer:
        result = MOVEMENT.BankMovement(peer.node(), path, state).prove(reference, record['receiptJson']['to'], 10**18)
        assert result['proof']['transaction_hash'] == reference
        assert result['proof']['receipt_digest'] == record['receiptDigest'][2:]
        assert all(header == AUTH for _, header, _ in peer.requests)
    with Peer(tmp_path / 'peer.db').serving() as peer:
        assert MOVEMENT.BankMovement(peer.node(), path, state).prove(reference, record['receiptJson']['to'], 10**18) == result
    assert json.loads((state / 'bank-authority.pin').read_text()) == profile
    assert stat.S_IMODE((state / (reference[2:] + '.proof')).stat().st_mode) == 0o600


@pytest.mark.parametrize('field,value', [('chain_id', 126), ('module', 'custody'), ('asset', 'foreign'),
    ('genesis_hash', '0x' + 'cc' * 32), ('owner', '0x' + '11' * 20), ('receipt_keys', [])])
def test_movement_refuses_authority_substitution(tmp_path, field, value):
    record, profile, path, state = bank_fixture(tmp_path)
    with Peer(tmp_path / 'peer.db').serving() as peer:
        MOVEMENT.BankMovement(peer.node(), path, state).prove(record['receiptJson']['txHash'], record['receiptJson']['to'], 10**18)
        before = {item.name: item.read_bytes() for item in state.iterdir()}
        changed = dict(profile, **{field: value})
        path.write_text(json.dumps(changed))
        with pytest.raises((RuntimeError, ValueError)):
            MOVEMENT.BankMovement(peer.node(), path, state)
        assert {item.name: item.read_bytes() for item in state.iterdir()} == before
        path.write_text(json.dumps(profile))
        MOVEMENT.BankMovement(peer.node(), path, state)


@pytest.mark.parametrize('damage', ['pending', 'signature', 'block', 'rpc-id', 'non-owner'])
def test_movement_refuses_uncommitted_or_forged(tmp_path, damage):
    record, profile, path, state = bank_fixture(tmp_path)
    if damage == 'non-owner':
        profile['owner'] = '0x' + '11' * 20
        path.write_text(json.dumps(profile))
    with Peer(tmp_path / 'peer.db').serving() as peer:
        movement = MOVEMENT.BankMovement(peer.node(), path, state)
        def corrupt(method, response):
            if method == 'pax_sequencerReceipt':
                if damage == 'pending':
                    response['result']['rung'] = 'receipted'
                elif damage == 'signature':
                    raw = bytearray.fromhex(response['result']['receipt'][2:])
                    raw[-1] ^= 1
                    response['result']['receipt'] = '0x' + raw.hex()
            if method == 'eth_getTransactionReceipt' and damage == 'block':
                response['result']['blockHash'] = '0x' + '01' * 32
            if method == 'pax_sequencerReceipt' and damage == 'rpc-id':
                response['id'] += 1
        peer.damage = corrupt
        with pytest.raises((RuntimeError, ValueError, SequencerReceiptVerificationError, merged.CHAIN._EXECUTION.Refusal)):
            movement.prove(record['receiptJson']['txHash'], record['receiptJson']['to'], 10**18)
        assert not list(state.glob('*.proof'))


def test_movement_restored_state_and_private_input_refusals(tmp_path):
    record, profile, path, state = bank_fixture(tmp_path)
    with Peer(tmp_path / 'peer.db').serving() as peer:
        MOVEMENT.BankMovement(peer.node(), path, state)
    pin = state / 'bank-authority.pin'
    original = pin.read_bytes()
    pin.write_bytes(original.replace(b'bank', b'evil'))
    with Peer(tmp_path / 'peer.db').serving() as peer:
        with pytest.raises(RuntimeError, match='retained'):
            MOVEMENT.BankMovement(peer.node(), path, state)
        pin.write_bytes(original)
        MOVEMENT.BankMovement(peer.node(), path, state)
        path.chmod(0o644)
        with pytest.raises(RuntimeError, match='owner-only'):
            MOVEMENT.BankMovement(peer.node(), path, state)
        path.chmod(0o600)
        link = tmp_path / 'linked-profile'
        link.symlink_to(path)
        with pytest.raises(RuntimeError):
            MOVEMENT.BankMovement(peer.node(), link, state)


def test_transfer_requires_explicit_private_directory(tmp_path, monkeypatch):
    monkeypatch.delenv('PAXEER_X_EVIDENCE_DIR', raising=False)
    monkeypatch.delenv('TASK_LOG_DIR', raising=False)
    with pytest.raises(TRANSFER.Fail, match='explicit'):
        TRANSFER.evidence_directory()
    monkeypatch.setenv('TASK_LOG_DIR', str(tmp_path))
    assert TRANSFER.evidence_directory() == tmp_path
    tmp_path.chmod(0o755)
    with pytest.raises(TRANSFER.Fail, match='0700'):
        TRANSFER.evidence_directory()


def test_transfer_exclusive_private_outputs(tmp_path):
    previous = os.umask(0)
    try:
        with TRANSFER.private_output(tmp_path, 'case.log') as stream:
            assert stat.S_IMODE(os.fstat(stream.fileno()).st_mode) == 0o600
            stream.write('private operational record')
    finally:
        os.umask(previous)
    output = tmp_path / 'case.log'
    assert output.read_text() == 'private operational record'
    with pytest.raises(FileExistsError):
        TRANSFER.private_output(tmp_path, output.name)
    link = tmp_path / 'symlink.log'
    link.symlink_to(output)
    with pytest.raises(FileExistsError):
        TRANSFER.private_output(tmp_path, link.name)
    os.link(output, tmp_path / 'hardlink.log')
    with pytest.raises(FileExistsError):
        TRANSFER.private_output(tmp_path, 'hardlink.log')
    with pytest.raises(TRANSFER.Fail):
        TRANSFER.private_output(tmp_path, '../outside.log')
    assert output.read_text() == 'private operational record'


def test_transfer_refuses_checkout_and_symlink(tmp_path, monkeypatch):
    monkeypatch.setenv('PAXEER_X_EVIDENCE_DIR', str(ROOT))
    with pytest.raises(TRANSFER.Fail, match='outside'):
        TRANSFER.evidence_directory()
    link = tmp_path / 'link'
    link.symlink_to(tmp_path, target_is_directory=True)
    monkeypatch.setenv('PAXEER_X_EVIDENCE_DIR', str(link))
    with pytest.raises(TRANSFER.Fail, match='canonical'):
        TRANSFER.evidence_directory()
    monkeypatch.setenv('PAXEER_X_EVIDENCE_DIR', '.')
    with pytest.raises(TRANSFER.Fail):
        TRANSFER.evidence_directory()


def deployment(abi=5, framed=True):
    fixture = json.loads((ROOT / 'sdk/platform/conformance/fixtures/native-program-deploy-v4.json').read_text())
    assert fixture['provenance'].startswith('tests/programs/test_call_activity.c native encoder')
    original = NativeProgramDeploy.decode(bytes.fromhex(fixture['payload_hex']))
    interface = bytearray(original.interface)
    interface[60:62] = abi.to_bytes(2, 'big')
    if abi >= 3:
        interface[26] = ord(str(abi))
    return replace(original, guest_abi=abi, interface=bytes(interface) if framed else None)


def test_all_form_inventory_and_real_lifecycle_codec(tmp_path):
    matrix = ALL_FORM.required_native_cases()
    for phase in ('initial', 'snapshot_replay'):
        for form in ('legacy', 'framed'):
            for abi in (1, 2, 3, 4, 5):
                assert matrix[('valid', form, abi, phase)] == 0
                for refusal in ('malformed_body', 'float_type', 'ambient_import'):
                    assert matrix[(refusal, form, abi, phase)] == -3
                payload = deployment(abi, form == 'framed').encode()
                path = write_private(tmp_path / f'{phase}-{form}-{abi}.bin', payload)
                recovered = NativeProgramDeploy.decode(path.read_bytes())
                assert recovered.guest_abi == abi and recovered.encode() == payload
            assert matrix[('unsupported_abi', form, 6, phase)] == -101
    with pytest.raises(ValueError, match='invalid program code'):
        deployment(6).encode()
    source = (ROOT / 'tests/programs/test_lifecycle.c').read_text()
    body = source[source.index('static size_t admission_abis'):source.index('static void migration_name')]
    assert 'lxp_programs_abi_transition_validate(0U, (uint16_t)candidate)' in body
    assert 'unallocated, valid, sizeof(valid), 0, LXP_ERR_VERSION_UNSUPPORTED' in body
    assert 'abi <= 4U' not in body


def test_all_form_refusal_evidence_remains_strict():
    # A producer that merely relabels the old ABI-5 refusal cannot qualify.
    cases = []
    for (name, form, abi, phase), expected in ALL_FORM.required_native_cases().items():
        cases.append({'name': name, 'form': form, 'abi': abi, 'phase': phase,
                      'result': expected, 'expected': expected, 'lifecycle_present': expected == 0,
                      'artifact_present': expected == 0, 'new_artifacts': 0, 'staged': 0,
                      'effects': 0, 'state_unchanged': expected != 0})
    def output(rows):
        return '\n'.join('ALL_FORM_CASE ' + json.dumps(row) for row in rows) + f'\nALL_FORM_VALIDATION cases={len(rows)} skipped=0\n'
    # These are malformed report inputs, not a claim that a native runtime executed.
    changed = deepcopy(cases)
    next(row for row in changed if row['name'] == 'valid' and row['abi'] == 5)['result'] = -101
    with pytest.raises(RuntimeError, match='canonical expected'):
        ALL_FORM.native_cases(output(changed))
    changed = deepcopy(cases)
    next(row for row in changed if row['name'] == 'ambient_import')['effects'] = 1
    with pytest.raises(RuntimeError, match='retained new executable'):
        ALL_FORM.native_cases(output(changed))
    with pytest.raises(RuntimeError, match='accounting'):
        ALL_FORM.native_cases(output(cases[:-1]))


def test_language_tables_match_every_frozen_host_surface(tmp_path):
    assert LANGUAGE.admitted_abis() == (1, 2, 3, 4, 5)
    assert LANGUAGE.unallocated_abi() == 6
    rows = LANGUAGE.manifest_parity()
    assert rows[-1] == (5, 'layerx_v5', 'market_step_adjudicate', '(i32,i32,i32,i32,i32,i32)->i32')
    assert len(rows) == 29
    path = LANGUAGE.write_manifest(tmp_path / 'abi-manifest.tsv')
    assert LANGUAGE.manifest_parity(path) == rows
    for version in (1, 2, 3, 4, 5):
        frozen = bytes.fromhex((ROOT / f'programs/tests/vectors/abi-v{version}.hex').read_text())
        assert frozen[:2] == version.to_bytes(2, 'big')
        assert (b'layerx_v5\0' in frozen) is (version == 5)
    path.write_text(path.read_text().replace('market_step_adjudicate', 'unregistered_market_host'))
    with pytest.raises(RuntimeError, match='canonical host'):
        LANGUAGE.manifest_parity(path)


def test_language_linker_requires_historical_and_v5_guests():
    names = ('finding_015_import_admission_matches_every_runtime_namespace',
             'finding_015_v5_sdk_import_reaches_the_real_validator_and_linker')
    transcript = '\n'.join('test ' + name + ' ... ok' for name in names) + '\ntest result: ok. 2 passed; 0 failed; 0 ignored;\n'
    assert LANGUAGE.linker_output(transcript) == list(names)
    for damaged in (transcript.replace(names[1], 'unrelated_case'),
                    transcript.replace('0 ignored', '1 ignored'),
                    transcript.replace('2 passed', '0 passed'),
                    transcript.replace(' ... ok', ' ... FAILED', 1)):
        with pytest.raises(RuntimeError, match='guest validation/linker'):
            LANGUAGE.linker_output(damaged)


def test_native_case_inventory_includes_v5():
    for abi in (1, 2, 3, 4, 5):
        for case in ('deploy', 'registry', 'bindings-valid', 'restart-registry-process', 'restart-call',
                     'hash-mismatch', 'unknown-native-abi-6', 'refusal-preserves-state'):
            assert f'abi{abi}-{case}' in NATIVE.REQUIRED
        assert f'abi{abi}-unknown-native-abi-5' not in NATIVE.REQUIRED
    assert 'market-v5-bindings-registry-process' in NATIVE.REQUIRED
    assert 'monotonic-upgrade-5' in NATIVE.REQUIRED


def test_native_registry_sdk_consumer_and_reopen(tmp_path):
    observed = {}
    with Peer(tmp_path / 'registry.db').serving() as peer:
        for abi in (1, 2, 3, 4, 5):
            deploy = replace(deployment(abi), program_id=bytes([abi]) * 32)
            peer.publish(deploy)
            parsed = NATIVE.registry_interface(peer.node(), peer.url, deploy.program_id, abi,
                deploy.new_hash, deploy.interface, 1, 1500, {'Authorization': AUTH})
            assert parsed.abi_version == abi and parsed.interface == deploy.interface
            observed[abi] = parsed
        assert any(path.endswith('/interface') for path, _, _ in peer.requests)
    with Peer(tmp_path / 'registry.db').serving() as peer:
        for abi, before in observed.items():
            deploy = replace(deployment(abi), program_id=bytes([abi]) * 32)
            assert NATIVE.registry_interface(peer.node(), peer.url, deploy.program_id, abi,
                deploy.new_hash, deploy.interface, 1, 1500, {'Authorization': AUTH}) == before


@pytest.mark.parametrize('field,value', [('abi_version', 4), ('code_hash', '11' * 32),
    ('interface_digest', '22' * 32), ('valid_through', 1400), ('verification', 'caller-asserted')])
def test_native_registry_refuses_substituted_interface(tmp_path, field, value):
    with Peer(tmp_path / 'registry.db').serving() as peer:
        deploy = peer.publish(deployment())
        def damage(method, result):
            if method == 'registry-interface':
                result[field] = value
        peer.damage = damage
        with pytest.raises(ValueError):
            NATIVE.registry_interface(peer.node(), peer.url, deploy.program_id, 5,
                deploy.new_hash, deploy.interface, 1, 1500, {'Authorization': AUTH})


def test_native_merged_manifest_and_artifact_identity(tmp_path):
    source = {'revision': 'aa5e0820b3652890f3812a220f4b91f4f8a45387', 'tree': '12' * 20}
    # Only manifest shape is claimed here; artifact hashing below consumes an actual source artifact.
    value = {'schema': NATIVE.SCHEMA, 'source': source, 'test': NATIVE.TEST,
             'required_cases': sorted(NATIVE.REQUIRED),
             'artifacts': {name: {'path': '/private/build/' + name} for name in NATIVE.ARTIFACTS},
             'build_logs': ['private build log'], 'bank_artifacts': ['committed bank funding'],
             'registry_configuration': '/private/registry.json', 'registry_cgroup_parent': '/sys/fs/cgroup/registry'}
    NATIVE.manifest_contract(value, source)
    assert not {'layerxd', 'genesis-builder', 'sign-credit', 'test-credit'} & NATIVE.ARTIFACTS
    for key in ('builder-isolation', 'builder-supervisor', 'paxd', 'bank-funding'):
        damaged = deepcopy(value)
        del damaged['artifacts'][key]
        with pytest.raises(ValueError):
            NATIVE.manifest_contract(damaged, source)
    with pytest.raises(ValueError):
        NATIVE.manifest_contract(value, dict(source, revision='ff' * 20))
    actual = NATIVE.producer.artifact(Path(NATIVE.__file__))
    assert actual['sha256'] == hashlib.sha256(Path(NATIVE.__file__).read_bytes()).hexdigest()
    private = tmp_path / 'producer.json'
    NATIVE.producer.write_private(private, {'source': source, 'artifact': actual})
    assert NATIVE.producer.load_private(private)['artifact'] == actual


def payment_binding(record):
    transaction, _ = decode_signed(bytes.fromhex(record['rawTransaction'][2:]))
    source, recipient, asset, amount, expires, reference = merged.decode_abi(
        ['bytes32', 'bytes32', 'bytes32', 'uint128', 'uint64', 'bytes32'], '0x' + transaction.data[4:].hex())
    return {'order_digest': reference, 'context': hashlib.sha256(b'recorded immutable quote context').digest(),
            'from': source, 'to': recipient, 'asset': asset, 'amount': amount, 'expires_at': expires}


def test_ramp_committed_destination_and_recovery(tmp_path):
    with Peer(tmp_path / 'ramp.db').serving() as peer:
        record = peer.data['records']['V1-layerxledger-pay-exact']
        expected = payment_binding(record)
        reference = record['receiptJson']['txHash']
        proof = RAMP.destination_receipt(peer.node(), reference, expected, tmp_path / 'destination-proof.json')
        assert proof['rung'] == 'committed' and proof['chain_id'] == 125
        assert proof['receipt_digest'] == record['receiptDigest'][2:]
        assert proof['to'] == expected['to'].hex() and proof['amount'] == '1000000'
    with Peer(tmp_path / 'ramp.db').serving() as peer:
        assert RAMP.destination_receipt(peer.node(), reference, expected, tmp_path / 'destination-proof.json') == RAMP.document(tmp_path / 'destination-proof.json')
        retained = tmp_path / 'destination-proof.json'
        original = retained.read_bytes()
        retained.write_text(json.dumps(dict(proof, amount='1')))
        with pytest.raises(RAMP.Refusal, match='recovered destination proof'):
            RAMP.destination_receipt(peer.node(), reference, expected, retained)
        retained.write_bytes(original)
        assert all(request['method'] != 'eth_sendRawTransaction' for _, _, request in peer.requests)


@pytest.mark.parametrize('field', ['order_digest', 'from', 'to', 'asset', 'amount', 'expires_at'])
def test_ramp_refuses_wrong_order_bindings(tmp_path, field):
    with Peer(tmp_path / 'ramp.db').serving() as peer:
        record = peer.data['records']['V1-layerxledger-pay-exact']
        expected = payment_binding(record)
        expected[field] = expected[field] + 1 if isinstance(expected[field], int) else bytes([7]) * 32
        with pytest.raises(RAMP.Refusal, match='bind the order'):
            RAMP.committed_destination(peer.node(), record['receiptJson']['txHash'], expected)


@pytest.mark.parametrize('damage', ['pending', 'signature', 'log', 'standard-block', 'foreign-chain', 'key'])
def test_ramp_refuses_forged_authority(tmp_path, damage):
    with Peer(tmp_path / 'ramp.db').serving() as peer:
        record = peer.data['records']['V1-layerxledger-pay-exact']
        node = peer.node()
        if damage == 'key':
            node.receipt_keys[0]['public_key'] = '11' * 32
        def corrupt(method, answer):
            if damage == 'foreign-chain' and method == 'eth_chainId':
                answer['result'] = '0x7e'
            if damage == 'standard-block' and method == 'eth_getTransactionReceipt':
                answer['result']['blockHash'] = '0x' + '01' * 32
            if method == 'pax_sequencerReceipt':
                if damage == 'pending':
                    answer['result']['rung'] = 'receipted'
                elif damage == 'signature':
                    raw = bytearray.fromhex(answer['result']['receipt'][2:])
                    raw[-1] ^= 1
                    answer['result']['receipt'] = '0x' + raw.hex()
                elif damage == 'log':
                    answer['result']['logs'][0]['data'] = '0x' + '00' * 64
        peer.damage = corrupt
        with pytest.raises((ValueError, RAMP.Refusal, SequencerReceiptVerificationError, merged.CHAIN._EXECUTION.Refusal)):
            RAMP.committed_destination(node, record['receiptJson']['txHash'], payment_binding(record))


def identity_data():
    did = 'did:layerx:' + '11' * 32
    name = ('agent:' + did + ':main').encode()
    native = hashlib.sha256(b'LX:ACCOUNT:v1' + len(name).to_bytes(4, 'big') + name).hexdigest()
    return {'evm_address': corpus()['V1-pax-transfer']['receiptJson']['from'],
            'layerx_did': did, 'layerx_account': native, 'bound': True}


def identity_target(peer, tls):
    return {'url': peer.url, 'ca_file': tls['ca']}


def test_identity_direct_gateway_node_and_reopen(tmp_path, tls):
    account = identity_data()
    with Peer(tmp_path / 'identity.db', tls).serving() as peer, Peer(tmp_path / 'node.db', tls).serving() as node:
        peer.data['identity'] = account
        peer.persist()
        target = identity_target(peer, tls)
        assert IDENTITY.identity_chain(target, identity_target(node, tls), GENESIS)['hash'] == GENESIS
        first = IDENTITY.identity_join(target, account['evm_address'], account['layerx_did'], account['layerx_account'])
        assert first == account
        assert len(peer.requests) == 18 and len(node.requests) == 2
    with Peer(tmp_path / 'identity.db', tls).serving() as peer, Peer(tmp_path / 'node.db', tls).serving() as node:
        target = identity_target(peer, tls)
        assert IDENTITY.identity_chain(target, identity_target(node, tls), GENESIS)['hash'] == GENESIS
        assert IDENTITY.identity_join(target, account['evm_address'], account['layerx_did'], account['layerx_account']) == first


def test_identity_refuses_foreign_chain_and_conflicting_owner(tmp_path, tls):
    account = identity_data()
    with Peer(tmp_path / 'identity.db', tls).serving() as peer:
        peer.data['identity'] = account
        target = identity_target(peer, tls)
        peer.data['chain'] = '0x7e'
        with pytest.raises(ValueError, match='wrong chain'):
            IDENTITY.identity_chain(target, target, GENESIS)
        peer.data['chain'] = '0x7d'
        with pytest.raises(ValueError, match='genesis'):
            IDENTITY.identity_chain(target, target, '0x' + 'cc' * 32)
        def corrupt(method, response):
            if method == 'px_getBalances':
                response['result']['account']['layerx_account'] = '22' * 32
        peer.damage = corrupt
        with pytest.raises(ValueError, match='join changed owner'):
            IDENTITY.identity_join(target, account['evm_address'], account['layerx_did'], account['layerx_account'])


def test_movement_committed_proof_pin_refuses_changed_recovery(tmp_path):
    record, profile, path, state = bank_fixture(tmp_path)
    reference = record['receiptJson']['txHash']
    with Peer(tmp_path / 'peer.db').serving() as peer:
        MOVEMENT.BankMovement(peer.node(), path, state).prove(reference, record['receiptJson']['to'], 10**18)
    retained = state / (reference[2:] + '.proof')
    original = retained.read_bytes()
    changed = json.loads(original)
    changed['proof']['block_hash'] = '0x' + 'cd' * 32
    retained.write_text(json.dumps(changed))
    with Peer(tmp_path / 'peer.db').serving() as peer:
        movement = MOVEMENT.BankMovement(peer.node(), path, state)
        with pytest.raises(RuntimeError, match='retained bank authority or committed proof changed'):
            movement.prove(reference, record['receiptJson']['to'], 10**18)
        retained.write_bytes(original)
        assert movement.prove(reference, record['receiptJson']['to'], 10**18) == json.loads(original)
        with pytest.raises(RuntimeError, match='binding mismatch'):
            movement.prove(reference, record['receiptJson']['to'], 10**18 + 1)
        assert retained.read_bytes() == original


def recorded_order():
    """In-process order producer using the toolkit's public length-prefixed wire format."""
    customer = 'agent:did:layerx:customer:main'
    operator = 'agent:did:layerx:operator:main'
    context, asset = bytes([3]) * 32, bytes([4]) * 32
    values = (b'on_ramp', b'order-1', b'quote-1', b'customer', customer.encode(),
              b'operator', operator.encode(), b'operator-key', asset, (1000000).to_bytes(16, 'big'),
              b'USD', (100).to_bytes(16, 'big'), (1).to_bytes(16, 'big'), (1).to_bytes(16, 'big'),
              (0).to_bytes(16, 'big'), (0).to_bytes(2, 'big'), context, b'provider', b'payout',
              (1750000000).to_bytes(8, 'big'), b'', b'')
    digest = hashlib.sha256(b'LXP/market-maker-ramp/order/v1\0' + b''.join(
        len(value).to_bytes(16, 'big') + value for value in values)).digest()
    return {'order_id': 'order-1', 'quote': {'direction': 'on_ramp', 'quote_id': 'quote-1',
        'layerx_asset': list(asset), 'layerx_amount': 1000000, 'external_currency': 'USD',
        'external_amount_minor': 100, 'rate_numerator': 1, 'rate_denominator': 1, 'fee_minor': 0,
        'maximum_slippage_bps': 0, 'context': list(context), 'provider_token': 'provider',
        'payout_token': 'payout', 'expires_at': 1750000000},
        'customer': {'principal_id': 'customer', 'account': customer},
        'operator': {'principal_id': 'operator', 'account': operator, 'signer_key_handle': 'operator-key'},
        'payer_grant': None, 'payer_grant_purpose': None, 'context': list(context), 'order_digest': list(digest)}


def test_ramp_order_digest_preserves_context_and_principals(tmp_path):
    path = write_private(tmp_path / 'order.json', recorded_order())
    order = RAMP.document(path)
    expected = RAMP.order_binding(order)
    assert expected['order_digest'] == bytes(order['order_digest'])
    assert expected['context'] == bytes(order['context'])
    assert expected['to'] == hashlib.sha256(
        b'LX:ACCOUNT:v1' + len(order['customer']['account']).to_bytes(4, 'big')
        + order['customer']['account'].encode()).digest()
    changed = deepcopy(order)
    changed['quote']['context'][0] ^= 1
    with pytest.raises(RAMP.Refusal, match='context differ'):
        RAMP.order_binding(changed)
    changed = deepcopy(order)
    changed['context'][0] ^= 1
    changed['quote']['context'] = changed['context']
    with pytest.raises(RAMP.Refusal, match='digest does not bind'):
        RAMP.order_binding(changed)
    changed = deepcopy(order)
    changed['customer']['account'] = order['operator']['account']
    with pytest.raises(RAMP.Refusal, match='digest does not bind'):
        RAMP.order_binding(changed)
