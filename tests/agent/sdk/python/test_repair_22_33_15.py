from __future__ import annotations

import importlib
import importlib.util
import json
import runpy
import sys
import zipfile
from dataclasses import replace
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Thread
from types import SimpleNamespace

import pytest


ROOT = Path(__file__).resolve().parents[4]
SDK_ROOT = ROOT / "services/agent/sdk/python"
VECTORS = json.loads(
    (ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text()
)
ACCEPTED = [case for case in VECTORS["cases"] if case["vector"] == "V1"]


def _bytes(value: str) -> bytes:
    return bytes.fromhex(value.removeprefix("0x"))


@pytest.fixture(scope="module")
def installed_example(tmp_path_factory):
    destination = tmp_path_factory.mktemp("installed-offline-example")
    spec = importlib.util.spec_from_file_location(
        "receipt_example_build_backend", SDK_ROOT / "build_backend.py"
    )
    backend = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(backend)
    wheel = destination / backend.build_wheel(str(destination))
    site = destination / "lib/site-packages"
    with zipfile.ZipFile(wheel) as archive:
        for name in archive.namelist():
            if ".data/data/" in name:
                target = destination / name.split(".data/data/", 1)[1]
            else:
                target = site / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(archive.read(name))

    saved_path = sys.path[:]
    saved_modules = {
        name: module
        for name, module in sys.modules.items()
        if name == "layerx_sdk" or name.startswith("layerx_sdk.")
    }
    for name in saved_modules:
        del sys.modules[name]
    sys.path.insert(0, str(site))
    try:
        sdk = importlib.import_module("layerx_sdk")
        rpc = importlib.import_module("layerx_sdk.x402_rpc")
        path = destination / "share/layerx-sdk/examples/offline_receipt_verification.py"
        example = runpy.run_path(str(path))
        assert Path(sdk.__file__).is_relative_to(site)
        assert example["verify_sequencer_receipt"] is sdk.verify_sequencer_receipt
        assert path.read_bytes() == (
            SDK_ROOT / "examples/offline_receipt_verification.py"
        ).read_bytes()
        yield SimpleNamespace(
            sdk=sdk, rpc=rpc, verify=example["offline_receipt_verification"]
        )
    finally:
        for name in tuple(sys.modules):
            if name == "layerx_sdk" or name.startswith("layerx_sdk."):
                del sys.modules[name]
        sys.modules.update(saved_modules)
        sys.path[:] = saved_path


@pytest.fixture
def recorded_server():
    requests = []
    by_hash = {case["receiptJson"]["txHash"]: case for case in ACCEPTED}
    by_raw = {case["rawTransaction"]: case for case in ACCEPTED}

    class RecordedReceiptHandler(BaseHTTPRequestHandler):
        def do_POST(self):
            assert self.headers.get_content_type() == "application/json"
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            assert self.path == ("/rpc" if request["method"] == "px_getReceiptKeys" else "/")
            assert set(request) == {"jsonrpc", "id", "method", "params"}
            assert request["jsonrpc"] == "2.0"
            assert type(request["id"]) is int
            requests.append(request)
            if request["method"] == "eth_sendRawTransaction":
                assert len(request["params"]) == 1
                result = by_raw[request["params"][0]]["receiptJson"]["txHash"]
            elif request["method"] == "pax_sequencerReceipt":
                assert len(request["params"]) == 1
                result = by_hash[request["params"][0]]["receiptJson"]
            else:
                assert request["method"] == "px_getReceiptKeys"
                assert request["params"] == []
                result = {"keys": [ACCEPTED[0]["pinnedKey"]]}
            body = json.dumps(
                {"jsonrpc": "2.0", "id": request["id"], "result": result}
            ).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, format, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), RecordedReceiptHandler)
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield (f"http://127.0.0.1:{server.server_port}/",
               f"http://127.0.0.1:{server.server_port}/rpc"), requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()


def _pin(sdk, vector):
    key = vector["pinnedKey"]
    return sdk.ReceiptKey(
        _bytes(key["keyId"]),
        _bytes(key["publicKey"]),
        key["firstHeight"],
        key["lastHeight"],
    )


