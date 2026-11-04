"""Real qualification client, root/hosted recording peers and bounded participants.

The merged owner currently refuses account proofs. The peer preserves that
refusal; it never manufactures account proof bytes to make coverage complete.
"""

from contextlib import contextmanager
from copy import deepcopy
from datetime import datetime, timedelta, timezone
import base64
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import ipaddress
import json
import os
from pathlib import Path
import re
import shlex
import signal
import ssl
import sys
import threading
from types import SimpleNamespace

import pytest
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.x509.oid import NameOID

QUAL = Path(__file__).resolve().parents[1]
ROOT = QUAL.parents[2]
sys.path.insert(0, str(QUAL / 'fixtures'))
spec = importlib.util.spec_from_file_location('repair_22_33_765_envelope', QUAL / 'agent_operation_envelope.py')
ENVELOPE = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ENVELOPE)
from layerx_sdk.account_derivation import keccak256
from layerx_sdk.agent_http import encode_proof_bundle_request
from layerx_sdk.production import PlatformSdkError, SdkErrorCode
from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt

REVISION = '5f6b5f0aad35a415da93671f70183d5e5688bda0'
CREDENTIAL = {'tenant': 'recorded-tenant', 'session_id': '11' * 32, 'token_id': '22' * 32, 'generation': '1'}


def corpus():
    document = json.loads((ROOT / 'sdk/platform/conformance/fixtures/vector-v5-portable.json').read_text())
    assert document['provenance']
    return document['bundles']


def verified(vector):
    key = vector['receiptKey']
    return verify_sequencer_receipt(
        bytes.fromhex(vector['signedReceipt'][2:]), bytes.fromhex(vector['rawTransaction'][2:]),
        [ReceiptKey(bytes.fromhex(key['keyId'][2:]), bytes.fromhex(key['publicKey'][2:]),
                    int(key['firstHeight'], 16), int(key['lastHeight'], 16))])


def anchor(vector):
    receipt = verified(vector)
    return {'activity_id': receipt.receipt.activity_id.hex(),
            'submission': {'state': 'Executed'},
            'receipt': {'canonical_bytes': receipt.canonical_bytes.hex()}}


def certificate(directory):
    directory.mkdir(mode=0o700)
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, 'localhost')])
    now = datetime.now(timezone.utc)
    cert = (x509.CertificateBuilder().subject_name(name).issuer_name(name)
            .public_key(key.public_key()).serial_number(x509.random_serial_number())
            .not_valid_before(now - timedelta(minutes=1)).not_valid_after(now + timedelta(hours=1))
            .add_extension(x509.BasicConstraints(ca=True, path_length=None), critical=True)
            .add_extension(x509.SubjectAlternativeName([x509.DNSName('localhost'),
                                                       x509.IPAddress(ipaddress.ip_address('127.0.0.1'))]), critical=False)
            .sign(key, hashes.SHA256()))
    (directory / 'ca.pem').write_bytes(cert.public_bytes(serialization.Encoding.PEM))
    key_path = directory / 'server.key'
    key_path.write_bytes(key.private_bytes(serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8,
                                          serialization.NoEncryption()))
    key_path.chmod(0o600)
    return directory


