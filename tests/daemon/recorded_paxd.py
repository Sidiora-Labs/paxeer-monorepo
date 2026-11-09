"""Replay node-produced wire evidence without implementing a node or its stores."""

from contextlib import contextmanager
from copy import deepcopy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import re
from threading import Thread

from layerx_sdk.ai_market import _decode_programs_call, _encode_programs_call
from layerx_sdk.evm_transaction import decode_signed, encode_call
from layerx_sdk.programs import ProgramTrustContext, _sequencer_submission
from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt
from layerx_sdk.x402_rpc import PaymentRpc


ROOT = Path(__file__).resolve().parents[2]
EXECUTION_CORPUS = ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
RECOVERY_RECORDING = Path(__file__).with_name("fixtures") / "programs-commit-reopen.json"


def programs_execution_case():
    corpus = json.loads(EXECUTION_CORPUS.read_text())
    assert corpus["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    return next(case for case in corpus["cases"] if case["id"] == "V1-layerxprograms-call")


def program_transaction(case, *, response_capacity=64):
    raw = bytes.fromhex(case["rawTransaction"][2:])
    transaction, signature = decode_signed(raw)
    assert transaction.chain_id == 125
    assert transaction.to == bytes.fromhex("000000000000000000000000000000000000101d")
    assert transaction.recover_signer(signature).lower() == case["receiptJson"]["from"].lower()
    call, fee = _decode_programs_call(transaction.data)
    assert call.entrypoint == "layerx_call"
    assert call.guest_abi == 1
    assert call.calldata == b""
    assert call.capabilities == b"\0\0"
    assert call.access_declaration == b"LayerX/programs/access-declaration/v1\0\0"
    assert call.response_capacity == response_capacity, "program response capacity differs from admission"
    assert call.resources == (
        1_000_000, 16_777_216, 1_048_576, 1_048_576, 64, 1_048_576, 4096,
    )
    assert _encode_programs_call(call, fee) == transaction.data
    return raw, call


def verify_program_answer(case, answer, *, response_capacity=64):
    raw, call = program_transaction(case, response_capacity=response_capacity)
    pin = case["pinnedKey"]
    key = ReceiptKey(
        bytes.fromhex(pin["keyId"][2:]), bytes.fromhex(pin["publicKey"][2:]),
        pin["firstHeight"], pin["lastHeight"],
    )
    trust = ProgramTrustContext(key.public_key, network_id=125, receipt_keys=(key,))
    verified = verify_sequencer_receipt(bytes.fromhex(answer["receipt"][2:]), raw, (key,))
    assert _sequencer_submission(
        answer, raw, trust, program_id=call.program_id.hex(),
        activity_id=verified.receipt.activity_id.hex(),
    ) == answer
    assert verified.receipt.result_code == 0
    assert answer["status"] == "0x1"
    assert verified.receipt_digest.hex() == case["receiptDigest"][2:]
    assert verified.receipt.previous_state_root != verified.receipt.resulting_state_root
    return verified


def source_function(path, declaration):
    source = (ROOT / path).read_text()
    match = re.search(declaration + r"[^\{]*\{(.*?)^\}", source, re.MULTILINE | re.DOTALL)
    assert match is not None, f"retained function missing: {path}: {declaration}"
    return re.sub(r"\s+", " ", match.group(1)).strip()


def assert_in_order(body, *operations):
    cursor = 0
    for operation in operations:
        position = body.find(operation, cursor)
        assert position >= 0, f"missing or out-of-order operation: {operation}"
        cursor = position + len(operation)


def recovery_recording():
    assert RECOVERY_RECORDING.is_file(), (
        "Missing node-produced programs commitment/reopen recording: "
        "tests/daemon/fixtures/programs-commit-reopen.json. "
        "The available V1-layerxprograms-call vector proves receipted execution only. "
        "Regenerate with the retained node recorder in tests/daemon/recorder; "
        "a replay server cannot manufacture block commitment or node recovery."
    )
    document = json.loads(RECOVERY_RECORDING.read_text())
    assert document["schema"] == "paxd/programs-commit-reopen/v1"
    provenance = document["provenance"]
    assert provenance["generator"] == "tests/daemon/recorder/main.go"
    assert re.fullmatch(r"[0-9a-f]{40}", provenance["paxd_revision"])
    assert document["merge_height"] < document["anchor_store_delete_height"]
    assert set(document["phases"]) == {"after_merge", "after_anchor_delete_reopen"}
    assert document["node_stopped_between_phases"] is True
    assert document["anchor_store_absent_after_reopen"] is True
    assert "layerxanchor" not in document["stores_after_reopen"]
    return document


class RecordedPaxd:
    def __init__(self, exchanges):
        self.exchanges = deepcopy(exchanges)
        self.requests = []
        self.errors = []

    @classmethod
    def execution(cls, case):
        answer = case["receiptJson"]
        return cls([
            {"method": "eth_sendRawTransaction", "params": [case["rawTransaction"]],
             "result": answer["txHash"]},
            {"method": "pax_sequencerReceipt", "params": [answer["txHash"]],
             "result": answer},
        ])

    @contextmanager
    def serving(self):
        peer = self

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                try:
                    assert self.path == "/"
                    assert self.headers["Content-Type"] == "application/json"
                    length = int(self.headers["Content-Length"])
                    assert 0 < length <= 1_048_576
                    request = json.loads(self.rfile.read(length))
                    assert set(request) == {"jsonrpc", "id", "method", "params"}
                    assert request["jsonrpc"] == "2.0"
                    assert type(request["id"]) is int and request["id"] > 0
                    index = len(peer.requests)
                    assert index < len(peer.exchanges), "unrecorded request"
                    recorded = peer.exchanges[index]
                    peer.requests.append(request)
                    assert request["method"] == recorded["method"]
                    assert request["params"] == recorded["params"]
                    assert ("result" in recorded) != ("error" in recorded)
                    payload = {"jsonrpc": "2.0", "id": request["id"]}
                    field = "result" if "result" in recorded else "error"
                    payload[field] = recorded[field]
                    body = json.dumps(payload).encode()
                    self.send_response(200)
                    self.send_header("Content-Type", "application/json")
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                except Exception as error:
                    peer.errors.append(error)
                    self.send_error(400, "request differs from node recording")

            def log_message(self, *args):
                pass

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True)
        thread.start()
        try:
            yield PaymentRpc(f"http://127.0.0.1:{server.server_port}/")
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=5)
        assert not thread.is_alive()
        assert not self.errors, self.errors
        assert len(self.requests) == len(self.exchanges), "recording not fully consumed"


