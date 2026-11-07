"""Fixture and production-source proofs for audit 22.30.110/a2a_profile_fixture.

Rust execution is outside this Python-only gate; no Python payment validator
stands in for the production model, mounted server or receipt verifier.
"""

import ast
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import tomllib


ROOT = Path(__file__).resolve().parents[3]
DIRECTORY = ROOT / "services/interop/specs/conformance/transport-a2a"
X402 = "services/interop/crates/layerx-x402/"
SERVICE = "services/interop/crates/layerx-interop-service/src/"
PRODUCER = "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
CASE_ID = "V1-layerxledger-pay-exact"
ROLES = ("payment-required", "payment-payload", "facilitator-request")
ORIGINAL_NAMES = (
    "payment_required", "payment_payload", "settlement_response",
    "facilitator_request", "verify_response", "facilitator_settlement",
    "supported_response", "http_header_payment_required_is_not_a_a2a_representation",
)


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def records():
    return json.loads((DIRECTORY / "role-messages.json").read_text(encoding="utf-8"))


def recorded_case():
    fixture = json.loads(source(PRODUCER))
    cases = [case for case in fixture["cases"] if case["id"] == CASE_ID]
    assert len(cases) == 1
    return fixture, cases[0]


def rust_function(text, name):
    match = re.search(
        r"(?m)^( *)(?:pub(?:\([^\n]*?\))? )?(?:const )?fn "
        + re.escape(name) + r"(?:<[^\n]*>)?\(", text
    )
    assert match, name
    end = re.search(r"(?m)^" + match[1] + r"\}", text[match.end():])
    assert end, name
    return text[match.start():match.end() + end.end()]


def rust_impl(text, name):
    marker = "impl " + name + " {"
    assert text.count(marker) == 1, name
    return text.split(marker, 1)[1].split("\n}", 1)[0]


def ordered(text, *facts):
    cursor = 0
    for fact in facts:
        position = text.find(fact, cursor)
        assert position >= 0, fact
        cursor = position + len(fact)


def offers(vector):
    document = vector["document"]
    if vector["role"] == "payment-required":
        return document["accepts"]
    if vector["role"] == "payment-payload":
        return [document["accepted"]]
    assert vector["role"] == "facilitator-request"
    return [document["paymentPayload"]["accepted"], document["paymentRequirements"]]


def rlp_item(data, offset=0):
    """Decode committed RLP bytes to inspect fixture facts, not verify a signature."""
    assert offset < len(data)
    prefix = data[offset]
    if prefix < 0x80:
        return data[offset:offset + 1], offset + 1
    is_list = prefix >= 0xc0
    base = 0xc0 if is_list else 0x80
    size = prefix - base
    start = offset + 1
    if size > 55:
        width = size - 55
        assert start + width <= len(data) and data[start] != 0
        size = int.from_bytes(data[start:start + width], "big")
        assert size >= 56
        start += width
    end = start + size
    assert end <= len(data)
    if not is_list:
        assert size != 1 or data[start] >= 0x80
        return data[start:end], end
    children = []
    cursor = start
    while cursor < end:
        child, cursor = rlp_item(data, cursor)
        children.append(child)
    assert cursor == end
    return children, end


def test_original_roles_and_assertions_are_preserved():
    vectors = records()
    assert len(vectors) == 14
    assert len({vector["name"] for vector in vectors}) == len(vectors)
    originals = vectors[:8]
    assert tuple(vector["name"] for vector in originals) == ORIGINAL_NAMES
    projection = [
        {key: value for key, value in vector.items() if key != "expectations"}
        for vector in originals
    ]
    canonical = json.dumps(projection, sort_keys=True, separators=(",", ":")).encode()
    assert hashlib.sha256(canonical).hexdigest() == (
        "4a9ba9ef243ab0f243decaf850d067b06b88680104f018ab633c72db4b77b6c1"
    )
    assert all(vector["transport"] == "a2a" for vector in vectors)
    assert {vector["role"] for vector in originals if vector["accepted"]} == {
        *ROLES, "settlement-response", "verify-response",
        "facilitator-settlement", "supported-response",
    }
    assert originals[-1]["expectations"]["model"] == {"accepted": False, "error": "Decode"}