@pytest.mark.parametrize("vector", ACCEPTED, ids=lambda vector: vector["id"])
def test_offline_example_installed_receipt_round_trip(
    installed_example, recorded_server, vector
):
    assert VECTORS["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    (node_endpoint, gateway_endpoint), requests = recorded_server
    client = installed_example.rpc.PaymentRpc(node_endpoint)
    transaction_hash = client.send(vector["rawTransaction"])
    receipt = client.receipt(transaction_hash)
    keys = installed_example.rpc.HostedGatewayRpc(gateway_endpoint).receipt_keys()
    assert keys == (_pin(installed_example.sdk, vector),)
    assert receipt == vector["receiptJson"]
    assert receipt["receipt"] == vector["signedReceipt"]
    verified = installed_example.verify(
        _bytes(receipt["receipt"]), _bytes(vector["rawTransaction"]), keys
    )
    assert isinstance(verified, installed_example.sdk.SequencerReceiptVerification)
    assert verified.level == "sequencer-signed"
    assert verified.height == int(receipt["height"], 16)
    assert verified.canonical_bytes == _bytes(vector["signedReceipt"])
    assert verified.receipt_digest == _bytes(vector["receiptDigest"])
    assert verified.receipt.activity_id == _bytes(transaction_hash)
    assert verified.receipt.context_hash == keys[0].key_id
    assert [request["method"] for request in requests] == [
        "eth_sendRawTransaction",
        "pax_sequencerReceipt",
        "px_getReceiptKeys",
    ]
    assert [request["id"] for request in requests] == [1, 2, 1]


@pytest.mark.parametrize("boundary", ["first_height", "last_height"])
def test_offline_example_inclusive_key_ranges(installed_example, boundary):
    vector = ACCEPTED[0]
    key = _pin(installed_example.sdk, vector)
    height = int(vector["receiptJson"]["height"], 16)
    key = replace(key, **{boundary: height})
    result = installed_example.verify(
        _bytes(vector["signedReceipt"]), _bytes(vector["rawTransaction"]), [key]
    )
    assert result.height == height
    assert result.receipt_digest == _bytes(vector["receiptDigest"])


@pytest.mark.parametrize(
    "change,check",
    [
        ("no-pins", "unpinned_key"),
        ("wrong-id", "unpinned_key"),
        ("not-yet-active", "key_out_of_range"),
        ("expired", "key_out_of_range"),
        ("wrong-public-key", "bad_signature"),
        ("changed-transaction", "hash_mismatch"),
        ("transaction-hash-as-raw", "hash_mismatch"),
        ("tampered-receipt", "bad_signature"),
    ],
)
def test_offline_example_preserves_verifier_refusals(installed_example, change, check):
    vector = ACCEPTED[0]
    signed = _bytes(vector["signedReceipt"])
    raw = _bytes(vector["rawTransaction"])
    key = _pin(installed_example.sdk, vector)
    height = int(vector["receiptJson"]["height"], 16)
    keys = [key]
    if change == "no-pins":
        keys = []
    elif change == "wrong-id":
        keys = [replace(key, key_id=b"\x2a" * 32)]
    elif change == "not-yet-active":
        keys = [replace(key, first_height=height + 1)]
    elif change == "expired":
        keys = [replace(key, first_height=height - 2, last_height=height - 1)]
    elif change == "wrong-public-key":
        keys = [replace(key, public_key=b"\x2a" * 32)]
    elif change == "changed-transaction":
        raw = _bytes(ACCEPTED[1]["rawTransaction"])
    elif change == "transaction-hash-as-raw":
        raw = _bytes(vector["receiptJson"]["txHash"])
    elif change == "tampered-receipt":
        signed = signed[:-1] + bytes([signed[-1] ^ 1])
    with pytest.raises(installed_example.sdk.SequencerReceiptVerificationError) as raised:
        installed_example.verify(signed, raw, keys)
    assert raised.value.check == check
    assert raised.value.code == installed_example.sdk.SdkErrorCode.VERIFICATION_FAILURE