def observe_committed_program(case, phase, *, submit):
    peer = RecordedPaxd(phase["rpc"])
    tx_hash = case["receiptJson"]["txHash"]
    with peer.serving() as client:
        if submit:
            assert client.send(case["rawTransaction"]) == tx_hash
        answer = client.receipt(tx_hash)
        assert answer is not None and answer["rung"] == "committed"
        verified = verify_program_answer(case, answer, response_capacity=16)
        status = client.status(tx_hash)
        assert status is not None and status["status"] == "0x1"
        assert status["blockNumber"] == answer["height"]
        assert status["blockHash"] == answer["blockHash"]
        state = phase["state"]
        _, call = program_transaction(case, response_capacity=16)
        data = encode_call("getProgram", ["bytes32"], [call.program_id])
        assert state["method"] == "eth_call"
        assert state["params"] == [
            {"to": "0x000000000000000000000000000000000000101d",
             "data": "0x" + data.hex()},
            status["blockNumber"],
        ]
        assert client.call(state["method"], state["params"]) == state["result"]
        assert state["result"] == phase["expected_program_state"]
        assert phase["program_id"] == call.program_id.hex()
        encoded_state = bytes.fromhex(state["result"][2:])
        assert len(encoded_state) == 6 * 32
        assert encoded_state[32:64] != bytes(32)
        assert encoded_state[64:96] != bytes(32)
        assert int.from_bytes(encoded_state[96:128], "big") == call.guest_abi
        assert 1 <= int.from_bytes(encoded_state[128:160], "big") <= 0xffffffff
        assert encoded_state[160:] == bytes(31) + b"\x01"
    return verified, status, state["result"]