def test_profile_free_offers_preserve_the_current_semantic_refusal():
    model = source(X402 + "src/model.rs")
    assert 'pub const LAYERX_NETWORK: &str = "eip155:125";' in model
    requirements = rust_impl(model, "PaymentRequirements")
    validate = rust_function(requirements, "validate")
    ordered(validate, "if self.is_layerx_network() {", "self.layerx_terms()?;",
            "else if layerx_profile(", "return Err(X402Error::ProfileMismatch)")
    assert "self.network == LAYERX_NETWORK" in rust_function(requirements, "is_layerx_network")
    ordered(rust_function(requirements, "layerx_terms"),
            "layerx_profile(self.extra.as_ref(), offer_asset.as_ref())?",
            ".ok_or(X402Error::ProfileMissing)?")
    refused = [vector for vector in records()
               if vector["expectations"]["purpose"] == "profile-free-regression"]
    assert {vector["role"] for vector in refused} == set(ROLES)
    assert len(refused) == 3
    for vector in refused:
        assert vector["accepted"] is False
        assert vector["expectations"]["model"] == {"accepted": False, "error": "ProfileMissing"}
        for offer in offers(vector):
            assert offer["network"] == "eip155:125"
            assert "extra" not in offer
        if vector["role"] == "facilitator-request":
            assert vector["document"]["paymentPayload"]["accepted"] == vector["document"]["paymentRequirements"]


