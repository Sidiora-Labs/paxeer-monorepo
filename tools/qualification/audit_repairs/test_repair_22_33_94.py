"""Authenticate chain recordings and source contracts; native runtime is separate."""

import ast
from copy import deepcopy
from hashlib import sha256
import importlib.util
import json
from pathlib import Path
import sys
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))

from layerx_sdk.evm_transaction import decode_signed
from layerx_sdk.verifier import (
    ReceiptKey,
    SequencerReceiptVerificationError,
    verify_sequencer_receipt,
)


FIAT = ROOT / "services/interop/specs/conformance/fiat"
UNBOUND_FIXTURE_PATH = "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
JOURNEYS = json.loads((FIAT / "sequenced-journeys.json").read_text())
UNBOUND_FIXTURE = json.loads((ROOT / UNBOUND_FIXTURE_PATH).read_text())
CREDITS = [journey for journey in JOURNEYS if journey["expect"].get("state") == "credited"]
SPEC = importlib.util.spec_from_file_location(
    "fiat_recording_importer", FIAT / "record_sequenced_journeys.py"
)
IMPORTER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(IMPORTER)


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def recorded_case(journey):
    fixture = json.loads((ROOT / journey["sequencer"]["fixture"]).read_text())
    matches = [case for case in fixture["cases"] if case["id"] == journey["sequencer"]["case"]]
    assert len(matches) == 1
    return matches[0]


def pinned_key(case):
    pin = case["pinnedKey"]
    return ReceiptKey(
        IMPORTER.unhex(pin["keyId"]),
        IMPORTER.unhex(pin["publicKey"]),
        pin["firstHeight"],
        pin["lastHeight"],
    )


@pytest.mark.parametrize("journey", CREDITS, ids=lambda journey: journey["name"])
def test_fiat_fixture_transaction_binding(journey):
    """Closure requires original submissions, never receipt-only re-signing."""
    case = recorded_case(journey)
    digest = IMPORTER.validate_capture(journey, case)
    declared = journey["sequencer"]
    assert declared["transaction_hash"] == case["receiptJson"]["txHash"]
    assert (
        declared["receipt_digest"]
        == case["receiptDigest"]
        == journey["expect"]["receipt_digest"]
        == digest
    )
    call = IMPORTER.payment_call(case)
    assert (
        declared["payment_reference"]
        == declared["idempotency_key"]
        == "0x" + call[164:196].hex()
    )
    assert declared["payment_calldata"] == "0x" + call.hex()
    assert IMPORTER.unhex(declared["counterparty"]) == call[4:36]
    assert declared["expires_at"] == int.from_bytes(call[156:164], "big")
    sequence = int(case["receiptJson"]["seq"], 16)
    assert sequence > 0
    raw = IMPORTER.unhex(case["rawTransaction"])
    transaction, signature = decode_signed(raw)
    assert transaction.data == call
    assert (
        transaction.recover_signer(signature).lower()
        == case["receiptJson"]["from"].lower()
    )
    assert transaction.attach_signature(signature, case["receiptJson"]["from"]) == raw
    verified = verify_sequencer_receipt(
        IMPORTER.unhex(case["signedReceipt"]), raw, [pinned_key(case)]
    )
    assert verified.receipt_digest == IMPORTER.unhex(digest)
    assert verified.receipt.activity_id == IMPORTER.unhex(declared["transaction_hash"])
    assert verified.receipt.global_sequence == sequence
    assert verified.receipt.previous_state_root == IMPORTER.unhex(case["prevSeqHash"])
    assert verified.height == int(case["receiptJson"]["height"], 16)
    assert journey["recorder"] == IMPORTER.RECORDER
    assert "test_signing_seed" not in declared
    assert "source_receipt_digest" not in declared


def test_provider_reference_matches_production_encoder_and_all_three_rails():
    assert len(JOURNEYS) == 5
    assert {journey["provider_facts"]["rail"] for journey in CREDITS} == {"card", "bank", "rtp"}
    assert len(CREDITS) == 3
    references = set()
    for journey in CREDITS:
        reference = IMPORTER.provider_reference(journey)
        assert "0x" + reference.hex() == journey["sequencer"]["payment_reference"]
        references.add(reference)
        facts = journey["provider_facts"]
        assert "0x" + sha256(journey["token"].encode()).hexdigest() == facts["token_reference_sha256"]
    assert len(references) == 3
    production = source("services/interop/crates/layerx-fiat/src/lib.rs")
    key = production.rsplit("fn idempotency_key(facts:", 1)[1].split("\n}", 1)[0]
    for fact in (
        "hash.update(IDEMPOTENCY_DOMAIN)",
        "hash.update(facts.provider.as_str().as_bytes())",
        "hash.update([0])",
        "hash.update(facts.settlement.as_str().as_bytes())",
        "hash.update([facts.rail.code(), facts.class.code()])",
    ):
        assert fact in key
    assert 'b"LayerX/interop/fiat/idempotency/v1\\0"' in production
    assert "data[164..196].copy_from_slice(&self.idempotency_key)" in production


