"""Recorded paxd -> HTTP gateway -> qualification/source-consumer proofs.

Receipts and transactions are replayed byte for byte from the committed RPC
corpora. New deployment envelopes are signed with a disposable key and receive
no invented execution receipt. The native peer here stores consumed source
receipts; it makes no claim to execute the escrow/vault guests or native gates.
"""
from contextlib import contextmanager
from copy import deepcopy
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
from pathlib import Path
import sys
import threading
import urllib.error
import urllib.request

import pytest

ROOT = Path(__file__).resolve().parents[3]
sys.dont_write_bytecode = True
sys.path.insert(0, str(ROOT / 'services/agent/sdk/python'))
from layerx_sdk.account_derivation import _N, _evm_address, _recover, keccak256
from layerx_sdk.evm_transaction import Type2Transaction, decode_signed, encode_call
from layerx_sdk.native_program_call import NativeProgramCall, encode_native_program_call
from layerx_sdk.production import PlatformSdkError
from layerx_sdk.program_lifecycle import NativeProgramDeploy
from layerx_sdk.verifier import SequencerReceiptVerificationError, verify_sequencer_receipt

BASE = Path(__file__).parent
REVISION = 'caa8b41ee668274ee1e5c1df6691288455c46278'
SOURCE = {'revision': REVISION, 'tree': 'cf582d1bf04b67f6e117557cd67add1ae6c19d5f'}
AUTH = 'LayerX-Key recorded:lxp_live_' + '17' * 32


