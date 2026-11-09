"""Task 22.33.886, audit 22.30.183 f041, criterion 22998.1.

The committed-copy and real-RPC tests prove the recorded producer, transport and
consumer path. Task 22.39.131 follows the chain-125 RPC producer for all V1-V3
cases, including the second V2 refusal added by 22.31.491. The committed corpus
still comes from the daemon recording. Every copy, drift and refusal check is
strict; the bounded hf-build/hf-done ledger records the verified revision.

The copy checker retains strict committed inclusion, signature, proof and key
equality, plus the complete recorded receipted/refusal case lists.
"""

import importlib.util
import json
from contextlib import contextmanager
from copy import deepcopy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import re
import runpy
import shutil
import sys
from threading import Thread

import pytest


HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location(
    "receipt_vector_copies", HERE / "check_receipt_vector_copies.py"
)
assert spec is not None and spec.loader is not None
vectors = importlib.util.module_from_spec(spec)
spec.loader.exec_module(vectors)
sys.path.insert(0, str(vectors.ROOT / "services/agent/sdk/python"))
from layerx_sdk.x402_rpc import CometRpc, HostedGatewayRpc, PaymentRpc


def read(root, path):
    return json.loads((root / path).read_text(encoding="utf-8"))


def write(root, path, document):
    (root / path).write_text(json.dumps(document), encoding="utf-8")


@pytest.fixture
def corpus(tmp_path):
    paths = [vectors.SOURCE, vectors.RETAINED_SOURCE, vectors.SDK, vectors.CONVERTER, vectors.EXPLORER,
             vectors.COMET, vectors.INTEROP, vectors.X402]
    for name in vectors.RETAINED:
        paths += ["sdk/platform/conformance/fixtures/" + name]
    for path in paths:
        destination = tmp_path / path
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(vectors.ROOT / path, destination)
    return tmp_path


def test_f041_committed_copies_match_recorded_producer(capsys):
    with pytest.raises(SystemExit) as result:
        runpy.run_path(str(HERE / "check_receipt_vector_copies.py"), run_name="__main__")
    assert result.value.code == 0
    message = capsys.readouterr().out
    assert vectors.SOURCE in message
    assert "Compared 27 receipt vector copies" in message
    assert vectors.RETAINED_SOURCE not in message


def test_f041_explorer_copy_refresh_uses_recorded_producer(corpus):
    original = read(corpus, vectors.EXPLORER)
    vectors.refresh_explorer_copy(corpus)
    refreshed = read(corpus, vectors.EXPLORER)
    assert refreshed["legacy_cases"] == original["legacy_cases"]
    assert refreshed["legacy_refusals"] == original["legacy_refusals"]
    assert [case["id"] for case in refreshed["cases"]] == [
        "committed-ok", "r7-height-below-receipt", "r8-commit-height-mismatch",
    ]
    assert [case["id"] for case in refreshed["refusals"]] == ["unrelated-valid-header"]
    assert refreshed["cases"][0]["settlement"] == read(corpus, vectors.SOURCE)["settlement"]
    vectors.check_receipt_vector_copies(corpus)


@contextmanager
def recorded_rpc(recording, explorer):
    settlement = recording["settlement"]
    requests = []
    replies = {
        "pax_sequencerReceipt": settlement["receipt"],
        "eth_getTransactionReceipt": explorer["cases"][0]["eth_receipt"],
        "px_getReceiptKeys": {"keys": [explorer["cases"][0]["pinnedKey"]]},
        "commit": explorer["cases"][0]["commit"],
        "validators": explorer["cases"][0]["validators"],
    }

    class RecordedNode(BaseHTTPRequestHandler):
        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            requests.append((self.path, self.headers["Content-Type"], request))
            response = json.dumps({"jsonrpc": "2.0", "id": request["id"],
                                   "result": replies[request["method"]]}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(response)))
            self.end_headers()
            self.wfile.write(response)

        def log_message(self, *_args):
            pass

    with ThreadingHTTPServer(("127.0.0.1", 0), RecordedNode) as server:
        thread = Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            endpoint = f"http://127.0.0.1:{server.server_port}"
            yield (PaymentRpc(endpoint + "/"), HostedGatewayRpc(endpoint + "/rpc"),
                   CometRpc(endpoint + "/")), requests
        finally:
            server.shutdown()
            thread.join(timeout=5)
            assert not thread.is_alive()


