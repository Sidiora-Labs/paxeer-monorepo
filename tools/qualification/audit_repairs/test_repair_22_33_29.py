"""Golden contracts plus the production Python SDK consuming recorded receipt bytes.

Rust producer execution is deliberately not claimed by this Python-only gate.
The colocated Rust tests compare the actual projections and serializers to these
fixtures; source assertions below keep those comparisons and routes connected.
"""

import ast
import base64
import binascii
import json
import re
import sys
import tomllib
from dataclasses import replace
from hashlib import sha256
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[3]
HUMAN = ROOT / "services/human"
GOLDEN = HUMAN / "schema/human-api/golden"
SERVER = HUMAN / "crates/layerx-human-service/src/server"
RECORD = Path(__file__).with_name("repair_22_33_29.kvx")
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))

from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import decode_signed
from layerx_sdk.verifier import (
    ReceiptKey,
    SequencerReceiptVerificationError,
    verify_sequencer_receipt,
)

BINDINGS = (
    "binding.rebind.action", "binding.rebind", "binding.statement",
    "binding.status", "binding.submit",
)
CURRENT = (
    "activity.entry.response.json", "activity.export.evidence.response.json",
    "evidence.get.response.json",
)
HISTORICAL_HASHES = {
    "activity.entry.response.json": "360bf3fe1c0ca6b40d021896dcb1ef0b4392582b3e007aabaf68968d616a3b7f",
    "activity.export.evidence.response.json": "30b3b863b42c6cde5a0357791d78f35e9039f0567ac0e4756443ddb3164116b6",
}


def source(path):
    return path.read_text(encoding="utf-8")


def function(path, name):
    """Extract a balanced Rust function body, without crossing sibling functions."""
    text = source(path)
    masked = re.sub(
        r'//[^\n]*|/\*.*?\*/|b?"(?:\\.|[^"\\])*"',
        lambda match: "".join("\n" if char == "\n" else " " for char in match[0]),
        text, flags=re.S,
    )
    start = re.search(r"\bfn " + re.escape(name) + r"\s*\(", masked)
    assert start, (path, name)
    opening = masked.index("{", start.end())
    depth = 1
    position = opening + 1
    while depth:
        assert position < len(text), (path, name)
        if masked[position] == "{":
            depth += 1
        elif masked[position] == "}":
            depth -= 1
        position += 1
    return text[opening + 1:position - 1]


def compact(text):
    return re.sub(r"\s+", "", text)


def require(text, *fragments):
    for fragment in fragments:
        assert compact(fragment) in compact(text), fragment


def section(path, name):
    match = re.search(r"^\[" + re.escape(name) + r"\]\n(.*?)(?=^\[|\Z)", source(path), re.M | re.S)
    assert match, (path, name)
    return tomllib.loads(match.group(1))


def golden(name):
    document = json.loads((GOLDEN / name).read_bytes())
    assert document["status"] == 200
    assert set(document["body"]) == {"ok", "result", "trace"}
    assert document["body"]["ok"] is True
    assert document["body"]["trace"].startswith("trc_")
    return document["body"]["result"]


