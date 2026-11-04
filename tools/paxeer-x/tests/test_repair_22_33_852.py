import base64
import copy
import hashlib
from http.server import BaseHTTPRequestHandler, HTTPServer
import importlib.util
import json
from pathlib import Path
import ssl
import struct
import threading
import time

import pytest


HERE = Path(__file__).resolve().parent
FIXTURES = HERE / 'fixtures' / 'repair_22_33_852'
SPEC = importlib.util.spec_from_file_location('routes_repair_22_33_852', HERE / 'routes_test.py')
routes = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(routes)


def recording():
    return json.loads((FIXTURES / 'current-reads.json').read_text())


def product():
    return {
        'chain_id': 125,
        'routes': [
            {'id': 'node:eth', 'service': 'node', 'transport': 'json-rpc',
             'method': 'POST', 'path': 'eth_*'},
            {'id': 'node:balances', 'service': 'node', 'transport': 'json-rpc',
             'method': 'POST', 'path': 'px_getBalances'},
        ],
    }


def node_case(phase):
    rpc = recording()['rpc']
    assertions = {'/0/result': rpc[phase][0]['result'], '/1/result': rpc[phase][1]['result']}
    if phase == 'degraded':
        assertions.update({'/2/error/code': -32001,
                           '/2/error/data/code': 'invalid_paxeer_response'})
    else:
        assertions['/2/result'] = rpc[phase][2]['result']
    return {'target': 'node', 'service': 'node', 'method': 'POST', 'path': '/',
            'status': 200, 'kind': 'positive' if phase != 'degraded' else 'missing-dependency',
            'body': rpc['request'], 'assertions': assertions,
            'response_headers': {'Content-Type': 'application/json'}}


class TranscriptPeer:
    def __init__(self, phase='before', port=0):
        self.transcript = recording()
        self.phase = phase
        self.port = port
        self.seen = []
        self.errors = []
        self.server = None

    def start(self):
        peer = self

        class Handler(BaseHTTPRequestHandler):
            protocol_version = 'HTTP/1.1'

            def log_message(self, *args):
                pass

            def do_POST(self):
                try:
                    assert self.path == '/'
                    assert self.headers['Content-Type'] == 'application/json'
                    body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                    assert body == peer.transcript['rpc']['request']
                    peer.seen.append((self.command, self.path, body))
                    phase = (routes.NodeRecovery.phases[len(peer.seen) - 1]
                             if peer.phase == 'sequence' else peer.phase)
                    data = json.dumps(peer.transcript['rpc'][phase], separators=(',', ':')).encode()
                    self.send_response(200)
                    self.send_header('Content-Type', 'application/json')
                    self.send_header('Content-Length', str(len(data)))
                    self.send_header('Connection', 'close')
                    self.end_headers()
                    self.wfile.write(data)
                except Exception as error:
                    peer.errors.append(error)
                    self.close_connection = True

            def do_GET(self):
                try:
                    phoenix = self.path == '/explorer/backend/socket/v2/websocket?vsn=2.0.0'
                    assert phoenix or self.path == '/'
                    assert self.headers['Upgrade'].lower() == 'websocket'
                    assert self.headers['Connection'].lower() == 'upgrade'
                    assert self.headers['Sec-WebSocket-Version'] == '13'
                    key = self.headers['Sec-WebSocket-Key']
                    assert len(base64.b64decode(key, validate=True)) == 16
                    accept = base64.b64encode(hashlib.sha1(
                        (key + '258EAFA5-E914-47DA-95CA-C5AB0DC85B11').encode()).digest()).decode()
                    self.send_response(101)
                    self.send_header('Upgrade', 'websocket')
                    self.send_header('Connection', 'Upgrade')
                    self.send_header('Sec-WebSocket-Accept', accept)
                    self.end_headers()
                    self.wfile.flush()
                    corpus = peer.transcript['phoenix_websocket' if phoenix else 'root_websocket']
                    for message in corpus:
                        first, second = self.rfile.read(2)
                        assert first == 0x81 and second & 128
                        size = second & 127
                        if size == 126:
                            size = struct.unpack('!H', self.rfile.read(2))[0]
                        assert size < 65536
                        mask = self.rfile.read(4)
                        data = self.rfile.read(size)
                        body = json.loads(bytes(byte ^ mask[i % 4] for i, byte in enumerate(data)))
                        assert body == message['send']
                        peer.seen.append((self.command, self.path, body))
                        reply = json.dumps(message['response'], separators=(',', ':')).encode()
                        frame = (bytes([0x81, len(reply)]) if len(reply) < 126
                                 else bytes([0x81, 126]) + struct.pack('!H', len(reply)))
                        self.wfile.write(frame + reply)
                        self.wfile.flush()
                    if phoenix:
                        first, second = self.rfile.read(2)
                        assert first == 0x88 and second == 0x82
                        mask, data = self.rfile.read(4), self.rfile.read(2)
                        assert bytes(byte ^ mask[i % 4] for i, byte in enumerate(data)) == b'\x03\xe8'
                    self.close_connection = True
                except Exception as error:
                    peer.errors.append(error)
                    self.close_connection = True

        self.server = HTTPServer(('127.0.0.1', self.port), Handler)
        self.port = self.server.server_port
        tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        tls.minimum_version = ssl.TLSVersion.TLSv1_2
        tls.load_cert_chain(FIXTURES / 'localhost-cert.pem', FIXTURES / 'localhost-key.pem')
        self.server.socket = tls.wrap_socket(self.server.socket, server_side=True)
        self.thread = threading.Thread(target=self.server.serve_forever,
                                       kwargs={'poll_interval': 0.01}, daemon=True)
        self.thread.start()
        return self

    @property
    def target(self):
        return {'url': f'https://localhost:{self.port}',
                'ca_file': str(FIXTURES / 'localhost-cert.pem')}

    def stop(self):
        if self.server is not None:
            self.server.shutdown()
            self.server.server_close()
            self.thread.join(timeout=2)
            assert not self.thread.is_alive()
            self.server = None
        assert not self.errors, self.errors

    def __enter__(self):
        return self.start()

    def __exit__(self, *args):
        self.stop()