def test_importer_refuses_original_unbound_producer_capture_without_mutation(tmp_path):
    original = next(
        case for case in UNBOUND_FIXTURE["cases"]
        if case["id"] == "V1-layerxledger-pay-exact"
    )
    call = IMPORTER.payment_call(original)
    log = original["receiptJson"]["logs"][0]
    assert call[164:] == IMPORTER.unhex(log["topics"][1])
    assert call[4:36] == IMPORTER.unhex(log["topics"][2])
    assert call[36:68] == IMPORTER.unhex(log["topics"][3])
    assert call[68:132] == IMPORTER.unhex(log["data"])
    before = deepcopy((JOURNEYS, UNBOUND_FIXTURE))
    for journey in CREDITS:
        assert call[164:] != IMPORTER.provider_reference(journey)
        with pytest.raises(ValueError, match="submitted reference is not provider-derived"):
            IMPORTER.validate_capture(journey, original)
    # Retain the real generic producer bytes under the journey's declared case:
    # rejection must concern its reference, rather than a missing case lookup.
    unbound = deepcopy(original)
    unbound["id"] = CREDITS[0]["sequencer"]["case"]
    path = tmp_path / CREDITS[0]["sequencer"]["fixture"]
    path.parent.mkdir(parents=True)
    path.write_text(json.dumps({"chainId": 125, "cases": [unbound]}))
    snapshot = path.read_bytes()
    with pytest.raises(ValueError, match="submitted reference is not provider-derived"):
        IMPORTER.regenerate(JOURNEYS, tmp_path)
    assert path.read_bytes() == snapshot
    assert (JOURNEYS, UNBOUND_FIXTURE) == before


