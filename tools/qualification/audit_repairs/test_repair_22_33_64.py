"""Offline source qualification for openrpc-retired-route-discovery.

Read the production discovery metadata, its Rust producer and transports, and
the OpenRPC consumer contract. No Rust execution or live deployment is claimed.
"""

import ast
import copy
import hashlib
import json
from pathlib import Path
import re
import tomllib


ROOT = Path(__file__).resolve().parents[3]
GATEWAY = ROOT / "services/platform/hosted/gateway"
RECORD = Path(__file__).with_name("repair_22_33_64.kvx")


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def method(name):
    matches = [
        item for item in read_json(GATEWAY / "openrpc.json")["methods"]
        if item["name"] == name
    ]
    assert len(matches) == 1, name
    return matches[0]


def rust_function(filename, name):
    source = (GATEWAY / "src" / filename).read_text(encoding="utf-8")
    declaration = re.search(r"\bfn " + re.escape(name) + r"\s*(?:<[^>]*>)?\(", source)
    assert declaration is not None, (filename, name)
    # These are top-level functions; nested blocks are indented.
    return source[declaration.start():source.index("\n}", declaration.start()) + 2]


def test_route_locations_match_openrpc_and_real_catalogue_producer():
    locations = read_json(GATEWAY / "route-locations.json")
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
    schema = method("px_getRouteCatalogue")["result"]["schema"]
    assert set(schema["required"]) == {
        "schema", "chain_id", "services", "routes", "readiness", *locations,
    }
    assert schema["properties"]["schema"]["const"] == "paxeer-x.routes.v1"
    assert schema["properties"]["chain_id"]["const"] == 125
    for name, value in locations.items():
        assert schema["properties"][name]["const"] == value
    assert schema["properties"]["rpc_paths"]["items"] == {
        "type": "string", "const": "/",
    }
    for name in ("node_rpc", "hosted_gateway"):
        assert schema["properties"][name]["type"] == "object"
    hosted = schema["properties"]["hosted_gateway"]["description"]
    assert "coordinates and credentials from its operator" in hosted
    assert "relative to that gateway, not node_rpc.origin" in hosted
    description = method("px_getRouteCatalogue")["description"]
    assert "merged Paxeer network (chain 125)" in description
    assert "POST /" in description and "GET /v1/routes" in description
    assert "/rpc interface and its subtree are retired and omitted" in description

    producer = rust_function("lib.rs", "unified_service_catalogue")
    assert "CATALOGUE.get_or_init(||" in producer
    assert '"../../../../../tools/paxeer-x/route-catalogue.json"' in producer
    assert 'serde_json::from_str(include_str!("../route-locations.json"))' in producer
    assert "let locations: serde_json::Map<String, serde_json::Value>" in producer
    assert re.search(
        r'catalogue\s*\.as_object_mut\(\)\s*\.expect\("route catalogue is an object"\)'
        r'\s*\.extend\(locations\);', producer,
    )
    assert producer.index(".extend(locations)") < producer.rindex("catalogue\n")
    assert (GATEWAY / "src/../route-locations.json").resolve() == (
        GATEWAY / "route-locations.json"
    ).resolve()


def test_retired_rpc_subtree_is_omitted_and_refused_by_production_ingress():
    producer = rust_function("lib.rs", "unified_service_catalogue")
    assert re.search(
        r'catalogue\["routes"\]\s*\.as_array_mut\(\)'
        r'\s*\.expect\("route catalogue has a routes array"\)'
        r'\s*\.retain\(\|route\| \{', producer,
    )
    assert 'let path = route["path"].as_str().expect("route path is a string");' in producer
    assert 'path != "/rpc" && !path.starts_with("/rpc/")' in producer
    ingress = rust_function("main.rs", "route_with_peer")
    assert re.search(
        r'if path == "/rpc"\s*\|\| path.starts_with\("/rpc/"\).*?'
        r'return response\(404, "not_found", None\);', ingress, re.S,
    )
    assert ingress.index('if path == "/rpc"') < ingress.index("routes::route(")
    assert 'if request.path == "/" && request.method == "POST"' in ingress
    assert "return rpc::route(config, request);" in ingress
    description = method("px_getRouteCatalogue")["description"]
    assert "unified /rpc interface and its subtree are retired" in description


def test_catalogue_source_path_preserves_authentication_readiness_and_json_transport():
    owner = rust_function("routes.rs", "catalogue")
    assert "layerx_platform_gateway::unified_service_catalogue()" in owner
    rpc = rust_function("routes.rs", "rpc_catalogue")
    assert 'if params.is_some_and(|p| p != &json!([]))' in rpc
    assert 'return super::rpc::error(id, -32602, "Invalid params");' in rpc
    assert "if authenticate_key(config, request).is_err()" in rpc
    assert 'return super::rpc::error(id, -32002, "Authentication required");' in rpc
    assert rpc.index("authenticate_key(") < rpc.index("catalogue().clone()")
    assert 'document["readiness"] = config.routes.readiness(config);' in rpc
    assert 'json!({"jsonrpc":"2.0", "id":id, "result":document})' in rpc
    dispatch = rust_function("rpc.rs", "dispatch")
    assert 'if method == "px_getRouteCatalogue"' in dispatch
    assert 'crate::routes::rpc_catalogue(config, request, &id, value.get("params"))' in dispatch
    transport = rust_function("rpc.rs", "route")
    assert "serde_json::from_slice::<Value>(&request.body)" in transport
    assert "dispatch(config, request, &value)" in transport
    assert "json_response(200, &value)" in transport
    http = rust_function("routes.rs", "route")
    assert 'if request.path == "/v1/routes"' in http
    assert http.index("authenticate_key(config, request)") < http.index("catalogue().clone()")
    assert 'document["readiness"] = config.routes.readiness(config);' in http
    assert "return Some(json_response(200, &document));" in http
    serializer = rust_function("main.rs", "json_response")
    assert 'content_type: "application/json".to_owned()' in serializer
    assert "body: value.to_string().into_bytes()" in serializer


