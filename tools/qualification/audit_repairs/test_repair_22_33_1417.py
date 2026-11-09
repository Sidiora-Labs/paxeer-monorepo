"""Offline source proof of audit 22.30.79 finding 035; no Rust runtime claim."""

import ast
import hashlib
import json
from pathlib import Path
import re
import tomllib
from urllib.parse import urlsplit

import yaml


ROOT = Path(__file__).resolve().parents[3]
GATEWAY = ROOT / "services/platform/hosted/gateway"
RECORD = Path(__file__).with_name("repair_22_33_1417.kvx")


def document():
    return json.loads((GATEWAY / "openrpc.json").read_text(encoding="utf-8"))


def catalogue_method():
    matches = [
        method for method in document()["methods"]
        if method["name"] == "px_getRouteCatalogue"
    ]
    assert len(matches) == 1
    return matches[0]


def rust_function(filename, name):
    source = (GATEWAY / "src" / filename).read_text(encoding="utf-8")
    declaration = re.search(r"\bfn " + re.escape(name) + r"\s*\(", source)
    assert declaration is not None, (filename, name)
    # The inspected functions are top-level; nested closing braces are indented.
    return source[declaration.start():source.index("\n}", declaration.start()) + 2]


def test_035_openrpc_has_no_retired_server_or_route_claims():
    api = document()
    assert api["openrpc"] == "1.3.2"
    assert "servers" not in api
    schema = catalogue_method()["result"]["schema"]
    locations = json.loads((GATEWAY / "route-locations.json").read_text())
    assert locations["rpc_paths"] == ["/"]
    assert locations["node_rpc"] == {
        "origin": "https://api-mainnet-beta.paxeer.network",
        "rpc_path": "/",
        "chain_id": 125,
    }
    assert locations["canonical_origin"] == locations["node_rpc"]["origin"]
    assert locations["hosted_gateway"] == {
        "location_source": "operator-provided",
        "rpc_path": "/",
        "catalogue_path": "/v1/routes",
    }
    assert set(locations) <= set(schema["required"])
    for key, value in locations.items():
        assert schema["properties"][key]["const"] == value
    assert schema["properties"]["rpc_paths"]["items"] == {
        "type": "string", "const": "/",
    }
    assert schema["properties"]["chain_id"]["const"] == 125
    description = catalogue_method()["description"]
    assert "POST /" in description and "GET /v1/routes" in description
    assert "unified /rpc interface and its subtree are retired and omitted" in description
    hosted = schema["properties"]["hosted_gateway"]["description"]
    assert "coordinates and credentials from its operator" in hosted
    assert "relative to that gateway, not node_rpc.origin" in hosted
    assert "Public node availability does not establish hosted px_* API deployment" in api["info"]["description"]


def test_035_locations_match_deployment_origins_and_node_chain():
    resources = list(yaml.safe_load_all((GATEWAY / "deployment.yaml").read_text()))
    ingress, = [item for item in resources if item["kind"] == "Ingress"]
    rule, = ingress["spec"]["rules"]
    origin = catalogue_method()["result"]["schema"]["properties"]["canonical_origin"]["const"]
    assert origin == "https://" + rule["host"]
    tls, = ingress["spec"]["tls"]
    assert tls["hosts"] == [rule["host"]]
    path, = rule["http"]["paths"]
    assert path["path"] == "/" and path["pathType"] == "Prefix"
    assert path["backend"]["service"] == {
        "name": "layerx-gateway", "port": {"name": "https"},
    }
    environment = dict(
        line.split("=", 1)
        for line in (GATEWAY / "railway.env.example").read_text().splitlines()
        if line and not line.startswith("#")
    )
    assert environment["LAYERX_GATEWAY_PUBLIC_HOST"] == rule["host"]
    upstreams = json.loads(environment["LAYERX_GATEWAY_PAXEER_RPC_URLS"])
    assert upstreams
    for upstream in upstreams:
        url = urlsplit(upstream)
        assert url.scheme == "https" and url.hostname
        assert url.path in ("", "/") and not url.query and not url.fragment
    node = list(yaml.safe_load_all(
        (GATEWAY.parent / "node/deployment.yaml").read_text()
    ))
    config, = [item for item in node if item["kind"] == "ConfigMap"]
    assert int(config["data"]["paxeer-chain-id"]) == 125
    assert int(environment["LAYERX_GATEWAY_PROTOCOL_NETWORK_ID"]) == 125
    assert urlsplit(config["data"]["rpc-origin"]).path in ("", "/")