def component(name):
    spec = importlib.util.spec_from_file_location('repair_808_' + name.replace('-', '_'), BASE / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def recording(name='V1-layerxprograms-call'):
    corpus = json.loads((ROOT / 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json').read_text())
    assert corpus['provenance']['generator'] == 'rpc/sequencer_receipt_vectors_test.go'
    return deepcopy(next(row for row in corpus['cases'] if row['id'] == name))


def committed_recording():
    original = json.loads((ROOT / 'daemon/committed-fixture/vectors/vector-v4-committed.json').read_text())
    converted = json.loads((ROOT / 'sdk/platform/conformance/fixtures/vector-v4-committed.json').read_text())['positive']
    result = original['settlement']
    assert result['receipt']['rung'] == 'committed'
    assert result['receipt']['receipt'] == converted['signedReceipt']
    assert result['raw_transaction'] == converted['rawTransaction']
    transaction, _ = decode_signed(bytes.fromhex(result['raw_transaction'][2:]))
    assert transaction.chain_id == 125
    assert result['raw_transaction'].startswith('0x02')
    return {'rawTransaction': result['raw_transaction'], 'receiptJson': result['receipt'],
            'signedReceipt': converted['signedReceipt'], 'pinnedKey': converted['pinnedKey'],
            'network_id': int.from_bytes(bytes.fromhex(original['positive']['header'][2:])[9:13], 'big')}


def pins(case):
    return {'keys': [deepcopy(case['pinnedKey'])]}


class RecordedChain:
    """A replay store and an HTTP relay, never a replacement receipt producer."""
    def __init__(self, case):
        self.case = deepcopy(case)
        self.requests = []
        self.accepted = {}
        self.errors = []
        self.gateway_change = None
        self.chain_id = hex(case.get('network_id', 125))
        self.gateway_authorization = AUTH

    def dispatch(self, request):
        method, params = request['method'], request['params']
        if method == 'eth_chainId':
            assert params == []
            return self.chain_id
        if method == 'px_getReceiptKeys':
            assert params == []
            return pins(self.case)
        if method == 'eth_sendRawTransaction':
            assert len(params) == 1
            raw = bytes.fromhex(params[0][2:])
            producer = component('program-http-producer')
            assert producer.transaction(raw)[0] == int(self.chain_id, 16)
            tx_hash = '0x' + keccak256(raw).hex()
            if tx_hash in self.accepted:
                assert self.accepted[tx_hash] == params[0]
            self.accepted[tx_hash] = params[0]
            return tx_hash
        if method == 'pax_sequencerReceipt':
            assert len(params) == 1
            if params[0] != self.case['receiptJson']['txHash']:
                return None
            return deepcopy(self.case['receiptJson'])
        raise AssertionError(method)

    @contextmanager
    def serving(self, tmp_path):
        peer = self

        class Node(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                try:
                    assert self.path == '/'
                    assert self.headers['Content-Type'] == 'application/json'
                    assert self.headers.get('Authorization') is None
                    size = int(self.headers['Content-Length'])
                    assert 0 < size <= 2_097_152
                    request = json.loads(self.rfile.read(size))
                    assert set(request) == {'jsonrpc', 'id', 'method', 'params'}
                    assert request['jsonrpc'] == '2.0' and type(request['id']) is int
                    peer.requests.append(('paxd', deepcopy(request)))
                    reply = {'jsonrpc': '2.0', 'id': request['id'], 'result': peer.dispatch(request)}
                    status = 200
                except Exception as error:
                    peer.errors.append(error)
                    status, reply = 500, {'error': 'recorded-request-mismatch'}
                encoded = json.dumps(reply).encode()
                self.send_response(status)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(encoded)))
                self.end_headers()
                self.wfile.write(encoded)

        node = ThreadingHTTPServer(('127.0.0.1', 0), Node)
        node_url = f'http://127.0.0.1:{node.server_port}/'

        class Gateway(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                try:
                    assert self.path == '/'
                    assert self.headers['Content-Type'] == 'application/json'
                    if self.headers.get('Authorization') != peer.gateway_authorization:
                        self.send_response(401)
                        self.send_header('Content-Length', '0')
                        self.end_headers()
                        return
                    size = int(self.headers['Content-Length'])
                    assert 0 < size <= 2_097_152
                    body = self.rfile.read(size)
                    request = json.loads(body)
                    peer.requests.append(('gateway', deepcopy(request)))
                    outbound = urllib.request.Request(node_url, data=body, headers={'Content-Type': 'application/json'})
                    with urllib.request.urlopen(outbound, timeout=5) as response:
                        reply = json.loads(response.read())
                    if peer.gateway_change:
                        peer.gateway_change(request, reply)
                    encoded = json.dumps(reply).encode()
                    status = 200
                except Exception as error:
                    peer.errors.append(error)
                    status, encoded = 500, b'{"error":"recorded-relay-mismatch"}'
                self.send_response(status)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(encoded)))
                self.end_headers()
                self.wfile.write(encoded)

        gateway = ThreadingHTTPServer(('127.0.0.1', 0), Gateway)
        threads = [threading.Thread(target=server.serve_forever, kwargs={'poll_interval': 0.01}, daemon=True)
                   for server in (node, gateway)]
        for thread in threads:
            thread.start()
        producer = component('program-http-producer')
        authorization = tmp_path / 'authorization'
        if not authorization.exists():
            producer.write(authorization, AUTH.encode())
        try:
            yield {'paxd': {'endpoint': node_url, 'authorization_file': None},
                   'gateway': {'endpoint': f'http://127.0.0.1:{gateway.server_port}/',
                               'authorization_file': str(authorization)}}
        finally:
            for server in (gateway, node):
                server.shutdown()
                server.server_close()
            for thread in threads:
                thread.join(timeout=5)
            assert all(not thread.is_alive() for thread in threads)
            assert not peer.errors


def call_row(producer, directory, case):
    raw = bytes.fromhex(case['rawTransaction'][2:])
    tx, signature = decode_signed(raw)
    call = NativeProgramCall(tx.data[4:36], 1, 'layerx_call', b'', b'\0\0',
        b'LayerX/programs/access-declaration/v1\0\0', 64,
        (1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096))
    payload = producer.write(directory / 'call.payload', encode_native_program_call(call))
    saved = producer.write(directory / 'call.transaction', raw)
    return {'name': 'recorded-call', 'operation': 'call', 'payload_file': payload,
            'transaction_file': saved, 'fee_limit': 100000000, 'expected_result_code': 0,
            'expected_terminal_result': 0, 'signer': '0x' + tx.recover_signer(signature)[2:].lower()}


def sign_transaction(tx):
    from cryptography.hazmat.primitives import hashes
    from cryptography.hazmat.primitives.asymmetric import ec, utils

    secret = ec.derive_private_key(17, ec.SECP256K1())
    numbers = secret.public_key().public_numbers()
    public = numbers.x.to_bytes(32, 'big') + numbers.y.to_bytes(32, 'big')
    signer = _evm_address(public)
    digest = tx.signing_digest()
    der = secret.sign(digest, ec.ECDSA(utils.Prehashed(hashes.SHA256())))
    r, s = utils.decode_dss_signature(der)
    s = min(s, _N - s)
    for parity in (0, 1):
        if _recover(digest, r, s, parity) == public:
            signature = r.to_bytes(32, 'big') + s.to_bytes(32, 'big') + bytes([27 + parity])
            return tx.attach_signature(signature, signer), signer.lower()
    raise AssertionError('real EVM signature did not recover')


def deployment_row(producer, directory):
    document = json.loads((ROOT / 'sdk/platform/conformance/fixtures/native-program-deploy-v3.json').read_text())
    payload = bytes.fromhex(document['payload_hex'])
    deploy = NativeProgramDeploy.decode(payload)
    assert deploy.encode() == payload
    calldata = encode_call('deploy', producer.DEPLOY_TYPES,
        [deploy.program_id, deploy.guest_abi, deploy.policy, deploy.authority, deploy.new_hash,
         deploy.interface or b'', deploy.wasm])
    assert '0x' + calldata.hex() == document['evm']['calldata']
    tx = Type2Transaction(125, 0, 1000000000, 3000000000, 2000000, producer.PROGRAMS, 0, calldata)
    raw, signer = sign_transaction(tx)
    return {'name': 'recorded-deploy-envelope', 'operation': 'deploy',
            'payload_file': producer.write(directory / 'deploy.payload', payload),
            'transaction_file': producer.write(directory / 'deploy.transaction', raw),
            'signer': signer, 'fee_limit': 1000, 'expected_result_code': 0,
            'expected_terminal_result': None}


def native_consumption(producer, directory, record):
    """Store genuine source bytes and export their consumption identity, without a native execution claim."""
    receipt = producer.rpc_bytes(record['receipts']['paxd']['receipt'])
    saved = producer.write(directory / 'consumed-source-receipt.bin', receipt)
    report = {'schema': 'paxeer-x.reference-custody-native.v1', 'authenticated_source': {
        'schema': 'paxeer-x.reference-custody-source-consumption.v1', 'receipts': [{
            'activity_id': record['receipts']['paxd']['txHash'][2:],
            'receipt_sha256': hashlib.sha256(receipt).hexdigest()}]}}
    return report, [producer.artifact(saved)]


def source_manifest(producer, directory, case, endpoints):
    raw_path = producer.write(directory / 'source.transaction', bytes.fromhex(case['rawTransaction'][2:]))
    return producer.write(directory / 'source-manifest.json', {
        'schema': producer.SCHEMA, 'source': SOURCE, 'network_id': case.get('network_id', 125),
        'endpoints': endpoints,
        'receipt_keys': pins(case), 'programs': [], 'source_transactions': [raw_path]})


def test_finding_43_signed_program_http_real_codec_and_receipt(tmp_path):
    producer = component('program-http-producer')
    case = recording()
    row = call_row(producer, tmp_path, case)
    peer = RecordedChain(case)
    with peer.serving(tmp_path) as endpoints:
        client = producer.ProgramHttpProducer(endpoints, pins(case), SOURCE, REVISION, case.get('network_id', 125))
        result = client.submit_program(row)
        verified = producer.consume_source(result['evidence'], client.pins, SOURCE)
        assert verified.canonical_bytes.hex() == case['signedReceipt'][2:]
        assert verified.receipt_digest.hex() == case['receiptDigest'][2:]
        assert verified.receipt.operation == 2
        assert result['envelope']['guest_abi'] == 1
        assert result['envelope']['program_id'] == decode_signed(bytes.fromhex(case['rawTransaction'][2:]))[0].data[4:36].hex()
        assert result['evidence']['receipts']['gateway'] == case['receiptJson']
        assert set(client.peers) == {'paxd', 'gateway'}
    sent = [request for role, request in peer.requests if role == 'gateway' and request['method'] == 'eth_sendRawTransaction']
    assert len(sent) == 1 and sent[0]['params'] == [case['rawTransaction']]
    assert peer.accepted == {case['receiptJson']['txHash']: case['rawTransaction']}


def test_finding_43_deployment_envelope_traverses_supported_http_without_invented_receipt(tmp_path):
    producer = component('program-http-producer')
    case = recording()
    row = deployment_row(producer, tmp_path)
    raw, bound = producer.program_envelope(row)
    assert bound['code_hash'] == NativeProgramDeploy.decode(producer.read(row['payload_file'])).new_hash.hex()
    peer = RecordedChain(case)
    with peer.serving(tmp_path) as endpoints:
        client = producer.ProgramHttpProducer(endpoints, pins(case), SOURCE, REVISION, case.get('network_id', 125))
        with pytest.raises(ValueError, match='actual source receipt required'):
            client.submit_program(row)
    assert peer.accepted == {'0x' + keccak256(raw).hex(): '0x' + raw.hex()}
    sent = [request for role, request in peer.requests if role == 'gateway' and request['method'] == 'eth_sendRawTransaction']
    assert len(sent) == 1 and sent[0]['params'] == ['0x' + raw.hex()]


@pytest.mark.parametrize('change', ['fee', 'signer', 'payload', 'deploy-substitution'])
def test_finding_43_changed_signed_envelope_is_refused_before_transport(tmp_path, change):
    producer = component('program-http-producer')
    row = call_row(producer, tmp_path, recording())
    if change == 'fee':
        row['fee_limit'] += 1
    elif change == 'signer':
        row['signer'] = '0x' + '22' * 20
    elif change == 'payload':
        payload = Path(row['payload_file'])
        payload.write_bytes(payload.read_bytes()[:-1] + b'\1')
    else:
        deploy = deployment_row(producer, tmp_path)
        row.update(operation='deploy', payload_file=deploy['payload_file'], expected_terminal_result=None)
    with pytest.raises(ValueError):
        producer.program_envelope(row)


def test_finding_43_receipted_program_never_claims_committed_qualification(tmp_path):
    producer = component('program-http-producer')
    case = recording()
    peer = RecordedChain(case)
    with peer.serving(tmp_path) as endpoints:
        manifest = source_manifest(producer, tmp_path, case, endpoints)
        assert producer.main(['--manifest', manifest, '--output', str(tmp_path / 'output'), '--revision', REVISION]) == 78
    result = producer.load(tmp_path / 'output/result.json')
    assert result['passed'] is False
    assert result['unqualified_criteria'] == ['block-commit']
    assert producer.load(result['authenticated_source'])['records'][0]['receipts']['paxd']['rung'] == 'receipted'


@pytest.mark.parametrize('field,value', [('expected_result_code', 1), ('expected_terminal_result', 1)])
def test_finding_43_signed_receipt_must_prove_the_declared_outcome(tmp_path, field, value):
    producer = component('program-http-producer')
    case = recording()
    row = call_row(producer, tmp_path, case)
    row[field] = value
    with RecordedChain(case).serving(tmp_path) as endpoints:
        client = producer.ProgramHttpProducer(endpoints, pins(case), SOURCE, REVISION)
        with pytest.raises(ValueError, match='differs from expected result'):
            client.submit_program(row)


def test_finding_53_committed_authenticated_source_producer_to_native_receipt_consumer(tmp_path, capsys):
    producer, custody = component('program-http-producer'), component('reference-custody')
    case = committed_recording()
    peer = RecordedChain(case)
    with peer.serving(tmp_path) as endpoints:
        manifest = source_manifest(producer, tmp_path, case, endpoints)
        assert producer.main(['--manifest', manifest, '--output', str(tmp_path / 'output'), '--revision', REVISION]) == 0
    emitted = json.loads(capsys.readouterr().out)
    assert emitted['passed'] is True and emitted['unqualified_criteria'] == []
    bundle = producer.load(emitted['authenticated_source'])
    assert bundle['schema'] == producer.SOURCE_SCHEMA
    record = bundle['records'][0]
    assert record['receipts']['paxd'] == case['receiptJson']
    verified = producer.consume_source(record, producer.decode_receipt_keys(pins(case)), SOURCE, case['network_id'])
    assert verified.receipt.operation == 2
    assert verified.canonical_bytes.hex() == case['signedReceipt'][2:]
    report, artifacts = native_consumption(producer, tmp_path, record)
    authenticated = custody.authenticate_source(bundle, pins(case), report, artifacts, SOURCE, case.get('network_id', 125))
    qualification = custody.authenticated_source_qualification(bundle, pins(case), report, artifacts,
        SOURCE, case['network_id'])
    assert qualification['passed'] is True and qualification['unqualified_criteria'] == []
    assert qualification['authenticated_source'] == authenticated
    assert authenticated == [{'activity_id': case['receiptJson']['txHash'][2:],
        'receipt_digest': verify_sequencer_receipt(producer.rpc_bytes(case['signedReceipt']),
            producer.rpc_bytes(case['rawTransaction']), producer.decode_receipt_keys(pins(case))).receipt_digest.hex(),
        'receipt_sha256': hashlib.sha256(producer.rpc_bytes(case['signedReceipt'])).hexdigest(),
        'height': int(case['receiptJson']['height'], 16), 'rung': 'committed',
        'block_hash': case['receiptJson']['blockHash']}]
    saved_report = producer.write(tmp_path / 'native-consumption.json', report)
    # Stop both HTTP peers and reopen the real consumer using only durable proof material.
    reopened = component('reference-custody')
    assert reopened.authenticate_source(producer.load(emitted['authenticated_source']), pins(case),
        producer.load(saved_report), artifacts, SOURCE, case.get('network_id', 125)) == authenticated
    assert 'authorization' not in json.dumps(bundle).lower()


def test_finding_53_receipted_source_keeps_explicit_unqualified_refusal(tmp_path):
    producer, custody = component('program-http-producer'), component('reference-custody')
    case = recording('V1-layerxledger-pay-exact')
    with RecordedChain(case).serving(tmp_path) as endpoints:
        record = producer.ProgramHttpProducer(endpoints, pins(case), SOURCE, REVISION).observe(
            producer.rpc_bytes(case['rawTransaction']))
    report, artifacts = native_consumption(producer, tmp_path, record)
    bundle = {'schema': producer.SOURCE_SCHEMA, 'source': SOURCE, 'network_id': 125, 'records': [record]}
    qualified = custody.authenticated_source_qualification(bundle, pins(case), report, artifacts, SOURCE)
    assert qualified['passed'] is False
    assert qualified['unqualified_criteria'] == ['authenticated-source-verification']
    assert qualified['authenticated_source'][0]['rung'] == 'receipted'


def test_finding_53_genuine_refused_source_cannot_be_a_successful_condition(tmp_path):
    producer, custody = component('program-http-producer'), component('reference-custody')
    case = recording('V2-layerxledger-revert')
    with RecordedChain(case).serving(tmp_path) as endpoints:
        record = producer.ProgramHttpProducer(endpoints, pins(case), SOURCE, REVISION).observe(
            producer.rpc_bytes(case['rawTransaction']))
    report, artifacts = native_consumption(producer, tmp_path, record)
    bundle = {'schema': producer.SOURCE_SCHEMA, 'source': SOURCE, 'network_id': 125, 'records': [record]}
    with pytest.raises(ValueError, match='successful authenticated source condition'):
        custody.authenticated_source_qualification(bundle, pins(case), report, artifacts, SOURCE)


@pytest.mark.parametrize('change', ['signature', 'raw', 'pin', 'json-status', 'return-data', 'log',
                                  'commit-height', 'commit-hash', 'payment-denom'])
def test_finding_53_authentication_rejects_changed_real_proof_material(tmp_path, change):
    producer, custody = component('program-http-producer'), component('reference-custody')
    case = committed_recording()
    peer = RecordedChain(case)
    with peer.serving(tmp_path) as endpoints:
        client = producer.ProgramHttpProducer(endpoints, pins(case), SOURCE, REVISION, case.get('network_id', 125))
        record = client.observe(producer.rpc_bytes(case['rawTransaction']))
    report, artifacts = native_consumption(producer, tmp_path, record)
    bundle = {'schema': producer.SOURCE_SCHEMA, 'source': SOURCE, 'network_id': case.get('network_id', 125), 'records': [record]}
    selected_pins = pins(case)
    for receipt in record['receipts'].values():
        if change == 'signature':
            raw = producer.rpc_bytes(receipt['receipt'])
            receipt['receipt'] = '0x' + (raw[:-1] + bytes([raw[-1] ^ 1])).hex()
        elif change == 'json-status':
            receipt['status'] = '0x0'
        elif change == 'return-data':
            receipt['returnData'] = '0x01'
        elif change == 'log':
            receipt['logs'][0]['data'] = '0x01'
        elif change == 'commit-height':
            receipt['blockNumber'] = '0x9'
        elif change == 'commit-hash':
            receipt['blockHash'] = '0x' + '00' * 32
        elif change == 'payment-denom':
            receipt['paymentDenom'] = 'foreign'
    if change == 'raw':
        record['raw_transaction'] = recording()['rawTransaction']
    if change == 'pin':
        selected_pins['keys'][0]['publicKey'] = '0x' + '22' * 32
    with pytest.raises((ValueError, PlatformSdkError)):
        custody.authenticate_source(bundle, selected_pins, report, artifacts, SOURCE, case.get('network_id', 125))


@pytest.mark.parametrize('change', ['missing-consumption', 'wrong-receipt', 'changed-artifact',
                                  'duplicate-source', 'unconsumed-source', 'revision'])
def test_finding_53_native_condition_binding_is_required(tmp_path, change):
    producer, custody = component('program-http-producer'), component('reference-custody')
    case = committed_recording()
    with RecordedChain(case).serving(tmp_path) as endpoints:
        record = producer.ProgramHttpProducer(endpoints, pins(case), SOURCE, REVISION, case.get('network_id', 125)).observe(
            producer.rpc_bytes(case['rawTransaction']))
    bundle = {'schema': producer.SOURCE_SCHEMA, 'source': SOURCE, 'network_id': case.get('network_id', 125), 'records': [record]}
    report, artifacts = native_consumption(producer, tmp_path, record)
    if change == 'missing-consumption':
        del report['authenticated_source']
    elif change == 'wrong-receipt':
        report['authenticated_source']['receipts'][0]['receipt_sha256'] = '22' * 32
    elif change == 'changed-artifact':
        Path(artifacts[0]['path']).write_bytes(b'changed-source-receipt')
    elif change == 'duplicate-source':
        bundle['records'].append(deepcopy(record))
    elif change == 'unconsumed-source':
        report['authenticated_source']['receipts'].append({'activity_id': '11' * 32, 'receipt_sha256': '22' * 32})
    else:
        bundle['source'] = {**SOURCE, 'revision': '22' * 20}
    with pytest.raises(ValueError):
        custody.authenticate_source(bundle, pins(case), report, artifacts, SOURCE, case.get('network_id', 125))


def test_finding_53_authentication_alone_cannot_qualify_unverified_native_custody(tmp_path):
    producer, custody = component('program-http-producer'), component('reference-custody')
    case = committed_recording()
    with RecordedChain(case).serving(tmp_path) as endpoints:
        record = producer.ProgramHttpProducer(endpoints, pins(case), SOURCE, REVISION, case.get('network_id', 125)).observe(
            producer.rpc_bytes(case['rawTransaction']))
    bundle = {'schema': producer.SOURCE_SCHEMA, 'source': SOURCE, 'network_id': case.get('network_id', 125), 'records': [record]}
    report, artifacts = native_consumption(producer, tmp_path, record)
    result = {'source': SOURCE, 'native_passed': False, 'passed': False,
              'unqualified_criteria': list(custody.UNQUALIFIED)}
    assert custody.authenticate_source(bundle, pins(case), report, artifacts, SOURCE, case.get('network_id', 125))[0]['rung'] == 'committed'
    with pytest.raises(ValueError, match='complete native custody verification required'):
        custody.complete_authenticated_source(result, bundle, pins(case), report, artifacts, SOURCE, case.get('network_id', 125))
    assert result['passed'] is False
    assert result['unqualified_criteria'] == ['authenticated-source-verification']


@pytest.mark.parametrize('change', ['rpc-id', 'receipt', 'receipt-keys', 'network'])
def test_finding_43_gateway_and_paxd_must_agree(tmp_path, change):
    producer = component('program-http-producer')
    case = recording()
    peer = RecordedChain(case)

    def changed(request, reply):
        if change == 'rpc-id':
            reply['id'] += 1
        elif change == 'receipt' and request['method'] == 'pax_sequencerReceipt':
            reply['result']['seq'] = '0xffff'
        elif change == 'receipt-keys' and request['method'] == 'px_getReceiptKeys':
            reply['result']['keys'][0]['publicKey'] = '0x' + '22' * 32
        elif change == 'network' and request['method'] == 'eth_chainId':
            reply['result'] = '0x7e'

    peer.gateway_change = changed
    with peer.serving(tmp_path) as endpoints:
        client = producer.ProgramHttpProducer(endpoints, pins(case), SOURCE, REVISION, case.get('network_id', 125))
        with pytest.raises((ValueError, PlatformSdkError)):
            client.observe(producer.rpc_bytes(case['rawTransaction']))


def test_finding_43_http_authentication_is_not_bypassed(tmp_path):
    producer = component('program-http-producer')
    case = recording()
    peer = RecordedChain(case)
    peer.gateway_authorization = 'Bearer different-recording-session'
    with peer.serving(tmp_path) as endpoints:
        client = producer.ProgramHttpProducer(endpoints, pins(case), SOURCE, REVISION, case.get('network_id', 125))
        with pytest.raises(urllib.error.HTTPError) as caught:
            client.observe(producer.rpc_bytes(case['rawTransaction']))
        assert caught.value.code == 401
    assert not peer.accepted


def test_retired_inventory_and_source_revision_are_refused(tmp_path):
    producer = component('program-http-producer')
    case = recording()
    with RecordedChain(case).serving(tmp_path) as endpoints:
        with pytest.raises(ValueError, match='independently selected revision'):
            producer.ProgramHttpProducer(endpoints, pins(case), SOURCE, '22' * 20)
        with pytest.raises(ValueError, match='only paxd and hosted gateway'):
            producer.ProgramHttpProducer({**endpoints, 'layerxd': endpoints['paxd']}, pins(case), SOURCE, REVISION)
        with pytest.raises(ValueError, match='distinct paxd and gateway'):
            producer.ProgramHttpProducer({'paxd': endpoints['paxd'], 'gateway': endpoints['paxd']}, pins(case), SOURCE, REVISION)
        for retired in ('/rpc', '/v1/programs/call'):
            with pytest.raises(ValueError, match='supported root JSON-RPC'):
                producer.ProducerRpc(endpoints['gateway']['endpoint'].rstrip('/') + retired, {})


def test_preexecution_refusal_retains_authenticated_kernel_outcome(tmp_path):
    producer = component('program-http-producer')
    case = recording('V2-pax-transfer-insufficient-funds')
    with RecordedChain(case).serving(tmp_path) as endpoints:
        record = producer.ProgramHttpProducer(endpoints, pins(case), SOURCE, REVISION).observe(
            producer.rpc_bytes(case['rawTransaction']))
    for receipt in record['receipts'].values():
        assert receipt == case['receiptJson']
        assert receipt['status'] == '0x0'
        assert receipt['returnData'] == '0x'
        assert receipt['logs'] == []
        assert receipt['vmError'] == 'insufficient balance for transfer'
        assert 'ledgerResultCode' not in receipt and 'lxpCode' not in receipt
    verified = producer.verified_receipt(case['receiptJson'],
        producer.rpc_bytes(case['rawTransaction']), producer.decode_receipt_keys(pins(case)))
    assert verified.canonical_bytes.hex() == case['signedReceipt'][2:]
    assert verified.receipt_digest.hex() == case['receiptDigest'][2:]
    assert verified.receipt.result_code == 1
    assert verified.receipt.amount > verified.receipt.from_balance_before > 0
    assert verified.receipt.from_balance_before == verified.receipt.from_balance_after
