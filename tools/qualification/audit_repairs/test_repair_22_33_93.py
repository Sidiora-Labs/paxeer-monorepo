"""Offline fixture and source contracts for 22.30.110/ucp_fixture_transaction_binding.

This gate does not execute the Go producer or Rust checkout/service journal.
It preserves their recorded bytes and checks the actual source connections;
runtime completion and service recovery are explicitly separated in the record.
"""

import ast
import hashlib
import json
from pathlib import Path
import tomllib
import uuid

import pytest


ROOT = Path(__file__).resolve().parents[3]
UCP = "services/interop/specs/conformance/ucp/sequenced-order.json"
PRODUCER = "sdk/platform/conformance/fixtures/ucp-checkout-receipt-v1.json"
CONSUMER = "services/interop/crates/layerx-ucp/tests/fixtures/sequenced-order.json"
RECORD = Path(__file__).with_name("repair_22_33_93.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def document(path):
    return json.loads(source(path))


def unhex(value):
    assert value.startswith("0x")
    return bytes.fromhex(value[2:])


def recorded_case(path, case_id):
    matches = [case for case in document(path)["cases"] if case["id"] == case_id]
    assert len(matches) == 1
    return matches[0]


def positive_vectors():
    vectors = document(UCP)
    assert [v["name"] for v in vectors] == [
        "receipted_order", "submitted_order", "tampered_log"
    ]
    return vectors[:2]


def rlp_item(data, offset=0):
    """Parse the recorded transaction's RLP, including its list boundaries."""
    assert offset < len(data)
    prefix = data[offset]
    if prefix < 0x80:
        return data[offset:offset + 1], offset + 1
    is_list = prefix >= 0xC0
    short_limit = 0xF7 if is_list else 0xB7
    short_base = 0xC0 if is_list else 0x80
    if prefix <= short_limit:
        length = prefix - short_base
        start = offset + 1
    else:
        size = prefix - short_limit
        length_bytes = data[offset + 1:offset + 1 + size]
        assert len(length_bytes) == size and length_bytes[0] != 0
        length = int.from_bytes(length_bytes, "big")
        assert length >= 56
        start = offset + 1 + size
    end = start + length
    assert end <= len(data)
    if not is_list:
        assert length != 1 or data[start] >= 0x80
        return data[start:end], end
    items = []
    cursor = start
    while cursor < end:
        item, cursor = rlp_item(data, cursor)
        items.append(item)
    assert cursor == end
    return items, end


def require_in_order(text, *facts):
    position = 0
    for fact in facts:
        position = text.index(fact, position) + len(fact)


def test_ucp_fixture_preserves_unmodified_node_recording():
    recording = document(PRODUCER)
    assert recording["chainId"] == 125
    assert recording["moduleId"] == 12
    assert recording["provenance"] == {
        "generator": "rpc/sequencer_receipt_vectors_test.go",
        "command": "go test -count=1 ./rpc/ -run '^TestRepairM_466_UCPCheckoutReceiptVector$' -update",
    }
    consumer_vectors = document(CONSUMER)
    for vector in positive_vectors():
        assert vector["source_case"] == "V1-ucp-checkout-pay-exact"
        recorded = recorded_case(PRODUCER, vector["source_case"])
        for field in ("rawTransaction", "signedReceipt", "pinnedKey"):
            assert vector[field] == recorded[field], field
        assert vector["evidence"]["transaction_hash"] == recorded["receiptJson"]["txHash"]
        assert vector["evidence"]["logs"] == recorded["receiptJson"]["logs"]
        assert vector == next(v for v in consumer_vectors if v["name"] == vector["name"])


@pytest.mark.parametrize("index", [0, 1], ids=["receipted", "submitted"])
def test_ucp_fixture_transaction_binding(index):
    vector = positive_vectors()[index]
    checkout = vector["checkout"]
    raw = unhex(vector["rawTransaction"])
    assert raw[0] == 2
    tx, end = rlp_item(raw, 1)
    assert end == len(raw) and len(tx) == 12
    assert int.from_bytes(tx[0], "big") == 125
    assert tx[5] == unhex("0x000000000000000000000000000000000000101a")
    assert tx[6] == b"" and tx[8] == []
    assert tx[9] in (b"", b"\x01")
    assert all(len(part) == 32 and int.from_bytes(part, "big") > 0 for part in tx[10:12])
    calldata = tx[7]
    assert len(calldata) == 196 and calldata[:4] == bytes.fromhex("78ea11db")
    words = [calldata[start:start + 32] for start in range(4, 196, 32)]
    key = hashlib.sha256(
        b"LayerX/interop/ucp/idempotency/v1\0" + uuid.UUID(checkout["idempotency_key"]).bytes
    ).digest()
    assert words[5] == key
    logs = vector["evidence"]["logs"]
    assert len(logs) == 1
    log = logs[0]
    assert unhex(log["address"]) == tx[5] and log["logIndex"] == "0x0"
    assert len(log["topics"]) == 4
    assert log["topics"][0] == "0xc4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1"
    assert [unhex(t) for t in log["topics"][1:]] == [key, words[0], words[1]]
    assert words[1] == unhex(checkout["layerx_recipient"])
    assert words[2] == unhex(checkout["layerx_asset"])
    assert unhex(log["data"]) == words[2] + words[3]
    assert checkout["currency"] == "USD" and checkout["total_minor"] == "100"
    assert int.from_bytes(words[3], "big") == int(checkout["total_minor"]) * 10 ** (6 - 2)
    recorded = recorded_case(PRODUCER, vector["source_case"])
    assert int.from_bytes(words[4], "big") > int(recorded["receiptJson"]["timestamp"], 16)


def test_ucp_fixture_authenticated_receipt_and_expected_order():
    for vector in positive_vectors():
        recorded = recorded_case(PRODUCER, vector["source_case"])
        native = recorded["receiptJson"]
        signed = unhex(vector["signedReceipt"])
        assert signed == unhex(native["receipt"])
        assert signed[:10] == bytes.fromhex("00025201000200000020")
        assert signed[10:42] == unhex(vector["evidence"]["transaction_hash"])
        assert signed[-69:-64] == bytes.fromhex("0100000040")
        digest = "0x" + hashlib.sha256(b"LXP/v1/receipt\0" + signed[:-69] + b"\0").hexdigest()
        assert digest == recorded["receiptDigest"]
        assert hashlib.sha256(unhex(native["logs"][0]["data"])).digest() in signed[:-69]
        key = vector["pinnedKey"]
        assert key["keyId"] == native["keyId"]
        assert key["publicKey"] == native["signingKey"]
        assert key["firstHeight"] <= int(native["height"], 16) <= key["lastHeight"]
        assert native["rung"] == "receipted" and native["status"] == "0x1"
        if vector["name"] == "submitted_order":
            assert vector["evidence"]["rung"] == "submitted"
            assert vector["expected"] == {
                "status": "complete_in_progress", "order": None,
                "stored_order": None, "refusal": None,
            }
        else:
            assert vector["evidence"]["rung"] == "receipted"
            checkout, metadata = vector["checkout"], vector["metadata"]
            assert vector["expected"] == {
                "status": "completed",
                "order": {
                    "id": metadata["order_id"], "checkout_id": checkout["id"],
                    "permalink_url": metadata["permalink_url"],
                    "currency": checkout["currency"], "total_minor": checkout["total_minor"],
                    "receipt_digest": digest, "transaction_hash": native["txHash"],
                },
                "stored_order": {
                    "checkout": checkout, "metadata": metadata,
                    "transaction_hash": native["txHash"],
                    "signed_receipt": recorded["signedReceipt"],
                    "logs": native["logs"], "rung": native["rung"],
                },
                "refusal": None,
            }


def test_ucp_fixture_preserves_tampered_log_refusal():
    vector = document(UCP)[2]
    recorded = recorded_case(
        "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", "V3-tampered-log"
    )
    assert vector["name"] == "tampered_log"
    assert vector["source_case"] == recorded["id"]
    for field in ("rawTransaction", "signedReceipt", "pinnedKey"):
        assert vector[field] == recorded[field]
    assert vector["evidence"]["logs"] == recorded["receiptJson"]["logs"]
    assert vector["evidence"]["transaction_hash"] == recorded["receiptJson"]["txHash"]
    assert vector["expected"] == {
        "status": None, "order": None, "stored_order": None, "refusal": "ReceiptMismatch"
    }
    # The producer intentionally changed the JSON log while retaining the signature.
    changed_digest = hashlib.sha256(unhex(vector["evidence"]["logs"][0]["data"])).digest()
    assert changed_digest not in unhex(vector["signedReceipt"])[:-69]


def test_ucp_real_producer_transport_consumer_source_contract():
    producer = source("rpc/sequencer_receipt_vectors_test.go")
    generation = producer.split("func TestRepairM_466_UCPCheckoutReceiptVector(", 1)[1].split(
        "func seqVecReadUCPCheckout(", 1
    )[0]
    require_in_order(
        generation, "pay.reference = seqVecUCPGatewayKey(t)",
        "tx := seqVecLedgerPaymentFor(t, 0, pay)",
        "b.receipt(key, seqVecKeyID, b.signTx(tx), tx,",
        "answer := seqVecServe(t, port, source, signed)",
        "seqVecAssertUCPCheckout(t, c)", "seqVecWriteFixture(",
    )
    consumer = source("services/interop/crates/layerx-ucp/src/lib.rs")
    assert 'b"LayerX/interop/ucp/idempotency/v1\\0"' in consumer
    require_in_order(
        consumer.split("pub fn complete_checkout(", 1)[1],
        "submission.payment_intent(&config)", "plane.execute(&intent, trace)",
        "verify_order_evidence(&evidence, receipt_keys)",
        "verify_sequencer_receipt_with_config(", ".settle_with_sequencer_evidence(",
        "let stored = StoredOrder {", "Ok(completed_outcome(stored, receipt_digest))",
    )
    require_in_order(
        consumer.split("pub fn read_order_with_config(", 1)[1],
        "config.atomic_amount(&stored.submission)?", "verify_order_evidence(&evidence, receipt_keys)?",
        "verify_sequencer_receipt_with_config(", "Ok(order(stored, receipt_digest))",
    )
    require_in_order(
        consumer.split("fn verify_order_evidence(", 1)[1],
        "verify_sequencer_signature(", "protocol.activity_id() != evidence.transaction_hash",
        "verify_logs(&verified, evidence.logs.clone())",
    )
    require_in_order(
        consumer.split("pub fn verify_sequencer_receipt_with_config(", 1)[1],
        "let amount = config.atomic_amount(submission)?", "verify_sequencer_signature(",
        "let reference = submission.idempotency_key.gateway_key()",
        "Sha256::digest(payment_data)", "if matches_payment_exact(",
        "if !matched", "canonical_receipt_digest(canonical_receipt)",
    )
    config = source("services/interop/crates/layerx-ucp/src/config.rs")
    assert "self.atomic_decimals - self.minor_decimals" in config
    assert ".checked_mul(multiplier)" in config
    assert ".ok_or(UcpError::CurrencyBindingRequired)?" in config


def test_ucp_reopened_replay_source_contract():
    """Inspect the real Rust consumer regression; do not claim Rust execution."""
    recovery = source("services/interop/crates/layerx-ucp/tests/support/producer_checkout.rs")
    assert "sdk/platform/conformance/fixtures/ucp-checkout-receipt-v1.json" in recovery
    require_in_order(
        recovery, "fn submit(", "OpenOptions::new().write(true).create_new(true).open(path)",
        "file.sync_all().unwrap()", 'fs::File::open(&self.directory).unwrap().sync_all().unwrap()',
        "pub fn producer_checkout(", "store.submit(source, &native_calldata)",
        "CheckoutStatus::CompleteInProgress", "drop(payment)", "drop(gateway)", "drop(store)",
        "RecordedSubmission::open(&directory.0)", "let recovered = store.read()",
        'assert_eq!(recovered["producer"], *source)',
        'store.submit(&recovered["producer"], &native_calldata)',
        "evidence: recorded_evidence(&recovered[\"producer\"])",
        "UcpAdapter::complete_checkout(", "CheckoutStatus::Completed",
        "UcpAdapter::read_order_with_config(&stored, &keys, &config)",
        "let retry = UcpAdapter::complete_checkout(",
        "assert_eq!(retry.order, Some(order))", "assert_eq!(retry.stored_order, Some(stored))",
        "assert_eq!(store.submission_count(), 1)",
    )
    vectors = source("services/interop/crates/layerx-ucp/tests/sequenced_vectors.rs")
    assert 'include_str!("fixtures/sequenced-order.json")' in vectors
    assert 'MerchantCurrencyBinding::new(submission.currency, submission.layerx_asset, 2, 6)' in vectors
    assert 'vector["signedReceipt"], source["signedReceipt"]' in vectors
    assert 'vector["evidence"]["logs"], source["receiptJson"]["logs"]' in vectors
    assert "UcpAdapter::complete_checkout(" in vectors
    assert "UcpAdapter::read_order_with_config(" in vectors


def test_ucp_repair_record_names_finding_proofs_and_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert record["repair"]["task"] == "22.33.93"
    assert set(record["finding"]) == {"ucp_fixture_transaction_binding"}
    finding = record["finding"]["ucp_fixture_transaction_binding"]
    assert finding["path"] == UCP
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_ucp_repair_record_names_finding_proofs_and_limits"
    }
    assert record["proof"]["minor_decimals"] == 2
    assert record["proof"]["atomic_decimals"] == 6
    assert record["qualification"]["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_93.py"
    )
    assert record["proof"]["runtime_executed"] is False
    assert "Domain 11" in record["wiring"]["durable_service_replay"]
    assert "services/interop/crates/layerx-interop-service" in record["wiring"]["durable_service_replay"]
