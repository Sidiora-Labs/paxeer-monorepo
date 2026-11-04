"""Offline documentation proof for audit 22.30.70: examples-authority-doc."""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib
from urllib.parse import urlsplit

import pytest


ROOT = Path(__file__).resolve().parents[3]
EXAMPLES = ROOT / "services/platform/examples"
RECORD = Path(__file__).with_name("repair_22_33_1318.kvx")
APPLICATIONS = (
    ("buyer-agent", "Buyer agent"),
    ("paid-api", "Paid API"),
    ("merchant-shop", "Merchant shop"),
    ("marketplace", "Programs marketplace"),
)


def source(path):
    return (EXAMPLES / path).read_text(encoding="utf-8")


def section(heading):
    marker = f"## {heading}\n"
    guide = source("README.md")
    assert guide.count(marker) == 1
    return guide.split(marker, 1)[1].split("\n## ", 1)[0]


def normalized(text):
    return " ".join(text.split())


@pytest.mark.parametrize("directory,label", APPLICATIONS)
def test_examples_authority_doc_matches_per_application_profiles(directory, label):
    profile = json.loads(source(f"{directory}/layerx.example.json"))["environments"]["beta"]
    guide = section("Endpoints and prerequisites")
    url = urlsplit(profile["rpcUrl"])
    assert url.scheme == "https" and url.path in ("", "/")
    assert not url.query and not url.fragment and not url.username
    assert profile["network"] == "eip155:125"
    keys = profile["receiptKeysEnvironment"]
    assert keys == (
        "LAYERX_RECEIPT_KEYS_JSON" if directory == "paid-api"
        else "LAYERX_TESTNET_RECEIPT_KEYS_JSON"
    )
    assert f"| {label} | `{profile['rpcUrl']}` | `{keys}` |" in guide
    manifest = json.loads(source("reference-apps.json"))["applications"]
    app = next(app for app in manifest if app["name"] == directory)
    command = shlex.join(app["commands"]["beta"])
    assert f"`{command}`" in guide
    package = json.loads(source(f"{directory}/package.json"))
    assert package["name"] == app["package"]
    expected_arguments = ["node", "index.mjs", "--environment", "beta"]
    if directory == "marketplace":
        expected_arguments += ["--action", "deploy"]
    assert shlex.split(package["scripts"]["start:beta"]) == expected_arguments
    assert "Native node JSON-RPC uses `/`, not `/rpc`" in normalized(guide)
    assert "setting the buyer's variable does not configure the paid API" in normalized(guide)


def test_examples_authority_doc_matches_paid_offer_and_pinned_key_consumer():
    guide = normalized(section("Paid API configuration and verification"))
    paid = source("paid-api/index.mjs")
    runtime = source("support/runtime.mjs")
    profile = json.loads(source("paid-api/layerx.example.json"))["environments"]["beta"]
    for field in (
        "receiptKeysEnvironment", "assetEnvironment", "payToEnvironment",
        "accountEnvironment", "currencyEnvironment", "priceEnvironment",
    ):
        assert f"`{profile[field]}`" in guide
        assert f"config.{field}" in paid
    for fact in (
        "x402 v2", "`scheme: exact`", "`network: eip155:125`",
        "commitment: receipted", "`rawTransaction`", "`pax_sequencerReceipt`",
        "`layerx-receipt-proof-v2`", "independently", "`firstHeight <= lastHeight`",
        "does not resolve a legacy batch-authority object",
        "does not prove BFT finality",
    ):
        assert fact in guide, fact
    for statement in (
        "receiptKeysFromEnvironment(config.receiptKeysEnvironment)",
        "new SequencerReceiptClient(config.rpcUrl, receiptKeys)",
        "x402Version: 2", 'commitment: "receipted"',
    ):
        assert statement in paid
    for statement in (
        "JSON.parse(requiredEnvironment(name))",
        "if (!Array.isArray(entries) || entries.length === 0)",
        "receiptKeyHeight(key.firstHeight)", "receiptKeyHeight(key.lastHeight)",
        'throw new Error("invalid_receipt_key_range")',
        "keyId: hex32(key.keyId)", "publicKey: hex32(key.publicKey)",
        "const txHash = receiptHex(keccak256(raw))",
        "boundedJsonRequest(this.rpcUrl", 'method: "pax_sequencerReceipt", params: [txHash]',
        "await verifySequencerReceipt(signedReceipt, raw, [key])",
        'format: "layerx-receipt-proof-v2"', 'verificationLevel: "receipted"',
    ):
        assert statement in runtime