class RecordingPeer:
    """A committed recording server with a durable private replay ledger.

    Signed transactions and receipts are the native recorder's unmodified bytes.
    Hosted network metadata uses the current gateway schema and recorded heads.
    Account requests retain the real merged owner's typed unavailability.
    """

    def __init__(self, directory, tls=None):
        self.directory = directory
        directory.mkdir(mode=0o700, parents=True, exist_ok=True)
        self.state = directory / 'replay.json'
        self.tls = tls
        self.server = self.thread = None
        self.requests = []
        self.errors = []
        self.invalid_id = False
        self.retired_status = 404
        self.port = 0
        recordings = json.loads((ROOT / 'tests/agent/sdk/python/fixtures_22_33_701/chain-125-root.json').read_text())
        self.root = {entry['request']['method']: entry['response'] for entry in recordings['nodes'][0]['exchanges']}
        self.vectors = corpus()
        self.anchors = [anchor(vector) for vector in self.vectors]
        self.starts = self.stops = 0

    def dispatch(self, path, headers, request):
        self.requests.append((path, dict(headers), deepcopy(request)))
        assert headers['Content-Type'] == 'application/json'
        if path == '/':
            assert 'Authorization' not in headers and 'LayerX-Tenant' not in headers
            assert request['jsonrpc'] == '2.0' and type(request['id']) is int
            method = request['method']
            if method == 'px_getNetwork':
                assert request['params'] == []
                answer = {'jsonrpc': '2.0', 'id': request['id'], 'result': {
                    'network_id': 'paxeer-x',
                    'paxeer': {'chain_id': self.root['eth_chainId']['result'],
                               'latest_block': self.root['eth_blockNumber']['result']},
                    'consensus': {'latest_height': str(self.vectors[1]['blockHeader']['Height'])}}}
            else:
                assert request['params'] == []
                answer = deepcopy(self.root[method])
                answer['id'] = request['id']
            if self.invalid_id:
                answer['id'] = str(request['id'])
            return 200, answer
        if path != ENVELOPE.ROUTE:
            return 404, {'error': 'route not found'}
        identifier = request['request_id']
        assert request['version'] == 1 and isinstance(identifier, str)
        if headers.get('Authorization') != 'LayerX-Key recording-key' or request['credential'] != CREDENTIAL:
            return self.refusal(identifier, 403, 'PolicyRefusal', 'session.invalid')
        operation = request['operation']
        if operation == 'submit':
            assert set(request) == {'version', 'request_id', 'operation', 'request', 'credential', 'idempotency_key'}
            assert re.fullmatch('[0-9a-f]{64}', request['idempotency_key'])
            raw = request['request']['raw_transaction']
            assert raw in [item['rawTransaction'][2:] for item in self.vectors]
            ledger = json.loads(self.state.read_text()) if self.state.exists() else {}
            key = request['idempotency_key']
            if key in ledger and ledger[key]['raw_transaction'] != raw:
                return self.refusal(identifier, 409, 'IdempotencyConflict', 'idempotency.changed_body')
            if key not in ledger:
                ledger[key] = {'raw_transaction': raw, 'activity_id': keccak256(bytes.fromhex(raw)).hex(), 'transmissions': 1}
                ENVELOPE.fixture.write_json(self.state, ledger)
            return 200, {'request_id': identifier, 'value': ledger[key],
                         'verification_status': {'state': 'achieved', 'rung': 'submitted'}}
        assert operation == 'read.proof_bundle'
        typed = encode_proof_bundle_request(request['request'])
        assert typed == request['request']
        target = bytes.fromhex(typed['target'])
        vector = next(item for item in self.vectors if keccak256(bytes.fromhex(item['rawTransaction'][2:])) == target[3:35])
        if target[2] == 2:
            # human_runtime::read_proof_bundle returns Unavailable for owned AccountState.
            return self.refusal(identifier, 503, 'UnavailableCapability', 'owner.unavailable')
        bundle = {key: vector[key] for key in ENVELOPE.PORTABLE_FIELDS}
        bundle = deepcopy(bundle)
        for key in ('rawTransaction', 'signedReceipt', 'receiptDigest'):
            bundle[key] = bundle[key][2:]
        bundle['verificationLevel'] = 'receipted'
        bundle['receiptKey'] = {key: str(int(value, 16)) if key.endswith('Height') else value[2:]
                                for key, value in bundle['receiptKey'].items()}
        height = str(int(vector['receiptJson']['height'], 16))
        block_hash = base64.b64decode(vector['commit']['BlockID']['Hash'], validate=True).hex()
        return 200, {'request_id': identifier, 'value': {
            'value': bundle, 'achieved_verification_level': 'receipted',
            'freshness': {'chain_head': height, 'value_height': height,
                          'relative_to': {'height': height, 'hash': block_hash}}},
            'verification_status': {'state': 'achieved', 'rung': 'receipted'}}

    @staticmethod
    def refusal(identifier, status, klass, reason):
        return status, {'request_id': identifier, 'class': klass, 'reason': reason,
                        'protocol_result_code': None, 'retriability': 'Terminal'}

    def start(self):
        peer = self

        class Handler(BaseHTTPRequestHandler):
            protocol_version = 'HTTP/1.1'

            def log_message(self, *args):
                pass

            def do_GET(self):
                peer.requests.append((self.path, dict(self.headers), None))
                self.reply(peer.retired_status, {'error': 'route not found'})

            def do_POST(self):
                try:
                    length = int(self.headers['Content-Length'])
                    assert 0 < length <= ENVELOPE.MAX_BODY
                    document = json.loads(self.rfile.read(length))
                    status, answer = peer.dispatch(self.path, self.headers, document)
                    self.reply(status, answer)
                except Exception as error:
                    peer.errors.append(repr(error))
                    self.reply(500, {'error': str(error)})

            def reply(self, status, answer):
                raw = json.dumps(answer, separators=(',', ':')).encode()
                self.send_response(status)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(raw)))
                self.send_header('Connection', 'close')
                self.end_headers()
                self.wfile.write(raw)

        self.server = ThreadingHTTPServer(('127.0.0.1', self.port), Handler)
        self.port = self.server.server_port
        if self.tls:
            ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
            ctx.load_cert_chain(str(self.tls / 'ca.pem'), str(self.tls / 'server.key'))
            self.server.socket = ctx.wrap_socket(self.server.socket, server_side=True)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.starts += 1
        return self

    def stop(self):
        if self.server is not None:
            self.server.shutdown()
            self.server.server_close()
            self.thread.join(timeout=5)
            assert not self.thread.is_alive()
            self.server = None
            self.stops += 1


