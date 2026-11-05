"""Offline source proof for buyer-rpc-path; no JavaScript or live RPC execution."""

import ast
import json
from pathlib import Path
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
BUYER = "services/platform/examples/buyer-agent/"
RECORD = "tools/qualification/audit_repairs/repair_22_33_1399.kvx"


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def profiles():
    document = json.loads(source(BUYER + "layerx.example.json"))
    assert document["version"] == 1
    assert document["application"] == "buyer-agent"
    assert set(document["environments"]) == {"beta", "emulator"}
    return document["environments"]


def test_beta_rpc_is_public_node_root():
    beta = profiles()["beta"]
    assert beta["rpcUrl"] == "https://api-mainnet-beta.paxeer.network"
    rpc = urlsplit(beta["rpcUrl"])
    assert rpc.scheme == "https"
    assert rpc.hostname == "api-mainnet-beta.paxeer.network"
    assert rpc.path == ""
    assert rpc.port is None
    assert rpc.username is None and rpc.password is None
    assert not rpc.query and not rpc.fragment
    human = urlsplit(beta["humanUrl"])
    assert human.scheme == "https"
    assert (human.hostname, human.port) != (rpc.hostname, rpc.port)
    assert beta["humanUrlEnvironment"] == "LAYERX_TESTNET_HUMAN_URL"
    assert beta["network"] == "eip155:125"


def test_emulator_retains_separate_local_rpc_service():
    environments = profiles()
    emulator = environments["emulator"]
    assert emulator["rpcUrl"] == "http://127.0.0.1:9402/rpc"
    assert emulator["humanUrl"] == "http://127.0.0.1:9402"
    rpc = urlsplit(emulator["rpcUrl"])
    assert rpc.path == "/rpc"
    assert rpc.hostname == "127.0.0.1" and rpc.port == 9402
    assert not rpc.query and not rpc.fragment
    assert rpc.username is None and rpc.password is None
    assert emulator["network"] == environments["beta"]["network"] == "eip155:125"


def test_real_loader_transport_and_node_route_preserve_configured_path():
    manifest = json.loads(source(BUYER + "package.json"))
    assert manifest["type"] == "module"
    assert manifest["scripts"]["start:beta"] == "node index.mjs --environment beta"
    assert manifest["scripts"]["start:emulator"] == "node index.mjs --environment emulator"
    entry = source(BUYER + "index.mjs")
    assert 'loadApplicationConfig(import.meta.url, "buyer-agent")' in entry
    assert "const node = secureUrl(config.rpcUrl);" in entry
    assert "human.origin === node.origin" in entry
    assert "baseUrl: humanUrl, bearerToken: token" in entry
    runtime = source("services/platform/examples/support/runtime.mjs")
    loader = runtime.split("export async function loadApplicationConfig(", 1)[1]
    assert '?? "layerx.example.json"' in loader
    assert "exactObject(environments[selected.environment])" in loader
    override = runtime.split("export function applyEndpointOverride(", 1)[1].split(
        "export function receiptKeysFromEnvironment", 1
    )[0]
    assert 'environment !== "beta") return config;' in override
    assert r'secureUrl(current).pathname.replace(/^\//u, "")' in override
    assert "rebased[field] = new URL(path, base).toString();" in override
    receipt = runtime.split("export class SequencerReceiptClient", 1)[1]
    assert "this.rpcUrl = secureUrl(rpcUrl).toString();" in receipt
    assert 'boundedJsonRequest(this.rpcUrl, {\n        method: "POST"' in receipt
    assert 'method: "pax_sequencerReceipt", params: [txHash]' in receipt
    assert 'if path == "" {\n\t\treturn r.URL.Path == "/"' in source("rpc/rpcstack.go")


def test_configuration_checks_keep_beta_root_and_emulator_rpc_distinct():
    checks = source(BUYER + "repair.test.mjs")
    assert "async function buyerHumanEndpointConfiguration()" in checks
    assert "test(buyerHumanEndpointConfiguration);" in checks
    assert 'assert.equal(config.rpcUrl, "https://api-mainnet-beta.paxeer.network");' in checks
    assert 'assert.equal(new URL(config.rpcUrl).pathname, "/");' in checks
    assert 'assert.equal(emulator.rpcUrl, "http://127.0.0.1:9402/rpc");' in checks
    assert 'assert.equal(new URL(emulator.rpcUrl).pathname, "/rpc");' in checks
    assert "assert.notEqual(new URL(emulator.rpcUrl).origin, new URL(config.rpcUrl).origin);" in checks
    # The out-of-scope consumer already expects the root; no wiring edit is needed.
    shared = source("services/platform/examples/test/buyer-agent.test.mjs")
    assert '? "http://127.0.0.1:9402/rpc"\n        : "https://api-mainnet-beta.paxeer.network"' in shared
    assert 'if (call.origin === fixture.rpcUrl) assert.equal(call.path, "/");' in checks


def test_record_maps_finding_to_declared_source_proofs():
    record = tomllib.loads(source(RECORD))
    assert record["repair"]["task"] == "22.33.1399"
    assert record["repair"]["base_revision"] == "b664b08b724ec5b190920b99ba70b283c8a796c9"
    finding = record["finding"]["buyer-rpc-path"]
    assert finding["audit"] == "22.30.78"
    assert finding["file"] == BUYER + "layerx.example.json"
    audit = tomllib.loads(source("spec/layerx-merge/programme/audit/records/22.30.78.kvx"))
    original = audit["finding"]["buyer-rpc-path"]
    assert finding["proposal"] == original["proposal"]
    assert finding["class"] == original["class"] == "config-drift"
    assert finding["severity"] == original["severity"] == "medium"
    tests = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == tests - {"test_record_maps_finding_to_declared_source_proofs"}
    assert record["repair"]["wiring_requests"] == []
    assert "not run" in record["repair"]["baseline"]
    assert "not executed" in record["repair"]["review_only"]
    assert record["evidence"]["ledger"] == "/out/run/gates.jsonl"
    assert record["evidence"]["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1399.py"
    )
