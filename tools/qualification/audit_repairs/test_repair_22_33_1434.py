"""Source contracts for retirement, plus real SDK consumers of node recordings."""

import ast
import json
import re
import sys
import tomllib
from hashlib import sha256
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[3]
EMULATOR = ROOT / "tools/platform/emulator"
RECORD = Path(__file__).with_name("repair_22_33_1434.kvx")
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))

from layerx_sdk.account_derivation import keccak256
from layerx_sdk.evm_transaction import Type2Transaction, decode_signed
from layerx_sdk.verifier import (
    ReceiptKey,
    SequencerReceiptVerificationError,
    verify_sequencer_receipt,
)


def source(path):
    return (EMULATOR / path).read_text(encoding="utf-8")


def function(name):
    return source("src/main.rs").split(f"\nfn {name}(", 1)[1].split("\nfn ", 1)[0]


def compact(value):
    return re.sub(r"\s+", "", value)


def unhex(value):
    assert value.startswith("0x")
    return bytes.fromhex(value[2:])


def recorded_case(case_id):
    document = json.loads(
        (ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text()
    )
    assert document["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    assert document["chainId"] == 125
    return next(case for case in document["cases"] if case["id"] == case_id)


def pinned_key(case):
    key = case["pinnedKey"]
    return ReceiptKey(
        unhex(key["keyId"]), unhex(key["publicKey"]),
        key["firstHeight"], key["lastHeight"],
    )


def test_retired_wire_dependency_and_consumers_are_removed():
    manifest = tomllib.loads(source("Cargo.toml"))
    dependencies = manifest["dependencies"]
    assert "layerx-wire" not in dependencies
    assert "layerx-proof" not in dependencies  # This crate also pulls in the retired codec.
    for path in EMULATOR.rglob("*.toml"):
        parsed = tomllib.loads(path.read_text())
        assert "layerx-wire" not in json.dumps(parsed)
        assert "layerx-proof" not in json.dumps(parsed)
    for path in (EMULATOR / "src").glob("*.rs"):
        code = path.read_text()
        assert not re.search(r"\blayerx_(wire|proof)\b", code), str(path)
        assert "verify_sequencer_signature" not in code
        assert "receipt-verified" not in code
    config = function("parse_config")
    assert "protocol_version: 2," in config
    assert '"2" => 2,' in config and '"3" => 3,' in config


def test_head_and_receipt_evidence_fail_closed_through_dispatch():
    route = compact(function("route"))
    assert compact('("GET", "/v1/protocol/account-state/head") => current_head(trace)') in route
    assert compact('("GET", path) if path.starts_with("/v1/evidence/") => evidence_read(&path[13..], trace)') in route
    assert "matchemulator.receipts.get(id)" in route
    assert "Some(_)=>receipt_read(trace)" in route
    assert compact('None => refusal(trace, 404, "not_found", "receipt is not present in this emulator process",)') in route
    for name, status, code in (
        ("current_head", 503, "principal_state_proof_unavailable"),
        ("receipt_read", 410, "legacy_receipt_verification_retired"),
        ("evidence_read", 410, "legacy_receipt_verification_retired"),
    ):
        body = function(name)
        assert compact(f'refusal(trace, {status}, "{code}",') in compact(body)
        assert "success(" not in body
        assert "unsafe" not in body
    evidence = compact(function("evidence_read"))
    assert '.strip_prefix("evd_")' in evidence
    assert ".filter(|hex|hex.len()==64)" in evidence
    assert ".and_then(|hex|hex_decode(hex).ok()).is_none()" in evidence
    assert '400,"invalid_evidence_id"' in evidence
    assert '(_,"/v1/settle")=>refusal(trace,410,settlement::RETIRED_CODE' in route
    assert 'pub const RETIRED_CODE: &str = "settlement_authority_retired";' in source("src/settlement.rs")


def test_discovery_declares_retirement_and_merged_replacement():
    text = source("src/surface.rs")
    expression = re.search(r"serde_json::json!\((\{.*\})\)", text, re.S).group(1)
    constants = dict(re.findall(r'pub const (\w+): &str = ("[^"\n]*");', source("src/settlement.rs")))
    for name, value in constants.items():
        expression = expression.replace(f"super::settlement::{name}", value)
    document = json.loads(expression)
    history = document["historical_imports"]
    assert history["mode"] == "inspection_only"
    assert history["legacy_receipts"] == "snapshot_preservation_only"
    assert history["legacy_receipt_verification"] == "retired"
    assert history["legacy_receipt_verification_http_status"] == 410
    assert history["legacy_head_evidence"] == "unavailable"
    assert history["legacy_head_evidence_http_status"] == 503
    assert history["produces_new_payments"] is False
    assert history["establishes_merged_finality"] is False
    payment = document["payments"]
    assert payment["supported"] is False
    assert payment["settlement_http_status"] == 410
    assert payment["replacement"]["submit"] == "eth_sendRawTransaction"
    assert payment["replacement"]["receipt"] == "pax_sequencerReceipt"
    assert payment["replacement"]["authority"] == "ReceiptPayloadAuthority"
    assert payment["replacement"]["committed_requires"] == "independently verified block-commit evidence"


def test_state_snapshot_host_linkage_and_receipt_preservation_remain():
    manifest = tomllib.loads(source("Cargo.toml"))
    assert manifest["package"]["build"] == "build.rs"
    assert manifest["lib"]["path"] == "src/main.rs"
    for name in ("runtime", "sandbox"):
        dependency = manifest["dependencies"][f"layerx-programs-{name}"]
        assert dependency["features"] == ["host-ffi"]
        assert dependency["path"] == f"../../../programs/crates/layerx-programs-{name}"
        assert (EMULATOR / dependency["path"] / "Cargo.toml").is_file()
        assert f"layerx_programs_{name}::retain_host_ffi_exports();" in function("platform_emulator")
    build = source("build.rs")
    for library in ("static=layerx_emulator_core", "static=layerx", "crypto", "sqlite3", "pthread"):
        assert f'println!("cargo:rustc-link-lib={library}");' in build
    assert '.arg("build/liblayerx.a")' in build
    assert 'manifest.join("core/emulator_core.c")' in build
    route = compact(function("route"))
    assert compact('("GET", "/v1/state") => inspect(emulator, trace)') in route
    assert compact('("GET", "/__emulator/snapshot") => export_snapshot(emulator, trace)') in route
    assert compact('("PUT", "/__emulator/snapshot") => import_snapshot(emulator, &request.body, trace)') in route
    for operation, consumer in (("inspect", "inspect_state"), ("snapshot_export", "export_snapshot"), ("snapshot_import", "import_snapshot")):
        symbol = f"platform_emulator_{operation}"
        assert symbol in function(consumer)
        assert symbol in source("core/emulator_core.h")
        assert symbol in source("core/emulator_core.c")
    assert "encode_snapshot_receipts(&mut encoded, receipts, receipt_order)?" in source("src/main.rs")
    assert "decode_snapshot_receipts(authenticated, &mut offset, receipt_count)?" in source("src/main.rs")
    assert "emulator.receipts = recovered.receipts;" in function("import_snapshot")


@pytest.mark.parametrize("case_id", [
    "V1-pax-transfer", "V1-erc20-approve", "V1-layerxledger-pay-exact",
    "V1-layerxbridge-bridge-out", "V1-layerxprograms-call", "V2-layerxledger-revert",
])
def test_merged_transaction_and_portable_receipt_encoding(case_id):
    case = recorded_case(case_id)
    raw = unhex(case["rawTransaction"])
    transaction, signature = decode_signed(raw)
    assert isinstance(transaction, Type2Transaction)
    assert transaction.chain_id == 125
    assert Type2Transaction.decode(transaction.encode()) == transaction
    view = case["receiptJson"]
    assert transaction.nonce == int(view["nonce"], 16)
    assert transaction.to == unhex(view["to"])
    assert transaction.recover_signer(signature).lower() == view["from"].lower()
    assert transaction.attach_signature(signature, view["from"]) == raw
    assert keccak256(raw) == unhex(view["txHash"])
    # The recorded JSON is the real RPC transport of the producer's signed bytes.
    transported = json.loads(json.dumps(view))
    signed = unhex(transported["receipt"])
    assert signed == unhex(case["signedReceipt"])
    verified = verify_sequencer_receipt(signed, raw, [pinned_key(case)])
    assert verified.canonical_bytes == signed
    assert verified.receipt_digest == unhex(case["receiptDigest"])
    assert verified.level == "sequencer-signed"
    assert verified.height == int(view["height"], 16)
    receipt = verified.receipt
    assert receipt.protocol_version == 2
    assert receipt.module_id == 12 and receipt.module_version == 1
    assert receipt.activity_id == keccak256(raw)
    assert receipt.global_sequence == int(view["seq"], 16)
    assert receipt.timestamp == int(view["timestamp"], 16)
    assert receipt.fee_charged == int(view["feeAmount"], 16)
    assert receipt.result_code == (0 if case["vector"] == "V1" else 1)
    assert receipt.previous_state_root == unhex(case["prevSeqHash"])
    assert receipt.resulting_state_root == sha256(
        b"PAXEER/v1/sequence\0" + receipt.previous_state_root
        + receipt.global_sequence.to_bytes(8, "big") + receipt.activity_id
    ).digest()
    assert receipt.activity_root == sha256(unhex(view["returnData"])).digest()


@pytest.mark.parametrize(("case_id", "code"), [
    ("V3-key-not-pinned", "unpinned_key"),
    ("V3-height-outside-key-range", "key_out_of_range"),
    ("V3-activity-id-not-raw-hash", "hash_mismatch"),
    ("V3-non-canonical-bytes", "bad_signature"),
])
def test_merged_receipt_refuses_invalid_recorded_evidence(case_id, code):
    case = recorded_case(case_id)
    with pytest.raises(SequencerReceiptVerificationError) as refused:
        verify_sequencer_receipt(
            unhex(case["signedReceipt"]), unhex(case["rawTransaction"]), [pinned_key(case)]
        )
    assert refused.value.check == code


def test_repair_record_maps_finding_proofs_and_evidence_limits():
    record = tomllib.loads(RECORD.read_text())
    task = record["repair"]["22"]["33"]["1434"]
    finding = record["finding"]["emulator-retired-wire-dependency"]
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {
        "test_repair_record_maps_finding_proofs_and_evidence_limits"
    }
    assert finding["audit"] == "22.30.80"
    assert finding["file"] == "tools/platform/emulator/Cargo.toml"
    assert task["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "No baseline failures are claimed" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert "same candidate revision" in task["evidence"]
    assert record["wiring"]["contracts"]["owner_domain"] == 8
    assert "spec/layerx-merge/interfaces.kvx" in record["wiring"]["contracts"]["request"]
