"""Loopback recording of the supported send/recovery boundary.

The transaction and receipt are unmodified node-produced conformance vectors.
This is a recorded peer, not an implementation of the gateway's signing,
authorization, nonce, policy, or PostgreSQL recovery machinery.
"""
from contextlib import contextmanager
from copy import deepcopy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
from pathlib import Path
import tempfile
import threading

ROOT = Path(__file__).resolve().parents[3]
CORPUS = ROOT / 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json'
# Public fixture text, never an operational JWT or credential.
RECORDING_TOKEN = 'public-recording-session-22-31-432'


def qualification():
    spec = importlib.util.spec_from_file_location('wallet_custody_recovery',
                                                 Path(__file__).with_name('wallet-custody-recovery.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def transfer_recording():
    corpus = json.loads(CORPUS.read_text())
    if corpus['provenance']['generator'] != 'rpc/sequencer_receipt_vectors_test.go':
        raise RuntimeError('node-generated transfer corpus required')
    return deepcopy(next(case for case in corpus['cases'] if case['id'] == 'V1-pax-transfer'))


class RecordedGateway:
    def __init__(self, state, case=None):
        self.module = qualification()
        self.case = transfer_recording() if case is None else deepcopy(case)
        self.state = Path(state)
        self.requests = []
        self.errors = []
        self.receipt = deepcopy(self.case['receiptJson'])
        self.rpc_id = 1
        self.visible = self.state.exists()
        self.accepted = json.loads(self.state.read_text()) if self.state.exists() else None
        _, _, self.wire, self.hash = self.module.native_transaction(self.case['rawTransaction'])
        if self.accepted is not None:
            assert self.accepted == {'tx': self.wire, 'raw': self.case['rawTransaction'], 'tx_hash': self.hash}

    def dispatch(self, path, authorization, body):
        self.requests.append((path, authorization, deepcopy(body)))
        if authorization != 'Bearer ' + RECORDING_TOKEN:
            return 401, {'error': 'unauthorized'}
        if path == '/v1/wallet/send':
            assert body == {'tx': self.wire}
            assert self.accepted is None, 'recording admits one economic send only'
            self.accepted = {'tx': body['tx'], 'raw': self.case['rawTransaction'], 'tx_hash': self.hash}
            self.state.write_text(json.dumps(self.accepted))
            self.state.chmod(0o600)
            return 202, {'error': 'outcome_unknown', 'tx_hash': self.hash,
                         'nonce': self.wire['nonce'], 'retry': 'poll_receipt',
                         'poll': 'eth_getTransactionReceipt', 'must_not_resubmit': True,
                         'member': 0, 'members': 1}
        assert path == '/rpc'
        assert body == {'jsonrpc': '2.0', 'id': 1, 'method': 'pax_sequencerReceipt', 'params': [self.hash]}
        return 200, {'jsonrpc': '2.0', 'id': self.rpc_id,
                     'result': deepcopy(self.receipt) if self.visible and self.accepted else None}

    @contextmanager
    def serving(self):
        peer = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                try:
                    assert self.headers['Content-Type'] == 'application/json'
                    length = int(self.headers['Content-Length'])
                    assert 0 < length <= 131072
                    body = json.loads(self.rfile.read(length))
                    status, value = peer.dispatch(self.path, self.headers.get('Authorization'), body)
                except Exception as error:
                    peer.errors.append(error)
                    status, value = 500, {'error': 'recording_mismatch'}
                data = json.dumps(value).encode()
                self.send_response(status)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(data)))
                self.end_headers()
                self.wfile.write(data)

        server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            yield f'http://127.0.0.1:{server.server_port}'
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=5)
        assert not thread.is_alive()
        assert not self.errors


def qualify_recording(directory=None):
    if directory is None:
        with tempfile.TemporaryDirectory(prefix='wallet-native-recovery-') as temporary:
            return qualify_recording(Path(temporary))
    directory = Path(directory)
    module = qualification()
    retired = module.assert_provider_contract()
    case = transfer_recording()
    state = directory / 'consumer.json'
    peer = RecordedGateway(directory / 'gateway-recording.json', case)
    with peer.serving() as url:
        client = module.RecoveryQualification(url, RECORDING_TOKEN, state,
                                               case['rawTransaction'], case['pinnedKey'])
        client.submit()
        assert client.recover() is None
        assert client.read_state()['phase'] == 'unknown'
    before = peer.state.read_bytes()
    # Destroy both objects and stop the listener before reopening the recording
    # and the actual qualification consumer from their private durable records.
    del client
    reopened = RecordedGateway(peer.state, case)
    with reopened.serving() as url:
        recovered = module.RecoveryQualification(url, RECORDING_TOKEN, state,
                                                  case['rawTransaction'], case['pinnedKey'])
        answer = recovered.recover()
        assert answer == case['receiptJson']
        assert recovered.read_state()['phase'] == 'settled'
    assert reopened.state.read_bytes() == before
    assert [path for path, _, _ in peer.requests].count('/v1/wallet/send') == 1
    assert [path for path, _, _ in reopened.requests] == ['/rpc']
    return {'qualification': 'recorded-native-send-recovery', 'exit_code': 0,
            'retired': retired, 'tx_hash': recovered.hash,
            'recovered_raw': reopened.accepted['raw'], 'economic_sends': 1,
            'proof': 'node-produced signed receipt verified by production Python SDK'}