def test_examples_authority_doc_distinguishes_receipt_states():
    guide = section("Paid API configuration and verification")
    rows = dict(re.findall(r"^\| `(pending|unknown|refused)` \| (.*?) \|$", guide, re.M))
    assert set(rows) == {"pending", "unknown", "refused"}
    assert "matching `submitted` receipt returns 202 with retry advice" in rows["pending"]
    assert "Unreachable RPC, mismatched evidence or an unrecognized rung returns 503" in rows["unknown"]
    assert "RPC refusal or failed receipt verification prevents release" in rows["refused"]
    for state in ("pending", "unknown"):
        assert "no resource is released" in rows[state]
    runtime = source("support/runtime.mjs")
    errors = dict(re.findall(
        r'new LayerXApplicationStateError\("(\w+)", "(\w+)"\)', runtime
    ))
    assert errors["pending"] == "sequencer_receipt_submitted"
    for state, reason in (
        ("unknown", "sequencer_receipt_unreachable"),
        ("unknown", "sequencer_receipt_mismatch"),
        ("unknown", "invalid_sequencer_receipt_rung"),
        ("refused", "sequencer_receipt_refused"),
        ("refused", "sequencer_receipt_verification_failed"),
    ):
        assert f'new LayerXApplicationStateError("{state}", "{reason}")' in runtime
    assert 'if (result.rung === "submitted")' in runtime
    paid = source("paid-api/index.mjs")
    assert 'error.state === "pending" ? 202 : error.state === "refused" ? 402 : 503' in paid
    assert 'error.state === "pending" || error.state === "unknown" ? { "retry-after": "1" }' in paid


def test_examples_authority_doc_separates_beta_qualification_from_history():
    journey = normalized(section("Running the merged-chain journey"))
    history = normalized(section("Historical emulator workflow"))
    qualification = section("Building and checking against real receipts")
    assert "`--scenario beta` runs the applications" in journey
    assert "only actual successful outcomes establish that journey" in journey
    assert "historical emulator workflow, not the merged production path" in history
    assert "Do not infer native compatibility from declaration checks or emulator health" in history
    assert "`LAYERX_EMULATOR_RECEIPT_KEYS_JSON`" in history
    assert "`LAYERX_RECEIPT_KEYS_JSON`" in history
    emulator = json.loads(source("paid-api/layerx.example.json"))["environments"]["emulator"]
    assert emulator["receiptKeysEnvironment"] == "LAYERX_RECEIPT_KEYS_JSON"
    assert "LAYERX_EMULATOR_RECEIPT_KEYS_JSON: emulatorReceiptKeys(emulator.anchor)" in source(
        "run-reference-apps.mjs"
    )
    blocks = re.findall(r"```sh\n(.*?)\n```", qualification, re.S)
    assert len(blocks) == 2
    assert [shlex.split(line) for line in blocks[1].strip().splitlines()] == [
        ["export", "PAXEER_X_REFERENCE_APPS_ENVIRONMENT=beta"],
        ["node", "services/platform/examples/qualify-reference-apps.mjs"],
    ]
    for fact in (
        "each beta profile's `rpcUrl` and `receiptKeysEnvironment`",
        "owner-only external directory", "defaults to the historical emulator workflow",
        "that default does not qualify the merged production path",
        "Declaration checks, emulator health and artifact hashes alone do not prove a successful payment",
    ):
        assert fact in normalized(qualification), fact
    qualifier = source("qualify-reference-apps.mjs")
    assert 'process.env.PAXEER_X_REFERENCE_APPS_ENVIRONMENT ?? "emulator"' in qualifier
    assert '"--scenario", environment' in qualifier
    assert "qualifyReferenceAppOutcomes(outcomesFile, environment)" in qualifier
    assert 'assert.deepEqual(JSON.parse(buyer.body), declaredResource' in qualifier


def test_examples_authority_doc_repair_record_maps_proofs_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["examples-authority-doc"]
    assert finding["audit"] == "22.30.70"
    assert finding["file"] == "services/platform/examples/README.md"
    assert finding["class"] == "doc-drift" and finding["severity"] == "medium"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_examples_authority_doc_")
    }
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["1318"]
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1318.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert task["wiring_requests"] == []
    assert "not executed" in task["proof_limit"]
    assert not (ROOT / "services/platform/tests/repair_22_33_1318.rs").exists()
    examples_record = tomllib.loads(source("repair-22-33-1318.kvx"))["repair"]["22"]["33"]["1318"]
    assert examples_record["proving_tests"] == [
        "tools/qualification/audit_repairs/test_repair_22_33_1318.py"
    ]
    assert examples_record["wiring_requests"] == []
    supplemental = source("test/repair-22-33-1318.test.mjs")
    for name in ("examplesAuthorityDocMatchesApplicationProfiles", "examplesAuthorityDocProvesRootRpcAndReceiptStates"):
        assert f"function {name}(" in supplemental
