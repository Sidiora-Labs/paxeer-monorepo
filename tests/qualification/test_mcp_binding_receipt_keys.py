import importlib.util
import json
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from threading import Thread

import pytest


PUBLIC_KEY = '0x' + 'Ab' * 32
RECEIPT_KEY = {
    'keyId': '0x' + '12' * 32,
    'publicKey': PUBLIC_KEY,
    'firstHeight': 0,
    'lastHeight': 100,
}


@pytest.fixture(scope='module')
def binding_listener():
    path = (Path(__file__).resolve().parents[2]
            / 'tools/qualification/paxeer-x/mcp_binding_listener.py')
    spec = importlib.util.spec_from_file_location('mcp_binding_listener', path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


@contextmanager
def receipt_keys_rpc(result, index='0'):
    requests = []

    class ReceiptKeysHandler(BaseHTTPRequestHandler):
        def do_POST(self):
            requests.append(json.loads(self.rfile.read(int(self.headers['Content-Length']))))
            body = json.dumps({'jsonrpc': '2.0', 'id': 1, 'result': result}).encode()
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, format, *args):
            pass

    with HTTPServer(('127.0.0.1', 0), ReceiptKeysHandler) as server:
        thread = Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            yield {
                'LAYERX_AGENT_NODE_ENDPOINT': f'http://127.0.0.1:{server.server_port}',
                'LAYERX_AGENT_MCP_WEB_RECEIPT_KEY_INDEX': index,
            }
        finally:
            server.shutdown()
            thread.join()
    assert requests == [{'jsonrpc': '2.0', 'id': 1,
                         'method': 'px_getReceiptKeys', 'params': []}]


@pytest.mark.parametrize('index', ['0', '1'])
def test_web_receipt_public_key_accepts_gateway_result(binding_listener, index):
    other_key = dict(RECEIPT_KEY, publicKey='0x' + 'cd' * 32)
    keys = [RECEIPT_KEY, other_key] if index == '0' else [other_key, RECEIPT_KEY]
    with receipt_keys_rpc({'keys': keys}, index) as daemon:
        assert binding_listener.web_receipt_public_key(daemon) == 'ab' * 32


def test_web_receipt_public_key_refuses_bare_list(binding_listener):
    with receipt_keys_rpc([RECEIPT_KEY]) as daemon:
        with pytest.raises(binding_listener.GateFailure,
                           match='^receipt key RPC response is malformed$'):
            binding_listener.web_receipt_public_key(daemon)


def test_web_receipt_public_key_refuses_extra_result_key(binding_listener):
    with receipt_keys_rpc({'keys': [RECEIPT_KEY], 'extra': True}) as daemon:
        with pytest.raises(binding_listener.GateFailure,
                           match='^receipt key RPC response is malformed$'):
            binding_listener.web_receipt_public_key(daemon)


def test_web_receipt_public_key_refuses_missing_keys(binding_listener):
    with receipt_keys_rpc({}) as daemon:
        with pytest.raises(binding_listener.GateFailure,
                           match='^receipt key RPC response is malformed$'):
            binding_listener.web_receipt_public_key(daemon)


@pytest.mark.parametrize('keys,index', [
    (None, '0'),
    ({}, '0'),
    ([], '0'),
    ([RECEIPT_KEY], '-1'),
    ([RECEIPT_KEY], '1'),
])
def test_web_receipt_public_key_refuses_missing_entry(binding_listener, keys, index):
    with receipt_keys_rpc({'keys': keys}, index) as daemon:
        with pytest.raises(binding_listener.GateFailure,
                           match='^receipt key index has no px_getReceiptKeys entry$'):
            binding_listener.web_receipt_public_key(daemon)


@pytest.mark.parametrize('public_key', [
    None, 123, 'ab' * 32, '0x' + 'gg' * 32, '0x' + 'ab' * 31, '0x' + 'ab' * 33,
])
def test_web_receipt_public_key_refuses_malformed_key(binding_listener, public_key):
    with receipt_keys_rpc({'keys': [dict(RECEIPT_KEY, publicKey=public_key)]}) as daemon:
        with pytest.raises(binding_listener.GateFailure,
                           match='^receipt public key is malformed$'):
            binding_listener.web_receipt_public_key(daemon)
