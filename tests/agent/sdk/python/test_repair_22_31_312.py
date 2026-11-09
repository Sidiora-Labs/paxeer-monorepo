"""Replay signed evidence and serve production-encoded guest results to the SDK."""

import json
from contextlib import contextmanager
from copy import deepcopy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Thread

import pytest

from layerx_sdk.ai_market import (
    AiMarketError,
    _decode_programs_call,
    _encode_programs_call,
    decode_programs_call_result,
)
from layerx_sdk.evm_transaction import _sequence, decode_signed
from layerx_sdk.verifier import SequencerReceiptVerification, SequencerReceiptVerificationError
from layerx_sdk.x402_rpc import PaymentRpc

from program_receipt_evidence import ProgramReceiptEvidence
from test_evm_transaction import proof_layerxprograms_0x101d


@contextmanager
def recorded_programs_rpc(evidence, terminal_output):
    requests = []
    receipt = evidence.vector["receiptJson"]
    transaction, _ = decode_signed(evidence.signed)
    call_params = [
        {
            "to": "0x" + transaction.to.hex(),
            "data": "0x" + transaction.data.hex(),
        },
        "latest",
    ]

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            requests.append((self.path, self.headers["Content-Type"], request))
            if (
                self.path != "/"
                or self.headers["Content-Type"] != "application/json"
                or request["jsonrpc"] != "2.0"
            ):
                self.send_error(400, "unrecorded request")
                return
            if (
                request["method"] == "pax_sequencerReceipt"
                and request["params"] == [receipt["txHash"]]
            ):
                result = receipt
            elif request["method"] == "eth_call" and request["params"] == call_params:
                result = "0x" + terminal_output.hex()
            else:
                self.send_error(400, "unrecorded request")
                return
            encoded = json.dumps({
                "jsonrpc": "2.0", "id": request["id"], "result": result,
            }).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(encoded)))
            self.end_headers()
            self.wfile.write(encoded)

        def log_message(self, format, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield PaymentRpc(f"http://127.0.0.1:{server.server_port}/"), requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


@pytest.fixture
def recorded_execution():
    evidence = ProgramReceiptEvidence()
    assert type(evidence.verified) is SequencerReceiptVerification
    assert evidence.verified.canonical_bytes == evidence.encoded
    transaction, _ = decode_signed(evidence.signed)
    native, fee = _decode_programs_call(transaction.data)
    assert transaction.to == bytes.fromhex("000000000000000000000000000000000000101d")
    assert native.guest_abi == 1
    assert native.response_capacity == 64
    assert _encode_programs_call(native, fee) == transaction.data
    assert evidence.vector["receiptJson"]["status"] == "0x1"
    assert evidence.submission() == evidence.vector["receiptJson"]
    return evidence, native


@pytest.mark.parametrize("guest_response", [b"", b"guest response", b"g" * 64])
def test_programs_terminal_response_recorded_call_survives_transport_reopen(
    recorded_execution, guest_response
):
    evidence, native = recorded_execution
    transaction, _ = decode_signed(evidence.signed)
    terminal_output = _sequence(["uint32", "uint8", "bytes"], [0, 1, guest_response])
    assert decode_programs_call_result(terminal_output, native.response_capacity) == (
        0, 1, guest_response,
    )
    outputs = []
    for _ in range(2):
        with recorded_programs_rpc(evidence, terminal_output) as (rpc, requests):
            receipt = rpc.receipt(evidence.vector["receiptJson"]["txHash"])
            assert receipt == evidence.vector["receiptJson"]
            assert evidence.submission(receipt) == receipt
            assert len(requests) == 1
            assert requests[0][0:2] == ("/", "application/json")
            result = rpc.call("eth_call", [
                {
                    "to": "0x" + transaction.to.hex(),
                    "data": "0x" + transaction.data.hex(),
                },
                "latest",
            ])
            assert len(requests) == 2
            assert requests[1][0:2] == ("/", "application/json")
            assert result == "0x" + terminal_output.hex()
            outputs.append(bytes.fromhex(result[2:]))
    assert outputs[0] == outputs[1]
    for output in outputs:
        assert decode_programs_call_result(output, native.response_capacity) == (
            0, 1, guest_response,
        )


def test_programs_terminal_response_proof_produces_recorded_calldata(
    recorded_execution, tmp_path, capsys
):
    evidence, _ = recorded_execution
    transaction, _ = decode_signed(evidence.signed)
    path = tmp_path / "producer.json"
    path.write_text(json.dumps({"callInput": transaction.data.hex()}))
    proof_layerxprograms_0x101d(str(path), "produce")
    assert bytes.fromhex(capsys.readouterr().out.strip()) == transaction.data


@pytest.mark.parametrize("field, error", [
    ("returnData", ValueError),
    ("logs", ValueError),
    ("receipt", SequencerReceiptVerificationError),
])
def test_programs_terminal_response_requires_authenticated_evidence(
    recorded_execution, field, error
):
    evidence, _ = recorded_execution
    changed = deepcopy(evidence.vector["receiptJson"])
    if field == "logs":
        changed["logs"][0]["data"] = "0x00"
    elif field == "receipt":
        receipt = bytearray.fromhex(changed[field][2:])
        receipt[-1] ^= 1
        changed[field] = "0x" + receipt.hex()
    else:
        changed[field] = "0x00"
    with pytest.raises(error):
        evidence.submission(changed)


@pytest.mark.parametrize("mutation", [
    "sign", "kind", "offset", "length", "padding", "trailing", "overflow",
])
def test_programs_terminal_response_refuses_malformed_and_overflow(
    recorded_execution, mutation
):
    _, native = recorded_execution
    output = bytearray(_sequence(["uint32", "uint8", "bytes"], [0, 1, b"guest response"]))
    assert decode_programs_call_result(bytes(output), native.response_capacity) == (
        0, 1, b"guest response",
    )
    if mutation == "overflow":
        output = bytearray(_sequence(
            ["uint32", "uint8", "bytes"], [0, 1, b"g" * (native.response_capacity + 1)]
        ))
    elif mutation == "trailing":
        output.extend(bytes(32))
    elif mutation == "padding":
        output[-1] ^= 1
    else:
        index = {"sign": 0, "kind": 63, "offset": 95, "length": 127}[mutation]
        output[index] ^= 0x80
    with pytest.raises(AiMarketError) as caught:
        decode_programs_call_result(bytes(output), native.response_capacity)
    assert caught.value.code == "ReceiptMismatch"