def test_035_real_catalogue_producer_overlays_locations_and_filters_retired_routes():
    producer = rust_function("lib.rs", "unified_service_catalogue")
    assert '"../../../../../tools/paxeer-x/route-catalogue.json"' in producer
    assert 'serde_json::from_str(include_str!("../route-locations.json"))' in producer
    assert re.search(
        r'catalogue\s*\.as_object_mut\(\)\s*\.expect\("route catalogue is an object"\)'
        r'\s*\.extend\(locations\);', producer,
    )
    assert re.search(
        r'catalogue\["routes"\]\s*\.as_array_mut\(\)'
        r'\s*\.expect\("route catalogue has a routes array"\)'
        r'\s*\.retain\(\|route\| \{', producer,
    )
    assert 'let path = route["path"].as_str().expect("route path is a string");' in producer
    assert 'path != "/rpc" && !path.starts_with("/rpc/")' in producer
    assert producer.index(".extend(locations)") < producer.index(".retain(")
    assert "layerx_platform_gateway::unified_service_catalogue()" in rust_function("routes.rs", "catalogue")


def test_035_source_transports_authenticate_and_serialize_the_real_catalogue():
    rpc = rust_function("routes.rs", "rpc_catalogue")
    assert 'if params.is_some_and(|p| p != &json!([]))' in rpc
    assert 'return super::rpc::error(id, -32602, "Invalid params");' in rpc
    assert 'return super::rpc::error(id, -32002, "Authentication required");' in rpc
    assert rpc.index("authenticate_key(config, request).is_err()") < rpc.index("catalogue().clone()")
    assert 'document["readiness"] = config.routes.readiness(config);' in rpc
    assert 'json!({"jsonrpc":"2.0", "id":id, "result":document})' in rpc
    dispatch = rust_function("rpc.rs", "dispatch")
    assert 'if method == "px_getRouteCatalogue"' in dispatch
    assert 'super::routes::rpc_catalogue(config, request, &id, value.get("params"))' in dispatch
    transport = rust_function("rpc.rs", "route")
    assert "serde_json::from_slice::<Value>(&request.body)" in transport
    assert "dispatch(config, request, &value)" in transport
    assert "json_response(200, &value)" in transport
    http = rust_function("routes.rs", "route")
    discovery = http[http.index('if request.path == "/v1/routes" ||'):]
    assert discovery.index('request.method != "GET"') < discovery.index("catalogue().clone()")
    assert discovery.index("authenticate_key(config, request)") < discovery.index("catalogue().clone()")
    assert 'document["readiness"] = config.routes.readiness(config);' in discovery
    assert "return Some(json_response(200, &document));" in discovery
    serializer = rust_function("main.rs", "json_response")
    assert 'content_type: "application/json".to_owned()' in serializer
    assert "body: value.to_string().into_bytes()" in serializer


def test_035_production_ingress_refuses_retired_subtree_before_dispatch():
    ingress = rust_function("main.rs", "route_with_peer")
    assert re.search(
        r'if path == "/rpc"\s*\|\| path.starts_with\("/rpc/"\).*?'
        r'return response\(404, "not_found", None\);', ingress, re.S,
    )
    assert ingress.index('if path == "/rpc"') < ingress.index("routes::route(")
    assert "public_reads::route(config, request)" in ingress
    public = rust_function("public_reads.rs", "route")
    assert 'matches!(request.path.as_str(), "/" | "/openrpc.json")' in public
    assert "return Some(super::rpc::route(config, request));" in public
    rpc = rust_function("rpc.rs", "route")
    assert 'if request.path != "/"' in rpc
    assert 'if request.method != "POST"' in rpc


def test_035_inherited_repair_preserves_entire_openrpc_document():
    snapshot = json.loads((
        Path(__file__).with_name("fixtures") / "repair_22_33_1417_openrpc.json"
    ).read_text())
    assert snapshot["base_revision"] == "b06a8e8502aeb3eee0ed79afb1979f134a140ff1"
    assert snapshot["source"] == "services/platform/hosted/gateway/openrpc.json"
    assert hashlib.sha256((ROOT / snapshot["source"]).read_bytes()).hexdigest() == snapshot["sha256"]


def test_035_record_maps_finding_proofs_and_qualification_limits():
    record = tomllib.loads(RECORD.read_text())
    finding = record["finding"]["035"]
    assert finding["audit"] == "22.30.79"
    assert finding["file"] == "services/platform/hosted/gateway/openrpc.json"
    assert finding["class"] == "retired-reference"
    tests = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == tests
    task = record["task"]["22"]["33"]["1417"]
    assert task["base_revision"] == "b06a8e8502aeb3eee0ed79afb1979f134a140ff1"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "No baseline test was run" in task["baseline"]
    assert "not executed" in task["review_only"]
    assert "tools/paxeer-x/route-catalogue.json" in task["wiring_request"]
    assert "domain 8" in task["wiring_request"]
