"""Real qualification clients, production codecs, recorded receipts and loopback peers.

The committed execution corpus was emitted by rpc/sequencer_receipt_vectors_test.go.
The in-process peer owns durable state and uses the SDK's ABI encoder and native
lifecycle producer to serve current node and gateway wire contracts. No client,
codec, signature verifier, network transport or executable admission is replaced.
"""
from copy import deepcopy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import sys
import threading

import pytest

QUAL = Path(__file__).resolve().parents[1]
ROOT = QUAL.parents[2]
sys.path.insert(0, str(QUAL))
import merged_qualification as merged
from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import decode_signed, encode_call
from layerx_sdk.program_lifecycle import NativeProgramDeploy, NativeProgramUpgrade
from layerx_sdk.x402_receive import encode_grant
from layerx_sdk.verifier import SequencerReceiptVerificationError


def load(name):
    spec = importlib.util.spec_from_file_location('repair_779_' + name.replace('-', '_'), QUAL / (name + '.py'))
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


TYPED = load('programs_typed_interfaces')
ROUTER = load('router-complete-readiness')
WALLET = load('wallet-402-caps')
CEREMONY = load('wallet-ceremony-resume')
AUTH = 'LayerX-Key qualification:lxp_live_' + '99' * 32
REVISION = '13531a9e95bc34a9b21e00a3e375a215261f2bd6'