def test_importer_resolves_each_declared_fixture_and_preserves_originals(tmp_path):
    journeys = deepcopy(JOURNEYS)
    snapshots = {}
    for index, journey in enumerate(journeys):
        case = recorded_case(journey)
        fixture = json.loads((ROOT / journey["sequencer"]["fixture"]).read_text())
        assert fixture["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
        assert (
            fixture["provenance"]["source"]
            == "daemon/committed-fixture/vectors/vector-v4-committed.json"
        )
        assert (
            sha256((ROOT / fixture["provenance"]["source"]).read_bytes()).hexdigest()
            == fixture["provenance"]["sha256"]
        )
        # Separate real captures prove that a common fixture cannot be reused.
        journey["sequencer"]["fixture"] = f"capture-{index}.json"
        fixture["cases"] = [case]
        path = tmp_path / journey["sequencer"]["fixture"]
        path.write_text(json.dumps(fixture))
        snapshots[path] = path.read_bytes()
    before = deepcopy(journeys)
    assert IMPORTER.regenerate(journeys, tmp_path) == journeys
    assert journeys == before
    assert all(path.read_bytes() == data for path, data in snapshots.items())
    assert IMPORTER.regenerate(JOURNEYS, ROOT) == JOURNEYS
    old_recorder = ROOT / "services/interop/crates/layerx-fiat/record_sequenced_journeys.py"
    assert not old_recorder.exists()


@pytest.mark.parametrize("journey", CREDITS, ids=lambda journey: journey["name"])
def test_importer_and_production_verifier_reject_changed_receipt_signature(journey):
    case = deepcopy(recorded_case(journey))
    signed = bytearray(IMPORTER.unhex(case["signedReceipt"]))
    signed[-1] ^= 1
    case["signedReceipt"] = case["receiptJson"]["receipt"] = "0x" + signed.hex()
    with pytest.raises(ValueError, match="invalid sequencer receipt signature"):
        IMPORTER.validate_capture(journey, case)
    with pytest.raises(SequencerReceiptVerificationError) as failure:
        verify_sequencer_receipt(
            bytes(signed), IMPORTER.unhex(case["rawTransaction"]), [pinned_key(case)]
        )
    assert failure.value.check == "bad_signature"


def test_submitted_and_reverted_recordings_preserve_their_original_evidence():
    pending, reverted = JOURNEYS[3:]
    assert pending["name"] == "card_submitted_waits"
    assert pending["sequencer"]["rung"] == "submitted"
    assert pending["sequencer"]["receipt_digest"] is None
    assert pending["expect"] == {"state": "credit-pending", "translation": "pending"}
    assert reverted["name"] == "card_reverted_never_credits"
    case = recorded_case(reverted)
    assert case["receiptJson"]["status"] == "0x0"
    assert case["receiptJson"]["logs"] == []
    assert reverted["sequencer"]["transaction_hash"] == case["receiptJson"]["txHash"]
    assert reverted["sequencer"]["receipt_digest"] == case["receiptDigest"]
    assert reverted["expect"] == {"refusal": "ReceiptMismatch", "translation": "pending"}


def test_production_callback_transport_and_credit_checks_remain_required():
    server = source("services/interop/crates/layerx-interop-service/src/server.rs")
    callback = server.split("fn fiat(\n", 1)[1].split("\nfn fiat_executed", 1)[0]
    for fact in ("FiatEvidenceVerifier", "FiatAdapter::verify_evidence", "FiatServicePlane", "plane.apply("):
        assert fact in callback
    plane = server.split("impl FiatPlane for FiatServicePlane", 1)[1].split("\nfn fiat(", 1)[0]
    for fact in (
        "decode_pay_exact(&self.transaction)",
        "transfer.reference != intent.idempotency_key",
        "transfer.asset != intent.asset",
        "transfer.amount != intent.amount",
        '.relay(&hex(&intent.idempotency_key), &self.transaction, sender)',
    ):
        assert fact in plane
    adapter = source("services/interop/crates/layerx-fiat/src/lib.rs")
    for fact in (
        "verify_sequencer_signature(",
        "protocol.activity_id() != evidence.transaction_hash || protocol.result_code() != 0",
        "&& *reference == idempotency_key",
        "if matches != 1",
        ".settle_with_sequencer_evidence(",
    ):
        assert fact in adapter
    peer = source("services/interop/crates/layerx-fiat/tests/support/payment_node.rs")
    assert "not an Ethereum transaction" in peer
    assert '"format": "signed-fixture-payment-v1"' in peer


def test_production_intent_sets_provider_reference_before_submission():
    """Keep provider derivation ahead of the production submission path."""
    adapter = source("services/interop/crates/layerx-fiat/src/lib.rs")
    execute = adapter.rsplit("    fn execute(\n", 1)[1].split("\n    fn ", 1)[0]
    derive = execute.index("let idempotency_key = Self::idempotency_key(facts);")
    intent = execute.index("let intent = FiatIntent {")
    submit = execute.index("match plane.execute(intent, trace)")
    assert derive < intent < submit
    assert "            idempotency_key," in execute[intent:submit]
    encoder = adapter.split("    pub fn payment_calldata(", 1)[1].split("\n}\n", 1)[0]
    assert "data[164..196].copy_from_slice(&self.idempotency_key);" in encoder
    assert encoder.index("data[164..196]") < encoder.rindex("        data")

    peer = source("services/interop/crates/layerx-fiat/tests/support/payment_node.rs")
    submit_peer = peer.split("    pub fn submit(", 1)[1].split("\n    pub fn ", 1)[0]
    encode = submit_peer.index("intent.payment_calldata(PAYER, EXPIRES_AT)")
    sign = submit_peer.index("SigningKey::from_bytes(&TRANSACTION_SEED).sign(&raw)")
    send = submit_peer.index('self.rpc("fixture_submitPayment"')
    assert encode < sign < send
    assert "Sha256::digest(&raw)" in submit_peer
    assert "eth_sendRawTransaction" not in submit_peer


def test_repair_record_keeps_unproven_runtime_and_capture_work_open():
    record = tomllib.loads(Path(__file__).with_name("repair_22_33_94.kvx").read_text())
    finding = record["finding"]["fiat_fixture_transaction_binding"]
    assert finding["status"] == "repaired"
    declared_tests = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) <= declared_tests
    assert "test_fiat_fixture_transaction_binding" in finding["tests"]
    assert len(record["wiring"]) == 3
    assert record["wiring"]["chain_capture"]["status"] == "resolved by 22.31.489"
    assert record["wiring"]["vector_consumer"]["status"] == "recorder replaced in this task"
    assert "not executed" in record["qualification"]["runtime"]
    assert record["qualification"]["evidence_ledger"] == "/out/run/gates.jsonl"


def test_producer_finding_identifies_call_builder_and_transport_refusal():
    record = tomllib.loads(Path(__file__).with_name("repair_22_33_94.kvx").read_text())
    producer = record["producer_finding"]["fiat_fixture_transaction_binding"]
    for location in producer["locations"]:
        path, expected = location
        assert path in {
            "services/interop/crates/layerx-fiat/src/lib.rs",
            "services/interop/crates/layerx-interop-service/src/server.rs",
        }
        assert any(line.strip() == expected for line in source(path).splitlines())

    server = source("services/interop/crates/layerx-interop-service/src/server.rs")
    plane = server.split("impl FiatPlane for FiatServicePlane", 1)[1].split("\nfn fiat(", 1)[0]
    mismatch = plane.index("if transfer.reference != intent.idempotency_key")
    refusal = plane.index('self.refusal = Some("provider_payment_mismatch");', mismatch)
    reject = plane.index("return Ok(FiatPlaneResult::Open(PlaneFiatOutcome::Refused));", refusal)
    relay = plane.index('.relay(&hex(&intent.idempotency_key), &self.transaction, sender)')
    assert mismatch < refusal < reject < relay
    assert producer["adapter_defect_observed"] is False
    assert producer["capture_available"] is True
    assert producer["producer_repair"] == "22.31.489"
