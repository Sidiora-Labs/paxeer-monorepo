"""smoke-receipt-shape: recorded node producer -> TLS -> payment file -> inspection."""

import copy
from contextlib import contextmanager
from datetime import datetime, timedelta, timezone
import importlib.util
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import ipaddress
import json
from pathlib import Path
import ssl
import sys
import threading

import pytest
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.x509.oid import NameOID

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[4]
sys.path.insert(0, str(HERE))
import receipt_inspection as inspection
from journey_common import write
from layerx_sdk.verifier import SequencerReceiptVerificationError
from layerx_sdk.x402_rpc import decode_receipt_keys

spec = importlib.util.spec_from_file_location("payment_journey_22_31_352", HERE / "payment-journey.py")
payment = importlib.util.module_from_spec(spec)
spec.loader.exec_module(payment)


@pytest.fixture
def recorded_payment():
    document = json.loads((ROOT / "daemon/committed-fixture/vectors/vector-v4-committed.json").read_text())
    settlement = document["settlement"]
    receipt = settlement["receipt"]
    authorization = document["authorization"]
    registry = {"keys": [{
        "keyId": receipt["keyId"],
        "publicKey": "0x" + bytes(authorization["PublicKey"]).hex(),
        "firstHeight": authorization["FirstBatchNumber"],
        "lastHeight": authorization["LastBatchNumber"],
    }]}
    return receipt, inspection.hex_bytes(settlement["raw_transaction"]), registry


@contextmanager
def recorded_node(tmp_path, receipt, *, wrong_id=False):
    key = ec.generate_private_key(ec.SECP256R1())
    subject = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "localhost")])
    now = datetime.now(timezone.utc)
    certificate = (
        x509.CertificateBuilder().subject_name(subject).issuer_name(subject)
        .public_key(key.public_key()).serial_number(x509.random_serial_number())
        .not_valid_before(now - timedelta(minutes=1)).not_valid_after(now + timedelta(hours=1))
        .add_extension(x509.SubjectAlternativeName([
            x509.DNSName("localhost"), x509.IPAddress(ipaddress.ip_address("127.0.0.1"))
        ]), critical=False)
        .add_extension(x509.BasicConstraints(ca=True, path_length=None), critical=True)
        .sign(key, hashes.SHA256())
    )
    ca = tmp_path / "ca.pem"
    ca.write_bytes(certificate.public_bytes(serialization.Encoding.PEM))
    private = tmp_path / "tls.key"
    private.write_bytes(key.private_bytes(
        serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8, serialization.NoEncryption()
    ))
    private.chmod(0o600)
    requests, failures = [], []

    class RecordedNode(BaseHTTPRequestHandler):
        def log_message(self, *_args):
            pass

        def do_POST(self):
            try:
                assert self.path == "/"
                assert self.headers["Content-Type"] == "application/json"
                assert self.headers.get("Authorization") is None
                body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                assert set(body) == {"jsonrpc", "id", "method", "params"}
                assert body["jsonrpc"] == "2.0" and type(body["id"]) is int
                assert body["method"] == "pax_sequencerReceipt"
                assert body["params"] == [receipt["txHash"]]
                requests.append(body)
                response = json.dumps({
                    "jsonrpc": "2.0", "id": body["id"] + int(wrong_id), "result": receipt
                }).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(response)))
                self.end_headers()
                self.wfile.write(response)
            except Exception as error:
                failures.append(error)
                self.send_error(400)

    server = ThreadingHTTPServer(("127.0.0.1", 0), RecordedNode)
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain(ca, private)
    server.socket = context.wrap_socket(server.socket, server_side=True)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"https://127.0.0.1:{server.server_port}", ca, requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()
        assert not failures, failures


def produce_payment_files(directory, receipt, raw):
    directory.mkdir(mode=0o700)
    payment.write_payment_receipt(directory, receipt)
    write(directory / "payment.raw", raw)
    result = {
        "activity_id": receipt["txHash"], "result": receipt, "replayed": True,
        "replay_refusal": {"code": -32000, "message": "already known"},
    }
    write(directory / "result.json", json.dumps(result))


def test_smoke_receipt_shape_real_node_payment_and_inspection(tmp_path, recorded_payment, capsys):
    receipt, raw, registry = recorded_payment
    assert "authority" not in receipt
    with recorded_node(tmp_path, receipt) as (endpoint, ca, requests):
        producer_evidence = tmp_path / "producer"
        producer_evidence.mkdir()
        received = inspection.NodeReceiptRpc(endpoint, ca, producer_evidence).receipt(receipt["txHash"])
        directory = tmp_path / "payment"
        produce_payment_files(directory, received, raw)
        assert json.loads((directory / "receipt.json").read_text()) == {"result": receipt}
        keys_file = tmp_path / "receipt-keys.json"
        keys_file.write_text(json.dumps(registry))
        output = tmp_path / "inspection"
        inspection.main([
            "--node", endpoint, "--ca", str(ca), "--receipt-keys", str(keys_file),
            "--payment", str(directory), "--output", str(output),
        ])
        result = json.loads((output / "verification.json").read_text())
        assert json.loads(capsys.readouterr().out) == result
        assert result["verified"] is True and result["result_code"] == 0
        assert result["activity_id"] == receipt["txHash"]
        assert result["rung"] == "committed" and result["verification_level"] == "sequencer-signed"
        assert result["key_id"] == registry["keys"][0]["keyId"]
        assert result["height"] == int(receipt["height"], 16)
        assert result["replayed"] is True and result["corruption_refused"] is True
        assert len(requests) == 3
        assert [request["id"] for request in requests] == [1, 1, 2]
        assert json.loads((output / "01-response.json").read_text())["result"] == receipt