def corpus():
    document = json.loads((ROOT / 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json').read_text())
    assert document['provenance']['generator'] == 'rpc/sequencer_receipt_vectors_test.go'
    return deepcopy([row for row in document['cases'] if row['id'] in ('V1-layerxprograms-call', 'V1-layerxledger-pay-exact')])


def native(name='deploy'):
    return json.loads((ROOT / ('sdk/platform/conformance/fixtures/native-program-' + name + '-v4.json')).read_text())


def pins(record):
    pin = record['pinnedKey']
    return [{'key_id': pin['keyId'][2:], 'public_key': pin['publicKey'][2:],
             'first_height': pin['firstHeight'], 'last_height': pin['lastHeight']}]


def encoded_result(surface, method, values):
    abi = merged.abi_method(surface, method)
    types = [merged.CHAIN.Node.abi_type(row) for row in abi['outputs']]
    return '0x' + encode_call('producedAbiResult', types, values)[4:].hex()


def calldata(surface, method, arguments):
    abi = merged.abi_method(surface, method)
    return '0x' + encode_call(method, [merged.CHAIN.Node.abi_type(row) for row in abi['inputs']], arguments).hex()


class Peer:
    """An in-process node/gateway with persistent projections and original receipts."""
    def __init__(self, path):
        self.path = path
        self.records = {row['receiptJson']['txHash']: row for row in corpus()}
        self.program_record = next(row for row in self.records.values() if row['id'] == 'V1-layerxprograms-call')
        self.payment_record = next(row for row in self.records.values() if row['id'] == 'V1-layerxledger-pay-exact')
        self.ports = {'node': 0, 'gateway': 0, 'wallet': 0}
        self.servers, self.threads, self.requests = {}, [], []
        self.failures = []
        self.response_damage = None
        self.data = {'accepted': {}, 'views': {}, 'logs': [], 'blocks': {}, 'unavailable': [], 'unconfigured': [],
                     'protocol_network_id': 77, 'chain_id': '0x7d', 'omit': [], 'event_producer': False}
        for reference, record in self.records.items():
            row = self.execution(reference)
            self.data['blocks'][row['blockNumber']] = {'number': row['blockNumber'], 'hash': row['blockHash'],
                'stateRoot': record['receiptJson']['stateRoot'] if 'stateRoot' in record['receiptJson'] else '0x' + 'ab' * 32,
                'transactions': [reference]}
        self.persist()
        self.start()

    @property
    def node_url(self):
        return 'http://127.0.0.1:' + str(self.ports['node'])

    @property
    def gateway_url(self):
        return 'http://127.0.0.1:' + str(self.ports['gateway'])

    @property
    def wallet_url(self):
        return 'http://127.0.0.1:' + str(self.ports['wallet'])

    @property
    def finalized(self):
        return self.data['blocks'][max(self.data['blocks'], key=lambda x: int(x, 16))]

    def node(self):
        return merged.CHAIN.Node(self.node_url, pins(self.program_record), {'Authorization': AUTH})

    def client(self):
        return merged.Client(self.node(), self.gateway_url, {'Authorization': AUTH})

    def persist(self):
        self.path.write_text(json.dumps(self.data, sort_keys=True))
        self.path.chmod(0o600)

    def execution(self, reference):
        row = deepcopy(self.records[reference]['receiptJson'])
        row.update(rung='committed', blockNumber=row['height'], blockHash='0x' + keccak256(reference.encode()).hex())
        return row

    def readiness(self):
        roster = {'durable_store', 'paxeer_chain', 'consensus'} | ({'event_producer'} if self.data['event_producer'] else set())
        backends = {}
        for name in roster - set(self.data['omit']):
            backends[name] = ({'state': 'unavailable', 'reason': 'not_configured'} if name in self.data['unconfigured'] else
                              {'state': 'unavailable', 'reason': 'unreachable'} if name in self.data['unavailable'] else
                              {'state': 'ready', 'reason': 'ready'})
        return {'readiness_version': 1, 'service': 'layerx-gateway', 'scope': 'core',
            'protocol_network_id': self.data['protocol_network_id'], 'protocol_version': 3,
            'observed_at_ms': 1_000_000, 'valid_until_ms': 1_030_000,
            'status': 'degraded' if self.data['unavailable'] else 'ready',
            'components': {'durable_store': 'unavailable' if 'durable_store' in self.data['unavailable'] else 'ready'},
            'capabilities': {'chain_serving': {**backends.get('paxeer_chain', {'state': 'ready', 'reason': 'ready'}), 'chain_id': 125},
                             'layerx_activity': {'state': 'not_configured', 'reason': 'not_configured'}},
            'backends': backends}

    def capabilities(self):
        # This is the current producer's full shape; addresses come from its ABI inventory.
        addresses = {'exchange': '1015', 'bridge': '1016', 'launchpad': '1017', 'ledger': '101a',
                     'pay': '101b', 'auth': '101c', 'programs': '101d'}
        return {**{name: True for name in addresses},
                'addresses': {name: '0x' + '00' * 18 + suffix for name, suffix in addresses.items()},
                'probed_at': 1_750_000_000, 'rpc_height': str(int(self.finalized['number'], 16))}

    def view(self, surface, address, method, arguments, values):
        self.data['views'][address + ':' + calldata(surface, method, arguments)] = encoded_result(surface, method, values)
        self.persist()

    def dispatch(self, role, path, document, headers):
        assert path == '/'
        assert headers.get('Authorization') == AUTH
        assert headers.get_content_type() == 'application/json'
        assert set(document) == {'jsonrpc', 'id', 'method', 'params'} and document['jsonrpc'] == '2.0'
        method, params = document['method'], document['params']
        if method.startswith('lx_'):
            assert role == 'gateway'
            return {'jsonrpc': '2.0', 'id': document['id'], 'error': {'code': -32601, 'message': 'Method not found'}}
        if role == 'gateway':
            assert method in ('px_getCapabilities', 'pax_sequencerReceipt')
            result = self.capabilities() if method == 'px_getCapabilities' else self.execution(params[0])
        elif method == 'eth_chainId':
            result = self.data['chain_id']
        elif method == 'eth_getBlockByNumber':
            assert params[1] is False
            result = deepcopy(self.finalized if params[0] == 'finalized' else self.data['blocks'][params[0]])
        elif method == 'eth_call':
            assert role == 'node' and params[1] in ('latest', 'finalized', self.finalized['number'])
            key = params[0]['to'] + ':' + params[0]['data']
            result = deepcopy(self.data['views'][key])
            if isinstance(result, dict):
                return {'jsonrpc': '2.0', 'id': document['id'], 'error': result}
        elif method == 'eth_getLogs':
            selection = params[0]
            assert set(selection) == {'address', 'topics', 'fromBlock', 'toBlock'}
            result = [deepcopy(row) for row in self.data['logs'] if row['address'] == selection['address']
                      and row['topics'][0] == selection['topics'][0]
                      and int(selection['fromBlock'], 16) <= int(row['blockNumber'], 16) <= int(selection['toBlock'], 16)]
        elif method == 'eth_sendRawTransaction':
            reference = '0x' + keccak256(bytes.fromhex(params[0][2:])).hex()
            assert self.records[reference]['rawTransaction'] == params[0]
            transaction, signature = decode_signed(bytes.fromhex(params[0][2:]))
            assert transaction.chain_id == 125 and transaction.recover_signer(signature)
            self.data['accepted'][reference] = params[0]
            self.persist()
            result = reference
        elif method in ('pax_sequencerReceipt', 'eth_getRawTransactionByHash', 'eth_getTransactionReceipt'):
            row = self.execution(params[0])
            if method == 'pax_sequencerReceipt':
                result = row
            elif method == 'eth_getRawTransactionByHash':
                result = self.records[params[0]]['rawTransaction']
            else:
                result = {'transactionHash': row['txHash'], 'blockHash': row['blockHash'], 'blockNumber': row['blockNumber'],
                    'transactionIndex': row['index'], 'status': row['status'], 'from': row['from'], 'to': row['to'], 'logs': row['logs']}
        else:
            raise AssertionError('undeclared peer method: ' + method)
        answer = {'jsonrpc': '2.0', 'id': document['id'], 'result': result}
        if self.response_damage:
            self.response_damage(role, method, answer)
        return answer

    def start(self):
        self.data = json.loads(self.path.read_text())
        peer = self
        for role in self.ports:
            def handler_for(bound_role):
                class Handler(BaseHTTPRequestHandler):
                    def log_message(self, *_):
                        pass

                    def emit(self, status, body):
                        raw = json.dumps(body, separators=(',', ':')).encode()
                        self.send_response(status)
                        self.send_header('Content-Type', 'application/json')
                        self.send_header('Content-Length', str(len(raw)))
                        self.end_headers()
                        self.wfile.write(raw)

                    def do_GET(self):
                        try:
                            assert bound_role == 'gateway' and self.path in ('/readyz', '/readyz/core')
                            assert self.headers.get('Authorization') == AUTH
                            peer.requests.append((bound_role, self.path, None))
                            self.emit(503 if peer.data['unavailable'] else 200, peer.readiness())
                        except BaseException as error:
                            peer.failures.append(error)
                            self.emit(500, {'error': str(error)})

                    def do_POST(self):
                        try:
                            raw = self.rfile.read(int(self.headers['Content-Length']))
                            value = json.loads(raw)
                            peer.requests.append((bound_role, self.path, value))
                            if bound_role == 'wallet':
                                assert self.path == '/v1/wallet/sign'
                                assert self.headers.get_content_type() == 'application/json'
                                token = self.headers.get('Authorization')
                                if token != 'Bearer owner-session':
                                    self.emit(401, {'error': 'unauthorized'})
                                elif set(value) != {'tx'} or 'from' in value['tx']:
                                    self.emit(400, {'error': 'invalid_body'})
                                elif value['tx'].get('chainId') != 125:
                                    self.emit(403, {'error': 'chain_mismatch'})
                                else:
                                    # The isolated custody plane has fewer than its required three signers.
                                    self.emit(503, {'error': 'attestor_quorum_unavailable'})
                            else:
                                self.emit(200, peer.dispatch(bound_role, self.path, value, self.headers))
                        except BaseException as error:
                            peer.failures.append(error)
                            self.emit(500, {'error': str(error)})
                return Handler
            server = ThreadingHTTPServer(('127.0.0.1', self.ports[role]), handler_for(role))
            server.daemon_threads = True
            self.ports[role] = server.server_port
            self.servers[role] = server
            thread = threading.Thread(target=server.serve_forever, kwargs={'poll_interval': .01}, daemon=True)
            thread.start()
            self.threads.append(thread)

    def stop(self):
        for server in self.servers.values():
            server.shutdown()
            server.server_close()
        for thread in self.threads:
            thread.join(2)
        self.servers, self.threads = {}, []

    def restart(self):
        self.persist()
        self.stop()
        self.data = None
        self.start()


@pytest.fixture
def peer(tmp_path):
    result = Peer(tmp_path / 'producer-state.json')
    try:
        yield result
        assert result.failures == []
    finally:
        result.stop()


def call_arguments(node, record):
    transaction, _ = decode_signed(bytes.fromhex(record['rawTransaction'][2:]))
    method = node.method('call')
    types = [node.abi_type(row) for row in method['inputs']]
    values = merged.decode_abi(types, '0x' + transaction.data[4:].hex())
    assert node.program_call('call', values) == transaction.data
    return values


def json_args(method, values):
    def convert(entry, value):
        if entry['type'].endswith(']'):
            return [convert(dict(entry, type=entry['type'][:entry['type'].rindex('[')]), item) for item in value]
        if entry['type'].startswith('bytes'):
            return value.hex()
        return value
    return [convert(entry, value) for entry, value in zip(merged.CHAIN.Node.method(method)['inputs'], values, strict=True)]


def install_program(peer, abi=2):
    produced = NativeProgramDeploy.decode(bytes.fromhex(native()['payload_hex']))
    interface = produced.interface
    assert produced.encode().hex() == native()['payload_hex']
    assert '0x' + peer.node().program_call('deploy', [produced.program_id, produced.guest_abi, produced.policy,
        produced.authority, produced.new_hash, interface, produced.wasm]).hex() == native()['evm']['calldata']
    if abi != produced.guest_abi:
        offset = len(b'LayerX/program-interface/v1\x00') + 32
        interface = interface[:offset] + abi.to_bytes(2, 'big') + interface[offset + 2:]
    peer.view('layerxprograms', merged.CHAIN.PROGRAMS, 'getProgram', [produced.program_id],
              [produced.policy, produced.authority, produced.new_hash, abi, 1, True])
    peer.view('layerxprograms', merged.CHAIN.PROGRAMS, 'getInterface', [produced.program_id],
              [1, hashlib.sha256(interface).digest(), interface])
    peer.view('layerxprograms', merged.CHAIN.PROGRAMS, 'getStateHead', [produced.program_id], [b'persisted-native-head'])
    return produced, interface


def test_typed_programs_deploy_native_producer_and_current_evm_interface(peer):
    produced, interface = install_program(peer)
    node = peer.node()
    args = [produced.program_id, produced.guest_abi, produced.policy, produced.authority, produced.new_hash, interface, produced.wasm]
    peer.view('layerxprograms', merged.CHAIN.PROGRAMS, 'deploy', args, [])
    action = {'method': 'deploy', 'arguments': json_args('deploy', args), 'raw_transaction': None, 'refusal': None, 'expected': '0x'}
    assert TYPED.TypedPrograms(node, peer.restart, peer.program_record['receiptJson']['from']).action(action) == '0x'
    assert peer.requests[-1][0:2] == ('node', '/')
    assert peer.requests[-1][2]['params'][0]['data'] == native()['evm']['calldata']
    assert TYPED.TypedPrograms(node, peer.restart, peer.program_record['receiptJson']['from']).discover(produced.program_id, produced.guest_abi,
        produced.new_hash, interface, 1)[1][2] == interface


@pytest.mark.parametrize('abi', [1, 2, 3, 4])
@pytest.mark.parametrize('field', ['code_hash', 'abi', 'version', 'interface'])
def test_typed_programs_discovery_preserves_every_target_refusal(peer, abi, field):
    produced, interface = install_program(peer, abi)
    args = {'program': produced.program_id, 'abi': abi, 'code_hash': produced.new_hash, 'interface': interface, 'version': 1}
    args[field] = {'code_hash': b'\xff' * 32, 'abi': 5, 'version': 2, 'interface': interface + b'\0'}[field]
    with pytest.raises(ValueError, match='CODE_HASH_MISMATCH|STALE_INTERFACE'):
        TYPED.TypedPrograms(peer.node(), peer.restart, peer.program_record['receiptJson']['from']).discover(**args)


def test_typed_programs_call_simulation_and_committed_recovery(peer):
    node = peer.node()
    session = TYPED.TypedPrograms(node, peer.restart, peer.program_record['receiptJson']['from'])
    args = call_arguments(node, peer.program_record)
    peer.data['views'][merged.CHAIN.PROGRAMS + ':0x' + node.program_call('simulateCall', args).hex()] = peer.program_record['receiptJson']['returnData']
    peer.view('layerxprograms', merged.CHAIN.PROGRAMS, 'getStateHead', [args[0]], [b'retained-state'])
    produced, interface = install_program(peer)
    peer.view('layerxprograms', merged.CHAIN.PROGRAMS, 'getProgram', [args[0]],
              [produced.policy, produced.authority, produced.new_hash, args[1], 1, True])
    peer.view('layerxprograms', merged.CHAIN.PROGRAMS, 'getInterface', [args[0]],
              [1, hashlib.sha256(interface).digest(), interface])
    peer.view('layerxprograms', merged.CHAIN.PROGRAMS, 'getStateHead', [args[0]], [b'retained-state'])
    before = session.state(args[0])
    action = {'method': 'call', 'arguments': json_args('call', args), 'raw_transaction': peer.program_record['rawTransaction'],
              'refusal': None, 'expected': peer.program_record['receiptJson']['returnData']}
    committed = session.action(action)
    assert committed['transaction_hash'] == peer.program_record['receiptJson']['txHash']
    simulation = {'method': 'simulateCall', 'arguments': json_args('simulateCall', args), 'raw_transaction': None,
                  'refusal': None, 'expected': peer.program_record['receiptJson']['returnData']}
    assert session.action(simulation) == simulation['expected']
    assert session.state(args[0]) == before
    session.recover(args[0], [committed])
    reopened = TYPED.TypedPrograms(peer.node(), peer.restart, peer.program_record['receiptJson']['from'])
    assert reopened.state(args[0]) == before
    assert reopened.node.committed(committed['transaction_hash']) == committed
    assert reopened.action(simulation) == simulation['expected']
    assert peer.data['accepted'] == {committed['transaction_hash']: peer.program_record['rawTransaction']}


@pytest.mark.parametrize('finding', ['wrong-hash', 'unknown-abi', 'wrong-schema', 'wrong-capability', 'incompatible-upgrade'])
def test_typed_programs_exact_registration_refusals_and_unchanged_state(peer, finding):
    produced, interface = install_program(peer)
    node = peer.node()
    session = TYPED.TypedPrograms(node, peer.restart, peer.program_record['receiptJson']['from'])
    method = 'upgrade' if finding == 'incompatible-upgrade' else 'deploy'
    if method == 'upgrade':
        value = NativeProgramUpgrade.decode(bytes.fromhex(native('upgrade')['payload_hex']))
        args = [value.program_id, value.guest_abi, int(bool(value.migration_hook)) | int(value.clear_interface) << 1,
                value.old_hash, value.new_hash, value.migration_hook, value.interface or b'', value.wasm, 1000]
    else:
        args = [produced.program_id, produced.guest_abi, produced.policy, produced.authority, produced.new_hash, interface, produced.wasm]
        if finding == 'wrong-hash':
            args[4] = b'\xff' * 32
        elif finding == 'unknown-abi':
            args[1] = 65535
        elif finding == 'wrong-schema':
            args[5] = b'noncanonical-interface'
        else:
            args[5] = interface + b'\xff'
    refusal = {'code': 3, 'message': 'execution reverted: LXP_INVALID_INPUT', 'data': '0x'}
    peer.data['views'][merged.CHAIN.PROGRAMS + ':0x' + node.program_call(method, args).hex()] = refusal
    peer.persist()
    before = session.state(produced.program_id)
    row = {'id': 'abi2-' + finding, 'program': produced.program_id.hex(), 'abi': 2, 'code_hash': produced.new_hash.hex(),
        'interface': interface.hex(), 'version': 1, 'restart': False,
        'actions': [{'method': method, 'arguments': json_args(method, args), 'raw_transaction': None,
                     'refusal': refusal, 'expected': None}]}
    assert session.case(row) == [{'refusal': refusal}]
    assert session.state(produced.program_id) == before
    changed = deepcopy(row)
    changed['actions'][0]['refusal']['message'] = 'unrelated failure'
    with pytest.raises(ValueError, match='wrong typed ABI refusal'):
        session.case(changed)


def test_typed_case_inventory_keeps_all_abis_refusals_and_languages():
    rows = [{'id': f'abi{abi}-{name}'} for abi in range(1, 5) for name in TYPED.ABI_CASES]
    rows += [{'id': 'untyped-deploy'}, {'id': 'untyped-interface-refused'}]
    assert TYPED.case_inventory(rows) == rows
    assert len(TYPED.LANGUAGES) == 8 and len(TYPED.CONSUMER_CASES) == 5
    for missing in range(len(rows)):
        with pytest.raises(ValueError, match='complete typed ABI'):
            TYPED.case_inventory(rows[:missing] + rows[missing + 1:])


def router(peer):
    return ROUTER.Run(peer.node(), peer.gateway_url, peer.program_record['receiptJson']['txHash'], 77, {'Authorization': AUTH})


def test_router_current_producer_transport_consumer_and_restart(peer):
    run = router(peer)
    baseline = run.readiness()
    committed = run.usable()
    run.retired_refusals()
    peer.restart()
    reopened = router(peer)
    assert reopened.readiness()['backends'] == baseline['backends']
    assert reopened.usable() == committed
    assert all(path != '/rpc' for _, path, _ in peer.requests)
    assert {role for role, _, _ in peer.requests} >= {'node', 'gateway'}


@pytest.mark.parametrize('backend', ['paxeer_chain', 'consensus', 'durable_store'])
def test_router_each_current_backend_outage_and_recovery(peer, backend):
    run = router(peer)
    peer.data['unavailable'] = [backend]
    assert run.readiness({backend})['status'] == 'degraded'
    with pytest.raises(RuntimeError, match='backend state mismatch'):
        run.readiness()
    peer.data['unavailable'] = []
    peer.persist()
    peer.restart()
    assert router(peer).readiness()['status'] == 'ready'
    assert router(peer).usable()['transaction_hash'] == peer.program_record['receiptJson']['txHash']


@pytest.mark.parametrize('backend', ['paxeer_chain', 'consensus', 'durable_store'])
def test_router_configured_roster_omission_is_never_healthy(peer, backend):
    peer.data['omit'] = [backend]
    with pytest.raises(RuntimeError, match='roster omitted'):
        router(peer).readiness()


def test_router_wrong_network_missing_consensus_and_chain_only_refusals(peer):
    run = router(peer)
    peer.data['protocol_network_id'] = 78
    with pytest.raises(RuntimeError, match='wrong protocol network'):
        run.readiness()
    peer.data['protocol_network_id'] = 77
    peer.data['unconfigured'] = ['consensus']
    assert run.readiness(unconfigured={'consensus'})['backends']['consensus']['reason'] == 'not_configured'
    with pytest.raises(RuntimeError, match='backend state mismatch: consensus'):
        run.readiness()
    peer.data['unconfigured'] = []
    run.retired_refusals()
    peer.data['chain_id'] = '0x1'
    with pytest.raises(RuntimeError, match='chain id'):
        run.usable()


@pytest.mark.parametrize('damage', ['rung', 'block', 'receipt', 'id'])
def test_router_never_accepts_uncommitted_or_substituted_receipt(peer, damage):
    def corrupt(role, method, answer):
        if method == 'pax_sequencerReceipt' and role == 'node':
            if damage == 'id':
                answer['id'] = 'foreign-request'
            elif damage == 'rung':
                answer['result']['rung'] = 'receipted'
            elif damage == 'block':
                answer['result']['blockHash'] = '0x' + 'ff' * 32
            else:
                wire = bytearray.fromhex(answer['result']['receipt'][2:]); wire[-1] ^= 1
                answer['result']['receipt'] = '0x' + wire.hex()
    peer.response_damage = corrupt
    with pytest.raises((ValueError, RuntimeError, SequencerReceiptVerificationError)):
        router(peer).usable()


def issue_event(peer, surface, address, name, identifier, arguments):
    abi = json.loads((ROOT / 'precompiles' / surface / 'abi.json').read_text())
    event = next(row for row in abi if row.get('type') == 'event' and row['name'] == name)
    signature = name + '(' + ','.join(merged.CHAIN.Node.abi_type(item) for item in event['inputs']) + ')'
    nonindexed = [merged.CHAIN.Node.abi_type(item) for item in event['inputs'] if not item['indexed']]
    block = peer.finalized
    peer.data['logs'].append({'address': address, 'topics': ['0x' + keccak256(signature.encode()).hex(), '0x' + identifier.hex()],
        'data': '0x' + encode_call('eventData', nonindexed, arguments)[4:].hex(), 'removed': False,
        'blockNumber': block['number'], 'blockHash': block['hash'], 'transactionHash': block['transactions'][0],
        'logIndex': hex(len(peer.data['logs']))})
    peer.persist()


def install_caps(peer):
    record = peer.payment_record
    owner = record['receiptJson']['from']
    accounts = []
    for index, address in enumerate((owner, '0x' + '88' * 20, '0x' + '77' * 20), 1):
        key, account = bytes([index]) * 32, bytes([index + 16]) * 32
        value = {'address': address, 'chain_id': 125, 'did': 'did:layerx:' + key.hex(), 'account_id': account.hex()}
        peer.view('addr', WALLET.ADDR, 'getUnifiedAccount', [bytes.fromhex(address[2:])], [bytes.fromhex(address[2:]), 'pax1recorded', key, account])
        accounts.append(value)
    budget_id = b'\xa1' * 32
    budget = [bytes.fromhex(accounts[0]['account_id']), b'\x22' * 32, b'\x33' * 32, b'\x44' * 32,
              100, 100, 10, 7, 0, 1000, 2000, 9000, 0, 0, False, False, [], bytes(32)]
    peer.view('layerxpay', WALLET.PAY, 'getBudget', [budget_id], [budget])
    peer.view('layerxpay', WALLET.PAY, 'budgetRemaining', [budget_id], [93])
    issue_event(peer, 'layerxpay', WALLET.PAY, 'BudgetCreated', budget_id, [budget[2], 100])
    grants = []
    for index, account in enumerate(accounts[:2]):
        identifier = bytes([0xb1 + index]) * 32
        grant = {'grant_id': identifier.hex(), 'from': account['account_id'], 'recipient': '22' * 32, 'asset': '33' * 32,
            'per_draw_maximum': '20', 'allowance': '100', 'recurring': True, 'window_length': '1000', 'expiration': '9000',
            'purpose_hash': '44' * 32, 'has_reference': False, 'reference_hash': '00' * 32, 'revocation_sequence': '0',
            'public_key': '55' * 32, 'signature': '66' * 64}
        wire = encode_grant(grant)
        assert WALLET.grant_record(wire) == grant
        peer.view('layerxledger', WALLET.LEDGER, 'getPayerGrant', [identifier], [wire, 3, 2, 2000, 0, False, False])
        issue_event(peer, 'layerxledger', WALLET.LEDGER, 'PayerGrantIssued', identifier, [])
        grants.append(grant)
    peer.view('xweb', WALLET.XWEB, 'fee', [], [42])
    return accounts, budget_id, budget, grants


def test_wallet_current_capabilities_budget_grant_fee_and_empty_reads(peer):
    accounts, budget_id, budget, grants = install_caps(peer)
    caps = WALLET.WalletCaps(peer.client(), 0)
    observed = caps.snapshot(accounts[0])
    assert observed['state'] == 'ready' and observed['fee'] == '42'
    assert observed['observation']['rung'] == 'committed'
    WALLET.exact(observed, {'budgets': {budget_id.hex(): {'limit': '100', 'spent': '7', 'remaining': '93'}},
        'grants': {grants[0]['grant_id']: {'per_draw_maximum': '20', 'allowance': '100', 'drawn_total': '3'}}})
    assert {row['owner'] for row in observed['budgets'] + observed['grants']} == {accounts[0]['account_id']}
    assert caps.snapshot(accounts[2])['state'] == 'empty'
    assert peer.client().capabilities()['ledger'] is True
    assert all(value['method'] != 'lx_getWalletCaps' for _, _, value in peer.requests if value and 'method' in value)
    assert {row[2]['params'][1] for row in peer.requests if row[2] and row[2].get('method') == 'eth_call'} == {peer.finalized['number']}


def test_wallet_mutation_revocation_and_reopen_keep_fresh_verified_state(peer):
    accounts, budget_id, budget, grants = install_caps(peer)
    caps = WALLET.WalletCaps(peer.client(), 0)
    before = caps.snapshot(accounts[0])
    budget[4] = 120
    peer.view('layerxpay', WALLET.PAY, 'getBudget', [budget_id], [budget])
    peer.view('layerxpay', WALLET.PAY, 'budgetRemaining', [budget_id], [113])
    identifier = bytes.fromhex(grants[0]['grant_id'])
    peer.view('layerxledger', WALLET.LEDGER, 'getPayerGrant', [identifier], [encode_grant(grants[0]), 3, 2, 2000, 2100, True, False])
    changed = caps.snapshot(accounts[0])
    assert changed['budgets'][0]['limit'] == '120' and changed['grants'][0]['revoked'] is True
    assert changed != before
    peer.restart()
    reopened = WALLET.WalletCaps(peer.client(), 0)
    assert reopened.snapshot(accounts[0]) == changed
    assert reopened.snapshot(accounts[2])['state'] == 'empty'


@pytest.mark.parametrize('damage', ['foreign-identity', 'wrong-chain', 'no-finalized', 'grant-padding', 'grant-boolean', 'budget-overflow'])
def test_wallet_preserves_identity_bounds_and_unavailable_refusals(peer, damage):
    accounts, budget_id, budget, grants = install_caps(peer)
    account = deepcopy(accounts[0])
    if damage == 'foreign-identity':
        account['account_id'] = accounts[1]['account_id']
    elif damage == 'wrong-chain':
        account['chain_id'] = 1
    else:
        def corrupt(role, method, answer):
            if damage == 'no-finalized' and method == 'eth_getBlockByNumber':
                answer['result'] = None
            if method == 'eth_call' and isinstance(answer.get('result'), str):
                value = answer['result']
                if damage == 'grant-padding' and len(value) == len(encoded_result('layerxledger', 'getPayerGrant', [encode_grant(grants[0]), 3, 2, 2000, 0, False, False])):
                    answer['result'] = value[:-2] + '01'
                elif damage == 'grant-boolean' and value == encoded_result('layerxledger', 'getPayerGrant', [encode_grant(grants[0]), 3, 2, 2000, 0, False, False]):
                    raw = bytearray.fromhex(value[2:]); raw[5 * 32 + 31] = 2
                    answer['result'] = '0x' + raw.hex()
                elif damage == 'budget-overflow' and value == encoded_result('layerxpay', 'getBudget', [budget]):
                    raw = bytearray.fromhex(value[2:]); raw[32 + 4 * 32] = 1
                    answer['result'] = '0x' + raw.hex()
        peer.response_damage = corrupt
    with pytest.raises((ValueError, RuntimeError)):
        WALLET.WalletCaps(peer.client(), 0).snapshot(account)


def test_wallet_payment_binds_custody_action_fee_and_committed_receipt(peer):
    install_caps(peer)
    caps = WALLET.WalletCaps(peer.client(), 0)
    record = peer.payment_record
    transaction, signature = decode_signed(bytes.fromhex(record['rawTransaction'][2:]))
    sender = transaction.recover_signer(signature)
    result = caps.mutate(record['rawTransaction'], sender, '0x' + transaction.to.hex(), '0x' + transaction.data.hex())
    assert result['transaction_hash'] == record['receiptJson']['txHash']
    assert peer.node().call('pax_sequencerReceipt', [result['transaction_hash']])['feeAmount'] == record['receiptJson']['feeAmount']
    for owner, target, data in [(sender, WALLET.PAY, transaction.data.hex()),
                                ('0x' + '99' * 20, transaction.to.hex(), transaction.data.hex()),
                                (sender, transaction.to.hex(), '00')]:
        with pytest.raises(RuntimeError, match='custody-signed mutation'):
            caps.mutate(record['rawTransaction'], owner, target, data)
    peer.restart()
    assert peer.node().committed(result['transaction_hash']) == result


@pytest.mark.parametrize('token,body,status,error', [
    (None, {'tx': {'chainId': 125}}, 401, 'unauthorized'),
    ('foreign-session', {'tx': {'chainId': 125}}, 401, 'unauthorized'),
    ('expired-session', {'tx': {'chainId': 125}}, 401, 'unauthorized'),
    ('owner-session', {'tx': {'chainId': 1}}, 403, 'chain_mismatch'),
    ('owner-session', {'tx': {'from': '0x' + '99' * 20, 'chainId': 125}}, 400, 'invalid_body'),
    ('owner-session', {'did': 'foreign-selector'}, 400, 'invalid_body'),
    ('owner-session', {'tx': {'chainId': 125}}, 503, 'attestor_quorum_unavailable'),
])
def test_wallet_custody_refusals_never_sign_or_change_cap_state(peer, token, body, status, error):
    accounts, _, _, _ = install_caps(peer)
    caps = WALLET.WalletCaps(peer.client(), 0)
    before = caps.snapshot(accounts[0])
    assert caps.custody_refusal(peer.wallet_url, token, body, status, error)['error'] == error
    assert caps.snapshot(accounts[0]) == before
    assert peer.data['accepted'] == {}


def artifacts(tmp_path):
    config = {'source_revision': REVISION, 'artifacts': {}}
    manifest = {'source_revision': REVISION, 'files': {}, 'artifacts': {}}
    for name in ('ceremony', 'attestor', 'attestor_durability_tests', 'psql'):
        path = tmp_path / name
        # Bytes are admission input only; the gate never launches an external executable.
        path.write_bytes(b'qualification artifact bytes: ' + name.encode())
        path.chmod(0o700)
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        row = {'path': str(path), 'sha256': digest, 'source_revision': REVISION}
        config['artifacts'][name] = row
        manifest['artifacts'][name] = {'sha256': digest, 'source_revision': REVISION}
    return config, manifest


def test_ceremony_manifest_current_revision_and_digest_survive_reopen(tmp_path):
    config, manifest = artifacts(tmp_path)
    config_path, manifest_path = tmp_path / 'config.json', tmp_path / 'source-manifest.json'
    for path, value in [(config_path, config), (manifest_path, manifest)]:
        path.write_text(json.dumps(value)); path.chmod(0o600)
    bindings = CEREMONY.bind_artifacts(config, manifest, REVISION)
    for artifact in config['artifacts'].values():
        argv = [artifact['path'], 'deliver']
        assert CEREMONY.admitted_argv(argv, bindings, REVISION) == argv
    reopened_config = json.loads(CEREMONY.private_file(config_path).read_text())
    reopened_manifest = json.loads(CEREMONY.private_file(manifest_path).read_text())
    assert CEREMONY.bind_artifacts(reopened_config, reopened_manifest, REVISION) == bindings


@pytest.mark.parametrize('place', ['config', 'manifest', 'artifact', 'binding'])
def test_ceremony_rejects_valid_old_revision_even_with_matching_binary_digest(tmp_path, place):
    config, manifest = artifacts(tmp_path)
    old = 'a' * 40
    if place in ('config', 'manifest'):
        (config if place == 'config' else manifest)['source_revision'] = old
    elif place == 'artifact':
        config['artifacts']['ceremony']['source_revision'] = old
    else:
        manifest['artifacts']['ceremony']['source_revision'] = old
    with pytest.raises(RuntimeError, match='revision'):
        CEREMONY.bind_artifacts(config, manifest, REVISION)


@pytest.mark.parametrize('damage', ['digest', 'manifest-digest', 'symlink', 'changed-after-admission', 'unbound', 'revision-after-admission'])
def test_ceremony_refuses_substitution_before_any_launch(tmp_path, damage):
    config, manifest = artifacts(tmp_path)
    binary = Path(config['artifacts']['ceremony']['path'])
    bindings = CEREMONY.bind_artifacts(config, manifest, REVISION)
    if damage == 'digest':
        binary.write_bytes(b'old or substituted executable')
    elif damage == 'manifest-digest':
        manifest['artifacts']['ceremony']['sha256'] = 'ff' * 32
    elif damage == 'symlink':
        original = tmp_path / 'original'
        binary.rename(original); binary.symlink_to(original)
    elif damage == 'changed-after-admission':
        binary.write_bytes(b'substituted after validation')
    elif damage == 'unbound':
        binary = tmp_path / 'unbound'
    with pytest.raises(RuntimeError):
        if damage in ('digest', 'manifest-digest', 'symlink'):
            CEREMONY.bind_artifacts(config, manifest, REVISION)
        else:
            CEREMONY.admitted_argv([str(binary), 'deliver'], bindings, 'a' * 40 if damage == 'revision-after-admission' else REVISION)
    CEREMONY.ARTIFACT_BINDINGS = bindings
    CEREMONY.QUALIFIED_REVISION = REVISION
    if damage == 'manifest-digest':
        # The direct launcher also refuses a binary that changes after its bound record.
        binary.write_bytes(b'changed before launch')
    if damage == 'revision-after-admission':
        CEREMONY.QUALIFIED_REVISION = 'a' * 40
    with pytest.raises(RuntimeError):
        CEREMONY.start([str(binary), 'deliver'], {}, 'must-not-launch')
    assert CEREMONY.PROCESSES == []


def test_typed_programs_foreign_custody_payer_refuses_before_submission(peer):
    node = peer.node()
    args = call_arguments(node, peer.program_record)
    session = TYPED.TypedPrograms(node, peer.restart, '0x' + '99' * 20)
    action = {'method': 'call', 'arguments': json_args('call', args), 'raw_transaction': peer.program_record['rawTransaction'],
              'refusal': None, 'expected': peer.program_record['receiptJson']['returnData']}
    with pytest.raises(ValueError, match='typed custody payer substitution'):
        session.action(action)
    assert peer.data['accepted'] == {}
    assert not any(row[2] and row[2].get('method') == 'eth_sendRawTransaction' for row in peer.requests)
