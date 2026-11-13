"""22.31.318: exercise production qualification against node-produced transcripts.

Finding 3: test_program_receipt_reopens_and_retries and test_paid_success_reopens
cross HTTP, the current signed receipt codec, pinned verification and the consumers.
Finding 4: test_program_call_signed_evm_round_trip and test_deploy_and_web_encodings
cross real signing, ABI encoding, EVM decoding, submission and receipt hashing.
The transcripts are unmodified output of rpc/sequencer_receipt_vectors_test.go.
No subprocess, live service, credential, native build, or receipt re-signing is used.
"""

import base64
from contextlib import contextmanager
from copy import deepcopy
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import io
import json
from pathlib import Path
import struct
import sys
import threading

import pytest

ROOT = Path(__file__).resolve().parents[4]
QUAL = ROOT / "tools/qualification/paxeer-x"
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))
from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import decode_signed, encode_call
from layerx_sdk.verifier import SequencerReceiptVerificationError

VECTORS = ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
SECRET = "8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63"
DID = "did:layerx:" + "11" * 32
AUTH = "LayerX-Key repair:lxp_live_" + "12" * 32
PROGRAMS = bytes.fromhex("00" * 18 + "101d")
WEB = bytes.fromhex("00" * 18 + "1019")


def production():
    spec = importlib.util.spec_from_file_location("paid_web_repair", QUAL / "paid-web-contract.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def vector(name):
    corpus = json.loads(VECTORS.read_text())
    assert corpus["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    return deepcopy(next(case for case in corpus["cases"] if case["id"] == name))


def gateway_config(case):
    key = case["pinnedKey"]
    return {"receipt_keys": [{
        "key_id": key["keyId"][2:], "public_key": key["publicKey"][2:],
        "first_height": key["firstHeight"], "last_height": key["lastHeight"],
    }]}


def write_private(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    if isinstance(value, bytes):
        path.write_bytes(value)
    else:
        path.write_text(value if isinstance(value, str) else json.dumps(value))
    path.chmod(0o600)
    return path


class RecordedNode:
    """Replay node JSON only for its exact recorded signed bytes; reopen accepted sends."""

    def __init__(self, case, state):
        self.case = deepcopy(case)
        self.state = state
        self.accepted = json.loads(state.read_text()) if state.exists() else {}
        self.requests = []
        self.contents = {}
        self.errors = []
        raw = bytes.fromhex(case["rawTransaction"][2:])
        tx, signature = decode_signed(raw)
        self.signer = tx.recover_signer(signature)
        self.nonce = tx.nonce
        self.chain_id = 125

    def dispatch(self, request):
        method, params = request["method"], request["params"]
        self.requests.append((method, params))
        if method == "eth_chainId":
            assert params == []
            return hex(self.chain_id)
        if method == "eth_call":
            expected = encode_call("getEvmAddrByLayerX", ["bytes32"], [bytes.fromhex(DID.split(":")[-1])])
            assert params == [{"to": "0x" + "00" * 18 + "1004", "data": "0x" + expected.hex()}, "pending"]
            return "0x" + "00" * 12 + self.signer[2:].lower()
        if method == "eth_getTransactionCount":
            assert params[0].lower() == self.signer.lower() and params[1] == "pending"
            return hex(self.nonce)
        if method == "eth_sendRawTransaction":
            raw = bytes.fromhex(params[0][2:])
            tx, signature = decode_signed(raw)
            assert tx.chain_id == 125 and tx.value == 0
            assert tx.recover_signer(signature).lower() == self.signer.lower()
            tx_hash = "0x" + keccak256(raw).hex()
            if tx_hash in self.accepted:
                assert self.accepted[tx_hash] == params[0]
            self.accepted[tx_hash] = params[0]
            write_private(self.state, self.accepted)
            return tx_hash
        if method == "pax_sequencerReceipt":
            assert params == [self.case["receiptJson"]["txHash"]]
            # Never synthesize a receipt for a newly generated transaction.
            raw = self.accepted.get(params[0])
            assert raw is None or raw == self.case["rawTransaction"]
            return deepcopy(self.case["receiptJson"])
        raise AssertionError(method)

    @contextmanager
    def serving(self):
        peer = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                try:
                    assert self.path == "/"
                    assert self.headers["Content-Type"] == "application/json"
                    assert self.headers["Authorization"] == AUTH
                    request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                    assert set(request) == {"jsonrpc", "id", "method", "params"}
                    assert request["jsonrpc"] == "2.0" and request["id"] == 1
                    result = peer.dispatch(request)
                    data = json.dumps({"jsonrpc": "2.0", "id": request["id"], "result": result}).encode()
                    self.send_response(200)
                except Exception as error:
                    peer.errors.append(error)
                    data = json.dumps({"error": str(error)}).encode()
                    self.send_response(500)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            def do_GET(self):
                data = peer.contents[self.path]
                self.send_response(200)
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            yield f"http://127.0.0.1:{server.server_port}/"
        finally:
            server.shutdown()
            server.server_close()
            thread.join()
        assert not self.errors


def kernel(module, tmp_path, url, case):
    harness = module.KernelHarness.__new__(module.KernelHarness)
    harness.gateway = module.endpoint(url)
    harness.authorization_file = write_private(tmp_path / "authorization", AUTH)
    harness.primary = {"gateway": gateway_config(case)}
    harness.network = 9
    harness.signing_key = write_private(tmp_path / "signing-key", SECRET)
    harness.caller = DID
    harness.revision = "61b2c3c98c61e72d831e1ce948a0bde6d218c980"
    harness.journal = io.StringIO()
    harness.count = 0
    tx, _ = decode_signed(bytes.fromhex(case["rawTransaction"][2:]))
    harness.m = {"activity_fee_limit": "100000000", "evm_fees": {
        "gas_limit": tx.gas_limit, "max_fee_per_gas": tx.max_fee_per_gas,
        "max_priority_fee_per_gas": tx.max_priority_fee_per_gas,
    }}
    harness.program = tx.data[4:36]
    return harness


def compact_call(program):
    entry = b"layerx_call"
    access = b"LayerX/programs/access-declaration/v1\0\0"
    budget = [1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096]
    return struct.pack(">32sHHIHII7Q", program, 1, len(entry), 0, 2, len(access), 64, *budget) + entry + b"\0\0" + access


def test_program_call_signed_evm_round_trip(tmp_path):
    module = production()
    case = vector("V1-layerxprograms-call")
    node = RecordedNode(case, tmp_path / "node-sends.json")
    with node.serving() as url:
        harness = kernel(module, tmp_path, url, case)
        harness.calls = {"request": compact_call(harness.program)}
        signed = harness.program_call("request")
        # RFC6979 signing reproduces the transaction the real node executed.
        assert "0x" + signed.hex() == case["rawTransaction"]
        facts, result = harness.receipt(signed)
        status, kind, response = module.program_result(result)
        assert (status, kind) == (0, 1) and response
        assert facts.module_id == 12 and facts.operation == 2
        assert facts.from_balance_before - facts.from_balance_after == facts.fee_charged
        assert facts.to_balance_before == facts.to_balance_after
        assert len(node.accepted) == 1
        assert any(method == "eth_sendRawTransaction" for method, _ in node.requests)
        assert any(method == "pax_sequencerReceipt" for method, _ in node.requests)


def test_program_receipt_reopens_and_retries(tmp_path):
    module = production()
    case = vector("V1-layerxprograms-call")
    raw = bytes.fromhex(case["rawTransaction"][2:])
    state = tmp_path / "accepted.json"
    node = RecordedNode(case, state)
    with node.serving() as url:
        harness = kernel(module, tmp_path, url, case)
        harness.send(raw)
        before = harness.receipt(raw)
        persisted = state.read_bytes()
    del harness, module, node
    module = production()
    reopened = RecordedNode(case, state)
    with reopened.serving() as url:
        harness = kernel(module, tmp_path, url, case)
        assert harness.receipt(raw) == before
        harness.send(raw)
        assert harness.receipt(raw) == before
        assert state.read_bytes() == persisted
        assert len(reopened.accepted) == 1
        facts, _ = before
        assert facts.from_balance_before - facts.from_balance_after == facts.fee_charged


@pytest.mark.parametrize("name,check", [
    ("V3-key-not-pinned", "unpinned_key"),
    ("V3-height-outside-key-range", "key_out_of_range"),
    ("V3-activity-id-not-raw-hash", "hash_mismatch"),
    ("V3-non-canonical-bytes", "bad_signature"),
])
def test_recorded_signature_refusals(tmp_path, name, check):
    module = production()
    case = vector(name)
    node = RecordedNode(case, tmp_path / "accepted.json")
    with node.serving() as url:
        harness = kernel(module, tmp_path, url, case)
        # The malformed activity-id vector is answered by the node under its raw hash.
        row = module.rpc(harness.gateway, "pax_sequencerReceipt", [case["receiptJson"]["txHash"]], harness.headers())
        with pytest.raises(SequencerReceiptVerificationError) as error:
            module.verified_execution(row, bytes.fromhex(case["rawTransaction"][2:]), harness.primary["gateway"])
        assert error.value.check == check


@pytest.mark.parametrize("field", ["signingKey", "keyId", "status", "nonce", "returnData", "gasUsed", "feeAmount", "vmError", "paymentDenom"])
def test_node_json_must_match_signed_bytes(tmp_path, field):
    module = production()
    case = vector("V1-layerxprograms-call")
    case["receiptJson"][field] = "changed" if field in ("vmError", "paymentDenom") else "0x" + "ff" * 32
    node = RecordedNode(case, tmp_path / "accepted.json")
    with node.serving() as url:
        harness = kernel(module, tmp_path, url, case)
        with pytest.raises(module.Refusal):
            harness.receipt(bytes.fromhex(case["rawTransaction"][2:]))


def test_recorded_tampered_log_refused(tmp_path):
    module = production()
    case = vector("V3-tampered-log")
    node = RecordedNode(case, tmp_path / "accepted.json")
    with node.serving() as url:
        harness = kernel(module, tmp_path, url, case)
        with pytest.raises(module.Refusal, match="log differs"):
            harness.receipt(bytes.fromhex(case["rawTransaction"][2:]))


def test_reverted_execution_uses_declared_result(tmp_path):
    module = production()
    case = vector("V2-layerxledger-revert")
    node = RecordedNode(case, tmp_path / "accepted.json")
    with node.serving() as url:
        harness = kernel(module, tmp_path, url, case)
        raw = bytes.fromhex(case["rawTransaction"][2:])
        facts, result = harness.receipt(raw, refused=True)
        assert facts.result_code == 1 and result[:4] == bytes.fromhex("08c379a0")
        with pytest.raises(module.Refusal, match="outcome mismatch"):
            harness.receipt(raw)


def test_deploy_and_web_encodings(tmp_path):
    module = production()
    case = vector("V1-layerxprograms-call")
    node = RecordedNode(case, tmp_path / "accepted.json")
    with node.serving() as url:
        harness = kernel(module, tmp_path, url, case)
        harness.wasm = b"\0asm\1\0\0\0"
        harness.m["reference_program"] = {
            "policy": 1, "authority": "22" * 32, "interface_encoding": "00",
        }
        abi = json.loads((ROOT / "precompiles/layerxprograms/abi.json").read_text())
        deploy = next(method for method in abi if method.get("name") == "deploy")
        encoded = encode_call("deploy", [entry["type"] for entry in deploy["inputs"]], [
            harness.program, 4, 1, bytes.fromhex("22" * 32),
            hashlib.sha256(harness.wasm).digest(), bytes.fromhex("00"), harness.wasm,
        ])
        assert harness.deployment_call() == encoded
        raw = harness.signed(0x00090001, encoded, harness.signing_key, DID)
        tx, signature = decode_signed(raw)
        assert tx.chain_id == 125 and tx.to == PROGRAMS and tx.data == encoded
        assert tx.recover_signer(signature) == node.signer
        harness.send(raw)
        assert node.accepted[module.transaction_hash(raw)] == "0x" + raw.hex()
        with pytest.raises(module.Refusal, match="artifact binding"):
            harness.signed(0x00090001, encoded[:-1] + b"\1", harness.signing_key, DID)
        envelope = bytes.fromhex((ROOT / "tests/fixtures/web/observation-activity.hex").read_text().strip())
        cursor = 5
        fields = {}
        for tag, width in enumerate((2, 4, 4, -1, -1, 8, 16, -1, 16, -1, -1, -1), 1):
            assert envelope[cursor] == tag
            cursor += 1
            if width == -1:
                width = int.from_bytes(envelope[cursor:cursor + 4], "big")
                cursor += 4
            fields[tag] = envelope[cursor:cursor + width]
            cursor += width
        assert cursor == len(envelope)
        observation = fields[11]
        # This is the unmodified producer payload also decoded by the Go xweb tests.
        assert observation[0] == 2 and len(observation) == 358
        web_abi = json.loads((ROOT / "precompiles/xweb/abi.json").read_text())
        fulfil = next(method for method in web_abi if method.get("name") == "fulfilProgram")
        target, calldata = harness.evm_call(0x000B0001, observation)
        assert target == WEB
        assert calldata == encode_call("fulfilProgram", [entry["type"] for entry in fulfil["inputs"]], [observation])
        raw = harness.signed(0x000B0001, observation, harness.signing_key, DID)
        tx, _ = decode_signed(raw)
        assert tx.chain_id == 125 and tx.to == WEB and tx.data == calldata and tx.value == 0
        harness.send(raw)
        assert node.accepted[module.transaction_hash(raw)] == "0x" + raw.hex()
        with pytest.raises(module.Refusal, match="chain-125 node"):
            node.chain_id = 7
            harness.signed(0x000B0001, observation, harness.signing_key, DID)


def test_deployment_receipt_rejects_call_transaction(tmp_path):
    module = production()
    case = vector("V1-layerxprograms-call")
    node = RecordedNode(case, tmp_path / "accepted.json")
    with node.serving() as url:
        harness = kernel(module, tmp_path, url, case)
        harness.deployment = bytes.fromhex(case["rawTransaction"][2:])
        harness.wasm = b"\0asm\1\0\0\0"
        harness.m.update({"reference_program": {"policy": 1, "authority": "22" * 32, "interface_encoding": "00"}, "deployment_sender": node.signer})
        with pytest.raises(module.Refusal, match="reference artifact"):
            harness.reference_deployment()
        assert not node.requests


def test_operational_observation_rejects_foreign_execution(tmp_path):
    module = production()
    case = vector("V1-layerxprograms-call")
    node = RecordedNode(case, tmp_path / "accepted.json")
    with node.serving() as url:
        harness = module.OperationalHarness.__new__(module.OperationalHarness)
        harness.gateway = module.endpoint(url)
        harness.authorization_file = write_private(tmp_path / "authorization", AUTH)
        harness.config = {"gateway": gateway_config(case)}
        harness.observation = {"activity_id": case["receiptJson"]["txHash"][2:], "raw_transaction": case["rawTransaction"][2:]}
        # A valid signed program execution cannot stand in for a web fulfilment.
        with pytest.raises(module.Refusal, match="observation receipt facts"):
            harness.observed()
        assert node.requests[0][0] == "pax_sequencerReceipt"


def paid_store(module, path, case):
    row = case["receiptJson"]
    log = row["logs"][0]
    data = bytes.fromhex(log["data"][2:])
    receipt = case["receiptDigest"][2:]
    text, url, media = b"recorded paid content", b"https://example.test/article", b"text/plain"
    raw = module.DOMAIN + b"\1" + len(url).to_bytes(4, "big") + url
    raw += len(media).to_bytes(4, "big") + media + len(text).to_bytes(8, "big") + text
    content = keccak256(raw).hex()
    body = json.dumps({"text": text.decode(), "url": url.decode(), "media_type": media.decode(), "length": len(text), "digest": content}).encode()
    resource = "GET /fetch?url=https%3A%2F%2Fexample.test%2Farticle"
    key = "33" * 32
    payment = {
        "principal": "paid-reader", "request_digest": "44" * 32, "resource": resource,
        "payer": log["topics"][2][2:], "raw_transaction": case["rawTransaction"][2:],
        "tx_hash": row["txHash"][2:], "receipt": row["receipt"][2:], "receipt_digest": receipt,
        "offer": {"scheme": "exact", "asset": data[:32].hex(), "amount": str(int.from_bytes(data[32:64], "big")), "payTo": log["topics"][3][2:]},
    }
    delivery = {"version": 1, "receipt_digest": receipt, "state": "ready", **{field: payment[field] for field in ("principal", "request_digest", "resource")}, "response": {
        "body": list(body), "body_digest": hashlib.sha256(body).hexdigest(), "canonical_digest": content, "canonical_content": list(raw),
    }}
    write_private(path / "payments/requests" / key, payment)
    write_private(path / "payments/receipts" / receipt, key)
    write_private(path / "payments/deliveries" / key, delivery)
    write_private(path / "content" / content, raw)
    settlement = {"success": True, "amount": payment["offer"]["amount"], "transaction": "lxp:" + receipt, "extensions": {
        "layerx": {"receiptDigest": receipt, "verificationLevel": "sequencer-signed", "receipt": base64.b64encode(bytes.fromhex(payment["receipt"])).decode()},
        "x-websearch": {"resource": resource, "receiptDigest": receipt, "delivery": "ready", "responseDigest": hashlib.sha256(body).hexdigest(), "contentDigest": content},
    }}
    response = (200, {"payment-response": base64.b64encode(json.dumps(settlement).encode()).decode(), "content-digest": "sha-256=:" + base64.b64encode(hashlib.sha256(body).digest()).decode() + ":"}, body)
    case_input = {"route": "fetch", "target": resource[4:], "scheme": "exact", "asset_id": payment["offer"]["asset"], "amount": payment["offer"]["amount"]}
    return case_input, response, content, raw


def test_paid_success_reopens(tmp_path):
    module = production()
    case = vector("V1-layerxledger-pay-exact")
    data_dir = tmp_path / "paid"
    case_input, response, content, raw = paid_store(module, data_dir, case)
    node = RecordedNode(case, tmp_path / "accepted.json")
    node.contents["/content/" + content] = raw
    for _ in range(2):
        module = production()
        with node.serving() as url:
            harness = module.Harness.__new__(module.Harness)
            harness.gateway = harness.url = module.endpoint(url)
            harness.authorization_file = write_private(tmp_path / "authorization", AUTH)
            harness.config = {"gateway": gateway_config(case)}
            harness.data = data_dir
            harness.cases = {"paid": case_input}
            assert harness.success("paid", response) == response[2]
        del harness
    assert len([request for request in node.requests if request[0] == "pax_sequencerReceipt"]) == 2


def test_program_result_refuses_noncanonical_shape():
    module = production()
    case = vector("V1-layerxprograms-call")
    data = bytes.fromhex(case["receiptJson"]["returnData"][2:])
    assert module.program_result(data)[:2] == (0, 1)
    for malformed in (data[:-1], data + bytes(32), b"\x01" + data[1:], data[:64] + bytes(32) + data[96:]):
        with pytest.raises(module.Refusal):
            module.program_result(malformed)
