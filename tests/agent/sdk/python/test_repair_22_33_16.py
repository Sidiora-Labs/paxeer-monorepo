from __future__ import annotations

import copy
import errno
import importlib.util
import json
import os
import struct
import threading
from contextlib import contextmanager
from dataclasses import asdict, fields, replace
from decimal import Decimal
from fractions import Fraction
from hashlib import sha256
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.request import Request, urlopen

import pytest
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec, utils

import layerx_sdk as sdk
from layerx_sdk import ai_market as market
from layerx_sdk import exchange, perps, spot
from layerx_sdk.account_derivation import _N, _recover, keccak256
from layerx_sdk.agent_http import (
    AgentEnvelopeTransport, AgentHttpTransport, AgentSessionCredential,
    LayerXKeyCredential,
)
from layerx_sdk.evm_transaction import Type2Transaction, decode_call, decode_signed, encode_call
from layerx_sdk.intent import (
    HumanIntentClient, IntentConstraints, IntentEndpointRef, IntentLeg, IntentLegBinding,
    IntentMoney, IntentPlan, IntentSigningRequirement, PlanIntentRequest, SubmitPlanRequest,
)
from layerx_sdk.native_program_call import NativeProgramCall, encode_native_program_call
from layerx_sdk.production import IdempotencyKey, PlatformSdkError, ProductionClient, ProtocolAmount, SdkErrorCode, SecretBytes
from layerx_sdk.program_lifecycle import (
    DEPLOY_ENVELOPE_BYTES, MAX_DEPLOY_PAYLOAD_BYTES, NativeProgramDeploy,
    NativeProgramLifecycleRequest, NativeProgramUpgrade, NativeProgramWindDown,
    decode_lifecycle_calldata,
)
from layerx_sdk.programs import NativeProgramRequest, ProgramOperations, ProgramTrustContext, _wire
from layerx_sdk.x402_rpc import HostedGatewayRpc, PaymentRpc