def test_f030_root_rpc_drives_real_https_codec():
    case = node_case('before')
    catalogue = product()
    case['route_ids'] = [route['id'] for route in catalogue['routes']]
    assert all(routes.route_matches(route, case) for route in catalogue['routes'])
    with TranscriptPeer() as peer:
        document = routes.http_request(peer.target, case)
        routes.require_rpc_results(case, document, {r['id']: r for r in catalogue['routes']})
        assert document == recording()['rpc']['before']
        assert peer.seen == [('POST', '/', case['body'])]
    for retired in ['/rpc', '/rpc/evm', '/rpc/ws']:
        obsolete = dict(case, path=retired)
        for route in catalogue['routes']:
            assert not routes.route_matches(dict(route, endpoint_paths=['/', retired]), obsolete)


def test_f030_root_websocket_drives_real_upgrade_and_masked_frames():
    message = recording()['root_websocket'][0]
    route = {'service': 'node', 'transport': 'websocket', 'method': 'GET', 'path': 'eth_chainId'}
    case = {'service': 'node', 'transport': 'websocket', 'method': 'GET', 'path': '/',
            'kind': 'positive', 'status': 101,
            'messages': [{'send': message['send'], 'assertions': {'/result': '0x7d'}}]}
    assert routes.route_matches(route, case)
    assert not routes.route_matches(route, dict(case, path='/rpc/ws'))
    assert routes.route_matches(dict(route, endpoint_paths=['/rpc/ws']), dict(case, path='/rpc/ws'))
    with TranscriptPeer() as peer:
        routes.websocket_dispatch(peer.target, case)
        assert peer.seen == [('GET', '/', message['send'])]


