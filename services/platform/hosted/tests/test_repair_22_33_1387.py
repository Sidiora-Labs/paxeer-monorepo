"""Native hosted topology and real SDK root/tenant HTTP contract regressions."""

from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
from pathlib import Path
import sys
import threading
from urllib.parse import urlsplit

import pytest

DIRECTORY = Path(__file__).resolve().parent
ROOT = DIRECTORY.parents[3]
SPEC = importlib.util.spec_from_file_location(
    "native_topology_regressions", DIRECTORY / "topology-regressions.py"
)
REGRESSIONS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(REGRESSIONS)

sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))
from layerx_sdk.chains import (  # noqa: E402
    ChainsClient, _decode_abi, _encode_abi, chain_rpc_url, chains_call,
    decode_chains_call, virtual_chain_id,
)
from layerx_sdk.exchange import encode_signature_call  # noqa: E402

ACCOUNT = "0x" + "11" * 32
ASSET = "0x" + "22" * 32
LEDGER = "0x000000000000000000000000000000000000101a"
BALANCE_CALL = encode_signature_call("balanceOf(bytes32,bytes32)", [ACCOUNT, ASSET])


@contextmanager
def rpc_peer():
    requests = []
    violations = []

    class NativeRpcPeer(BaseHTTPRequestHandler):
        def do_POST(self):
            try:
                assert self.headers.get_content_type() == "application/json"
                assert self.headers["Accept"] == "application/json"
                assert self.headers["User-Agent"] == "layerx-python/0.1.0"
                length = int(self.headers["Content-Length"])
                assert 0 < length <= 1024 * 1024
                raw = self.rfile.read(length)
                body = json.loads(raw)
                assert set(body) == {"jsonrpc", "id", "method", "params"}
                assert body["jsonrpc"] == "2.0" and type(body["id"]) is int
                assert body["method"] == "eth_call"
                call, block = body["params"]
                assert block == "latest" and set(call) == {"to", "data"}
                if self.path == "/":
                    expected = chains_call("chainIdOf", 7)
                    assert call == {"to": expected.to, "data": expected.data}
                    assert decode_chains_call(call["data"]) == ("chainIdOf", (7,))
                    result = "0x" + _encode_abi(("uint64",), (virtual_chain_id(7),)).hex()
                else:
                    assert self.path == "/chain/7", self.path
                    assert call == {"to": LEDGER, "data": BALANCE_CALL}
                    result = "0x" + _encode_abi(("uint128",), (90210,)).hex()
                requests.append((self.path, body, raw))
                identifier = body["id"] + (1 if self.server.wrong_id else 0)
                answer = json.dumps({"jsonrpc": "2.0", "id": identifier, "result": result}).encode()
                self.send_response(200)
                self.send_header("Content-Type", self.server.content_type)
            except (AssertionError, ValueError, KeyError, TypeError) as error:
                violations.append(repr(error))
                answer = json.dumps({"error": "RPC contract violation"}).encode()
                self.send_response(400)
                self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(answer)))
            self.end_headers()
            self.wfile.write(answer)

        def log_message(self, *_args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), NativeRpcPeer)
    server.wrong_id = False
    server.content_type = "application/json"
    worker = threading.Thread(target=server.serve_forever, daemon=True)
    worker.start()
    try:
        yield server, "http://127.0.0.1:%d" % server.server_port, requests, violations
    finally:
        server.shutdown()
        server.server_close()
        worker.join(timeout=5)
        assert not worker.is_alive()


def test_topology_retired_baseline(capsys):
    assert REGRESSIONS.main() == 0
    output = capsys.readouterr().out
    assert "PASS native root/tenant RPC topology and full-manifest parser parity" in output
    assert output.count("PASS refusal:") == 18
    assert output.count("PASS producer policy refusal:") == 6
    assert output.count("PASS short Service DNS ingress, egress and DNS refusals:") == 2
    assert output.count("PASS optional and required ConfigMap references:") == 2
    assert "PASS native-home sequence literal block and following fields" in output
    assert "PASS naming program deployment follows" in output