@contextmanager
def served(tmp_path):
    tls = certificate(tmp_path / 'tls')
    node = RecordingPeer(tmp_path / 'node').start()
    hosted = RecordingPeer(tmp_path / 'hosted', tls).start()
    owner = ENVELOPE.Qualification.__new__(ENVELOPE.Qualification)
    owner.d = tmp_path / 'evidence'
    owner.d.mkdir(mode=0o700)
    (owner.d / 'responses').mkdir(mode=0o700)
    owner.tls = tls
    owner.gateway_port = hosted.port
    owner.credential = deepcopy(CREDENTIAL)
    owner.request_id = 1000
    owner.results = []
    owner.config = {'requests': {'read': {'request': {'account_id': 'ab' * 32}}},
                    'gateway_api_key_file': str(tmp_path / 'gateway-key')}
    Path(owner.config['gateway_api_key_file']).write_text('recording-key\n')
    owner.runtime = SimpleNamespace()
    owner.runtime.rpc = ENVELOPE.fixture.RootRpc('http://127.0.0.1:' + str(node.port))
    owner.runtime.rpc_url = owner.runtime.rpc.endpoint
    owner.runtime.directory = tmp_path / 'generation'
    owner.agent_port = 12345
    try:
        yield owner, node, hosted
        assert not node.errors and not hosted.errors
    finally:
        hosted.stop()
        node.stop()


def test_retired_public_router_uses_merged_root_and_hosted_px_contract(tmp_path):
    with served(tmp_path) as (owner, node, hosted):
        owner.public_rpc_cases()
        assert node.requests[0][0] == '/'
        assert node.requests[0][2]['method'] == 'eth_chainId'
        assert [row[0] for row in hosted.requests] == ['/', '/rpc', '/rpc/schema']
        assert hosted.requests[0][2]['method'] == 'px_getNetwork'
        assert all('Authorization' not in headers for _, headers, _ in hosted.requests)
        assert [case['case'] for case in owner.results] == ['native_rpc_unchanged']
        substituted = owner.substitute({'rpc': '{node_rpc_url}', 'hosted': '{hosted_rpc_url}'})
        assert substituted['rpc'] == node_url(node)
        assert substituted['hosted'] == 'https://localhost:' + str(hosted.port) + '/'
        for removed in ('replica_url', 'node_socket'):
            with pytest.raises(KeyError):
                owner.substitute({'retired': '{' + removed + '}'})


def node_url(peer):
    return 'http://127.0.0.1:' + str(peer.port) + '/'


