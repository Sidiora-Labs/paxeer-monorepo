"""Offline source contracts for audit 22.30.71/36; no native execution is claimed."""

import ast
import hashlib
from pathlib import Path
import re
import tomllib


ROOT = Path(__file__).resolve().parents[3]
WEBHOOKS = ROOT / "services/platform/hosted/webhooks"
RECORD = ROOT / "tools/qualification/audit_repairs/repair_22_33_1331.kvx"
PACKAGE = "layerx-platform-webhooks"
LOCK_BLOCK = re.compile(
    r'(?ms)^\[\[package\]\]\nname = "layerx-platform-webhooks"\n'
    r'.*?(?=^\[\[package\]\]|\Z)'
)


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def compact(text):
    return re.sub(r"\s+", " ", text)


def between(text, start, end):
    begin = text.index(start)
    return text[begin:text.index(end, begin + len(start))]


def test_finding_36_manifest_removes_retired_codec_and_resolves_proof_contract():
    manifest = tomllib.loads((WEBHOOKS / "Cargo.toml").read_text())
    workspace_path = WEBHOOKS.parents[1] / "Cargo.toml"
    workspace = tomllib.loads(workspace_path.read_text())
    assert manifest["package"]["name"] == PACKAGE
    assert "hosted/webhooks" in workspace["workspace"]["members"]
    for table in ("dependencies", "dev-dependencies", "build-dependencies"):
        assert "layerx-wire" not in manifest.get(table, {})
    for target in manifest.get("target", {}).values():
        for table in ("dependencies", "dev-dependencies", "build-dependencies"):
            assert "layerx-wire" not in target.get(table, {})
    assert manifest["dependencies"]["layerx-proof"] == {"workspace": True}
    proof = workspace["workspace"]["dependencies"]["layerx-proof"]
    proof_path = (workspace_path.parent / proof["path"]).resolve()
    assert tomllib.loads((proof_path / "Cargo.toml").read_text())["package"]["name"] == "layerx-proof"
    inclusion = (proof_path / "src/inclusion.rs").read_text()
    assert "pub fn verify_header(" in inclusion
    assert "reproduced != header_bytes" in inclusion
    assert "ed25519::verify_digest(&authorization.public_key, signature, &digest)" in inclusion
    assert manifest["dependencies"]["layerx-platform-gateway"] == {"path": "../gateway"}
    assert (WEBHOOKS / manifest["lib"]["path"]).is_file()
    assert all((WEBHOOKS / binary["path"]).is_file() for binary in manifest["bin"])


def test_finding_36_every_affected_lock_matches_manifest_without_resolution_churn():
    manifest = tomllib.loads((WEBHOOKS / "Cargo.toml").read_text())
    expected = set(manifest["dependencies"]) | set(manifest["dev-dependencies"])
    affected = []
    for path in sorted(ROOT.rglob("Cargo.lock")):
        text = path.read_text(encoding="utf-8")
        if f'name = "{PACKAGE}"' not in text:
            continue
        lock = tomllib.loads(text)
        packages = [item for item in lock["package"] if item["name"] == PACKAGE]
        assert len(packages) == 1, path
        dependencies = packages[0]["dependencies"]
        assert {value.split()[0] for value in dependencies} == expected, path
        assert len(dependencies) == len(expected), path
        assert "layerx-wire" not in dependencies
        assert "layerx-proof" in dependencies
        assert len([item for item in lock["package"] if item["name"] == "layerx-proof"]) == 1
        match = LOCK_BLOCK.search(text)
        assert match is not None
        remainder = text[:match.start()] + text[match.end():]
        assert hashlib.sha256(remainder.encode()).hexdigest() == (
            "cf33982a73d396089041e0baf39c172257ea758d1663e3dc0572c962cd280288"
        )
        affected.append(path.relative_to(ROOT).as_posix())
    assert affected == ["services/platform/Cargo.lock"]


