"""Offline source proof for merchant-emulator-rpc; no runtime certification."""

import hashlib
import json
from pathlib import Path
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
SHOP = ROOT / "services/platform/examples/merchant-shop"
SUPPORT = SHOP.parent / "support"
RECORD = Path(__file__).with_name("repair_22_33_1450.kvx")


def source(path):
    return path.read_text(encoding="utf-8")


def test_merchant_emulator_root_rpc_and_explicit_settlement():
    document = json.loads(source(SHOP / "layerx.example.json"))
    assert document["version"] == 1
    assert document["application"] == "merchant-shop"
    emulator = document["environments"]["emulator"]
    assert emulator["rpcUrl"] == "http://127.0.0.1:9402"
    rpc = urlsplit(emulator["rpcUrl"])
    settlement = urlsplit(emulator["settlementUrl"])
    assert rpc.path in ("", "/")
    assert not rpc.query and not rpc.fragment and rpc.username is None
    assert emulator["settlementUrl"] == "http://127.0.0.1:9402/v1/settle"
    assert settlement.path == "/v1/settle"
    assert (settlement.scheme, settlement.netloc) == (rpc.scheme, rpc.netloc)
    assert emulator["network"] == "eip155:125"
    assert emulator["receiptKeysEnvironment"] == "LAYERX_EMULATOR_RECEIPT_KEYS_JSON"
    assert emulator["tokenEnvironment"] == "LAYERX_EMULATOR_TOKEN"

    # Fingerprint every base configuration value except the one repaired URL.
    del emulator["rpcUrl"]
    unchanged = json.dumps(document, sort_keys=True, separators=(",", ":"))
    assert hashlib.sha256(unchanged.encode()).hexdigest() == (
        "dbc239427817244359ed7c462327a84a169ba7b31378d4f038d45accbd28e0ae"
    )


def test_production_configuration_rpc_transport_and_settlement_wiring():
    manifest = json.loads(source(SHOP / "package.json"))
    assert manifest["type"] == "module"
    assert manifest["scripts"]["start"] == "node index.mjs --environment emulator"
    assert manifest["scripts"]["start:emulator"] == manifest["scripts"]["start"]
    entrypoint = source(SHOP / "index.mjs")
    assert 'from "./config.mjs"' in entrypoint
    assert 'from "../support/merchant-app.mjs"' in entrypoint
    assert "await loadMerchantShopConfig(import.meta.url)" in entrypoint
    assert "createMerchantApplicationServer(config).listen(" in entrypoint
    configuration = source(SHOP / "config.mjs")
    assert 'if (config.name !== "beta") return config;' in configuration
    assert 'loadApplicationConfig(moduleUrl, "merchant-shop")' in configuration

    runtime = source(SUPPORT / "runtime.mjs")
    loader = runtime.split("export async function loadApplicationConfig(", 1)[1]
    loader = loader.split("export function applicationStatePath(", 1)[0]
    assert '?? "layerx.example.json"' in loader
    assert "exactObject(environments[selected.environment])" in loader
    assert "...config," in loader
    assert 'if (override === undefined || environment !== "beta") return config;' in runtime
    client = runtime.split("export class SequencerReceiptClient", 1)[1]
    assert "this.rpcUrl = secureUrl(rpcUrl).toString();" in client
    assert "boundedJsonRequest(this.rpcUrl, {" in client
    assert 'method: "pax_sequencerReceipt", params: [txHash]' in client
    assert 'fetch(secureUrl(url), { ...request, redirect: "error", signal: controller.signal })' in runtime

    merchant = source(SUPPORT / "merchant-app.mjs")
    assert "new TransactionReceipts(config.rpcUrl, receiptKeys, config.transportLimits)" in merchant
    assert "new SequencerReceiptClient(rpcUrl, receiptKeys, transportLimits)" in merchant
    assert "boundedJsonRequest(this.client.rpcUrl, {" in merchant
    assert 'method: "eth_getRawTransactionByHash", params: [transaction]' in merchant
    assert "return this.client.resolve(raw);" in merchant
    assert "new SettlementAuthority(config.settlementUrl, token, receipts, config.transportLimits)" in merchant
    authority = merchant.split("class SettlementAuthority {", 1)[1]
    authority = authority.split("export class FileFulfillments", 1)[0]
    assert "this.url = secureUrl(url);" in authority
    assert "boundedJsonRequest(this.url.toString(), {" in authority
    assert 'authorization: `Bearer ${this.token}`' in authority
    assert "...await this.receipts.resolve(result.transaction)" in authority


def test_repair_record_maps_proof_limits_and_owning_domain_wiring():
    record = tomllib.loads(source(RECORD))
    task = record["task"]["22"]["33"]["1450"]
    finding = record["finding"]["merchant-emulator-rpc"]
    assert finding["audit"] == "22.30.83"
    assert finding["file"] == "services/platform/examples/merchant-shop/layerx.example.json"
    assert finding["tests"] == [
        "test_merchant_emulator_root_rpc_and_explicit_settlement",
        "test_production_configuration_rpc_transport_and_settlement_wiring",
    ]
    assert task["base_revision"] == "b664b08b724ec5b190920b99ba70b283c8a796c9"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1450.py"
    )
    assert "No baseline run" in task["baseline"]
    assert "source" in task["source_proof"]
    assert "not executed" in task["review_only"]
    wiring = task["wiring_request"]
    for fact in (
        "tools/platform/emulator/src/main.rs",
        "services/platform/examples/support/runtime.mjs",
        "services/platform/examples/support/merchant-app.mjs",
        "domain 8",
        "POST /",
        "pax_sequencerReceipt",
        "/v1/settle",
        "410",
    ):
        assert fact in wiring
