"""22.33.878 closes f017 with committed receipt, refusal and recovery proofs.

The loopback peer uses an in-memory block store and the node-produced native
transfer corpus, decoded by the production SDK. Explorer responses follow
TransactionStatusView's current three-field JSON contract. No checkpoint peer
or later finality event exists. No out-of-path wiring change is required.

The ordered bounded gates prohibit a baseline verify before implementation;
no baseline test result is claimed. Gate evidence records the verified revision.
"""
from contextlib import contextmanager
from copy import deepcopy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
from pathlib import Path
import sys
import threading
from urllib.error import HTTPError

import pytest

ROOT = Path(__file__).resolve().parents[4]
QUALIFIER = ROOT / 'tools/qualification/paxeer-x/wallet-transfer-truth.py'
sys.path.insert(0, str(ROOT / 'services/agent/sdk/python'))
from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import decode_signed


def load_qualifier():
    spec = importlib.util.spec_from_file_location('wallet_transfer_truth', QUALIFIER)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class TransferPeer:
    """HTTP peer over a block store containing a real signed native transfer."""

    def __init__(self):
        corpus = json.loads((ROOT / 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json').read_text())
        assert corpus['provenance']['generator'] == 'rpc/sequencer_receipt_vectors_test.go'
        self.case = next(case for case in corpus['cases'] if case['id'] == 'V1-pax-transfer')
        self.raw = bytes.fromhex(self.case['rawTransaction'][2:])
        self.tx, self.signature = decode_signed(self.raw)
        self.sender = self.tx.recover_signer(self.signature)
        self.hash = '0x' + keccak256(self.raw).hex()
        assert self.hash == self.case['receiptJson']['txHash']
        self.number = 1000
        self.block_hash = '0x' + keccak256(b'in-process transfer block at height 1000').hex()
        self.transaction = {
            'hash': self.hash, 'from': self.sender, 'to': '0x' + self.tx.to.hex(),
            'value': hex(self.tx.value), 'input': '0x' + self.tx.data.hex(),
            'nonce': hex(self.tx.nonce), 'chainId': hex(self.tx.chain_id),
            'blockNumber': hex(self.number), 'blockHash': self.block_hash, 'transactionIndex': '0x0',
        }
        self.receipt = {
            'transactionHash': self.hash, 'from': self.sender, 'to': self.transaction['to'],
            'status': self.case['receiptJson']['status'], 'blockNumber': hex(self.number),
            'blockHash': self.block_hash, 'transactionIndex': '0x0', 'logs': [],
            'gasUsed': self.case['receiptJson']['gasUsed'],
        }
        self.block = {'number': hex(self.number), 'hash': self.block_hash, 'transactions': [self.hash]}
        self.status = {'rung': 'committed', 'block_number': self.number, 'committed_height': self.number}
        self.chain_id = '0x7d'
        self.http_status = 200
        self.rpc_error = False
        self.rpc_id = 1
        self.requests = []
        self.errors = []

    def rpc(self, body):
        assert body['jsonrpc'] == '2.0' and body['id'] == 1
        method, params = body['method'], body['params']
        self.requests.append((method, params))
        if method == 'eth_chainId':
            assert params == []
            return self.chain_id
        if method == 'eth_getTransactionByHash':
            assert params == [self.hash]
            return self.transaction
        if method == 'eth_getTransactionReceipt':
            assert params == [self.hash]
            return self.receipt
        assert method == 'eth_getBlockByNumber'
        assert params == [self.receipt['blockNumber'], False]
        return self.block

    @contextmanager
    def serving(self):
        peer = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def answer(self, code, value):
                data = json.dumps(value).encode()
                self.send_response(code)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            def do_POST(self):
                try:
                    assert self.path == '/rpc'
                    assert self.headers['Content-Type'] == 'application/json'
                    length = int(self.headers['Content-Length'])
                    assert 0 < length < 4096
                    body = json.loads(self.rfile.read(length))
                    value = {'jsonrpc': '2.0', 'id': peer.rpc_id, 'result': peer.rpc(body)}
                    if peer.rpc_error:
                        value['error'] = {'code': -32000, 'message': 'unavailable'}
                    self.answer(200, value)
                except Exception as error:
                    peer.errors.append(error)
                    self.answer(500, {'error': 'invalid request'})

            def do_GET(self):
                try:
                    assert self.path == f'/api/v2/transactions/{peer.hash}/status'
                    assert self.headers['Accept'] == 'application/json'
                    peer.requests.append(('explorer', self.path))
                    self.answer(peer.http_status, peer.status)
                except Exception as error:
                    peer.errors.append(error)
                    self.answer(500, {'error': 'invalid request'})

        server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=lambda: server.serve_forever(poll_interval=0.01), daemon=True)
        thread.start()
        try:
            yield f'http://127.0.0.1:{server.server_port}'
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=5)
            assert not thread.is_alive()
            assert not self.errors

    def check(self, module, url):
        return module.check_bound_evidence(url + '/rpc', url, self.hash, self.sender)

    def expected(self):
        return {'transaction_hash': self.hash, 'rung': 'committed',
                'block_number': self.number, 'block_hash': self.block_hash}


