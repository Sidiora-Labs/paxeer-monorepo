"""Offline source proof for the hosted gateway OpenRPC transport description."""

import ast
import json
from pathlib import Path
import re
import tomllib
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
GATEWAY = ROOT / "services/platform/hosted/gateway"
RECORD = Path(__file__).with_name("repair_22_33_634.kvx")


def openrpc():
    return json.loads((GATEWAY / "openrpc.json").read_text(encoding="utf-8"))


def source(name):
    return (GATEWAY / "src" / name).read_text(encoding="utf-8")


def test_openrpc_retired_server_alias_is_not_advertised():
    document = openrpc()
    assert document["openrpc"] == "1.3.2"
    assert document.get("servers", []) == []
    for method in document["methods"]:
        assert method.get("servers", []) == []
    description = document["info"]["description"]
    urls = re.findall(r"https://[^\s]+", description)
    assert len(urls) == 3
    assert {urlsplit(url).hostname for url in urls} == {
        "api-mainnet-beta.paxeer.network",
        "api1.mainnet-beta.paxeer.network",
        "api14.mainnet-beta.paxeer.network",
    }
    for url in urls:
        parsed = urlsplit(url.rstrip("."))
        assert parsed.path == "/"
        assert not parsed.query and not parsed.fragment
    assert "The unified /rpc interface and its subtree are retired" in description


def test_openrpc_introduction_describes_merged_hosted_namespace_contract():
    document = openrpc()
    names = [method["name"] for method in document["methods"]]
    assert len(names) == len(set(names)) == 17
    assert "pax_sequencerReceipt" in names
    assert "px_getRouteCatalogue" in names
    assert all(name.startswith("px_") or name == "pax_sequencerReceipt" for name in names)
    description = document["info"]["description"]
    for fact in (
        "one execution domain in paxd",
        "EVM chain ID 125 (0x7d)",
        "BFT block commitment as finality",
        "declares the hosted px_* methods and pax_sequencerReceipt",
        "obtain hosted gateway coordinates and credentials from its operator",
        "JSON-RPC 2.0 POST / with Content-Type: application/json",
        "authenticated route discovery uses GET /v1/routes at that gateway",
        "Public node availability does not establish hosted px_* API deployment at public node URLs",
        "no separate LayerX interface or execution router",
        "native signed transactions and chain reads",
        "submitted and receipted do not establish finality",
        "committed block evidence",
        "unavailable evidence produces an explicit refusal",
    ):
        assert fact in description, fact
    for stale in (
        "authenticated activity submission",
        "scoped WebSocket subscriptions",
        "The same endpoint relays",
        "batches that mix EVM and LayerX calls",
    ):
        assert stale not in description, stale


def test_openrpc_locations_match_production_catalogue_and_transport_source():
    document = openrpc()
    catalogue = next(
        method for method in document["methods"] if method["name"] == "px_getRouteCatalogue"
    )
    properties = catalogue["result"]["schema"]["properties"]
    locations = json.loads((GATEWAY / "route-locations.json").read_text(encoding="utf-8"))
    assert locations == {
        "canonical_origin": "https://api-mainnet-beta.paxeer.network",
        "rpc_paths": ["/"],
        "node_rpc": {
            "origin": "https://api-mainnet-beta.paxeer.network",
            "rpc_path": "/",
            "chain_id": 125,
        },
        "hosted_gateway": {
            "location_source": "operator-provided",
            "rpc_path": "/",
            "catalogue_path": "/v1/routes",
        },
    }
    for key, value in locations.items():
        assert properties[key]["const"] == value, key
    assert "operator-provided gateway coordinates" in catalogue["description"]
    producer = source("lib.rs").split("pub fn unified_service_catalogue()", 1)[1]
    assert 'serde_json::from_str(include_str!("../route-locations.json"))' in producer
    assert ".extend(locations);" in producer
    transport = source("main.rs").split("fn route_with_peer(", 1)[1]
    retired = transport.index('if path == "/rpc"')
    refusal = transport.index('return response(404, "not_found", None);', retired)
    dispatch = transport.index('if request.path == "/" && request.method == "POST"')
    assert retired < refusal < dispatch
    assert 'path.starts_with("/rpc/")' in transport[retired:refusal]
    assert "return rpc::route(config, request);" in transport[dispatch:]
    consumer = source("routes.rs")
    assert "layerx_platform_gateway::unified_service_catalogue()" in consumer
    discovery = consumer.split('if request.path == "/v1/routes" ||', 1)[1]
    assert discovery.index('request.method != "GET"') < discovery.index("catalogue().clone()")
    assert discovery.index("authenticate_key(config, request)") < discovery.index("catalogue().clone()")
    assert "return Some(json_response(200, &document));" in discovery
    rpc = source("rpc.rs")
    assert 'if method == "px_getRouteCatalogue"' in rpc
    assert 'crate::routes::rpc_catalogue(config, request, &id, value.get("params"))' in rpc


def test_openrpc_repair_record_maps_finding_and_proof_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["task"]["22"]["33"]["634"]
    finding = record["finding"]["openrpc-retired-server-alias"]
    assert finding["audit"] == "22.30.164"
    assert finding["file"] == "services/platform/hosted/gateway/openrpc.json"
    assert finding["owner_domain"] == 8
    declared = {
        node.name
        for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert len(finding["tests"]) == len(declared)
    assert task["wiring_requests"] == []
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_634.py"
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "No pre-change verification was run" in task["baseline"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
