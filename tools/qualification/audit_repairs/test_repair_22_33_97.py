"""Offline fixture/source proofs; Rust execution remains an explicit wiring request."""

import ast
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import tomllib


ROOT = Path(__file__).resolve().parents[3]
SUITE = "services/interop/specs/conformance/transport-mcp"
X402 = "services/interop/crates/layerx-x402"
SERVICE = "services/interop/crates/layerx-interop-service/src"
GATEWAY = "services/interop/crates/layerx-interop-gateway/src/server.rs"
PRODUCER = "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
CASE_ID = "V1-layerxledger-pay-exact"
RECORD = Path(__file__).with_name("repair_22_33_97.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def vectors():
    return json.loads(source(f"{SUITE}/role-messages.json"))


def producer_case():
    corpus = json.loads(source(PRODUCER))
    assert corpus["chainId"] == 125
    cases = [case for case in corpus["cases"] if case["id"] == CASE_ID]
    assert len(cases) == 1
    return cases[0]


def offers(vector):
    document = vector["document"]
    if vector["role"] == "payment-required":
        return document["accepts"]
    if vector["role"] == "payment-payload":
        return [document["accepted"]]
    if vector["role"] == "facilitator-request":
        return [
            document["paymentRequirements"],
            document["paymentPayload"]["accepted"],
        ]
    return []


def compact(text):
    return re.sub(r"\s+", " ", text).strip()


def rust_body(text, declaration):
    """Extract balanced source blocks for assertions, without executing Rust."""
    start = text.index("{", text.index(declaration))
    depth = 1
    for end in range(start + 1, len(text)):
        depth += (text[end] == "{") - (text[end] == "}")
        if depth == 0:
            return text[start + 1 : end]
    raise AssertionError(f"Unclosed source block: {declaration}")


def rlp_item(data, offset=0):
    """Read committed RLP bytes to inspect fixture facts, not validate a payment."""
    assert offset < len(data)
    prefix = data[offset]
    if prefix < 0x80:
        return data[offset : offset + 1], offset + 1
    is_list = prefix >= 0xC0
    short = 0xC0 if is_list else 0x80
    if prefix < short + 56:
        size = prefix - short
        start = offset + 1
    else:
        width = prefix - short - 55
        length = data[offset + 1 : offset + 1 + width]
        assert len(length) == width and length[0] != 0
        size = int.from_bytes(length)
        assert size >= 56
        start = offset + 1 + width
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


def test_mcp_profile_fixture_preserves_envelope_cases():
    records = vectors()
    assert len(records) == 11
    assert len({record["name"] for record in records}) == len(records)
    original = records[:8]
    keys = {
        "name", "transport", "role", "accepted", "representation", "header", "document"
    }
    retained = [{k: v for k, v in row.items() if k in keys} for row in original]
    encoded = json.dumps(retained, sort_keys=True, separators=(",", ":")).encode()
    assert hashlib.sha256(encoded).hexdigest() == (
        "80bcd3727a517997c445c02624bc3f835af241ec32ff25ad3dcde08b377c4411"
    )
    for record in records:
        assert record["transport"] == "mcp"
        assert len(json.dumps(record["document"]).encode()) <= 64 * 1024
    for record in original:
        assert record["validationScope"] == "envelope-only"
        expectation = record["semanticExpectation"]
        if offers(record):
            assert expectation["requirements"] == "ProfileMismatch"
            assert expectation["binding"] == "refuse"
            for offer in offers(record):
                assert offer["network"] == "eip155:125"
                assert offer["extra"]["layerx"]["account"]
        else:
            assert expectation["settlement"] == "not-proven"
        assert expectation["reason"]


def test_mcp_profile_fixture_current_offer_matches_producer():
    case = producer_case()
    log = case["receiptJson"]["logs"][0]
    expected = {
        "scheme": "exact",
        "network": "eip155:125",
        "amount": str(int(log["data"][-64:], 16)),
        "asset": log["data"][2:66],
        "payTo": log["topics"][3][2:],
        "maxTimeoutSeconds": 60,
    }
    current = [row for row in vectors() if row["validationScope"] == "current-offer"]
    assert {row["name"] for row in current} == {
        "eip155_125_payment_required",
        "eip155_125_payment_payload",
        "eip155_125_facilitator_request",
    }
    for record in current:
        assert record["accepted"] is True
        assert record["representation"] == "json"
        assert record["document"]["x402Version"] == 2
        assert record["semanticExpectation"]["requirements"] == "accept"
        assert record["producerFixture"] == {
            "path": PRODUCER,
            "case": CASE_ID,
            "observedAtMillis": int(case["receiptJson"]["timestamp"], 16),
            "transactionHash": case["receiptJson"]["txHash"],
        }
        for offer in offers(record):
            assert offer == expected
            assert "extra" not in offer
        if record["role"] == "payment-required":
            assert record["document"]["resource"] == vectors()[0]["document"]["resource"]
            continue
        document = record["document"]
        if record["role"] == "facilitator-request":
            document = document["paymentPayload"]
        assert document["payload"] == {
            "transaction": case["rawTransaction"],
            "layerxTransaction": case["rawTransaction"],
            "layerxIdempotencyKey": log["topics"][1][2:],
        }
        assert "authorization" not in document["payload"]


def test_mcp_profile_fixture_signed_payer_binds_receipt():
    generator = source("rpc/sequencer_receipt_vectors_test.go")
    signer = rust_body(generator, "func (b *seqVecBuilder) signTx(")
    assert "ethtypes.SignTx(ethtypes.NewTx(&ethtypes.DynamicFeeTx{" in signer
    assert "ChainID:   big.NewInt(seqVecChainID)" in signer
    assert f'accept("{CASE_ID}"' in generator
    generated_payment = rust_body(generator, "func seqVecLedgerPaymentFor(")
    assert "data: seqVecPayExactCall(t, pay)" in generated_payment
    assert "seqVecLog(seqVecLedger, data, topic, pay.reference, pay.from, pay.to)" in generated_payment
    case = producer_case()
    raw = bytes.fromhex(case["rawTransaction"][2:])
    assert raw[0] == 2
    fields, end = rlp_item(raw, 1)
    assert end == len(raw) and len(fields) == 12
    assert int.from_bytes(fields[0]) == 125
    assert fields[5] == bytes.fromhex("000000000000000000000000000000000000101a")
    assert fields[6] == b"" and fields[8] == []
    assert fields[9] in (b"", b"\x01")
    for signature in fields[10:]:
        assert len(signature) == 32 and int.from_bytes(signature) > 0
    calldata = fields[7]
    assert len(calldata) == 4 + 6 * 32
    assert calldata[:4] == bytes.fromhex("78ea11db")
    words = [calldata[start : start + 32] for start in range(4, len(calldata), 32)]
    receipt = case["receiptJson"]
    log = receipt["logs"][0]
    assert log["address"] == receipt["to"] == "0x" + fields[5].hex()
    assert log["topics"][1:] == ["0x" + words[i].hex() for i in (5, 0, 1)]
    assert bytes.fromhex(log["data"][2:]) == words[2] + words[3]
    assert int.from_bytes(words[3]) == 1000000
    assert int.from_bytes(words[4]) > int(receipt["timestamp"], 16)
    assert receipt["status"] == "0x1" and receipt["rung"] == "receipted"
    assert receipt["receipt"] == case["signedReceipt"]
    signed = bytes.fromhex(case["signedReceipt"][2:])
    assert bytes.fromhex(receipt["txHash"][2:]) in signed
    assert bytes.fromhex("".join(topic[2:] for topic in log["topics"])) in signed
    assert hashlib.sha256(bytes.fromhex(log["data"][2:])).digest() in signed
    height = int(receipt["height"], 16)
    assert case["pinnedKey"]["firstHeight"] <= height <= case["pinnedKey"]["lastHeight"]
    assert case["expected"] == {"rung": "receipted", "verifier": "accepts"}


def test_mcp_profile_fixture_semantic_source_contract():
    model = source(f"{X402}/src/model.rs")
    requirements = rust_body(model, "impl PaymentRequirements")
    validate = compact(rust_body(requirements, "pub fn validate"))
    assert "if self.is_layerx_network() { self.layerx_terms()?; } else if" in validate
    assert "layerx_profile(self.extra.as_ref(), parse_hex32(&self.asset).ok().as_ref())?" in validate
    assert ".is_some() { return Err(X402Error::ProfileMismatch); }" in validate
    assert "self.amount.value() == 0 || self.max_timeout_seconds == 0" in validate
    for model_name in ("PaymentRequired", "PaymentPayload"):
        body = rust_body(model, f"impl {model_name}")
        assert "requirements.validate()?" in body or "self.accepted.validate()?" in body
    transport = source(f"{X402}/src/transport.rs")
    for role in ("payment_required", "payment_payload", "facilitator_request"):
        assert "value.validate()?;" in rust_body(transport, f"pub fn encode_{role}")
        assert "decoded.validate()?;" in rust_body(transport, f"pub fn decode_{role}")
    # Source-only evidence of the existing mismatch; no runtime success claimed.
    facilitator = rust_body(source(f"{X402}/src/facilitator.rs"), "impl FacilitatorRequest")
    assert "self.payment_requirements.validate()?;" in facilitator
    assert "self.payment_requirements.layerx_terms()?;" in facilitator
    terms = rust_body(requirements, "pub fn layerx_terms")
    assert ".ok_or(X402Error::ProfileMissing)?" in terms
    current = next(row for row in vectors() if row["name"] == "eip155_125_facilitator_request")
    expectation = current["semanticExpectation"]
    assert expectation["binding"] == "refuse"
    assert expectation["facilitator"] == "ProfileMissing"
    assert expectation["mountedStatus"] == 400
    assert expectation["mountedError"] == "invalid_x402_request"
    seller = rust_body(source(f"{X402}/src/seller.rs"), "impl Seller")
    assert "accepted.layerx_terms()?;" in rust_body(seller, "pub fn new(")
    required = next(row for row in vectors() if row["name"] == "eip155_125_payment_required")
    assert required["semanticExpectation"]["seller"] == "ProfileMissing"
    assert required["semanticExpectation"]["mountedError"] == "invalid_x402_offer"


def test_mcp_profile_fixture_mounted_ingress_source_path():
    router = compact(source(GATEWAY))
    assert '.strip_prefix("/v1/")' in router
    assert '"mcp" => Ok(IngressTransport::Mcp)' in router
    for tail, route in (
        ('["seller", "offer"]', "X402SellerOffer"),
        ('["verify"] | ["facilitator", "verify"]', "X402Verify"),
        ('["settle"] | ["seller" | "facilitator", "settle"]', "X402Settle"),
    ):
        assert f'("POST", transport, "x402", {tail}) => {{ InteropRoute::{route}' in router
    server = source(f"{SERVICE}/server.rs")
    assert "IngressTransport::Mcp => TransportKind::Mcp" in server
    seller = rust_body(server, "fn x402_seller(")
    assert "Seller::new(required)" in seller and "seller.payment_required()" in seller
    assert 'Dispatch::error(400, "refused", "invalid_x402_offer")' in seller
    ingress = rust_body(server, "fn x402_request(")
    assert "decode_facilitator_request(transport_kind(transport), &TransportValue::Json(value))" in ingress
    for function in ("x402_verify", "x402_settle"):
        body = compact(rust_body(server, f"fn {function}("))
        assert 'return Dispatch::error(400, "refused", "invalid_x402_request")' in body
        assert body.index("x402_request(request, transport)") < body.index("Facilitator::new(")
    verified = rust_body(server, "fn verified_x402_transfer_reference(")
    for guard in (
        "decode_pay_exact(&transaction)", "transfer.reference != expected_reference",
        "transfer.asset != asset", "transfer.to != recipient",
        "transfer.amount != request.payment_requirements.amount.value()",
        "payment_expired(transfer.expires_at, observed_at)",
    ):
        assert guard in verified
    plane = rust_body(server, "impl FacilitatorPlane for X402SettlePlane")
    assert plane.index("verified_x402_transfer(&request") < plane.index(".relay(&hex(")
    payer = rust_body(source(f"{SERVICE}/payer_tx.rs"), "pub(crate) fn decode_pay_exact(")
    assert "transaction.chain_id != crate::config::PAXEER_CHAIN_ID" in payer
    assert "recover_sender(" in payer
    for record in vectors()[8:]:
        if record["role"] == "payment-required":
            assert record["ingress"]["paths"] == ["/v1/mcp/x402/seller/offer"]
            assert record["ingress"]["expectedStatus"] == 400
        elif record["role"] == "facilitator-request":
            assert record["ingress"]["paths"] == [
                "/v1/mcp/x402/facilitator/verify", "/v1/mcp/x402/facilitator/settle"
            ]
        else:
            continue
        assert record["ingress"]["method"] == "POST"
        assert record["ingress"]["contentType"] == "application/json"


def test_mcp_profile_fixture_production_renderer_pins_current_suite():
    path = ROOT / "services/interop/deploy/gateway/render.py"
    spec = importlib.util.spec_from_file_location("mcp_repair_gateway_render", path)
    renderer = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(renderer)
    name, count, digest = renderer.first_party_suite(ROOT, "transport-mcp")
    assert name == renderer.SUITE_NAME % "transport-mcp"
    assert count == len(vectors()) == 11
    data = (ROOT / SUITE / "role-messages.json").read_bytes()
    expected = hashlib.sha256(b"role-messages.json\x00" + data + b"\x00").hexdigest()
    assert digest == expected
    consumer = source(f"{X402}/tests/transports.rs")
    assert 'include_str!("../../../specs/conformance/transport-mcp/role-messages.json")' in consumer


def test_mcp_profile_fixture_recorded_evidence_and_wiring():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["mcp_profile_fixture"]
    assert finding["audit"] == "22.30.110" and finding["owner_domain"] == 11
    assert finding["file"] == f"{SUITE}/role-messages.json"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["97"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_97.py"
    )
    assert task["baseline"] and task["source_proof"] and task["review_only"]
    assert task["runtime_closure"] == "pending owning-domain wiring"
    assert {row["path"] for row in record["wiring"].values()} == {
        f"{X402}/tests/transports.rs", f"{X402}/src/facilitator.rs",
        f"{X402}/src/seller.rs", f"{SERVICE}/server.rs",
        "services/interop/deploy/gateway/render.py",
    }
    for row in record["wiring"].values():
        assert row["owner_domain"] == 11 and row["request"]
    readme = source(f"{SUITE}/README.md")
    for fact in ("ProfileMismatch", "ProfileMissing", "does not", "owning-domain wiring"):
        assert fact in readme
