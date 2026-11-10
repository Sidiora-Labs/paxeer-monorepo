"""Exercise the repaired qualification clients over real pipes and loopback HTTP.

Execution bytes and signatures come unchanged from the node's committed conformance
corpus. The peer owns a persistent acceptance ledger and block/tenant projections;
it never replaces the clients, the EVM codec or the pinned receipt verifier.
"""
from copy import deepcopy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import threading
import time
from types import SimpleNamespace

import pytest

QUAL = Path(__file__).resolve().parents[1]
ROOT = QUAL.parents[2]
sys.path.insert(0, str(ROOT / 'services/agent/sdk/python'))
from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import decode_signed, encode_call
from layerx_sdk.program_lifecycle import NativeProgramDeploy, NativeProgramUpgrade, NativeProgramWindDown
from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt


def load(name):
    spec = importlib.util.spec_from_file_location('repair_778_' + name.replace('-', '_'), QUAL / (name + '.py'))
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


HTTP = load('agent_http_bounds')
READINESS = load('agent_tenant_readiness')
WRITES = load('mcp_served_writes')
PAYER = load('mcp_web_payer')
WINDDOWN = load('programs_accounts_winddown')
SOURCE = load('programs_source_abi')
CHAIN = WINDDOWN.CHAIN
AUTH = 'LayerX-Key recorded:lxp_live_' + '99' * 32


