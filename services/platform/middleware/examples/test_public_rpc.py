import copy
import json
import os
import subprocess
import sys
import threading
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[4]
EXAMPLE = Path(__file__).with_name("public_rpc.py")
VECTORS = json.loads(
    (ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text()
)
PAYMENT = next(
    row for row in VECTORS["cases"] if row["id"] == "V1-layerxledger-pay-exact"
)
PAYER = PAYMENT["receiptJson"]["from"][2:].zfill(64)
DID = "did:layerx:" + PAYER


@contextmanager
def payment_listener(receipt):
    requests = []

    class Listener(BaseHTTPRequestHandler):
        def do_POST(self):
            body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            requests.append((self.path, body))
            results = {
                "px_resolveAccount": {
                    "evm_address": PAYMENT["receiptJson"]["from"],
                    "pax_address": None,
                    "layerx_did": DID,
                    "layerx_account": PAYER,
                    "bound": True,
                },
                "eth_getTransactionCount": PAYMENT["receiptJson"]["nonce"],
                "eth_sendRawTransaction": PAYMENT["receiptJson"]["txHash"],
                "pax_sequencerReceipt": receipt,
            }
            if body["method"] not in results or self.path != "/rpc":
                self.send_error(400, "unexpected RPC request")
                return
            payload = json.dumps({
                "jsonrpc": "2.0", "id": body["id"],
                "result": results[body["method"]],
            }).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

        def log_message(self, format, *args):
            pass

    with ThreadingHTTPServer(("127.0.0.1", 0), Listener) as server:
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            yield f"http://127.0.0.1:{server.server_port}/rpc", requests
        finally:
            server.shutdown()
            thread.join(timeout=5)
            assert not thread.is_alive()


def run_example(url, *arguments):
    environment = {
        key: value for key, value in os.environ.items()
        if not key.startswith("LAYERX_")
    }
    environment["PYTHONPATH"] = str(ROOT / "services/agent/sdk/python")
    return subprocess.run(
        [sys.executable, str(EXAMPLE), "--rpc", url, "--did", DID, *arguments],
        env=environment, capture_output=True, text=True, timeout=10,
    )


@pytest.mark.parametrize("case", [
    "valid", "default-commitment", "unpinned", "expired", "wrong-amount",
    "wrong-payee", "wrong-purpose", "wrong-payer", "bad-signature", "pending",
])
def test_python_example_verifies_receipted_payment(tmp_path, case):
    import base64

    receipt = copy.deepcopy(PAYMENT["receiptJson"])
    log = receipt["logs"][0]
    offer = {
        "scheme": "exact",
        "network": "eip155:125",
        "amount": str(int(log["data"][66:], 16)),
        "asset": log["data"][:66],
        "payTo": log["topics"][3],
        "maxTimeoutSeconds": 120,
        "extra": {"layerx": {
            "commitment": "receipted",
            "payer": log["topics"][2][2:],
            "purposeHash": log["topics"][1][2:],
        }},
    }
    keys = [copy.deepcopy(PAYMENT["pinnedKey"])]
    payer = PAYER
    failure = None
    if case == "default-commitment":
        del offer["extra"]["layerx"]["commitment"]
    elif case == "unpinned":
        keys[0]["keyId"] = "0x" + "2a" * 32
        failure = "SequencerReceiptVerificationError"
    elif case == "expired":
        keys[0]["lastHeight"] = int(receipt["height"], 16) - 1
        failure = "SequencerReceiptVerificationError"
    elif case == "wrong-amount":
        offer["amount"] = str(int(offer["amount"]) + 1)
        failure = "payment-binding-mismatch"
    elif case == "wrong-payee":
        offer["payTo"] = "0x" + "2a" * 32
        failure = "payment-binding-mismatch"
    elif case == "wrong-purpose":
        offer["extra"]["layerx"]["purposeHash"] = "2a" * 32
        failure = "payment-binding-mismatch"
    elif case == "wrong-payer":
        payer = "2a" * 32
        failure = "payment-binding-mismatch"
    elif case == "bad-signature":
        signed = bytearray.fromhex(receipt["receipt"][2:])
        signed[-1] ^= 1
        receipt["receipt"] = "0x" + signed.hex()
        failure = "SequencerReceiptVerificationError"
    elif case == "pending":
        receipt = None

    activity_file = tmp_path / "transaction.hex"
    offer_file = tmp_path / "offer.header"
    keys_file = tmp_path / "receipt-keys.json"
    activity_file.write_text(PAYMENT["rawTransaction"] + "\n")
    offer_file.write_text(base64.b64encode(json.dumps({
        "x402Version": 2,
        "resource": {"url": "https://seller.example/paid"},
        "accepts": [offer],
    }).encode()).decode())
    keys_file.write_text(json.dumps(keys))
    with payment_listener(receipt) as (url, requests):
        result = run_example(
            url, "--activity", str(activity_file), "--offer", str(offer_file),
            "--receipt-keys", str(keys_file), "--payer", payer,
        )
    assert requests == [
        ("/rpc", {
            "jsonrpc": "2.0", "id": 1,
            "method": "px_resolveAccount", "params": [DID],
        }),
        ("/rpc", {
            "jsonrpc": "2.0", "id": 2, "method": "eth_getTransactionCount",
            "params": [PAYMENT["receiptJson"]["from"], "pending"],
        }),
        ("/rpc", {
            "jsonrpc": "2.0", "id": 3, "method": "eth_sendRawTransaction",
            "params": [PAYMENT["rawTransaction"]],
        }),
        ("/rpc", {
            "jsonrpc": "2.0", "id": 4, "method": "pax_sequencerReceipt",
            "params": [PAYMENT["receiptJson"]["txHash"]],
        }),
    ]
    output = [json.loads(line) for line in result.stdout.splitlines()]
    sequence = {"sequence": PAYMENT["receiptJson"]["nonce"]}
    if failure is not None:
        assert result.returncode == 1, result.stderr
        assert failure in result.stderr
        assert output == [sequence]
    elif case == "pending":
        assert result.returncode == 2, result.stderr
        assert result.stderr == ""
        assert output == [sequence, {
            "state": "pending", "transaction": PAYMENT["receiptJson"]["txHash"],
        }]
    else:
        assert result.returncode == 0, result.stderr
        assert result.stderr == ""
        assert output == [sequence, {
            "transaction": PAYMENT["receiptJson"]["txHash"],
            "commitment": "receipted",
        }]


def test_python_example_has_no_authority_option(tmp_path):
    authority = tmp_path / "authority.json"
    authority.write_text("{}")
    with payment_listener(PAYMENT["receiptJson"]) as (url, requests):
        result = run_example(url, "--authority", str(authority))
    assert result.returncode == 2
    assert "unrecognized arguments: --authority" in result.stderr
    assert result.stdout == ""
    assert requests == []