def test_f030_retained_phoenix_gateway_transport_is_connected():
    path = '/explorer/backend/socket/v2/websocket'
    route = {'service': 'explorer', 'transport': 'websocket', 'method': 'GET',
             'path': path, 'websocket_protocol': 'phoenix-v2'}
    messages = [{'send': row['send'], 'assertions': {'/4/status': 'ok', '/4/response': {}}}
                for row in recording()['phoenix_websocket']]
    case = {'service': 'explorer', 'transport': 'websocket', 'method': 'GET',
            'path': path + '?vsn=2.0.0', 'kind': 'positive', 'status': 101,
            'websocket_protocol': 'phoenix-v2', 'messages': messages}
    assert routes.route_matches(route, case)
    assert not routes.route_matches(route, dict(case, websocket_protocol='json-rpc'))
    with TranscriptPeer() as peer:
        routes.websocket_dispatch(peer.target, case)
        assert [row[2] for row in peer.seen] == [message['send'] for message in messages]
    assert routes.required_websocket_scenarios([route]) == {'phoenix-websocket'}
    assert routes.required_websocket_scenarios([]) == set()
    aliases = [{'transport': 'websocket', 'path': path}
               for path in ['/', '/rpc/ws', '/rpc/evm/ws']]
    assert routes.required_websocket_scenarios([route] + aliases) == {
        'phoenix-websocket', 'json-rpc-websocket'}


@pytest.mark.parametrize('transport', ['json-rpc', 'websocket'])
def test_f030_retired_native_names_cannot_claim_coverage(transport):
    call = {'jsonrpc': '2.0', 'id': 1, 'method': 'lx_getAccount', 'params': []}
    route = {'service': 'node', 'transport': transport, 'path': 'lx_getAccount'}
    case = {'service': 'node', 'method': 'POST' if transport == 'json-rpc' else 'GET',
            'transport': transport, 'path': '/', 'body': call, 'messages': [{'send': call}]}
    assert not routes.route_matches(route, case)


def test_f031_current_reads_refuse_degradation_and_recover_after_peer_restart():
    recovery = routes.NodeRecovery(product())
    peer = TranscriptPeer()
    with peer:
        target = peer.target
        baseline = recovery.observe('before', target, node_case('before'))
    with pytest.raises(OSError):
        routes.http_request(target, node_case('before'))
    with TranscriptPeer('degraded', peer.port) as degraded:
        assert degraded.target == target
        refusal = recovery.observe('degraded', target, node_case('degraded'))
        assert refusal[2]['error'] == recording()['rpc']['degraded'][2]['error']
    with TranscriptPeer('recovered', peer.port) as reopened:
        assert reopened.target == target
        recovered = recovery.observe('recovered', target, node_case('recovered'))
        assert recovered == baseline
        assert recovered[2]['result']['balances'][1]['amount'] == (
            '1606938044258990275541962092341162602522202993782792835301376')
        assert reopened.seen == [('POST', '/', node_case('recovered')['body'])]
    assert recovery.completed == ['before', 'degraded', 'recovered']


def test_f031_plan_runner_executes_every_current_node_observation():
    with TranscriptPeer('sequence') as peer:
        plan = {'targets': {'node': peer.target},
                'node_recovery_cases': [{phase: node_case(phase) for phase in routes.NodeRecovery.phases}]}
        before = routes.COUNT
        routes.run_node_recovery_cases(plan, product(), time.monotonic())
        assert routes.COUNT - before == 3
        assert len(peer.seen) == 3


@pytest.mark.parametrize('fault', ['duplicate-id', 'unmatched-id', 'protocol', 'wrong-chain',
                                 'missing-state', 'height-mismatch', 'both-result-and-error'])