def case(name):
    document = json.loads((ROOT / 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json').read_text())
    assert document['provenance']['generator'] == 'rpc/sequencer_receipt_vectors_test.go'
    return deepcopy(next(row for row in document['cases'] if row['id'] == name))


def pins(record):
    pin = record['pinnedKey']
    return [{'key_id': pin['keyId'][2:], 'public_key': pin['publicKey'][2:],
             'first_height': pin['firstHeight'], 'last_height': pin['lastHeight']}]


def receipt_identity(record):
    pin = pins(record)[0]
    verified = verify_sequencer_receipt(bytes.fromhex(record['signedReceipt'][2:]),
        bytes.fromhex(record['rawTransaction'][2:]),
        (ReceiptKey(bytes.fromhex(pin['key_id']), bytes.fromhex(pin['public_key']), pin['first_height'], pin['last_height']),))
    return verified.receipt_digest.hex()


class ForkedMcp:
    """A real child speaking newline JSON-RPC; no subprocess binary or toolchain."""
    def __init__(self, mode):
        request_read, request_write = os.pipe()
        response_read, response_write = os.pipe()
        self.returncode = None
        self.pid = os.fork()
        if self.pid == 0:
            os.close(request_write)
            os.close(response_read)
            try:
                with os.fdopen(request_read, 'rb') as stream:
                    for raw in stream:
                        request = json.loads(raw)
                        assert request['jsonrpc'] == '2.0'
                        assert set(request) == {'jsonrpc', 'id', 'method', 'params'}
                        assert request['method'] in ('initialize', 'tools/call')
                        if mode == 'partial':
                            os.write(response_write, b'{')
                            # Remain alive without completing the frame.
                            signal.pause()
                        if mode == 'eof':
                            os.write(response_write, b'{"jsonrpc":')
                            break
                        if mode == 'oversize':
                            body = b' ' * 1_048_576
                        else:
                            if request['method'] == 'tools/call':
                                assert set(request['params']) == {'name', 'arguments', '_meta'}
                                assert request['params']['_meta'] == {'layerx/idempotency_key': '44' * 32}
                            value = {'tool': request['params'].get('name', 'initialize'), 'accepted': True}
                            result = {'isError': False, 'content': [{'type': 'text', 'text': json.dumps(value)}],
                                      'structuredContent': value}
                            response = {'jsonrpc': '2.0', 'id': request['id'] + (1 if mode == 'wrong-id' else 0), 'result': result}
                            if mode == 'exact-bound':
                                response['padding'] = ''
                                length = len(json.dumps(response, separators=(',', ':')).encode()) + 1
                                response['padding'] = 'a' * (1_048_576 - length)
                            body = json.dumps(response, separators=(',', ':')).encode() + b'\n'
                        for offset in range(0, len(body), 4093):
                            os.write(response_write, body[offset:offset + 4093])
                os.close(response_write)
                os._exit(0)
            except BaseException:
                os._exit(71)
        os.close(request_read)
        os.close(response_write)
        self.stdin = os.fdopen(request_write, 'wb')
        self.stdout = os.fdopen(response_read, 'rb')

    def poll(self):
        if self.returncode is None:
            pid, status = os.waitpid(self.pid, os.WNOHANG)
            if pid:
                self.returncode = os.waitstatus_to_exitcode(status)
        return self.returncode

    def terminate(self):
        if self.poll() is None:
            os.kill(self.pid, signal.SIGTERM)

    def kill(self):
        if self.poll() is None:
            os.kill(self.pid, signal.SIGKILL)

    def wait(self, timeout):
        deadline = time.monotonic() + timeout
        while self.poll() is None:
            if time.monotonic() >= deadline:
                raise subprocess.TimeoutExpired('forked MCP peer', timeout)
            time.sleep(0.005)
        return self.returncode

    def close(self):
        self.terminate()
        self.wait(3)
        self.stdin.close()
        self.stdout.close()


def mcp_client(kind, peer, directory):
    if kind == 'writes':
        client = WRITES.Client.__new__(WRITES.Client)
        client.input, client.output = peer.stdin, peer.stdout
        client.transport, client.log = 'stdio', None
    else:
        client = PAYER.Stdio.__new__(PAYER.Stdio)
        client.stream = None
    client.process, client.connection = peer, None
    client.candidate = True
    client.sequence, client.starts = 0, 1
    client.response_pending = bytearray()
    client.directory, client.label = directory, kind
    return client


def bounded_call(client, peer):
    result = []
    def invoke():
        try:
            result.append(client.message('initialize', {}))
        except BaseException as error:
            result.append(error)
    worker = threading.Thread(target=invoke, daemon=True)
    worker.start()
    worker.join(33)
    if worker.is_alive():
        peer.close()
        worker.join(3)
        pytest.fail('partial MCP line escaped the complete 30-second response deadline')
    assert len(result) == 1
    return result[0]


@pytest.mark.parametrize('kind', ['writes', 'payer'])
def test_mcp_partial_line_has_one_deadline_and_terminates_child(tmp_path, kind):
    peer = ForkedMcp('partial')
    client = mcp_client(kind, peer, tmp_path)
    started = time.monotonic()
    try:
        result = bounded_call(client, peer)
        assert isinstance(result, ValueError) and 'deadline' in str(result)
        assert 29 <= time.monotonic() - started <= 33
        assert peer.poll() is not None
        assert not list(tmp_path.glob('*response*'))
    finally:
        peer.close()


@pytest.mark.parametrize('kind', ['writes', 'payer'])
@pytest.mark.parametrize('mode', ['chunked', 'exact-bound', 'oversize', 'eof', 'wrong-id'])
def test_mcp_real_json_codec_bounds_and_invocation_identity(tmp_path, kind, mode):
    peer = ForkedMcp(mode)
    client = mcp_client(kind, peer, tmp_path)
    try:
        if mode in ('chunked', 'exact-bound'):
            result = client.message('initialize', {})
            assert result['structuredContent'] == {'tool': 'initialize', 'accepted': True}
            assert client.sequence == 1
            # Exercise the real tools/call producer and its content consumer too.
            refused, content = client.call({'tool': 'wallet.send', 'arguments': {'amount': '7'}, 'idempotency_key': '44' * 32})
            assert not refused and content == {'tool': 'wallet.send', 'accepted': True}
            assert client.sequence == 2
        else:
            failure = WRITES.OWNERS.READINESS.HTTP.Failure if kind == 'writes' and mode == 'wrong-id' else ValueError
            with pytest.raises(failure):
                client.message('initialize', {})
            assert not list(tmp_path.glob('*response*'))
    finally:
        client.stop()
        peer.close()


@pytest.mark.parametrize('kind', ['writes', 'payer'])
def test_mcp_trickle_cannot_reset_the_deadline_and_coalesced_lines_survive(kind):
    framing = WRITES.FRAMING if kind == 'writes' else PAYER.FRAMING
    read_fd, write_fd = os.pipe()
    stream = os.fdopen(read_fd, 'rb')
    stopped = threading.Event()
    def trickle():
        while not stopped.wait(0.01):
            os.write(write_fd, b' ')
    thread = threading.Thread(target=trickle)
    thread.start()
    started = time.monotonic()
    try:
        with pytest.raises(ValueError, match='deadline'):
            framing.read_line(stream, bytearray(), started + 0.15)
        assert 0.12 <= time.monotonic() - started < 0.5
        assert os.get_blocking(read_fd)
    finally:
        stopped.set()
        thread.join()
        stream.close()
        os.close(write_fd)
    read_fd, write_fd = os.pipe()
    with os.fdopen(read_fd, 'rb') as stream:
        os.write(write_fd, b'{"id":1}\n{"id":2}\n')
        pending = bytearray()
        assert json.loads(framing.read_line(stream, pending, time.monotonic() + 1)) == {'id': 1}
        assert json.loads(framing.read_line(stream, pending, time.monotonic() + 1)) == {'id': 2}
        assert not pending
    os.close(write_fd)


class WirePeer:
    """Committed node recordings, exact ABI views, and tenant-bound gateway state."""
    def __init__(self, state, records=None):
        self.state = state
        self.records = records or [case('V1-layerxledger-pay-exact'), case('V1-layerxprograms-call')]
        self.by_hash = {row['receiptJson']['txHash']: row for row in self.records}
        self.views, self.errors, self.requests = {}, [], []
        self.tenant_records = {'tenant-a': self.records[0], 'tenant-b': self.records[-1]}
        self.credentials = {name: {'tenant': name, 'session_id': char * 64, 'token_id': token * 64, 'generation': '1'}
                            for name, char, token in [('tenant-a', '1', '2'), ('tenant-b', '3', '4')]}
        self.store = json.loads(state.read_text()) if state.exists() else {'accepted': {}, 'failed': [], 'submissions': {}}
        self.server = self.thread = None
        self.port = 0
        self.transform = None
        self.chain_id = '0x7d'
        self.starts = 0

    def persist(self):
        self.state.write_text(json.dumps(self.store, sort_keys=True))
        self.state.chmod(0o600)

    def execution(self, reference):
        row = deepcopy(self.by_hash[reference]['receiptJson'])
        row.update(rung='committed', blockHash='0x' + keccak256(bytes.fromhex(reference[2:])).hex(), blockNumber=row['height'])
        return row

    def dispatch(self, path, request):
        if request.get('jsonrpc') == '2.0':
            assert path == '/'
            method, params = request['method'], request['params']
            if method == 'eth_chainId':
                result = self.chain_id
            elif method == 'px_getReceiptKeys':
                result = {'keys': [self.records[0]['pinnedKey']]}
            elif method == 'eth_sendRawTransaction':
                raw = params[0]
                reference = '0x' + keccak256(bytes.fromhex(raw[2:])).hex()
                assert raw == self.by_hash[reference]['rawTransaction']
                self.store['accepted'][reference] = raw
                self.persist()
                result = reference
            elif method in ('pax_sequencerReceipt', 'eth_getRawTransactionByHash', 'eth_getTransactionReceipt'):
                reference = params[0]
                row = self.execution(reference)
                if method == 'pax_sequencerReceipt':
                    result = row
                elif method == 'eth_getRawTransactionByHash':
                    result = self.by_hash[reference]['rawTransaction']
                else:
                    result = {'transactionHash': reference, 'blockHash': row['blockHash'], 'blockNumber': row['blockNumber'],
                              'transactionIndex': row['index'], 'status': row['status'], 'from': row['from'], 'to': row['to'], 'logs': row['logs']}
            elif method == 'eth_getBlockByNumber':
                rows = [self.execution(reference) for reference in self.by_hash if self.execution(reference)['blockNumber'] == params[0]]
                assert len(rows) == 1 and params[1] is False
                result = {'hash': rows[0]['blockHash'], 'number': params[0], 'transactions': [rows[0]['txHash']]}
            elif method == 'eth_call':
                assert params[0]['to'] == CHAIN.PROGRAMS and params[1] == 'latest'
                result = self.views[params[0]['data']]
                if isinstance(result, dict) and 'code' in result:
                    raise CHAIN.RpcError(result)
            else:
                raise AssertionError('unexpected method ' + method)
            if self.transform:
                self.transform(method, result)
            return 200, {'jsonrpc': '2.0', 'id': request['id'], 'result': result}
        assert path == '/v1/agent/rpc'
        identifier, credential = request['request_id'], request['credential']
        tenant = credential['tenant']
        if credential != self.credentials.get(tenant):
            return 401, {'request_id': identifier, 'class': 'PolicyRefusal', 'reason': 'session.not_authorized'}
        operation = request['operation']
        record = self.tenant_records[tenant]
        reference = record['receiptJson']['txHash'][2:]
        payload = request['request']
        if operation == 'tenant.readiness':
            if payload:
                return 403, {'request_id': identifier, 'class': 'PolicyRefusal', 'reason': 'envelope.coordinate_mismatch'}
            failed = tenant in self.store['failed']
            value = {'transport_ready': True, 'verified_reads_ready': True, 'writes_admitted': not failed,
                     'recovery_reason': 'budget_state_unverified' if failed else None}
            verification = {'state': 'achieved', 'level': 'Unverified'}
        elif operation == 'read.account':
            value = {'account_id': payload['account_id'], 'canonical_value': '01', 'proof': record['prevSeqHash'], 'sequence': '7'}
            verification = {'state': 'achieved', 'level': 'StateProven'}
        else:
            if operation == 'submit':
                if tenant in self.store['failed']:
                    return 403, {'request_id': identifier, 'class': 'PolicyRefusal', 'reason': 'owner.refused'}
                assert payload == {'raw_transaction': record['rawTransaction'][2:]}
                assert request['idempotency_key'] == '55' * 32
                self.store['submissions'][tenant] = reference
                self.store['accepted']['0x' + reference] = record['rawTransaction']
                self.persist()
            elif operation == 'track':
                if payload['submission_ref'] != self.store['submissions'].get(tenant):
                    return 403, {'request_id': identifier, 'class': 'PolicyRefusal', 'reason': 'owner.refused'}
            elif operation == 'program.receipt':
                assert payload['requested_rung'] == 'committed' and payload['idempotency_key'] == '55' * 32
                if payload['expected_activity_id'] != self.store['submissions'].get(tenant):
                    return 403, {'request_id': identifier, 'class': 'PolicyRefusal', 'reason': 'owner.refused'}
            else:
                raise AssertionError(operation)
            verification = {'state': 'achieved', 'rung': 'committed'}
            if operation == 'program.receipt':
                value = {'found': True, 'receipt': {'tx_hash': reference, 'canonical_bytes': record['signedReceipt'][2:], 'rung': 'committed'}}
            else:
                value = {'submission_ref': reference, 'tx_hash': reference, 'state': 'Committed', 'rung': 'committed',
                         'receipt_ref': receipt_identity(record), 'evidence': [], 'transitions': []}
        return 200, {'request_id': identifier, 'value': value, 'verification_status': verification}

    def start(self):
        peer = self
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass
            def do_POST(self):
                request = {}
                try:
                    assert self.headers['Content-Type'] == 'application/json'
                    assert self.headers['Authorization'] == AUTH
                    assert self.headers.get('LayerX-Tenant') is None
                    request = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                    peer.requests.append((self.path, deepcopy(request)))
                    status, result = peer.dispatch(self.path, request)
                except CHAIN.RpcError as error:
                    status, result = 200, {'jsonrpc': '2.0', 'id': request['id'], 'error': error.error}
                except BaseException as error:
                    peer.errors.append(error)
                    status, result = 500, {'error': repr(error)}
                raw = json.dumps(result).encode()
                self.send_response(status)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)
        self.server = ThreadingHTTPServer(('127.0.0.1', self.port), Handler)
        self.port = self.server.server_port
        self.thread = threading.Thread(target=self.server.serve_forever, kwargs={'poll_interval': 0.01}, daemon=True)
        self.thread.start()
        self.starts += 1
        return f'http://127.0.0.1:{self.port}/'

    def stop(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(3)
        assert not self.thread.is_alive()
        self.server = None
        assert not self.errors, self.errors

    def restart(self):
        self.stop()
        self.store = json.loads(self.state.read_text())
        self.start()


def fixture_for(tmp_path, node_url, gateway_url, record):
    fixture = HTTP.AgentdFixture.__new__(HTTP.AgentdFixture)
    authorization = tmp_path / 'authorization'
    authorization.write_text(AUTH)
    authorization.chmod(0o600)
    fixture.values = {'key': str(authorization)}
    fixture.document = {'seeds': {'key': str(authorization)}, 'receipt_keys': pins(record), 'endpoints': {
        name: {'url': url, 'authorization_seed': 'key'} for name, url in
        [('node', node_url), ('gateway', gateway_url), ('agent', gateway_url + 'v1/agent/rpc')]}}
    fixture.configure_endpoints()
    return fixture


@pytest.fixture
def peers(tmp_path):
    node = WirePeer(tmp_path / 'node.json')
    gateway = WirePeer(tmp_path / 'gateway.json')
    node_url, gateway_url = node.start(), gateway.start()
    fixture = fixture_for(tmp_path, node_url, gateway_url, node.records[0])
    try:
        yield node, gateway, fixture
    finally:
        node.stop()
        gateway.stop()


def corpus(tmp_path, fixture, peer):
    result = READINESS.Corpus.__new__(READINESS.Corpus)
    result.fixture = fixture
    result.daemon = SimpleNamespace(fixture=fixture)
    result.evidence = tmp_path
    tmp_path.mkdir(mode=0o700, parents=True, exist_ok=True)
    result.next_request, result.calls, result.outcomes = 1000, 0, {}
    result.tenants = []
    for name, credential in peer.credentials.items():
        record = peer.tenant_records[name]
        result.tenants.append({'credential': credential, 'evm_address': record['receiptJson']['from'],
            'read': {'request': {'account_id': ('aa' if name == 'tenant-a' else 'bb') * 32}},
            'prepare': {'idempotency_key': '55' * 32, 'request': {'idempotency_key': '55' * 32}},
            'submit': {'raw_transaction': record['rawTransaction'][2:]}})
    return result


def test_agent_rpc_routes_node_root_hosted_root_and_authenticated_gateway(peers, tmp_path):
    node, gateway, fixture = peers
    daemon = SimpleNamespace(fixture=fixture)
    assert HTTP.rpc_request(daemon, {'jsonrpc': '2.0', 'id': 7, 'method': 'eth_chainId', 'params': []}) == (
        200, {'jsonrpc': '2.0', 'id': 7, 'result': '0x7d'})
    status, keys = HTTP.rpc_request(daemon, {'jsonrpc': '2.0', 'id': 8, 'method': 'px_getReceiptKeys', 'params': []})
    assert status == 200 and keys['result']['keys'] == [gateway.records[0]['pinnedKey']]
    qualifier = corpus(tmp_path / 'evidence', fixture, gateway)
    assert qualifier.readiness(qualifier.tenants[0], True)['writes_admitted']
    assert [path for path, _ in node.requests] == ['/']
    assert [path for path, _ in gateway.requests] == ['/', '/v1/agent/rpc']
    assert gateway.requests[-1][1]['credential'] == qualifier.tenants[0]['credential']
    with pytest.raises(HTTP.FixtureRefused):
        fixture.routed_request({'jsonrpc': '2.0', 'id': 9, 'method': 'undeclared', 'params': []})
    fixture.document['endpoints']['node']['url'] = fixture.endpoints['node']['url'] + 'rpc'
    with pytest.raises(HTTP.FixtureRefused, match='route'):
        fixture.configure_endpoints()


def test_agent_dispatch_producer_recording_uses_authenticated_gateway_route(tmp_path):
    spec = importlib.util.spec_from_file_location('repair_recorded_agent', QUAL / 'fixtures/mcp_recovery_recording.py')
    recording = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(recording)
    peer = recording.RecordedAgentd(tmp_path / 'recorded')
    peer.start()
    try:
        envelope = peer.envelope('submit')
        status, response = HTTP.rpc_request(peer, envelope)
        assert status == 200 and response['request_id'] == envelope['request_id']
        assert response['value'] == peer.recording['pending']['submission']
        assert peer.requests == [envelope]
    finally:
        peer.stop()


def test_tenant_readiness_committed_write_cross_tenant_refusal_and_reopen(peers, tmp_path):
    node, gateway, fixture = peers
    qualifier = corpus(tmp_path / 'first', fixture, gateway)
    gateway.store['failed'] = ['tenant-a']
    gateway.persist()
    assert not qualifier.readiness(qualifier.tenants[0], False, 'budget_state_unverified')['writes_admitted']
    assert qualifier.readiness(qualifier.tenants[1], True)['writes_admitted']
    status, refusal = qualifier.call(qualifier.envelope(qualifier.tenants[0], 'submit', qualifier.tenants[0]['submit'], True))
    assert status == 403 and refusal['reason'] == 'owner.refused'
    facts = qualifier.write(1)
    assert facts['block_hash'] and facts['block_number'] and facts['transaction_hash'].endswith(facts['activity_id'])
    foreign = qualifier.envelope(qualifier.tenants[0], 'track', {'submission_ref': facts['activity_id']})
    assert qualifier.call(foreign)[0] == 403
    accepted = gateway.state.read_bytes()
    gateway.restart()
    node.persist()
    node.restart()
    reopened = corpus(tmp_path / 'reopened', fixture, gateway)
    reopened.outcomes = deepcopy(qualifier.outcomes)
    assert not reopened.readiness(reopened.tenants[0], False, 'budget_state_unverified')['writes_admitted']
    assert reopened.retained(1) == facts
    assert gateway.state.read_bytes() == accepted
    assert len(gateway.store['accepted']) == 1
    # Repairing the retained owner only admits that tenant; committed identity stays stable.
    gateway.store['failed'] = []
    gateway.persist()
    gateway.restart()
    assert reopened.readiness(reopened.tenants[0], True)['writes_admitted']
    assert reopened.retained(1) == facts


@pytest.mark.parametrize('damage', ['uncommitted', 'block-hash', 'block-number', 'block-membership', 'logs', 'index'])
def test_committed_receipt_requires_matching_node_block_and_standard_receipt(peers, damage):
    node, gateway, fixture = peers
    reference = node.records[0]['receiptJson']['txHash']
    def transform(method, result):
        if damage == 'uncommitted' and method == 'pax_sequencerReceipt':
            result['rung'] = 'receipted'
        if damage == 'block-hash' and method == 'eth_getTransactionReceipt':
            result['blockHash'] = '0x' + '01' * 32
        if damage == 'block-number' and method == 'eth_getTransactionReceipt':
            result['blockNumber'] = '0x1'
        if damage == 'block-membership' and method == 'eth_getBlockByNumber':
            result['transactions'] = []
        if damage == 'logs' and method == 'eth_getTransactionReceipt':
            result['logs'] = []
        if damage == 'index' and method == 'eth_getTransactionReceipt':
            result['transactionIndex'] = '0x99'
    node.transform = transform
    with pytest.raises(ValueError):
        fixture.node().committed(reference)


def test_tenant_receipt_rejects_foreign_payer_and_canonical_receipt(peers):
    node, gateway, fixture = peers
    record = node.records[0]
    value = {'activity_id': record['receiptJson']['txHash'][2:],
        'submission': {'state': 'Committed', 'rung': 'committed', 'tx_hash': record['receiptJson']['txHash'][2:],
                       'receipt_ref': receipt_identity(record)},
        'receipt': {'rung': 'committed', 'tx_hash': record['receiptJson']['txHash'][2:], 'canonical_bytes': record['signedReceipt'][2:]}}
    tenant = {'evm_address': record['receiptJson']['from']}
    assert READINESS.committed_facts(fixture, value, tenant)['receipt_digest'] == receipt_identity(record)
    with pytest.raises(ValueError, match='tenant payer'):
        READINESS.committed_facts(fixture, value, {'evm_address': '0x' + 'ee' * 20})
    for missing in (None, '', '0', 'zz'):
        value['receipt']['canonical_bytes'] = missing
        with pytest.raises(READINESS.HTTP.Failure, match='canonical tenant receipt'):
            READINESS.committed_facts(fixture, value, tenant)
    value['receipt']['canonical_bytes'] = node.records[1]['signedReceipt'][2:]
    with pytest.raises(ValueError, match='tenant receipt'):
        READINESS.committed_facts(fixture, value, tenant)


def settlement(record, peer):
    tx, _ = decode_signed(bytes.fromhex(record['rawTransaction'][2:]))
    words = [tx.data[index:index + 32].hex() for index in range(4, len(tx.data), 32)]
    offer = {'payer': words[0], 'payTo': words[1], 'asset': words[2], 'amount': str(int(words[3], 16)),
             'currency': 'PAX', 'scheme': 'exact', 'network': 'paxeer:125'}
    row = peer.execution(record['receiptJson']['txHash'])
    import hashlib
    value = dict(offer, rung='committed', transactionHash=row['txHash'], blockHash=row['blockHash'], blockNumber=row['blockNumber'],
                 receiptDigest=hashlib.sha256(bytes.fromhex(record['signedReceipt'][2:])).hexdigest())
    return offer, value


def test_mcp_payer_exact_offer_committed_evidence_and_deduplicated_reopen(peers):
    node, gateway, fixture = peers
    record = node.records[0]
    offer, payment = settlement(record, node)
    reference = fixture.node().call('eth_sendRawTransaction', [record['rawTransaction']])
    first = PAYER.committed_settlement(fixture, payment, offer)
    before = node.state.read_bytes()
    node.restart()
    assert fixture.node().call('eth_sendRawTransaction', [record['rawTransaction']]) == reference
    assert PAYER.committed_settlement(fixture, payment, offer) == first
    assert node.state.read_bytes() == before and len(node.store['accepted']) == 1


@pytest.mark.parametrize('field', ['payer', 'payTo', 'asset', 'amount', 'currency', 'scheme', 'network',
                                  'rung', 'transactionHash', 'blockHash', 'blockNumber', 'receiptDigest'])
def test_mcp_payer_refuses_changed_offer_or_commitment(peers, field):
    node, gateway, fixture = peers
    offer, payment = settlement(node.records[0], node)
    payment[field] = 'changed'
    with pytest.raises(ValueError):
        PAYER.committed_settlement(fixture, payment, offer)


@pytest.mark.parametrize('field', ['payer', 'payTo', 'asset', 'amount'])
def test_mcp_payer_claimed_offer_must_also_match_signed_transaction(peers, field):
    node, gateway, fixture = peers
    offer, payment = settlement(node.records[0], node)
    offer[field] = payment[field] = '9' if field == 'amount' else '09' * 32
    with pytest.raises(ValueError, match='transaction differs'):
        PAYER.committed_settlement(fixture, payment, offer)


def arguments_for_call(record):
    tx, _ = decode_signed(bytes.fromhex(record['rawTransaction'][2:]))
    data = tx.data[4:]
    def word(index):
        return int.from_bytes(data[index * 32:(index + 1) * 32], 'big')
    def dynamic(index):
        offset = word(index)
        size = int.from_bytes(data[offset:offset + 32], 'big')
        return data[offset + 32:offset + 32 + size]
    return [data[:32].hex(), word(1), dynamic(2).decode(), dynamic(3).hex(), dynamic(4).hex(), dynamic(5).hex(),
            word(6), [word(i) for i in range(7, 14)], word(14)]


@pytest.mark.parametrize('task', ['6.4', '6.6'])
def test_programs_case_inventory_keeps_every_historical_invariant(task):
    required = WINDDOWN.REQUIRED if task == '6.6' else WINDDOWN.REQUIRED | WINDDOWN.ACCOUNTS_REQUIRED
    rows = [{'id': name} for name in sorted(required)]
    assert WINDDOWN.case_inventory(task, rows) == rows
    for index in range(len(rows)):
        with pytest.raises(ValueError, match='inventory'):
            WINDDOWN.case_inventory(task, rows[:index] + rows[index + 1:])
    with pytest.raises(ValueError, match='inventory'):
        WINDDOWN.case_inventory(task, rows[:-1] + [rows[0]])


def observe_balance(program='11' * 32, account='22' * 32):
    return {'method': 'getValueAccount', 'arguments': [program, account], 'expected': '0x' + (7).to_bytes(32, 'big').hex() + '00' * 32}


def bind_views(node, peer, reads):
    for row in reads:
        calldata = node.program_call(row['method'], node.json_arguments(row['method'], row['arguments']))
        peer.views['0x' + calldata.hex()] = row['expected']


def test_programs_winddown_merged_send_receipt_conservation_and_restart(peers):
    peer, gateway, fixture = peers
    record = peer.records[1]
    node = CHAIN.Node(fixture.endpoints['node']['url'], fixture.document['receipt_keys'], {'Authorization': AUTH})
    reads = [observe_balance()]
    bind_views(node, peer, reads)
    row = {'id': 'conservation', 'before': reads, 'after': deepcopy(reads), 'restart': True, 'expected_total_delta': 0,
           'steps': [{'method': 'call', 'arguments': arguments_for_call(record), 'raw_transaction': record['rawTransaction'], 'refusal': None}]}
    # Source-generated chain-125 calldata traverses the actual Programs send client,
    # pinned receipt verifier and committed block reader before and after reopen.
    peer.persist()
    result = WINDDOWN.WinddownSession(node, peer.restart).case(row)
    assert len(result['receipts']) == 1 and result['refusals'] == 0
    assert peer.starts == 2 and len(peer.store['accepted']) == 1
    assert result['before'] == result['after']
    assert any(request['method'] == 'eth_sendRawTransaction' for _, request in peer.requests)
    assert all(path == '/' for path, _ in peer.requests)


@pytest.mark.parametrize('name', sorted(name for name in WINDDOWN.REQUIRED | WINDDOWN.ACCOUNTS_REQUIRED
                                       if name.endswith('refused') or name == 'missing-transfer-capability'))
def test_programs_all_retained_refusals_keep_exact_state_and_balances(peers, name):
    peer, gateway, fixture = peers
    node = CHAIN.Node(fixture.endpoints['node']['url'], fixture.document['receipt_keys'], {'Authorization': AUTH})
    reads = [observe_balance()]
    bind_views(node, peer, reads)
    arguments = ['11' * 32, '22' * 32, '33' * 32, '44' * 32, '73656564']
    encoded = node.program_call('setExitRoute', node.json_arguments('setExitRoute', arguments))
    refusal = {'code': 3, 'message': 'execution reverted', 'data': '0x08c379a0' + '00' * 32}
    peer.views['0x' + encoded.hex()] = refusal
    row = {'id': name, 'before': reads, 'after': deepcopy(reads), 'restart': False, 'expected_total_delta': 0,
           'steps': [{'method': 'setExitRoute', 'arguments': arguments, 'raw_transaction': None, 'refusal': refusal}]}
    result = WINDDOWN.WinddownSession(node, peer.restart).case(row)
    assert result['refusals'] == 1 and result['before'] == result['after'] and not result['receipts']


@pytest.mark.parametrize('name', ['native-restart', 'restart-exit-eligibility'])
def test_programs_restart_reopens_accounts_and_exit_eligibility(peers, name):
    peer, gateway, fixture = peers
    node = CHAIN.Node(fixture.endpoints['node']['url'], fixture.document['receipt_keys'], {'Authorization': AUTH})
    reads = [observe_balance(), {'method': 'getWindDown', 'arguments': ['11' * 32],
        'expected': '0x' + '00' * 31 + '01' + '22' * 32 + '00' * 31 + '64' + '00' * 31 + '07' + '00' * 31 + '01' + '00' * 31 + '01'}]
    bind_views(node, peer, reads)
    peer.persist()
    row = {'id': name, 'before': reads, 'after': deepcopy(reads), 'steps': [], 'restart': True, 'expected_total_delta': 0}
    assert WINDDOWN.WinddownSession(node, peer.restart).case(row)['after'] == [read['expected'] for read in reads]
    assert peer.starts == 2


@pytest.mark.parametrize('name', ['deploy', 'upgrade', 'wind-down-route', 'wind-down-deprecate', 'wind-down-exit', 'wind-down-tombstone'])
def test_programs_current_evm_lifecycle_matches_committed_native_producer_codec(peers, name):
    peer, gateway, fixture = peers
    node = CHAIN.Node(fixture.endpoints['node']['url'], fixture.document['receipt_keys'], {'Authorization': AUTH})
    record = json.loads((ROOT / ('sdk/platform/conformance/fixtures/native-program-' + name + '-v4.json')).read_text())
    payload = bytes.fromhex(record['payload_hex'])
    if name == 'deploy':
        value = NativeProgramDeploy.decode(payload)
        method, arguments = 'deploy', [value.program_id, value.guest_abi, value.policy, value.authority, value.new_hash, value.interface or b'', value.wasm]
    elif name == 'upgrade':
        value = NativeProgramUpgrade.decode(payload)
        method, arguments = 'upgrade', [value.program_id, value.guest_abi, int(bool(value.migration_hook)) | int(value.clear_interface) << 1,
            value.old_hash, value.new_hash, value.migration_hook, value.interface or b'', value.wasm, 1000]
    else:
        # The independently committed native encoder supplies the ABI call, too.
        from importlib.util import spec_from_file_location, module_from_spec
        spec = spec_from_file_location('lifecycle_fixture_producer', ROOT / 'sdk/platform/conformance/fixtures/generate_native_lifecycle_fixtures.py')
        producer = module_from_spec(spec)
        spec.loader.exec_module(producer)
        assert producer.evm(record) == record['evm']
        wind = NativeProgramWindDown.decode(payload)
        assert wind.encode() == payload
        if name == 'wind-down-route':
            method, arguments = 'setExitRoute', [payload[:32], payload[33:65], payload[65:97], payload[97:129], payload[131:]]
        elif name == 'wind-down-deprecate':
            method, arguments = 'deprecate', [payload[:32], payload[33:65], int.from_bytes(payload[65:73], 'big')]
        elif name == 'wind-down-exit':
            method, arguments = 'exitValue', [payload[:32], payload[33:65]]
        else:
            method, arguments = 'tombstone', [payload[:32]]
    encoded = node.program_call(method, arguments)
    assert '0x' + encoded.hex() == record['evm']['calldata']
    peer.views['0x' + encoded.hex()] = '0x'
    assert node.program_read(method, arguments) == '0x'
    assert peer.requests[-1][1]['params'][0]['to'] == CHAIN.PROGRAMS


def test_programs_never_admits_wrong_signed_action_or_chain(peers):
    peer, gateway, fixture = peers
    node = CHAIN.Node(fixture.endpoints['node']['url'], fixture.document['receipt_keys'], {'Authorization': AUTH})
    record = peer.records[1]
    with pytest.raises(ValueError, match='declared action'):
        node.program_send(record['rawTransaction'], 'tombstone', [bytes.fromhex('11' * 32)])
    peer.chain_id = '0x1'
    with pytest.raises(ValueError, match='chain 125'):
        node.program_read('getProgram', [bytes.fromhex('11' * 32)])
    assert not peer.store['accepted']


def test_programs_source_config_uses_root_evm_and_preserves_abi_binding(peers):
    peer, gateway, fixture = peers
    chain, url = SOURCE.chain_configuration({'LAYERX_REGISTRY_CHAIN_RPC_URL': fixture.endpoints['node']['url']})
    assert url.endswith('/') and chain.PROGRAMS == '0x000000000000000000000000000000000000101d'
    node = CHAIN.Node(fixture.endpoints['node']['url'], fixture.document['receipt_keys'], {'Authorization': AUTH})
    program, code_hash = '11' * 32, '22' * 32
    result = '0x' + (1).to_bytes(32, 'big').hex() + '33' * 32 + code_hash + (4).to_bytes(32, 'big').hex() + (7).to_bytes(32, 'big').hex() + (1).to_bytes(32, 'big').hex()
    calldata = '0x' + node.program_call('getProgram', [bytes.fromhex(program)]).hex()
    peer.views[calldata] = result
    assert SOURCE.program_binding(node, program, 4, code_hash, 7) == result
    for abi in (0, 1, 5, 65535):
        with pytest.raises(ValueError, match='source/ABI'):
            SOURCE.program_binding(node, program, abi, code_hash, 7)
    with pytest.raises(ValueError, match='source/ABI'):
        SOURCE.program_binding(node, program, 4, 'ff' * 32, 7)
    with pytest.raises(ValueError):
        SOURCE.chain_configuration({'LAYERX_REGISTRY_CHAIN_RPC_URL': url + 'rpc'})
    with pytest.raises(ValueError):
        SOURCE.chain_configuration({'LAYERX_REGISTRY_NODE_ENDPOINT': url})


def test_programs_source_archive_and_wasm_negative_cases_still_execute(tmp_path):
    record = json.loads((ROOT / 'sdk/platform/conformance/fixtures/native-program-deploy-v4.json').read_text())
    module = NativeProgramDeploy.decode(bytes.fromhex(record['payload_hex'])).wasm
    name = b'module.wasm'
    archive = b'LXSRCv1\0' + (1).to_bytes(4, 'big') + len(name).to_bytes(2, 'big') + name + b'\0' + len(module).to_bytes(4, 'big') + module
    (tmp_path / 'source.archive').write_bytes(archive)
    (tmp_path / 'module.wasm').write_bytes(module)
    (tmp_path / 'source.uri').write_text('ipfs://recorded-program-module')
    plan = 'version = 1\n' + ''.join(key + ' = ' + '11' * 32 + '\n' for key in ('toolchain_digest', 'dependency_lock_digest', 'builder_image_digest'))
    (tmp_path / 'source.plan').write_text(plan)
    assert SOURCE.source_input(tmp_path)['archive'] == archive
    assert isinstance(SOURCE.wasm_imports(module), list)
    (tmp_path / 'source.archive').write_bytes(b'not-an-archive')
    with pytest.raises(ValueError, match='source archive'):
        SOURCE.source_input(tmp_path)
    with pytest.raises(ValueError, match='Wasm'):
        SOURCE.wasm_imports(b'not-wasm')
    (tmp_path / 'source.archive').write_bytes(archive)
    (tmp_path / 'source.plan').write_text(plan + 'version = 1\n')
    with pytest.raises(ValueError, match='duplicate'):
        SOURCE.source_input(tmp_path)