@pytest.mark.parametrize("field,value", [
    ("txHash", "0x" + "42" * 32), ("nonce", "0x7"), ("seq", "0xa"),
    ("height", "0xb"), ("timestamp", "0x1"), ("status", "0x0"),
    ("keyId", "0x" + "42" * 32), ("signingKey", "0x" + "42" * 32),
    ("from", "0x" + "42" * 20), ("to", "0x" + "42" * 20),
    ("paymentDenom", "usid"), ("feeAmount", "0x1"), ("feeDenom", "usid"),
    ("gasUsed", "0x1"), ("cumulativeGasUsed", "0x1"), ("effectiveGasPrice", "0x1"),
    ("index", "0x2"), ("contractAddress", "0x" + "42" * 20),
    ("sponsor", "0x" + "42" * 20), ("returnData", "0x01"), ("vmError", "corrupt"),
    ("blockNumber", "0x9"), ("blockHash", "0x" + "00" * 32),
])
def test_smoke_receipt_shape_binds_every_node_view_field(recorded_payment, field, value):
    receipt, raw, registry = recorded_payment
    changed = {**receipt, field: value}
    assert changed != receipt
    with pytest.raises(AssertionError):
        inspection.verify_node_receipt(changed, raw, decode_receipt_keys(registry), receipt["txHash"])


@pytest.mark.parametrize("field", ["data", "address", "topics", "logIndex"])
def test_smoke_receipt_shape_binds_logs(recorded_payment, field):
    receipt, raw, registry = recorded_payment
    changed = copy.deepcopy(receipt)
    changed["logs"][0][field] = {
        "data": "0x01", "address": "0x" + "42" * 20,
        "topics": ["0x" + "42" * 32], "logIndex": "0x1",
    }[field]
    with pytest.raises(AssertionError):
        inspection.verify_node_receipt(changed, raw, decode_receipt_keys(registry), receipt["txHash"])


@pytest.mark.parametrize("mutation,check", [
    ("signature", "bad_signature"), ("transaction", "hash_mismatch"),
    ("unpinned", "unpinned_key"), ("expired", "key_out_of_range"),
    ("public_key", "bad_signature"),
])
def test_smoke_receipt_shape_current_verifier_refusals(recorded_payment, mutation, check):
    receipt, raw, registry = copy.deepcopy(recorded_payment)
    if mutation == "signature":
        signed = bytearray(inspection.hex_bytes(receipt["receipt"]))
        signed[-1] ^= 1
        receipt["receipt"] = "0x" + signed.hex()
    elif mutation == "transaction":
        raw = raw[:-1] + bytes([raw[-1] ^ 1])
    elif mutation == "unpinned":
        registry["keys"][0]["keyId"] = "0x" + "42" * 32
    elif mutation == "expired":
        registry["keys"][0]["lastHeight"] = 9
        registry["keys"][0]["firstHeight"] = 1
    elif mutation == "public_key":
        registry["keys"][0]["publicKey"] = "0x" + "42" * 32
    with pytest.raises(SequencerReceiptVerificationError) as raised:
        inspection.verify_node_receipt(receipt, raw, decode_receipt_keys(registry), receipt["txHash"])
    assert raised.value.check == check


def test_smoke_receipt_shape_refuses_unachieved_rung():
    recorded = json.loads((ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text())
    vector = next(case for case in recorded["cases"] if case["id"] == "V1-pax-transfer")
    keys = decode_receipt_keys({"keys": [vector["pinnedKey"]]})
    with pytest.raises(AssertionError, match="committed rung"):
        inspection.verify_node_receipt(vector["receiptJson"], inspection.hex_bytes(vector["rawTransaction"]),
                                       keys, vector["receiptJson"]["txHash"])


@pytest.mark.parametrize("change", ["payment_hash", "replay", "stored_receipt"])
def test_smoke_receipt_shape_refuses_payment_artifact_mismatch(tmp_path, recorded_payment, change):
    receipt, raw, registry = recorded_payment
    directory = tmp_path / "payment"
    produce_payment_files(directory, receipt, raw)
    result_path = directory / "result.json"
    result = json.loads(result_path.read_text())
    if change == "payment_hash":
        result["activity_id"] = "0x" + "42" * 32
    elif change == "replay":
        result["replay_refusal"]["code"] = -1
    else:
        result["result"]["nonce"] = "0x7"
    result_path.write_text(json.dumps(result))
    with recorded_node(tmp_path, receipt) as (endpoint, ca, requests):
        output = tmp_path / "inspection"
        output.mkdir()
        client = inspection.NodeReceiptRpc(endpoint, ca, output)
        with pytest.raises(AssertionError):
            inspection.inspect_payment(client, directory, decode_receipt_keys(registry))
        assert not requests


def test_smoke_receipt_shape_refuses_wrong_rpc_id(tmp_path, recorded_payment):
    receipt, _, _ = recorded_payment
    with recorded_node(tmp_path, receipt, wrong_id=True) as (endpoint, ca, _):
        output = tmp_path / "inspection"
        output.mkdir()
        with pytest.raises(AssertionError):
            inspection.NodeReceiptRpc(endpoint, ca, output).receipt(receipt["txHash"])


def test_smoke_receipt_shape_shell_uses_node_inspection():
    source = (HERE / "hosted-smoke.sh").read_text()
    block = source.split("admit_journey receipt-inspection\n", 1)[1]
    assert '"$(dirname "$0")/receipt_inspection.py" --node "$LAYERX_TEST_NODE_URL"' in block
    assert '--receipt-keys "$LAYERX_TEST_RECEIPT_KEYS_FILE"' in block
    assert '--payment "$work/payment"' in block
    assert '.rung == "committed"' in block and '.corruption_refused == true' in block
    assert ".authority" not in block and "--batch-id" not in block
    assert "/v1/receipts/" not in block
