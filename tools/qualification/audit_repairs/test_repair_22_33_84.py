"""Offline source proofs for audit 22.30.11/spot-openapi-dialect."""

import ast
import hashlib
import json
from pathlib import Path
import re
import tomllib


ROOT = Path(__file__).resolve().parents[3]
CONTRACT = ROOT / "apps/perps/swagger_spot.json"
RECORD = Path(__file__).with_name("repair_22_33_84.kvx")
BASELINE_SEMANTIC_SHA256 = "cd6bafe2e763cbc5d5bbb46ee37ff5e88fb0974527e25c882dddf108884d7916"
SCHEMA_TYPES = {"string", "number", "integer", "boolean", "array", "object"}
SCHEMA_KEYS = {
    "type", "nullable", "description", "format", "pattern", "minimum",
    "maximum", "required", "properties", "items", "oneOf", "anyOf", "allOf",
    "additionalProperties", "enum", "default", "example",
}


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        assert key not in result, f"duplicate JSON key: {key}"
        result[key] = value
    return result


def contract():
    return json.loads(CONTRACT.read_text(encoding="utf-8"), object_pairs_hook=unique_object)


def schema_roots(document):
    for path, item in document["paths"].items():
        operation = item["get"]
        for parameter in operation.get("parameters", []):
            yield f"{path}/parameter/{parameter['name']}", parameter["schema"]
        for status, response in operation["responses"].items():
            for media, content in response.get("content", {}).items():
                yield f"{path}/response/{status}/{media}", content["schema"]
    yield from document["components"]["schemas"].items()


def schema_nodes(schema, path):
    yield path, schema
    for name, child in schema.get("properties", {}).items():
        yield from schema_nodes(child, f"{path}/properties/{name}")
    if "items" in schema:
        yield from schema_nodes(schema["items"], f"{path}/items")
    for keyword in ("oneOf", "anyOf", "allOf"):
        for index, child in enumerate(schema.get(keyword, [])):
            yield from schema_nodes(child, f"{path}/{keyword}/{index}")
    additional = schema.get("additionalProperties")
    if isinstance(additional, dict):
        yield from schema_nodes(additional, f"{path}/additionalProperties")


def canonical_schema(schema):
    """Compare null unions across dialects, retaining all other schema facts."""
    result = dict(schema)
    for keyword in ("properties",):
        if keyword in result:
            result[keyword] = {
                name: canonical_schema(child) for name, child in result[keyword].items()
            }
    for keyword in ("items", "additionalProperties"):
        if isinstance(result.get(keyword), dict):
            result[keyword] = canonical_schema(result[keyword])
    for keyword in ("oneOf", "anyOf", "allOf"):
        if keyword in result:
            branches = []
            for child in result[keyword]:
                normalized = canonical_schema(child)
                if keyword == "oneOf" and set(normalized) == {"oneOf"}:
                    branches.extend(normalized["oneOf"])
                else:
                    branches.append(normalized)
            result[keyword] = sorted(branches, key=lambda x: json.dumps(x, sort_keys=True))
    if "example" in result:
        result["examples"] = [result.pop("example")]
    types = result.get("type")
    nullable = result.pop("nullable", False)
    if isinstance(types, list) or nullable:
        members = list(types) if isinstance(types, list) else [types]
        if nullable and "null" not in members:
            members.append("null")
        attributes = {key: value for key, value in result.items() if key != "type"}
        branches = [
            {"type": member, **(attributes if member != "null" else {})}
            for member in members
        ]
        return {"oneOf": sorted(branches, key=lambda x: json.dumps(x, sort_keys=True))}
    return result


def canonical_document(value):
    if isinstance(value, dict):
        return {
            key: canonical_schema(child) if key == "schema" else canonical_document(child)
            for key, child in value.items()
        }
    if isinstance(value, list):
        return [canonical_document(child) for child in value]
    return value


def response_schema(document, suffix):
    return document["paths"]["/v1/users/{address}/" + suffix]["get"]["responses"]["200"][
        "content"
    ]["application/json"]["schema"]


def admitted_types(schema):
    types = {schema["type"]} if "type" in schema else set()
    if schema.get("nullable") is True:
        types.add("null")
    for child in schema.get("oneOf", []):
        types.update(admitted_types(child))
    return types


def test_spot_openapi_303_schema_dialect():
    document = contract()
    assert document["openapi"] == "3.0.3"
    assert len(document["paths"]) == 14
    nodes = [node for path, root in schema_roots(document) for node in schema_nodes(root, path)]
    assert len(nodes) > 300
    nullable_count = 0
    for path, schema in nodes:
        assert isinstance(schema, dict), path
        assert set(schema) <= SCHEMA_KEYS, (path, set(schema) - SCHEMA_KEYS)
        if "type" in schema:
            assert isinstance(schema["type"], str), path
            assert schema["type"] in SCHEMA_TYPES, path
        if "nullable" in schema:
            assert schema["nullable"] is True and "type" in schema, path
            nullable_count += 1
            assert "enum" not in schema or None in schema["enum"], path
        for keyword in ("oneOf", "anyOf", "allOf"):
            if keyword in schema:
                assert isinstance(schema[keyword], list) and schema[keyword], path
        if "required" in schema:
            required = schema["required"]
            assert isinstance(required, list) and len(required) == len(set(required)), path
            assert set(required) <= set(schema["properties"]), path
        if schema.get("type") == "array":
            assert isinstance(schema["items"], dict), path
        if "pattern" in schema:
            re.compile(schema["pattern"])
        if "minimum" in schema and "maximum" in schema:
            assert schema["minimum"] <= schema["maximum"], path
    assert nullable_count == 53