def test_retired_public_routes_and_rpc_identity_are_strict(tmp_path):
    with served(tmp_path) as (owner, node, hosted):
        node.invalid_id = True
        with pytest.raises(RuntimeError, match='RPC envelope'):
            owner.public_rpc_cases()
        node.invalid_id = False
        hosted.retired_status = 200
        with pytest.raises(RuntimeError, match='retired public router'):
            owner.public_rpc_cases()
        assert owner.results == []
    for suffix in ('/rpc', '/rpc/schema', '/?key=secret'):
        with pytest.raises(RuntimeError, match='root RPC origin'):
            ENVELOPE.fixture.RootRpc('http://127.0.0.1:8545' + suffix)


def test_private_tenant_idempotency_and_recovery_stay_on_agent_route(tmp_path):
    with served(tmp_path) as (owner, node, hosted):
        vectors = corpus()
        request = {'raw_transaction': vectors[0]['rawTransaction'][2:]}
        key = '55' * 32
        sent, status, body = owner.call('private.first', 'submit', request, idempotency=key)
        first = owner.read_success(status, body, sent['request_id'], 'private.first')
        before = hosted.state.read_bytes()
        hosted.stop()
        hosted = RecordingPeer(hosted.directory, hosted.tls).start()
        owner.gateway_port = hosted.port
        try:
            sent, status, body = owner.call('private.recovered', 'submit', request, idempotency=key)
            restored = owner.read_success(status, body, sent['request_id'], 'private.recovered')
            assert restored['value'] == first['value']
            assert restored['value']['transmissions'] == 1
            assert hosted.state.read_bytes() == before
            sent, status, body = owner.call('private.conflict', 'submit',
                                           {'raw_transaction': vectors[1]['rawTransaction'][2:]}, idempotency=key)
            owner.refusal(status, body, 409, 'IdempotencyConflict', 'idempotency.changed_body', 'private.conflict', sent['request_id'])
            owner.credential['tenant'] = 'foreign-tenant'
            sent, status, body = owner.call('private.foreign', 'submit', request, idempotency=key)
            owner.refusal(status, body, 403, 'PolicyRefusal', 'session.invalid', 'private.foreign', sent['request_id'])
            assert hosted.state.read_bytes() == before
            assert all(path == ENVELOPE.ROUTE for path, _, _ in hosted.requests)
            assert all(headers['Authorization'] == 'LayerX-Key recording-key' for _, headers, _ in hosted.requests)
            assert not node.requests and not hosted.errors
        finally:
            hosted.stop()


def test_non_maintained_account_has_distinct_real_producer_target_after_reopen(tmp_path):
    with served(tmp_path) as (owner, node, hosted):
        anchors = [anchor(vector) for vector in reversed(corpus())]
        plan = owner.proof_bundle_plan(*anchors)
        assert set(plan) == {'proof_bundle.' + name for name in ENVELOPE.PROOF_CASES}
        assert {entry['expected_kind'] for entry in plan.values()} == {1, 2, 3}
        maintained = plan['proof_bundle.maintained_account']['request']
        other = plan['proof_bundle.non_maintained_account']['request']
        assert maintained != other
        assert other['target'][6:70] == anchors[1]['activity_id']
        assert other['target'][70:] == maintained['target'][70:] == 'ab' * 32
        assert other['requested_verification_level'] == 'committed'
        observed = []
        for phase in ('before', 'after'):
            for case, entry in plan.items():
                sent, status, body = owner.call(case + '.' + phase, entry['operation'], entry['request'])
                assert sent['credential'] == CREDENTIAL
                assert sent['request'] == encode_proof_bundle_request(entry['request'])
                if entry['expected_kind'] == 2:
                    owner.refusal(status, body, 503, 'UnavailableCapability', 'owner.unavailable', case, sent['request_id'])
                    with pytest.raises(RuntimeError, match='expected 200'):
                        owner.read_success(status, body, sent['request_id'], case)
                else:
                    response = owner.read_success(status, body, sent['request_id'], case)
                    bundle = response['value']['value']
                    assert bundle['format'] == 'layerx-receipt-proof-v2'
                    assert bundle['signedReceipt'] == entry['receipt_bytes']
                    assert keccak256(bytes.fromhex(bundle['rawTransaction'])).hex() == entry['request']['target'][6:70]
                    assert bundle['receiptDigest'] == verified(corpus()[1]).receipt_digest.hex()
                observed.append((phase, case, status))
            if phase == 'before':
                hosted.stop()
                hosted.start()
                owner.gateway_port = hosted.port
        assert len(observed) == 8
        assert [row[2] for row in observed] == [200, 200, 503, 503] * 2
        assert hosted.starts == 2 and hosted.stops == 1
        assert not node.requests