def test_f041_real_rpc_transport_feeds_copy_checker(corpus):
    recording = read(corpus, vectors.SOURCE)
    explorer = read(corpus, vectors.EXPLORER)
    interop = read(corpus, vectors.INTEROP)
    transaction = recording["settlement"]["receipt"]["txHash"]
    with recorded_rpc(recording, explorer) as ((client, gateway, comet), requests):
        receipt = client.receipt(transaction)
        eth_receipt = client.status(transaction)
        keys = gateway.receipt_keys()
        execution_commit = comet.call("commit", ["10"])
        validators = comet.call("validators", ["10"])
        for method in ("px_getReceiptKeys", "commit", "validators"):
            with pytest.raises(ValueError, match="^invalid-node-method$"):
                client.call(method, [])
        with pytest.raises(ValueError, match="^invalid-gateway-method$"):
            gateway.call("eth_getTransactionReceipt", [transaction])
        for method in ("px_getReceiptKeys", "eth_getTransactionReceipt"):
            with pytest.raises(ValueError, match="^invalid-comet-method$"):
                comet.call(method, [])
    assert receipt == recording["settlement"]["receipt"]
    assert receipt["rung"] == "committed"
    assert eth_receipt["blockHash"] == receipt["blockHash"]
    assert execution_commit["signed_header"]["commit"]["block_id"]["hash"].lower() == receipt["blockHash"][2:]
    assert keys[0].public_key == bytes(recording["authorization"]["PublicKey"])
    assert [request for _, _, request in requests] == [
        {"jsonrpc": "2.0", "id": index, "method": method, "params": params}
        for index, method, params in [
            (1, "pax_sequencerReceipt", [transaction]),
            (2, "eth_getTransactionReceipt", [transaction]),
            (1, "px_getReceiptKeys", []), (1, "commit", ["10"]), (2, "validators", ["10"]),
        ]
    ]
    assert [path for path, _, _ in requests] == ["/", "/", "/rpc", "/", "/"]
    assert all(content_type == "application/json" for _, content_type, _ in requests)
    interop["cases"][0]["receiptJson"] = receipt
    explorer["cases"][0].update(eth_receipt=eth_receipt, commit=execution_commit, validators=validators)
    write(corpus, vectors.INTEROP, interop)
    write(corpus, vectors.EXPLORER, explorer)
    vectors.check_receipt_vector_copies(corpus)
    interop["cases"][0]["receiptJson"]["blockHash"] = "0x" + "00" * 32
    write(corpus, vectors.INTEROP, interop)
    with pytest.raises(ValueError, match=r"receiptJson.blockHash"):
        vectors.check_receipt_vector_copies(corpus)


@pytest.mark.parametrize("path,location", [
    (vectors.SDK, ("provenance", "sha256")),
    (vectors.INTEROP, ("provenance", "sourceSha256")),
    (vectors.EXPLORER, ("source", "sha256")),
    (vectors.SDK, ("positive", "receiptInclusion", "TransactionProof", "Aunts", 0, 0)),
    (vectors.SDK, ("positive", "blockHeader", "DataHash")),
    (vectors.SDK, ("positive", "commit", "Signatures", 0, "Signature")),
    (vectors.SDK, ("positive", "receiptDigest")),
    (vectors.INTEROP, ("cases", 0, "receiptJson", "blockHash")),
    (vectors.INTEROP, ("cases", 0, "header", "height")),
    (vectors.INTEROP, ("cases", 0, "validators", 0, "voting_power")),
    (vectors.INTEROP, ("cases", 0, "receiptHeader")),
    (vectors.INTEROP, ("settlement", "transaction_proof", "data")),
    (vectors.INTEROP, ("cases", 3, "batchProof", "aunts", 0)),
    (vectors.EXPLORER, ("cases", 0, "eth_receipt", "blockHash")),
    (vectors.EXPLORER, ("cases", 0, "results_commit", "signed_header", "header", "last_results_hash")),
    (vectors.EXPLORER, ("cases", 0, "batch", "proof", "aunts", 0)),
    (vectors.EXPLORER, ("cases", 0, "receiptJson", "logs", 0, "data")),
    (vectors.EXPLORER, ("cases", 1, "eth_receipt", "blockNumber")),
    (vectors.X402, (3, "document", "extensions", "layerx", "signedReceipt")),
    (vectors.X402, (3, "document", "extensions", "layerx", "receiptDigest")),
    (vectors.X402, (3, "document", "extensions", "layerx", "verificationLevel")),
    (vectors.X402, (3, "document", "extensions", "layerx", "blockHash")),
    (vectors.X402, (3, "document", "amount")),
])
def test_f041_copy_drift_is_refused(corpus, path, location):
    document = read(corpus, path)
    parent = document
    for key in location[:-1]:
        parent = parent[key]
    key = location[-1]
    original = parent[key]
    parent[key] = original + 1 if type(original) is int else original + "00"
    write(corpus, path, document)
    with pytest.raises(ValueError, match=re.escape(path)):
        vectors.check_receipt_vector_copies(corpus)