def test_spot_entire_contract_preserves_baseline_semantics():
    normalized = canonical_document(contract())
    serialized = json.dumps(normalized, sort_keys=True, separators=(",", ":"))
    assert hashlib.sha256(serialized.encode()).hexdigest() == BASELINE_SEMANTIC_SHA256


def test_spot_nullable_fields_match_producer_and_consumer_sources():
    document = contract()
    producer = source("services/platform/perps-data/internal/rest/spotusers.go")
    consumer = source("apps/perps/src/core/spot/api.ts")
    holdings = response_schema(document, "holdings")["properties"]["erc20"]["items"]
    positions = response_schema(document, "positions")["properties"]["items"]["items"]
    closed = response_schema(document, "closed-legs")["properties"]["items"]["items"]
    orders = response_schema(document, "orders")["properties"]["items"]["items"]
    mappings = [
        ("spotHoldingJSON", "SpotHoldingsResponse", holdings),
        ("spotPositionJSON", "SpotPosition", positions),
        ("spotDerivedJSON", "SpotPosition", positions["properties"]["derived"]),
        ("spotClosedLegJSON", "SpotClosedLeg", closed),
        ("spotOrderJSON", "SpotOrder", orders),
    ]
    pointer_count = 0
    for go_name, ts_name, schema in mappings:
        go_body = re.search(r"type " + go_name + r" struct \{(.*?)^\}", producer, re.M | re.S)
        ts_body = re.search(r"export interface " + ts_name + r" \{(.*?)^\}", consumer, re.M | re.S)
        assert go_body and ts_body, (go_name, ts_name)
        ts_fields = ts_body[1]
        if ts_name == "SpotHoldingsResponse":
            ts_fields = ts_fields.split("erc20: Array<{", 1)[1].split("}>", 1)[0]
        fields = re.findall(r'^\s*\w+\s+(\S+)\s+`json:"([^",]+)"`', go_body[1], re.M)
        assert {name for _, name in fields} == set(schema["properties"])
        for go_type, name in fields:
            types = admitted_types(schema["properties"][name])
            assert ("null" in types) == go_type.startswith("*"), (go_name, name)
            if go_type.startswith("*"):
                pointer_count += 1
                ts_field = re.search(r"\b" + name + r"\??:\s*([^;]+);", ts_fields)
                assert ts_field and "null" in ts_field[1].split(" | "), (ts_name, name)
    assert pointer_count == 33
    assert set(positions["required"]) == set(positions["properties"])
    derived = positions["properties"]["derived"]
    assert set(derived["required"]) == set(derived["properties"])
    assert admitted_types(holdings["properties"]["decimals"]) == {"integer", "string", "null"}
    assert admitted_types(closed["properties"]["holdingSeconds"]) == {"integer", "string", "null"}
    stats = response_schema(document, "trades/stats")
    assert stats["required"] == ["address", "breakdown"]
    breakdown = stats["properties"]["breakdown"]["items"]
    assert breakdown["required"] == ["source", "contract", "count", "volumeUsd"]
    assert admitted_types(breakdown["properties"]["contract"]) == {"string", "null"}


def test_spot_producer_transport_consumer_source_path():
    producer = source("services/platform/perps-data/internal/rest/spotusers.go")
    consumer = source("apps/perps/src/core/spot/api.ts")
    for route, handler in (("holdings", "holdings"), ("positions", "positions"),
                           ("orders", "orders"), ("closed-legs", "closedLegs"),
                           ("trades/stats", "tradesStats")):
        assert re.search(r'"' + re.escape(route) + r'":\s+s\.' + handler + r'\b', producer)
        assert "/" + route in consumer
        assert "/v1/users/{address}/" + route in contract()["paths"]
    assert 'mux.HandleFunc("GET /v1/users/{address}/"+path, s.serveUser(route))' in producer
    assert "body, err := route(r.Context(), common.HexToAddress(raw), r.URL.Query())" in producer
    assert "spotWrite(w, body, err)" in producer
    assert "writeJSON(w, http.StatusOK, body)" in producer
    assert 'method: "GET"' in consumer and 'headers: { Accept: "application/json" }' in consumer
    assert "body = await res.json()" in consumer and "return body as T" in consumer
    assert "return addr.toLowerCase()" in consumer
    assert "Promise<SpotPositionsResponse>" in consumer
    assert "Promise<SpotTradesStats>" in consumer
    portfolio = source("apps/perps/src/hooks/spot/useSpotPortfolio.ts")
    assert "spotApi.getPositions(account, snapshot.controller.signal)" in portfolio


def test_spot_dialect_repair_record_and_proving_tests():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["spot-openapi-dialect"]
    assert record["task"]["id"] == "22.33.84"
    assert finding["audit"] == "22.30.11"
    assert finding["file"] == "apps/perps/swagger_spot.json"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared and len(declared) == 5
    assert record["baseline"]["semantic_sha256"] == BASELINE_SEMANTIC_SHA256
    assert record["proof"]["review_only"]
    assert record["task"]["wiring_requests"]
    assert record["qualification"]["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_84.py"
    )