def test_missing_or_unrecovered_account_can_never_satisfy_final_completeness(tmp_path):
    with served(tmp_path) as (owner, _, _):
        expected = {'proof_bundle.' + name for name in ENVELOPE.PROOF_CASES}
        owner.proof_bundle_kinds = {1, 2, 3}
        for missing in expected:
            owner.proof_bundle_cases = expected - {missing}
            owner.proof_bundle_recovered = expected
            with pytest.raises(RuntimeError, match='qualification incomplete'):
                owner.check_proof_completeness()
            owner.proof_bundle_cases = expected
            owner.proof_bundle_recovered = expected - {missing}
            with pytest.raises(RuntimeError, match='qualification incomplete'):
                owner.check_proof_completeness()
        owner.proof_bundle_cases = owner.proof_bundle_recovered = expected
        owner.check_proof_completeness()
        anchors = [anchor(vector) for vector in corpus()]
        with pytest.raises(RuntimeError, match='own genuine producer activity'):
            owner.proof_bundle_plan(anchors[0], anchors[0])
        anchors[1]['submission']['state'] = 'Queued'
        with pytest.raises(RuntimeError, match='genuine executed receipt'):
            owner.proof_bundle_plan(*anchors)


@pytest.mark.parametrize('escape_session', (False, True))
def test_timeout_retains_private_manifest_log_and_reaps_participant_tree(tmp_path, capsys, escape_session):
    evidence = tmp_path / 'private evidence'
    evidence.mkdir(mode=0o700)
    worker = '''import os, subprocess, sys, time
child = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(60)'], start_new_session=ESCAPE)
print('PARTICIPANTS', os.getpid(), child.pid, flush=True)
print('PAXEER_X_PROGRESS cases=7 last=retained', flush=True)
print('PAXEER_X_GATE tests=7 skipped=0', flush=True)
time.sleep(60)
'''
    argv = [sys.executable, '-c', worker.replace('ESCAPE', repr(escape_session))]
    assert ENVELOPE.run_participant(argv, dict(os.environ), evidence, REVISION, tmp_path / 'runtime', timeout=1) == 124
    result = json.loads((evidence / 'result.json').read_text())
    log = (evidence / 'worker.log').read_text()
    match = re.search(r'PARTICIPANTS (\d+) (\d+)', log)
    assert match is not None
    for pid in map(int, match.groups()):
        assert not Path('/proc', str(pid)).exists(), f'participant {pid} was not reaped'
    assert result['participant_pid'] == int(match[1])
    assert result['candidate_revision'] == REVISION
    assert result['command'] == shlex.join(argv) and result['participant_argv'] == argv
    assert result['exit_code'] == 124 and result['participant_exit_code'] == -signal.SIGKILL
    assert result['status'] == 'timeout' and result['timed_out'] is True
    assert result['cases_passed'] == 7 and result['gate_marker'] is False
    assert result['timeout_seconds'] == 1
    assert result['worker_log'] == str(evidence / 'worker.log')
    assert result['case_results'] == str(tmp_path / 'runtime/case-results.json')
    assert result['effect_manifest'] == str(tmp_path / 'runtime/effect-manifest.json')
    assert result['started_utc'] <= result['finished_utc']
    for path in (evidence / 'result.json', evidence / 'worker.log'):
        assert path.stat().st_mode & 0o777 == 0o600
    assert 'incomplete=1' in capsys.readouterr().out


