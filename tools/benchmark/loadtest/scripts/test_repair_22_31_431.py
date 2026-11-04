"""Replay consensus HTTP transcripts through the production metrics client."""

import importlib.util
import json
import threading
from datetime import datetime
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest
import requests


SCRIPTS = Path(__file__).resolve().parent
FIXTURES = SCRIPTS / "fixtures"
SPEC = importlib.util.spec_from_file_location("loadtest_metrics", SCRIPTS / "metrics.py")
metrics = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(metrics)
POPULATED_HEIGHT = 33716909
EMPTY_HEIGHT = 33716908
READERS = [metrics.get_block_info, metrics.get_transaction_breakdown]


def transcript(name):
    return (FIXTURES / f"consensus_block_{name}.json").read_bytes()


@pytest.fixture
def consensus_http(monkeypatch):
    routes = {}
    received = []

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            received.append((self.command, self.path))
            status, body = routes.get(self.path, (404, b"Unknown recorded request"))
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, format, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01})
    thread.start()
    monkeypatch.setattr(metrics, "rpc_base_url", f"http://127.0.0.1:{server.server_port}")
    try:
        yield routes, received
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()


def test_benchmark_rpc_envelope_populated_block(consensus_http):
    routes, received = consensus_http
    path = f"/block?height={POPULATED_HEIGHT}"
    routes[path] = (200, transcript("populated"))

    assert metrics.get_block_info(POPULATED_HEIGHT) == {
        "height": POPULATED_HEIGHT,
        "timestamp": datetime(2026, 11, 4, 2, 47, 17, 920941),
        "number_of_txs": 2,
    }
    assert metrics.get_transaction_breakdown(POPULATED_HEIGHT) == {"staking": 1, "bank": 1}
    assert metrics.get_block_time(POPULATED_HEIGHT) == datetime(
        2026, 11, 4, 2, 47, 17, 920941
    )
    assert received == [("GET", path)] * 3


@pytest.mark.parametrize("null_txs", [False, True], ids=["empty-array", "null-array"])
def test_benchmark_rpc_envelope_empty_block(consensus_http, null_txs):
    routes, received = consensus_http
    path = f"/block?height={EMPTY_HEIGHT}"
    body = transcript("empty")
    assert body.count(b'"txs":[]') == 1
    if null_txs:
        # consensus/libs/json/encoder.go emits a nil transaction slice as null.
        body = body.replace(b'"txs":[]', b'"txs":null')
    routes[path] = (200, body)

    info = metrics.get_block_info(EMPTY_HEIGHT)
    assert info["height"] == EMPTY_HEIGHT
    assert info["timestamp"] == datetime(2026, 11, 4, 2, 47, 16, 51308)
    assert info["number_of_txs"] == 0
    assert metrics.get_transaction_breakdown(EMPTY_HEIGHT) == {}
    assert received == [("GET", path)] * 2


def test_benchmark_rpc_envelope_block_statistics(consensus_http):
    routes, received = consensus_http
    empty_path = f"/block?height={EMPTY_HEIGHT}"
    next_path = f"/block?height={POPULATED_HEIGHT}"
    routes[empty_path] = (200, transcript("empty"))
    routes[next_path] = (200, transcript("populated"))

    block = metrics.get_block_info(EMPTY_HEIGHT)
    assert metrics.get_best_block_stats([block]) == (0, EMPTY_HEIGHT, 1869)
    assert received == [("GET", empty_path), ("GET", next_path)]


@pytest.mark.parametrize("reader", READERS, ids=lambda reader: reader.__name__)
@pytest.mark.parametrize("status", [400, 500, 503])
def test_benchmark_rpc_envelope_http_errors(consensus_http, reader, status):
    routes, received = consensus_http
    path = f"/block?height={POPULATED_HEIGHT}"
    routes[path] = (status, b"upstream unavailable")

    with pytest.raises(requests.HTTPError) as raised:
        reader(POPULATED_HEIGHT)
    assert raised.value.response.status_code == status
    assert received == [("GET", path)]


@pytest.mark.parametrize("reader", READERS, ids=lambda reader: reader.__name__)
def test_benchmark_rpc_envelope_jsonrpc_errors(consensus_http, reader):
    routes, received = consensus_http
    height = 9999999999999
    path = f"/block?height={height}"
    routes[path] = (200, transcript("error"))

    with pytest.raises(RuntimeError, match="Block JSON-RPC error -32603") as raised:
        reader(height)
    assert "Internal error" in str(raised.value)
    assert "height 9999999999999 must be less than or equal" in str(raised.value)
    assert received == [("GET", path)]


@pytest.mark.parametrize("reader", READERS, ids=lambda reader: reader.__name__)
@pytest.mark.parametrize(
    "body",
    [
        b"not JSON",
        b"[]",
        b'{"jsonrpc":"1.0","result":{}}',
        b'{"jsonrpc":"2.0","result":null}',
        b'{"jsonrpc":"2.0","result":{"block":null}}',
        b'{"jsonrpc":"2.0","error":"invalid error"}',
    ],
)
def test_benchmark_rpc_envelope_invalid_responses(consensus_http, reader, body):
    routes, received = consensus_http
    path = f"/block?height={POPULATED_HEIGHT}"
    routes[path] = (200, body)

    with pytest.raises(ValueError):
        reader(POPULATED_HEIGHT)
    assert received == [("GET", path)]


@pytest.mark.parametrize("reader", READERS, ids=lambda reader: reader.__name__)
@pytest.mark.parametrize("txs", ["not-an-array", {}, [42], False])
def test_benchmark_rpc_envelope_invalid_transactions(consensus_http, reader, txs):
    routes, received = consensus_http
    path = f"/block?height={POPULATED_HEIGHT}"
    body = json.loads(transcript("populated"))
    body["result"]["block"]["data"]["txs"] = txs
    routes[path] = (200, json.dumps(body).encode())

    with pytest.raises(ValueError, match="Invalid result.block.data.txs"):
        reader(POPULATED_HEIGHT)
    assert received == [("GET", path)]


@pytest.mark.parametrize("reader", READERS, ids=lambda reader: reader.__name__)
def test_benchmark_rpc_envelope_missing_transactions(consensus_http, reader):
    routes, received = consensus_http
    path = f"/block?height={POPULATED_HEIGHT}"
    body = json.loads(transcript("populated"))
    del body["result"]["block"]["data"]["txs"]
    routes[path] = (200, json.dumps(body).encode())

    with pytest.raises(ValueError, match="Missing result.block.data.txs"):
        reader(POPULATED_HEIGHT)
    assert received == [("GET", path)]