ROOT = Path(__file__).resolve().parents[4]
VECTORS = json.loads((ROOT / 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json').read_text())
PROGRAM_VECTOR = next(case for case in VECTORS['cases'] if case['id'] == 'V1-layerxprograms-call')
INTENT_VECTOR = json.loads((ROOT / 'sdk/platform/conformance/fixtures/intent-plan-v1.json').read_text())
WASM = b'\0asm\x01\0\0\0'
PROGRAM = b'\x11' * 32
GATEWAY_SECRET = 'lxp_live_' + 'ab' * 32
AUTHORIZATION = 'LayerX-Key repair:' + GATEWAY_SECRET


def raw(value):
    return bytes.fromhex(value.removeprefix('0x'))


def pin(vector=PROGRAM_VECTOR):
    value = vector['pinnedKey']
    return sdk.ReceiptKey(raw(value['keyId']), raw(value['publicKey']), value['firstHeight'], value['lastHeight'])


def credential():
    return LayerXKeyCredential('repair', SecretBytes(GATEWAY_SECRET.encode()))


def program_client(endpoint):
    key = pin()
    return ProgramOperations(
        ProductionClient(AgentHttpTransport(endpoint, credential=credential())),
        signatures(),
        ProgramTrustContext(key.public_key, protocol_version=3, network_id=125, receipt_keys=(key,)),
    )


def signatures():
    spec = importlib.util.spec_from_file_location(
        'repair_22_33_16_signatures', ROOT / 'sdk/integrations/fastapi/layerx_fastapi/signatures.py'
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module.LayerXSignatureVerifier()


def sign_transaction(calldata, *, chain=125, target='101d', nonce=7):
    key = ec.derive_private_key(42, ec.SECP256K1())
    public = key.public_key().public_bytes(serialization.Encoding.X962, serialization.PublicFormat.UncompressedPoint)[1:]
    address = '0x' + keccak256(public)[-20:].hex()
    transaction = Type2Transaction(chain, nonce, 1, 2, 2_000_000, bytes.fromhex(target.rjust(40, '0')), 0, calldata)
    digest = transaction.signing_digest()
    r, s = utils.decode_dss_signature(key.sign(digest, ec.ECDSA(utils.Prehashed(hashes.SHA256()))))
    s = min(s, _N - s)
    parity = next(value for value in (0, 1) if _recover(digest, r, s, value) == public)
    signature = r.to_bytes(32, 'big') + s.to_bytes(32, 'big') + bytes([27 + parity])
    canonical = transaction.attach_signature(signature, address)
    return canonical, address


@contextmanager
def server(dispatch):
    failures = []

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            self.exchange()

        def do_GET(self):
            self.exchange()

        def exchange(self):
            try:
                body = self.rfile.read(int(self.headers.get('Content-Length', '0')))
                status, value = dispatch(self.command, self.path, self.headers, body)
                encoded = json.dumps(value, allow_nan=False, separators=(',', ':')).encode()
                self.send_response(status)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(encoded)))
                self.end_headers()
                self.wfile.write(encoded)
            except Exception as failure:
                failures.append(failure)
                self.send_error(500)

        def log_message(self, *args):
            pass

    http = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
    worker = threading.Thread(target=http.serve_forever, daemon=True)
    worker.start()
    try:
        yield f'http://127.0.0.1:{http.server_port}'
    finally:
        http.shutdown()
        http.server_close()
        worker.join(timeout=5)
        assert not worker.is_alive()
        assert not failures, repr(failures)


class ProgramNode:
    def __init__(self):
        self.requests = []
        self.transactions = {}
        self.receipts = {}
        self.mode = 'normal'
        self.decoded = []

    def dispatch(self, method, path, headers, body):
        assert method == 'POST' and path == '/'
        assert headers['Content-Type'] == 'application/json'
        assert headers['Authorization'] == AUTHORIZATION
        request = json.loads(body)
        assert set(request) == {'jsonrpc', 'id', 'method', 'params'}
        assert request['jsonrpc'] == '2.0' and type(request['id']) is int
        self.requests.append(request)
        operation, params = request['method'], request['params']
        if operation == 'eth_sendRawTransaction':
            assert len(params) == 1 and params[0].startswith('0x02')
            canonical = raw(params[0])
            transaction, signature = decode_signed(canonical)
            assert transaction.chain_id == 125 and transaction.to == bytes.fromhex('000000000000000000000000000000000000101d')
            assert transaction.recover_signer(signature)
            identity = keccak256(canonical).hex()
            assert headers['Idempotency-Key'] in (None, identity)
            if transaction.data[:4] == market._PROGRAMS_CALL_SELECTOR:
                decoded = market._decode_programs_call(transaction.data)
            else:
                ordinal = next(ordinal for ordinal, name in ((1, 'deploy'), (2, 'upgrade'), (7, 'deprecate'), (7, 'tombstone'), (7, 'setExitRoute'), (7, 'exitValue'))
                               if transaction.data[:4] == LIFECYCLE_SELECTORS[name])
                decoded = decode_lifecycle_calldata(transaction.data, ordinal)
            self.decoded.append(decoded)
            self.transactions.setdefault(identity, canonical)
            assert self.transactions[identity] == canonical
            if self.mode == 'lost-response':
                self.mode = 'normal'
                return 502, {'upstream': 'response lost after mempool admission'}
            if self.mode == 'already-known':
                return 200, {'jsonrpc': '2.0', 'id': request['id'],
                             'error': {'code': -32000, 'message': 'already known'}}
            result = '0x' + ('ff' * 32 if self.mode == 'wrong-hash' else identity)
        else:
            assert operation == 'pax_sequencerReceipt' and len(params) == 1
            identity = params[0][2:]
            assert params[0] == '0x' + identity and len(identity) == 64
            result = self.receipts.get(identity)
            if result is None and identity in self.transactions:
                transaction, _ = decode_signed(self.transactions[identity])
                result = {'txHash': params[0], 'nonce': hex(transaction.nonce), 'rung': 'submitted'}
        response = {'jsonrpc': '2.0', 'id': request['id'], 'result': result}
        if self.mode == 'wrong-id':
            response['id'] += 1
        return 200, response


LIFECYCLE_TYPES = {
    'deploy': ('bytes32', 'uint16', 'uint8', 'bytes32', 'bytes32', 'bytes', 'bytes'),
    'upgrade': ('bytes32', 'uint16', 'uint8', 'bytes32', 'bytes32', 'bytes', 'bytes', 'bytes', 'uint128'),
    'deprecate': ('bytes32', 'bytes32', 'uint64'),
    'tombstone': ('bytes32',),
    'exitValue': ('bytes32', 'bytes32'),
    'setExitRoute': ('bytes32', 'bytes32', 'bytes32', 'bytes32', 'bytes'),
}
LIFECYCLE_SELECTORS = {name: keccak256(f"{name}({','.join(types)})".encode())[:4] for name, types in LIFECYCLE_TYPES.items()}


def lifecycle_cases():
    digest = sha256(WASM).digest()
    return [
        (1, NativeProgramDeploy(PROGRAM, 5, bytes(32), digest, WASM), 'deploy', (PROGRAM, 5, 0, bytes(32), digest, b'', WASM)),
        (2, NativeProgramUpgrade(PROGRAM, 5, b'\x22' * 32, digest, WASM), 'upgrade', (PROGRAM, 5, 0, b'\x22' * 32, digest, b'', b'', WASM, 100)),
        (7, NativeProgramWindDown(PROGRAM, 2, exit_program=b'\x33' * 32, deadline_batch=9), 'deprecate', (PROGRAM, b'\x33' * 32, 9)),
        (7, NativeProgramWindDown(PROGRAM, 3), 'tombstone', (PROGRAM,)),
        (7, NativeProgramWindDown(PROGRAM, 4, account=b'\x44' * 32), 'exitValue', (PROGRAM, b'\x44' * 32)),
        (7, NativeProgramWindDown(PROGRAM, 1, account=b'\x44' * 32, asset=b'\x55' * 32, destination=b'\x66' * 32, seed=b'seed'),
         'setExitRoute', (PROGRAM, b'\x44' * 32, b'\x55' * 32, b'\x66' * 32, b'seed')),
    ]


def test_retired_public_exports_preserve_recorded_receipt_path():
    for name in ('AuthorizedReceiptBatch', 'MerkleProof', 'PaymentCommitmentEvidence', 'rpc_batch_evidence'):
        assert name not in sdk.__all__ and not hasattr(sdk, name)
    accepted = [case for case in VECTORS['cases'] if case['vector'] == 'V1']
    by_hash = {case['receiptJson']['txHash']: case for case in accepted}
    requests = []

    def dispatch(method, path, headers, body):
        assert method == 'POST'
        assert headers['Content-Type'] == 'application/json'
        request = json.loads(body)
        requests.append(request)
        if request['method'] == 'px_getReceiptKeys':
            assert path == '/rpc'
            result = {'keys': [accepted[0]['pinnedKey']]}
        else:
            assert request['method'] == 'pax_sequencerReceipt' and path == '/'
            result = by_hash[request['params'][0]]['receiptJson']
        return 200, {'jsonrpc': '2.0', 'id': request['id'], 'result': result}

    with server(dispatch) as endpoint:
        rpc = PaymentRpc(endpoint)
        keys = HostedGatewayRpc(endpoint + '/rpc').receipt_keys()
        for vector in accepted:
            receipt = rpc.receipt(vector['receiptJson']['txHash'])
            verified = sdk.verify_sequencer_receipt(raw(receipt['receipt']), raw(vector['rawTransaction']), keys)
            assert isinstance(verified, sdk.SequencerReceiptVerification)
            assert verified.receipt.activity_id == raw(receipt['txHash'])
            assert verified.height == int(receipt['height'], 16)
            assert verified.receipt_digest == raw(vector['receiptDigest'])
            with pytest.raises(sdk.SequencerReceiptVerificationError):
                sdk.verify_sequencer_receipt(raw(receipt['receipt']), raw(vector['rawTransaction']), ())
    assert len(requests) == 1 + len(accepted)


@pytest.mark.parametrize('ordinal,value,name,values', lifecycle_cases(), ids=lambda value: value if isinstance(value, str) else None)
def test_retired_program_routes_use_signed_evm_and_hash_recovery(tmp_path, ordinal, value, name, values):
    calldata = encode_call(name, LIFECYCLE_TYPES[name], values)
    canonical, _ = sign_transaction(calldata)
    request = NativeProgramLifecycleRequest(ordinal, value.encode(), canonical)
    identity = keccak256(canonical).hex()
    node = ProgramNode()
    with server(node.dispatch) as endpoint:
        client = program_client(endpoint)
        operation = {1: client.deploy, 2: client.upgrade, 7: client.wind_down}[ordinal]
        pending = operation(request, IdempotencyKey(identity))
        assert pending == {'state': 'pending', 'activity_id': identity, 'idempotency_key': identity, 'retained_signed_activity': canonical.hex()}
        retained_path = tmp_path / 'signed-transaction'
        retained_path.write_bytes(raw(pending['retained_signed_activity']))
        del client, operation, request
        restored = NativeProgramLifecycleRequest(ordinal, value.encode(), retained_path.read_bytes())
        client = program_client(endpoint)
        result = client._client.agent('program.receipt', {'idempotency_key': identity, 'expected_activity_id': identity,
                                      'requested_verification_level': 'sequencer-signed'})
        assert result == {'state': 'pending', 'activity_id': identity, 'idempotency_key': identity}
        operation = {1: client.deploy, 2: client.upgrade, 7: client.wind_down}[ordinal]
        assert operation(restored, IdempotencyKey(identity)) == pending
        assert node.transactions == {identity: canonical}
        assert node.decoded == [value, value]
        assert [row['method'] for row in node.requests] == ['eth_sendRawTransaction', 'pax_sequencerReceipt', 'eth_sendRawTransaction']


@pytest.mark.parametrize('mode', ['lost-response', 'wrong-hash', 'wrong-id', 'already-known'])
def test_program_unknown_outcome_retains_signed_transaction_across_reopen(tmp_path, mode):
    ordinal, value, name, values = lifecycle_cases()[0]
    canonical, _ = sign_transaction(encode_call(name, LIFECYCLE_TYPES[name], values))
    identity = keccak256(canonical).hex()
    request = NativeProgramLifecycleRequest(ordinal, value.encode(), canonical)
    node = ProgramNode()
    node.mode = mode
    with server(node.dispatch) as endpoint:
        client = program_client(endpoint)
        result = client.deploy(request, IdempotencyKey(identity))
        assert result == {'state': 'unknown', 'activity_id': identity, 'idempotency_key': identity, 'retained_signed_activity': canonical.hex()}
        path = tmp_path / 'retained'
        path.write_bytes(raw(result['retained_signed_activity']))
        del client
        node.mode = 'normal'
        reopened = program_client(endpoint)
        restored = NativeProgramLifecycleRequest(ordinal, value.encode(), path.read_bytes())
        assert reopened.deploy(restored, IdempotencyKey(identity))['state'] == 'pending'
        assert node.transactions == {identity: canonical}
        assert len(node.requests) == 2


def test_program_call_and_receipt_use_recorded_node_producer_after_reopen(tmp_path):
    canonical = raw(PROGRAM_VECTOR['rawTransaction'])
    transaction, _ = decode_signed(canonical)
    native, fee = market._decode_programs_call(transaction.data)
    request = NativeProgramRequest(native, fee, canonical)
    identity = PROGRAM_VECTOR['receiptJson']['txHash'][2:]
    node = ProgramNode()
    with server(node.dispatch) as endpoint:
        client = program_client(endpoint)
        pending = client.submit(request, IdempotencyKey(identity))
        assert pending['state'] == 'pending' and pending['retained_signed_activity'] == canonical.hex()
        retained = tmp_path / 'call'
        retained.write_bytes(raw(pending['retained_signed_activity']))
        assert client.receipt(identity, identity)['state'] == 'pending'
        del client
        node.receipts[identity] = copy.deepcopy(PROGRAM_VECTOR['receiptJson'])
        reopened = program_client(endpoint)
        restored = NativeProgramRequest(native, fee, retained.read_bytes())
        assert reopened.submit(restored, IdempotencyKey(identity))['state'] == 'pending'
        result = reopened.receipt(identity, identity)
        assert result == PROGRAM_VECTOR['receiptJson']
        assert result['rung'] == 'receipted' and len(node.transactions) == 1
        node.receipts[identity]['height'] = hex(int(result['height'], 16) + 1)
        with pytest.raises(ValueError, match='receipt binding'):
            reopened.receipt(identity, identity)


def test_program_mutations_refuse_bad_binding_before_transport():
    node = ProgramNode()
    ordinal, value, name, values = lifecycle_cases()[0]
    calldata = encode_call(name, LIFECYCLE_TYPES[name], values)
    with server(node.dispatch) as endpoint:
        transport = AgentHttpTransport(endpoint, credential=credential())
        for changed in ({'chain': 126}, {'target': '101a'}):
            canonical, _ = sign_transaction(calldata, **changed)
            with pytest.raises(PlatformSdkError) as failure:
                transport.call('agent', 'program.deploy', {'signed_activity': canonical.hex()}, IdempotencyKey(keccak256(canonical).hex()))
            assert failure.value.code is SdkErrorCode.INVALID_ARGUMENT
        canonical, _ = sign_transaction(calldata)
        with pytest.raises(PlatformSdkError):
            transport.call('agent', 'program.deploy', {'signed_activity': canonical.hex()}, IdempotencyKey('99' * 32))
        call_raw = raw(PROGRAM_VECTOR['rawTransaction'])
        tx, _ = decode_signed(call_raw)
        native, fee = market._decode_programs_call(tx.data)
        wire = dict(_wire(NativeProgramRequest(native, fee, call_raw)))
        wire['calldata'] = '00'
        with pytest.raises(PlatformSdkError):
            transport.call('agent', 'program.call', wire, IdempotencyKey(keccak256(call_raw).hex()))
        with pytest.raises(PlatformSdkError):
            transport.call('agent', 'program.receipt', {'idempotency_key': '11' * 32, 'expected_activity_id': '22' * 32,
                           'requested_verification_level': 'sequencer-signed'}, None)
    assert node.requests == []


def test_program_preparation_keeps_session_authenticated_envelope():
    session = AgentSessionCredential('tenant', '11' * 32, '22' * 32, 3)
    observed = []
    ready = {'transport_ready': True, 'verified_reads_ready': True, 'writes_admitted': True, 'recovery_reason': None}
    canonical = raw(PROGRAM_VECTOR['rawTransaction'])
    transaction, signature = decode_signed(canonical)
    prepared = Type2Transaction.decode(transaction.encode())
    assert prepared == transaction
    assert prepared.attach_signature(signature, transaction.recover_signer(signature)) == canonical

    def dispatch(method, path, headers, body):
        assert method == 'POST' and path == '/v1/agent/rpc'
        assert headers['Authorization'] == AUTHORIZATION and headers['Content-Type'] == 'application/json'
        request = json.loads(body)
        assert request['credential'] == session.coordinates()
        assert request['operation'] == 'tenant.readiness' and request['idempotency_key'] is None
        assert request['request'] == {}
        observed.append(request)
        return 200, {'request_id': request['request_id'], 'value': ready,
                     'verification_status': {'state': 'achieved', 'rung': 'submitted'}}

    with server(dispatch) as endpoint:
        transport = AgentEnvelopeTransport(endpoint, gateway_key=credential(), session=session)
        result = transport.tenant_readiness()
        assert result.value == ready and len(observed) == 1


def signed_market_record():
    key = ec.derive_private_key(42, ec.SECP256K1())
    public = key.public_key().public_bytes(serialization.Encoding.X962, serialization.PublicFormat.CompressedPoint)
    envelope = market.PaxaiEnvelope(
        market.FUND, b'\x01' * 32, b'\x02' * 32, b'\x03' * 32, b'\x04' * 32,
        0, 1, None, 1, 1300, b'\x05' * 32,
        (100).to_bytes(16, 'big') + b'\x06' * 32 + (1).to_bytes(8, 'big') + b'\x01',
    )
    native = NativeProgramCall(envelope.program, market.GUEST_ABI, market.ENTRYPOINT, envelope.encode(),
                               b'', b'', 1024, (1000,) * 7)
    canonical, _ = sign_transaction(market._encode_programs_call(native, 100))
    return market.OperationRecord.signed(canonical, sha256(public).digest(), envelope.request_digest(), signatures())


@pytest.mark.parametrize('fault', ['short-success', 'interrupted', 'partial-error', 'zero-progress'])
def test_journal_short_write_restarts_with_intact_record(tmp_path, monkeypatch, fault):
    record = signed_market_record()
    journal = market.OperationJournal(tmp_path, signatures())
    journal.record_signed(record)
    original = journal.record_path(record.activity_id).read_bytes()
    updated = replace(record, state=market.SdkState.PENDING, attempt=1, rung=market.Rung.SUBMITTED)
    complete = updated.encode()
    write = os.write
    calls = []

    def storage_write(descriptor, data):
        calls.append(len(data))
        if len(calls) == 1:
            if fault == 'interrupted':
                raise InterruptedError(errno.EINTR, 'interrupted before progress')
            return write(descriptor, data[:17])
        if fault == 'partial-error':
            raise OSError(errno.ENOSPC, 'full after actual partial write')
        if fault == 'zero-progress':
            return 0
        return write(descriptor, data[:29])

    with monkeypatch.context() as injection:
        injection.setattr(os, 'write', storage_write)
        if fault in ('partial-error', 'zero-progress'):
            with pytest.raises(OSError):
                journal.persist(updated)
            assert journal.record_path(record.activity_id).read_bytes() == original
            assert journal.record_path(record.activity_id).with_suffix('.partial').read_bytes() == complete[:17]
        else:
            journal.persist(updated)
            assert journal.record_path(record.activity_id).read_bytes() == complete
    assert len(calls) > 1
    del journal
    reopened = market.OperationJournal(tmp_path, signatures())
    expected = record if fault in ('partial-error', 'zero-progress') else updated
    recovered = reopened.load(record.activity_id)
    assert recovered == expected and recovered.signed_bytes == record.signed_bytes
    assert market.OperationRecord.decode(recovered.encode(), signatures()) == recovered

    class MarketNodeConnection:
        def __init__(self, endpoint):
            self.transport = AgentHttpTransport(endpoint, credential=credential())

        def request(self, method, params):
            return self.transport._node_rpc(method, params, 'program.call')

    node = ProgramNode()
    with server(node.dispatch) as endpoint:
        if expected.state is market.SdkState.SIGNED:
            assert reopened.submit(recovered, MarketNodeConnection(endpoint)) is market.SdkState.PENDING
            assert node.transactions == {record.activity_id.hex(): record.signed_bytes}
        else:
            assert recovered.rung is market.Rung.SUBMITTED and recovered.attempt == 1
    assert market.OperationJournal(tmp_path, signatures()).load(record.activity_id).state is market.SdkState.PENDING


def trading_market(halted=False):
    return perps.PerpsMarketCreate(
        '01' * 32, '02' * 32, '03' * 32, '04' * 32, '05' * 32, '06' * 32, '07' * 32,
        1, 1, 1, 1, 2000, 1000, 100, 100, 10, 10, 1000, 1000, 1, 100, ('08' * 32,), 1, halted,
    )


def activity_exchange(endpoint, activity):
    if activity.MODULE == perps.PERPS_MODULE_ID:
        activity_type, payload = perps.build_perps_activity(activity)
    else:
        activity_type, payload = spot.build_spot_activity(activity)
    request = Request(endpoint + '/activities', data=struct.pack('>I', activity_type) + payload,
                      headers={'Content-Type': 'application/octet-stream'}, method='POST')
    with urlopen(request, timeout=5) as response:
        return json.load(response)


def activity_peer(observed):
    def dispatch(method, path, headers, body):
        assert method == 'POST' and path == '/activities' and headers['Content-Type'] == 'application/octet-stream'
        activity_type = int.from_bytes(body[:4], 'big')
        decoder = perps.decode_perps_activity if activity_type >> 16 == perps.PERPS_MODULE_ID else spot.decode_spot_activity
        value = decoder(activity_type, body[4:])
        assert value.encode() == body[4:]
        observed.append(value)
        return 200, {'activity_type': activity_type, 'payload': value.encode().hex(),
                     'flags': {field.name: getattr(value, field.name) for field in fields(value) if type(getattr(value, field.name)) is bool}}
    return dispatch


@pytest.mark.parametrize('whitespace', [' ', '\t', '\n', '\r', '\v', '\f'])
def test_trading_id_whitespace_refused_in_perps_and_spot_before_send(whitespace):
    bad = '11' * 15 + whitespace * 2 + '11' * 16
    assert len(bad) == 64 and len(bytes.fromhex(bad)) == 31
    templates = [
        perps.PerpsOrderPlace('11' * 32, '22' * 32, '33' * 32, 1, 7, 9),
        perps.PerpsPositionOpen('11' * 32, '22' * 32, '33' * 32, 1, 7, 0, 9),
        spot.SpotOrderPlace('11' * 32, '22' * 32, '33' * 32, '44' * 32, 1, 1, 1, 7, 9),
        spot.SpotMarketCreate('11' * 32, '22' * 32, '33' * 32, 1, 1, '44' * 32),
    ]
    observed = []
    with server(activity_peer(observed)) as endpoint:
        for template in templates:
            count = len(observed)
            for field, layout in zip(fields(template), template.LAYOUT):
                if layout == 'id':
                    with pytest.raises(perps.TradingPayloadError) as failure:
                        activity_exchange(endpoint, replace(template, **{field.name: bad}))
                    assert failure.value.code == 'non_canonical'
            assert len(observed) == count
            result = activity_exchange(endpoint, template)
            assert result['payload'] == template.encode().hex() and observed[-1] == template
        for value in (perps.PerpsAdl('11' * 32, (bad,)), replace(trading_market(), permitted_oracle_keys=(bad,))):
            with pytest.raises(perps.TradingPayloadError):
                activity_exchange(endpoint, value)
    assert len(observed) == len(templates)
    with pytest.raises(perps.TradingPayloadError):
        perps.Writer().id(bad, nonzero=False)
    assert perps.Writer().id('00' * 32, nonzero=False).out == bytes(32)


@pytest.mark.parametrize('invalid', [0, 1, -1, '', 'false', 'true', None, [], {}, 0.0, 1.0])
def test_trading_flags_refuse_non_booleans_without_transport(invalid):
    flag_layouts = {kind for kind in perps.PERPS_ACTIVITIES.values()
                    if 'flag' in kind.LAYOUT or 'halted' in kind.__annotations__}
    assert flag_layouts == {perps.PerpsMarketCreate, perps.PerpsMarketHalt}
    observed = []
    with server(activity_peer(observed)) as endpoint:
        for activity in (perps.PerpsMarketHalt('11' * 32, invalid), trading_market(invalid)):
            with pytest.raises(perps.TradingPayloadError) as failure:
                activity_exchange(endpoint, activity)
            assert failure.value.code == 'non_canonical'
        for flag in (False, True):
            for activity in (perps.PerpsMarketHalt('11' * 32, flag), trading_market(flag)):
                result = activity_exchange(endpoint, activity)
                assert result['flags']['halted'] is flag
                assert observed[-1] == activity and type(observed[-1].halted) is bool
    assert len(observed) == 4


def test_legacy_exchange_writes_are_historical_and_current_calls_use_perps():
    names = ('place_order', 'cancel_order', 'request_settlement', 'deposit_margin', 'deposit_margin_token', 'withdraw_margin')
    for name in names:
        assert not hasattr(exchange, f'exchange_{name}_call')
        assert not hasattr(exchange, f'send_exchange_{name}')
        assert callable(getattr(exchange, f'pre_v6120_exchange_{name}_call'))
    historical = exchange.pre_v6120_exchange_request_settlement_call('0x' + '11' * 32)
    assert historical.data[:10] == exchange.abi_selector('requestSettlement(bytes32)')
    current = [
        (perps.perps_open_position_call(9, '0x' + '22' * 20, 100, 2, True), 'openPosition', ('uint256', 'address', 'uint256', 'uint256', 'bool'), (9, bytes.fromhex('22' * 20), 100, 2, True)),
        (perps.perps_cancel_order_call(7), 'cancelOrder', ('uint256',), (7,)),
        (perps.perps_close_position_call(5), 'closePosition', ('uint256',), (5,)),
        (perps.perps_call('placeLimitOrder(uint256,bool,uint256,uint256,uint256,address,uint256)', [9, False, 10, 100, 2, '0x' + '22' * 20, 11]),
         'placeLimitOrder', ('uint256', 'bool', 'uint256', 'uint256', 'uint256', 'address', 'uint256'), (9, False, 10, 100, 2, bytes.fromhex('22' * 20), 11)),
    ]
    observed = []

    def dispatch(method, path, headers, body):
        assert method == 'POST' and path == '/' and headers['Content-Type'] == 'application/json'
        request = json.loads(body)
        assert request['method'] == 'eth_sendTransaction'
        wallet_request, = request['params']
        assert set(wallet_request) == {'from', 'to', 'data', 'value'}
        index = len(observed)
        call, name, types, values = current[index]
        assert wallet_request['to'] == perps.PERPS_PRECOMPILE and wallet_request['value'] == '0x0'
        assert wallet_request['data'] == call.data
        assert decode_call(name, types, raw(wallet_request['data'])) == values
        observed.append(wallet_request)
        return 200, {'jsonrpc': '2.0', 'id': request['id'], 'result': '0x' + f'{index + 1:064x}'}

    with server(dispatch) as endpoint:
        rpc = PaymentRpc(endpoint)
        for index, (call, *_) in enumerate(current):
            result = exchange.send_precompile_call(rpc.call, '0x' + '33' * 20, call)
            assert result == '0x' + f'{index + 1:064x}'
    assert len(observed) == len(current)
    with pytest.raises(exchange.PrecompileAbiError):
        perps.perps_call('requestSettlement(bytes32)', ['0x' + '11' * 32])


def intent_requests(amount):
    request = PlanIntentRequest(
        IntentEndpointRef('paxeer-wallet', None), IntentEndpointRef('agent', 'agent:did:layerx:bob:main'),
        '99' * 32, IntentMoney(amount, 'LXP'),
        IntentConstraints('2026-08-18T09:35:00Z', IntentMoney(250, 'LXP'), False),
    )
    submission = SubmitPlanRequest('4b' * 32, '7c' * 32, (
        IntentLegBinding(0, 'a1' * 32, 'did:layerx:alice', 'custody-key', 'self', 7, 1, 2, IntentMoney(amount, 'LXP')),
    ))
    return request, submission


@pytest.mark.parametrize('amount', [1.9, -0.1, float('nan'), float('inf'), float('-inf'), 1.0,
                                    Decimal('1.9'), Decimal('NaN'), Fraction(3, 2), True, None,
                                    '01', '+1', '1.0', ' 1', '1 ', '\u0661', -1, 1 << 128])
def test_amount_float_truncation_refused_before_financial_mutation(amount):
    observed = []

    def dispatch(method, path, headers, body):
        observed.append((path, body))
        return 500, {'unexpected': 'invalid amount reached service'}

    with server(dispatch) as endpoint:
        client = HumanIntentClient(endpoint, credential=credential())
        with pytest.raises(PlatformSdkError) as failure:
            ProtocolAmount(amount)
        assert failure.value.code is SdkErrorCode.INVALID_ARGUMENT
        request, submission = intent_requests(amount)
        for action in (lambda: client.plan_intent(request),
                       lambda: client.submit_plan(submission, IdempotencyKey('amount-repair'))):
            with pytest.raises(PlatformSdkError) as failure:
                action()
            assert failure.value.code is SdkErrorCode.INVALID_ARGUMENT
    assert observed == []


@pytest.mark.parametrize('amount', [0, 1, (1 << 128) - 1, '0', '1', str((1 << 128) - 1)])
def test_exact_amounts_reach_real_intent_transport_without_loss(amount):
    observed = []

    def dispatch(method, path, headers, body):
        assert method == 'POST' and headers['Content-Type'] == 'application/json'
        assert headers['Authorization'] == AUTHORIZATION
        request = json.loads(body)
        observed.append(request)
        if path == '/v1/intents/plan':
            assert request['money']['amount'] == str(amount)
            source = IntentEndpointRef(request['source']['kind'], None)
            destination = IntentEndpointRef(request['destination']['kind'], request['destination']['account'])
            plan = IntentPlan(
                INTENT_VECTOR['plan_result']['plan_digest'], 'transfer', IntentMoney(9, 'LXP'),
                (IntentLeg(0, 'transfer', 'paxeer', source, destination,
                           IntentMoney(ProtocolAmount(request['money']['amount']), request['money']['currency']),
                           IntentMoney(9, 'LXP')),),
                (IntentSigningRequirement(0, 'a1' * 32, 'c3' * 32, 'evm-sender'),),
            )
            result = asdict(plan)
            assert result.pop('tx_hash') is None
            result['total_fee']['amount'] = str(plan.total_fee.amount)
            for leg in result['legs']:
                for key in ('to', 'value', 'calldata', 'nonce', 'chain_id', 'gas_limit',
                            'max_fee_per_gas', 'max_priority_fee_per_gas'):
                    if leg[key] is None:
                        del leg[key]
                leg['source'] = request['source']
                leg['destination'] = request['destination']
                leg['money']['amount'] = str(leg['money']['amount'])
                leg['fee']['amount'] = str(leg['fee']['amount'])
        else:
            assert path == '/v1/intents/submit'
            assert headers['Idempotency-Key'] == 'amount-repair'
            assert request['bindings'][0]['fee_limit']['amount'] == str(amount)
            result = {'journey_id': 'jrn_01j2gx3fam9kq4vte8n5w6y7z8', 'plan_digest': '4b' * 32,
                      'state': 'processing', 'state_copy_key': 'status.processing'}
        return 200, {'ok': True, 'result': result, 'trace': INTENT_VECTOR['plan_trace']}

    with server(dispatch) as endpoint:
        client = HumanIntentClient(endpoint, credential=credential())
        request, submission = intent_requests(amount)
        assert int(ProtocolAmount(amount)) == int(amount)
        planned = client.plan_intent(request)
        assert planned.plan_digest == INTENT_VECTOR['plan_result']['plan_digest']
        assert planned.legs[0].money.amount == int(amount)
        assert client.submit_plan(submission, IdempotencyKey('amount-repair')).state == 'processing'
    assert len(observed) == 2


@pytest.mark.parametrize('interface', [None, b'i', b'i' * 952])
def test_deployment_complete_payload_bound_exact_and_one_over(interface):
    budget = MAX_DEPLOY_PAYLOAD_BYTES - DEPLOY_ENVELOPE_BYTES - (0 if interface is None else 4 + len(interface))
    wasm = WASM + bytes(budget - len(WASM))
    deployment = NativeProgramDeploy(PROGRAM, 5, bytes(32), sha256(wasm).digest(), wasm, interface)
    assert deployment.wasm_budget == budget
    encoded = deployment.encode()
    assert len(encoded) == MAX_DEPLOY_PAYLOAD_BYTES == 524288
    assert NativeProgramDeploy.decode(encoded) == deployment
    oversized_wasm = wasm + b'\0'
    oversized = replace(deployment, wasm=oversized_wasm, new_hash=sha256(oversized_wasm).digest())
    with pytest.raises(ValueError, match=f'remaining WASM budget is {budget} bytes'):
        oversized.encode()
    frame = bytearray(encoded)
    frame[100:104] = (budget + 1).to_bytes(4, 'big')
    with pytest.raises(ValueError, match=f'remaining WASM budget is {budget} bytes'):
        NativeProgramDeploy.decode(bytes(frame) + b'\0')


def test_deployment_bound_is_enforced_on_signed_transport_path():
    interface = b'interface'
    budget = MAX_DEPLOY_PAYLOAD_BYTES - DEPLOY_ENVELOPE_BYTES - 4 - len(interface)
    wasm = WASM + bytes(budget - len(WASM))
    node = ProgramNode()
    with server(node.dispatch) as endpoint:
        client = program_client(endpoint)
        for excess in (0, 1):
            code = wasm + bytes(excess)
            digest = sha256(code).digest()
            calldata = encode_call('deploy', LIFECYCLE_TYPES['deploy'], (PROGRAM, 5, 0, bytes(32), digest, interface, code))
            canonical, _ = sign_transaction(calldata)
            identity = keccak256(canonical).hex()
            if excess:
                with pytest.raises(PlatformSdkError) as failure:
                    client._client.agent('program.deploy', {'signed_activity': canonical.hex()}, idempotency_key=IdempotencyKey(identity))
                assert failure.value.code is SdkErrorCode.INVALID_ARGUMENT
            else:
                value = NativeProgramDeploy(PROGRAM, 5, bytes(32), digest, code, interface)
                request = NativeProgramLifecycleRequest.deploy(value, canonical)
                assert client.deploy(request, IdempotencyKey(identity))['state'] == 'pending'
                assert node.decoded == [value]
                assert len(node.decoded[0].encode()) == MAX_DEPLOY_PAYLOAD_BYTES
    assert len(node.transactions) == len(node.requests) == 1


def test_lifecycle_hash_recovery_refuses_receipt_for_other_signed_transaction():
    ordinal, value, name, values = lifecycle_cases()[0]
    canonical, _ = sign_transaction(encode_call(name, LIFECYCLE_TYPES[name], values))
    identity = keccak256(canonical).hex()
    request = NativeProgramLifecycleRequest(ordinal, value.encode(), canonical)
    node = ProgramNode()
    node.receipts[identity] = {**PROGRAM_VECTOR['receiptJson'], 'txHash': '0x' + identity}
    with server(node.dispatch) as endpoint:
        client = program_client(endpoint)
        assert client.deploy(request, IdempotencyKey(identity))['state'] == 'pending'
        with pytest.raises(sdk.SequencerReceiptVerificationError) as failure:
            client.lifecycle_receipt(request)
        assert failure.value.check == 'hash_mismatch'
    assert [row['method'] for row in node.requests] == ['eth_sendRawTransaction', 'pax_sequencerReceipt']