def test_finding_36_versions_match_live_node_producer_and_rpc_consumer():
    chain = (WEBHOOKS / "src/chain.rs").read_text()
    producer = source("node/sequencer/receipt.go")
    rpc = source("rpc/sequencer_receipt.go")
    constants = dict(re.findall(r"pub const (NODE_RECEIPT_\w+): u(?:16|32) = (\d+);", chain))
    assert constants == {
        "NODE_RECEIPT_PROTOCOL_VERSION": "2",
        "NODE_RECEIPT_LAYOUT_VERSION": "1",
        "NODE_RECEIPT_PARAMETER_VERSION": "0",
    }
    for node_name, rpc_name, name in (
        ("receiptProtocolVersion", "sequencerReceiptProtocolVersion", "NODE_RECEIPT_PROTOCOL_VERSION"),
        ("ReceiptLayoutVersion", "sequencerReceiptLayoutVersion", "NODE_RECEIPT_LAYOUT_VERSION"),
    ):
        for text, symbol in ((producer, node_name), (rpc, rpc_name)):
            match = re.search(rf"\b{symbol}\s+uint(?:16|32)\s*=\s*(\d+)", text)
            assert match is not None
            assert match[1] == constants[name]
    assert "b = binary.BigEndian.AppendUint16(b, receiptProtocolVersion)" in producer
    assert "b = binary.BigEndian.AppendUint32(b, ReceiptLayoutVersion)" in producer
    assert "b = binary.BigEndian.AppendUint32(b, 0)" in producer
    assert "decoded.ParameterVersion != 0" in rpc
    for method, name in (
        ("protocol_version", "NODE_RECEIPT_PROTOCOL_VERSION"),
        ("module_version", "NODE_RECEIPT_LAYOUT_VERSION"),
        ("parameter_version", "NODE_RECEIPT_PARAMETER_VERSION"),
    ):
        assert f"protocol.{method}() != {name}" in chain
    assert "STATE_COMMITMENT_PROTOCOL_VERSION" not in chain


def test_finding_36_live_transport_height_is_authenticated_before_delivery():
    trusted = (WEBHOOKS / "src/trusted.rs").read_text()
    live = trusted[trusted.index("impl ReceiptVerifier {"):]
    fetch = compact(between(live, "pub fn verify(&self,", "pub fn verify_evidence("))
    assert '"pax_sequencerReceipt"' in fetch
    assert 'receipt_json["rung"] != "committed"' in fetch
    assert 'hex_decode( receipt_json["receipt"]' in fetch
    assert 'u64::from_str_radix( receipt_json["height"]' in fetch
    assert '.strip_prefix("0x")' in fetch
    assert "if height <= 1" in fetch
    assert '"commit", serde_json::json!({"height":height.to_string()})' in fetch
    assert "if commit.height()? != height" in fetch
    assert '"block", serde_json::json!({"height":height.to_string()})' in fetch
    assert '"eth_getBlockByNumber"' in fetch
    assert '"/store/sequencer/key"' in fetch
    assert '"height":(height-1).to_string(), "prove":true' in fetch
    assert "!= Some(height - 1)" in fetch
    assert "crate::chain::Membership::from_abci(response)?" in fetch
    assert "self.verify_evidence(&document, expected, fixed_hex::<32>(served_hash)?)" in fetch
    verified = compact(live[live.index("pub fn verify_evidence("):])
    facts = (
        "document.verify(&self.policy, activity)?",
        ".verify(&self.policy, &document.governed_keys.proof)?",
        "if served_hash != committed.block_hash",
        "verification: Verification::BlockCommitted",
    )
    assert [verified.index(fact) for fact in facts] == sorted(verified.index(fact) for fact in facts)
    assert 'json:"height"' in source("rpc/sequencer_receipt.go")
    assert "verifier: ReceiptVerifier" in trusted
    assert "self.verifier.verify(activity)" in trusted
    assert "let verification = operation.verification;" in trusted
    assert "layerx_proof::receipt::verify_sequencer_signature" not in live
    assert "HistoricalKernel" not in live


