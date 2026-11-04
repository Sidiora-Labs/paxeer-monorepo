from __future__ import annotations

import asyncio
import json
import runpy
import select
import subprocess
import threading
from base64 import b64encode
from contextlib import contextmanager
from dataclasses import replace
from hashlib import sha256
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest
from fastapi import Request, Response
from fastapi.testclient import TestClient

from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import decode_signed
from layerx_sdk.verifier import ReceiptKey, verify_sequencer_receipt

from .config import IntegrationError, offer_purpose_hash
from .integration import LayerXSellerRuntime, assert_receipt_backed, layerx_payment_gate
from .protocol import (
    MiddlewareError,
    MiddlewareErrorCode,
    decode_payment_payload_header,
    decode_payment_required_header,
    decode_settlement_header,
    encode_payment_payload_header,
    parse_layerx_evidence,
    parse_network,
    verify_payment_receipt,
)
from .seller import ReceiptPayloadAuthority, SellerMiddleware
from .signatures import LayerXSignatureVerifier

ROOT = Path(__file__).resolve().parents[4]


def _vector():
    document = json.loads((ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text())
    assert document["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    return next(row for row in document["cases"] if row["id"] == "V1-layerxledger-pay-exact")


@pytest.fixture
def example(tmp_path, monkeypatch):
    vector = _vector()
    log = vector["receiptJson"]["logs"][0]
    resource = tmp_path / "resource.json"
    resource.write_bytes(b'{"access":"paid-resource"}')
    values = {
        "LAYERX_PRINCIPAL": "merchant",
        "LAYERX_PROTECTED_PATH": "/paid",
        "LAYERX_RESOURCE_URL": "https://merchant.example/paid",
        "LAYERX_X402_SCHEME": "exact",
        "LAYERX_X402_NETWORK": "eip155:125",
        "LAYERX_PRICE": str(int(log["data"][66:], 16)),
        "LAYERX_ASSET": log["data"][:66],
        "LAYERX_PAY_TO": log["topics"][3],
        "LAYERX_PAYMENT_TIMEOUT_SECONDS": "120",
        "LAYERX_RECEIPT_KEYS_JSON": json.dumps({"keys": [vector["pinnedKey"]]}),
        "LAYERX_WEBHOOK_PATH": "/webhook",
        "LAYERX_WEBHOOK_PUBLIC_KEYS_JSON": json.dumps({"receipt": b64encode(bytes.fromhex(vector["pinnedKey"]["publicKey"][2:])).decode()}),
        "LAYERX_RESOURCE_FILE": str(resource),
        "LAYERX_FULFILLMENT_DIR": str(tmp_path / "fulfillments"),
    }
    for name, value in values.items():
        monkeypatch.setenv(name, value)
    loaded = runpy.run_path(str(ROOT / "sdk/integrations/fastapi/examples/paid_api.py"))
    loaded["vector"] = vector
    loaded["directory"] = tmp_path / "recorded-fulfillments"
    loaded["releases"] = []
    return loaded


def _requirements(example, **changes):
    log = example["vector"]["receiptJson"]["logs"][0]
    terms = {"commitment": "receipted", "account": "agent:did:layerx:payee:main", "currency": "LXP",
             "purposeHash": log["topics"][1][2:], "payer": log["topics"][2][2:]}
    return replace(example["mount"].config.requirements, extra={"layerx": terms}, has_extra=True, **changes)


def _application(example, requirements=None):
    app = example["FastAPI"]()
    requirements = requirements or _requirements(example)
    required = replace(example["mount"].config.payment_required, accepts=(requirements,))
    releases = example["releases"]
    base = example["StaticResourceHandler"]

    class Resources(base):
        async def release(self, request):
            resource = await super().release(request)
            releases.append(resource)
            return resource

    resources = Resources(b'{"access":"paid-resource"}' if not releases else b'{"access":"changed-after-reopen"}')
    seller = SellerMiddleware(required, ReceiptPayloadAuthority(example["mount"].config.receipt_keys),
                              example["FileFulfillmentRepository"](example["directory"]), LayerXSignatureVerifier())
    runtime = LayerXSellerRuntime(seller, requirements, "merchant", resources)
    app.add_api_route("/recorded-paid", layerx_payment_gate(runtime), methods=["GET"], response_model=None)
    return app, runtime


@contextmanager
def _http_server(app):
    class Handler(BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def log_message(self, *_args):
            pass

        def do_GET(self):
            self.dispatch()

        def do_POST(self):
            self.dispatch()

        def dispatch(self):
            body = self.rfile.read(int(self.headers.get("content-length", "0")))
            messages = []
            scope = {"type": "http", "asgi": {"version": "3.0"}, "http_version": "1.1",
                     "method": self.command, "scheme": "http", "path": self.path.split("?")[0],
                     "raw_path": self.path.split("?")[0].encode(), "query_string": b"",
                     "headers": [(key.lower().encode(), value.encode()) for key, value in self.headers.items()],
                     "client": self.client_address, "server": self.server.server_address}

            async def receive():
                return {"type": "http.request", "body": body, "more_body": False}

            async def send(message):
                messages.append(message)

            asyncio.run(app(scope, receive, send))
            start = messages[0]
            content = b"".join(message.get("body", b"") for message in messages[1:])
            self.close_connection = True
            self.send_response(start["status"])
            self.send_header("connection", "close")
            for key, value in start["headers"]:
                self.send_header(key.decode(), value.decode())
            self.end_headers()
            self.wfile.write(content)

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    server.daemon_threads = True
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}"
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()


@contextmanager
def _buyer(tmp_path):
    with (tmp_path / "buyer.stderr").open("w+") as errors:
        process = subprocess.Popen(["node", "--experimental-transform-types", str(Path(__file__).with_name("repair_22_31_287_buyer.mjs"))],
                                   stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=errors, text=True)

        def exchange(action, url):
            process.stdin.write(json.dumps({"action": action, "url": url}) + "\n")
            process.stdin.flush()
            ready, _, _ = select.select([process.stdout], [], [], 60)
            errors.flush()
            assert ready, "buyer did not respond within 60 seconds"
            line = process.stdout.readline()
            assert line, Path(errors.name).read_text()
            result = json.loads(line)
            assert "error" not in result, result
            return result

        try:
            yield exchange
        finally:
            if process.poll() is None:
                process.stdin.write('{"action":"close"}\n')
                process.stdin.flush()
            process.communicate(timeout=10)
            assert process.returncode == 0, Path(errors.name).read_text()


def _human_routes(app, example):
    recorded = {}
    calls = []
    golden = ROOT / "services/human/schema/human-api/golden"
    quote = json.loads((golden / "move.quote.response.json").read_text())["body"]["result"]
    quote["money"]["amount"] = _requirements(example).amount
    journey = json.loads((golden / "move.commit.response.json").read_text())["body"]["result"]
    evidence = json.loads((golden / "evidence.get.response.json").read_text())["body"]["result"]
    reference = {"evidence_id": evidence["evidence_id"], "class": "layerx-receipt", "verification": "receipted"}
    journey["evidence"] = [reference]
    journey["stages"] = [{**journey["stages"][0], "evidence": [reference]}]

    @app.post("/rpc")
    async def rpc(request: Request):
        body = await request.json()
        assert body["jsonrpc"] == "2.0"
        assert body["method"] == "pax_sequencerReceipt"
        assert body["params"] == [example["vector"]["receiptJson"]["txHash"]]
        calls.append("rpc")
        return {"jsonrpc": "2.0", "id": body["id"], "result": example["vector"]["receiptJson"]}

    @app.post("/recorded-evidence")
    async def stage(request: Request):
        recorded["proof"] = await request.json()
        return Response(status_code=204)

    @app.post("/v1/moves/quote")
    async def move_quote(request: Request):
        assert request.headers["authorization"] == "Bearer recorded-session"
        assert await request.json() == {"source": "acct:buyer-portable-receipts", "destination": "agent:did:layerx:payee:main",
                                             "money": {"amount": quote["money"]["amount"], "currency": "LXP"}}
        calls.append("move.quote")
        return {"ok": True, "result": quote}

    @app.post("/v1/moves")
    async def move_commit(request: Request):
        assert request.headers["idempotency-key"] == "repair-22-31-287"
        assert await request.json() == {"quote_id": quote["quote_id"]}
        calls.append("move.commit")
        return {"ok": True, "result": {**journey, "state": "processing"}}

    @app.get("/v1/journeys/{journey_id}")
    async def journey_get(journey_id: str):
        assert journey_id == journey["journey_id"]
        calls.append("journey.get")
        return {"ok": True, "result": journey}

    @app.get("/v1/evidence/{evidence_id}")
    async def evidence_get(evidence_id: str):
        assert evidence_id == evidence["evidence_id"]
        calls.append("evidence.get")
        return {"ok": True, "result": {**evidence, "verification": "receipted",
                "content_type": "application/vnd.layerx.receipt-proof-v2+json",
                "bytes_base64": b64encode(json.dumps(recorded["proof"]).encode()).decode()}}

    return calls, recorded


@pytest.fixture
def buyer_payment(example, tmp_path):
    app, runtime = _application(example)
    calls, recorded = _human_routes(app, example)
    with _buyer(tmp_path) as buyer:
        server = _http_server(app)
        url = server.__enter__()
        running = True

        def close_server():
            nonlocal running
            if running:
                server.__exit__(None, None, None)
                running = False

        try:
            prepared = buyer("prepare", url)
            assert calls == ["rpc", "move.quote", "move.commit", "journey.get", "evidence.get"]
            assert prepared["proof"] == recorded["proof"]
            yield prepared, buyer, url, runtime, close_server
        finally:
            close_server()


def test_012_013_014_real_buyer_http_and_replay_after_reopen(example, buyer_payment):
    prepared, buyer, url, runtime, close_server = buyer_payment
    payload = decode_payment_payload_header(prepared["header"])
    assert payload.accepted.network == "eip155:125"
    assert payload.payload == prepared["proof"]
    evidence = parse_layerx_evidence(payload.payload)
    assert evidence.verification_level == "receipted"
    assert evidence.receipt_key == example["mount"].config.receipt_keys[0]
    first = buyer("pay", url)
    assert first["body"] == '{"access":"paid-resource"}'
    assert first["settlement"]["extensions"]["layerx"] == prepared["proof"]
    assert buyer("pay", url) == first
    assert len(example["releases"]) == 1
    stored_files = list(example["directory"].glob("*.json"))
    assert len(stored_files) == 1
    stored = stored_files[0].read_bytes()
    assert json.loads(stored)["body"] == b64encode(first["body"].encode()).decode()
    close_server()
    for _ in range(2):
        reopened, _runtime = _application(example)
        with _http_server(reopened) as reopened_url:
            assert buyer("pay", reopened_url) == first
        assert len(example["releases"]) == 1
        assert stored_files[0].read_bytes() == stored


def test_012_configured_offer_has_native_network_and_bound_purpose(example):
    with TestClient(example["app"]) as client:
        challenge = client.get("/paid")
    assert challenge.status_code == 402
    offer = decode_payment_required_header(challenge.headers["PAYMENT-REQUIRED"])
    assert offer.accepts[0].network == "eip155:125"
    purpose = offer.accepts[0].extra["layerx"]["purposeHash"]
    assert purpose == sha256(b"layerx-fastapi-purpose:https://merchant.example/paid").hexdigest()
    assert purpose == offer_purpose_hash(offer.resource.url)
    assert purpose != offer_purpose_hash(offer.resource.url + "/other")
    assert offer.accepts[0].extra["layerx"]["commitment"] == "receipted"
    for network in ("layerx:125", "eip155:0", "eip155:0125", "eip155:4294967296", "eip155:125:1", "eip155:-125"):
        with pytest.raises(MiddlewareError):
            parse_network(network)


@pytest.mark.parametrize("scheme", ["exact", "metered", "subscription"])
def test_015_native_payment_and_wrong_purpose_refusals(example, buyer_payment, scheme):
    prepared, _buyer_process, _url, _runtime, _close = buyer_payment
    requirements = _requirements(example, scheme=scheme)
    raw = bytes.fromhex(prepared["proof"]["rawTransaction"][2:])
    signed = bytes.fromhex(prepared["proof"]["signedReceipt"][2:])
    verified = verify_payment_receipt(signed, raw, example["mount"].config.receipt_keys, requirements)
    transaction, _ = decode_signed(raw)
    assert transaction.chain_id == 125
    assert verified.receipt.amount == 0
    assert verified.receipt.to_account[-20:] == transaction.to
    assert verified.receipt.to_account != bytes.fromhex(requirements.pay_to[2:])
    assert verified.receipt_digest.hex() == example["vector"]["receiptDigest"][2:]
    assert prepared["proof"]["receiptDigest"] == "0x" + keccak256(signed).hex()
    assert prepared["proof"]["receiptDigest"] != example["vector"]["receiptDigest"]
    app, _runtime = _application(example, requirements)
    payload = replace(decode_payment_payload_header(prepared["header"]), accepted=requirements)
    header = encode_payment_payload_header(payload)
    with TestClient(app) as client:
        paid = client.get("/recorded-paid", headers={"PAYMENT-SIGNATURE": header})
        assert paid.status_code == 200
        settlement = decode_settlement_header(paid.headers["PAYMENT-RESPONSE"])
        assert settlement.amount == requirements.amount
        assert settlement.transaction == example["vector"]["receiptJson"]["txHash"]
    terms = requirements.extra["layerx"]
    wrong_purpose = replace(requirements, extra={"layerx": {**terms, "purposeHash": "ab" * 32}})
    missing_purpose = replace(requirements, extra={"layerx": {key: value for key, value in terms.items() if key != "purposeHash"}})
    for invalid in (wrong_purpose, missing_purpose):
        invalid_app, _ = _application(example, invalid)
        with TestClient(invalid_app) as client:
            refused = client.get("/recorded-paid", headers={"PAYMENT-SIGNATURE": encode_payment_payload_header(replace(payload, accepted=invalid))})
        assert refused.status_code == 402
        assert refused.json() == {"error": "verification-failure"}
    assert len(example["releases"]) == 1


def test_013_strict_portable_evidence_and_pinned_keys(example, buyer_payment):
    prepared, _, _, _, _ = buyer_payment
    original = prepared["proof"]
    payload = decode_payment_payload_header(prepared["header"])
    changes = [
        {"format": "layerx-receipt-proof-v1"}, {"verificationLevel": "sequencer-signed"},
        {"rawTransaction": b64encode(bytes.fromhex(original["rawTransaction"][2:])).decode()},
        {"signedReceipt": original["signedReceipt"].upper()}, {"receiptDigest": "11" * 32},
        {"receiptKey": {**original["receiptKey"], "firstHeight": 1000}},
        {"receiptKey": {**original["receiptKey"], "firstHeight": "0x03e8"}},
        {"receiptKey": {**original["receiptKey"], "lastHeight": "0x10000000000000000"}},
        {"receiptKey": {**original["receiptKey"], "firstHeight": "0x7d0"}},
        {"receiptKey": {**original["receiptKey"], "extra": True}}, {"idempotencyKey": "legacy"},
    ]
    for change in changes:
        with pytest.raises(MiddlewareError) as error:
            parse_layerx_evidence({**original, **change})
        assert error.value.code == MiddlewareErrorCode.INVALID_PAYMENT_PAYLOAD
    for key in original:
        with pytest.raises(MiddlewareError):
            parse_layerx_evidence({name: value for name, value in original.items() if name != key})
    app, _ = _application(example)
    invalid_proofs = [
        {**original, "receiptDigest": "0x" + "00" * 32},
        {**original, "receiptKey": {**original["receiptKey"], "publicKey": "0x" + "ab" * 32}},
        {**original, "receiptKey": {**original["receiptKey"], "keyId": "0x" + "ab" * 32}},
        {**original, "receiptKey": {**original["receiptKey"], "lastHeight": "0x7d0"}},
        {**original, "verificationLevel": "committed"},
        {**original, "rawTransaction": original["rawTransaction"] + "00"},
    ]
    damaged = bytearray.fromhex(original["signedReceipt"][2:])
    damaged[-1] ^= 1
    invalid_proofs.append({**original, "signedReceipt": "0x" + damaged.hex(), "receiptDigest": "0x" + keccak256(damaged).hex()})
    with TestClient(app) as client:
        for invalid in invalid_proofs:
            response = client.get("/recorded-paid", headers={"PAYMENT-SIGNATURE": encode_payment_payload_header(replace(payload, payload=invalid))})
            assert response.status_code == 402
            assert response.json() == {"error": "verification-failure"}
    assert example["releases"] == []
    assert not example["directory"].exists()


def test_015_offer_scheme_chain_asset_amount_payee_and_payer_are_bound(example, buyer_payment):
    prepared, _, _, _, _ = buyer_payment
    payload = decode_payment_payload_header(prepared["header"])
    terms = payload.accepted.extra["layerx"]
    invalid_offers = [
        replace(payload.accepted, scheme="transfer"), replace(payload.accepted, network="eip155:126"),
        replace(payload.accepted, asset="0x" + "ab" * 32), replace(payload.accepted, pay_to="0x" + "ab" * 32),
        replace(payload.accepted, amount=str(int(payload.accepted.amount) + 1)),
        replace(payload.accepted, extra={"layerx": {**terms, "payer": "ab" * 32}}),
        replace(payload.accepted, extra={"layerx": {**terms, "commitment": "committed"}}),
    ]
    for invalid in invalid_offers:
        app, _ = _application(example, invalid)
        with TestClient(app) as client:
            response = client.get("/recorded-paid", headers={"PAYMENT-SIGNATURE": encode_payment_payload_header(replace(payload, accepted=invalid))})
        assert response.status_code == 402
        assert response.json() == {"error": "verification-failure"}
    assert example["releases"] == []
    assert not example["directory"].exists()


def test_015_signed_execution_effect_must_agree_with_calldata(example, buyer_payment):
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
    from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

    prepared, _, _, _, _ = buyer_payment
    proof = prepared["proof"]
    raw = bytes.fromhex(proof["rawTransaction"][2:])
    signed = bytes.fromhex(proof["signedReceipt"][2:])
    secret = Ed25519PrivateKey.from_private_bytes(bytes(range(32)))
    key = replace(example["mount"].config.receipt_keys[0], public_key=secret.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw).hex())
    # Re-sign the recorded receipt after altering a committed log word. The signature
    # and raw-transaction binding remain valid; only payment semantics are invalid.
    transaction, _ = decode_signed(raw)
    for word in (transaction.data[164:196], transaction.data[4:36], transaction.data[36:68],
                 sha256(transaction.data[68:100] + transaction.data[100:132]).digest()):
        changed = bytearray(signed)
        offset = signed.index(word)
        changed[offset] ^= 1
        unsigned = bytes(changed[:-69]) + b"\0"
        digest = sha256(b"LXP/v1/receipt\0" + unsigned).digest()
        changed[-64:] = secret.sign(digest)
        canonical = bytes(changed)
        verified = verify_sequencer_receipt(canonical, raw, (ReceiptKey(bytes.fromhex(key.key_id), bytes.fromhex(key.public_key), key.first_height, key.last_height),))
        assert verified.receipt.activity_id == keccak256(raw)
        with pytest.raises(MiddlewareError) as error:
            verify_payment_receipt(canonical, raw, (key,), _requirements(example))
        assert error.value.code == MiddlewareErrorCode.VERIFICATION_FAILURE
        changed_example = {**example, "mount": replace(example["mount"], config=replace(example["mount"].config, receipt_keys=(key,)))}
        app, _ = _application(changed_example)
        altered_proof = {**proof, "signedReceipt": "0x" + canonical.hex(),
                         "receiptDigest": "0x" + keccak256(canonical).hex(),
                         "receiptKey": {**proof["receiptKey"], "publicKey": "0x" + key.public_key}}
        payload = replace(decode_payment_payload_header(prepared["header"]), payload=altered_proof)
        with TestClient(app) as client:
            response = client.get("/recorded-paid", headers={"PAYMENT-SIGNATURE": encode_payment_payload_header(payload)})
        assert response.status_code == 402
        assert response.json() == {"error": "verification-failure"}
    assert example["releases"] == []
    assert not example["directory"].exists()


def test_014_guard_refuses_mismatched_settlement_and_evidence(example, buyer_payment):
    prepared, _, _, runtime, _ = buyer_payment

    async def release():
        return await runtime.resources.release(Request({"type": "http", "method": "GET", "path": "/recorded-paid", "headers": []}))

    decision = asyncio.run(runtime.seller.handle("merchant", prepared["header"], release))
    assert_receipt_backed(decision, runtime.requirements)
    invalid_settlements = [
        replace(decision.settlement, transaction="lxp:" + decision.verification.receipt_digest.hex()),
        replace(decision.settlement, network="eip155:126"),
        replace(decision.settlement, amount="1"),
        replace(decision.settlement, payer="ab" * 32),
        replace(decision.settlement, success=False),
        replace(decision.settlement, extensions={"layerx": {**prepared["proof"], "receiptDigest": "0x" + "00" * 32}}),
        replace(decision.settlement, extensions={"layerx": {**prepared["proof"], "signedReceipt": prepared["proof"]["signedReceipt"] + "00"}}),
        replace(decision.settlement, extensions={"layerx": {**prepared["proof"], "rawTransaction": prepared["proof"]["rawTransaction"] + "00"}}),
    ]
    for invalid in invalid_settlements:
        with pytest.raises(IntegrationError):
            assert_receipt_backed(replace(decision, settlement=invalid), runtime.requirements)
    assert len(example["releases"]) == 1