@pytest.mark.parametrize(('source', 'expected_exit', 'status', 'count', 'marker'), (
    ("print('PAXEER_X_GATE tests=3 skipped=0')", 0, 'passed', 3, True),
    ("print('PAXEER_X_PROGRESS cases=2 last=retained'); raise SystemExit(9)", 9, 'failed', 2, False),
    ("print('PAXEER_X_PROGRESS cases=2 last=retained')", 1, 'failed', 2, False),
    ("print('PAXEER_X_GATE tests=0 skipped=0')", 1, 'failed', 0, False),
    ("print('PAXEER_X_GATE tests=3 skipped=0'); raise SystemExit(4)", 4, 'failed', 3, False),
))
def test_terminal_evidence_uses_same_manifest_without_false_success(tmp_path, source, expected_exit, status, count, marker):
    argv = [sys.executable, '-c', source]
    assert ENVELOPE.run_participant(argv, dict(os.environ), tmp_path, REVISION, tmp_path / 'runtime', timeout=10) == expected_exit
    result = json.loads((tmp_path / 'result.json').read_text())
    assert result['status'] == status and result['timed_out'] is False
    assert result['cases_passed'] == count and result['gate_marker'] is marker
    assert result['exit_code'] == expected_exit
    assert shlex.split(result['command']) == argv
    assert result['candidate_revision'] == REVISION
    retained = (tmp_path / 'result.json').read_bytes()
    with pytest.raises(FileExistsError):
        ENVELOPE.write_private(tmp_path / 'result.json', {'forged': True})
    assert (tmp_path / 'result.json').read_bytes() == retained


def test_rpc_fault_relay_preserves_real_reply_then_changes_one_root_answer(tmp_path):
    with served(tmp_path) as (owner, node, hosted):
        relay = ENVELOPE.fixture.RpcProofFaultRelay(
            node_url(node), 'https://localhost:' + str(hosted.port) + '/',
            owner.context(), 'wrong_native_network').start()
        try:
            client = ENVELOPE.fixture.RootRpc(relay.endpoint)
            assert client.request('eth_chainId', []) == '0x7d'
            relay.arm()
            assert client.request('eth_chainId', []) == '0x7c'
            assert client.request('eth_chainId', []) == '0x7d'
            assert relay.trigger_count == relay.mutated_frame_count == 1
            assert not relay.failure_info and not hosted.requests
            assert len(node.requests) == 3
        finally:
            relay.stop()


def test_rpc_head_fault_invokes_real_producer_once_and_preserves_observed_bytes(tmp_path):
    with served(tmp_path) as (owner, node, hosted):
        relay = ENVELOPE.fixture.RpcProofFaultRelay(
            node_url(node), 'https://localhost:' + str(hosted.port) + '/',
            owner.context(), 'moving_head').start()
        try:
            client = ENVELOPE.fixture.RootRpc(relay.endpoint)
            head = client.request('eth_blockNumber', [])
            effects = []

            def read_producer():
                effects.append(owner.runtime.rpc.request('eth_chainId', []))

            relay.arm(on_selected=read_producer)
            assert client.request('eth_blockNumber', []) == head
            assert client.request('eth_blockNumber', []) == head
            assert effects == ['0x7d']
            assert relay.trigger_count == 1 and relay.mutated_frame_count == 0
            assert not relay.failure_info
        finally:
            relay.stop()


def test_current_portable_reply_is_not_silently_converted_to_retired_sdk_format(tmp_path):
    with served(tmp_path) as (owner, _, _):
        plan = owner.proof_bundle_plan(*(anchor(vector) for vector in reversed(corpus())))
        entry = plan['proof_bundle.receipt']
        sent, status, body = owner.call('portable.sdk-boundary', entry['operation'], entry['request'])
        response = owner.read_success(status, body, sent['request_id'], 'portable.sdk-boundary')
        assert response['value']['value']['format'] == 'layerx-receipt-proof-v2'
        # The SDK still consumes PB1. Keep the real check and expose its failure;
        # the task record requests the owning domain's consumer migration.
        with pytest.raises(PlatformSdkError) as failure:
            owner.proof_bundle_record(response, entry['request'], 3, entry['receipt_bytes'])
        assert failure.value.code == SdkErrorCode.DECODE_FAILURE