def test_finding_36_governed_receipt_and_batch_checks_keep_exact_facts():
    chain = compact((WEBHOOKS / "src/chain.rs").read_text())
    evidence = chain[chain.index("impl NodeReceiptEvidence {"):]
    for fact in (
        "self.commit.verify(policy, &self.governed_keys.proof)?",
        '.verify(committed.app_hash, b"sequencer", &[3], keys_value)?',
        "let decoded = keys.verify_receipt(&self.receipt)?;",
        "protocol.activity_id() != activity",
        "receipt_height != committed.height",
        "one_bytes(&self.commit.header_fields[6], 1)? != data",
        "sequence: protocol.global_sequence()",
        "previous_state_root: protocol.previous_state_root()",
        "resulting_state_root: protocol.resulting_state_root()",
        "receipt: &SequenceReceipt",
        "let sequence = receipt.sequence;",
        "previous != receipt.previous_state_root",
        "next != receipt.resulting_state_root",
        "self.activity_ids[(sequence - from) as usize] != activity",
        "keys.authorization_for_header(id, height)?",
        "policy.chain_id.as_bytes()",
        ".verify_strict(&digest(&preimage), &Signature::from_bytes(&signature))",
        "layerx_platform_gateway::verify_live_receipt(",
        "Some(activity)",
    ):
        assert fact in evidence, fact
    assert evidence.index("keys.verify_receipt") < evidence.index("&SequenceReceipt {")
    assert evidence.index("self.verify_batch(") < evidence.index("layerx_platform_gateway::verify_live_receipt(")
    height = compact(between((WEBHOOKS / "src/chain.rs").read_text(), "pub(crate) fn height(", "pub fn verify(&self, policy:"))
    assert ".header_fields .get(2) .ok_or(WebhookError::VerificationRequired)?" in height
    assert "one_number(field, 1)?" in height
    assert "height == 0 || height > i64::MAX as u64" in height
    gateway = source("services/platform/hosted/gateway/src/lib.rs")
    assert "pub fn verify_receipt(" in gateway
    assert "ReceiptKeys::HeightRanges(self.0.as_slice())" in gateway


def test_finding_36_historical_proofs_preserve_refusals_without_direct_wire_imports():
    paths = sorted(WEBHOOKS.rglob("*.rs"))
    assert paths
    for path in paths:
        text = path.read_text()
        assert not re.search(r"\blayerx_wire\b", text), path
    trusted = (WEBHOOKS / "src/trusted.rs").read_text()
    historical = compact(between(trusted, "impl AuthorityReceiptVerifier {", "fn read_secret("))
    for fact in (
        "layerx_proof::inclusion::verify_header( &bytes, &fixed_hex::<64>(signature)?, &self.sequencer_authorization",
        "header.header().network_id() != self.protocol_network_id",
        "authority.protocol_network_id != self.protocol_network_id",
        "authority.network_id != self.network_id",
        "hex_decode(&authority.receipt)? != hex_decode(&receipt.receipt)?",
        "verify_activity_operation(",
    ):
        assert fact in historical, fact
    for path in (
        "tests/repair_22_31_188.rs",
        "tests/repair_22_31_340.rs",
        "src/repair_22_33_1328_authority.rs",
    ):
        assert "layerx_proof::inclusion::verify_header(" in (WEBHOOKS / path).read_text()
    preserved = (WEBHOOKS / "tests/repair_22_31_340.rs").read_text()
    assert "layerx_proof::receipt::verify_sequencer_signature(" in preserved
    assert "assert_eq!(facts.resulting_state_root, header.resulting_state_root());" in preserved
    assert "foreign network {foreign} must not establish receipt authority" in preserved
    assert 'let foreign_networks = ["testnet", "foreign-network", "layerx-testnet-other"]' in preserved
    support = (WEBHOOKS / "src/repair_22_33_1328_authority.rs").read_text()
    for refusal in ("null evidence", "unknown field", "foreign protocol network", "receipt mismatch"):
        assert f'"{refusal}"' in support
    assert 'field("sequencer_id_hex")' in support
    assert "layerx_proof::receipt::verify_sequencer_signature(" in support


def test_finding_36_record_names_all_proofs_and_external_wiring():
    record = tomllib.loads(RECORD.read_text())
    repair = record["repair"]
    finding = record["finding"]["36"]
    assert repair["task"] == "22.33.1331"
    assert repair["requirement"] == "22998.1"
    assert repair["input_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert repair["build_cmd"] == "python3 --version"
    assert repair["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1331.py"
    assert finding["audit"] == "22.30.71"
    assert finding["path"] == "services/platform/hosted/webhooks/Cargo.toml"
    assert finding["class"] == "retired-reference"
    assert finding["severity"] == "medium"
    declared = {
        item.name for item in ast.parse(Path(__file__).read_text()).body
        if isinstance(item, ast.FunctionDef) and item.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared - {"test_finding_36_record_names_all_proofs_and_external_wiring"}
    assert len(finding["tests"]) == 6
    assert repair["affected_locks"] == ["services/platform/Cargo.lock"]
    assert "No pre-change verification was run" in repair["baseline"]
    assert "not executed" in repair["review_only"]
    assert "same candidate revision" in repair["evidence"]
    requests = record["wiring"]
    assert requests["proof"]["owning_domain"] == 9
    assert requests["gateway"]["owning_domain"] == 8
    assert requests["proof"]["path"] == "services/agent/crates/layerx-proof"
    assert requests["gateway"]["path"] == "services/platform/hosted/gateway"
