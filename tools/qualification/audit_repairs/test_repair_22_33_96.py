"""Offline fixture and production-source proofs for audit http_profile_fixture.

These checks inspect committed bytes and Rust call sites. They do not execute a
Python substitute for the Rust validator, HTTP dispatcher, signer or relay.
"""

import ast
import base64
import hashlib
import json
import re
import tomllib
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
SUITE = ROOT / "services/interop/specs/conformance/transport-http"
RECORD = ROOT / "tools/qualification/audit_repairs/repair_22_33_96.kvx"
X402 = "services/interop/crates/layerx-x402/src/"
SERVICE = "services/interop/crates/layerx-interop-service/src/"
GATEWAY = "services/interop/crates/layerx-interop-gateway/src/"
FIXTURE = "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
CASE = "V1-layerxledger-pay-exact"
ACCOUNT = "agent:did:layerx:payee:main"


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def vectors():
    return json.loads((SUITE / "role-messages.json").read_text(encoding="utf-8"))


def semantic_cases():
    cases = [
        case
        for vector in vectors()
        for case in vector.get("semantic_cases", [])
    ]
    assert len({case["name"] for case in cases}) == len(cases)
    for vector in vectors():
        for case in vector.get("semantic_cases", []):
            assert case["role"] == vector["role"]
            assert case["transport"] == vector["transport"] == "http"
    return {
        case["name"]: case
        for case in cases
    }


