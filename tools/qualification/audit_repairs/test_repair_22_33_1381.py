"""Offline source qualification for merchant-node-url; no JavaScript execution.

The real application, transport, receipt producer and verifier are inspected in
place. Runtime receipt-read qualification belongs to the shared Node test owner
named in the repair record, outside this task's Python-only gate.
"""

import ast
import json
from pathlib import Path
import re
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
APP = "services/platform/examples/merchant-checkout"
SUPPORT = "services/platform/examples/support"
RECORD = Path(__file__).with_name("repair_22_33_1381.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def document():
    return json.loads(source(f"{APP}/layerx.example.json"))


def between(text, start, end):
    assert text.count(start) == 1, start
    body = text.split(start, 1)[1]
    assert end in body, end
    return body.split(end, 1)[0]


def test_merchant_node_url_beta_uses_public_node_root():
    config = document()
    assert config["version"] == 1
    assert config["application"] == "merchant-checkout"
    assert set(config["environments"]) == {"beta", "emulator"}
    beta = config["environments"]["beta"]
    assert beta["rpcUrl"] == "https://api-mainnet-beta.paxeer.network"
    url = urlsplit(beta["rpcUrl"])
    assert url.scheme == "https"
    assert url.hostname == "api-mainnet-beta.paxeer.network"
    assert url.path == "" and not url.query and not url.fragment
    assert url.username is None and url.password is None and url.port is None
    assert beta["network"] == "eip155:125"
    assert beta["receiptKeysEnvironment"] == "LAYERX_TESTNET_RECEIPT_KEYS_JSON"
    assert beta["settlementUrl"] != beta["rpcUrl"]
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_merchant_node_url_emulator_adapter_is_explicit():
    emulator = document()["environments"]["emulator"]
    assert emulator["rpcUrl"] == "http://127.0.0.1:9402/rpc"
    assert emulator["network"] == "eip155:125"
    assert emulator["receiptKeysEnvironment"] == "LAYERX_EMULATOR_RECEIPT_KEYS_JSON"
    readme = source(f"{APP}/README.md")
    for fact in (
        document()["environments"]["beta"]["rpcUrl"], emulator["rpcUrl"],
        "Do not append `/rpc`", "separate local emulator adapter",
        "legacy adapter coordinate", "not a\npublic node route",
        "eth_getRawTransactionByHash", "pax_sequencerReceipt",
        "independently pinned receipt keys", "submitted receipt remains pending",
        "deployment review",
    ):
        assert fact in readme, fact


def test_merchant_node_url_profile_reaches_receipt_transport_unchanged():
    package = json.loads(source(f"{APP}/package.json"))
    assert package["type"] == "module"
    assert package["scripts"]["start:beta"] == "node index.mjs --environment beta"
    assert package["scripts"]["start:emulator"] == "node index.mjs --environment emulator"
    assert set(package["dependencies"]) == {
        "@sidiora/layerx-merchant-middleware", "@sidiora/layerx-seller-middleware",
    }
    assert source(f"{APP}/index.mjs") == (
        'import { runMerchantApplication } from "../support/merchant-app.mjs";\n\n'
        'await runMerchantApplication(import.meta.url, "merchant-checkout");\n'
    )
    runtime = source(f"{SUPPORT}/runtime.mjs")
    loader = between(runtime, "export async function loadApplicationConfig(",
                     "export function applicationStatePath(")
    assert '"layerx.example.json"' in loader
    assert "exactObject(environments[selected.environment])" in loader
    assert "...config," in loader
    override = between(runtime, "export function applyEndpointOverride(",
                       "export function receiptKeysFromEnvironment(")
    assert 'if (override === undefined || environment !== "beta") return config;' in override
    assert 'const path = secureUrl(current).pathname.replace(/^\//u, "");' in override
    assert "rebased[field] = new URL(path, base).toString();" in override
    merchant = source(f"{SUPPORT}/merchant-app.mjs")
    assert "const config = await loadApplicationConfig(moduleUrl, application);" in merchant
    assert "return createMerchantApplicationServer(config).listen(" in merchant
    assert "new TransactionReceipts(config.rpcUrl, receiptKeys, config.transportLimits)" in merchant
    assert "this.client = new SequencerReceiptClient(rpcUrl, receiptKeys, transportLimits);" in merchant
    assert "this.rpcUrl = secureUrl(rpcUrl).toString();" in runtime
    transport = between(runtime, "export async function boundedJsonRequest(",
                        "export const requiredEnvironment")
    assert 'fetch(secureUrl(url), { ...request, redirect: "error", signal: controller.signal })' in transport
    assert "new AbortController()" in transport and "response_too_large" in transport
    assert 'JSON.parse(new TextDecoder("utf-8", { fatal: true })' in transport


def test_merchant_node_url_real_receipt_source_path_checks_evidence():
    merchant = source(f"{SUPPORT}/merchant-app.mjs")
    transactions = between(merchant, "class TransactionReceipts {", "export class MerchantCheckout")
    assert "boundedJsonRequest(this.client.rpcUrl," in transactions
    assert 'body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "eth_getRawTransactionByHash", params: [transaction] })' in transactions
    assert "if (transactionHash(raw) !== transaction)" in transactions
    assert "transaction_hash_mismatch" in transactions
    assert "return this.client.resolve(raw);" in transactions
    settlement = between(merchant, "class SettlementAuthority {", "export class FileFulfillments")
    assert "...await this.receipts.resolve(result.transaction)" in settlement
    client = between(source(f"{SUPPORT}/runtime.mjs"),
                     "export class SequencerReceiptClient {", "function receiptKeyHeight(")
    assert "boundedJsonRequest(this.rpcUrl," in client
    assert 'body: JSON.stringify({ jsonrpc: "2.0", id: 1, method: "pax_sequencerReceipt", params: [txHash] })' in client
    assert "const txHash = receiptHex(keccak256(raw));" in client
    assert "result.txHash !== txHash" in client
    assert 'if (result.rung === "submitted") throw new LayerXApplicationStateError("pending", "sequencer_receipt_submitted");' in client
    assert 'if (result.rung !== "receipted" && result.rung !== "committed")' in client
    assert "await verifySequencerReceipt(signedReceipt, raw, [key]);" in client
    assert client.index("await verifySequencerReceipt(") < client.index('format: "layerx-receipt-proof-v2"')
    assert "sequencer_receipt_verification_failed" in client
    producer = source("rpc/sequencer_receipt.go")
    assert 'const SequencerReceiptMethod = "pax_sequencerReceipt"' in producer
    assert set(re.findall(r'Rung\w+ Rung = "(\w+)"', producer)) == {
        "submitted", "receipted", "committed",
    }


def test_merchant_node_url_record_maps_proof_limits_and_runtime_wiring():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["merchant-node-url"]
    original = tomllib.loads(source(
        "spec/layerx-merge/programme/audit/records/22.30.75.kvx"
    ))["finding"]["merchant-node-url"]
    for key, value in original.items():
        assert finding[key] == value, key
    assert finding["audit"] == "22.30.75"
    tests = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == tests
    task = record["task"]["22"]["33"]["1381"]
    assert task["base_revision"] == "b664b08b724ec5b190920b99ba70b283c8a796c9"
    assert "already" in task["initial_state"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1381.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "not executed" in task["review_only"]
    wiring = record["wiring"]["beta_receipt_read"]
    assert wiring["path"] == "services/platform/examples/test/merchant-app.test.mjs"
    assert wiring["owner_domain"] == 8
    assert wiring["status"] == "requested; not executed by this gate"
    assert "configured beta" in wiring["request"]