def recorded_case():
    document = json.loads((ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_bytes())
    assert document["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    assert document["chainId"] == 125
    return next(case for case in document["cases"] if case["id"] == "V1-pax-transfer")


def unhex(value):
    assert value.startswith("0x")
    result = bytes.fromhex(value[2:])
    assert "0x" + result.hex() == value
    return result


def pinned_key():
    key = recorded_case()["pinnedKey"]
    return ReceiptKey(unhex(key["keyId"]), unhex(key["publicKey"]), key["firstHeight"], key["lastHeight"])


def receipt_material():
    material = golden("evidence.get.response.json")
    encoded = base64.b64decode(material["bytes_base64"], validate=True)
    assert base64.b64encode(encoded).decode() == material["bytes_base64"]
    portable = json.loads(encoded)
    assert set(portable) == {
        "format", "verificationLevel", "signedReceipt", "receiptDigest",
        "receiptKey", "inclusionHeight",
    }
    assert portable["format"] == "layerx-receipt-proof-v2"
    assert portable["verificationLevel"] == "receipted"
    return material, encoded, portable


@pytest.mark.parametrize("operation", BINDINGS)
def test_retired_binding_envelopes_match_producer(operation):
    constructor = function(SERVER / "backend.rs", "operation_retired")
    strings = dict(re.findall(r'(code|copy_key|retry):\s*"([^"]+)"\.to_owned\(\)', constructor))
    assert strings == {"code": "operation-retired", "copy_key": "error.operation.retired", "retry": "final"}
    assert re.search(r"status:\s*410", constructor)
    require(constructor, "retry_after_ms: None", "field: None")
    fixture = json.loads((GOLDEN / f"{operation}.failure.json").read_bytes())
    assert fixture["status"] == int(re.search(r"status:\s*(\d+)", constructor)[1])
    assert fixture["body"] == {"ok": False, "error": strings, "trace": fixture["body"]["trace"]}
    assert fixture["body"]["trace"].startswith("trc_")
    operation_schema = section(HUMAN / "schema/human-api/identity.kvx", f"operation.{operation}")
    assert operation_schema["retired"] == "true"
    assert operation_schema["path"].startswith("/v1/wallet-binding")
    http = source(SERVER / "http.rs")
    require(http, "if operation.retired() { return error_response(&trace, &ApiFailure::operation_retired()); }")
    assert http.index("if operation.retired()") < http.index("if operation.mutates()")
    require(function(SERVER / "http.rs", "error_response"),
            'failure.status', 'json!({ "ok": false, "error": failure.envelope(), "trace": trace.as_str() })')
    envelope = function(SERVER / "backend.rs", "envelope")
    require(envelope, 'if let Some(retry_after_ms) = self.retry_after_ms', 'if let Some(field) = &self.field')
    require(function(SERVER / "production_reads.rs", "golden_retired_binding_producer_equivalence"),
            'ApiFailure::operation_retired()', 'fixture["body"]["error"], failure.envelope()',
            'fixture["status"], failure.status', f'"{operation}"')


def test_evidence_get_real_sdk_verifier_round_trip():
    material, encoded, portable = receipt_material()
    case = recorded_case()
    signed = unhex(portable["signedReceipt"])
    assert signed == unhex(case["signedReceipt"]) == unhex(case["receiptJson"]["receipt"])
    assert portable["receiptDigest"] == case["receiptDigest"]
    key = pinned_key()
    assert portable["receiptKey"] == {
        "keyId": "0x" + key.key_id.hex(), "publicKey": "0x" + key.public_key.hex(),
        "firstHeight": hex(key.first_height), "lastHeight": hex(key.last_height),
    }
    # The actual SDK verifies canonical bytes, signature, key range and transaction binding.
    verified = verify_sequencer_receipt(signed, unhex(case["rawTransaction"]), [key])
    assert verified.canonical_bytes == signed
    assert verified.receipt_digest == unhex(portable["receiptDigest"])
    assert verified.height == int(portable["inclusionHeight"], 16) == material["inclusion_height"]
    assert verified.receipt.activity_id == unhex(case["receiptJson"]["txHash"])
    assert verified.receipt.activity_id == keccak256(unhex(case["rawTransaction"]))
    assert material["evidence_id"] == "evd_" + sha256(signed).hexdigest()
    assert material["class"] == "layerx-receipt"
    assert material["verification"] == "receipt-verified"
    assert material["rung"] == "receipted"
    assert material["content_type"] == "application/vnd.layerx.receipt-proof-v2+json"
    assert verified.receipt.result_code == 0
    assert json.dumps(portable, separators=(",", ":")).encode() == encoded


@pytest.mark.parametrize(("fault", "code"), [
    ("malformed", "bad_signature"), ("signature", "bad_signature"),
    ("wrong-key", "bad_signature"), ("unknown-key", "unpinned_key"),
    ("expired-key", "key_out_of_range"), ("wrong-transaction", "hash_mismatch"),
])
def test_evidence_get_preserves_negative_verifier_coverage(fault, code):
    _, _, portable = receipt_material()
    signed = unhex(portable["signedReceipt"])
    raw = unhex(recorded_case()["rawTransaction"])
    key = pinned_key()
    if fault == "malformed":
        signed = b"layerx-receipt-canonical-bytes"
    elif fault == "signature":
        signed = signed[:-1] + bytes([signed[-1] ^ 1])
    elif fault == "wrong-key":
        key = replace(key, public_key=bytes([255]) * 32)
    elif fault == "unknown-key":
        key = replace(key, key_id=bytes(32))
    elif fault == "expired-key":
        key = replace(key, first_height=int(portable["inclusionHeight"], 16) + 1)
    elif fault == "wrong-transaction":
        raw = raw[:-1] + bytes([raw[-1] ^ 1])
    with pytest.raises(SequencerReceiptVerificationError) as refused:
        verify_sequencer_receipt(signed, raw, [key])
    assert refused.value.check == code
    with pytest.raises(binascii.Error):
        base64.b64decode("not base64!", validate=True)
    with pytest.raises(json.JSONDecodeError):
        json.loads(b"{")


def test_evidence_route_producer_and_portable_consumer_remain_connected():
    path = SERVER / "production_reads.rs"
    execute = function(path, "execute")
    require(execute, '"evidence.get" => super::ProductionComponents::evidence_get(scope, request)')
    route = source(path).split("fn evidence_get(\n", 2)[2]
    require(route, 'journey_receipt(scope, expected)?', 'receipt_label(',
            'if verification != "receipt-verified"',
            'super::ProductionComponents::managed_receipt_material(id, &evidence)?')
    producer = function(path, "managed_receipt_material")
    require(producer, 'Some(evidence.receipt_digest)',
            'Sha256::digest(&evidence.canonical_receipt)',
            'evidence.canonical_receipt != evidence.signed_receipt',
            'Rung::Submitted => return Err(ApiFailure::upstream_degraded())',
            'Rung::Receipted => "receipt-verified"', 'Rung::Committed => "committed"',
            'PortableReceiptV2::export(&evidence.signed_receipt, &key_entry, evidence.inclusion_height,)',
            '"bytes_base64":STANDARD.encode(portable)', '"rung":evidence.rung.name()',
            '"inclusion_height":evidence.inclusion_height')
    portable_path = ROOT / "services/interop/crates/layerx-portable/src/receipt.rs"
    require(function(portable_path, "export"), 'verify_sequencer_signature(', 'portable.verify_v2(',
            'receipt_digest: encode_hex(&signed_receipt_digest(signed_receipt)?)')
    require(function(portable_path, "verify_v2"),
            'verify_sequencer_signature(&decoded.signed_receipt, receipt_keys)',
            'if *pinned != decoded.receipt_key', 'if decoded.inclusion_height < height',
            'if digest != decoded.receipt_digest', 'transaction_hash != protocol.activity_id()')
    rust_test = function(path, "golden_evidence_verifier_round_trip")
    require(rust_test, 'managed_receipt_material(&id, &evidence)', 'golden("evidence.get.response.json")',
            'schema.encode_response(', 'PortableReceiptV2::verify_json(&bytes, &pins)',
            'portable.to_json()', 'verified.activity_id(), evidence.tx_hash',
            'verified.inclusion_height(), evidence.inclusion_height',
            'portable.verify_v2(&wrong_key).is_err()', 'PortableReceiptV2::from_json(b"{").is_err()',
            'managed_receipt_material(&tampered_id, &tampered).is_err()')


def test_activity_entry_uses_verified_current_receipt_facts():
    entry = golden("activity.entry.response.json")
    material, _, portable = receipt_material()
    case = recorded_case()
    verified = verify_sequencer_receipt(unhex(portable["signedReceipt"]), unhex(case["rawTransaction"]), [pinned_key()])
    transaction, signature = decode_signed(unhex(case["rawTransaction"]))
    assert transaction.chain_id == 125
    assert transaction.recover_signer(signature).lower() == case["receiptJson"]["from"].lower()
    assert entry["kind"] == "movement" and entry["state"] == "done"
    assert entry["state_copy_key"] == "status.done"
    assert entry["summary_copy_key"] == "activity.move.summary"
    assert entry["money"] == {"amount": str(verified.receipt.amount), "currency": "HPX"}
    assert entry["fees"] == {"amount": str(verified.receipt.fee_charged), "currency": "HPX"}
    assert int(entry["fees"]["amount"]) == int(case["receiptJson"]["feeAmount"], 16)
    assert entry["evidence"] == [{key: material[key] for key in ("evidence_id", "class", "verification")}]
    for index, (stage, key) in enumerate(zip(entry["stages"], ["getting_ready", "sending", "processing", "done"], strict=True)):
        assert stage == {"stage_id": f"stg_{entry['entry_id']}_{index}", "copy_key": f"status.{key}", "state": "done", "evidence": []}
    producer = function(SERVER / "production_reads.rs", "activity_detail_json")
    require(producer, '"state": activity_status(detail.status())',
            '"state_copy_key": activity_state_key(detail.status())',
            'EvidenceKind::Receipt => "layerx-receipt"', '"verification":verification',
            '"amount": actual.amount().to_string()', '"amount": actual.fee().to_string()',
            '"copy_key": activity_stage_key(stage.label())?')
    entry_route = function(SERVER / "production_reads.rs", "activity_entry_json")
    require(entry_route, "activity_entry_json_with(scope, id, receipt_keys,",
            "GatewayEndpoint::from_environment()", "gateway.list_assets()")
    route = function(SERVER / "production_reads.rs", "activity_entry_json_with")
    require(route, 'Feed::entry(scope, id)', 'receipt_label_with_keys(scope, row, digest, &keys()?)?',
            'ReceiptActual::from_verified_journey_bytes(', 'EntryDetail::assemble(', 'activity_detail_json(')
    require(function(SERVER / "production_components.rs", "execute_activity_entry"),
            'super::production_reads::activity_entry(scope, request)')


def test_activity_export_matches_encoded_receipt_bundle_and_route():
    payload = (GOLDEN / "current/activity.evidence.bundle").read_bytes()
    position = 0

    def take(length):
        nonlocal position
        assert length >= 0 and position + length <= len(payload)
        value = payload[position:position + length]
        position += length
        return value

    def integer(width=4):
        return int.from_bytes(take(width), "big")

    def sized():
        return take(integer())

    entry = golden("activity.entry.response.json")
    export = golden("activity.export.evidence.response.json")
    material, _, portable = receipt_material()
    signed = unhex(portable["signedReceipt"])
    case = recorded_case()
    assert take(8) == b"LXHEXPV3" and integer(1) == 3
    assert take(32) == sha256(b"layerx-human-export-principal/v1golden-receipt").digest()
    assert integer() == 1
    assert sized().decode() == entry["entry_id"]
    assert integer() == 1
    reference = sized().decode()
    assert reference == sha256(signed).hexdigest()
    for protocol in (False, True):
        if protocol:
            assert integer() == 1  # protocol export count
            assert integer() == 1  # receipt count
            assert sized().decode() == f"activity {entry['entry_id']} receipt {reference}"
            transported = sized()
            assert transported == signed
            verified = verify_sequencer_receipt(transported, unhex(case["rawTransaction"]), [pinned_key()])
            assert verified.height == material["inclusion_height"]
        assert take(32) == unhex(case["receiptJson"]["txHash"])
        assert take(32) == sha256(signed).digest()
        assert integer(8) == material["inclusion_height"]
        assert integer(1) == 1  # receipted, no committed evidence
    assert integer(1) == 0 and integer(1) == 0  # commit and header-binding flags
    assert integer() == 0  # inclusion count
    assert integer(1) == 0  # audit flag
    assert position == len(payload)
    digest = sha256(payload).hexdigest()
    assert export["export_id"] == "exp_" + digest
    assert export["download_path"] == "/v1/evidence/evd_" + digest
    assert export["kind"] == "evidence-bundle"
    assert export["content_type"] == "application/vnd.layerx.evidence-bundle"
    assert export["evidence"] == entry["evidence"]
    components = SERVER / "production_components.rs"
    route = function(components, "execute_activity_export_evidence")
    require(route, 'EvidenceExport::new(', '.evidence(', 'bundle.encode()',
            'bundle.verify(digest, scope.principal(), &receipt_keys)',
            '"class":"layerx-receipt","verification":verification', 'cache_activity_export(scope, bytes, true, evidence, self.now()?)')
    cache = function(components, "cache_activity_export")
    require(cache, 'Sha256::digest(&bytes)', '"application/vnd.layerx.evidence-bundle"',
            '"download_path":format!("/v1/evidence/evd_{id}")', 'Table::Cache', 'bytes,')
    codec = function(HUMAN / "crates/layerx-human-service/src/activity/export.rs", "encode")
    require(codec, 'out.push(self.version)', 'out.extend_from_slice(&self.principal_binding)',
            'push_bytes(&mut out, &receipt.canonical_receipt_bytes)?', 'out.push(rung_byte(receipt.rung))')


def test_current_goldens_are_tied_to_real_rust_producers():
    test = function(SERVER / "production_reads.rs", "golden_current_activity_projections")
    require(test, 'Feed::record_agent_event(', 'Feed::entry(', 'verify(&evidence.canonical_receipt, &evidence.authorised_batch)',
            'ReceiptActual::from_verified(&verified)', 'EvidenceBundle::verify_receipt(&fact, &keys,)',
            'activity_detail_json(', 'assert_eq!(value, expected_entry)', 'EvidenceExport::new(',
            '.evidence(', 'bundle.encode()', 'golden/current/activity.evidence.bundle',
            'EvidenceBundle::decode(&bytes)', 'decoded.verify(digest, &principal, &keys)',
            'cache_activity_export(', 'assert_eq!(export, expected_export)', 'schema.encode_response(',
            'serde_json::from_slice::<Value>(&serialized)', '"paxeer-finality", "paxeer-finalised"')
    for name in CURRENT:
        golden(name)
        encoded = (GOLDEN / name).read_text()
        for retired in ("paxeer-finality", "paxeer-finalised", "settlement-anchored"):
            assert retired not in encoded


@pytest.mark.parametrize("name", HISTORICAL_HASHES)
def test_historical_decode_payloads_preserve_old_bytes(name):
    payload = (GOLDEN / "historical-decode" / name).read_bytes()
    assert sha256(payload).hexdigest() == HISTORICAL_HASHES[name]
    historical = json.loads(payload)
    assert historical["status"] == 200 and historical["body"]["ok"] is True
    assert "paxeer-finality" in payload.decode() and "paxeer-finalised" in payload.decode()
    schema = HUMAN / "schema/human-api/journeys.kvx"
    for type_name, value in (("EvidenceClass", "paxeer-finality"), ("VerificationLevel", "paxeer-finalised")):
        contract = section(schema, f"type.{type_name}")
        assert value in contract["variants"] and value in contract["retired_variants"]
    assert json.loads(json.dumps(historical)) == historical


def test_record_maps_every_finding_and_states_evidence_limits():
    record = tomllib.loads(RECORD.read_text())
    tests = {node.name for node in ast.parse(source(Path(__file__))).body if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    findings = record["finding"]
    expected = {
        "activity.entry-retired-evidence", "activity.export.evidence-retired-evidence",
        "evidence-placeholder", *(f"{operation}-error-shape" for operation in BINDINGS),
    }
    # Dots in finding IDs are quoted in the KVX so the identity remains exact.
    assert set(findings) == expected
    audit = ROOT / "spec/layerx-merge/programme/audit/records/22.30.102.kvx"
    for finding_id, finding in findings.items():
        original = section(audit, f"finding.{finding_id}")
        assert finding["file"] == original["file"]
        assert finding["tests"] and set(finding["tests"]) <= tests
        assert finding["audit"] == "22.30.102"
    task = record["repair"]["22"]["33"]["29"]
    assert task["base_revision"] == "2a8418f3ce46275830d68ea57514c30466a3493f"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "No baseline failures are claimed" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert "same candidate revision" in task["evidence"]
    assert "tools/schema-check/tests/retired_binding_goldens.rs" in record["wiring"]["schema_check"]["request"]
    assert record["wiring"]["schema_check"]["owner_domain"] == 10
    assert "receipt_evidence_v2.rs" in record["wiring"]["http_evidence"]["request"]