def test_f031_rejects_invalid_connected_current_node_responses(fault):
    with TranscriptPeer() as peer:
        replies = peer.transcript['rpc']['before']
        if fault == 'duplicate-id':
            replies[2]['id'] = replies[1]['id']
        elif fault == 'unmatched-id':
            replies[2]['id'] = 'other'
        elif fault == 'protocol':
            replies[2]['jsonrpc'] = '1.0'
        elif fault == 'wrong-chain':
            replies[0]['result'] = '0x1'
        elif fault == 'missing-state':
            replies[2]['result'] = None
        elif fault == 'height-mismatch':
            replies[2]['result']['height'] = 15
        else:
            replies[2]['error'] = {'code': -32001, 'message': 'Paxeer read unavailable'}
        # Assert the adversarial peer bytes too; semantic validation must reject them.
        case = node_case('before')
        case['assertions'] = {'/0': replies[0], '/1': replies[1], '/2': replies[2]}
        with pytest.raises(routes.Invalid):
            routes.NodeRecovery(product()).observe('before', peer.target, case)
        assert len(peer.seen) == 1


@pytest.mark.parametrize('code,data_code', [(-32601, 'invalid_paxeer_response'),
                                          (-32010, 'kernel_unavailable'),
                                          (-32001, 'kernel_unavailable')])
def test_f031_method_not_found_and_retired_kernel_errors_are_not_degradation(code, data_code):
    with TranscriptPeer() as peer:
        recovery = routes.NodeRecovery(product())
        recovery.observe('before', peer.target, node_case('before'))
        peer.phase = 'degraded'
        refusal = peer.transcript['rpc']['degraded'][2]['error']
        refusal['code'], refusal['data']['code'] = code, data_code
        case = node_case('degraded')
        case['assertions']['/2/error/code'] = code
        case['assertions']['/2/error/data/code'] = data_code
        with pytest.raises(routes.Invalid, match='typed state-unavailable refusal'):
            recovery.observe('degraded', peer.target, case)


def test_f031_recovered_balance_drift_is_refused():
    with TranscriptPeer() as peer:
        recovery = routes.NodeRecovery(product())
        recovery.observe('before', peer.target, node_case('before'))
        peer.phase = 'degraded'
        recovery.observe('degraded', peer.target, node_case('degraded'))
        peer.phase = 'recovered'
        changed = copy.deepcopy(peer.transcript['rpc']['recovered'])
        changed[2]['result']['balances'][0]['amount'] = '123456788'
        peer.transcript['rpc']['recovered'] = changed
        case = node_case('recovered')
        case['assertions']['/2/result'] = changed[2]['result']
        with pytest.raises(routes.Invalid, match='did not recover the committed account state'):
            recovery.observe('recovered', peer.target, case)


def test_f031_degradation_requires_exact_typed_assertions():
    with TranscriptPeer() as peer:
        recovery = routes.NodeRecovery(product())
        recovery.observe('before', peer.target, node_case('before'))
        peer.phase = 'degraded'
        case = node_case('degraded')
        del case['assertions']['/2/error/data/code']
        with pytest.raises(routes.Invalid, match='exact typed refusal assertions'):
            recovery.observe('degraded', peer.target, case)


@pytest.mark.parametrize('plan', [{}, {'chain_only_cases': [node_case('before')]},
                                  {'node_recovery_cases': []},
                                  {'node_recovery_cases': [{'before': node_case('before')}]}])
def test_f031_old_or_incomplete_recovery_plans_are_refused(plan):
    with pytest.raises(routes.Invalid):
        routes.run_node_recovery_cases(plan, product(), time.monotonic())


def test_f031_recovery_cannot_skip_a_phase_or_change_the_target():
    recovery = routes.NodeRecovery(product())
    with pytest.raises(routes.Invalid, match='phases must execute in order'):
        recovery.observe('recovered', {}, node_case('recovered'))
    with TranscriptPeer() as peer:
        recovery.observe('before', peer.target, node_case('before'))
        with pytest.raises(routes.Invalid, match='same state through the same target'):
            recovery.observe('degraded', dict(peer.target, url='https://another.invalid'),
                             node_case('degraded'))
        changed = node_case('degraded')
        changed['body'][2]['params'] = ['0x' + '11' * 20]
        with pytest.raises(routes.Invalid, match='same state through the same target'):
            recovery.observe('degraded', peer.target, changed)