def recorded_case():
    corpus = json.loads(source(FIXTURE))
    assert corpus["chainId"] == 125
    assert corpus["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    matching = [case for case in corpus["cases"] if case["id"] == CASE]
    assert len(matching) == 1
    return matching[0]


def function(text, name):
    """Read a Rust function through its closing brace at the same indentation."""
    match = re.search(
        r"^( *)(?:pub(?:\([^)]*\))? )?(?:const )?fn " + name + r"\b",
        text,
        re.MULTILINE,
    )
    assert match, name
    end = re.search(r"^" + match[1] + r"\}", text[match.end():], re.MULTILINE)
    assert end, name
    return text[match.start():match.end() + end.end()]


def requirements(case):
    document = case["document"]
    if case["role"] == "payment-required":
        return document["accepts"]
    if case["role"] == "payment-payload":
        return [document["accepted"]]
    assert case["role"] == "facilitator-request"
    assert document["paymentPayload"]["accepted"] == document["paymentRequirements"]
    return [document["paymentRequirements"], document["paymentPayload"]["accepted"]]


def rlp_item(data):
    """Inspect the recorded transaction encoding; no validation/signing substitute."""
    assert data
    prefix = data[0]
    if prefix < 0x80:
        return False, data[:1], data[1:]
    is_list = prefix >= 0xC0
    size = prefix - (0xC0 if is_list else 0x80)
    offset = 1
    if size > 55:
        width = size - 55
        assert len(data) >= 1 + width and data[1] != 0
        size = int.from_bytes(data[1:1 + width], "big")
        assert size >= 56
        offset += width
    assert len(data) >= offset + size
    value = data[offset:offset + size]
    if not is_list and len(value) == 1:
        assert value[0] >= 0x80
    return is_list, value, data[offset + size:]


def test_http_profile_fixture_preserves_roles_and_marks_envelope_only_payments():
    suite = vectors()
    assert [vector["name"] for vector in suite] == [
        "payment_required", "payment_payload", "settlement_response",
        "facilitator_request", "verify_response", "facilitator_settlement",
        "supported_response", "json_payment_required_is_not_an_http_representation",
    ]
    assert [vector["accepted"] for vector in suite] == [True] * 7 + [False]
    headers = {
        "payment-required": "PAYMENT-REQUIRED",
        "payment-payload": "PAYMENT-SIGNATURE",
        "settlement-response": "PAYMENT-RESPONSE",
    }
    for vector in suite:
        assert vector["transport"] == "http"
        if vector["accepted"] and vector["role"] in headers:
            assert vector["representation"] == "http-header"
            assert vector["header"] == headers[vector["role"]]
        else:
            assert vector["representation"] == "json"
    for name in ("payment_payload", "facilitator_request"):
        vector = next(vector for vector in suite if vector["name"] == name)
        assert vector["semantic"] == {
            "model": "accept", "payment": "refuse", "reason": "typed_intent_required"
        }
        document = vector["document"]
        payment = document if name == "payment_payload" else document["paymentPayload"]
        assert payment["payload"] == {"authorization": "opaque-signed-payment"}
    verifier = function(source(SERVICE + "server.rs"), "verified_x402_transfer")
    assert '.get("layerxTransaction")' in verifier
    assert '.ok_or("typed_intent_required")?' in verifier
    assert '.get("layerxIdempotencyKey")' in verifier


def test_http_profile_fixture_native_offer_matches_real_producer_and_consumer_facts():
    cases = semantic_cases()
    assert len(cases) == 7
    fixture = recorded_case()
    log = fixture["receiptJson"]["logs"][0]
    account = ACCOUNT.encode()
    native_id = hashlib.sha256(
        b"LX:ACCOUNT:v1" + len(account).to_bytes(4, "big") + account
    ).hexdigest()
    assert native_id == log["topics"][3][2:]
    expected = {
        "scheme": "exact", "network": "eip155:125", "amount": "1000000",
        "asset": log["data"][2:66], "payTo": native_id,
        "maxTimeoutSeconds": 60,
        "extra": {"layerx": {
            "commitment": "executed", "account": ACCOUNT, "currency": "LXP"
        }},
    }
    for name in ("payment_required", "payment_payload", "facilitator_request"):
        case = cases["recorded_native_" + name]
        assert case["accepted"] is True
        assert case["semantic"] == {"model": "accept", "payment": "not-proven"}
        assert case["document"]["x402Version"] == 2
        assert all(offer == expected for offer in requirements(case))
    producer = source("rpc/sequencer_receipt_vectors_test.go")
    assert 'codec.DeriveAccountID([]byte("' + ACCOUNT + '"))' in producer
    account_codec = source("layerxproof/codec/state.go")
    assert 'accountIdentifierDomain = "LX:ACCOUNT:v1"' in account_codec
    for fact in (
        "binary.BigEndian.PutUint32(length[:], uint32(len(name)))",
        "h.Write([]byte(accountIdentifierDomain))", "h.Write(length[:])", "h.Write(name)",
    ):
        assert fact in account_codec
    model = source(X402 + "model.rs")
    terms = function(model, "layerx_terms")
    assert "!account_identifiers(&account)?.contains(&payee)" in terms
    assert "return Err(X402Error::ProfileMismatch)" in terms
    assert 'const NATIVE_ACCOUNT_ID_DOMAIN: &[u8] = b"LX:ACCOUNT:v1";' in model
    identifiers = function(model, "account_identifiers")
    assert ".to_be_bytes()" in identifiers
    assert "account_digest(&[NATIVE_ACCOUNT_ID_DOMAIN, &length, name])" in identifiers


def test_http_profile_fixture_profile_free_and_foreign_cases_have_explicit_refusals():
    cases = semantic_cases()
    for name in ("payment_required", "payment_payload", "facilitator_request"):
        case = cases["profile_free_" + name]
        assert case["accepted"] is False
        assert case["semantic"] == {
            "model": "refuse", "reason": "ProfileMissing", "payment": "refuse"
        }
        for offer in requirements(case):
            assert offer["network"] == "eip155:125"
            assert "extra" not in offer
            native = requirements(cases["recorded_native_" + name])[0]
            assert offer == {key: value for key, value in native.items() if key != "extra"}
    foreign = cases["foreign_network_layerx_profile"]
    assert foreign["accepted"] is False
    assert foreign["semantic"] == {
        "model": "refuse", "reason": "ProfileMismatch", "payment": "refuse"
    }
    offer = requirements(foreign)[0]
    assert offer["network"] == "eip155:1" and "layerx" in offer["extra"]
    model = source(X402 + "model.rs")
    assert 'pub const LAYERX_NETWORK: &str = "eip155:125";' in model
    validation = function(model[model.index("impl PaymentRequirements {"):], "validate")
    assert "if self.is_layerx_network() {\n            self.layerx_terms()?;" in validation
    assert "layerx_profile(self.extra.as_ref(), parse_hex32(&self.asset).ok().as_ref())?" in validation
    assert "return Err(X402Error::ProfileMismatch)" in validation
    assert "self.network == LAYERX_NETWORK" in function(model, "is_layerx_network")
    assert ".ok_or(X402Error::ProfileMissing)?" in function(model, "layerx_terms")


def test_http_profile_fixture_header_and_json_paths_reach_semantic_validation():
    cases = semantic_cases()
    transport = source(X402 + "transport.rs")
    for role, suffix in (
        ("payment-required", "payment_required"),
        ("payment-payload", "payment_payload"),
        ("facilitator-request", "facilitator_request"),
    ):
        for case in (case for case in cases.values() if case["role"] == role):
            document = case["document"]
            wire = json.dumps(document, separators=(",", ":")).encode()
            assert len(wire) < 64 * 1024
            if case["representation"] == "http-header":
                header = base64.b64encode(wire)
                assert case["header"] == {
                    "payment-required": "PAYMENT-REQUIRED",
                    "payment-payload": "PAYMENT-SIGNATURE",
                }[role]
                assert json.loads(base64.b64decode(header, validate=True)) == document
            else:
                assert case["representation"] == "json"
                assert json.loads(wire) == document
        assert "value.validate()?;" in function(transport, "encode_" + suffix)
        assert "decoded.validate()?;" in function(transport, "decode_" + suffix)
    model = source(X402 + "model.rs")
    required = model[model.index("impl PaymentRequired {"):]
    payload = model[model.index("impl PaymentPayload {"):]
    assert "requirements.validate()?;" in function(required, "validate")
    assert "self.accepted.validate()?;" in function(payload, "validate")
    request = function(source(X402 + "facilitator.rs"), "validate")
    for fact in (
        "self.payment_payload.validate()?;", "self.payment_requirements.validate()?;",
        "self.payment_requirements.layerx_terms()?;",
        "self.payment_payload.accepted != self.payment_requirements",
        "return Err(X402Error::RequirementsMismatch);",
    ):
        assert fact in request
    codec = source(X402 + "codec.rs")
    assert "STANDARD.encode(json)" in function(codec, "encode_header")
    assert "serde_json::from_slice(&decoded)" in function(codec, "decode_header")


def test_http_profile_fixture_settlement_uses_unchanged_signed_payer_bytes():
    cases = semantic_cases()
    native = cases["recorded_native_facilitator_request"]
    payload = native["document"]["paymentPayload"]
    assert payload == cases["recorded_native_payment_payload"]["document"]
    fixture = recorded_case()
    metadata = native["recorded_payer"]
    assert metadata["path"] == FIXTURE and metadata["case"] == CASE
    assert payload["payload"]["layerxTransaction"] == fixture["rawTransaction"]
    raw = bytes.fromhex(payload["payload"]["layerxTransaction"][2:])
    assert raw[0] == 2
    is_list, fields, remainder = rlp_item(raw[1:])
    assert is_list and not remainder
    decoded = []
    while fields:
        is_list, value, fields = rlp_item(fields)
        decoded.append((is_list, value))
    assert len(decoded) == 12
    assert decoded[0] == (False, bytes([125]))
    assert decoded[5] == (False, bytes.fromhex("000000000000000000000000000000000000101a"))
    assert decoded[6] == (False, b"")
    assert decoded[8] == (True, b"")
    assert decoded[9][0] is False and decoded[9][1] in (b"", b"\x01")
    assert all(not item[0] and len(item[1]) == 32 for item in decoded[10:])
    calldata = decoded[7][1]
    assert not decoded[7][0] and len(calldata) == 4 + 6 * 32
    assert calldata[:4] == bytes.fromhex("78ea11db")
    words = [calldata[4 + index * 32:4 + (index + 1) * 32] for index in range(6)]
    log = fixture["receiptJson"]["logs"][0]
    offer = native["document"]["paymentRequirements"]
    assert words[0].hex() == metadata["payer_account"] == log["topics"][2][2:]
    assert words[1].hex() == offer["payTo"] == log["topics"][3][2:]
    assert words[2].hex() == offer["asset"] == log["data"][2:66]
    assert words[3][:16] == bytes(16)
    assert int.from_bytes(words[3], "big") == int(offer["amount"]) == 1000000
    assert words[4][:24] == bytes(24)
    assert int.from_bytes(words[4], "big") == metadata["expires_at_ms"]
    assert metadata["observed_at_ms"] < metadata["expires_at_ms"]
    assert words[5].hex() == payload["payload"]["layerxIdempotencyKey"] == log["topics"][1][2:]
    assert metadata["sender"] == fixture["receiptJson"]["from"]
    assert metadata["transaction_hash"] == fixture["receiptJson"]["txHash"]
    assert native["settlement"]["pending_response"] == {
        "success": False, "errorReason": "settlement_pending",
        "transaction": fixture["receiptJson"]["txHash"], "network": "eip155:125",
    }
    payer = function(source(SERVICE + "payer_tx.rs"), "decode_pay_exact")
    for fact in (
        "decode_signed(raw)", "recover_sender(&transaction, &signature)",
        "transaction.chain_id != crate::config::PAXEER_CHAIN_ID",
        "transaction.to != LAYERX_LEDGER", "Some(PAY_EXACT.as_slice())",
        "transaction_hash(raw)",
    ):
        assert fact in payer
    server = source(SERVICE + "server.rs")
    verifier = function(server, "verified_x402_transfer_reference")
    for fact in (
        "decode_pay_exact(&transaction)", "transfer.reference != expected_reference",
        "transfer.asset != asset", "transfer.to != recipient",
        "transfer.amount != request.payment_requirements.amount.value()",
        "payment_expired(transfer.expires_at, observed_at)",
    ):
        assert fact in verifier
    settle = server[server.index("impl FacilitatorPlane for X402SettlePlane {"):]
    settle = function(settle, "settle")
    assert "verified_x402_transfer(&request, self.observed_at)" in settle
    assert '.relay(&hex(&request.idempotency_key), &raw, sender)' in settle


def test_http_profile_fixture_mounted_ingress_contract_and_proof_boundaries():
    cases = semantic_cases()
    server = source(SERVICE + "server.rs")
    router = source(GATEWAY + "server.rs")
    assert '.strip_prefix("/v1/")' in router
    for tail in ('["seller", "offer"]', '["verify"] | ["facilitator", "verify"]',
                 '["settle"] | ["seller" | "facilitator", "settle"]'):
        assert tail in router
    for name in ("recorded_native_payment_required", "profile_free_payment_required",
                 "foreign_network_layerx_profile"):
        ingress = cases[name]["ingress"]
        assert ingress["method"] == "POST"
        assert ingress["path"] == "/v1/http/x402/seller/offer"
        assert ingress["content_type"] == "application/json"
        assert ingress["expected_status"] == (200 if cases[name]["accepted"] else 400)
        if cases[name]["accepted"]:
            assert ingress["expected_payment_status"] == 402
        else:
            assert ingress["expected_error"] == "invalid_x402_offer"
    for name in ("recorded_native_facilitator_request", "profile_free_facilitator_request"):
        ingress = cases[name]["ingress"]
        assert ingress["method"] == "POST" and ingress["content_type"] == "application/json"
        assert ingress["path"] == "/v1/http/x402/verify"
        assert ingress["expected_status"] == (200 if cases[name]["accepted"] else 400)
        if cases[name]["accepted"]:
            assert ingress["expected_is_valid"] is True
        else:
            assert ingress["expected_error"] == "invalid_x402_request"
    seller = function(server, "x402_seller")
    assert "Seller::new(required)" in seller
    assert 'Dispatch::error(400, "refused", "invalid_x402_offer")' in seller
    assert "seller.payment_required()" in seller
    assert '"status": signal.status' in seller
    assert "decode_facilitator_request" in function(server, "x402_request")
    assert 'Some("application/json")' in function(server, "x402_body")
    assert 'Dispatch::error(400, "refused", "invalid_x402_request")' in function(server, "x402_verify")
    settle = cases["recorded_native_facilitator_request"]["settlement"]
    assert settle["method"] == "POST" and settle["path"] == "/v1/http/x402/settle"
    assert settle["requires_authorization"] is True
    assert 'contains_key("authorization")' in function(server, "x402_settle")
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["http_profile_fixture"]
    assert finding["audit"] == "22.30.110" and finding["owner_domain"] == 11
    assert finding["file"] == "services/interop/specs/conformance/transport-http/role-messages.json"
    tests = {
        item.name for item in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(item, ast.FunctionDef) and item.name.startswith("test_")
    }
    assert set(finding["tests"]) == tests
    task = record["task"]["22"]["33"]["96"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_96.py"
    assert task["source_proof"] and task["review_only"] and task["baseline"]
    assert record["wiring"]["profile_free"]["owner_domain"] == 11
    assert record["wiring"]["mounted_ingress"]["owner_domain"] == 11
    assert "ProfileMissing" in record["wiring"]["profile_free"]["request"]
    assert record["wiring"]["mounted_ingress"]["status"] == "pending"
    readme = (SUITE / "README.md").read_text(encoding="utf-8")
    for fact in ("semantic_cases", "ProfileMissing", "typed_intent_required", "review-only"):
        assert fact in readme
