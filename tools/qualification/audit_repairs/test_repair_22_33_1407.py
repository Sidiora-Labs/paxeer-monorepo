"""Offline source and producer-transcript proof for marketplace findings 021/024."""

import ast
import hashlib
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit

import pytest


ROOT = Path(__file__).resolve().parents[3]
MARKETPLACE = ROOT / "services/platform/examples/marketplace"
RECORD = Path(__file__).with_name("repair_22_33_1407.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


@pytest.mark.parametrize(
    ("environment", "expected_url"),
    [
        ("emulator", "http://127.0.0.1:8545"),
        ("beta", "https://api-mainnet-beta.paxeer.network"),
    ],
)
def test_config_targets_root_chain_rpc(environment, expected_url):
    config = json.loads((MARKETPLACE / "layerx.example.json").read_text())
    assert config["version"] == 1
    assert config["application"] == "marketplace"
    profile = config["environments"][environment]
    assert profile["rpcUrl"] == profile["endpoint"] == expected_url
    assert profile["network"] == "eip155:125"
    url = urlsplit(profile["rpcUrl"])
    assert url.path in ("", "/")
    assert not url.query and not url.fragment and not url.username
    index = (MARKETPLACE / "index.mjs").read_text()
    assert 'loadApplicationConfig(import.meta.url, "marketplace")' in index
    assert "new SequencerReceiptClient(config.rpcUrl," in index
    assert "endpoint: config.rpcUrl," in index
    assert "return verifyProgramOutcome(output, receipts);" in index


def test_local_profile_matches_merged_node_instead_of_inspection_emulator():
    profile = json.loads((MARKETPLACE / "layerx.example.json").read_text())[
        "environments"
    ]["emulator"]
    node_config = source("rpc/config/config.go")
    port = re.search(r"HTTPPort:\s+(\d+),", node_config)
    assert port is not None
    assert urlsplit(profile["rpcUrl"]).port == int(port.group(1)) == 8545
    emulator = source("tools/platform/emulator/src/main.rs")
    assert "const DEFAULT_PORT: u16 = 9402;" in emulator
    routes = emulator.split("fn route(", 1)[1].split("fn parse_amount(", 1)[0]
    assert '("GET", "/healthz") => health(emulator, trace)' in routes
    assert '("POST", "/")' not in routes
    assert '_ => refusal(trace, 404, "not_found", "route does not exist")' in routes
    assert urlsplit(profile["rpcUrl"]).port != 9402


def test_package_runs_receipt_regressions_after_existing_checks():
    manifest = json.loads((MARKETPLACE / "package.json").read_text())
    assert manifest["type"] == "module"
    commands = [shlex.split(part) for part in manifest["scripts"]["test"].split("&&")]
    assert commands == [
        ["node", "--check", "index.mjs"],
        ["node", "test/deploy-payload.test.mjs"],
        ["node", "test/receipt-outcome.test.mjs"],
    ]
    for command in commands:
        assert (MARKETPLACE / command[-1]).is_file()
    assert manifest["dependencies"] == {"@sidiora/layerx-sdk": "0.1.0"}


def test_receipt_regressions_replay_producer_shape_over_root_transport():
    test = (MARKETPLACE / "test/receipt-outcome.test.mjs").read_text()
    for fact in (
        '"../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"',
        'assert.equal(vectors.provenance.generator, "rpc/sequencer_receipt_vectors_test.go")',
        'assert.equal(vector.expected.verifier, "accepts")',
        "assert.equal(vector.receiptJson.receipt, vector.signedReceipt)",
        'assert.equal(vector.receiptJson.rung, "receipted")',
        "assert.equal(vector.receiptJson.status, status)",
        'assert.equal(vector.receiptJson.txHash, `0x${Buffer.from(keccak256(bytes(vector.rawTransaction))).toString("hex")}`)',
        "assert.equal(vector.receiptJson.keyId, vector.pinnedKey.keyId)",
        "assert.equal(vector.receiptJson.signingKey, vector.pinnedKey.publicKey)",
        "result: vector.receiptJson,",
        "keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),",
        "first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight),",
        "const server = createServer(async (request, response) => {",
        'assert.equal(request.method, "POST")',
        'assert.equal(request.url, "/")',
        'assert.deepEqual(Object.keys(message).sort(), ["id", "jsonrpc", "method", "params"])',
        "assert.deepEqual(message.params, [service.signed.txHash])",
        'if (message.method === "eth_getRawTransactionByHash") result = service.signed.raw',
        'assert.equal(message.method, "pax_sequencerReceipt")',
        "result = service.signed.result;",
        'JSON.stringify({ jsonrpc: "2.0", id: message.id, result })',
        'server.listen(0, "127.0.0.1")',
        "service.rpcUrl = `http://127.0.0.1:${server.address().port}`;",
        "server.close((error) => error ? reject(error) : resolve())",
        "assert.deepEqual(service.failures, [])",
    ):
        assert fact in test, fact
    assert 'assert.equal(request.url, "/rpc")' not in test
    assert "${server.address().port}/rpc" not in test


@pytest.mark.parametrize(
    ("case_id", "status", "result_code"),
    [("V1-layerxprograms-call", "0x1", 0), ("V2-layerxledger-revert", "0x0", 1)],
)
def test_recorded_receipts_bind_outcome_to_canonical_producer_bytes(
    case_id, status, result_code
):
    vectors = json.loads(source(
        "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    assert vectors["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    cases = [case for case in vectors["cases"] if case["id"] == case_id]
    assert len(cases) == 1
    vector = cases[0]
    result = vector["receiptJson"]
    assert result["receipt"] == vector["signedReceipt"]
    assert result["rung"] == vector["expected"]["rung"] == "receipted"
    assert vector["expected"]["verifier"] == "accepts"
    assert result["status"] == status
    assert result["keyId"] == vector["pinnedKey"]["keyId"]
    assert result["signingKey"] == vector["pinnedKey"]["publicKey"]
    assert vector["pinnedKey"]["firstHeight"] <= int(result["height"], 16)
    assert int(result["height"], 16) <= vector["pinnedKey"]["lastHeight"]
    receipt = bytes.fromhex(vector["signedReceipt"][2:])
    assert receipt[:6] == bytes.fromhex("000252010002")
    assert int.from_bytes(receipt[6:10], "big") == 32
    assert receipt[10:42].hex() == result["txHash"][2:]
    # The producer's fixed receipt header: activity ID, sequence, three roots.
    result_offset = 6 + 4 + 32 + 8 + 3 * (4 + 32)
    assert int.from_bytes(receipt[result_offset:result_offset + 4], "big") == result_code
    assert receipt[-69:-64] == bytes.fromhex("0100000040")
    unsigned = receipt[:-69] + b"\x00"
    digest = hashlib.sha256(b"LXP/v1/receipt\x00" + unsigned).hexdigest()
    assert "0x" + digest == vector["receiptDigest"]
    codec = source("services/agent/sdk/typescript/src/sequencer-receipt-codec.ts")
    assert "integer(BigInt.asUintN(32, BigInt(receipt.resultCode)), 4)" in codec


def test_receipt_tests_keep_success_failure_and_unpinned_key_assertions():
    test = (MARKETPLACE / "test/receipt-outcome.test.mjs").read_text()
    assert re.findall(r"^test\((\w+)\);$", test, re.M) == [
        "programOutcomeCompletesOnSuccessfulReceipt",
        "programOutcomeRefusesReceiptUnderUnpinnedKey",
    ]
    for fact in (
        "async function programOutcomeCompletesOnSuccessfulReceipt(t)",
        "async function programOutcomeRefusesReceiptUnderUnpinnedKey(t)",
        "assert.deepEqual(await verifyProgramOutcome(output, client), {",
        'state: "completed",',
        "receiptDigest: canonicalReceiptDigest(signed),",
        "transactionHash: signed.txHash,",
        "resultCode: 0,",
        'service.signed = signedReceipt("V2-layerxledger-revert", "0x0")',
        "assert.deepEqual(failed, {",
        'state: "refused",',
        "receiptDigest: canonicalReceiptDigest(service.signed),",
        "resultCode: 1,",
        "assertReceiptRequests(service)",
        "{ ...signed.key, keyId: Buffer.alloc(32, 0x66) }",
        'generateKeyPairSync("ed25519")',
        "await assert.rejects(verifyProgramOutcome(output, new SequencerReceiptClient(service.rpcUrl, [key]))",
        "assert(error instanceof LayerXApplicationStateError)",
        'assert.equal(error.state, "refused")',
        'assert.equal(error.message, "sequencer_receipt_verification_failed")',
        'assert.notEqual(digest, Buffer.from(keccak256(signed.receipt)).toString("hex"))',
    ):
        assert fact in test, fact
    assert not re.search(r"\b(?:test|describe)\.(?:skip|todo|only)\b", test)


def test_production_receipt_path_authenticates_before_reporting_outcome():
    outcome = (MARKETPLACE / "receipt-outcome.mjs").read_text()
    runtime = source("services/platform/examples/support/runtime.mjs")
    verifier = source("services/agent/sdk/typescript/src/verifier.ts")
    for fact in (
        "response = await fetch(client.rpcUrl, {",
        'method: "eth_getRawTransactionByHash", params: [txHash]',
        "const proof = await client.resolve(raw);",
        "await verifySequencerReceipt(bytes(proof.signedReceipt), raw, [{",
        "first_height: BigInt(key.firstHeight),",
        "last_height: BigInt(key.lastHeight),",
        "return programReceiptOutcome(output, verified, proof.verificationLevel);",
        "const resultCode = verified.receipt.resultCode;",
        "if (resultCode !== 0 && resultCode !== 1)",
        'state: resultCode === 0 ? "completed" : "refused",',
        'receiptDigest: Buffer.from(verified.receiptDigest).toString("hex"),',
        'transactionHash: `0x${Buffer.from(verified.receipt.activityId).toString("hex")}`,',
    ):
        assert fact in outcome, fact
    assert 'method: "pax_sequencerReceipt", params: [txHash]' in runtime
    assert "await verifySequencerReceipt(signedReceipt, raw, [key]);" in runtime
    assert 'verificationLevel: "receipted"' in runtime
    assert '"sequencer_receipt_verification_failed"' in runtime
    assert "const receiptDigest = await sha256(RECEIPT_DOMAIN, encodeSequencerReceipt(receipt, false));" in verifier
    assert "verifyEd25519(key.publicKey, receipt.sequencerSignature, receiptDigest)" in verifier
    assert 'if (signingKey === undefined) return sequencerReceiptFailure("bad_signature")' in verifier
    assert 'if (!equal(receipt.activityId, transactionHash)) return sequencerReceiptFailure("hash_mismatch")' in verifier


def test_repair_record_names_findings_proofs_and_gate_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    repair = record["repair"]
    assert repair["task"] == "22.33.1407"
    assert repair["audit"] == "22.30.79"
    assert repair["requirement"] == "22998.1"
    assert repair["input_revision"] == "b664b08b724ec5b190920b99ba70b283c8a796c9"
    assert set(record["finding"]) == {"021", "024"}
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    mapped = set()
    for finding_id, filename in [("021", "layerx.example.json"), ("024", "package.json")]:
        finding = record["finding"][finding_id]
        assert finding["path"] == f"services/platform/examples/marketplace/{filename}"
        assert finding["tests"] and set(finding["tests"]) <= declared
        mapped.update(finding["tests"])
    assert mapped == declared - {"test_repair_record_names_findings_proofs_and_gate_limits"}
    assert "No pre-change verification was run" in repair["baseline"]
    assert "not executed" in repair["review_only"]
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["verified_revision"]
    assert repair["build_cmd"] == "python3 --version"
    assert repair["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/"
        "test_repair_22_33_1407.py"
    )
    assert "tools/platform/emulator/src/main.rs" in record["finding"]["021"]["wiring_request"]
    assert "domain 8" in record["finding"]["021"]["wiring_request"]