@pytest.mark.parametrize("path,field,index", [
    (vectors.SDK, "refusals", 0),
    (vectors.INTEROP, "cases", 1), (vectors.INTEROP, "cases", 2), (vectors.INTEROP, "cases", 3),
    (vectors.EXPLORER, "cases", 1), (vectors.EXPLORER, "cases", 2),
    (vectors.EXPLORER, "refusals", 0),
    ("sdk/platform/conformance/fixtures/" + vectors.RETAINED[1], "cases", 0),
    ("sdk/platform/conformance/fixtures/" + vectors.RETAINED[1], "cases", 1),
    ("sdk/platform/conformance/fixtures/" + vectors.RETAINED[2], "cases", 0),
    ("sdk/platform/conformance/fixtures/" + vectors.RETAINED[2], "cases", 1),
    ("sdk/platform/conformance/fixtures/" + vectors.RETAINED[2], "cases", 2),
    ("sdk/platform/conformance/fixtures/" + vectors.RETAINED[2], "cases", 3),
    ("sdk/platform/conformance/fixtures/" + vectors.RETAINED[2], "cases", 4),
    ("sdk/platform/conformance/fixtures/" + vectors.RETAINED[2], "cases", 5),
])
def test_f041_refusal_coverage_cannot_be_removed(corpus, path, field, index):
    document = read(corpus, path)
    del document[field][index]
    write(corpus, path, document)
    with pytest.raises(ValueError, match=re.escape(path)):
        vectors.check_receipt_vector_copies(corpus)


@pytest.mark.parametrize("replacement", [[], None])
def test_f041_missing_commitment_is_still_refused(corpus, replacement):
    document = read(corpus, vectors.INTEROP)
    document["cases"][0]["commit"] = replacement
    write(corpus, vectors.INTEROP, document)
    with pytest.raises(ValueError, match=r"cases\[0\].commit"):
        vectors.check_receipt_vector_copies(corpus)


@pytest.mark.parametrize("operation", ["extra", "missing", "duplicate", "type"])
def test_f041_strict_copy_equality(corpus, operation):
    document = read(corpus, vectors.INTEROP)
    case = document["cases"][0]
    if operation == "extra":
        case["unexpected"] = True
    elif operation == "missing":
        del case["receiptJson"]["blockHash"]
    elif operation == "duplicate":
        document["cases"].append(deepcopy(case))
    else:
        case["header"]["version"]["app"] = False
    write(corpus, vectors.INTEROP, document)
    with pytest.raises(ValueError, match=vectors.INTEROP):
        vectors.check_receipt_vector_copies(corpus)


@pytest.mark.parametrize("operation", ["missing", "duplicate", "accept", "transaction"])
def test_f041_x402_refusal_cannot_be_removed_or_accepted(corpus, operation):
    document = read(corpus, vectors.X402)
    refusal = next(vector for vector in document if vector["name"] == "committed_without_proof")
    if operation == "missing":
        document.remove(refusal)
    elif operation == "duplicate":
        document.append(deepcopy(refusal))
    elif operation == "accept":
        refusal["expected"]["verdict"] = "accept"
    else:
        refusal["document"]["transaction"] = "0x" + "00" * 32
    write(corpus, vectors.X402, document)
    with pytest.raises(ValueError, match=vectors.X402):
        vectors.check_receipt_vector_copies(corpus)


@pytest.mark.parametrize("name,location,refused_field", [
    (vectors.RETAINED[0], ("cases", 0, "rawTransaction"), ":cases[0].rawTransaction"),
    (vectors.RETAINED[1], ("cases", 0, "receiptJson", "returnData"), ":cases[0].receiptJson.returnData"),
    (vectors.RETAINED[2], ("cases", 4, "receiptJson", "logs", 0, "data"), ":cases[4].receiptJson.logs[0].data"),
])
def test_f041_retained_transport_drift_is_refused(corpus, name, location, refused_field):
    path = "sdk/platform/conformance/fixtures/" + name
    document = read(corpus, path)
    parent = document
    for key in location[:-1]:
        parent = parent[key]
    parent[location[-1]] += "00"
    write(corpus, path, document)
    refusal = path + refused_field + ": differs from conformance source"
    with pytest.raises(ValueError, match="^" + re.escape(refusal) + "$"):
        vectors.check_receipt_vector_copies(corpus)