@pytest.mark.parametrize("parser", REGRESSIONS.PARSERS)
def test_native_rpc_producer_transport_consumer(parser):
    topology = REGRESSIONS.load(parser)
    assert not REGRESSIONS.failures(topology)
    origin = topology.configmaps[("layerx-testnet", "layerx-node-config")]["rpc-origin"]
    urls = [url for workload in topology.workloads
            for _, url in topology.resolved_env(workload) if url.startswith("http://")]
    assert len(urls) == 5
    with rpc_peer() as (_server, loopback, requests, violations):
        for url in urls:
            parsed = urlsplit(url)
            assert parsed.port == urlsplit(origin).port == 8545
            key, kind = REGRESSIONS.module["resolve_service_host"](
                parsed.hostname, "layerx-testnet", set())
            assert kind == "service"
            alias = topology.services[key]
            key, kind = REGRESSIONS.module["resolve_service_host"](
                alias["externalName"], key[0], set())
            assert (key, kind) == (("layerx-testnet", "layerx-native-chain"), "service")
            if parsed.path:
                assert parsed.path == urlsplit(chain_rpc_url(origin, 7)).path
                client = ChainsClient(loopback + parsed.path, timeout=2)
                result = client.request("eth_call", [{"to": LEDGER, "data": BALANCE_CALL}, "latest"])
                assert _decode_abi(("uint128",), bytes.fromhex(result[2:])) == (90210,)
            else:
                client = ChainsClient(loopback + "/", timeout=2)
                assert client.chain_id_of(7) == virtual_chain_id(7)
        assert not violations, violations
        assert [path for path, _, _ in requests] == ["/", "/chain/7", "/", "/", "/chain/7"]
        assert all(body["id"] == 1 for _, body, _ in requests)


@pytest.mark.parametrize("path", ("/", "/chain/7"))
@pytest.mark.parametrize("fault", ("wrong-id", "content-type"))
def test_native_rpc_transport_refuses_invalid_response(path, fault):
    with rpc_peer() as (server, loopback, requests, violations):
        server.wrong_id = fault == "wrong-id"
        server.content_type = "text/plain" if fault == "content-type" else "application/json"
        client = ChainsClient(loopback + path, timeout=2)
        expected = "invalid-px-envelope" if fault == "wrong-id" else "invalid-px-http-answer"
        with pytest.raises(ValueError, match=expected):
            if path == "/":
                client.chain_id_of(7)
            else:
                client.request("eth_call", [{"to": LEDGER, "data": BALANCE_CALL}, "latest"])
        assert len(requests) == 1 and not violations, violations


@pytest.mark.parametrize("parser", REGRESSIONS.PARSERS)
def test_native_missing_peer_and_dns_are_not_silently_external(parser):
    topology = REGRESSIONS.load(parser)
    del topology.services[("layerx-testnet", "native-rpc")]
    rows = REGRESSIONS.failures(topology)
    assert len(rows) == len(REGRESSIONS.RESOLVED_EDGES)
    assert all("no Service layerx-testnet/native-rpc" in detail for _, _, detail in rows)
    resolver = REGRESSIONS.module["resolve_service_host"]
    for host in ("api.invalid.example", "192.0.2.1", "bad_name"):
        with pytest.raises(ValueError):
            resolver(host, "layerx-testnet", set())
    for host in ("layerx-native-chain", "layerx-native-chain.layerx-testnet",
                 "layerx-native-chain.layerx-testnet.svc",
                 "layerx-native-chain.layerx-testnet.svc.cluster.local."):
        assert resolver(host, "layerx-testnet", set()) == (
            ("layerx-testnet", "layerx-native-chain"), "service")


@pytest.mark.parametrize("parser", REGRESSIONS.PARSERS)
def test_topology_cli_consumer_uses_native_inputs(parser, capsys):
    consumer = REGRESSIONS.module
    original = {name: consumer[name] for name in ("MODE", "STRICT", "MANIFESTS")}
    try:
        consumer["MODE"] = "pyyaml" if parser == "load_pyyaml" else "builtin"
        consumer["STRICT"] = True
        consumer["MANIFESTS"] = [("incorrect-default", str(REGRESSIONS.NATIVE_MANIFEST)),
                                 ("layerx-developer", str(REGRESSIONS.ADAPTER_FIXTURE))]
        assert consumer["main"]() == 0
        output = capsys.readouterr().out
        assert "ok=5 external=0 offcluster=0 note=0 failed=0" in output
        consumer["MANIFESTS"] = consumer["MANIFESTS"][:1]
        assert consumer["main"]() == 1
        assert "refusing to pass an empty topology" in capsys.readouterr().out
    finally:
        consumer.update(original)