def test_account_alias_wording_preserves_exact_lookup_selectors_and_refusals():
    entry = method("px_resolveAccount")
    assert "merged Paxeer network (chain 125)" in entry["description"]
    assert "aliases for one account" in entry["description"]
    assert "across both domains" not in entry["description"]
    assert "native_account_reverse_resolution_unsupported" in entry["description"]
    assert "conflicting_identity_binding" in entry["description"]
    expected = "^(0x[0-9a-fA-F]{40}|did:layerx:[0-9a-fA-F]{64})$"
    for name in ("px_resolveAccount", "px_getAccount", "px_getBalances"):
        selector = method(name)["params"][0]
        assert selector["required"] is True
        assert selector["schema"] == {"type": "string", "pattern": expected}
        for valid in ("0x" + "aB" * 20, "did:layerx:" + "aB" * 32):
            assert re.fullmatch(expected, valid)
        for invalid in ("aB" * 32, "0x" + "ab" * 32, "did:layerx:" + "ab" * 31):
            assert re.fullmatch(expected, invalid) is None
    selector = rust_function("paxeer.rs", "account_selector")
    assert 'account.strip_prefix("did:layerx:")' in selector
    assert "Selector::NativeAccount(key)" in selector
    resolve = rust_function("paxeer.rs", "resolve")
    assert "Selector::NativeAccount(account)" in resolve
    assert 'return Err("native_account_reverse_resolution_unsupported");' in resolve


def test_exact_bank_amount_asset_limit_and_consensus_head_schemas_are_preserved():
    balances = method("px_getBalances")["result"]["schema"]
    amount = balances["properties"]["balances"]["items"]["properties"]["amount"]
    assert amount == {"type": "string", "pattern": "^[0-9]+$"}
    for value in ("0", str(2**64), str(2**256 - 1)):
        assert re.fullmatch(amount["pattern"], value)
    assert balances["additionalProperties"] is False
    assert method("px_listAssets")["result"]["schema"]["properties"]["assets"]["maxItems"] == 16
    bank = rust_function("paxeer.rs", "get_balances")
    assert "resolved_at_height(config, id, params)" in bank
    assert "node_at_height(" in bank
    assert "evm::SELECTOR_BANK_ALL_BALANCES" in bank
    assert '"balances": balances' in bank
    assert "evm::uint256_decimal(&amount)" in rust_function("paxeer.rs", "decode_coins")
    assert "word: &[u8; 32]" in rust_function("evm.rs", "uint256_decimal")

    network = method("px_getNetwork")["result"]["schema"]
    head = network["properties"]["consensus"]
    assert head["type"] == ["object", "null"]
    assert head["additionalProperties"] is False
    assert head["required"] == [
        "latest_height", "latest_block_hash", "latest_app_hash", "catching_up",
    ]
    assert head["properties"]["latest_height"] == {"type": "string", "pattern": "^[0-9]+$"}
    assert head["properties"]["catching_up"] == {"type": "boolean"}
    for name in ("latest_block_hash", "latest_app_hash"):
        assert head["properties"][name] == {
            "type": "string", "pattern": "^([0-9a-fA-F]{2})*$",
        }
    assert network["properties"]["reason"]["enum"] == ["consensus_unavailable"]
    assert len(network["oneOf"]) == 2
    consensus = (GATEWAY / "src/chain_reads.rs").read_text(encoding="utf-8")
    assert '#[serde(rename(deserialize = "latest_block_height"))]' in consensus
    for field in ("latest_height: String", "latest_block_hash: String", "latest_app_hash: String", "catching_up: bool"):
        assert field in consensus
    assert "super::chain_reads::consensus_head(config, id)" in rust_function("paxeer.rs", "get_network")
    result = rust_function("paxeer.rs", "network_result")
    assert 'result["reason"] = json!("consensus_unavailable");' in result


def test_all_unrelated_openrpc_contracts_match_the_recorded_base():
    snapshot = read_json(GATEWAY / "tests/fixtures/repair_22_33_64_openrpc.json")
    assert snapshot["base_revision"] == "716e8dfe1518af0033ed3d0e2b7b4f34fa81f0aa"
    document = copy.deepcopy(read_json(GATEWAY / "openrpc.json"))
    document["methods"] = [
        item for item in document["methods"] if item["name"] != "px_getRouteCatalogue"
    ]
    for item in document["methods"]:
        if item["name"] == "px_resolveAccount":
            del item["description"]
    canonical = json.dumps(document, sort_keys=True, separators=(",", ":")).encode()
    assert hashlib.sha256(canonical).hexdigest() == snapshot["sha256"]


def test_repair_record_maps_finding_proofs_scope_limits_and_revision_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["openrpc-retired-route-discovery"]
    assert finding["audit"] == "22.30.107"
    assert finding["file"] == "services/platform/hosted/gateway/openrpc.json"
    assert finding["class"] == "doc-drift"
    tests = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == tests
    task = record["task"]["22"]["33"]["64"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "source facts" in task["source_proof"]
    assert "not executed" in task["review_only"]
    assert "tools/paxeer-x/route-catalogue.json" in task["wiring_request"]
    assert "docs/system/routes.md" in task["wiring_request"]
    assert "domain 8" in task["wiring_request"]