@pytest.fixture
def peer():
    return TransferPeer()


def test_f017_committed_transfer_is_final_without_checkpoint_fields(peer):
    assert peer.tx.chain_id == 125 and peer.tx.value > 0 and peer.tx.data == b''
    assert peer.tx.attach_signature(peer.signature, peer.sender) == peer.raw
    assert set(peer.status) == {'rung', 'block_number', 'committed_height'}
    with peer.serving() as url:
        assert peer.check(load_qualifier(), url) == peer.expected()
    assert [method for method, _ in peer.requests] == [
        'eth_chainId', 'eth_getTransactionByHash', 'eth_getTransactionReceipt',
        'eth_getBlockByNumber', 'explorer',
    ]


@pytest.mark.parametrize('height', [1000, 1001, 2000])
def test_f017_committed_head_covers_receipt_block(peer, height):
    peer.status['committed_height'] = height
    with peer.serving() as url:
        assert peer.check(load_qualifier(), url) == peer.expected()


@pytest.mark.parametrize('status', [
    {'rung': rung, 'block_number': 1000, 'committed_height': 1000}
    for rung in ('sealed', 'final', 'submitted', 'receipted')
] + [
    {'rung': 'committed', 'block_number': block, 'committed_height': head}
    for block, head in [(1001, 1001), (None, 1000), ('1000', 1000),
                        (1000, 999), (1000, None), (1000, '1000'), (1000, True)]
] + [None, [], {'rung': 'committed', 'block_number': 1000}])
def test_f017_refuses_unbound_or_retired_explorer_status(peer, status):
    peer.status = deepcopy(status)
    with peer.serving() as url:
        with pytest.raises(RuntimeError, match='explorer evidence'):
            peer.check(load_qualifier(), url)


@pytest.mark.parametrize('target,field,value', [
    ('transaction', 'hash', '0x' + '99' * 32),
    ('transaction', 'from', '0x' + '99' * 20),
    ('transaction', 'to', None),
    ('transaction', 'input', '0x1234'),
    ('transaction', 'value', '0x0'),
    ('transaction', 'value', '1'),
    ('transaction', 'blockNumber', '0x3e9'),
    ('transaction', 'blockHash', '0x' + '99' * 32),
    ('receipt', 'transactionHash', '0x' + '99' * 32),
    ('receipt', 'status', '0x0'),
    ('receipt', 'blockNumber', None),
    ('receipt', 'blockHash', None),
    ('block', 'hash', '0x' + '99' * 32),
    ('block', 'number', '0x3e9'),
    ('block', 'transactions', []),
    ('block', 'transactions', [{'hash': '0x' + '99' * 32}]),
])
def test_f017_refuses_failed_or_misbound_transfer_facts(peer, target, field, value):
    getattr(peer, target)[field] = value
    with peer.serving() as url:
        with pytest.raises(RuntimeError):
            peer.check(load_qualifier(), url)


@pytest.mark.parametrize('field', ['transaction', 'receipt', 'block'])
def test_f017_refuses_missing_node_evidence(peer, field):
    setattr(peer, field, None)
    with peer.serving() as url:
        with pytest.raises(RuntimeError):
            peer.check(load_qualifier(), url)


@pytest.mark.parametrize('damage', ['chain', 'rpc-error', 'rpc-id'])
def test_f017_refuses_wrong_chain_and_invalid_rpc_envelopes(peer, damage):
    if damage == 'chain':
        peer.chain_id = '0x7e'
    elif damage == 'rpc-error':
        peer.rpc_error = True
    else:
        peer.rpc_id = 2
    with peer.serving() as url:
        with pytest.raises(RuntimeError):
            peer.check(load_qualifier(), url)
    assert [method for method, _ in peer.requests] == ['eth_chainId']


@pytest.mark.parametrize('refusal', ['pending', 'mismatched-block', 404, 503])
def test_f017_refusal_recovers_same_transfer_after_peer_and_client_reopen(peer, refusal):
    current = deepcopy(peer.status)
    if refusal == 'pending':
        peer.status = {'rung': 'submitted', 'block_number': None, 'committed_height': None}
    elif refusal == 'mismatched-block':
        peer.status['block_number'] += 1
    else:
        peer.http_status = refusal
    client = load_qualifier()
    with peer.serving() as url:
        with pytest.raises(HTTPError if isinstance(refusal, int) else RuntimeError):
            peer.check(client, url)
    del client
    before = deepcopy((peer.transaction, peer.receipt, peer.block))
    peer.status = current
    peer.http_status = 200
    with peer.serving() as url:
        assert peer.check(load_qualifier(), url) == peer.expected()
    assert (peer.transaction, peer.receipt, peer.block) == before
    assert [method for method, _ in peer.requests].count('eth_getTransactionByHash') == 2
    assert all(not method.startswith('eth_send') for method, _ in peer.requests)