def test_recorded_offers_bind_the_real_producer_and_all_roles():
    fixture, case = recorded_case()
    assert fixture["chainId"] == 125
    assert fixture["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    producer = source(fixture["provenance"]["generator"])
    assert 'to, err := codec.DeriveAccountID([]byte("agent:did:layerx:payee:main"))' in producer
    ordered(producer, "func seqVecPayExactCall(",
            'seqVecSelector("payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)")',
            "Pack([32]byte(pay.from), [32]byte(pay.to), [32]byte(pay.asset), pay.amount, pay.expiresAt, [32]byte(pay.reference))")
    assert 'accept("V1-layerxledger-pay-exact", "V1"' in producer
    log = case["receiptJson"]["logs"][0]
    native = [vector for vector in records()
              if vector["expectations"]["purpose"] == "recorded-payer"]
    assert len(native) == 3 and {vector["role"] for vector in native} == set(ROLES)
    expected = {
        "scheme": "exact", "network": "eip155:125", "amount": "1000000",
        "asset": log["data"][2:66], "payTo": log["topics"][3][2:], "maxTimeoutSeconds": 60,
        "extra": {"layerx": {"commitment": "executed",
                             "account": "agent:did:layerx:payee:main", "currency": "LXP"}},
    }
    account = expected["extra"]["layerx"]["account"].encode()
    assert hashlib.sha256(b"LX:ACCOUNT:v1" + len(account).to_bytes(4, "big") + account).hexdigest() == expected["payTo"]
    for vector in native:
        assert vector["accepted"] is True
        assert vector["expectations"]["model"] == {"accepted": True}
        assert vector["provenance"] == {
            "fixture": PRODUCER, "case": CASE_ID,
            "generator": fixture["provenance"]["generator"],
        }
        assert all(offer == expected for offer in offers(vector))
        assert vector["expectations"]["observed_at_ms"] == int(case["receiptJson"]["timestamp"], 16)
    free = [vector for vector in records() if vector["name"].startswith("profile_free_")]
    for vector in free:
        counterpart = next(item for item in native if item["role"] == vector["role"])
        for offer, bound in zip(offers(vector), offers(counterpart), strict=True):
            assert offer == {key: value for key, value in bound.items() if key != "extra"}


def test_signed_payer_bytes_match_calldata_receipt_and_reference():
    _, case = recorded_case()
    receipt = case["receiptJson"]
    raw = bytes.fromhex(case["rawTransaction"][2:])
    assert raw[0] == 2
    fields, end = rlp_item(raw, 1)
    assert end == len(raw) and len(fields) == 12
    assert int.from_bytes(fields[0], "big") == 125
    assert int.from_bytes(fields[1], "big") == int(receipt["nonce"], 16)
    assert fields[5].hex() == receipt["to"][2:] == "000000000000000000000000000000000000101a"
    assert fields[6] == b"" and fields[8] == []
    assert int.from_bytes(fields[9], "big") in (0, 1)
    assert all(0 < int.from_bytes(value, "big") < 2**256 for value in fields[10:])
    calldata = fields[7]
    assert calldata[:4].hex() == "78ea11db" and len(calldata) == 196
    words = [calldata[start:start + 32] for start in range(4, len(calldata), 32)]
    log = receipt["logs"][0]
    assert words[0].hex() == log["topics"][2][2:]
    assert words[1].hex() == log["topics"][3][2:]
    assert words[2].hex() == log["data"][2:66]
    assert words[3].hex() == log["data"][66:]
    assert int.from_bytes(words[3], "big") == 1_000_000
    assert int.from_bytes(words[4], "big") > int(receipt["timestamp"], 16)
    assert words[5].hex() == log["topics"][1][2:]
    for vector in records()[8:]:
        if vector["role"] == "payment-required":
            continue
        document = vector["document"]
        payment = document["paymentPayload"] if vector["role"] == "facilitator-request" else document
        assert payment["payload"] == {
            "transaction": receipt["txHash"], "layerxTransaction": case["rawTransaction"],
            "layerxIdempotencyKey": words[5].hex(),
        }
    evidence = next(vector for vector in records() if vector["name"] == "recorded_native_facilitator_request")["expectations"]["settlement_evidence"]
    assert evidence["fixture"] == PRODUCER and evidence["case"] == CASE_ID
    assert evidence["signedReceipt"] == case["signedReceipt"] == receipt["receipt"]
    assert evidence["receiptDigest"] == case["receiptDigest"]
    assert evidence["pinnedKey"] == case["pinnedKey"]
    assert evidence["rung"] == "receipted"
    assert "submission alone is pending" in evidence["success_requires"]


def test_production_codecs_validate_the_model_in_both_directions():
    transport = source(X402 + "src/transport.rs")
    for role, type_name in (("payment_required", "PaymentRequired"),
                            ("payment_payload", "PaymentPayload"),
                            ("facilitator_request", "FacilitatorRequest")):
        encoder = rust_function(transport, "encode_" + role)
        decoder = rust_function(transport, "decode_" + role)
        assert "value.validate()?;" in encoder
        ordered(decoder, "let decoded: " + type_name, "decoded.validate()?;", "Ok(decoded)")
    assert "TransportKind::Mcp | TransportKind::A2a => encode_json(value)" in rust_function(transport, "encode_core")
    assert "(TransportKind::Mcp | TransportKind::A2a, TransportValue::Json(_)) => decode_json(value)" in rust_function(transport, "decode_core")
    assert "_ => Err(X402Error::Decode)" in rust_function(transport, "decode_core")
    model = source(X402 + "src/model.rs")
    assert "requirements.validate()?;" in rust_function(rust_impl(model, "PaymentRequired"), "validate")
    assert "self.accepted.validate()?;" in rust_function(rust_impl(model, "PaymentPayload"), "validate")
    facilitator = rust_function(rust_impl(source(X402 + "src/facilitator.rs"), "FacilitatorRequest"), "validate")
    ordered(facilitator, "self.payment_payload.validate()?;", "self.payment_requirements.validate()?;",
            "self.payment_requirements.layerx_terms()?;", "self.payment_payload.accepted != self.payment_requirements",
            "X402Error::RequirementsMismatch")
    harness = source(X402 + "tests/transports.rs")
    assert 'include_str!("../../../specs/conformance/transport-a2a/role-messages.json")' in harness
    assert "$decode($transport, &$vector.literal).is_err()" in harness
    assert 'record["accepted"].as_bool()' in rust_function(harness, "role_vectors")


def test_opaque_role_examples_explicitly_refuse_payment():
    by_name = {vector["name"]: vector for vector in records()}
    for name in ("payment_payload", "facilitator_request"):
        vector = by_name[name]
        assert vector["accepted"] is True
        assert vector["expectations"]["model"] == {"accepted": True}
        assert vector["expectations"]["payment"] == {"accepted": False, "reason": "typed_intent_required"}
    expected = by_name["facilitator_request"]["expectations"]["ingress"]
    assert expected == [
        {"method": "POST", "path": "/v1/a2a/x402/verify", "status": 200,
         "isValid": False, "invalidReason": "transaction_form"},
        {"method": "POST", "path": "/v1/a2a/x402/settle", "status": 400,
         "error": "protocol_idempotency_required"},
    ]
    facilitator = source(X402 + "src/facilitator.rs")
    verify = rust_function(rust_impl(facilitator, "Facilitator"), "verify")
    ordered(verify, '.get("transaction")', "X402Error::TransactionForm",
            "plane.verify(&plane_request, trace)", 'reason: "transaction_form"')
    server = source(SERVICE + "server.rs")
    ordered(rust_function(server, "verified_x402_transfer"), '.get("layerxTransaction")',
            '.ok_or("typed_intent_required")?')
    ordered(rust_function(server, "x402_settle"), 'let Ok(parsed) = x402_request(',
            '.get("layerxIdempotencyKey")', '"protocol_idempotency_required"', "facilitator.settle(")


def test_mounted_ingress_expectations_match_production_dispatch_and_guards():
    routes = source("services/interop/crates/layerx-interop-gateway/src/server.rs")
    mounted = rust_function(routes, "interop_gateway_routes")
    assert '.strip_prefix("/v1/")' in mounted
    assert '"a2a" => Ok(IngressTransport::A2a)' in rust_function(routes, "parse_transport")
    for tail, variant in (('["seller", "offer"]', "X402SellerOffer"),
                          ('["verify"] | ["facilitator", "verify"]', "X402Verify"),
                          ('["settle"] | ["seller" | "facilitator", "settle"]', "X402Settle")):
        assert '("POST", transport, "x402", ' + tail + ") => {" in mounted
        assert "InteropRoute::" + variant + " { transport }" in mounted
    server = source(SERVICE + "server.rs")
    dispatch = rust_function(server, "dispatch")
    for variant, handler in (("X402SellerOffer", "x402_seller"),
                             ("X402Verify", "x402_verify"), ("X402Settle", "x402_settle")):
        ordered(dispatch, "InteropRoute::" + variant, handler + "(")
    assert "route => authenticated(config, request, &route)" in rust_function(server, "route")
    assert "IngressTransport::A2a => TransportKind::A2a" in rust_function(server, "transport_kind")
    assert "decode_facilitator_request(transport_kind(transport), &TransportValue::Json(value))" in rust_function(server, "x402_request")
    assert 'Some("application/json")' in rust_function(server, "x402_body")
    seller = rust_function(server, "x402_seller")
    ordered(seller, "Seller::new(required)", 'Dispatch::error(400, "refused", "invalid_x402_offer")',
            "seller.payment_required()", "Dispatch::result(", "200,")
    for name in ("x402_verify", "x402_settle"):
        ordered(rust_function(server, name), "x402_request(request, transport)",
                'Dispatch::error(400, "refused", "invalid_x402_request")')
    for vector in records()[8:]:
        expected = vector["expectations"]
        if vector["role"] == "payment-payload":
            assert "ingress" not in expected
            continue
        for ingress in expected["ingress"]:
            assert ingress["method"] == "POST"
            if vector["accepted"]:
                assert ingress["status"] == 200 and "error" not in ingress
                if vector["role"] == "facilitator-request":
                    assert ingress["isValid"] is True
            else:
                assert ingress["status"] == 400
                assert ingress["error"] == ("invalid_x402_offer" if vector["role"] == "payment-required" else "invalid_x402_request")


def test_settlement_path_preserves_signed_bytes_and_requires_real_evidence():
    payer = rust_function(source(SERVICE + "payer_tx.rs"), "decode_pay_exact")
    ordered(payer, "decode_signed(raw)", "transaction.chain_id != crate::config::PAXEER_CHAIN_ID",
            "transaction.to != LAYERX_LEDGER", "transaction.data.get(..4)",
            "recover_sender(&transaction, &signature)", "transaction_hash(raw)")
    server = source(SERVICE + "server.rs")
    checked = rust_function(server, "verified_x402_transfer_reference")
    ordered(checked, "decode_pay_exact(&transaction)", "transfer.reference != expected_reference",
            "x402_facts(&request.payment_requirements)", "transfer.asset != asset",
            "transfer.to != recipient", "transfer.amount != request.payment_requirements.amount.value()",
            "payment_expired(transfer.expires_at, observed_at)")
    settle_plane = rust_function(rust_impl(server, "FacilitatorPlane for X402SettlePlane"), "settle")
    ordered(settle_plane, "verified_x402_transfer(&request, self.observed_at)",
            '.get("layerxTransaction")', "decode_pay_exact(&raw)",
            '.relay(&hex(&request.idempotency_key), &raw, sender)',
            "RelayOutcome::Pending", "FacilitatorSettlementOutcome::Pending",
            "RelayOutcome::Executed(evidence)", "FacilitatorSettlementOutcome::Executed(evidence)")
    facilitator = source(X402 + "src/facilitator.rs")
    ordered(rust_function(facilitator, "settle_executed"),
            "evidence.rung == SettlementRung::Submitted", "X402Error::EvidenceMissing", "Seller::settled(")
    settled = rust_function(rust_impl(source(X402 + "src/seller.rs"), "Seller"), "settled")
    ordered(settled, "verify_sequencer_signature(", "verify_logs(", "settlement_payment(")


def test_real_deployment_renderer_pins_the_augmented_fixture_suite():
    path = ROOT / "services/interop/deploy/gateway/render.py"
    spec = importlib.util.spec_from_file_location("a2a_production_renderer", path)
    assert spec is not None and spec.loader is not None
    renderer = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(renderer)
    data = (DIRECTORY / "role-messages.json").read_bytes()
    expected_digest = hashlib.sha256(b"role-messages.json\0" + data + b"\0").hexdigest()
    suite = renderer.first_party_suite(ROOT, "transport-a2a")
    assert suite == ("layerx-transport-a2a-conformance-v1", 14, expected_digest)
    _, transports = renderer.derived(ROOT)
    assert transports["a2a"]["conformance_sha256"] == (
        expected_digest, "services/interop/specs/conformance/transport-a2a"
    )


def test_repair_record_maps_the_finding_proofs_limits_and_wiring():
    record = tomllib.loads(Path(__file__).with_name("repair_22_33_98.kvx").read_text(encoding="utf-8"))
    repair = record["repair"]
    assert repair["task"] == "22.33.98" and repair["acceptance"] == "22998.1"
    assert repair["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert repair["build_cmd"] == "python3 --version"
    assert repair["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_98.py"
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "final passing hf-done row" in repair["verified_revision"]
    assert "same candidate revision" in repair["evidence"]
    assert "No pre-change verification was run" in repair["baseline"]
    assert "not executed" in repair["review_only"]
    finding = record["finding"]["a2a_profile_fixture"]
    assert finding["audit"] == "22.30.110" and finding["owner_domain"] == 11
    assert finding["file"] == "services/interop/specs/conformance/transport-a2a/role-messages.json"
    assert finding["class"] == "test-gap" and finding["severity"] == "medium"
    functions = {node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
                 if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    assert set(finding["tests"]) == functions
    assert len(repair["wiring_requests"]) == 3
    for request, path in zip(repair["wiring_requests"], (
            X402 + "src/model.rs", X402 + "tests/transports.rs", SERVICE + "server.rs"), strict=True):
        assert "domain 11" in request and path in request
    readme = (DIRECTORY / "README.md").read_text(encoding="utf-8")
    for fact in ("ProfileMissing", "typed_intent_required", "protocol_idempotency_required",
                 "do not execute Rust", "not consumed by the existing Rust", "observed_at_ms"):
        assert fact in readme
